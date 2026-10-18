#!/usr/bin/env bash
set -Eeuo pipefail
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
. "$SCRIPT_DIR/common.sh"
. "$SCRIPT_DIR/wizard.sh"
. "$SCRIPT_DIR/install.sh"
. "$SCRIPT_DIR/update.sh"
. "$SCRIPT_DIR/uninstall.sh"
. "$SCRIPT_DIR/access.sh"
. "$SCRIPT_DIR/acme.sh"
. "$SCRIPT_DIR/ui.sh"
# Real entry points require a terminal. Source-based unit tests exercise functions separately.
if ! (: </dev/tty) 2>/dev/null; then die 'Interactive terminal required (/dev/tty); no unattended mode is offered.'; fi
exec 0</dev/tty
command_run() {
 local command=${1:-}; shift || true
 case $command in
 install|repair|update)
  if [[ $# == 0 ]]; then
   bash "$SCRIPT_DIR/../../../install.sh" --channel "$RELEASE_CHANNEL" --action "$command"
  elif [[ $command == install ]]; then install_panel "$@";
  elif [[ $command == repair ]]; then repair_panel "$@";
  else update_panel "$@"; fi;;
 rollback) rollback_panel;;
 uninstall) uninstall_panel "$@";;
 start|stop|restart) root; systemctl "$command" mvm;;
 status) root; systemctl --no-pager --full status mvm;;
 access) access_panel "$@";;
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
  scheme=${MVM_ORIGIN%%://*}
  host=${MVM_ORIGIN#*://}; host=${host%%:*}
  [[ $host =~ ^[A-Za-z0-9.-]+$ ]] || die 'IPv6/custom origin requires reviewed manual configuration'
  address=${MVM_LISTEN%:*}
  cp -p "$CONF/config.env" "$CONF/config.env.previous"
  sed -i "s|^MVM_LISTEN=.*|MVM_LISTEN=$address:$1|;s|^MVM_ORIGIN=.*|MVM_ORIGIN=$scheme://$host:$1|" "$CONF/config.env"
  local old_port=${MVM_LISTEN##*:} alias aliases=
  local -a origin_aliases
  IFS=, read -r -a origin_aliases <<< "${MVM_ADDITIONAL_ORIGINS:-}"
  for alias in "${origin_aliases[@]}"; do
   [[ -n $alias ]] || continue
   if [[ $alias == *":$old_port" ]]; then alias="${alias%:*}:$1"; fi
   aliases+="${aliases:+,}$alias"
  done
  sed -i '/^MVM_ADDITIONAL_ORIGINS=/d' "$CONF/config.env"
  printf 'MVM_ADDITIONAL_ORIGINS=%s\n' "$aliases" >> "$CONF/config.env"
  if ! systemctl restart mvm || ! ready; then cp -p "$CONF/config.env.previous" "$CONF/config.env"; systemctl restart mvm; die 'Port change failed; previous configuration restored'; fi;;
 tls) preflight; tls_panel "$@";;
 ssl) ssl_panel "$@";;
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
trap 'printf "\n"' INT
# A child shell retains errexit while failures return to the interactive menu.
command_run() {
 local result=0
 bash "$SCRIPT_DIR/menu.sh" "$@" || result=$?
 if ((result != 0)); then printf 'Operation ended (status %s). See the message above.\n' "$result"; fi
}
while true; do
 ui_menu
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
 16) read -r -p 'Server IP or domain (blank: detect): ' panel_host
     read -r -p 'Protocol [http/auto, default http]: ' mode
     if [[ ${mode:-http} == http ]]; then
      if [[ -n $panel_host ]]; then command_run access --http "$panel_host"; else command_run access --http; fi
     elif [[ $mode == auto ]]; then
      if [[ -n $panel_host ]]; then command_run access "$panel_host"; else command_run access; fi
     else printf 'Choose http or auto\n'; fi;;
 17) command_run ssl;;
 18) command_run access --http;;
 *) printf 'Invalid selection\n';;
 esac
done






