# MikroTik VPN Manager 1.0

Linux management panel for MikroTik OpenVPN, with English/Persian UI, scoped roles, encrypted storage, verified RouterOS HTTPS, reviewed provisioning jobs and single-use profile delivery.

[v1.0.2 is published](https://github.com/AjMercury/MVM-OpenVPN/releases/tag/v1.0.2), with automatic server-IP HTTPS and live installation progress. [Public installation and native Ubuntu 24.04 lifecycle pass](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39176357352), and [security/race/container checks pass](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39176357366).

**Release scope:** stable native Linux manager. RouterOS VPN automation remains experimental and disabled by default. Actual router/client interoperability, effective access restrictions and certificate offboarding still require an authorized lab. This release does not certify those workflows for production.

## Install on Ubuntu

Ubuntu 24.04 amd64 is the primary runtime validation target. The installer also accepts Ubuntu 22.04 and Debian 12/13 on amd64/arm64; those combinations require separate runtime validation.

```bash
curl -fsSL https://raw.githubusercontent.com/AjMercury/MVM-OpenVPN/master/install.sh | sudo bash
```

The default command selects the newest stable release. For a preview, explicitly append `-s -- --channel preview` to `sudo bash`. Stable and preview channels never silently fall back to one another. An interactive SSH terminal is required.

The wizard asks for the administrator username and hidden password, port, language and timezone. No default password is shipped. From 1.0.2, default access uses the detected server IP or supplied domain on port 8443. Automatic HTTPS generates a certificate for this server IP and any supplied domain; verify/import the displayed certificate fingerprint on first browser access. Local-only access is optional. The installer does not modify your firewall or existing web server.

Release signatures are verified before executing downloaded code. The official Sigstore trust root is pinned by commit and SHA-256 and fetched through GitHub; the blocked Sigstore TUF CDN is not required. No GitHub login is needed. Checksums alone are not accepted as a replacement for provenance verification. See [release trust and troubleshooting](docs/operations/releases.md).

```bash
sudo mvm
sudo systemctl status mvm
journalctl -u mvm -f
```

For an existing preview installation, use `sudo mvm` to update with the existing database and master key. An older preview manager may select its preview channel; use the new bootstrap with `--action update` to explicitly select stable. Do not delete data or regenerate keys to retry an installation.

To move an existing localhost-only installation to direct IP access after upgrading to 1.0.2, run `sudo mvm access SERVER_IP_OR_DOMAIN` (or menu option 16). This preserves accounts and takes a verified backup before changing the listener. If a domain is supplied, the detected server IP is also accepted for HTTPS access. Firewall rules remain under your control.

```bash
curl -fsSL https://raw.githubusercontent.com/AjMercury/MVM-OpenVPN/master/install.sh | sudo bash -s -- --action update
```

## فارسی

برای نصب روی اوبونتو ۲۴٫۰۴ دستور بالا را در ترمینال SSH اجرا کنید. نام کاربری و رمز مدیر در ویزارد تعیین می‌شوند. نسخه پایدار به تأیید Preview نیاز ندارد. بررسی امضای نصب بدون وابستگی به CDN مسدودشده Sigstore انجام می‌شود. از نسخه ۱٫۰٫۱ پنل به‌صورت پیش‌فرض با IP سرور یا دامنه باز می‌شود؛ localhost اختیاری است. برای رمزگذاری اتصال، حالت TLS را انتخاب کنید. تنظیمات روتر به‌صورت پیش‌فرض تغییر نمی‌کنند؛ آزمایش عملی VPN روی روتر واقعی همچنان لازم است.

## Editions and architecture

| Edition | Storage | Deployment |
|---|---|---|
| Linux | SQLite with AES-256-GCM encrypted record payloads | Native unprivileged systemd; Docker optional |
| MikroTik Lite | Compatible AES-256-GCM bbolt record payloads | RouterOS Container; runtime unverified |

The shared Go/Gin modular monolith separates HTTP, application, jobs, RouterOS transport, provisioning, security, IPAM and storage. Local assets support English/Persian RTL. Argon2id protects account passwords; role/router scopes and CSRF protect authenticated operations. Router destinations require an allowlist and verified TLS, and numeric dialing prevents DNS rebinding. Writes require explicit configuration and confirmation.

Ready previews expire after fifteen minutes and release unapplied reservations. Ambiguous jobs remain blocked for review. Terminal jobs discard PPP passwords, expired sessions are purged, and delivery tokens expire after five minutes. Interrupted downloads need a newly issued link.

For a custom reverse proxy, set `MVM_TRUSTED_PROXIES` only to its actual IPs or restricted CIDRs and prevent direct access to the backend. The supplied root Compose deployment trusts only its fixed Caddy address. Account and IP sign-in limits are separate.

## Build and verify

CI and release builds use Go 1.27.2. The module language minimum is Go 1.26.

```bash
go test -tags nomsgpack ./...
go vet -tags nomsgpack ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -tags nomsgpack ./...
bash tests/installer/run.sh
bash scripts/package-linux.sh 1.0.0 amd64
```

Windows source builds can use `.tools/go/bin/go.exe`. Bash mocks and cross-builds do not prove systemd or RouterOS compatibility. [Verification](docs/verification.md) distinguishes local tests, CI runtime evidence and unexecuted acceptance tests.

## Recovery and documentation

Preserve the database, configuration and external master key together. Existing bbolt installations require explicit offline migration to a new SQLite destination; never point the Linux server at a legacy database or generate a replacement master key. Normal uninstall preserves persistent data and does not revoke router credentials.

- [1.0.0 release notes](docs/releases/1.0.0.md)
- [1.0.2 direct IP/domain HTTPS and installer progress](docs/releases/1.0.2.md)
- [Installation](docs/operations/installation.md), [upgrade](docs/operations/upgrade.md), [backup/restore](docs/operations/backup-restore.md), [migration](docs/operations/edition-migration.md)
- [Limitations](docs/limitations.md), [threat model](docs/security/threat-model.md), [secrets and PKI](docs/security/secrets-and-pki.md)
- [Linux edition](editions/linux/README.md), [Lite edition](editions/mikrotik/README.md), [authorized lab](tests/lab/README.md)
- [Persian source review](docs/project-review-fa-2026-10-19.md)
