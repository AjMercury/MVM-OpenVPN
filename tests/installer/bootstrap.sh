#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/install.sh"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
if [[ -n ${MVM_TEST_PYTHON:-} ]]; then python3(){ "$MVM_TEST_PYTHON" "$@"; }; fi
CHANNEL=preview PIN= ARCH=amd64
printf '[]' > "$tmp/releases.json"
if (select_release "$tmp/releases.json"); then exit 1; fi
printf '{"message":"API rate limit"}' > "$tmp/releases.json"
if (select_release "$tmp/releases.json"); then exit 1; fi
python3 - "$tmp/releases.json" <<'PY'
import json,sys
def release(tag,preview=True):
 return dict(tag_name=tag,prerelease=preview,draft=False,published_at=tag,assets=[dict(name=n) for n in [f'mvm-linux-amd64-{tag[1:]}.tar.gz','SHA256SUMS','SHA256SUMS.sigstore.json']])
json.dump([release('v0.2.0-preview.1'),release('v0.2.0-preview.2'),release('v0.1.0',False)],open(sys.argv[1],'w'))
PY
[[ $(select_release "$tmp/releases.json") == v0.2.0-preview.2 ]]
PIN=v0.2.0-preview.1
[[ $(select_release "$tmp/releases.json") == "$PIN" ]]
PIN= CHANNEL=stable
[[ $(select_release "$tmp/releases.json") == v0.1.0 ]]
ARCH=mips
if (select_release "$tmp/releases.json"); then exit 1; fi
curl(){ return 22; }
if (fetch https://api.github.com/test "$tmp/result"); then exit 1; fi
unset -f curl
WORK=$tmp TAG=v0.2.0-preview.1
# The separately pinned trust root must be obtained and checked before verification.
printf '{}\n' > "$tmp/official-root.json"
SIGSTORE_ROOT_SHA256=$(sha256sum "$tmp/official-root.json" | cut -d ' ' -f 1)
fetch(){ cp "$tmp/official-root.json" "$2"; }
gh(){ printf '%s\n' "$*" > "$tmp/gh-arguments"; return 1; }
if (verify_manifest); then exit 1; fi
grep -q -- '--source-ref refs/tags/v0.2.0-preview.1 --deny-self-hosted-runners' "$tmp/gh-arguments"
grep -q -- '--cert-identity https://github.com/AjMercury/MVM-OpenVPN/.github/workflows/release.yaml@refs/tags/' "$tmp/gh-arguments"
unset -f gh
gh(){ return 0; }
verify_manifest
SIGSTORE_ROOT_SHA256=$(printf invalid | sha256sum | cut -d ' ' -f 1)
if (verify_manifest); then echo 'Tampered trust root accepted'; exit 1; fi
unset -f gh fetch
ASSET=test.tar.gz
for member in safe traversal absolute symlink duplicate control; do
 rm -rf -- "$WORK/payload"
 python3 - "$WORK/$ASSET" "$member" <<'PY'
import io,sys,tarfile
mode=sys.argv[2]
with tarfile.open(sys.argv[1],'w:gz') as t:
 m=tarfile.TarInfo({'safe':'metadata.env','traversal':'../escape','absolute':'/absolute','symlink':'link','duplicate':'file','control':'bad\nname'}.get(mode,mode))
 if mode=='symlink': m.type=tarfile.SYMTYPE; m.linkname='/etc/passwd'; t.addfile(m)
 else:
  m.size=3; t.addfile(m,io.BytesIO(b'abc'))
  if mode=='duplicate': t.addfile(m,io.BytesIO(b'abc'))
PY
 if [[ $member == safe ]]; then extract_payload; [[ -f $WORK/payload/metadata.env ]];
 elif (extract_payload); then echo 'Unsafe archive accepted'; exit 1; fi
done
# The direct entry point cannot consume script stdin as answers in a detached session.
if [[ $(uname -s) == Linux ]]; then
 setsid bash -c 'cat "$1" | bash' _ "$root/install.sh" > "$tmp/pipe.log" 2>&1 && exit 1
 grep -q 'Interactive terminal required' "$tmp/pipe.log"
 printf '0\n' | python3 "$root/tests/installer/terminal.py" bash -c 'cat "$1" | bash -s -- --invalid' _ "$root/install.sh" > "$tmp/tty.log" && exit 1
 grep -q 'Usage:' "$tmp/tty.log"
else
 printf 'SKIP: pipe/controlling-PTY integration requires Linux\n'
fi
printf 'PASS: release selection/pinning/channel, missing releases/API errors, download failure, signature refusal, exact identity/tag policy, safe extraction and hostile archives\n'
