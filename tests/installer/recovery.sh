#!/usr/bin/env bash
# Fault-injection tests. Service and backup execution are mocked; no host service/data is modified.
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
. "$root/editions/linux/installer/update.sh"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
BASE="$tmp/opt"
CONF="$tmp/etc"
DATA="$tmp/data"
BACKUPS="$tmp/backups"
mkdir -p "$BASE/releases/1" "$CONF" "$DATA" "$BACKUPS"
printf 'SCHEMA=1\n' > "$BASE/releases/1/metadata.env"
printf '#!/usr/bin/env bash\nexit 0\n' > "$BASE/releases/1/mvmctl"
chmod +x "$BASE/releases/1/mvmctl"
printf '%s\n' "$BASE/releases/1" > "$BASE/active"
active_release(){ [[ -f $BASE/active ]]; }
switch_release(){ printf '%s\n' "$1" > "$BASE/active"; }
readlink(){ if [[ ${2:-} == "$BASE/current" ]]; then cat "$BASE/active"; else command readlink "$@"; fi; }
printf 'original database\n' > "$DATA/mvm.db"
printf 'preserved key\n' > "$CONF/master.key"
preflight(){ :; }
load_config(){ :; }
preview_confirm(){ :; }
install_service_unit(){ printf '%s\n' "$1" >> "$tmp/unit-updates"; }
stage_release(){
 TARGET="$BASE/releases/$3"
 mkdir -p "$TARGET"
 printf 'SCHEMA=1\n' > "$TARGET/metadata.env"
 printf '#!/usr/bin/env bash\nexit 0\n' > "$TARGET/mvmctl"
 chmod +x "$TARGET/mvmctl"
}
runuser(){ shift 3; "$@"; }
systemctl(){ printf '%s\n' "$*" >> "$tmp/service-calls"; }
ready(){ [[ $(basename "$(readlink -f "$BASE/current")") != broken ]]; }
backup_db(){
 local backup="$BACKUPS/pre-update-$RANDOM.db"
 cp "$DATA/mvm.db" "$backup"
 printf '%s\n' "$backup"
}
update_panel ignored ignored 2
[[ $(readlink -f "$BASE/current") == "$BASE/releases/2" ]]
cmp "$DATA/mvm.db" "$(tail -n 1 "$BASE/recovery")"
rollback_panel
[[ $(readlink -f "$BASE/current") == "$BASE/releases/1" ]]
printf 'VERSION=1.0.0\nSCHEMA=1\n' > "$BASE/releases/1/metadata.env"
MVM_TLS_CA_FILE=/example/automatic.crt
if (rollback_panel); then echo 'Incompatible automatic TLS rollback accepted'; exit 1; fi
[[ $(readlink -f "$BASE/current") == "$BASE/releases/1" ]]
MVM_TLS_CA_FILE=
if ( update_panel ignored ignored broken ); then printf 'Failed update reported success\n' >&2; exit 1; fi
[[ $(readlink -f "$BASE/current") == "$BASE/releases/1" ]]
grep -qx 'preserved key' "$CONF/master.key"
grep -qx 'original database' "$DATA/mvm.db"
# Backup failure must stop before activating the target.
backup_db(){ return 1; }
if ( update_panel ignored ignored 3 ); then exit 1; fi
[[ $(readlink -f "$BASE/current") == "$BASE/releases/1" ]]
printf 'PASS: mocked update success, failed readiness recovery, backup failure, rollback, key/data preservation.\n'

