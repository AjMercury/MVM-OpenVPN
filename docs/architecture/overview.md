> Edition update — 2026-10-19: Current edition boundaries, SQLite schema/driver choice and WAL/backup behavior are documented in [editions](editions.md). The domain and workflow description below remains shared; bbolt-only storage references apply to Lite/v0.1.0.

# Architecture

The application is one Go process and one bbolt file. Gin and HTML presentation depend on application services; services depend on domain records, a small encrypted repository interface, and a RouterOS `API` transport interface. OpenVPN validation and planning are separate from executing a persisted plan. No speculative packages or unsupported protocol adapters are exposed.

Desired state: `servers`, `users`, and approved `jobs`. Observed state: the last explicit router sync plus live read-back during operations. A normalized configuration hash flags changes between syncs. This is detection, not a full semantic diff/import/reconcile engine. The app never automatically repairs WinBox changes. Telemetry is not part of the hash; router observations older than two minutes are stale.

All domain records are encrypted, including secrets, sessions and delivery bytes. Schema 1 is initialized transactionally; unknown versions fail startup. The sentinel prevents startup with an incorrect recovery key. bbolt is single-writer and single-instance; do not share the database across replicas or network filesystems.

## Provisioning

Validate local input → observe router prerequisites → persist READY preview → explicit confirmed Apply → persist APPLYING with an in-flight intent before each request → record returned resource IDs → VERIFYING → read back desired values and firewall order → persist server/user metadata → COMPLETED. A completed job ID is replay-safe. Keys are reserved per router; duplicate keys identify the original job instead of starting another plan.

Definite HTTP client rejections may trigger reverse-order compensation, checking current values before deletion. Transport failures and restart interruptions become MANUAL_INTERVENTION; no automatic mutation retry occurs. Another provisioning operation is blocked on that router. See rollback limitations before using the adapter on a lab.

## UI

The local `SKILL.md` and `DESIGN.md` remain unchanged and are authoritative. Ember & Slate tokens, white-neutral translucent surfaces, inset highlights, warm background blobs, pills, raw stroke SVGs and local Outfit/DM Sans fonts are used. Tables and technical output are opaque. Normal operational text is raised above the skill's low-opacity examples for readability. English and Persian dictionaries are embedded; technical values use `bdi` / LTR blocks. No runtime CDN or external QR service is involved.

HTMX is vendored but primary actions use ordinary server forms for progressive enhancement. JavaScript only handles appearance, clipboard, fragment delivery and HTMX CSRF headers. Bootstrap is not included because these components use the authoritative local CSS without needing a second design layer.

