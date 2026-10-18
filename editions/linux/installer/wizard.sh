#!/usr/bin/env bash
# Interactive input is never accepted from arguments or environment variables.
validate_port() { [[ $1 =~ ^[1-9][0-9]{3,4}$ && $1 -ge 1024 && $1 -le 65535 ]]; }
port_available() { local listeners; listeners=$(ss -H -ltn "sport = :$1") || die 'Cannot inspect listening ports'; [[ -z $listeners ]]; }
validate_password() { LC_ALL=C; [[ ${#1} -ge 14 && ${#1} -le 233 && $1 == "$2" ]]; }
read_password() {
 set +x
 local confirmation
 while true; do
  IFS= read -r -s -p 'Administrator password (14–233 bytes): ' ADMIN_PASSWORD; printf '\n'
  IFS= read -r -s -p 'Confirm password: ' confirmation; printf '\n'
  if validate_password "$ADMIN_PASSWORD" "$confirmation"; then unset confirmation; return; fi
  printf 'Passwords must match and contain 14–233 bytes. Use a long unique passphrase.\n'
 done
}
validate_tls() {
 local cert=$1 key=$2 host=$3 certpub keypub
 openssl x509 -in "$cert" -noout -checkend 0 >/dev/null || die 'Invalid/expired certificate'
 local sans; sans=$(openssl x509 -in "$cert" -noout -ext subjectAltName) || die 'Missing certificate SAN'
 if [[ $host =~ ^[0-9.]+$ ]]; then
  [[ $sans == *'IP Address:'* ]] || die 'IP SAN required'
  openssl x509 -in "$cert" -noout -checkip "$host" >/dev/null || die 'TLS IP SAN mismatch'
 else
  [[ $sans == *'DNS:'* ]] || die 'DNS SAN required'
  openssl x509 -in "$cert" -noout -checkhost "$host" >/dev/null || die 'TLS hostname mismatch'
 fi
 openssl verify -purpose sslserver -CApath /etc/ssl/certs -untrusted "$cert" "$cert" >/dev/null || die 'Certificate must chain to system trust'
 certpub=$(openssl x509 -in "$cert" -pubkey -noout | openssl sha256)
 keypub=$(openssl pkey -in "$key" -pubout -passin pass: | openssl sha256)
 [[ $certpub == "$keypub" ]] || die 'Certificate/key mismatch'
}
validate_zone() { [[ $1 != /* && $1 != *..* && -f /usr/share/zoneinfo/$1 ]]; }
detect_panel_ip() {
 ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<NF;i++) if($i=="src") {print $(i+1);exit}}' || true
}
validate_panel_host() {
 local host=$1 label
 [[ ${#host} -le 253 && $host =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]] || return 1
 if [[ $host =~ ^[0-9.]+$ ]]; then
  awk -v ip="$host" 'BEGIN {n=split(ip,a,".");if(n!=4) exit 1;for(i=1;i<=4;i++) if(a[i]!~/^[0-9]+$/ || a[i]>255 || (length(a[i])>1 && substr(a[i],1,1)=="0")) exit 1}'
 else
  local -a labels
  IFS=. read -r -a labels <<< "$host"
  [[ $host != *. && $host != *..* ]] || return 1
  for label in "${labels[@]}"; do [[ ${#label} -le 63 && $label != -* && $label != *- ]] || return 1; done
 fi
}
configure_direct_access() {
 local detected_ip=$1
 [[ $PANEL_HOST != localhost && $PANEL_HOST != 127.* && $PANEL_HOST != 0.0.0.0 ]] || die 'Use a server IP or domain for direct access'
 validate_panel_host "$detected_ip" && [[ $detected_ip =~ ^[0-9.]+$ && $detected_ip != 127.* && $detected_ip != 0.0.0.0 ]] || die 'Cannot detect a server IPv4 bind address'
 PANEL_BIND=0.0.0.0
 PANEL_ORIGIN="https://$PANEL_HOST:$PANEL_PORT"
 PANEL_TLS=true
 PANEL_AUTO_TLS=true
 PANEL_ADDITIONAL_ORIGINS=
 local address origin
 while IFS= read -r address; do
  [[ $address =~ ^[0-9.]+$ && $address != 127.* && $address != 0.0.0.0 ]] && validate_panel_host "$address" || continue
  origin="https://$address:$PANEL_PORT"
  if [[ $origin != "$PANEL_ORIGIN" && ,$PANEL_ADDITIONAL_ORIGINS, != *",$origin,"* ]]; then
   PANEL_ADDITIONAL_ORIGINS+="${PANEL_ADDITIONAL_ORIGINS:+,}$origin"
  fi
 done < <(printf '%s\n' "$detected_ip"; ip -4 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 || true)
 printf 'Direct access: %s (automatic HTTPS; trust the displayed certificate fingerprint on first access).\n' "$PANEL_ORIGIN"
}
generate_panel_tls() {
 local directory=$1 suffix san
 suffix=$(openssl rand -hex 12)
 install -d -o root -g mvm -m 750 "$directory"
 PANEL_CERT="$directory/server-$suffix.crt"
 PANEL_KEY="$directory/server-$suffix.key"
 if [[ $PANEL_HOST =~ ^[0-9.]+$ ]]; then san="IP:$PANEL_HOST"; else san="DNS:$PANEL_HOST"; fi
 local alias address
 local -a aliases
 IFS=, read -r -a aliases <<< "$PANEL_ADDITIONAL_ORIGINS"
 for alias in "${aliases[@]}"; do
  [[ -n $alias ]] || continue
  address=${alias#https://}; address=${address%:*}
  san+=",IP:$address"
 done
 step 'Generating HTTPS certificate for this server IP/domain'
 openssl req -x509 -newkey rsa:3072 -sha256 -days 365 -nodes -subj '/CN=MikroTik VPN Manager' \
  -addext "subjectAltName=$san" -addext 'basicConstraints=critical,CA:FALSE' \
  -addext 'keyUsage=critical,digitalSignature,keyEncipherment' -addext 'extendedKeyUsage=serverAuth' \
  -keyout "$PANEL_KEY" -out "$PANEL_CERT" >/dev/null 2>&1 || die 'Cannot generate panel TLS certificate'
 chmod 640 "$PANEL_CERT" "$PANEL_KEY"
 chown root:mvm "$PANEL_CERT" "$PANEL_KEY"
}
configure_http_access() {
 configure_direct_access "$1" >/dev/null
 PANEL_ORIGIN=${PANEL_ORIGIN/https:/http:}
 PANEL_ADDITIONAL_ORIGINS=${PANEL_ADDITIONAL_ORIGINS//https:/http:}
 PANEL_TLS=false
 PANEL_AUTO_TLS=false
 PANEL_HTTP=true
 printf 'Direct HTTP access: %s (unencrypted; no TLS certificate is required).\n' "$PANEL_ORIGIN"
}
print_panel_fingerprint() {
 printf 'Automatic HTTPS certificate: verify this SHA-256 fingerprint before trusting it in your browser:\n'
 openssl x509 -in "$PANEL_CERT" -noout -fingerprint -sha256
 printf 'Certificate file for importing into your browser trust store: %s\n' "$PANEL_CERT"
}
installation_wizard() {
 local answer detected_ip
 detected_ip=$(detect_panel_ip)
 read -r -p 'Panel port [8443]: ' PANEL_PORT; PANEL_PORT=${PANEL_PORT:-8443}
 validate_port "$PANEL_PORT" || die 'Port must be 1024–65535'
 port_available "$PANEL_PORT" || die 'Panel port is already in use'
 read -r -p "Domain or server IPv4 address [$detected_ip]: " PANEL_HOST
 PANEL_HOST=${PANEL_HOST:-$detected_ip}
 PANEL_HOST=${PANEL_HOST,,}
 validate_panel_host "$PANEL_HOST" || die 'Invalid hostname/address'
 DEFER_ADMIN=false
 if [[ $DEFER_ADMIN == false ]]; then
  read -r -p 'Administrator username: ' ADMIN_USERNAME
  [[ $ADMIN_USERNAME =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,26}$ ]] || die 'Invalid username (1–27 characters)'
  read_password
 else ADMIN_USERNAME='Deferred: secure /setup required'; fi
 read -r -p 'Timezone [UTC]: ' PANEL_ZONE; PANEL_ZONE=${PANEL_ZONE:-UTC}
 validate_zone "$PANEL_ZONE" || die 'Invalid timezone'
 read -r -p 'Language English/Persian [en/fa, default en]: ' PANEL_LANGUAGE; PANEL_LANGUAGE=${PANEL_LANGUAGE:-en}
 [[ $PANEL_LANGUAGE == en || $PANEL_LANGUAGE == fa ]] || die 'Language must be en or fa'
 PANEL_BIND=127.0.0.1; PANEL_ORIGIN="http://127.0.0.1:$PANEL_PORT"; PANEL_TLS=false; PANEL_AUTO_TLS=false; PANEL_HTTP=false; PANEL_ADDITIONAL_ORIGINS=
 read -r -p 'Access mode: HTTP (unencrypted), automatic HTTPS, existing TLS or local [http/auto/tls/local, default http]: ' answer
 answer=${answer:-http}
 [[ $answer == http || $answer == auto || $answer == local || $answer == tls ]] || die 'Choose http, auto, tls or local'
 if [[ $answer == http ]]; then configure_http_access "$detected_ip"; fi
 if [[ $answer == auto ]]; then configure_direct_access "$detected_ip"; fi
 if [[ $answer == tls ]]; then
  read -r -p 'Full-chain certificate PEM path: ' PANEL_CERT
  read -r -p 'Private key PEM path: ' PANEL_KEY
  validate_tls "$PANEL_CERT" "$PANEL_KEY" "$PANEL_HOST"
  read -r -p 'Explicit local IPv4 bind address (no wildcard): ' PANEL_BIND
  [[ $PANEL_BIND =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ && $PANEL_BIND != 0.0.0.0 ]] || die 'Explicit IPv4 address required'
  ip -4 -o addr show | awk '{print $4}' | cut -d/ -f1 | grep -Fxq "$PANEL_BIND" || die 'Bind address is not local'
  read -r -p 'Review management firewall rules now; type FIREWALL REVIEWED to confirm restricted access: ' answer
  [[ $answer == 'FIREWALL REVIEWED' ]] || die 'Management firewall review required'
  PANEL_ORIGIN="https://$PANEL_HOST:$PANEL_PORT"; PANEL_TLS=true
 fi
}
completion_summary() {
 local version maturity
 version=$(sed -n 's/^VERSION=//p' "$BASE/current/metadata.env")
 maturity=$(sed -n 's/^MATURITY=//p' "$BASE/current/metadata.env")
 [[ $maturity != DevelopmentPreview ]] || maturity='Development Preview'
 printf '\n=========================================\n       MikroTik VPN Manager\n         Installation Complete\n=========================================\n\nVersion:      %s\nEdition:      Linux\nService:      mvm.service\nStatus:       RUNNING\nDatabase:     SQLite\n\nPanel URL:    %s\nPanel Port:   %s\nUsername:     %s\nPassword:     Configured securely\n\nManagement:   sudo mvm\n\nRelease:      %s\n=========================================\n' "$version" "$PANEL_ORIGIN" "$PANEL_PORT" "$ADMIN_USERNAME" "$maturity"
 if [[ ${PANEL_BIND:-127.0.0.1} == 127.0.0.1 ]]; then
  printf 'Loopback only. On your workstation run:\n  ssh -N -L %s:127.0.0.1:%s <ssh-user>@%s\nThen open %s\n' "$PANEL_PORT" "$PANEL_PORT" "$PANEL_HOST" "$PANEL_ORIGIN"
 else
  printf 'Open %s from another computer. Allow TCP port %s in the host/network firewall if blocked.\n' "$PANEL_ORIGIN" "$PANEL_PORT"
  if [[ -n ${PANEL_ADDITIONAL_ORIGINS:-} ]]; then printf 'IP access:    %s\n' "$PANEL_ADDITIONAL_ORIGINS"; fi
  if [[ ${PANEL_AUTO_TLS:-false} == true ]]; then print_panel_fingerprint; fi
  if [[ ${PANEL_HTTP:-false} == true ]]; then printf 'Optional HTTPS and automatic certificate renewal: sudo mvm ssl (menu 17).\n'; fi
 fi
}
