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
release_tag=${MVM_TEST_RELEASE_TAG:-}
release_channel=stable
if [[ $release_tag == *-* || -z $release_tag ]]; then release_channel=preview; fi
# Python keeps the password out of process arguments, environment and logs.
python3 - "$testdir/password" "$release_channel" "${MVM_TEST_DIRECT_HTTPS:-no}" <<'PY' | python3 "$root/tests/installer/terminal.py" bash -o pipefail -c \
 'curl -fsSL https://raw.githubusercontent.com/AjMercury/MvM-OpenVPN/master/install.sh | sudo bash -s -- --channel "$1" ${2:+--version "$2"}' _ "$release_channel" "$release_tag" > "$testdir/output"
import pathlib,sys
p=pathlib.Path(sys.argv[1]).read_text().strip()
if sys.argv[2]=='preview': print('INSTALL PREVIEW')
if sys.argv[3]=='YES': print(f'8443\n\nrelease-owner\n{p}\n{p}\nUTC\nen\n')
else: print(f'8443\nlocalhost\nrelease-owner\n{p}\n{p}\nUTC\nen\nlocal')
PY
grep -q 'Installation Complete' "$testdir/output"
grep -q 'Status:       RUNNING' "$testdir/output"
grep -q '\[MVM\] Checking operating system and architecture' "$testdir/output"
if [[ ${MVM_TEST_DIRECT_HTTPS:-no} == YES ]]; then grep -q '\[MVM\] Initializing the encrypted SQLite database' "$testdir/output"; fi
if grep -Fq -f "$testdir/password" "$testdir/output"; then echo 'Credential leaked'; exit 1; fi
if [[ ${MVM_TEST_DIRECT_HTTPS:-no} == YES ]]; then
 panel_ip=$(ip -4 route get 1.1.1.1 | awk '{for(i=1;i<NF;i++)if($i=="src"){print $(i+1);exit}}')
 python3 "$root/tests/systemd/direct-access.py" "$panel_ip" release-owner < "$testdir/password"
else
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
fi
systemctl is-active --quiet mvm
runuser -u mvm -- env MVM_MASTER_KEY_FILE=/etc/mvm/master.key /opt/mvm/current/mvmctl check /var/lib/mvm/mvm.db
version=$(sed -n 's/^VERSION=//p' /opt/mvm/current/metadata.env)
curl -fsSL "https://github.com/AjMercury/MvM-OpenVPN/releases/download/v$version/SHA256SUMS" -o "$testdir/SHA256SUMS"
curl -fsSL "https://github.com/AjMercury/MvM-OpenVPN/releases/download/v$version/SHA256SUMS.sigstore.json" -o "$testdir/SHA256SUMS.sigstore.json"
verify_live_manifest() {
 ( . "$root/install.sh"; WORK="$testdir" TAG="v$version"; verify_manifest ) > "$testdir/verification" 2>&1
}
verify_live_manifest
printf 'tampered\n' >> "$testdir/SHA256SUMS"
if verify_live_manifest; then echo 'Tampered signed manifest accepted'; exit 1; fi
printf 'PASS: real published signature accepted; cryptographically mismatched manifest rejected\n'
printf 'PASS: anonymous authenticated public release, raw piped bootstrap, controlling terminal, hidden first credentials, SQLite, systemd readiness and normal login\n'
