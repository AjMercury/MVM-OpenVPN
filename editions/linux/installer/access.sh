#!/usr/bin/env bash
access_panel() {
 preflight
 load_config
 [[ $# -le 1 ]] || die 'Usage: mvm access [SERVER_IP_OR_DOMAIN]'
 PANEL_HOST=${1:-$(detect_panel_ip)}
 PANEL_HOST=${PANEL_HOST,,}
 validate_panel_host "$PANEL_HOST" || die 'Invalid server IP or domain'
 PANEL_PORT=${MVM_LISTEN##*:}
 configure_direct_access "$(detect_panel_ip)"
 backup_db >/dev/null || die 'Verified backup failed; access settings unchanged'
 generate_panel_tls "$CONF/tls"
 local staged
 staged=$(mktemp "$CONF/.access-XXXXXXXX")
 sed '/^MVM_LISTEN=/d;/^MVM_ORIGIN=/d;/^MVM_ADDITIONAL_ORIGINS=/d;/^MVM_DEV_HTTP=/d;/^MVM_PROXY_TLS=/d;/^MVM_TLS_CERT=/d;/^MVM_TLS_KEY=/d;/^MVM_TLS_CA_FILE=/d' "$CONF/config.env" > "$staged"
 printf 'MVM_LISTEN=%s:%s\nMVM_ORIGIN=%s\nMVM_ADDITIONAL_ORIGINS=%s\nMVM_DEV_HTTP=false\nMVM_PROXY_TLS=false\nMVM_TLS_CERT=%s\nMVM_TLS_KEY=%s\nMVM_TLS_CA_FILE=%s\n' "$PANEL_BIND" "$PANEL_PORT" "$PANEL_ORIGIN" "$PANEL_ADDITIONAL_ORIGINS" "$PANEL_CERT" "$PANEL_KEY" "$PANEL_CERT" >> "$staged"
 chmod --reference="$CONF/config.env" "$staged"
 chown --reference="$CONF/config.env" "$staged"
 cp -p "$CONF/config.env" "$CONF/config.env.previous"
 mv -f -- "$staged" "$CONF/config.env"
 if ! systemctl restart mvm || ! ready; then
  cp -p "$CONF/config.env.previous" "$CONF/config.env"
  systemctl restart mvm || true
  die 'Access change failed; previous configuration restored'
 fi
 printf 'Panel URL: %s\n' "$PANEL_ORIGIN"
 if [[ -n $PANEL_ADDITIONAL_ORIGINS ]]; then printf 'IP access: %s\n' "$PANEL_ADDITIONAL_ORIGINS"; fi
 printf 'Allow TCP port %s in the host/network firewall if blocked.\n' "$PANEL_PORT"
 print_panel_fingerprint
}
