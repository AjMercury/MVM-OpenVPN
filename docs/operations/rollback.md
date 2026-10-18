> Edition update — 2026-10-19: Local Linux binary/database rollback is implemented separately from router-operation compensation: see [Linux recovery](../../editions/linux/README.md) and [migration rollback](edition-migration.md). Standard application uninstall preserves local data and never deletes router resources. The remainder describes router workflow recovery.

# Operation rollback and manual intervention

Do not delete pre-existing router resources. A job records each new resource's exact menu, `.id`, desired nonsecret values and ownership marker/name. Before compensation the app reads it back; changed or missing objects stop cleanup. Resources are removed in reverse dependency order. Definite HTTP 4xx rejection can compensate earlier resources. Transport failures, 408, malformed responses, signing uncertainty and read-back mismatches do not trigger blind automatic retries.

The server menu lacks a program-owned comment in the initial plan, so ownership uses the exact returned ID plus uniquely reserved desired configuration and durable job record. Other resources carry `mvm:<job-id>` comments. Name/ID alone is not a license to remove an externally modified object.

## Interrupted job

1. Keep writes disabled. Preserve the job and app/database backup. Do not re-submit the original request with another key.
2. Observe each recorded resource and the last in-flight menu using the management console or explicit read-only REST requests. A write may have committed without an ID being saved. Search the exact unique job name/comment, never a broad deletion pattern.
3. Identify dependencies, including externally added PPP profiles/users, certificate issuers, pools and server references. Current automatic compensation is not a complete cross-menu dependency analyzer; if other operators can modify concurrently, disable their changes during the authorized lab window. Do not approve deletion until dependency review is complete.
4. Choose completion, isolated manual compensation, or retaining the objects as unmanaged. There is currently no GUI to import changes or resolve MANUAL_INTERVENTION. An independently reviewed maintenance migration is required to mark a resolved job; never edit encrypted bbolt bytes directly.
5. Check firewall order, existing tunnels and permitted/forbidden traffic. Preserve a record of the actions and verification, then resolve the job through the maintenance procedure. The block is intentional until resolved.

## Firewall failure

Use out-of-band/console access; do not depend on the path being changed. Restore only the rules owned by this operation or the separately authorized router snapshot. Recheck original first-chain anchor IDs/order, FastTrack, raw/mangle/NAT and return paths. Existing established/fasttracked flows may have policy history; prove negative access using fresh test connections. Never conceal missing return routes with automatic masquerade.

## Uninstall

Stop/remove the application containers and proxy. Preserve or securely erase app data/keys under your retention policy. No application shutdown or uninstall command deletes RouterOS VPN services/users/certificates/rules. Removing the application does not revoke VPN credentials. Offboard explicitly on the router if that is the authorized goal.

