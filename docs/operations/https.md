# Panel access and optional HTTPS

The 1.0.3 installer defaults to `http://SERVER_IP:PORT`, binding IPv4 interfaces and detecting each host's own addresses. No certificate is required. HTTP sends credentials and session traffic without encryption; choose HTTPS when you need encrypted access. Neither changing the protocol nor issuing a certificate opens firewall ports automatically.

After updating an existing installation, `sudo mvm access --http` detects the server IP and keeps the existing port. Append a domain to accept both it and the server's assigned IPv4 addresses. `sudo mvm access` selects a self-signed certificate instead; verify/import its printed fingerprint. The menu shows the current protocol, port and URL.

## Public certificate

Run `sudo mvm ssl` or choose **17) SSL Certificates**. Enter the domain or public IPv4. Private addresses such as `172.16.x.x` cannot receive a public certificate: use HTTP, local self-signed HTTPS, or a domain resolving to a publicly reachable server.

Choose standalone when public TCP port 80 reaches this server and is unused. If Nginx/Apache already uses port 80, choose webroot and provide that virtual host's document root. Its `/.well-known/acme-challenge/` path must be publicly served from that directory. MVM does not stop existing web servers or edit their configuration.

CLI equivalents (replace the example values):

```bash
sudo mvm ssl issue panel.example.com
sudo mvm ssl issue panel.example.com /var/www/html
sudo mvm ssl renew
sudo mvm ssl status
```

Domain DNS and any NAT forwarding must point to this server. An IP certificate requires a public IPv4 reachable on port 80; the client requests Let's Encrypt's `shortlived` profile and checks renewal every three days. The systemd timer runs checks twice daily with a randomized delay. Public IPv6 certificate input is not implemented in this version.

Certificates are validated for key match, hostname/IP SAN, expiry and system trust before deployment. Deployment backs up the database, saves the prior configuration and rolls back if the service fails readiness. Issuance failure keeps existing panel access. After public-certificate deployment, use its matching domain/IP URL; other IP aliases are removed because they are not covered by that certificate.

Existing PEM files can be imported through menu 10 or `sudo mvm tls CERT KEY https://HOST:CURRENT_PORT`.

## Renewal and trust

The official [acme.sh](https://github.com/acmesh-official/acme.sh) client is downloaded only when requested, pinned to commit `37d064f7b580055d3f2ddf9a9c960f754f3694d6` and SHA-256 `fa4842619ddf50f774d79092ae06887a2107c660757283b763bef834dfc150ff`. A hash mismatch prevents execution. No remote shell pipe, automatic client upgrade, user-profile change or cron installation is used.

Account state and exported keys live in root-only `/etc/mvm/acme`; only serving copies are readable by the unprivileged panel. `mvm-cert-renew.timer` runs `/opt/mvm/current/editions/linux/installer/acme.sh renew`. Inspect failures with `journalctl -u mvm-cert-renew.service`. Returning to HTTP or importing another certificate prevents renewal from overriding that explicit access change. Uninstall disables/removes the timer while preserving certificate/account data unless purge is requested.

The workflow was independently implemented after reviewing [3x-ui](https://github.com/MHSanaei/3x-ui/tree/a8d0700bf2ed06d77c077fc45fc0387d9abdbaab). No 3x-ui code was copied. Let's Encrypt documents [public IP and six-day certificate availability](https://letsencrypt.org/2026/01/15/6day-and-ip-general-availability/).

Live CA issuance needs an authorized public domain/IP and is not proven by mocked tests. Release validation covers native HTTP/self-signed HTTPS, transitions, port preservation and simulated certificate failures/renewal.
