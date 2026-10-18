# MikroTik Container Lite Edition

Development Preview: real RouterOS/container/VPN client interoperability remains NOT RUN. Production deployment is not recommended.

Same `cmd/server`, domain/application/RouterOS/OpenVPN logic, UI and security controls as Linux. Build tag `lite` selects the existing encrypted bbolt repository; the Lite server dependency graph excludes SQLite/Wazero. `cmd/mvmctl` is a Linux SQLite tool and is not shipped in Lite. The Bash installer/systemd unit do not apply here.

```sh
docker buildx build -f editions/mikrotik/Dockerfile --platform linux/arm64 \
  --build-arg VERSION=0.2.0-dev --output type=oci,dest=mvm-mikrotik-arm64-0.2.0-dev.tar .
```

This command is a build instruction, not an executed image result. Independently cross-compiled Linux amd64/arm64 Lite server binaries exist locally. OCI build/import, RAM/CPU and image size measurements have not run. No armv5 artifact is claimed. The build-only workflow never publishes an image. Distribution requires per-architecture build/security checks plus authorized device/runtime compatibility evidence.

Official [Container manual](https://manual.mikrotik.com/docs/containers/) requires the container package, enabled device capability and appropriate hardware/storage. The [architecture matrix](https://help.mikrotik.com/docs/spaces/ROS/pages/146440194/Feature%20support%20based%20on%20architecture) and manual distinguish ARM, ARM64 and x86; EN7562CT devices such as hEX Refresh require arm32v5 images. An ARM label alone is not sufficient. Check exact model/CPU/RouterOS release and supported image format before using a template. No hardware setting is enabled automatically.

Review `routeros/install.rsc` before import. Supply every `mvm*` global, provision a secure envlist and existing persistent data/secrets storage, validate VETH addresses/gateway, image architecture and management ACLs, then explicitly set `mvmApproved`. Templates refuse missing approval/configuration and existing tracked containers/interfaces. They do not replace bridge, NAT or firewall settings and do not automatically start the new container. Scripts are unverified on RouterOS and may need version-specific syntax adjustment, including mount property differences. Apply only in an explicitly authorized disposable lab first.

Use `/data/manager.db`, externally provisioned `/secrets/master.key`, `/secrets/bootstrap.token`, TLS PEM files and the paths in `config/container.env.example`. The mount must be private and writable by UID 65532 for data; secrets readable but not application-writable where the RouterOS version supports mount restrictions. Never store the actual master key/REST passwords in image layers or RouterOS script globals/envlist. RouterOS envlist may contain only paths/nonsecret settings. Supply certificate trust and exact origin before remote access. Explicit `MVM_ROUTER_CIDRS` must restrict router management destinations; empty denies all. Keep router writes disabled until separate authorization. No plaintext public setup listener is configured by the scripts.

## Versioned updates and rollback

Stop the exact owned container. Verify an encrypted bbolt transaction backup and original external master key through the existing server backup/restore/check commands in an isolated compatible environment. Preserve original data and old image/root/config. `update.rsc` validates explicit candidate image/root/name/VETH/envlist/mounts, stops the owned container, disables its automatic start and stages a separately tracked candidate with automatic start disabled. Provision a separate reviewed candidate VETH first and perform offline database compatibility/rehearsal. Never run two containers against the same bbolt file. Rollback means stop the replacement, preserve its database, verify the old matching backup/key, restore to a new persistent location, and start only the previous image/container with its compatible database. Router writes remain disabled during reconciliation. This procedure is documented, not validated on hardware.

`uninstall.rsc` requires separate application-only approval and an exact ownership comment identifying one container. It stops/removes that container only, retains data/secrets/mount/network definitions for reviewed cleanup, and never calls VPN/firewall/certificate menus. Names alone are not enough to authorize deleting shared networking.

Resource goals remain compressed image <=15 MB, idle <=25 MB RAM, peak <=64 MB RAM, **unmeasured and not guaranteed**. Existing Argon2id password verification alone allocates 32 MiB per concurrent verification, so do not promise those RAM goals. Measure exact architecture/image format/RouterOS/workload and five-minute idle/peak samples before stating results. Linux release delivery is independent of Lite optimization/runtime completion.

