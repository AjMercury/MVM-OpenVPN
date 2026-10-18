# RouterOS compatibility and references

Reviewed 2026-10-18. Minimum design baseline is stable RouterOS 7.17 due to named OpenVPN servers. Only major version 7, minor ≥17, is eligible for the current adapter. Prerelease and unknown versions are rejected. Version eligibility does not certify every newer release.

| Capability | Implementation / gate | Live status |
|---|---|---|
| REST GET/PUT/PATCH/DELETE/POST | HTTPS, bounded requests, object/array parsing | NOT RUN |
| Named OpenVPN | stable 7.17+, readable server/certificate menus | NOT RUN |
| TCP / UDP, TUN, CBC / SHA256 | fixed validated options only | NOT RUN |
| Certificate template/sign/export | router-held PKI, encrypted leaf export | NOT RUN |
| File chunk reads | fail closed when unsupported or partial | NOT RUN |
| Firewall place-before | reviewed first-chain static anchors, read-back order | NOT RUN |
| Resource / model / architecture | system/resource observation | NOT RUN |
| WireGuard, L2TP, SSTP, PPTP | unavailable | NOT RUN |
| RouterOS Container | architectural preparation only | NOT RUN |

Menu reads are actual capability observations; they cannot prove support for every writable property. RouterOS rejects unsupported fields; the job stops and records failure/uncertainty. Certificate signing behavior, exported file naming and REST command argument selection need verification on the selected lab version. All such commands are documented families, not claimed proven against hardware.

Primary references:

* [REST API](https://manual.mikrotik.com/docs/developer-guides/rest-api/): verbs, JSON encoding, least-privilege policies, file reads, request bounds and rule placement.
* [OpenVPN](https://help.mikrotik.com/docs/spaces/ROS/pages/2031655/OpenVPN): server settings, version-dependent named servers, limited cipher negotiation and certificate requirements.
* [Certificates](https://manual.mikrotik.com/docs/authentication-authorization-accounting/certificates/): signing and PEM export; private-key export requires a passphrase and sensitive privilege.
* [Files](https://manual.mikrotik.com/docs/system-information-and-utilities/files/): bounded chunk reading and exact offsets.
* [Initial Meet AJ reference](https://meetaj.ir/articles/mikrotik-openvpn-setup-v7): requested baseline; this URL could not be retrieved in this environment. Its contents have **not** been verified or represented as applied.

Official documentation moved from help.mikrotik.com to manual.mikrotik.com; the old pages identify themselves as frozen. REST command execution is bounded by RouterOS (approximately 60 seconds historically); the app uses 20 seconds by default, configurable to 55 seconds. A timeout is treated as uncertain, without mutation retry.
