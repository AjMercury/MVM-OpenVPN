# Offline migration from encrypted v0.1.0 bbolt

Development Preview; real RouterOS/client tests remain incomplete. This procedure changes local storage only and never contacts RouterOS.

1. Stop the old application and disable any automatic restart/container start. Preserve its exact binary/configuration, database, TLS files and existing external master key. Never generate a replacement key. Retain router PKI backups under separately authorized custody.
2. Export the **original** base64 32-byte key into a protected file outside the databases; set `MVM_MASTER_KEY_FILE` to that file without printing its contents.
3. Choose absent SQLite and bbolt backup paths, with enough local disk for a full backup, imported database, snapshot and restore verification copy. The destination filesystem must support atomic hard links. Do not use network filesystems.
4. Run the new Linux tool:

   ```sh
   MVM_MASTER_KEY_FILE=/secure/original-master.key ./mvmctl migrate-bbolt \
     /secure/original-bbolt.db /secure/new-sqlite.db /secure/verified-bbolt-backup.db
   ```

5. The implementation takes a read-only bbolt lock (a running exclusive writer makes it fail), validates schema/key/page structure and every encrypted record, calls the original transaction-backed backup mechanism, syncs and independently verifies the backup before import. It never rewrites the source.
6. A private SQLite staging directory is the migration rehearsal. Import preserves every collection, stable key/identifier and JSON value, including unknown metadata collections; parent entities precede children. SQLite uniqueness and referential constraints reject inconsistent existing data. Counts and every record value are compared and all payloads decrypted, then integrity and foreign-key checks run. Ciphertext is re-encrypted with fresh nonces under the unchanged key and associated-data identifiers; byte-for-byte ciphertext identity is not expected.
7. A supported SQLite snapshot is independently restored/checked privately, closed/checkpointed, and published via an atomic hard link to the absent destination. No empty destination is published on a pre-activation error. Source and verified backup remain available. Interrupted import can leave a private staging directory after process death; inspect/remove only the confirmed staging path. Existing output or backup paths are refused on retry; use new paths, never overwrite the original.
8. With the service stopped, configure the Linux Edition's database to the validated SQLite file (normally install it with mode 0600 and owner `mvm:mvm` at `/var/lib/mvm/mvm.db`). Preserve original configuration/key and the old bbolt file separately. Check `mvmctl check` using the original key, start with router writes disabled, inspect accounts/jobs/ownership and reconcile before any authorized router operation. This activation of service configuration is explicitly administrative; migration does not rewrite environment files or start a service.
9. Rollback before new application changes: stop SQLite service, retain the SQLite file/sidecars and use the original compatible bbolt binary/configuration/database/key pair. After new operations, database rollback alone cannot undo router changes; reconcile independently. Never point the old binary at SQLite.

Verification uses generated encrypted fixtures in the original v0.1.0 schema, including accounts, router credentials, jobs, servers/users, sessions, delivery profiles and unknown metadata. Wrong-key, unsafe-repeat, orphan-reference and simulated interrupted-rehearsal cases are tested. No real user's deployment database has been opened or migrated during this task. An actual deployment migration rehearsal remains the operator's required pre-activation step.
