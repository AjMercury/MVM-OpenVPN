#!/usr/bin/env bash
update_panel() {
 preflight
 load_config
 preview_confirm
 active_release || die 'Incomplete installation; no active release'
 local old backup
 old=$(readlink -f "$BASE/current")

  if [[ $# == 0 ]]; then
  [[ -f $CONF/release.env ]] || die 'No release source configured. Supply ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 VERSION.'
  . "$CONF/release.env"
  set -- "${MVM_RELEASE_SOURCE:?}" "${MVM_RELEASE_SHA256:?}" "${MVM_RELEASE_VERSION:?}"
 fi
 printf 'Current: %s\nTarget: %s\n' "$(basename "$old")" "$3"
 stage_release "$@"
 # Both old and new binaries must declare the same supported schema. Unknown schema aborts.
 grep -qx 'SCHEMA=1' "$old/metadata.env" || die 'Old schema metadata unsupported'
 systemctl stop mvm
 if ! backup=$(backup_db); then systemctl start mvm; die 'Verified backup failed; update aborted'; fi
 if ! runuser -u mvm -- env MVM_MASTER_KEY_FILE="$CONF/master.key" "$TARGET/mvmctl" check "$DATA/mvm.db"; then
  systemctl start mvm; die 'Target database compatibility check failed'
 fi
 printf '%s\n%s\n' "$old" "$backup" > "$BASE/recovery"
 switch_release "$TARGET"
 if systemctl start mvm && ready; then printf 'Update ready: %s\n' "$TARGET"; else
  systemctl stop mvm || true
  switch_release "$old"
  systemctl start mvm && ready || die 'Update failed and old service failed readiness; recovery record preserved'
  die 'Update failed; previous schema-compatible release restored'
 fi
}
rollback_panel() {
 preflight
 load_config
 [[ -f $BASE/recovery ]] || die 'No known-good recovery record'
 mapfile -t recovery < "$BASE/recovery"
 local old=${recovery[0]} snapshot=${recovery[1]}
 [[ $old == "$BASE/releases/"* && -d $old && $snapshot == "$BACKUPS/"* ]] || die 'Invalid recovery record'
 # Current migration contract is schema 1 only. Never downgrade across unknown schemas.
 grep -qx 'SCHEMA=1' "$old/metadata.env" || die 'Incompatible rollback schema'
 systemctl stop mvm
 backup_db >/dev/null || { systemctl start mvm; die 'Pre-rollback backup failed'; }
 if ! runuser -u mvm -- env MVM_MASTER_KEY_FILE="$CONF/master.key" "$old/mvmctl" check "$DATA/mvm.db"; then
  die 'Schema mismatch. Service left stopped. Restore the verified pre-update snapshot using the recovery runbook.'
 fi
 switch_release "$old"
 systemctl start mvm
 ready || die 'Rollback readiness failed'
 printf 'Rollback ready: %s\n' "$old"
}



