#!/usr/bin/env bash
# Destructive application lifecycle test for an EMPTY DISPOSABLE Debian/Ubuntu systemd VM only.
set -Eeuo pipefail
[[ ${MVM_DISPOSABLE_LINUX:-} == YES && $EUID == 0 && -d /run/systemd/system ]] || { echo 'Requires explicit disposable Linux VM authorization'; exit 1; }
[[ ! -e /opt/mvm && ! -e /etc/mvm && ! -e /var/lib/mvm && ! -e /var/backups/mvm ]] || { echo 'Existing installation/data found; refusing test'; exit 1; }
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"
bash scripts/package-linux.sh 0.2.0-test1 amd64
archive="$root/artifacts/editions/mvm-linux-amd64-0.2.0-test1.tar.gz"
digest=$(sha256sum "$archive" | cut -d ' ' -f 1)
export MVM_ACCEPT_PREVIEW='I ACCEPT DEVELOPMENT PREVIEW'
# Random test credential stays on stdin and is never printed.
secret=$(openssl rand -base64 24)
printf '8443\nlocalhost\ny\nvm-owner\n%s\n%s\nUTC\nen\nlocal\n' "$secret" "$secret" | bash editions/linux/installer/install.sh install "$archive" "$digest" 0.2.0-test1
unset secret
systemd-analyze verify /etc/systemd/system/mvm.service
systemctl is-enabled --quiet mvm
systemctl stop mvm
! systemctl is-active --quiet mvm
systemctl start mvm
systemctl restart mvm
systemctl is-active --quiet mvm
before=$(sha256sum /etc/mvm/master.key)
if mvm install "$archive" "$digest" 0.2.0-test1; then echo 'Repeated installation accepted'; exit 1; fi
mvm backup
bash scripts/package-linux.sh 0.2.0-test2 amd64
archive2="$root/artifacts/editions/mvm-linux-amd64-0.2.0-test2.tar.gz"
digest2=$(sha256sum "$archive2" | cut -d ' ' -f 1)
mvm update "$archive2" "$digest2" 0.2.0-test2
mvm rollback
[[ $before == "$(sha256sum /etc/mvm/master.key)" ]]
printf 'REMOVE\n' | mvm uninstall
[[ -f /etc/mvm/master.key && -f /var/lib/mvm/mvm.db && -d /var/backups/mvm && ! -e /etc/systemd/system/mvm.service ]]
printf 'PASS: actual native install/start/stop/restart/enable/update/rollback/standard uninstall on disposable systemd host.\n'
