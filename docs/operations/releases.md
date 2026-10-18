# Authenticated Linux release delivery

Status (2026-10-19, Asia/Tehran): **[v0.2.0-preview.3](https://github.com/AjMercury/MVM-OpenVPN/releases/tag/v0.2.0-preview.3) is published** as a Development Preview. The [release workflow](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39170730251) passed both architecture builds, Go and installer suites, actual native Ubuntu 24.04 amd64 lifecycle, manifest attestation, publication and anonymous public one-line installation/login. The preview command is operational on that tested platform. No stable Release exists; the default stable command deliberately fails. Source maturity remains Development Preview.

## Trust and selection

The self-contained root `install.sh` uses the official AjMercury/MvM-OpenVPN release API. Default stable and explicitly selected preview channels never cross-fallback. `--version vX.Y.Z[-preview.N]` pins a tag within its channel. Drafts, inconsistent pre-release flags, malformed tags and releases without all required assets are excluded. The newest published eligible release is chosen within the latest 100 GitHub release records; explicit pins use the tag-specific API endpoint, including older tags. API/rate-limit/network errors are failures, never source-build fallbacks.

Archives are `mvm-linux-amd64-VERSION.tar.gz` and `mvm-linux-arm64-VERSION.tar.gz`. VERSION omits the tag's leading `v`. Each includes `metadata.env` (VERSION, ARCH, SCHEMA, MATURITY), binaries, Linux wizard/service and technical documentation. Adjacent `.sha256`, `.buildinfo.txt`, `.server.spdx.json`, `.mvmctl.spdx.json` files are included in the top-level `SHA256SUMS` manifest. SBOM license fields remain NOASSERTION.

`SHA256SUMS.sigstore.json` is the GitHub Actions Sigstore provenance bundle for **SHA256SUMS**. The installer verifies it using independently distributed GitHub CLI trust roots, the GitHub OIDC issuer, exact certificate identity `https://github.com/AjMercury/MVM-OpenVPN/.github/workflows/release.yaml@refs/tags/TAG`, exact source tag, SLSA provenance and denial of self-hosted runners. GitHub's canonical repository spelling is intentional. Only after verification does it accept the archive's manifest digest. It validates every archive path/type and size budget before extraction in a root-private directory. No adjacent unchecked checksum authenticates an artifact.

The CLI is installed through APT after confirmation. For missing/old distro CLI versions, the installer offers the official `https://cli.github.com/packages` signed APT repository. The keyring SHA-256 is pinned to `6084d5d7bd8e288441e0e94fc6275570895da18e6751f70f057485dc2d1a811b`, independently published in [official GitHub CLI installation documentation](https://github.com/cli/cli/blob/trunk/docs/install_linux.md). Key rotation requires a reviewed bootstrap change. HTTPS and distro CA roots authenticate key delivery; APT authenticates CLI packages; CLI/Sigstore roots authenticate the application's manifest. Never replace a failed verification with `--insecure`, a downloaded verifier from the app release, or an unverified digest. Online trust-root refresh may be required; there is no guaranteed offline bootstrap. Public bundle verification should need no GitHub login; this remains an integration test gate.

The raw bootstrap itself is trusted via GitHub HTTPS and the selected master branch; a compromise of that branch can change bootstrap policy. Pin/review the bootstrap commit for stronger operational control. This mechanism authenticates release provenance; it does not establish production suitability.

## Repository configuration and publishing

1. Enable Actions and allow `contents: write`, `id-token: write`, and `attestations: write` for the release job. Public-repository artifact attestations must be available. No long-lived private signing key is used or needs a repository secret: GitHub OIDC provisions short-lived signing identity.
2. Protect master, `.github/workflows/release.yaml`, and `v*` tags with review/rulesets. Restrict tag creation to release maintainers. Create the `linux-release` environment with required reviewers and tag deployment restrictions **before** tagging. Without environment protection, a passing tag workflow can publish a preview automatically.
3. Push reviewed changes to master. Create an explicit new preview tag such as `v0.2.0-preview.1` only after choosing that version and reviewing the source. The example is a convention, not a published version. Push the tag to run `.github/workflows/release.yaml`.
4. Build/test jobs package both architectures. A separate disposable Ubuntu 24.04 amd64 systemd lifecycle job must pass before the environment-gated publication job. Publication signs the aggregate manifest and publishes a GitHub **Pre-release**. Existing tags/assets are never overwritten. Stable tags are deliberately rejected while this source's maturity is DevelopmentPreview; promoting to stable requires reviewed maturity/policy changes and runtime evidence, not merely a successful build.
5. On clean supported disposable Linux VMs, run the **actual raw GitHub command**, including PTY prompts, signature rejection, and first login. Test both architectures and every distribution before claiming them runtime-supported. Verify no `/setup` appears and no credential leaks to output. Run upgrade/rollback and failed-readiness recovery with original key/database/TLS preserved.
6. Only after successful publication and public one-line tests update README's availability banner and runtime-tested matrix. CI's local archive lifecycle test alone does not certify public release discovery, anonymous attestation verification, download or raw pipe execution.

## Recovery and operation

No unattended credential workflow is provided. `/dev/tty` is mandatory. No password is read from piped script stdin; hidden terminal input goes only to `mvmctl bootstrap-admin` via a protected process pipe, using the existing transactional Argon2id implementation. HTTP binds loopback; remote TLS requires trusted SAN/key validation and explicit assigned bind/firewall review. Both readiness and the normal login URL must succeed before the completion banner.

On bootstrap success temporary downloads are removed. On failure `/var/tmp/mvm-bootstrap-*` remains root-private with release metadata/downloads and verifier diagnostics. No password transcript is captured. After review remove that exact directory manually. Preserve `/etc/mvm`, `/var/lib/mvm`, `/var/backups/mvm` and inspect `journalctl -u mvm` after a partially completed installation. Re-running fresh install refuses persistent state. Use `sudo mvm` Install / Repair with an existing valid database; incomplete initialization without a database still needs manual recovery. Automatic repair never makes an empty replacement database.

`sudo mvm` install/repair/update obtains authenticated official preview artifacts without checksum entry. Low-level explicit archive/digest/version commands remain for independently trusted offline deployment. Updates back up/check SQLite, preserve key/config/TLS/credentials, activate atomically and recover the previous compatible version on failed readiness. Normal installation/update refuses a repeated staged version. Repair stages an independently verified copy under a new repair directory, including when repairing the current version; it never overwrites the old directory in place. Rollback retains the original schema-compatibility safeguards. The one-line installer handles only Linux; MikroTik files/device settings are untouched.

## Test evidence and outstanding gates

See `tests/installer/run.sh`, `tests/installer/bootstrap.sh`, `tests/installer/terminal.py`, and `tests/systemd/disposable-vm.sh`. Source-based tests mock service/APT operations and exercise state, password validation/redaction, dependencies, checksum failures, signature-verifier failure and exact policy arguments, safe archives, channel/pinning, API errors, failed readiness, backup failure, update/rollback and uninstall preservation. Linux-only PTY tests cover detached-pipe refusal and piped-script parsing with a controlling terminal. The systemd suite performs actual SQLite/admin/service lifecycle tests on a guarded empty disposable host using a controlling PTY.

Executed results:

| Test | Result |
|---|---|
| Windows Go `go test -tags nomsgpack ./...` | PASS outside sandbox; sandbox-only sockets/hard-link failures disappeared |
| Windows Bash installer suite | PASS; Linux PTY/POSIX permission assertions explicitly skipped locally |
| Ubuntu release build matrix (amd64/arm64 artifacts) | PASS; Go and installer suites run on amd64 host; arm64 binaries cross-built |
| Linux pipe + controlling `/dev/tty`, detached-pipe refusal | PASS in both release build jobs |
| Live Ubuntu 24.04 amd64 install, SQLite admin, systemd enable/start/stop/restart/readiness | PASS, lifecycle job 117803017544 |
| Live same-version repair, update, rollback, key preservation, standard uninstall with persistent data retained | PASS, same lifecycle job |
| Attested manifest generation + exact tag/workflow verification | PASS, publish job 117803271551 |
| Public raw `curl ... | sudo bash -s -- --channel preview`, no GitHub login/token | PASS, public-install job 117803347885 |
| Public hidden password entry, transcript redaction, normal login and `/setup` closed | PASS, same public-install job |
| Failed download/API/missing Release, wrong checksum, unsafe archives, state detection, failed readiness/backup recovery, bounded APT approval/refusal | PASS in source-based tests; failures/APT/service recovery are mocked |

Preview.1 was deliberately not released: its real Linux lifecycle gate caught private release-directory permissions that blocked the unprivileged service. Preview.2 fixed code directory/file modes without weakening private config/key/database modes. Failed tags are retained; assets/tags were not overwritten.

Unexecuted or unresolved: Debian 12/13, Ubuntu 22.04 and arm64 runtime installation; actual missing-dependency APT/official CLI-repository installation (runner had dependencies); trusted remote TLS/firewall validation; real failure-injected systemd recovery (mock coverage exists); stable promotion and real RouterOS/VPN interoperability. Local WSL has no installed distribution, so actual Linux tests ran on disposable GitHub-hosted Ubuntu instead. Environment review/tag protections still need repository-owner review; no claim is made that protection rules were provisioned. The separate existing `quality` container job fails because `aquasecurity/trivy-action@0.33.1` does not resolve; it is independent of the passing Linux release workflow and remains unchanged.

Additional real signature regression: [systemd/public release run 39170628175](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39170628175), public-release job 117802728738, passed normal installation and accepted the actual signed manifest while rejecting a tampered copy. A local Windows CLI verification attempt could not initialize its public-good verifier; Linux public verification passed independently.

Final release: v0.2.0-preview.3, source commit e34686a. Release run 39170730251 completed successfully with both builds, live same-version repair/update/rollback lifecycle, attestation/publication, public piped install/login/redaction and actual manifest-tamper rejection. Preview.3 retains the validated Preview.2 permission fix and adds safe same-version repair. Documentation-only commits after the tag do not alter the runtime bootstrap.
