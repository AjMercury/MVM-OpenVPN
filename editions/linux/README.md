# Linux Edition

Stable Linux manager release path. RouterOS VPN automation remains experimental and disabled by default; real router/client interoperability is incomplete. Ubuntu 24.04 amd64 is the primary native lifecycle/public installation target. Other distribution/architecture runtimes remain unverified. Current execution evidence is in [verification](../../docs/verification.md).

The management workflow references the [3x-ui installation/menu experience](https://github.com/MHSanaei/3x-ui/wiki/Installation). These scripts were authored for this repository; no 3x-ui source or assets were copied.

## Native layout and installation

Supported installation targets are Debian 12/13 and Ubuntu Server 22.04/24.04 LTS, amd64/arm64, with running systemd. These are implementation targets; distribution-specific runtime certification remains outstanding. Dependencies: Bash, coreutils, systemd/journalctl, curl, OpenSSL, tar, util-linux (flock/runuser), useradd and at least 512 MiB free in `/var`. Allow additional capacity proportional to database snapshots/releases. SQLite WAL requires a local filesystem; do not use NFS/SMB.

Default paths:

```text
/opt/mvm/releases/VERSION/{server,mvmctl,metadata.env,editions/linux/...}
/opt/mvm/current -> releases/VERSION
/etc/mvm/{config.env,master.key,bootstrap.token,tls/}
/var/lib/mvm/mvm.db
/var/backups/mvm/
/usr/local/bin/mvm
/etc/systemd/system/mvm.service
```

Build a local release with `bash scripts/package-linux.sh VERSION amd64` (or `arm64`). The output contains binaries, installer and service definition; adjacent files include SHA-256, binary build info and SPDX dependency SBOMs. SBOM license fields use NOASSERTION; no license/vulnerability audit is implied.

The self-contained root `install.sh` discovers official stable releases and verifies a Sigstore-attested manifest before executing the wizard. See [release trust, configuration and availability gates](../../docs/operations/releases.md). The independently pinned official trust root avoids the blocked TUF CDN. Stable management menus select the stable channel; preview menus retain the explicit preview channel. To upgrade from an older preview to stable, run the new root bootstrap with `--action update`. A controlling `/dev/tty` is required, including for piped execution. Low-level reviewed local archive commands still accept an independently trusted digest; no unattended credential workflow is supported.

Fresh installation creates system account `mvm`, root-owned key/config readable by that group, private data/backups, a native unit and readiness check. Existing, partial and preserved-data installations are refused by `install` without replacing credentials. Use `repair` only with an existing valid SQLite database and original key; it stages another version, verifies a mandatory backup, reinstalls packaging, and checks readiness. Incomplete initialization without a database requires reviewed manual recovery rather than automatic empty initialization. If a fresh install fails after creating credentials, preserve them for diagnosis.

The management menu opens with `sudo mvm`. The Bash wizard always creates the administrator offline with hidden password confirmation, before starting the service. For local mode use the displayed SSH tunnel and sign in at `http://127.0.0.1:PORT` (default 8443). The normal login page is used; initial credentials are not created in the browser. Once an account exists, GET and POST `/setup` return 404.

## Exact commands

```sh
sudo mvm install ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 VERSION
sudo mvm repair ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 NEW_VERSION
sudo mvm update ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 NEW_VERSION
sudo mvm update  # authenticated official preview release selection
sudo mvm start
sudo mvm stop
sudo mvm restart
sudo mvm status
sudo mvm logs
sudo mvm port 8443                    # retains bind address and protocol; 1024..65535
sudo mvm tls /secure/server.crt /secure/server.key https://panel.example:8443
sudo mvm reset-admin EXISTING_USERNAME # password is prompted without echo, never a process argument
sudo mvm backup                      # verified snapshot under /var/backups/mvm
sudo mvm backup /var/backups/mvm/manual.db
sudo mvm restore /var/backups/mvm/manual.db
sudo mvm rollback
sudo mvm version
sudo mvm uninstall
sudo mvm uninstall --purge
```

All backup destinations must be new paths inside `/var/backups/mvm`. Native master key file is external to SQLite. Environment examples contain no real secrets. Preserve configuration/key/certificates separately through protected off-host backup custody.

TLS installation checks expiry and certificate/key association and tests readiness with normal certificate verification; a trusted CA/SAN/origin/current port must agree. The management TLS command retains the current listener. The installation wizard can configure an explicit assigned IPv4 bind address after firewall review; later remote binding requires reviewed administrator edits to `config.env`. The installer does not request ACME certificates or change firewall rules. A failed TLS or port readiness check restores the previous config. Encrypted TLS keys requiring interactive passphrases are not supported by the unattended service.

`reset-admin` stops the service, requires a verified database backup, changes only an existing super-administrator password and invalidates local sessions. The CLI reads the password from stdin. No default/reset password is printed.

## Updates, rollback and restore

No-argument menu/CLI updates use the installed authenticated bootstrap. Explicit offline updates can still supply source/digest/version from an independent trusted channel. The legacy low-level `release.env` fallback is retained for reviewed recovery scripts, but the menu does not require this file.

Update preflights existing configuration/key/TLS and disk, verifies/stages the archive, stops the service, creates and decrypt-verifies a SQLite snapshot, and checks the target binary against the current database before atomic symlink activation. Only schema 1 is currently supported; metadata with another schema is rejected. Config, master key and TLS are not overwritten. Readiness failure switches back to the prior schema-compatible release; recovery is covered by fault-injection tests. Old releases are retained. Failed staging leaves a release directory for inspection; a repeated update version is deliberately refused, while explicit repair uses a new independently verified repair directory even for the current version.

`rollback` verifies the prior release and backs up current data before checking database compatibility and switching. Schema-changing automatic updates/downgrades are **not implemented**; incompatible schema checks leave the service stopped with the recovery record available. Do not force an old binary onto another schema. For controlled schema recovery, stop the service, preserve the failed database/WAL/SHM together, verify the pre-update snapshot using its matching binary/key, restore into a new destination with that binary's `mvmctl restore`, and activate only after validation. Native `mvm restore BACKUP` verifies a private restored file before replacing a stopped database, keeps the prior database and reverses activation on readiness failure. A database restore does not undo router changes.

Backups use SQLite `VACUUM INTO`, not live file copying. Each snapshot is checked and decrypted by restore rehearsal. Treat `mvm.db-wal` as part of the live database until SQLite checkpoints it; never omit sidecars when preserving a crashed installation for investigation. Never run an independently restored copy with router writes enabled until reconciliation.

## Uninstall

Standard removal displays exact code/service paths and requires `REMOVE`. It preserves `/etc/mvm`, `/var/lib/mvm`, `/var/backups/mvm` and the service account. `--purge` additionally displays those paths and requires a separate `DELETE ALL MVM DATA` confirmation. No RouterOS API is invoked. VPN servers/users/certificates/pools/firewall rules on routers remain unchanged.

## systemd and validation

The service is unprivileged, restarts on failure, follows network-online, handles SIGTERM gracefully, has a private temp directory, restrictive umask, read-only system directories, private home access, bounded processes/files/CPU/RAM and explicit persistent write access. Secrets are read from protected files/config, not embedded in the unit. The sandbox/resource choices need runtime validation on each supported distro.

```sh
systemd-analyze verify /etc/systemd/system/mvm.service
systemctl enable mvm
systemctl start mvm
systemctl stop mvm
systemctl restart mvm
systemctl status mvm
journalctl -u mvm -f
```

`tests/systemd/disposable-vm.sh` implements actual lifecycle tests with strict empty-host and explicit disposable-VM guards. It is NOT RUN locally; the manual systemd workflow has not been executed. Bash syntax and mocked lifecycle tests do not establish systemd success.

## Docker

Copy `config/container.env.example` to `config/container.env`, create `docker/{data,secrets,tls}` with UID 65532 ownership/read permissions, provision the external master key and bootstrap token in `secrets`, and valid PEM files in `tls`. Preserve existing keys; migrate old bbolt data offline first. Then run from the repository root:

```sh
docker compose -f editions/linux/docker/compose.yaml build
docker compose -f editions/linux/docker/compose.yaml up -d
```

The image is non-root/read-only with writable `/data` and bounded private `/tmp`. Published HTTPS binds host loopback. Origin, certificate SAN and trust must match. Docker builds/runtime are NOT RUN on this host.



## Installer wizard and first credentials (2026-10-19)

Run from the reviewed repository or extracted verified native release:

```bash
sudo bash editions/linux/installer/install.sh install /absolute/path/mvm-linux-amd64-0.2.0-dev.tar.gz <INDEPENDENTLY_TRUSTED_SHA256> 0.2.0-dev
```

The root bootstrap discovers official releases and verifies the signed manifest before installation. Preview publication and Ubuntu 24.04 amd64 public end-to-end verification passed; stable publication remains gated. See the authenticated release-delivery documentation. Low-level offline artifacts require an independently trusted digest.

The wizard collects port (default 8443, range 1024–65535, conflict check), hostname/management IPv4 address, username, hidden password and confirmation, timezone, English/Persian, and local-only or trusted TLS access. Use a unique passphrase of 14–233 bytes. Passwords are not arguments, environment variables, configuration values, logs, or summary output. No default credentials exist. `mvmctl bootstrap-admin DB USERNAME TIMEZONE en|fa` reads the exact password from protected stdin with no terminating newline. The installer calls it as the unprivileged service user after SQLite initialization, using the existing master-key file. Argon2id hashing, account-existence guard, encrypted preferences and Super Administrator account creation commit in one transaction. Any existing account causes refusal. Browser setup returns 404 once an account exists, so even a leftover setup token is unusable. Installer bootstrap creates no token.

The native Bash wizard always creates the first administrator with hidden password confirmation. Browser-deferred setup is not offered.

Without TLS, the actual listener and origin are `127.0.0.1:PORT` and `http://127.0.0.1:PORT`. The completion summary reports the verified local URL and an SSH forwarding command. TLS installation validates system trust, expiry, hostname/IP SAN and key pairing, requires an explicit assigned IPv4 bind address and explicit management-firewall review. It does not change firewall rules. Readiness connects to that bind address with the configured hostname for TLS verification, bypassing proxies without disabling certificate checks. Remote reachability is not implied. Supply a full-chain PEM; custom trust roots must already be installed in system trust. IPv6 wizard binds are currently unsupported.

Ubuntu LTS 22.04/24.04 and Debian 12/13, amd64/arm64, root permission, systemd, disk space and a usable network route are required. Missing runtime packages trigger a package list and explicit confirmation before `apt-get update` and targeted installation. No full-system upgrade, Go SDK, Docker or external SQLite server is required. A local release needs no Internet download when prerequisites are present; HTTPS releases and APT fail explicitly on unavailable networks.

`sudo mvm` opens the management menu. Existing install/update/uninstall/start/stop/restart/status/logs/port/tls/reset-admin/backup/restore/rollback/version operations remain implemented. Port and TLS configuration changes now require verified database backups; credential resets use hidden input and confirmation. Fresh-install retries refuse existing or incomplete persistent state. Use reviewed repair/update procedures; no database or master key is regenerated on a retry. Failed readiness leaves files available for repair.

Validation: installer Bash syntax and mocked installer suites passed, including wizard confirmation/redaction, port bounds/conflicts, loopback summary, prerequisite refusal, repeated staging, failed-update recovery, backups, rollback and uninstall preservation. Go suites `./cmd/mvmctl ./internal/storage/sqlite ./internal/http ./internal/security` passed outside the Windows sandbox (the sandbox denied existing backup/restore hard links). Added transactional bootstrap and offline command tests cover password length, preferences, existing-account protection and retries; existing HTTP tests cover setup-token authorization and closure. Actual Ubuntu 24.04 amd64 systemd lifecycle and public preview installation tests now pass in GitHub Actions. Live trusted-TLS/remote firewall tests remain unexecuted. The disposable VM test supplies random first-run credentials through an echo-disabled controlling PTY. Development Preview remains explicit; production acceptance and RouterOS/VPN interoperability remain outstanding.

Changed source files for this task: `editions/linux/installer/{common,install,menu,wizard}.sh`, `cmd/mvmctl/{main.go,main_test.go}`, `internal/storage/sqlite/{bootstrap.go,bootstrap_test.go}`, `tests/installer/{run,wizard}.sh`, `tests/systemd/disposable-vm.sh`, `README.md`, `DELIVERY.md`, `editions/linux/README.md`, `docs/operations/installation.md`.
