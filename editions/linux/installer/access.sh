#!/usr/bin/env bash
access_panel() {
 preflight
 load_config --replace-tls
 local http=false
 if [[ ${1:-} == --http ]]; then http=true; shift; fi
 [[ $# -le 1 ]] || die 'Usage: mvm access [--http] [SERVER_IP_OR_DOMAIN]'
 PANEL_HOST=${1:-$(detect_panel_ip)}
 PANEL_HOST=${PANEL_HOST,,}
 validate_panel_host "$PANEL_HOST" || die 'Invalid server IP or domain'
 PANEL_PORT=${MVM_LISTEN##*:}
 PANEL_HTTP=false
 if [[ $http == true ]]; then configure_http_access "$(detect_panel_ip)"; else configure_direct_access "$(detect_panel_ip)"; fi
 backup_db '' --replace-tls >/dev/null || die 'Verified backup failed; access settings unchanged'
 if [[ $http != true ]]; then generate_panel_tls "$CONF/tls"; fi
 local staged
 staged=$(mktemp "$CONF/.access-XXXXXXXX")
 sed '/^MVM_LISTEN=/d;/^MVM_ORIGIN=/d;/^MVM_ADDITIONAL_ORIGINS=/d;/^MVM_DEV_HTTP=/d;/^MVM_ALLOW_HTTP=/d;/^MVM_PROXY_TLS=/d;/^MVM_TLS_CERT=/d;/^MVM_TLS_KEY=/d;/^MVM_TLS_CA_FILE=/d' "$CONF/config.env" > "$staged"
 printf 'MVM_LISTEN=%s:%s\nMVM_ORIGIN=%s\nMVM_ADDITIONAL_ORIGINS=%s\nMVM_DEV_HTTP=false\nMVM_PROXY_TLS=false\nMVM_ALLOW_HTTP=%s\n' "$PANEL_BIND" "$PANEL_PORT" "$PANEL_ORIGIN" "$PANEL_ADDITIONAL_ORIGINS" "$PANEL_HTTP" >> "$staged"
 if [[ $http != true ]]; then printf 'MVM_TLS_CERT=%s\nMVM_TLS_KEY=%s\nMVM_TLS_CA_FILE=%s\n' "$PANEL_CERT" "$PANEL_KEY" "$PANEL_CERT" >> "$staged"; fi
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
 if [[ $http != true ]]; then print_panel_fingerprint; fi
}

# Shared by interactive import and the protected ACME deployment hook.
tls_panel() {
  load_config --replace-tls
  [[ $# == 3 && $3 =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || die 'Usage: mvm tls CERT_PEM KEY_PEM HTTPS_ORIGIN'
  openssl x509 -in "$1" -noout -checkend 0 >/dev/null || die 'Expired or invalid certificate'
  local certpub keypub
  certpub=$(openssl x509 -in "$1" -pubkey -noout | openssl sha256)
  keypub=$(openssl pkey -in "$2" -pubout -passin pass: | openssl sha256)
  [[ $certpub == "$keypub" ]] || die 'Certificate/key mismatch'
  local tls_host=${3#https://}; tls_host=${tls_host%%:*}
  [[ $3 == "https://$tls_host:${MVM_LISTEN##*:}" ]] || die 'TLS origin must include current listener port'
  validate_tls "$1" "$2" "$tls_host"
  local tls_bind=${MVM_LISTEN%:*}
  if [[ $tls_bind == 127.0.0.1 ]]; then
   tls_bind=$(detect_panel_ip)
   [[ -n $tls_bind ]] || die 'Cannot detect a server IP for TLS'
  fi
  backup_db '' --replace-tls >/dev/null
  local suffix; suffix=$(openssl rand -hex 12)
  install -d -o root -g mvm -m 750 "$CONF/tls"
  install -o root -g mvm -m 640 "$1" "$CONF/tls/server-$suffix.crt"
  install -o root -g mvm -m 640 "$2" "$CONF/tls/server-$suffix.key"
  cp -p "$CONF/config.env" "$CONF/config.env.previous"
  sed -i '/^MVM_TLS_CERT=/d;/^MVM_TLS_KEY=/d;/^MVM_TLS_CA_FILE=/d;/^MVM_ORIGIN=/d;/^MVM_ADDITIONAL_ORIGINS=/d;/^MVM_DEV_HTTP=/d;/^MVM_ALLOW_HTTP=/d;/^MVM_PROXY_TLS=/d' "$CONF/config.env"
  sed -i "s|^MVM_LISTEN=.*|MVM_LISTEN=$tls_bind:${MVM_LISTEN##*:}|" "$CONF/config.env"
  printf 'MVM_TLS_CERT=%s\nMVM_TLS_KEY=%s\nMVM_ORIGIN=%s\nMVM_DEV_HTTP=false\nMVM_ALLOW_HTTP=false\nMVM_PROXY_TLS=false\n' "$CONF/tls/server-$suffix.crt" "$CONF/tls/server-$suffix.key" "$3" >> "$CONF/config.env"
  if ! systemctl restart mvm || ! ready; then cp -p "$CONF/config.env.previous" "$CONF/config.env"; systemctl restart mvm; die 'TLS readiness failed; previous configuration restored'; fi
}
