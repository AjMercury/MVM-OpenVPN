#!/usr/bin/env bash
# Terminal presentation only; never source the environment file to display status.
ui_menu() {
 local cyan= green= dim= reset= state=STOPPED version=unknown url='Not configured' port='-' protocol='-'
 if [[ -t 1 && ${TERM:-dumb} != dumb && -z ${NO_COLOR:-} ]]; then
  cyan=$'\033[1;36m'; green=$'\033[1;32m'; dim=$'\033[2m'; reset=$'\033[0m'
 fi
 if systemctl is-active --quiet mvm 2>/dev/null; then state=RUNNING; fi
 if [[ -f $BASE/current/metadata.env ]]; then version=$(sed -n 's/^VERSION=//p' "$BASE/current/metadata.env"); fi
 if [[ -f $CONF/config.env ]]; then
  url=$(sed -n 's/^MVM_ORIGIN=//p' "$CONF/config.env")
  port=$(sed -n 's/^MVM_LISTEN=.*://p' "$CONF/config.env")
  protocol=${url%%://*}
 fi
 printf '\n%s+--------------------------------------------------+\n' "$cyan"
 printf '|              M I K R O T I K   V P N             |\n'
 printf '|                 MVM Control Panel                |\n'
 printf '+--------------------------------------------------+%s\n' "$reset"
 printf '  Version  %-14s  Service  %s%s%s\n' "$version" "$green" "$state" "$reset"
 printf '  Port     %-14s  Access   %s\n' "$port" "${protocol^^}"
 printf '  Panel    %s\n' "$url"
 printf '%s----------------------------------------------------%s\n' "$dim" "$reset"
 printf '%s  INSTALLATION%s\n  1) Install / Repair       2) Update Panel\n  3) Uninstall Panel       14) Rollback Version\n' "$cyan" "$reset"
 printf '%s  SERVICE%s\n  4) Start    5) Stop    6) Restart\n  7) Status                 8) Live Logs (Ctrl+C)\n' "$cyan" "$reset"
 printf '%s  ACCESS & SECURITY%s\n  9) Change Port           16) IP / Domain Access\n 10) Import TLS Files      17) SSL Certificates\n 11) Reset Admin           18) Enable HTTP Access\n' "$cyan" "$reset"
 printf '%s  DATA%s\n 12) Backup Database       13) Restore Database\n 15) Panel Information      0) Exit\n' "$cyan" "$reset"
 printf '%s----------------------------------------------------%s\n' "$dim" "$reset"
 printf '%s\n' "$WARNING"
}
