#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
BASE="$tmp/opt";CONF="$tmp/etc";DATA="$tmp/data"
mkdir -p "$BASE" "$CONF" "$DATA"
[[ $(installation_state) == fresh ]]
printf key > "$CONF/master.key"
[[ $(installation_state) == incomplete ]]
printf config > "$CONF/config.env";printf db > "$DATA/mvm.db"
[[ $(installation_state) == preserved-data ]]
if (ID=unsupported;VERSION_ID=1;validate_os);then exit 1;fi
if (ID=ubuntu;VERSION_ID=25.10;validate_os);then exit 1;fi
(ID=ubuntu;VERSION_ID=24.04;validate_os)
(ID=debian;VERSION_ID=13;validate_os)
if (validate_arch mips);then exit 1;fi
validate_arch aarch64;[[ $ARCH == arm64 ]]
validate_arch x86_64;[[ $ARCH == amd64 ]]
df(){ printf 'Filesystem 1024-blocks Used Available Capacity Mounted\nfixture 100 99 1 99%% /var\n'; }
if (check_space);then exit 1;fi
# Preview cannot be accepted implicitly or with an unrelated yes value.
if (MVM_ACCEPT_PREVIEW=;preview_confirm </dev/null);then exit 1;fi
(MVM_ACCEPT_PREVIEW='I ACCEPT DEVELOPMENT PREVIEW';preview_confirm)
printf main > "$tmp/live.db"; printf wal > "$tmp/live.db-wal"; printf shm > "$tmp/live.db-shm"
move_database "$tmp/live.db" "$tmp/saved.db"
[[ ! -e $tmp/live.db && ! -e $tmp/live.db-wal && ! -e $tmp/live.db-shm ]]
grep -qx wal "$tmp/saved.db-wal"
move_database "$tmp/saved.db" "$tmp/live.db"
if (id(){ printf '0\n'; }; ensure_service_user);then exit 1;fi
printf 'PASS: fresh/incomplete/preserved state, OS/architecture, disk and explicit preview confirmation checks.\n'


