> Edition update — 2026-10-19: The maintained Lite packaging now lives in [editions/mikrotik](../../editions/mikrotik/README.md). This older planning document remains historical context; new templates/build files are separate from the Linux installer.

# Experimental Lite preparation

No RouterOS Container deployment is certified. Cross-compilation and OCI packaging are not hardware validation. First complete the Linux OpenVPN lab and benchmark before enabling Lite deployment work.

Candidates: linux/amd64 and linux/arm64. linux/arm/v5 is a separate **unverified** build target and is not included in CI. RouterOS container support depends on model, CPU architecture, release, installed Container package, device-mode and available storage/RAM. Verify those facts in current official documentation for the actual device. Do not infer that every arm router can run containers.

An operator must separately authorize installation/enablement of the package and device-mode (which can require physical confirmation), VETH/bridge/IP routing, firewall rules, image import, root directory and persistent mounts. This application never enables these. Store the database in an explicit persistent mount on suitable external/reliable storage; avoid wearing out small flash. Deliver master keys via secured configuration outside image/database. Use verified HTTPS exposure and non-root execution if supported by the selected runtime.

Targets, not measurements: compressed image ≤15 MB, idle RAM ≤25 MB, peak ≤64 MB. The default Argon2id password cost alone uses 32 MiB per authentication, up to two concurrent checks, so the Lite peak target may be exceeded. Never reduce security silently to reach a target.

Measure separately for each architecture/model/RouterOS build: image transfer bytes, root storage, database growth, idle RSS after five minutes, peak during login/sign/export, CPU latency, flash writes and restart recovery. Record whether each target is met. No numbers are fabricated in this release.

