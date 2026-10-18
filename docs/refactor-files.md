# Refactor file inventory — 2026-10-19

Workspace: `D:\MikroTik VPN Manager`. No Git metadata was supplied; this is a task inventory rather than a Git-generated diff. Original data/key files were not opened or migrated.

New implementation files:

```text
install.sh
cmd/server/storage_full.go
cmd/server/storage_lite.go
cmd/mvmctl/main.go
cmd/mvmctl/main_test.go
internal/config/edition_full.go
internal/config/edition_lite.go
internal/http/setup.go
internal/http/setup_test.go
internal/storage/repository.go
internal/storage/legacy.go
internal/storage/contract_test.go
internal/storage/bbolt/repository.go
internal/storage/sqlite/repository.go
internal/storage/sqlite/references.go
internal/storage/sqlite/repository_test.go
internal/storage/migration/migrate.go
internal/storage/migration/migrate_test.go
web/templates/setup.html
editions/linux/installer/common.sh
editions/linux/installer/install.sh
editions/linux/installer/menu.sh
editions/linux/installer/update.sh
editions/linux/installer/uninstall.sh
editions/linux/systemd/mvm.service
editions/linux/docker/Dockerfile
editions/linux/docker/compose.yaml
editions/linux/config/container.env.example
editions/linux/config/release.env.example
editions/linux/README.md
editions/mikrotik/Dockerfile
editions/mikrotik/routeros/install.rsc
editions/mikrotik/routeros/update.rsc
editions/mikrotik/routeros/uninstall.rsc
editions/mikrotik/config/container.env.example
editions/mikrotik/README.md
scripts/package-linux.sh
scripts/sbom.go
tests/installer/run.sh
tests/installer/check.sh
tests/installer/preflight.sh
tests/installer/archive.sh
tests/installer/recovery.sh
tests/installer/uninstall.sh
tests/systemd/disposable-vm.sh
.github/workflows/linux-edition.yaml
.github/workflows/mikrotik-edition.yaml
.github/workflows/systemd-edition.yaml
docs/architecture/editions.md
docs/operations/edition-migration.md
docs/refactor-files.md
```

Existing files updated:

```text
go.mod
go.sum
cmd/server/main.go
internal/config/config.go
internal/app/service.go
internal/app/integration_test.go
internal/http/server.go
web/templates/layout.html
web/locales/en.json
web/locales/fa.json
Dockerfile
compose.yaml
.env.example
scripts/build.ps1
scripts/check-manifests.go
README.md
DELIVERY.md
docs/architecture/overview.md
docs/architecture/decisions.md
docs/security/secrets-and-pki.md
docs/security/threat-model.md
docs/operations/installation.md
docs/operations/backup-restore.md
docs/operations/upgrade.md
docs/operations/rollback.md
docs/operations/troubleshooting.md
docs/limitations.md
docs/verification.md
docs/implementation-record.md
deployments/mikrotik/README.md
```

`SKILL.md`, `DESIGN.md`, CSS/fonts/HTMX/JavaScript, RouterOS REST transport, VPN planning, certificate/profile implementation, security primitives and the legacy bbolt implementation remain intact. Existing formatting was normalized by gofmt where applicable.

Generated outputs are under ignored `artifacts/editions/` (Linux archives, checksums, SPDX binary dependency SBOMs, build metadata, Lite server binaries and manifest). Test logs are `artifacts/editions-tests.log`, `artifacts/editions-lite-tests.log`, `artifacts/editions-shell-tests.log`, `artifacts/editions-manifests.log` and `artifacts/editions-vet.log`. No artifact is claimed published or production certified.


