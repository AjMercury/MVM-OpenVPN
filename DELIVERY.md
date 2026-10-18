# Current delivery — authenticated Linux preview installer

Published to `master`; [v0.2.0-preview.3](https://github.com/AjMercury/MVM-OpenVPN/releases/tag/v0.2.0-preview.3) contains real amd64/arm64 archives, metadata, SBOMs, checksum manifest and its GitHub OIDC/Sigstore attestation. [Release run 39170730251](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39170730251) passed both builds, Go/installer tests, actual Ubuntu 24.04 amd64 native install/same-version repair/update/rollback/uninstall, and anonymous public `curl | sudo bash` installation/login with real tamper rejection. Use the README's explicit preview command; no stable Release exists. Credentials are always created in the Bash wizard via hidden `/dev/tty` input. See [trust/configuration, full results and remaining gates](docs/operations/releases.md).

The following describes the earlier source-only delivery. Its statements about unavailable releases and unexecuted systemd tests are historical and superseded by the results above. RouterOS/VPN interoperability, other Linux runtimes, actual missing-dependency APT and remote TLS/firewall testing remain outstanding.

# Historical delivery — 0.2.0-dev two-edition Development Preview

**WARNING: This application is currently a Development Preview. Real RouterOS and VPN client interoperability testing is incomplete. Production deployment is not recommended.**

## 1. Actual structure and changed files

The existing Go/Gin/HTMX application remains shared. `editions/linux/` contains native installer/menu/update/uninstall/common scripts, systemd unit, Docker/Compose and config/docs. `editions/mikrotik/` contains its separate Dockerfile, validated-input RouterOS script templates and config/docs. Server build selection is in `cmd/server/storage_{full,lite}.go`; offline operations live in `cmd/mvmctl`. SQLite and bbolt share repository interfaces. See the [actual tree](README.md) and [complete task file inventory](docs/refactor-files.md). No Git metadata was present.

## 2. Database migration status

Linux now selects encrypted SQLite; Lite selects compatible bbolt. SQLite includes transactional schema initialization, foreign keys, unique account/user/job/server/allocation projections, indexes, busy timeout on each driver connection, serialized updates, WAL/FULL durability, encrypted persisted domain/jobs/session/delivery values, integrity checking and verified snapshot backup/restore. The selected maintained CGO-free SQLite/WASM driver and CGO/modernc tradeoffs are documented in [architecture](docs/architecture/editions.md).

Offline `mvmctl migrate-bbolt SOURCE NEW_SQLITE NEW_BACKUP` validates the original key/read-only source, mandates the existing transaction-backed backup mechanism and verifies that backup, rehearses import, compares complete records/stable IDs/counts and references, checks integrity/decryption and publishes without replacing anything. Wrong key, invalid relations, unsafe repeats and interrupted rehearsals fail without activating a destination; original database and key remain intact. Tests use encrypted v0.1.0-format fixtures. **No actual user database migration was performed.** Follow the [migration runbook](docs/operations/edition-migration.md) before any deployment activation.

## 3. Linux installation and interactive commands

From the reviewed repository, `sudo bash install.sh` opens the management menu. Explicit installation uses `sudo bash install.sh install ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 VERSION`. After installation, use `sudo mvm` or documented subcommands. The exact warning and explicit consent are mandatory; no default password is generated or shipped. The installer creates the first administrator offline by default; intentionally deferred creation retains protected browser setup. Existing data/key/config cause fresh install refusal; `repair` operates on verified existing SQLite data. The [Linux guide](editions/linux/README.md) documents paths, commands, release checksum trust, TLS, settings, credential recovery and Docker.

## 4. Service verification

`mvm.service` implements unprivileged execution, network ordering, graceful SIGTERM, restart, protected filesystem/private temp, restrictive umask and resource limits. start/stop/restart/status/enable/journal commands are wired to systemd. **Actual systemd unit verification/lifecycle tests are NOT RUN locally**: this host is Windows and WSL access is denied. A guarded empty-disposable-host test and manual CI workflow are implemented; neither is reported passed.

## 5. Update, rollback and uninstall verification

Versioned local/HTTPS archives require independently trusted SHA-256 and compatible architecture/schema metadata. Update stages first, stops service, creates/decrypt-verifies a consistent snapshot, checks target compatibility, switches atomically and checks readiness. Failure switches back to the schema-compatible known-good release. Config/key/TLS and old releases are retained. Configured root-owned release selection is supported; no official endpoint or public version feed is invented. Only schema 1 is shipped; incompatible automatic schema upgrades/downgrades are refused and controlled verified-snapshot recovery is documented.

Mocked shell tests pass for update success, readiness failure, mandatory-backup failure and rollback, preserving data/key. Archive/checksum/download/state/OS/architecture/disk/consent tests pass. Standard uninstall requires confirmation and preserves data/key/config/TLS/backups; purge requires a separate irreversible confirmation. Mocked uninstall path/consent tests pass. No uninstall command calls RouterOS. Native lifecycle and full host-changing TLS/reset/repair/restore/purge tests remain unexecuted.

## 6. Browser setup and preserved features

The setup flow includes secure administrator creation, origin/token authorization, bounded authentication work, stored timezone/language, database/TLS status and post-login optional router registration. It closes after initialization, including concurrent setup protection through the existing accounts lock. English/Persian setup rendering and authorization/closure tests pass. About/menu report Development Preview. Existing local Glassmorphism assets and other pages remain intact. Timestamps continue to render UTC even though setup preferences are stored. New browser visual QA is not claimed.

The existing mock OpenVPN vertical slice now runs successfully against **both bbolt and SQLite**. Existing RouterOS TLS/allowlist, RBAC/audit, IP allocation, job/ownership, certificate automation, secure delivery and QR/profile tests pass. This does not establish live VPN interoperability.

## 7. Build artifacts and Lite status

Linux amd64/arm64 versioned archives, SHA-256 files, build metadata and binary dependency SPDX SBOMs are generated locally under `artifacts/editions/`. Lite Linux amd64/arm64 server cross-builds pass and exclude SQLite/Wazero from their server dependency graph. Lite binary SBOMs are generated. See the final artifact manifest for byte counts/hashes. No RouterOS OCI tar/image, ARMv5 support, compressed-image size or RAM/CPU measurements are claimed. Docker is unavailable. Linux release packaging is independent of Lite image optimization and device compatibility. Separate manual workflows are supplied; hosted CI was not executed.

## 8. Executed tests

- `go test -tags nomsgpack -count=1 ./...`: PASS, including shared contracts, SQLite initialization/transactions/concurrency/encryption/constraints/backup/restore/crash recovery/unknown schema and migration/CLI/setup cases.
- `go test -tags 'nomsgpack lite' ./...`: PASS (the Linux CLI/SQLite tests also remain in the repository; Lite server build itself excludes SQLite).
- `go vet -tags nomsgpack ./...`: PASS.
- `go run scripts/check-manifests.go`: PASS for all supplied YAML definitions; syntax only.
- Git Bash installer menu/preflight/archive/recovery/uninstall suites: PASS; host-changing service/removal operations are mocked.
- Native Linux and Lite amd64/arm64 Go cross-builds: PASS. SBOM generation: PASS; license fields are NOASSERTION, not a license audit.
- Race detector: UNAVAILABLE (`-race requires cgo`), not passed.
- systemd/Docker/RouterOS/real VPN clients: NOT RUN. No real-router mutation tests were executed.

Windows sandbox initially blocked local test sockets/hard links and Git Bash runtime. Those checks were rerun with authorized execution outside that sandbox and passed; these restrictions were not treated as implementation successes. modernc module downloads returned 403; the selected ncruces module and dependencies were fetched with normal Go checksum verification. No TLS verification bypass was added.

## 9. Remaining acceptance criteria and risks

This delivery is implemented code plus local/mocked evidence, **not full deployment acceptance or production readiness**. Required next evidence is actual disposable Debian/Ubuntu systemd installation/service/update/rollback/uninstall/repair/TLS/restore/purge execution, Docker image health/security checks, exact-model RouterOS Container import/runtime testing and real RouterOS/OpenVPN client positive/negative connectivity/offboarding tests. Publish official authenticated release distribution only after those gates pass; no live release endpoint is configured here. Race/security/vulnerability scans, external review, performance and Lite resource measurements remain outstanding. Cross-schema automated upgrades are not offered; schema incompatibility stops instead of forcing a downgrade.

Previous security/lifecycle caveats (MFA, full PKI/CRL proof, semantic firewall reconciliation, complete lifecycle UI and manual-intervention resolution) remain visible in [limitations](docs/limitations.md). Database rollback cannot undo router changes. Preserve external keys, router PKI backups and an independently verified recovery path.

## Installer wizard and first credentials (2026-10-19)

Run from the reviewed repository or extracted verified native release:

```bash
sudo bash install.sh install /absolute/path/mvm-linux-amd64-0.2.0-dev.tar.gz <INDEPENDENTLY_TRUSTED_SHA256> 0.2.0-dev
```

Use arm64 on supported ARM systems. Build new archives from this source before distributing these changes; existing archives have not been replaced. No official release endpoint or signing identity is configured. Automatic latest-release discovery is intentionally unavailable until authenticated official metadata and an independently trusted artifact-verification policy exist. Never substitute an untrusted adjacent checksum.

The wizard collects port (default 8443, range 1024–65535, conflict check), hostname/management IPv4 address, username, hidden password and confirmation, timezone, English/Persian, and local-only or trusted TLS access. Use a unique passphrase of 14–233 bytes. Passwords are not arguments, environment variables, configuration values, logs, or summary output. No default credentials exist. `mvmctl bootstrap-admin DB USERNAME TIMEZONE en|fa` reads the exact password from protected stdin with no terminating newline. The installer calls it as the unprivileged service user after SQLite initialization, using the existing master-key file. Argon2id hashing, account-existence guard, encrypted preferences and Super Administrator account creation commit in one transaction. Any existing account causes refusal. Browser setup returns 404 once an account exists, so even a leftover setup token is unusable. Installer bootstrap creates no token.

Choosing deferred administrator creation retains token-protected `/setup`; read `/etc/mvm/bootstrap.token` locally as root and access setup over the tunnel or trusted TLS. No password summary is displayed for deferred creation. Deferred preferences are chosen again in browser setup.

Without TLS, the actual listener and origin are `127.0.0.1:PORT` and `http://127.0.0.1:PORT`. The completion summary reports the verified local URL and an SSH forwarding command. TLS installation validates system trust, expiry, hostname/IP SAN and key pairing, requires an explicit assigned IPv4 bind address and explicit management-firewall review. It does not change firewall rules. Readiness connects to that bind address with the configured hostname for TLS verification, bypassing proxies without disabling certificate checks. Remote reachability is not implied. Supply a full-chain PEM; custom trust roots must already be installed in system trust. IPv6 wizard binds are currently unsupported.

Ubuntu LTS 22.04+ and Debian 12+, amd64/arm64, root permission, systemd, disk space and a usable network route are required. Missing runtime packages trigger a package list and explicit confirmation before `apt-get update` and targeted installation. No full-system upgrade, Go SDK, Docker or external SQLite server is required. A local release needs no Internet download when prerequisites are present; HTTPS releases and APT fail explicitly on unavailable networks.

`sudo mvm` opens the management menu. Existing install/update/uninstall/start/stop/restart/status/logs/port/tls/reset-admin/backup/restore/rollback/version operations remain implemented. Port and TLS configuration changes now require verified database backups; credential resets use hidden input and confirmation. Fresh-install retries refuse existing or incomplete persistent state. Use reviewed repair/update procedures; no database or master key is regenerated on a retry. Failed readiness leaves files available for repair.

Validation: installer Bash syntax and mocked installer suites passed, including wizard confirmation/redaction, port bounds/conflicts, loopback summary, prerequisite refusal, repeated staging, failed-update recovery, backups, rollback and uninstall preservation. Go suites `./cmd/mvmctl ./internal/storage/sqlite ./internal/http ./internal/security` passed outside the Windows sandbox (the sandbox denied existing backup/restore hard links). Added transactional bootstrap and offline command tests cover password length, preferences, existing-account protection and retries; existing HTTP tests cover setup-token authorization and closure. Actual systemd lifecycle and live trusted-TLS/remote firewall tests were not executed: no accessible disposable Ubuntu/Debian systemd environment is available. The disposable VM test now supplies random first-run credentials via stdin. Development Preview remains explicit; production acceptance and RouterOS/VPN interoperability remain outstanding.

Changed source files for this task: `editions/linux/installer/{common,install,menu,wizard}.sh`, `cmd/mvmctl/{main.go,main_test.go}`, `internal/storage/sqlite/{bootstrap.go,bootstrap_test.go}`, `tests/installer/{run,wizard}.sh`, `tests/systemd/disposable-vm.sh`, `README.md`, `DELIVERY.md`, `editions/linux/README.md`, `docs/operations/installation.md`.

Final build verification: `go build` succeeded for both `cmd/server` (nomsgpack) and `cmd/mvmctl` targeting Linux amd64 and arm64. Build-check outputs are under `.tools/test-*`; distributable release archives were not regenerated. Final wizard tests also passed after extending prerequisite detection to base runtime utilities.
