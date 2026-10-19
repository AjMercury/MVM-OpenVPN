# VPN server forms and monitoring

The server form has separate entry points for OpenVPN, WireGuard, SSTP,
L2TP/IPsec and IKEv2. Select and inspect the router first, complete the settings, then save the
preview. The final job review retains the existing explicit apply confirmation
and `MVM_ALLOW_MUTATIONS` gate. No router is changed while editing a form.

A supplied domain is used as the endpoint and DNS SAN. With no domain, the
public IP becomes the endpoint and IP SAN. These are private-CA certificates;
this workflow does not issue publicly trusted ACME certificates. WireGuard
generates keys on the router and does not create certificates.

Jobs create and verify certificate signatures before configuring dependencies,
install firewall restrictions before listeners, and verify the observed firewall
order before activation. SSTP singleton settings are captured in the preview,
checked again before writes and restored during a safe rollback. Existing enabled
SSTP services and existing IPsec peers are rejected for manual conflict review.

WireGuard peers receive distinct X25519 key pairs. Client private keys are
stored in an encrypted collection, never in job responses or router peer
commands. Operators and assigned portal users can export native `.conf` files
and a QR containing the actual WireGuard configuration. Downloads remain
authenticated, single-use and valid for five minutes. A scanned configuration
is a credential and remains usable until its peer is disabled.

Users now share one menu and may have multiple service bindings under a single
identity. New shared PPP users use one username/password and PPP profile for the
managed OpenVPN, SSTP and L2TP listeners on the same router. When OpenVPN exists,
its pool and access policy are primary. Create the desired PPP servers before
shared users; provisioning another PPP listener while shared users are active is
blocked to prevent an unreviewed authentication expansion. PPTP and PPPoE must be
disabled and observable. WireGuard is added through the existing user's Add
Service action and retains a distinct private key/IP. Existing users are not
automatically converted. OpenVPN exports an
inline `.ovpn` file with encrypted client key material; SSTP and L2TP/IPsec use
their PPP credentials. PPP identities still share RouterOS's username namespace;
WireGuard has a separate router namespace but can share the panel identity. Each protocol has its own server identity
in SQLite while legacy OpenVPN records retain their original uniqueness rule.

IKEv2 currently provisions server infrastructure only. Its identity deliberately remains disabled until client
certificates and restrictive identity matching are configured by an operator.
OpenVPN's existing user enrollment and profile delivery remain available.
SSTP uses MSCHAPv2 with server TLS and does not require a client certificate,
which permits Windows SSTP clients after trusting the private CA.
L2TP uses MSCHAPv2 with `use-ipsec=required`, opens IKE/NAT-T and ESP, accepts
UDP 1701 only under an inbound IPsec policy and explicitly drops plaintext L2TP.
The previous disabled singleton settings must be observable and its old PSK
empty before provisioning; the panel does not overwrite existing secrets.
PSKs are redacted from previews and scrubbed from terminal job payloads.

Monitoring displays scoped router samples, CPU, memory in bytes, uptime, drift
and recent jobs. Samples older than two minutes are marked stale; failed samples
retain offline status. Sync collects a new sample. Active PPP connections,
WireGuard peer counters/handshakes and active IKEv2 peers can be viewed separately.
The graphs cover CPU, used RAM/disk percentages, physical-interface RX/TX rates,
PPP sessions, recent WireGuard handshakes, active IPsec peers, reachability and
job states. Up to 120 samples per router are persisted. Opening Monitoring enables
30-second sampling for routers the account can sync, while the page is visible;
the checkbox pauses collection. Read-only roles can view scoped histories without
collecting. Unknown counters remain null, rate calculations require two samples,
and counter resets or sampling gaps are not plotted as invented traffic.

Local mock tests verify the plans and execution guards. Real RouterOS/client
interoperability still requires the authorized lab described in
`tests/lab/README.md`.

## Network and NAT selections

The **Network & NAT** sidebar entry and Settings link open a separate reviewed
configuration form. Load a router first; sources are completed panel VPN subnets,
the router's existing IPv4 pools, and enabled interface address networks. Only
enabled, currently existing interfaces can be selected as the output. There are
no free-text source-range or output-interface fields. Pool start/end ranges are
decomposed into exact CIDRs rather than widened to an enclosing subnet.

The default creates only a scoped `srcnat` masquerade rule. An explicit checkbox
adds a forwarding pair for outbound traffic through the selected interface and
established return connections. IPsec outbound traffic is excluded from
masquerade; client profile routes are unchanged. Router-administrator permission,
the existing mutation gate and final job confirmation are required.

The preview records the selected resource identities, source addresses and first
static NAT/forward anchors. Apply rereads the router and rejects removed, disabled
or renamed interfaces, changed pool ranges, or moved anchors. A job-owned address
list scopes the rules; its entries and rule order are verified before permitting
outbound forwarding. Existing managed VPN rules remain verified and usable after
an approved NAT job, while unapproved preceding rules still block enrollment.

Command/property references:

- [WireGuard](https://help.mikrotik.com/docs/spaces/ROS/pages/69664792/WireGuard)
- [SSTP](https://help.mikrotik.com/docs/spaces/ROS/pages/2031645/SSTP)
- [L2TP/IPsec](https://help.mikrotik.com/docs/spaces/ROS/pages/2031631/L2TP)
- [IKEv2 RSA road warrior](https://help.mikrotik.com/docs/spaces/ROS/pages/11993097/IPsec#IPsec-RoadWarriorsetupusingIKEv2withRSAauthentication)
