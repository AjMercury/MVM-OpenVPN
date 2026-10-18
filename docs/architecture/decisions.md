> Edition update — 2026-10-19: Storage/deployment decisions are superseded for Linux by [the two-edition architecture](editions.md): SQLite on Linux; compatible bbolt on Lite. Existing domain/protocol/security decisions remain in force.

# Decisions

1. **One managed OpenVPN server per router.** Named servers do not isolate the shared PPP secret namespace. Preflight rejects existing server records and enabled `ovpn`/`any` PPP accounts. This conservative requirement can block factory/default configurations; the application never removes them automatically.
2. **Fixed initial crypto.** TUN, AES-256-CBC, SHA256, TLS 1.2 and client certificates; TCP/UDP only. No compression, insecure ciphers, full tunnel, VRF, IPv6 or speculative negotiation settings. New combinations need documented lab tests.
3. **Router-held PKI.** CA and signing remain on RouterOS. No CA private-key export. Client keys are exported only encrypted with a user-provided passphrase. The backend retrieves actual file bytes and attempts exact-name temporary cleanup.
4. **Reserved addresses.** Even automatic assignment reserves a static address in the configured pool for deterministic tracking. Disabled identities retain reservations. Pending previews also reserve usernames and addresses until they expire or roll back.
5. **Explicit firewall anchors.** The wizard inspects the selected router and fills the first static rule IDs in input/forward (or an explicit empty-chain marker); no terminal command is needed. The operator reviews the displayed rules. New rules are placed first and read-back checked. Source-subnet allowlists, client isolation, input denial and return-path restrictions avoid blanket LAN access. Raw/mangle/NAT, FastTrack and route correctness still need independent review and negative connectivity testing.
6. **No automatic mutation retry.** HTTP timeout may follow a committed write. Persist intent and stop for manual intervention. Router outages never intentionally disable established VPN services.
7. **Authenticated delivery capabilities.** Random tokens in URL fragments, POST consumption, five-minute TTL, one-time transactional consumption, separate portal accounts and explicit current authorization. A QR is a URL to an authenticated download page, not native OpenVPN provisioning.
8. **Honest release scope.** A generated profile is an implemented format, not a certified client import. WireGuard and other protocols stay unavailable until the first vertical slice passes actual lab tests.

