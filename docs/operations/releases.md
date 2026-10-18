# Authenticated Linux releases

The 1.0 release path supports stable tags (`vX.Y.Z`) and explicit preview tags (`vX.Y.Z-preview.N`). Stable scope is the Linux manager; RouterOS automation remains experimental and disabled by default. Release publication and public-install results are recorded in [verification](../verification.md).

## Installation and recovery

```bash
curl -fsSL https://raw.githubusercontent.com/AjMercury/MVM-OpenVPN/master/install.sh | sudo bash
```

Use `sudo bash -s -- --channel preview` for a preview. Add `--version vX.Y.Z` to pin a published stable release, or `--action update` / `--action repair` for existing state. Never delete the master key or database to retry. The wizard requires a controlling terminal, prompts securely for credentials and from 1.0.2 defaults to automatic HTTPS on the detected server IP or supplied domain. Trusted TLS and explicit local-only access remain available. The configured TCP port must be allowed by the host/network firewall.

The default stable channel never silently installs a preview. Missing releases now fail before installing the GitHub verification tool. Preview acceptance is requested once; stable packages do not request preview acceptance. Download/verification errors retain a root-private directory under `/var/tmp/mvm-bootstrap-*`; verifier output is displayed and retained in `verification.log`. Preserve `/etc/mvm`, `/var/lib/mvm` and `/var/backups/mvm` after partial installation.

## Trust chain and the Sigstore HTTP 403 fix

The bootstrap is trusted through GitHub HTTPS and the selected branch/commit. For stronger operational control, review and pin that bootstrap commit. A compromised bootstrap can change all subsequent policy.

The CLI comes from the official signed GitHub APT repository after confirmation. Its keyring SHA-256 remains pinned to `6084d5d7bd8e288441e0e94fc6275570895da18e6751f70f057485dc2d1a811b`. Independent [GitHub CLI installation instructions](https://github.com/cli/cli/blob/trunk/docs/install_linux.md) publish that distribution method.

Some networks return HTTP 403 for `tuf-repo-cdn.sigstore.dev`, preventing the CLI's normal trust-root refresh. The installer instead obtains the official Sigstore public-good root from:

- Source: `sigstore/root-signing`, commit `63528388118c1002d1221321741159da24c256af`, `targets/trusted_root.json`.
- SHA-256: `6494e21ea73fa7ee769f85f57d5a3e6a08725eae1e38c755fc3517c9e6bc0b66`.

The pin is part of the reviewed bootstrap, independent of the application release. The root is checked before parsing and converted to JSONL for `gh attestation verify --custom-trusted-root`. This follows the CLI's [offline verification mechanism](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/verify-attestations-offline); it does not perform a live TUF refresh. Root updates require a reviewed pin change and regression verification of real bundles. Never trust an arbitrary root delivered next to an untrusted application archive.

Verification enforces the repository `AjMercury/MVM-OpenVPN`, exact workflow certificate identity `.github/workflows/release.yaml@refs/tags/TAG`, exact source tag, the default GitHub OIDC issuer, SLSA provenance and denial of self-hosted runners. The signed subject is `SHA256SUMS`; only then is the archive digest accepted. Safe extraction rejects links, traversal, duplicates, special files and oversized archives. No `--insecure`, signature bypass, unverified source build or checksum-only fallback exists.

## Publication gates

The tag workflow builds amd64 and arm64 Linux archives, runs Go tests/vet/vulnerability checks and Bash suites, and requires an actual Ubuntu 24.04 amd64 systemd lifecycle job. The latter installs a stable archive without preview confirmation, creates the first administrator and exercises repair, start/stop/restart, update, rollback and uninstall with key/data preservation. The publication job signs the aggregate checksum manifest and publishes the release only after those jobs pass. A separate anonymous public-install job then exercises real raw bootstrap, signed release assets, first login, closed browser setup and tampered-manifest rejection.

The quality workflow additionally runs race tests and container scanning. Protect the default branch, release workflow, tags and `linux-release` environment. Existing release tags/assets must not be overwritten.

Each archive contains VERSION, ARCH, SCHEMA and MATURITY metadata, server and management binaries, installer/systemd files and documentation. Build metadata and SPDX SBOMs accompany the archives. SBOM license review and real RouterOS validation remain separate responsibilities.

## Historical evidence

Preview `v0.2.0-preview.3`, commit `e34686a`, passed [release run 39170730251](https://github.com/AjMercury/MVM-OpenVPN/actions/runs/39170730251), including Ubuntu lifecycle and public installation. That result did not cover networks blocking the Sigstore CDN. A subsequent quality run identified quic-go 0.59.0 advisory GO-2026-5676 and an invalid Trivy action reference; 1.0 upgrades quic-go to 0.59.1 and uses Trivy action `v0.36.0` with scanner `v0.76.0`.
