#!/usr/bin/env bash
install_panel() {
 preflight
 preview_confirm
 if [[ $(installation_state) != fresh ]]; then
  die 'Existing or incomplete installation detected. No keys or data replaced. Use update or documented repair.'
 fi
 ip route get 1.1.1.1 >/dev/null 2>&1 || die 'No usable network route; configure networking before installation'
 installation_wizard
 install -d -m 755 "$BASE"
 stage_release "$@"
 ensure_service_user
 install -d -m 750 -o root -g mvm "$CONF" "$CONF/tls"
 install -d -m 700 -o mvm -g mvm "$DATA" "$BACKUPS"
 openssl rand -base64 32 > "$CONF/master.key"
 chown root:mvm "$CONF/master.key"; chmod 640 "$CONF/master.key"
 if [[ $DEFER_ADMIN == true ]]; then
  openssl rand -hex 32 > "$CONF/bootstrap.token"
  chown root:mvm "$CONF/bootstrap.token"; chmod 640 "$CONF/bootstrap.token"
 fi
 cat > "$CONF/config.env" <<ENV
MVM_MASTER_KEY_FILE=/etc/mvm/master.key
MVM_BOOTSTRAP_TOKEN_FILE=/etc/mvm/bootstrap.token
MVM_DATABASE=/var/lib/mvm/mvm.db
MVM_LISTEN=$PANEL_BIND:$PANEL_PORT
MVM_ORIGIN=$PANEL_ORIGIN
MVM_DEV_HTTP=$([[ $PANEL_TLS == false ]] && printf true || printf false)
MVM_ALLOW_MUTATIONS=false
ENV
 if [[ $PANEL_TLS == true ]]; then
  install -o root -g mvm -m 640 "$PANEL_CERT" "$CONF/tls/server.crt"
  install -o root -g mvm -m 640 "$PANEL_KEY" "$CONF/tls/server.key"
  printf 'MVM_TLS_CERT=%s\nMVM_TLS_KEY=%s\n' "$CONF/tls/server.crt" "$CONF/tls/server.key" >> "$CONF/config.env"
 fi
 chmod 640 "$CONF/config.env"; chown root:mvm "$CONF/config.env"
 switch_release "$TARGET"
 ctl init "$DATA/mvm.db"
 if [[ $DEFER_ADMIN == false ]]; then
  printf '%s' "$ADMIN_PASSWORD" | ctl bootstrap-admin "$DATA/mvm.db" "$ADMIN_USERNAME" "$PANEL_ZONE" "$PANEL_LANGUAGE"
  unset ADMIN_PASSWORD
 fi
 install -m 644 "$TARGET/editions/linux/systemd/mvm.service" /etc/systemd/system/mvm.service
 cat > /usr/local/bin/mvm <<'WRAPPER'
#!/usr/bin/env bash
exec bash /opt/mvm/current/editions/linux/installer/menu.sh "$@"
WRAPPER
 chmod 755 /usr/local/bin/mvm
 systemctl daemon-reload
 systemctl enable --now mvm
 ready || die 'Service readiness failed; installation retained for repair. See journalctl -u mvm.'
 completion_summary
}
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
 SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
 exec bash "$SCRIPT_DIR/menu.sh" "$@"
fi
#!/usr/bin/env bash
repair_panel() {
 preflight
 preview_confirm
 load_config
 ensure_service_user
 [[ -f $DATA/mvm.db ]] || die 'Repair requires an existing SQLite database. Incomplete initialization needs reviewed manual recovery; no empty database created.'

 install -d -m 755 "$BASE"
 stage_release "$@"
 systemctl stop mvm 2>/dev/null || true
 # Use the staged compatible binary if the management installation was removed.
 local snapshot="$BACKUPS/pre-repair-$(date +%s)-$RANDOM.db"
 runuser -u mvm -- env MVM_MASTER_KEY_FILE="$CONF/master.key" "$TARGET/mvmctl" backup "$DATA/mvm.db" "$snapshot" || die 'Verified repair backup failed'
 runuser -u mvm -- env MVM_MASTER_KEY_FILE="$CONF/master.key" "$TARGET/mvmctl" check "$DATA/mvm.db" || die 'Repair compatibility check failed'
 switch_release "$TARGET"
 install -m 644 "$TARGET/editions/linux/systemd/mvm.service" /etc/systemd/system/mvm.service
 printf '#!/usr/bin/env bash\nexec bash /opt/mvm/current/editions/linux/installer/menu.sh "$@"\n' > /usr/local/bin/mvm
 chmod 755 /usr/local/bin/mvm
 systemctl daemon-reload
 systemctl enable --now mvm
 ready || die 'Repair readiness failed; backup/configuration retained'
}


