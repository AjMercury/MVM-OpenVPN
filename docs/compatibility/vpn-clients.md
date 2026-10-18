# Client provisioning matrix

No client platform is currently marked supported. The code generates a unified OpenVPN profile with embedded CA, client certificate and encrypted private key, explicit TLS identity verification, CBC cipher/data-ciphers, SHA256 and explicit split routes. Passwords are prompted, never embedded. A ZIP bundle function exists in the protocol package; it is not exposed as a download option yet.

| Protocol | RouterOS | OS | Client | Format | QR | Certificates/auth | Status / limitation |
|---|---|---|---|---|---|---|---|
| OpenVPN | 7.17+ target | Windows | OpenVPN Connect / Community (exact version pending) | .ovpn | authenticated download URL | CA + client cert + encrypted key + PPP password | NOT RUN |
| OpenVPN | 7.17+ target | macOS | OpenVPN Connect (version pending) | .ovpn | download URL | same | NOT RUN |
| OpenVPN | 7.17+ target | Linux | OpenVPN 2.6 candidate | .ovpn | download URL | same | NOT RUN |
| OpenVPN | 7.17+ target | Android | OpenVPN Connect (version pending) | .ovpn | download URL | same | NOT RUN |
| OpenVPN | 7.17+ target | iOS | OpenVPN Connect (version pending) | .ovpn | download URL | same | NOT RUN |
| WireGuard | undetermined | all | undetermined | planned .conf | planned native QR | keys | UNIMPLEMENTED |
| L2TP/IPsec | undetermined | platform-specific | undetermined | guided platform instructions planned | none claimed | IPsec secrets | UNIMPLEMENTED |
| SSTP | undetermined | Windows primarily | undetermined | guided profile/script planned | none claimed | TLS trust + credentials | UNIMPLEMENTED |
| PPTP | undetermined | legacy only | undetermined | management planned | none claimed | insecure legacy protocol | UNIMPLEMENTED / disabled |

An OpenVPN download QR does not automatically configure any client. Browser authentication and assigned ownership are required. DNS option behavior differs by OS and must be tested; on Linux additional integration may be needed. The selected endpoint must match the server certificate CN/SAN. Never bypass TLS checks to fix import failures.

For each actual test record: RouterOS build, model/architecture, client exact version, OS build, transport, profile SHA256 (not bytes), key format, import result, assigned IP, allowed/forbidden destination tests, disable/disconnect result and cleanup. Never put key material or passwords in test reports.
