> Edition update — 2026-10-19: For Linux service failures use journalctl -u mvm, mvm status and the [edition recovery guide](../../editions/linux/README.md). A legacy-format/unknown-schema/key error must stop startup; do not remove data or initialize a replacement database. Docker/systemd/RouterOS runtime results remain unverified locally.

# Troubleshooting

Diagnose from observed evidence. The current UI shows exact RouterOS snapshots and bounded transport categories; it does not automatically establish root cause.

| Evidence | Check / expected result | Interpretation and action |
|---|---|---|
| TLS transport fails | Verify private CA, SAN, clock, expiry with your PKI tools | Repair certificate trust/identity; never disable verification |
| DNS resolution fails | Resolve management/public hostname from the relevant network | Fix DNS or management routing; no guessed status |
| Destination forbidden | Every resolved IP must be in MVM_ROUTER_CIDRS and not loopback/link-local | Review intended management destination; don't allow 0.0.0.0/0 as a shortcut |
| HTTP 401/403 | Test dedicated service account privileges through authorized console | Distinguish credentials from permission failure; never log password |
| HTTP 429/timeout | Inspect router load/storage and API limits | Reduce requests. Mutation may already have committed; review the job |
| Preflight overlap | Inspect ip/address, ip/route and ip/pool | Choose another subnet; unparsable/incomplete observed ranges fail closed |
| Existing OpenVPN/PPP records | Inspect shared authentication scope | One managed server only; don't delete manual accounts automatically |
| Certificate sign/export/read failure | Inspect cert metadata, file sizes, sensitive/ftp/test privileges and storage | Do not deliver partial files; clean only exact random export names |
| Profile cannot import | Record OS, client build, encrypted key format and cipher support | This release has no certified client; use the lab plan |
| Tunnel connects but resource fails | Check allowed CIDR, input restrictions, DNS route, return route, raw/mangle and NAT | No blanket LAN accept or masquerade workaround is added |
| Forbidden destination reachable | Test a new connection, inspect FastTrack/connection history and firewall order | Stop rollout and restore reviewed policy; read-back isn't an access test |
| Disabled user remains connected | Inspect ppp/active and job/action result | Disable and disconnect are distinct; verify no reconnection |
| Stale data | Last successful sync older than two minutes | Explicitly sync; enabled is not connected |
| Router reset/same IP/new hardware | Compare certificate identity, architecture/model and mappings | Do not provision until replacement is explicitly reviewed |
| No public inbound access | Check CGNAT, public IP, DNS and upstream port forwarding | A RouterOS listener alone cannot traverse CGNAT |

Application errors carry `X-Request-ID`; structured logs contain method, route pattern, response status and latency. Avoid logging profile contents, tokens, passwords or export passphrases in reverse proxies or packet captures. Health endpoints don't prove VPN functionality. `/metrics` requires an authenticated administrator/auditor and intentionally contains no credential/IP labels.

For Linux resource measurement after the functional lab, record build flags, image compressed size by architecture, `docker stats --no-stream` idle/peak RSS, workload and five-minute samples. Argon2id allocates at least 32 MiB per password verification; Lite RAM goals are not currently met or claimed. See Lite notes.

