#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
. "$root/editions/linux/installer/wizard.sh"
for port in 1024 8443 65535; do validate_port "$port"; done
for port in 0 1023 65536 008443 abc '8443;id'; do if validate_port "$port"; then exit 1; fi; done
validate_password 'a long secure passphrase' 'a long secure passphrase'
if validate_password short short; then exit 1; fi
if validate_password 'a long secure passphrase' 'a different passphrase'; then exit 1; fi
ss(){ printf 'LISTEN 0 4096 127.0.0.1:8443\n'; }
if port_available 8443; then exit 1; fi
ss(){ :; }; port_available 8443
# The wizard keeps HTTP local, consumes hidden password input, and redacts it in output.
validate_zone(){ [[ $1 == UTC ]]; }
output=$(installation_wizard <<'INPUT'
8443
panel.example.test
owner
unique long secret passphrase
unique long secret passphrase
UTC
fa
local
INPUT
printf '%s %s %s' "$PANEL_BIND" "$PANEL_ORIGIN" "$ADMIN_USERNAME")
[[ $output == *'127.0.0.1 http://127.0.0.1:8443 owner'* ]]
[[ $output != *'unique long secret passphrase'* ]]
BASE=$(mktemp -d); trap 'rm -rf -- "$BASE"' EXIT
mkdir -p "$BASE/current"; printf 'VERSION=0.2.0-dev\nMATURITY=DevelopmentPreview\n' > "$BASE/current/metadata.env"
PANEL_ORIGIN=http://127.0.0.1:8443; PANEL_PORT=8443; ADMIN_USERNAME=owner; DEFER_ADMIN=false; PANEL_TLS=false; PANEL_HOST=panel.example.test
output=$(completion_summary)
[[ $output == *'Status:       RUNNING'* && $output == *'Version:      0.2.0-dev'* && $output == *'ssh -N -L 8443:127.0.0.1:8443'* && $output == *'Configured securely'* ]]
# No package mutation after refusal.
command(){ return 1; }
apt-get(){ printf 'MUST NOT EXECUTE'; exit 99; }
if (ensure_prerequisites <<<no); then exit 1; fi
printf 'PASS: wizard input, confirmation, redaction, port conflicts, loopback, completion and prerequisite refusal\n'
# Simulated APT approval installs a bounded package set, never an upgrade.
(
 command(){ [[ $2 != ss ]]; }
 apt-get(){ printf '%s\n' "$*" >> "$BASE/apt-calls"; }
 ensure_prerequisites <<<yes
 grep -qx update "$BASE/apt-calls"
 grep -q '^install -y --no-install-recommends iproute2' "$BASE/apt-calls"
 ! grep -q upgrade "$BASE/apt-calls"
)
# Missing SAN and failed trust checks must reject TLS before any service changes.
if (openssl(){ if [[ $* == *subjectAltName* ]]; then printf 'CN only'; fi; }; validate_tls cert key panel.example.test); then exit 1; fi
if (openssl(){ if [[ $* == *subjectAltName* ]]; then printf 'DNS:panel.example.test'; elif [[ $1 == verify ]]; then return 1; fi; }; validate_tls cert key panel.example.test); then exit 1; fi
printf 'PASS: approved targeted prerequisites and TLS SAN/trust refusal\n'
