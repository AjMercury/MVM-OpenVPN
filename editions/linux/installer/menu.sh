#!/usr/bin/env bash
set -Eeuo pipefail
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
. "$SCRIPT_DIR/common.sh"
. "$SCRIPT_DIR/wizard.sh"
. "$SCRIPT_DIR/install.sh"
. "$SCRIPT_DIR/update.sh"
. "$SCRIPT_DIR/uninstall.sh"
# Real entry points require a terminal. Source-based unit tests exercise functions separately.
if ! (: </dev/tty) 2>/dev/null; then die 'Interactive terminal required (/dev/tty); no unattended mode is offered.'; fi
exec 0</dev/tty
command_run() {
 local command=${1:-}; shift || true
 case $command in
 install|repair|update)
  if [[ $# == 0 ]]; then
   bash "$SCRIPT_DIR/../../../install.sh" --channel preview --action "$command"
  elif [[ $command == install ]]; then install_panel "$@";
  elif [[ $command == repair ]]; then repair_panel "$@";
  else update_panel "$@"; fi;;
 rollback) rollback_panel;;
 uninstall) uninstall_panel "$@";;
 start|stop|restart|status) root; systemctl "$command" mvm;;
 logs) root; journalctl -u mvm -f;;
 backup) preflight; backup_db "${1:-}";;
 restore)
  preflight; load_config
  [[ $# == 1 ]] || die 'Usage: mvm restore BACKUP'
  systemctl stop mvm
  backup_db >/dev/null || { systemctl start mvm; die 'Pre-restore backup failed'; }
  local staged="$DATA/restore-$RANDOM.db" saved="$DATA/preserved-$(date +%s)-$RANDOM.db"
  ctl restore "$staged" "$1" || { systemctl start mvm; die 'Restore verification failed'; }
  move_database "$DATA/mvm.db" "$saved"
  move_database "$staged" "$DATA/mvm.db"
  if ! systemctl start mvm || ! ready; then systemctl stop mvm; move_database "$DATA/mvm.db" "$DATA/failed-$RANDOM.db"; move_database "$saved" "$DATA/mvm.db"; systemctl start mvm; die 'Restore failed; original restored'; fi;;
 port)
  preflight; load_config
  [[ $# == 1 ]] && validate_port "$1" || die 'Usage: mvm port PORT (1024..65535)'
  port_available "$1" || die 'Panel port is already in use'
  backup_db >/dev/null
    local scheme=http host address
  if [[ ${MVM_DEV_HTTP:-} != true ]]; then scheme=https; fi
  host=${MVM_ORIGIN#*://}; host=${host%%:*}
  [[ $host =~ ^[A-Za-z0-9.-]+$ ]] || die 'IPv6/custom origin requires reviewed manual configuration'
  address=${MVM_LISTEN%:*}
  cp -p "$CONF/config.env" "$CONF/config.env.previous"
  sed -i "s|^MVM_LISTEN=.*|MVM_LISTEN=$address:$1|;s|^MVM_ORIGIN=.*|MVM_ORIGIN=$scheme://$host:$1|" "$CONF/config.env"
  if ! systemctl restart mvm || ! ready; then cp -p "$CONF/config.env.previous" "$CONF/config.env"; systemctl restart mvm; die 'Port change failed; previous configuration restored'; fi;;
  tls)
  preflight; load_config
  [[ $# == 3 && $3 =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || die 'Usage: mvm tls CERT_PEM KEY_PEM HTTPS_ORIGIN'
  openssl x509 -in "$1" -noout -checkend 0 >/dev/null || die 'Expired or invalid certificate'
  local certpub keypub
  certpub=$(openssl x509 -in "$1" -pubkey -noout | openssl sha256)
  keypub=$(openssl pkey -in "$2" -pubout -passin pass: | openssl sha256)
  [[ $certpub == "$keypub" ]] || die 'Certificate/key mismatch'
  local tls_host=${3#https://}; tls_host=${tls_host%%:*}
  [[ $3 == "https://$tls_host:${MVM_LISTEN##*:}" ]] || die 'TLS origin must include current listener port'
  validate_tls "$1" "$2" "$tls_host"
  backup_db >/dev/null
  local suffix; suffix=$(openssl rand -hex 12)
  install -o root -g mvm -m 640 "$1" "$CONF/tls/server-$suffix.crt"
  install -o root -g mvm -m 640 "$2" "$CONF/tls/server-$suffix.key"
  cp -p "$CONF/config.env" "$CONF/config.env.previous"
  sed -i '/^MVM_TLS_CERT=/d;/^MVM_TLS_KEY=/d;/^MVM_ORIGIN=/d;/^MVM_DEV_HTTP=/d' "$CONF/config.env"
  printf 'MVM_TLS_CERT=%s\nMVM_TLS_KEY=%s\nMVM_ORIGIN=%s\nMVM_DEV_HTTP=false\n' "$CONF/tls/server-$suffix.crt" "$CONF/tls/server-$suffix.key" "$3" >> "$CONF/config.env"
  if ! systemctl restart mvm || ! ready; then cp -p "$CONF/config.env.previous" "$CONF/config.env"; systemctl restart mvm; die 'TLS readiness failed; previous configuration restored'; fi;;
 reset-admin)
  preflight; load_config
  [[ $# == 1 ]] || die 'Usage: mvm reset-admin USERNAME (password read securely)'
  read_password
  local password=$ADMIN_PASSWORD; unset ADMIN_PASSWORD
  systemctl stop mvm
  backup_db >/dev/null || { systemctl start mvm; die 'Verified backup failed'; }
  if ! printf '%s' "$password" | ctl reset-admin "$DATA/mvm.db" "$1"; then unset password; systemctl start mvm; die 'Credential reset failed'; fi
  unset password
  systemctl start mvm; ready || die 'Service failed readiness';;
 version|info) printf '%s\n' "$WARNING"; printf 'Installation state: %s\n' "$(installation_state)"; [[ ! -f $BASE/current/metadata.env ]] || cat "$BASE/current/metadata.env";;
 *) die 'Commands: install/update ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 VERSION; start stop restart status logs; backup [PATH]; restore BACKUP; rollback; uninstall [--purge]; port PORT; version';;
 esac
}
if [[ $# -gt 0 ]]; then command_run "$@"; exit; fi
while true; do
 printf '\n========================================\n MikroTik VPN Manager\n Installation & Management\n Development Preview\n========================================\n%s\n' "$WARNING"
 printf '1. Install / Repair\n2. Update Panel\n3. Uninstall Panel\n4. Start Service\n5. Stop Service\n6. Restart Service\n7. Service Status\n8. View Logs\n9. Change Panel Port\n10. Configure TLS Certificate\n11. Reset Admin Credentials\n12. Backup Database\n13. Restore Database\n14. Rollback Version\n15. Panel Information\n0. Exit\n'
 read -r -p 'Select an option: ' choice || exit
 case $choice in
 0) exit;;
 1) if [[ $(installation_state) == fresh ]]; then command_run install; else command_run repair; fi;;
 2) command_run update;;
 3) command_run uninstall;; 4) command_run start;;5) command_run stop;;6) command_run restart;;7) command_run status;;8) command_run logs;;
 9) read -r -p 'Panel port (existing bind address retained): ' port; command_run port "$port";;
  10) read -r -p 'Certificate PEM path: ' cert; read -r -p 'Key PEM path: ' key; read -r -p 'HTTPS origin (trusted SAN, current port): ' origin; command_run tls "$cert" "$key" "$origin";;
 11) read -r -p 'Existing administrator username: ' username; command_run reset-admin "$username";;
 12) command_run backup;;13) read -r -p 'Backup path: ' path; command_run restore "$path";;14) command_run rollback;;15) command_run info;;
 *) printf 'Invalid selection\n';;
 esac
done






