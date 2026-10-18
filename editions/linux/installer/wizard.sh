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
installation_wizard() {
 local answer
 read -r -p 'Panel port [8443]: ' PANEL_PORT; PANEL_PORT=${PANEL_PORT:-8443}
 validate_port "$PANEL_PORT" || die 'Port must be 1024–65535'
 port_available "$PANEL_PORT" || die 'Panel port is already in use'
 read -r -p 'Hostname or management IPv4 address: ' PANEL_HOST
 [[ $PANEL_HOST =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]] || die 'Invalid hostname/address'
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
 PANEL_BIND=127.0.0.1; PANEL_ORIGIN="http://127.0.0.1:$PANEL_PORT"; PANEL_TLS=false
 read -r -p 'Access mode: secure local-only or trusted TLS [local/tls, default local]: ' answer
 [[ ${answer:-local} == local || $answer == tls ]] || die 'Choose local or tls'
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
 if [[ $PANEL_TLS == false ]]; then
  printf 'Loopback only. On your workstation run:\n  ssh -N -L %s:127.0.0.1:%s <ssh-user>@%s\nThen open %s\n' "$PANEL_PORT" "$PANEL_PORT" "$PANEL_HOST" "$PANEL_ORIGIN"
 fi
}
