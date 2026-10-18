> Edition update — 2026-10-19: Historical v0.1.0 record below. The existing repository has now been incrementally refactored as 0.2.0-dev; see [edition architecture](architecture/editions.md) and [current delivery](../DELIVERY.md). No original user database was modified.

# Implementation record — 2026-10-18

## Repository assessment

The supplied root contained only `SKILL.md` and `DESIGN.md`, both read completely before UI work. No existing application, Git history, `AGENTS.md`, dependency manifest or build setup was present. The two design references were preserved. Git was installed; Go and Docker were absent from PATH. No real router information or permission to mutate a real network was supplied.

## Milestones delivered

1. **Assessment:** local design and workspace inspection; primary RouterOS REST/OpenVPN/certificate/file references reviewed. The Meet AJ URL failed retrieval and was recorded as unverified.
2. **Foundation:** Go/Gin modular monolith, environment configuration, encrypted bbolt schema 1, Argon2id bootstrap/session login, role scopes, local assets, HTML layouts and English/Persian dictionaries.
3. **Router integration:** HTTPS/system-private CA trust, bounded calls and typed rejection errors; SSRF/DNS controls; inventory/sync/version/resources/menu observation; stale/drift labels; verified TLS mock tests.
4. **OpenVPN automation:** preflight, reserved IPs, router-held certificate templates/signing, bounded read-only readiness polling, pool/profile/server resources, read-only firewall inspection, approved rule preview, durable intent/ownership, compensation guards, read-back and one-server PPP isolation.
5. **Client delivery:** actual export/read/cleanup code, durable export intent, encrypted-key and chain validation, bounded PKCS#8 parsing, key/certificate association, unified profile, locally generated URL QR, authenticated one-time delivery and scoped revocation.
6. **Security/visibility:** user disable+disconnect, session/certificate snapshots, audit views, scope checks, authenticated metrics, health/readiness, strict origin/CSRF/CSP and retention for temporary delivery/unused preview secrets. Full diagnostics/MFA and additional lifecycle operations remain pending.
7. **Packaging:** non-root scratch multi-stage Dockerfile, versioned Compose with trusted-certificate reverse proxy, read-only runtime, CI quality/scanning/build definitions, build script/manifest, backup and non-overwriting restore, security/operations/compatibility docs. Docker execution is blocked by the missing engine.
8. **Future protocols:** deliberately not expanded before live OpenVPN validation. No fake functional adapter or native OpenVPN QR is exposed.

## Significant findings corrected

Encrypted records preserve password hashes and router credentials internally but response objects explicitly redact them. Scoped auditors receive only permitted events. Delivery consumption avoids nested database transactions and serializes against user-disable operations. Read-back checks include firewall rule order. New identities recheck the managed server/policy and reject newly introduced unmanaged PPP accounts. Compensation refuses observed external dependencies or unreadable safety menus. Export crash recovery retains the exact random temporary prefix without retaining its passphrase.

Real-browser testing caught `Referrer-Policy: no-referrer` suppressing form Origin; it was changed to `same-origin`, preserving strict same-origin checks and suppressing cross-origin referrers. A stale test process initially served old headers; a fresh port/process confirmed the fix. The delivery fragment is preserved across the local sign-in redirect without putting it in a query string or persistent browser storage.

## Toolchain

Direct official Go downloads returned 404 and the official module proxy returned regional access denial. A public Go module mirror supplied Go 1.25.3 initially. The final compiler is the official Microsoft Go 1.27.2 Windows amd64 build obtained from Microsoft's release manifest/download service; the archive's SHA256 was checked against that manifest. Go dependencies used the working proxy with checksum-database verification enabled. Final pins are in go.mod/go.sum, including Gin 1.12.0, bbolt 1.5.0, x/crypto 0.58.0 and an exact PKCS#8 module revision. No verification bypass or insecure router TLS option was introduced.

## Verification and honest scope

See `verification.md` for executed results and `limitations.md` for unfinished application features. The code and local/mocked checks are real, but no RouterOS or VPN client interoperability evidence exists. The highest-priority next milestone is an explicitly authorized disposable lab run, correction of any version-specific behavior it reveals, and positive/negative client connectivity proof before feature expansion or a production claim.

