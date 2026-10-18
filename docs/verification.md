# 1.0.3 optional HTTPS and terminal manager

[Ubuntu lifecycle run 39178017699](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39178017699) passes native HTTP installation with detected IPv4, domain/IP login, self-signed HTTPS, port migration to 8444, return to HTTP, same-version repair, update/rollback and uninstall. Accounts and master keys are preserved. The first expanded lifecycle run reached systemd's default start-rate limit after deliberately rapid transitions; independent test scenarios now reset that counter without weakening the shipped service policy.

Local Go tests and vet pass. Linux Bash tests cover ACME private-IP refusal, public-IP short-lived profile, standalone/webroot arguments, changed-client hash refusal, issuance failure preservation, renewal/no-change behavior, preventing renewal from overwriting HTTP, failed TLS readiness rollback and terminal color/error-return behavior. Actual public CA issuance is not run: it requires an authorized public domain/IP and inbound challenge reachability. The implementation uses a pinned official acme.sh dependency; no 3x-ui source is copied.

The historical 1.0.2 sections below describe its former HTTPS-only network default; 1.0.3 adds explicit network HTTP and optional ACME HTTPS. Release/public-install results will be recorded after publication.

# 1.0.2 direct HTTPS and installer progress

No server IP is hardcoded. The wizard detects the primary and other assigned IPv4 addresses, generates a private HTTPS certificate covering those IPs plus an optional domain, and displays its fingerprint for browser trust. Network cookies remain Secure; plaintext HTTP remains loopback-only. Existing installations use `sudo mvm access` after update, with automatic address detection and a verified backup. Installation phases and download progress are visible without logging passwords or keys.

[Quality run 39175825354](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39175825354) and [Ubuntu run 39175825499](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39175825499) pass. Tests cover multiple different server IPs, secondary interface discovery, verified non-loopback HTTPS login/logout, both configured origins, migration, recovery, unit refresh, update/rollback and secret redaction. Public stable installation of the new release is checked after publication.

Final [v1.0.2 release run 39176357352](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39176357352) passes both native builds, Go/Bash/security gates, actual Ubuntu HTTPS lifecycle, publication and public installation. Public installation checks visible phase logs, credential redaction, verified non-loopback HTTPS login, Secure cookies, closed setup and manifest tamper rejection. [Tag quality run 39176357366](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39176357366) passes Linux installer suites, Go race/vulnerability/secret checks and both architecture image scans. The unpublished v1.0.1 tag was stopped by a test fixture that accidentally included live runner interfaces; its release assets were never published or overwritten.

# 1.0 verification status (2026-10-19)

Local full `go test -tags nomsgpack ./...`, `go vet` and manifest validation pass outside the Windows sandbox. Added regression coverage exercises successful repeated login, account throttling across source IPs, proxy header trust, dependency-read failure, expired reservation reuse in both storage editions, legacy session cleanup and terminal secret removal. Git Bash installer suites pass with bundled Python; Linux PTY/systemd assertions require CI.

The pinned official root successfully verified the real preview.3 bundle without the blocked TUF CDN. The installer test rejects an altered trust root; public installation tests verify authentic and tampered manifests. The release workflow additionally gates stable publication on Go vulnerability scans and native stable Ubuntu lifecycle. This is not real RouterOS/VPN interoperability certification.

Pre-publication evidence:

- [Quality run 39173458360](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39173458360): PASS — formatting, vet, manifests, Go race tests, vulnerability scanner, secret scan, and separate amd64/arm64 OCI image build and HIGH/CRITICAL vulnerability scans.
- [Ubuntu run 39173296693](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39173296693): PASS — stable archive install without preview acceptance, hidden administrator credentials, SQLite, systemd start/stop/restart, same-version repair, update/rollback, key preservation and standard uninstall. The public preview install job also passes with the new root verifier, ordinary login, closed setup and authentic/tampered manifest checks.

Final v1.0.0 evidence:

- [Release run 39173605545](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39173605545): PASS — amd64/arm64 native builds, Go tests/vet/vulnerability checks, Bash suites, actual stable Ubuntu lifecycle, signed manifest, publication and anonymous public stable installation. Public installation verifies first login, closed browser setup, credential redaction and rejection of a cryptographically altered manifest.
- [Tag quality run 39173605706](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39173605706): PASS — Go race/vulnerability/secret checks and both architecture OCI image scans.
- [v1.0.0](https://github.com/AjMercury/MVM-OpenVPN/releases/tag/v1.0.0) was published as a non-draft, non-prerelease release. Its twelve assets include both archives, build metadata/SBOMs, SHA256SUMS and its Sigstore bundle. Source tag points to commit `fc922b8`; later documentation changes do not replace the release artifacts.

The sections below describe historical preview evidence; current release status supersedes historical availability and scanner limitations.
# Authenticated Linux release verification — 2026-10-19

[v0.2.0-preview.3](https://github.com/AjMercury/MVM-OpenVPN/releases/tag/v0.2.0-preview.3) is published. [Release run 39170730251](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39170730251) passed both architecture builds, Linux Bash/Go tests, actual Ubuntu 24.04 amd64 systemd/SQLite administrator installation, same-version repair, update/rollback, key preservation, and standard uninstall. The separate public-install job passed the actual raw GitHub piped command without GitHub credentials, normal login, disabled browser setup, transcript redaction, valid attestation acceptance and real signed-manifest tamper rejection. See [detailed evidence and unresolved tests](operations/releases.md). No stable/production or RouterOS/VPN interoperability certification is claimed.

The following source-only verification is preserved as historical context; its unavailable-host/release statements are superseded above.

# Historical two-edition verification — 2026-10-19

Current implementation: `0.2.0-dev`, Windows amd64, Go 1.27.2. Current results are in [DELIVERY](../DELIVERY.md). Full Linux-selected and Lite-selected Go suites, vet and YAML syntax checks pass. New shared repository contracts, both-engine mock OpenVPN workflows, SQLite crash/constraint/encryption/backup/restore tests, encrypted legacy migration success/failure/interruption tests, CLI recovery and token-protected bilingual setup tests pass. Git Bash menu/preflight/archive and mocked update/rollback/uninstall tests pass. Native Linux amd64/arm64 archives and Lite server cross-builds plus dependency SBOMs are produced locally. The artifact manifest records final bytes/hashes.

Native systemd lifecycle, Docker/OCI runtime, RouterOS and real VPN-client tests are NOT RUN. Race testing is unavailable because CGO is required. No production readiness is inferred. Source checks require permitted loopback sockets and atomic hard links; the Windows sandbox blocked those operations initially and the suite was rerun successfully outside it. Hosted CI/manual systemd job has not run. Tests against real deployment databases were not performed; generated encrypted v0.1.0 schema fixtures are used.

The following is preserved historical v0.1.0 verification, including its old coverage/build numbers and browser screenshots. It must not be interpreted as newly executed UI/container tests or current-version coverage.

---
# Verification — 2026-10-18

Environment: Windows amd64, PowerShell, official Microsoft build of Go reporting `go1.27.2 windows/amd64`. Source module minimum: Go 1.26.0. No Docker Engine or C compiler was available. No real RouterOS endpoint was supplied, connected, or mutated.

| Check | Result | Evidence / limit |
|---|---|---|
| gofmt | PASS | cmd, internal, web/assets.go and manifest helper formatted |
| `go test -tags nomsgpack -count=1 -coverprofile=artifacts/coverage.out ./...` | PASS | final output in artifacts/tests.log |
| `go vet -tags nomsgpack ./...` | PASS | exit 0; artifacts/vet.log empty |
| YAML syntax for Compose, workflow, OpenAPI | PASS | `go run scripts/check-manifests.go`; syntax validation, not Docker/OpenAPI semantic execution |
| Windows amd64 binary | PASS | versioned release build and real browser smoke |
| Linux amd64 / arm64 cross-builds | PASS | CGO disabled, stripped, embedded assets; not executed on Linux |
| Race detector | NOT RUN | local command reports cgo required; no C toolchain; configured in Linux CI |
| govulncheck | BLOCKED | scanner ran but advisory DB endpoint returned HTTP 403; no clean result claimed |
| Container build / Compose / SBOM / Trivy / gitleaks CI | NOT RUN | definitions supplied; Docker/hosted CI unavailable locally |
| Real RouterOS provisioning / signing / export / rollback | NOT RUN | disposable authorized lab required |
| Real OpenVPN import/tunnel/allow-deny/offboarding | NOT RUN | exact client/OS/router versions required |
| RouterOS Container / Lite resource targets | NOT RUN | no compatible hardware test |

## Tests actually covered

The Go suite exercises authenticated encryption/context tampering, password verification, persistence of encrypted credentials, wrong master-key rejection, backup integrity and non-overwriting restore, router-scoped roles, version parsing, SSRF/invalid destinations, REST object/array parsing, 401/403/429 errors, invalid JSON, untrusted TLS, bounded timeout, chunked export-file assembly and partial-read rejection.

Job tests cover completed replay, definite rejection/compensation, uncertain response, read-back mismatch, stale previews, idempotency collision, restart recovery/blocking and external-dependency refusal. The application integration test drives a **verified HTTPS mock REST server** through a complete server preview/apply, certificate-readiness observation, owned pool/profile/firewall/server read-back, user IP reservation, duplicate/out-of-pool rejection and disable+disconnect. This is explicitly a test double, not a functioning RouterOS instance.

Profile tests use real generated X.509 fixtures. They cover CA/client chain validity, expiry, client EKU, encrypted legacy RSA/PKCS#8 key parsing, wrong passphrase and mismatched-key rejection, explicit TLS/cipher/routes, directive-injection rejection and omission of plaintext passwords/full-tunnel defaults. QR tests validate the exact fragment URL shape and a locally generated PNG; camera/decoder interoperability is not certified.

Security HTTP tests cover unauthenticated API access, CSRF/origin failures, privilege escalation, expired sessions, scoped secret redaction, XSS escaping, bilingual page rendering and RTL. Delivery tests cover cross-user isolation, expiration, one-use consumption, disabled credentials and scoped revocation. Overall statement coverage with standard per-package instrumentation: **42.9%**. This is not exhaustive; imported code exercised by another package's integration tests is not counted as coverage in its own package under this command.

## Real browser smoke

Used an isolated loopback application with a generated master key and disposable test database; no router records or network credentials were inserted. Verified sign-in, actual empty-inventory dashboard, English dark mode, Persian RTL/light mode, desktop 1440×1000 layout and mobile 390×844 onboarding with no document horizontal overflow. Verified the final server wizard exposes read-only firewall inspection and planned protocols accurately. Verified a synthetic delivery URL returns through sign-in to the protected download page and removes the URL fragment. A synthetic token is not a real delivered profile.

Screenshots: `artifacts/dashboard-dark.png`, `artifacts/dashboard-fa-light.png`. Temporary test servers are stopped after inspection. These screenshots contain honest zero inventory counts, not fabricated operational telemetry.

## Resource evidence

`scripts/build.ps1` produced stripped static artifacts with SHA256 records in `artifacts/release/manifest.json`:

| Artifact | Bytes |
|---|---:|
| mvm-0.1.0-windows-amd64.exe | 17,455,104 |
| mvm-0.1.0-linux-amd64 | 17,465,607 |
| mvm-0.1.0-linux-arm64 | 16,253,191 |

These are binary sizes, **not compressed container-image sizes**. No CPU/RAM/load benchmark was completed. Lite goals (15 MB image, 25 MB idle RAM, 64 MB peak) are unverified. Argon2id intentionally consumes 32 MiB per verification and up to two verifications may run simultaneously; do not infer the Lite peak goal is met.

The next acceptance gate is the documented disposable RouterOS lab with actual profile import and positive/negative connectivity evidence. Do not label this release production ready based on mock tests.

