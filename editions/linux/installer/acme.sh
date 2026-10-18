#!/usr/bin/env bash
# Original MVM integration. acme.sh remains an independently downloaded dependency.
ACME_REVISION=37d064f7b580055d3f2ddf9a9c960f754f3694d6
ACME_SHA256=fa4842619ddf50f774d79092ae06887a2107c660757283b763bef834dfc150ff
acme_run() {
 sh "$BASE/acme/acme.sh" --home "$BASE/acme" --config-home "$CONF/acme" --cert-home "$CONF/acme/certs" "$@"
}
acme_prepare() {
 local answer package temp
 local -a missing=()
 for package in socat curl openssl python3; do command -v "$package" >/dev/null || missing+=("$package"); done
 if ((${#missing[@]})); then
  printf 'Certificate prerequisites: %s\n' "${missing[*]}"
  read -r -p 'Install these packages with APT? [yes/no]: ' answer
  [[ $answer == yes ]] || die 'Certificate prerequisites declined; panel unchanged'
  apt-get update && apt-get install -y --no-install-recommends "${missing[@]}" || die 'APT failed'
 fi
 install -d -o root -g root -m 700 "$BASE/acme" "$CONF/acme" "$CONF/acme/certs"
 if [[ -f $BASE/acme/acme.sh ]] && printf '%s  %s\n' "$ACME_SHA256" "$BASE/acme/acme.sh" | sha256sum --check --status; then return; fi
 step 'Downloading and verifying the pinned official acme.sh client'
 temp=$(mktemp "$BASE/acme/.download-XXXXXXXX")
 if ! curl --fail --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 15 --max-time 120 --max-filesize 2097152 \
  "https://raw.githubusercontent.com/acmesh-official/acme.sh/$ACME_REVISION/acme.sh" -o "$temp"; then
  rm -f -- "$temp"; die 'ACME client download failed; panel unchanged'
 fi
 if ! printf '%s  %s\n' "$ACME_SHA256" "$temp" | sha256sum --check --status; then
  rm -f -- "$temp"; die 'ACME client checksum mismatch; no downloaded code executed'
 fi
 chmod 700 "$temp"; mv -f -- "$temp" "$BASE/acme/acme.sh"
}
acme_validate_host() {
 validate_panel_host "$1" || die 'Enter a valid domain or public IPv4 address'
 [[ $1 != localhost && $1 == *.* ]] || die 'A public domain or public IPv4 is required'
 if [[ $1 =~ ^[0-9.]+$ ]]; then
  python3 - "$1" <<'PY' || die 'Public certificates require a public IP. For private IPs use HTTP or automatic local HTTPS.'
import ipaddress, sys
raise SystemExit(0 if ipaddress.ip_address(sys.argv[1]).is_global else 1)
PY
 fi
}
acme_timer() {
 cat > /etc/systemd/system/mvm-cert-renew.service <<'UNIT'
[Unit]
Description=Renew MVM TLS certificate
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
UMask=0077
ExecStart=/bin/bash /opt/mvm/current/editions/linux/installer/acme.sh renew
UNIT
 cat > /etc/systemd/system/mvm-cert-renew.timer <<'UNIT'
[Unit]
Description=Check MVM certificate renewal twice daily
[Timer]
OnCalendar=*-*-* 00,12:00:00
RandomizedDelaySec=1h
Persistent=true
[Install]
WantedBy=timers.target
UNIT
 systemctl daemon-reload
 systemctl enable --now mvm-cert-renew.timer
}
acme_deploy() {
 local host=$1 directory="$CONF/acme/deploy-$1"
 local MANAGED_CERT_DEPLOY=true
 load_config --replace-tls
 # Serving copies are immutable per deployment; failed readiness restores the previous config.
 tls_panel "$directory/fullchain.pem" "$directory/key.pem" "https://$host:${MVM_LISTEN##*:}"
 sed -n 's/^MVM_TLS_CERT=//p' "$CONF/config.env" > "$CONF/acme/managed-cert"
}
acme_issue() {
 preflight; load_config --replace-tls
 local host=${1:-} webroot=${2:-} directory
 host=${host,,}
 acme_validate_host "$host"
 local -a challenge=(--standalone --httpport 80) profile=()
 if [[ -n $webroot ]]; then
  [[ $webroot == /* && -d $webroot ]] || die 'Webroot must be an existing absolute directory'
  challenge=(--webroot "$webroot")
 else
  port_available 80 || die 'Port 80 is busy. Choose webroot mode for the existing web server; no service was stopped.'
 fi
 if [[ $host =~ ^[0-9.]+$ ]]; then profile=(--certificate-profile shortlived --days 3); fi
 printf 'The domain must resolve to this server; public TCP port 80 must reach the standalone listener or selected webroot.\n'
 acme_prepare
 step "Requesting a Let's Encrypt certificate for $host (live ACME log follows)"
 acme_run --issue --server letsencrypt --keylength ec-256 -d "$host" "${challenge[@]}" "${profile[@]}" || die 'Certificate issuance failed; existing panel access retained'
 directory="$CONF/acme/deploy-$host"
 install -d -o root -g root -m 700 "$directory"
 acme_run --install-cert --ecc -d "$host" --key-file "$directory/key.pem" --fullchain-file "$directory/fullchain.pem" || die 'Certificate export failed; panel unchanged'
 acme_deploy "$host"
 printf '%s\n' "$host" > "$CONF/acme/managed-host"
 chmod 600 "$CONF/acme/managed-host"
 acme_timer
 printf 'HTTPS ready: https://%s:%s\nAutomatic renewal: mvm-cert-renew.timer\n' "$host" "${MVM_LISTEN##*:}"
}
acme_renew() {
 preflight
 [[ -f $CONF/acme/managed-host ]] || die 'No managed certificate; use mvm ssl first'
 local host directory
 IFS= read -r host < "$CONF/acme/managed-host"
 acme_validate_host "$host"
 directory="$CONF/acme/deploy-$host"
 # Detect explicit HTTP or imported-certificate changes; never re-enable HTTPS behind the user's back.
 load_config --replace-tls
 [[ ${MVM_ORIGIN:-} == "https://$host:${MVM_LISTEN##*:}" && -f $CONF/acme/managed-cert && ${MVM_TLS_CERT:-} == "$(cat "$CONF/acme/managed-cert")" ]] || die 'Access mode changed; automatic certificate deployment skipped'
 acme_prepare
 step 'Checking certificate renewal (live ACME log follows)'
 acme_run --cron --server letsencrypt || die 'ACME renewal failed; current serving certificate retained'
 if ! cmp -s "$directory/fullchain.pem" "$MVM_TLS_CERT" || ! cmp -s "$directory/key.pem" "$MVM_TLS_KEY"; then acme_deploy "$host"; fi
}
ssl_panel() {
 case ${1:-menu} in
 issue) shift; [[ $# -ge 1 && $# -le 2 ]] || die 'Usage: mvm ssl issue DOMAIN_OR_PUBLIC_IP [WEBROOT]'; acme_issue "$@";;
 renew) acme_renew;;
 status) root; systemctl --no-pager --full status mvm-cert-renew.timer; journalctl -u mvm-cert-renew --no-pager -n 20;;
 menu)
  printf '\n  SSL CERTIFICATES\n  1) Issue domain / public IP certificate (Let\047s Encrypt)\n  2) Renew managed certificate\n  3) Renewal status and logs\n  4) Automatic local HTTPS (self-signed)\n  0) Back\n'
  local choice host challenge webroot=
  read -r -p 'Select: ' choice
  case $choice in
   1) read -r -p 'Domain or public IPv4: ' host
      read -r -p 'Challenge [standalone/webroot, default standalone]: ' challenge
      case ${challenge:-standalone} in
       webroot) read -r -p 'Existing web server document root: ' webroot;;
       standalone) :;; *) die 'Choose standalone or webroot';;
      esac
      acme_issue "$host" "$webroot";;
   2) acme_renew;; 3) ssl_panel status;; 4) access_panel;; 0) return;; *) die 'Invalid selection';;
  esac;;
 *) die 'Usage: mvm ssl [issue DOMAIN_OR_PUBLIC_IP [WEBROOT]|renew|status]';;
 esac
}
# systemd entry: separate from the interactive /dev/tty manager.
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
 SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
 . "$SCRIPT_DIR/common.sh"
 . "$SCRIPT_DIR/wizard.sh"
 . "$SCRIPT_DIR/access.sh"
 [[ ${1:-} == renew && $# == 1 ]] || die 'This entry point is for certificate renewal only'
 acme_renew
fi
