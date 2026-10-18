> Edition update — 2026-10-19: Linux uses SQLite VACUUM INTO plus verified restore rehearsal; use [the Linux guide](../../editions/linux/README.md). The bbolt-specific backup procedure below applies to v0.1.0 and Lite. Migration requires [the offline fail-safe procedure](edition-migration.md). Never copy a live SQLite main file while ignoring WAL.

# Backup and restore

Back up three independent things: encrypted application database, its external encryption key (separate secure custody), and authorized encrypted RouterOS configuration/PKI backups. Include secured application environment, TLS configuration, desired-state records and the exact release artifact. An application backup does not recreate router certificates, rules or users. bbolt supports one process; CLI backup is deliberately offline to avoid a second process opening the live file.

## Backup

1. Stop the service gracefully (Compose: `docker compose stop manager`). Preserve current environment and master key through the secret manager.
2. Source: `go run -tags nomsgpack ./cmd/server backup backups/manager-YYYYMMDD.db`. Create the backup directory first. Destination must be new; overwrite is refused.
3. Container: mount an approved backup directory into a one-shot manager container, with UID 65532 write permission, and run `backup /backup/manager-YYYYMMDD.db`. Example: `docker compose run --rm -v "$PWD/backups:/backup" manager backup /backup/manager-YYYYMMDD.db`.
4. Check a **copy**: `MVM_DATABASE=backups/manager-YYYYMMDD.db go run -tags nomsgpack ./cmd/server check`. Opening verifies schema and the external key; bbolt page checks run as well. Keep original backups immutable.
5. Restart the original service. Archive backups encrypted, access-controlled and offline/off-host. Suggested policy: seven daily and four weekly copies, enforced by your backup system; automatic retention scheduling is not included.

## Restore rehearsal

1. Keep the application stopped and save the original database unchanged.
2. Verify the backup copy with the corresponding master key and the same compatible application version. Refuse unknown schema versions. Never test an old migration in-place on the sole backup.
3. With the original database preserved elsewhere and the destination **absent**, run `go run -tags nomsgpack ./cmd/server restore backups/manager-YYYYMMDD.db`. The CLI opens the backup read-only, checks schema, key and bbolt pages, writes a consistent temporary copy, then publishes through an atomic hard link that refuses to overwrite an existing target. The destination filesystem must support hard links. Restore the required environment and key.
4. Start with `MVM_ALLOW_MUTATIONS=false`. Sessions/delivery tokens in restored backups may be stale; clear sessions/deliveries through an independently reviewed offline maintenance procedure if a compromise is suspected. The current build has no bulk session reset API.
5. Interrupted APPLYING/VERIFYING/ROLLING_BACK jobs are marked MANUAL_INTERVENTION. Sync each authorized router and compare actual configuration with ownership maps. Do not reapply jobs until differences and dependencies are reviewed. The app provides detection, not a complete restore reconciliation engine.
6. Test portal authorization, server identities, active sessions and the lab's allow/deny connectivity checks before enabling writes.

Storage tests exercise backup → restore → reopen → integrity check → decrypt a credential, reject an incorrect key and refuse to overwrite an existing target. Host-loss and real RouterOS restoration remain NOT RUN.

## Recovery scenarios

* **Host loss/corruption:** restore validated app backup and external key on a new secured host; compare against unchanged router state.
* **Router reset/replacement:** preserve old mappings, restore authorized router/PKI backup or explicitly reprovision after manual inventory review. Same IP does not prove same router; check management TLS identity, model and resource observations.
* **Lost REST credentials:** repair through an authorized management console and update records using a reviewed offline process; credential-edit UI is pending.
* **Lost CA key:** existing certificates may remain usable, but new signing cannot be restored from the app database. Create a new CA/server/client chain under an explicit migration plan; don't pretend reissue is automatic.
* **Expired certificate:** fix PKI/clock with authorized maintenance; never disable verification.
* **Old backup/version:** retain both old/new release artifacts and keys, test a copy, reject unsupported schema. No downgrade migration is supplied.

## فارسی

برای بازیابی، هم پایگاه داده رمزگذاری‌شده، هم کلید اصلی جداگانه و هم پشتیبان مجاز روتر و PKI لازم است. ابتدا سرویس را متوقف کنید؛ دستور `backup` فایل جدید می‌سازد و روی فایل قبلی نمی‌نویسد. نسخه کپی را با همان کلید و دستور `check` بررسی کنید. هنگام بازیابی ابتدا پایگاه فعلی را حفظ کنید، نوشتن روی روتر را خاموش نگه دارید و پس از راه‌اندازی وضعیت واقعی روتر را با مالکیت منابع مقایسه کنید. بازیابی پایگاه برنامه به‌تنهایی روتر را بازیابی نمی‌کند.

