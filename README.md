# MikroTik VPN Manager — two-edition development preview

**WARNING: This application is currently a Development Preview. Real RouterOS and VPN client interoperability testing is incomplete. Production deployment is not recommended.**

This is the existing Go/Gin modular monolith, refactored as `0.2.0-dev`. Router writes remain disabled by default. No real router mutation or VPN-client interoperability test has been executed.

| | Linux Edition | MikroTik Container Lite Edition |
|---|---|---|
| Shared application | Gin, HTMX, English/Persian RTL, local Glassmorphism assets, RBAC, audit, OpenVPN workflows | Same domain, application, transport and UI |
| Storage | AES-256-GCM encrypted SQLite records | Existing AES-256-GCM bbolt format |
| Build tags | `nomsgpack` | `nomsgpack,lite` |
| Deployment | Native unprivileged systemd service; Docker optional | RouterOS Container only |
| Packaging | Versioned Linux archives and systemd/Bash management | Separate Dockerfile and reviewed RouterOS templates |

The existing verified-HTTPS RouterOS transport, destination allowlist/rebinding defense, certificate automation, durable provisioning/ownership jobs, IP reservations, encrypted profile generation, single-use delivery and local QR download links remain shared. Mock workflows are evidence of implemented behavior, not live interoperability.

## Repository

```text
cmd/server/                    shared server; edition-selected repository
cmd/mvmctl/                    SQLite operations and explicit offline bbolt migration
internal/storage/              shared contracts and compatible legacy bbolt implementation
  bbolt/                       edition facade
  sqlite/                      encrypted SQLite repository, references, tests
  migration/                   verified backup, rehearsal, atomic no-replace activation
internal/{app,domain,jobs,routers,security,provisioning,ipam,vpn}/
internal/http/                 existing UI/API plus token-protected setup
web/                           local embedded assets, templates, en/fa locales
editions/linux/
  installer/{install,menu,update,uninstall,common}.sh
  systemd/mvm.service
  docker/{Dockerfile,compose.yaml}
  config/                      non-secret configuration examples
editions/mikrotik/
  Dockerfile
  routeros/{install,update,uninstall}.rsc
  config/
scripts/                       archive packaging, binary dependency SBOM generation
 tests/installer/              Bash menu, staging and mocked recovery tests
 tests/systemd/                guarded disposable-Linux lifecycle test (not run locally)
 docs/                         architecture, security, operations, compatibility
.github/workflows/             independent Linux, Lite build-only and systemd jobs
```

## Build and test

Go 1.26+ is required. This Windows workspace includes `.tools/go/bin/go.exe`. No Git metadata is present in this workspace.

```sh
go test -tags nomsgpack ./...
go vet -tags nomsgpack ./...
go build -tags nomsgpack -o server ./cmd/server
go build -tags nomsgpack -o mvmctl ./cmd/mvmctl
bash scripts/package-linux.sh 0.2.0-dev amd64
bash scripts/package-linux.sh 0.2.0-dev arm64
# Lite excludes the SQLite driver from its server dependency graph:
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags nomsgpack,lite ./cmd/server
```

For local HTTP development, explicitly set `MVM_DATABASE` to a new local SQLite path, `MVM_MASTER_KEY` to a securely preserved base64 random 32-byte key, `MVM_DEV_HTTP=true`, `MVM_LISTEN=127.0.0.1:8080`, and `MVM_ORIGIN=http://127.0.0.1:8080`. Prefer `MVM_MASTER_KEY_FILE` for a protected key file. Never regenerate an existing installation's key. Run `server bootstrap` with `MVM_ADMIN_USERNAME` and a securely prompted `MVM_ADMIN_PASSWORD`, or configure a protected 32+ character `MVM_BOOTSTRAP_TOKEN_FILE` and visit `/setup`. No default password is shipped. The native installer uses protected-stdin offline creation by default and offers browser setup only when explicitly deferred.

## Native Linux installation

Read [Linux installation and management](editions/linux/README.md) first. Obtain an archive and its SHA-256 digest from an authenticated independent channel. There is no published official release URL in this preview.

```sh
# From a reviewed source checkout; opens the interactive menu:
sudo bash install.sh
# Explicit local artifact installation:
sudo bash editions/linux/installer/install.sh install /secure/mvm-linux-amd64-0.2.0-dev.tar.gz TRUSTED_SHA256 0.2.0-dev
# After installation:
sudo mvm
sudo mvm status
journalctl -u mvm -f
```

Installation requires explicit Development Preview acceptance. The wizard defaults to loopback HTTP and creates the administrator before starting the service. Use the displayed SSH tunnel to sign in. Only explicitly deferred creation requires `/setup` and the locally read bootstrap token. Configure trusted HTTPS before exposing any remote listener. TLS setup never requests a public certificate merely from an IP address.

## Existing v0.1.0 data

**Do not point a new installation at a legacy database or generate a new key.** Stop/quiesce the old app. Preserve its configuration and external master key. With new destination/backup paths:

```sh
export MVM_MASTER_KEY_FILE=/secure/existing-master.key
./mvmctl migrate-bbolt /secure/original-bbolt.db /secure/new-sqlite.db /secure/consistent-bbolt-backup.db
```

The source opens read-only under a bbolt lock. Migration verifies the key and every encrypted record, uses the original supported transaction backup mechanism, verifies the backup, rehearses the SQLite import privately, compares all records/IDs/counts and foreign keys, checks integrity/decryption, then publishes atomically without overwriting. Original data/key remain untouched. A failed import does not activate an empty database. Existing destination/backup paths cause refusal. See [migration and recovery](docs/operations/edition-migration.md).

## Docker and MikroTik

Linux: [edition Docker configuration](editions/linux/README.md). Root Docker/Compose files remain Linux aliases and now use SQLite; existing root-container data needs explicit migration before use. Lite: [hardware, container and rollback notes](editions/mikrotik/README.md). Only Go cross-builds were executed locally; Docker images and RouterOS runtime are unverified. Lite resource limits remain goals, not guarantees.

## فارسی

این نسخه پیش‌نمایش توسعه است و برای استقرار عملیاتی توصیه نمی‌شود. نسخه لینوکس از SQLite رمزگذاری‌شده و نسخه سبک میکروتیک از bbolt استفاده می‌کند؛ منطق برنامه و رابط فارسی/انگلیسی مشترک است. پیش از مهاجرت سرویس را متوقف کنید، کلید قبلی را حفظ کنید و دستور مهاجرت را فقط روی مسیرهای جدید اجرا کنید. پشتیبان معتبر و بررسی رمزگشایی اجباری است. نصب اولیه فقط روی loopback در دسترس است و ایجاد مدیر به توکن محلی نیاز دارد. حذف معمولی داده‌ها و کلید را نگه می‌دارد و هیچ تنظیم VPN روتر را حذف نمی‌کند. آزمایش واقعی RouterOS و کلاینت VPN هنوز اجرا نشده است.

See [DELIVERY](DELIVERY.md), [verification](docs/verification.md), [edition architecture](docs/architecture/editions.md), [known limitations](docs/limitations.md) and the unchanged [authorized lab procedure](tests/lab/README.md).


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
