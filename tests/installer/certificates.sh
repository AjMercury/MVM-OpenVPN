#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
. "$root/editions/linux/installer/wizard.sh"
. "$root/editions/linux/installer/acme.sh"
. "$root/editions/linux/installer/ui.sh"
tmp=$(mktemp -d); trap 'rm -rf -- "$tmp"' EXIT
BASE="$tmp/opt"; CONF="$tmp/etc"; DATA="$tmp/data"
mkdir -p "$BASE/current" "$CONF" "$DATA"
if [[ -n ${MVM_TEST_PYTHON:-} ]]; then python3(){ "$MVM_TEST_PYTHON" "$@"; }; fi
for host in 127.0.0.1 172.16.180.58 192.168.1.1 10.0.0.1 localhost 'panel.test;id'; do
 if (acme_validate_host "$host"); then echo 'Nonpublic ACME host accepted'; exit 1; fi
done
acme_validate_host 8.8.8.8
acme_validate_host panel.example.com
preflight(){ :; }
load_config(){ MVM_LISTEN=0.0.0.0:8444; }
port_available(){ return 0; }
acme_prepare(){ :; }
acme_run(){ printf '%s\n' "$*" >> "$tmp/acme-calls"; }
acme_deploy(){ printf '%s\n' "$1" > "$tmp/deployed"; }
acme_timer(){ printf 'timer enabled\n' > "$tmp/timer"; }
install(){ command install "$@" 2>/dev/null || { [[ $* == *'-d'* ]] && mkdir -p "${@: -1}"; }; }
acme_issue 8.8.8.8
grep -q -- '--certificate-profile shortlived --days 3' "$tmp/acme-calls"
grep -q -- '--standalone --httpport 80' "$tmp/acme-calls"
grep -qx '8.8.8.8' "$CONF/acme/managed-host"
[[ -f $tmp/timer ]]
mkdir -p "$tmp/webroot"
port_available(){ return 1; }
acme_issue panel.example.com "$tmp/webroot"
grep -q -- "--webroot $tmp/webroot" "$tmp/acme-calls"
cp "$CONF/acme/managed-host" "$tmp/known-host"
if (acme_issue panel.example.com); then echo 'Busy standalone port accepted'; exit 1; fi
acme_run(){ return 1; }
if (acme_issue another.example.com "$tmp/webroot"); then echo 'ACME failure accepted'; exit 1; fi
cmp "$tmp/known-host" "$CONF/acme/managed-host"
grep -qx panel.example.com "$tmp/deployed"
(
 . "$root/editions/linux/installer/acme.sh"
 load_config(){ MVM_LISTEN=0.0.0.0:8444; MVM_ORIGIN=https://panel.example.com:8444; MVM_TLS_CERT="$CONF/tls/active.crt"; MVM_TLS_KEY="$CONF/tls/active.key"; }
 mkdir -p "$CONF/tls" "$CONF/acme/deploy-panel.example.com"
 printf cert > "$CONF/tls/active.crt"; printf key > "$CONF/tls/active.key"
 cp "$CONF/tls/active.crt" "$CONF/acme/deploy-panel.example.com/fullchain.pem"
 cp "$CONF/tls/active.key" "$CONF/acme/deploy-panel.example.com/key.pem"
 printf '%s\n' "$CONF/tls/active.crt" > "$CONF/acme/managed-cert"
 acme_prepare(){ :; }; acme_run(){ :; }
 acme_deploy(){ touch "$tmp/renewed"; }
 acme_renew
 [[ ! -f $tmp/renewed ]]
 printf renewed > "$CONF/acme/deploy-panel.example.com/fullchain.pem"
 acme_renew
 [[ -f $tmp/renewed ]]
 rm "$tmp/renewed"
 load_config(){ MVM_LISTEN=0.0.0.0:8444; MVM_ORIGIN=http://panel.example.com:8444; }
 if (acme_renew); then echo 'Renewal overwrote explicit HTTP'; exit 1; fi
 [[ ! -f $tmp/renewed ]]
)
(
 . "$root/editions/linux/installer/access.sh"
 load_config(){ MVM_LISTEN=0.0.0.0:8444; }
 validate_tls(){ :; }; backup_db(){ :; }; systemctl(){ :; }; ready(){ return 1; }
 openssl(){ if [[ $1 == rand ]]; then printf testsuffix; else printf publickey; fi; }
 install(){ local target=${@: -1}; if [[ $1 == -d ]]; then mkdir -p "$target"; else cp "${@: -2:1}" "$target"; fi; }
 printf 'MVM_LISTEN=0.0.0.0:8444\nMVM_ORIGIN=http://panel.example.com:8444\nMVM_ALLOW_HTTP=true\n' > "$CONF/config.env"
 cp "$CONF/config.env" "$tmp/before-tls"
 if (tls_panel "$CONF/tls/active.crt" "$CONF/tls/active.key" https://panel.example.com:8444); then echo 'Failed TLS deployment accepted'; exit 1; fi
 cmp "$tmp/before-tls" "$CONF/config.env"
)
# Production preparation rejects changed upstream content before invoking it.
(
 . "$root/editions/linux/installer/acme.sh"
 install(){ mkdir -p "${@: -1}"; }
 mkdir -p "$BASE/acme" "$CONF/acme" "$CONF/acme/certs"
 command(){ if [[ $1 == -v ]]; then return 0; else builtin command "$@"; fi; }
 curl(){ touch "$tmp/download-attempted"; printf 'untrusted bytes' > "${@: -1}"; }
 if (acme_prepare); then echo 'Unverified ACME source accepted'; exit 1; fi
 [[ ! -f $BASE/acme/acme.sh ]]
 [[ -f $tmp/download-attempted ]]
)
printf 'VERSION=1.0.3\n' > "$BASE/current/metadata.env"
printf 'MVM_LISTEN=0.0.0.0:8444\nMVM_ORIGIN=http://192.0.2.45:8444\nMVM_MASTER_KEY=SECRET_NEVER_SHOW\n' > "$CONF/config.env"
systemctl(){ return 1; }
WARNING='RouterOS automation remains experimental.'
output=$(NO_COLOR=1 ui_menu)
[[ $output == *'1.0.3'* && $output == *'STOPPED'* && $output == *'8444'* && $output == *'http://192.0.2.45:8444'* && $output == *'17) SSL Certificates'* ]]
[[ $output != *SECRET_NEVER_SHOW* && $output != *$'\033'* ]]
printf 'PASS: ACME private-IP rejection, short-lived public-IP profile, webroot, failure preservation, pinned download, plain menu status/redaction\n'
