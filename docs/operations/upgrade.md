> Edition update — 2026-10-19: Use [Linux versioned update/rollback commands](../../editions/linux/README.md) or [Lite container procedures](../../editions/mikrotik/README.md). Only schema-compatible SQLite updates are currently automated. The historical container workflow below does not substitute for mandatory verified backup.

# Upgrade and release

Use versioned artifacts (initial development release 0.1.0); do not rely on `latest`. Record the image digest, dependency manifest/SBOM, test report and configuration with each deployment. Container digests and CI actions should be pinned in your release process after verifying availability; current files use explicit version tags, which are not immutable digests.

Before upgrade: read compatibility changes, confirm a tested backup/key pair and router PKI backup, stop writes, finish/review jobs, stop the service, and snapshot the database. Build and test the new version separately. Unknown database schema fails startup; future migrations must test backward recovery on copies. Recreate only the manager service after validating proxy/certificate compatibility. Check readiness, login, role scopes, explicit router observations and operation history before enabling writes.

Rollback means stopping the new binary, preserving its database for investigation, restoring the old compatible database snapshot and corresponding binary/configuration/key. Do not run an old binary against a migrated database unless its schema compatibility is explicit. An app rollback does not undo RouterOS changes made since the snapshot; reconcile those independently. See rollback runbook.

