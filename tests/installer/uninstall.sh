#!/usr/bin/env bash
# Authorization/path tests only: removal and service operations are intercepted.
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
. "$root/editions/linux/installer/uninstall.sh"
tmp=$(mktemp -d)
trap 'command rm -rf -- "$tmp"' EXIT
preflight(){ :; }
systemctl(){ printf 'service %s\n' "$*" >> "$tmp/actions"; }
rm(){ printf 'remove %s\n' "$*" >> "$tmp/actions"; }
if (uninstall_panel <<< 'NO');then exit 1;fi
[[ ! -f $tmp/actions ]]
uninstall_panel <<< 'REMOVE'
grep -q '/opt/mvm' "$tmp/actions"
! grep -q '/var/lib/mvm\|/var/backups/mvm\|/etc/mvm' "$tmp/actions"
: > "$tmp/actions"
if (uninstall_panel --purge <<< $'REMOVE\nNO');then exit 1;fi
! grep -q '/var/lib/mvm\|/var/backups/mvm\|/etc/mvm' "$tmp/actions"
: > "$tmp/actions"
uninstall_panel --purge <<< $'REMOVE\nDELETE ALL MVM DATA'
grep -q '/var/lib/mvm' "$tmp/actions"
# The workflow only invokes service/local file operations; no network/router operation exists.
printf 'PASS: mocked standard uninstall, cancellation, separate purge confirmation and removal path scope.\n'
