#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
. "$root/editions/linux/installer/wizard.sh"
. "$root/editions/linux/installer/access.sh"
tmp=$(mktemp -d); trap 'rm -rf -- "$tmp"' EXIT
CONF="$tmp/etc";mkdir -p "$CONF"
printf 'MVM_LISTEN=127.0.0.1:8443\nMVM_ORIGIN=http://127.0.0.1:8443\nMVM_DEV_HTTP=true\nMVM_MASTER_KEY_FILE=/original/key\n' > "$CONF/config.env"
preflight(){ :; }
load_config(){ MVM_LISTEN=127.0.0.1:8443; }
backup_db(){ cp "$CONF/config.env" "$tmp/backup"; }
detect_panel_ip(){ printf '172.16.180.58\n'; }
systemctl(){ :; }
chown(){ :; }
generate_panel_tls(){ PANEL_CERT="$CONF/test.crt"; PANEL_KEY="$CONF/test.key"; }
print_panel_fingerprint(){ :; }
ready(){ return 0; }
access_panel panel.example.test
grep -qx 'MVM_LISTEN=0.0.0.0:8443' "$CONF/config.env"
grep -qx 'MVM_ORIGIN=https://panel.example.test:8443' "$CONF/config.env"
grep -qx 'MVM_ADDITIONAL_ORIGINS=https://172.16.180.58:8443' "$CONF/config.env"
grep -qx 'MVM_MASTER_KEY_FILE=/original/key' "$CONF/config.env"
cp "$CONF/config.env" "$tmp/known-good"
ready(){ return 1; }
if (access_panel 172.16.180.58); then exit 1; fi
cmp "$tmp/known-good" "$CONF/config.env"
for host in '999.1.1.1' 'panel.test;id' localhost 127.0.0.1; do
 if (access_panel "$host");then echo 'Invalid direct host accepted';exit 1;fi
done
printf 'PASS: direct IP/domain access, secondary IP origin, master-key preservation and rollback on readiness failure\n'
