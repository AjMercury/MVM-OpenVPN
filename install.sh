#!/usr/bin/env bash
# Self-contained Linux bootstrap; safe when this file is the shell's stdin.
set +x
set -Eeuo pipefail
umask 077
REPOSITORY=AjMercury/MvM-OpenVPN
SIGNER_REPOSITORY=AjMercury/MVM-OpenVPN
# Official Sigstore public-good root, pinned independently of application releases.
# Refresh only after reviewing sigstore/root-signing changes and testing old/new bundles.
SIGSTORE_ROOT_COMMIT=63528388118c1002d1221321741159da24c256af
SIGSTORE_ROOT_SHA256=6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66
bootstrap_die() { printf 'MVM: %s\n' "$*" >&2; exit 1; }
fetch() {
 curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 \
  --connect-timeout 15 --max-time 600 --retry 2 --max-filesize 536870912 --output "$2" "$1" || bootstrap_die 'GitHub API or asset download failed; no installation performed.'
}
bootstrap_platform() {
 [[ $EUID == 0 ]] || bootstrap_die 'Run with sudo.'
 [[ $(uname -s) == Linux ]] || bootstrap_die 'Linux required.'
 . /etc/os-release
 case "$ID:${VERSION_ID:-}" in
  ubuntu:22.04|ubuntu:24.04|debian:12|debian:13) ;;
  *) bootstrap_die 'Implementation targets: Ubuntu 22.04/24.04 LTS and Debian 12/13 only.';;
 esac
 case $(uname -m) in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) bootstrap_die 'Unsupported architecture; amd64 or arm64 required.';; esac
 [[ -d /run/systemd/system ]] || bootstrap_die 'A running systemd host is required.'
}
bootstrap_dependencies() {
 local pair answer
 local -a packages=()
 for pair in curl:curl python3:python3 tar:tar sha256sum:coreutils; do
  command -v "${pair%%:*}" >/dev/null || packages+=("${pair#*:}")
 done
 [[ -s /etc/ssl/certs/ca-certificates.crt ]] || packages+=(ca-certificates)
 if ((${#packages[@]})); then
  printf 'Required bootstrap packages: %s\n' "${packages[*]}"
  read -r -p 'Install these packages using APT (no system upgrade)? [yes/no]: ' answer
  [[ $answer == yes ]] || bootstrap_die 'Dependencies declined.'
  apt-get update && apt-get install -y --no-install-recommends "${packages[@]}" || bootstrap_die 'APT failed.'
 fi
 [[ ${1:-all} != base ]] || return 0
 local help
 help=$(gh attestation verify --help 2>/dev/null) || help=
 if [[ $help != *--source-ref* || $help != *--deny-self-hosted-runners* ]]; then
  printf 'A current GitHub CLI is required to verify release attestations.\n'
  read -r -p 'Install gh from the official HTTPS cli.github.com signed APT repository? [yes/no]: ' answer
  [[ $answer == yes ]] || bootstrap_die 'Authenticated verifier installation declined.'
  install -d -m 755 /etc/apt/keyrings
  local key_tmp
  key_tmp=$(mktemp /etc/apt/keyrings/.mvm-gh-XXXXXXXX)
  # Verifier trust is bootstrapped from the independent official CLI distribution,
  # over system-CA-verified HTTPS, not from this application's release assets.
  if ! curl --fail --silent --show-error --proto '=https' --tlsv1.2 --connect-timeout 15 --max-time 60 \
    https://cli.github.com/packages/githubcli-archive-keyring.gpg -o "$key_tmp"; then
   rm -f -- "$key_tmp"; bootstrap_die 'Official GitHub CLI key download failed.'
  fi
  # Published independently in github.com/cli/cli/docs/install_linux.md.
  if ! printf '6084d5d7bd8e288441e0e94fc6275570895da18e6751f70f057485dc2d1a811b  %s\n' "$key_tmp" | sha256sum --check --status; then
   rm -f -- "$key_tmp"; bootstrap_die 'GitHub CLI APT key differs from pinned trust root. Maintainer review required.'
  fi
  chmod 644 "$key_tmp"; mv -f -- "$key_tmp" /etc/apt/keyrings/mvm-githubcli.gpg
  printf 'deb [arch=%s signed-by=/etc/apt/keyrings/mvm-githubcli.gpg] https://cli.github.com/packages stable main\n' "$ARCH" > /etc/apt/sources.list.d/mvm-githubcli.list
  apt-get update && apt-get install -y --no-install-recommends gh || bootstrap_die 'Signed GitHub CLI APT installation failed.'
  help=$(gh attestation verify --help) || bootstrap_die 'GitHub CLI verification unavailable.'
 fi
 for flag in --bundle --source-ref --cert-identity --deny-self-hosted-runners --custom-trusted-root; do
  [[ $help == *"$flag"* ]] || bootstrap_die "Installed gh lacks $flag; see docs/operations/releases.md."
 done
}
select_release() {
 python3 - "$1" "$CHANNEL" "$PIN" "$ARCH" <<'PY'
import json, re, sys
releases=json.load(open(sys.argv[1])); channel,pin,arch=sys.argv[2:]
if pin and isinstance(releases,dict) and releases.get('tag_name')==pin: releases=[releases]
if not isinstance(releases,list): raise SystemExit('Invalid GitHub release response')
eligible=[]
for r in releases:
    tag=r.get('tag_name',''); version=tag.removeprefix('v')
    if not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?',tag): continue
    preview='-' in version
    if r.get('draft') or bool(r.get('prerelease')) != preview: continue
    if preview != (channel=='preview') or (pin and tag != pin): continue
    names=[a.get('name') for a in r.get('assets',[])]
    required=[f'mvm-linux-{arch}-{version}.tar.gz','SHA256SUMS','SHA256SUMS.sigstore.json']
    if any(names.count(n)!=1 for n in required): continue
    eligible.append(r)
if not eligible: raise SystemExit('No eligible official release for channel/version/architecture.')
print(max(eligible,key=lambda r:r.get('published_at') or '')['tag_name'])
PY
}
verify_manifest() {
 # Fetch the independently pinned official root via GitHub, avoiding blocked TUF CDNs.
 fetch "https://raw.githubusercontent.com/sigstore/root-signing/$SIGSTORE_ROOT_COMMIT/targets/trusted_root.json" "$WORK/trusted-root.json"
 printf '%s  %s\n' "$SIGSTORE_ROOT_SHA256" "$WORK/trusted-root.json" | sha256sum --check --status \
  || bootstrap_die 'Official Sigstore trust root checksum mismatch. No archive executed.'
 python3 - "$WORK/trusted-root.json" "$WORK/trusted-root.jsonl" <<'PY'
import json,sys
with open(sys.argv[1]) as source: root=json.load(source)
with open(sys.argv[2],'w') as output: output.write(json.dumps(root,separators=(',',':'))+'\n')
PY
 gh attestation verify "$WORK/SHA256SUMS" --bundle "$WORK/SHA256SUMS.sigstore.json" \
  --custom-trusted-root "$WORK/trusted-root.jsonl" \
  --repo "$SIGNER_REPOSITORY" --cert-identity "https://github.com/$SIGNER_REPOSITORY/.github/workflows/release.yaml@refs/tags/$TAG" \
  --source-ref "refs/tags/$TAG" --deny-self-hosted-runners >"$WORK/verification.log" 2>&1 \
  || { cat "$WORK/verification.log" >&2; bootstrap_die 'Release attestation verification failed. No archive executed.'; }
}
extract_payload() {
 python3 - "$WORK/$ASSET" "$WORK/payload" <<'PY'
import pathlib, shutil, sys, tarfile
destination=pathlib.Path(sys.argv[2]); destination.mkdir(mode=0o700)
with tarfile.open(sys.argv[1],'r:gz') as archive:
    members=archive.getmembers(); seen=set(); total=0
    for m in members:
        p=pathlib.PurePosixPath(m.name)
        if (p.is_absolute() or '..' in p.parts or '\\' in m.name or ':' in m.name or
            any(ord(c)<32 or ord(c)==127 for c in m.name) or
            not (m.isfile() or m.isdir()) or p in seen):
            raise SystemExit('Unsafe archive member rejected')
        seen.add(p); total+=m.size
        if total>1073741824 or len(members)>20000: raise SystemExit('Archive exceeds limits')
    for m in members:
        target=destination.joinpath(*pathlib.PurePosixPath(m.name).parts)
        if m.isdir(): target.mkdir(parents=True,exist_ok=True,mode=0o700)
        else:
            target.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
            with archive.extractfile(m) as src, target.open('xb') as dst: shutil.copyfileobj(src,dst)
            target.chmod(0o700 if m.mode & 0o111 else 0o600)
PY
}
bootstrap_cleanup() {
 local code=$?
 if ((code==0)); then rm -rf -- "$WORK";
 else printf 'MVM failed. Private diagnostics/downloads retained: %s\nInspect journalctl -u mvm; preserve /etc/mvm and /var/lib/mvm for repair.\n' "$WORK" >&2; fi
}
bootstrap_main() {
 CHANNEL=stable PIN= ACTION=install
 while (($#)); do
  case $1 in
   --channel) [[ $# -ge 2 ]] || bootstrap_die 'Missing channel'; CHANNEL=$2; shift 2;;
   --version) [[ $# -ge 2 ]] || bootstrap_die 'Missing version'; PIN=$2; shift 2;;
   --action) [[ $# -ge 2 ]] || bootstrap_die 'Missing action'; ACTION=$2; shift 2;;
   *) bootstrap_die 'Usage: install.sh [--channel stable|preview] [--version vX.Y.Z[-preview.N]] [--action install|update|repair]';;
  esac
 done
 [[ $CHANNEL == stable || $CHANNEL == preview ]] || bootstrap_die 'Invalid channel.'
 [[ $ACTION == install || $ACTION == update || $ACTION == repair ]] || bootstrap_die 'Invalid action.'
 [[ -z $PIN || $PIN =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]] || bootstrap_die 'Invalid version tag.'
 bootstrap_platform
 if [[ $ACTION == install ]]; then
  for path in /opt/mvm/current /etc/mvm/config.env /etc/mvm/master.key /var/lib/mvm/mvm.db /usr/local/bin/mvm /etc/systemd/system/mvm.service; do
   [[ ! -e $path && ! -L $path ]] || bootstrap_die 'Existing/incomplete installation detected. Use sudo mvm Update or Install / Repair; data retained.'
  done
 fi
 bootstrap_dependencies base
 WORK=$(mktemp -d /var/tmp/mvm-bootstrap-XXXXXXXX)
 trap bootstrap_cleanup EXIT
 if [[ -n $PIN ]]; then
  fetch "https://api.github.com/repos/$REPOSITORY/releases/tags/$PIN" "$WORK/releases.json"
 else
  fetch "https://api.github.com/repos/$REPOSITORY/releases?per_page=100" "$WORK/releases.json"
 fi
 TAG=$(select_release "$WORK/releases.json") || bootstrap_die "No eligible $CHANNEL release for $ARCH. For Development Preview use: bash -s -- --channel preview. No installation performed."
 VERSION=${TAG#v}; ASSET="mvm-linux-$ARCH-$VERSION.tar.gz"
 if [[ $CHANNEL == preview ]]; then
  printf 'WARNING: Development Preview %s. Production deployment is not recommended.\n' "$VERSION"
  read -r -p 'Type INSTALL PREVIEW to continue: ' answer
  [[ $answer == 'INSTALL PREVIEW' ]] || bootstrap_die 'Preview declined.'
 fi
 bootstrap_dependencies
 local url="https://github.com/$REPOSITORY/releases/download/$TAG" digest
 fetch "$url/SHA256SUMS" "$WORK/SHA256SUMS"
 fetch "$url/SHA256SUMS.sigstore.json" "$WORK/SHA256SUMS.sigstore.json"
 verify_manifest
 digest=$(python3 - "$WORK/SHA256SUMS" "$ASSET" <<'PY'
import re,sys
lines=[line.split() for line in open(sys.argv[1])]
matches=[p[0] for p in lines if len(p)==2 and p[1]==sys.argv[2] and re.fullmatch('[a-f0-9]{64}',p[0])]
if len(matches)!=1: raise SystemExit('Missing/ambiguous signed artifact digest')
print(matches[0])
PY
 ) || bootstrap_die 'Invalid signed manifest.'
 fetch "$url/$ASSET" "$WORK/$ASSET"
 printf '%s  %s\n' "$digest" "$WORK/$ASSET" | sha256sum --check --status || bootstrap_die 'Archive checksum mismatch.'
 extract_payload || bootstrap_die 'Unsafe or corrupt archive.'
 grep -qx "VERSION=$VERSION" "$WORK/payload/metadata.env" || bootstrap_die 'Version mismatch.'
 grep -qx "ARCH=$ARCH" "$WORK/payload/metadata.env" || bootstrap_die 'Architecture mismatch.'
 local maturity=Stable
 [[ $CHANNEL != preview ]] || maturity=DevelopmentPreview
 grep -qx "MATURITY=$maturity" "$WORK/payload/metadata.env" || bootstrap_die 'Release maturity does not match selected channel.'
 if [[ $CHANNEL == preview ]]; then export MVM_ACCEPT_PREVIEW='I ACCEPT DEVELOPMENT PREVIEW'; fi
 bash "$WORK/payload/editions/linux/installer/menu.sh" "$ACTION" "$WORK/$ASSET" "$digest" "$VERSION"
}
# Redirect only the function, leaving the pipe available to Bash's script parser.
if [[ ${BASH_SOURCE[0]:-} == "$0" || -z ${BASH_SOURCE[0]:-} ]]; then
 if ! (: </dev/tty) 2>/dev/null; then bootstrap_die 'Interactive terminal required (/dev/tty). No noninteractive installation mode is offered.'; fi
 bootstrap_main "$@" </dev/tty
fi
