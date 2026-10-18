# Release limitations and incomplete acceptance criteria

The 1.0 release stabilizes the native Linux manager. OpenVPN automation remains mock-tested and experimental, **not the completed production VPN MVP**. The following distinctions are intentional and visible in the UI. No real router has been connected or modified.

## External validation blockers

* No authorized RouterOS CHR/hardware lab or real VPN client was provided. Server/sign/export commands, imported profile, tunnel establishment, allowed/forbidden access, effective offboarding and live rollback are NOT RUN. The opt-in plan is in `tests/lab/README.md`.
* Docker Engine/Compose is absent. Dockerfile/Compose/CI definitions are supplied; actual image build, container health, reverse-proxy deployment, OCI/SBOM and container scans are NOT RUN. Linux binaries can be cross-compiled; that does not establish RouterOS Container support.
* The Windows host has no C compiler/cgo configuration. Race testing is NOT RUN locally; CI includes Linux race detection.
* The Go vulnerability scanner was downloaded/run, but `https://vuln.go.dev/index/modules.json.gz` returned HTTP 403. This is **BLOCKED**, not a clean vulnerability result. CI must obtain the advisory database and pass scans before release.
* The requested Meet AJ article could not be retrieved. Official MikroTik docs were consulted, and commands are guarded; no claim is made that the unavailable article was followed in detail.

## Application work still pending

* Router edit/credential rotation and a friendly inventory-removal form; deletion is currently a super-admin API with dependency guards. Stable router identity verification beyond HTTPS/model/architecture and a full semantic configuration diff/import/reconcile UI are pending.
* Server edit/disable/delete, full user edit/enable/revoke/rotate/reissue workflows, port restrictions per policy and individual user policies. Implemented user operations are create, reserved address, disable+disconnect, disconnect, encrypted profile generation and delivery. Do not present disabled PPP credentials as cryptographic certificate revocation.
* MFA; account edit/delete and bulk session reset; immutable external audit shipping; stronger account-aware/proxy-aware distributed throttling. Five roles and explicit router scopes exist; group labels are not group-derived grants.
* Complete PKI inventory lifecycle, automatic renewal, CRL enforcement, negative certificate-revocation tests and offline credential expiration. Expiration is not offered as a functional feature. Certificate validity is checked during profile generation; metadata snapshots are available.
* Full diagnostics/routing simulator, raw/mangle/NAT/FastTrack semantic analysis, exhaustive port-conflict detection, advanced cipher/VRF/IPv6/full-tunnel combinations and complete dependent-resource graph. Current preflight validates local fields, observed networks/pools, one-server PPP scope and first-chain firewall placement. Independent firewall/return-routing review and actual negative access tests are mandatory.
* Job manual-intervention resolution/import GUI. Safe stop-and-review behavior exists, but recovery currently needs the documented reviewed maintenance procedure. A failed automatic compensation never authorizes deleting another operator's resources.
* Search/pagination/filter UI across every large table; the audit API pages events, but inventory/jobs/users currently load complete scoped lists. No high-scale performance claim.
* Persistent background monitoring, pool-utilization dashboard, automatic auth-failure analysis and full Prometheus telemetry. Inventory counts are real; observed router state is explicit and timestamped; enabled users are never called connected.
* End-user renewal-request queue and automatic profile regeneration. End users see only assigned profiles and consume authorized links; operators generate the encrypted material.
* Automated key-rotation migration and backup-retention scheduler. Consistent backup and refusal-to-overwrite offline restore are implemented/tested. Rotation/retention runbooks describe responsibilities, not shipped automation.
* WireGuard/native QR, L2TP/IPsec, SSTP and legacy PPTP remain unavailable. Lite packaging and Go cross-builds exist, but RouterOS Container deployment/runtime compatibility remains unverified. No functional buttons claim otherwise.

## Known operational caveats

One managed server per router. Existing OpenVPN records or enabled external PPP `any`/`ovpn` identities block provisioning. This includes some factory or manually configured setups; the app never removes them automatically. Router signing/export behavior and exact file names need live verification. Read-back comparison is conservative and may flag harmless RouterOS normalization as manual intervention rather than guessing equivalence.

Compensation checks observed dependencies in relevant pool/profile/certificate consumers and stops if required menus are unreadable. It conservatively refuses CA deletion when any external certificate exists. This favors safe incomplete cleanup over deletion; it is not an exhaustive proof against simultaneous WinBox edits. Use an exclusive authorized maintenance window for the lab.

A profile token is consumed before streaming, so an interrupted browser download requires fresh delivery generation. After a host crash, encrypted export files may remain on the router; the persisted random prefix identifies the exact cleanup scope. Deleted database records or lost app data do not revoke existing router credentials. An app outage leaves router VPN services running.

An encrypted private key is transiently decrypted for certificate association, with bounded PBKDF2/scrypt parameters. Formats outside that parser policy fail closed. Passphrases and decrypted keys are not persisted, but complete in-memory erasure is not guaranteed by Go. Keep the host and PKI recovery path secure.

## Remaining edition acceptance

Ubuntu 24.04 amd64 preview lifecycle evidence exists; 1.0 stable lifecycle and publication evidence is recorded in docs/verification.md. Debian, Ubuntu 22.04, arm64 runtime, remote trusted TLS/firewall behavior, MikroTik Container runtime and real router/client VPN acceptance need separate validation. Full Go tests/vet pass locally outside sandbox; release CI must pass vulnerability checks and native lifecycle before publication. Schema 1 is unchanged. SQLite uses one connection per store; single-instance deployment is required. Timezone/language preferences persist, but timestamp rendering remains UTC. Driver license/performance review, immutable external audit and external security review remain outstanding.
