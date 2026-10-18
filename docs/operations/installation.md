> Edition update — 2026-10-19: Current Linux SQLite/native installation, menu, TLS and Docker instructions are in [the Linux Edition guide](../../editions/linux/README.md). MikroTik bbolt/container instructions are in [the Lite guide](../../editions/mikrotik/README.md). The bootstrap/container examples below describe the legacy preview workflow; bbolt statements apply to Lite. Never run a new Linux binary on old data without [explicit migration](edition-migration.md).

# Installation

## Prerequisites

Linux host, Go 1.26+ for source builds or Docker Engine/Compose for containers, a secured management path to RouterOS, a trusted application TLS certificate, and an independently bootstrapped RouterOS HTTPS REST service. Plan initially around 2 vCPU, 1 GB RAM and 10 GB SSD; these are **unmeasured planning assumptions**. Runtime application data lives in one local persistent directory. Never run multiple instances against one database.

Configure:

| Variable | Meaning |
|---|---|
| MVM_MASTER_KEY | required random 32 bytes, standard base64; preserve separately |
| MVM_DATABASE | data/manager.db by default; /data/manager.db in image |
| MVM_ORIGIN | exact trusted origin, e.g. https://vpn-admin.example:8443 |
| MVM_LISTEN | loopback default, 0.0.0.0:8080 in private container network |
| MVM_TLS_CERT / MVM_TLS_KEY | optional native TLS PEM files |
| MVM_ROUTER_CIDRS | comma-separated approved management hosts/networks; empty denies connections |
| MVM_ALLOW_MUTATIONS | false default; true only with authorization and reviewed Apply |
| MVM_ROUTER_TIMEOUT_SECONDS | 1–55; default 20 |
| MVM_DEV_HTTP | explicit loopback-only development HTTP |
| MVM_PROXY_TLS | explicit HTTP backend behind a verified TLS proxy only; set by Compose, never publish the raw app port |

See README for exact build/bootstrap/Compose commands. Never use sample passwords or a newly generated master key for an existing database. For native Linux installation, use the installer wizard and protected-stdin `mvmctl bootstrap-admin` operation. The native wizard always creates the administrator in Bash; browser setup closes after account creation. Create additional roles after sign-in in Settings → Accounts. Account passwords and portal accounts are independent of PPP credentials.

Compose uses a non-root scratch application container, read-only root filesystem, no Linux capabilities, resource limits, persistent bind mount and health checks. Provide `deployments/docker/tls/server.crt` and `server.key` from your trusted PKI; align SAN, `MVM_TLS_HOST` and `MVM_ORIGIN`. Set restrictive permissions and ownership for `data` (UID 65532). Caddy terminates TLS on 8443 with no automatic certificate issuance. The published socket defaults to host loopback. Remote management exposure requires an explicit reviewed binding/firewall change and trusted browser CA configuration. No public-registry push is performed by CI.

## Router manual bootstrap

Do this independently, with authorized console/WinBox access and a verified backup. **The app does not execute bootstrap commands.** Consult the current MikroTik REST manual for your version. Configure `www-ssl` with a valid management certificate matching the selected hostname/IP and enable HTTPS REST. Restrict the service address and input firewall to the management application host. Disable plaintext management services where appropriate only under separate approval. Sync router NTP/clock.

Create a dedicated RouterOS group/account. Reading needs `read,rest-api`. Provisioning needs `write`; file exports need `ftp`, file reads may also need `test`; private-key exports need `sensitive`. Certificate signing permission requirements must be tested on the chosen release; do not automatically grant `policy`, reboot or full administrative rights. Scope access with the management network and service restrictions; RouterOS policy granularity is limited.

Add the router with its credentials and the private CA PEM if necessary. Registering only stores inventory. Request a sync, review version/resources and any missing-menu warnings. Do not approve a VPN job until firewall ordering, destination allowlist, return routes, upstream port forwarding/public endpoint, router clock and PKI backup are reviewed. A router behind CGNAT is not made reachable by opening a local listener; arrange a supported public path first.

## فارسی

نصب عملیاتی نیازمند HTTPS معتبر، کلید اصلی محفوظ، مسیر مدیریت محدود و یک نمونه برنامه برای هر پایگاه داده است. ابتدا مدیر را با `bootstrap` بسازید. گواهی و دسترسی REST روتر باید به‌صورت دستی و با مجوز آماده شده باشد. محدوده مقصد را در `MVM_ROUTER_CIDRS` به کوچک‌ترین شبکه ممکن محدود کنید. افزودن روتر فقط اطلاعات را ذخیره می‌کند؛ اتصال با درخواست شما انجام می‌شود. پیش از اعمال، پشتیبان روتر، ترتیب فایروال، NTP، مسیر برگشت و نشانی عمومی را بررسی کنید.


## Installer wizard and first credentials (2026-10-19)

Run from the reviewed repository or extracted verified native release:

```bash
sudo bash editions/linux/installer/install.sh install /absolute/path/mvm-linux-amd64-0.2.0-dev.tar.gz <INDEPENDENTLY_TRUSTED_SHA256> 0.2.0-dev
```

Use arm64 on the ARM implementation targets. See [authenticated release delivery](releases.md) for the self-contained root bootstrap, release pipeline, trust policy and publication gates. Low-level local archives still require an independently trusted digest. A controlling terminal is mandatory. No unattended installation workflow is provided.

The wizard collects port (default 8443, range 1024–65535, conflict check), hostname/management IPv4 address, username, hidden password and confirmation, timezone, English/Persian, and local-only or trusted TLS access. Use a unique passphrase of 14–233 bytes. Passwords are not arguments, environment variables, configuration values, logs, or summary output. No default credentials exist. `mvmctl bootstrap-admin DB USERNAME TIMEZONE en|fa` reads the exact password from protected stdin with no terminating newline. The installer calls it as the unprivileged service user after SQLite initialization, using the existing master-key file. Argon2id hashing, account-existence guard, encrypted preferences and Super Administrator account creation commit in one transaction. Any existing account causes refusal. Browser setup returns 404 once an account exists, so even a leftover setup token is unusable. Installer bootstrap creates no token.

The Bash wizard always creates the first administrator. Browser-deferred setup is not offered by the native installer.

Without TLS, the actual listener and origin are `127.0.0.1:PORT` and `http://127.0.0.1:PORT`. The completion summary reports the verified local URL and an SSH forwarding command. TLS installation validates system trust, expiry, hostname/IP SAN and key pairing, requires an explicit assigned IPv4 bind address and explicit management-firewall review. It does not change firewall rules. Readiness connects to that bind address with the configured hostname for TLS verification, bypassing proxies without disabling certificate checks. Remote reachability is not implied. Supply a full-chain PEM; custom trust roots must already be installed in system trust. IPv6 wizard binds are currently unsupported.

Ubuntu LTS 22.04/24.04 and Debian 12/13, amd64/arm64, root permission, systemd, disk space and a usable network route are required. Missing runtime packages trigger a package list and explicit confirmation before `apt-get update` and targeted installation. No full-system upgrade, Go SDK, Docker or external SQLite server is required. A local release needs no Internet download when prerequisites are present; HTTPS releases and APT fail explicitly on unavailable networks.

`sudo mvm` opens the management menu. Existing install/update/uninstall/start/stop/restart/status/logs/port/tls/reset-admin/backup/restore/rollback/version operations remain implemented. Port and TLS configuration changes now require verified database backups; credential resets use hidden input and confirmation. Fresh-install retries refuse existing or incomplete persistent state. Use reviewed repair/update procedures; no database or master key is regenerated on a retry. Failed readiness leaves files available for repair.

Validation: installer Bash syntax and mocked installer suites passed, including wizard confirmation/redaction, port bounds/conflicts, loopback summary, prerequisite refusal, repeated staging, failed-update recovery, backups, rollback and uninstall preservation. Go suites `./cmd/mvmctl ./internal/storage/sqlite ./internal/http ./internal/security` passed outside the Windows sandbox (the sandbox denied existing backup/restore hard links). Added transactional bootstrap and offline command tests cover password length, preferences, existing-account protection and retries; existing HTTP tests cover setup-token authorization and closure. Actual systemd lifecycle and live trusted-TLS/remote firewall tests were not executed: no accessible disposable Ubuntu/Debian systemd environment is available. The disposable VM test feeds random first-run credentials through a controlling PTY with echo disabled; script stdin is never used for passwords. Development Preview remains explicit; production acceptance and RouterOS/VPN interoperability remain outstanding.

Changed source files for this task: `editions/linux/installer/{common,install,menu,wizard}.sh`, `cmd/mvmctl/{main.go,main_test.go}`, `internal/storage/sqlite/{bootstrap.go,bootstrap_test.go}`, `tests/installer/{run,wizard}.sh`, `tests/systemd/disposable-vm.sh`, `README.md`, `DELIVERY.md`, `editions/linux/README.md`, `docs/operations/installation.md`.
