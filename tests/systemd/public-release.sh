#!/usr/bin/env bash
# EMPTY disposable supported systemd host only. Installs from real public GitHub URLs.
set +x
set -Eeuo pipefail
umask 077
[[ ${MVM_DISPOSABLE_LINUX:-} == YES && $EUID == 0 && -d /run/systemd/system ]] || exit 1
for path in /opt/mvm /etc/mvm /var/lib/mvm /var/backups/mvm; do [[ ! -e $path ]] || exit 1; done
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
testdir=$(mktemp -d)
trap 'rm -rf -- "$testdir"' EXIT
openssl rand -base64 24 > "$testdir/password"
# Python keeps the password out of process arguments, environment and logs.
python3 - "$testdir/password" <<'PY' | python3 "$root/tests/installer/terminal.py" bash -o pipefail -c \
 'curl -fsSL https://raw.githubusercontent.com/AjMercury/MvM-OpenVPN/master/install.sh | sudo bash -s -- --channel preview' > "$testdir/output"
import pathlib,sys
p=pathlib.Path(sys.argv[1]).read_text().strip()
print(f'INSTALL PREVIEW\nINSTALL PREVIEW\n8443\nlocalhost\nrelease-owner\n{p}\n{p}\nUTC\nen\nlocal')
PY
grep -q 'Installation Complete' "$testdir/output"
grep -q 'Status:       RUNNING' "$testdir/output"
if grep -Fq -f "$testdir/password" "$testdir/output"; then echo 'Credential leaked'; exit 1; fi
python3 - "$testdir/password" <<'PY'
import json,pathlib,sys,urllib.request,urllib.error
password=pathlib.Path(sys.argv[1]).read_text().strip()
request=urllib.request.Request('http://127.0.0.1:8443/login',data=json.dumps({'username':'release-owner','password':password}).encode(),headers={'Origin':'http://127.0.0.1:8443','Content-Type':'application/json'})
with urllib.request.urlopen(request,timeout=10) as response:
    assert response.status==200 and 'mvm_session=' in str(response.headers), 'Normal login failed'
try: urllib.request.urlopen('http://127.0.0.1:8443/setup')
except urllib.error.HTTPError as exc: assert exc.code==404
else: raise SystemExit('Browser setup remained enabled')
PY
systemctl is-active --quiet mvm
runuser -u mvm -- env MVM_MASTER_KEY_FILE=/etc/mvm/master.key /opt/mvm/current/mvmctl check /var/lib/mvm/mvm.db
version=$(sed -n 's/^VERSION=//p' /opt/mvm/current/metadata.env)
curl -fsSL "https://github.com/AjMercury/MvM-OpenVPN/releases/download/v$version/SHA256SUMS" -o "$testdir/SHA256SUMS"
curl -fsSL "https://github.com/AjMercury/MvM-OpenVPN/releases/download/v$version/SHA256SUMS.sigstore.json" -o "$testdir/bundle.json"
verify_live_manifest() {
 gh attestation verify "$testdir/SHA256SUMS" --bundle "$testdir/bundle.json" \
  --repo AjMercury/MVM-OpenVPN --source-ref "refs/tags/v$version" --deny-self-hosted-runners \
  --cert-identity "https://github.com/AjMercury/MVM-OpenVPN/.github/workflows/release.yaml@refs/tags/v$version" > "$testdir/verification" 2>&1
}
verify_live_manifest
printf 'tampered\n' >> "$testdir/SHA256SUMS"
if verify_live_manifest; then echo 'Tampered signed manifest accepted'; exit 1; fi
printf 'PASS: real published signature accepted; cryptographically mismatched manifest rejected\n'
printf 'PASS: anonymous authenticated public release, raw piped bootstrap, controlling terminal, hidden first credentials, SQLite, systemd readiness and normal login\n'
