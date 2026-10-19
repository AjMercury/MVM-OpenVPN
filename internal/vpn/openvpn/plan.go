package openvpn

import (
	"context"
	"errors"
	"fmt"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/ipam"
	"mikrotik-vpn-manager/internal/routers"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

var Identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,26}$`)

func Validate(s domain.ServerSpec) error {
	if !Identifier.MatchString(s.Name) {
		return errors.New("name: use 1–27 English letters, numbers, dots, underscores or hyphens; start with a letter or number")
	}
	if !routers.ValidHost(s.Endpoint) {
		return errors.New("endpoint: enter a domain or IP address without https://, a port or a path")
	}
	if s.PublicIP != "" {
		a, e := netip.ParseAddr(s.PublicIP)
		if e != nil || !a.IsGlobalUnicast() || a.IsPrivate() {
			return errors.New("public_ip: enter a public IP address")
		}
	}
	if s.Port < 1024 || s.Port > 65535 || s.Transport != "tcp" && s.Transport != "udp" {
		return errors.New("use TCP or UDP on an unprivileged port 1024–65535")
	}
	if e := ipam.Range(s.Subnet, s.Start, s.End, s.Gateway); e != nil {
		return e
	}
	if len(s.DNS) == 0 || len(s.DNS) > 3 {
		return errors.New("provide 1–3 IPv4 DNS servers")
	}
	for _, raw := range s.DNS {
		a, e := netip.ParseAddr(raw)
		if e != nil || !a.Is4() {
			return errors.New("invalid DNS address")
		}
	}
	if len(s.Allowed) == 0 || len(s.Allowed) > 32 {
		return errors.New("provide 1–32 explicit allowed IPv4 networks; full tunnel is unavailable")
	}
	p, _ := netip.ParsePrefix(s.Subnet)
	for _, raw := range s.Allowed {
		q, e := netip.ParsePrefix(raw)
		if e != nil || !q.Addr().Is4() || q.Bits() == 0 || q != q.Masked() || p.Overlaps(q) {
			return errors.New("allowed destinations must be canonical IPv4 CIDRs, distinct from the VPN subnet and not a default route")
		}
	}
	if s.InputAnchor == "" || s.ForwardAnchor == "" {
		return errors.New("select first input and forward firewall rule IDs as reviewed anchors")
	}
	return nil
}
func Preflight(ctx context.Context, c routers.API, s domain.ServerSpec) error {
	if e := Validate(s); e != nil {
		return e
	}
	snapshot, e := routers.Discover(ctx, c)
	if e != nil {
		return e
	}
	if !snapshot.Capabilities["openvpn_baseline"] {
		return errors.New("RouterOS stable 7.17+ and observable OpenVPN/certificate menus required")
	}
	servers, e := routers.List(ctx, c, "interface/ovpn-server/server")
	if e != nil {
		return e
	}
	if len(servers) > 0 {
		return errors.New("MVP requires an empty OpenVPN server menu; review existing servers manually")
	}
	users, e := routers.List(ctx, c, "ppp/secret")
	if e != nil {
		return e
	}
	for _, u := range users {
		if u["disabled"] != "true" && u["disabled"] != "yes" && (u["service"] == "ovpn" || u["service"] == "any") {
			return errors.New("existing PPP identities may authenticate to OpenVPN; isolate before provisioning")
		}
	}
	return NetworkPreflight(ctx, c, s)
}

// NetworkPreflight checks address allocation and reviewed firewall anchors.
func NetworkPreflight(ctx context.Context, c routers.API, s domain.ServerSpec) error {
	addresses, e := routers.List(ctx, c, "ip/address")
	if e != nil {
		return e
	}
	routes, e := routers.List(ctx, c, "ip/route")
	if e != nil {
		return e
	}
	pools, e := routers.List(ctx, c, "ip/pool")
	if e != nil {
		return e
	}
	if e = ipam.Overlap(s.Subnet, addresses, routes, pools); e != nil {
		return e
	}
	rules, e := routers.List(ctx, c, "ip/firewall/filter")
	if e != nil {
		return e
	}
	for chain, anchor := range map[string]string{"input": s.InputAnchor, "forward": s.ForwardAnchor} {
		found := false
		for _, r := range rules {
			if r["chain"] != chain {
				continue
			}
			if r[".id"] != anchor || r["dynamic"] == "true" || r["dynamic"] == "yes" {
				return fmt.Errorf("%s anchor must be the first static rule in its chain", chain)
			}
			found = true
			break
		}
		if !found {
			if anchor != "__empty__" {
				return errors.New("missing firewall anchor")
			}
		}
	}
	// Other listening services, NAT, raw, mangle and return paths require explicit operator review.
	return nil
}
func Plan(id string, s domain.ServerSpec) *domain.Job {
	prefix := "mvm-" + id[:12]
	marker := "mvm:" + id
	server := &domain.Server{ID: id, Spec: s, JobID: id, CA: prefix + "-ca", Certificate: prefix + "-server", Pool: prefix + "-pool", Profile: prefix + "-profile"}
	j := &domain.Job{ID: id, RouterID: s.RouterID, Kind: "server", State: "READY", Server: server}
	add := func(path string, v map[string]string) {
		j.Steps = append(j.Steps, domain.Step{Path: path, Method: "PUT", Values: v})
	}
	sign := func(name, ca string) {
		v := map[string]string{"numbers": name, "name": name}
		if ca != "" {
			v["ca"] = ca
		}
		j.Steps = append(j.Steps, domain.Step{Path: "certificate/sign", Method: "POST", Values: v})
	}
	add("certificate", map[string]string{"name": server.CA, "common-name": server.CA, "key-size": "2048", "days-valid": "3650", "key-usage": "key-cert-sign,crl-sign", "trusted": "yes", "comment": marker})
	sign(server.CA, "")
	san := "DNS:" + s.Endpoint
	if a, e := netip.ParseAddr(s.Endpoint); e == nil && a.IsValid() {
		san = "IP:" + s.Endpoint
	}
	add("certificate", map[string]string{"name": server.Certificate, "common-name": s.Endpoint, "subject-alt-name": san, "key-size": "2048", "days-valid": "365", "key-usage": "digital-signature,key-encipherment,tls-server", "comment": marker})
	sign(server.Certificate, server.CA)
	add("ip/pool", map[string]string{"name": server.Pool, "ranges": s.Start + "-" + s.End, "comment": marker})
	add("ppp/profile", map[string]string{"name": server.Profile, "local-address": s.Gateway, "remote-address": server.Pool, "dns-server": strings.Join(s.DNS, ","), "use-encryption": "required", "only-one": "yes", "comment": marker})
	rule := func(v map[string]string, anchor string) {
		v["comment"] = marker
		if anchor != "__empty__" {
			v["place-before"] = anchor
		}
		add("ip/firewall/filter", v)
	}
	rule(map[string]string{"chain": "input", "src-address": s.Subnet, "action": "drop"}, s.InputAnchor)
	rule(map[string]string{"chain": "input", "protocol": s.Transport, "dst-port": strconv.Itoa(s.Port), "action": "accept"}, s.InputAnchor)
	rule(map[string]string{"chain": "forward", "src-address": s.Subnet, "dst-address": s.Subnet, "action": "drop"}, s.ForwardAnchor)
	for _, dst := range s.Allowed {
		rule(map[string]string{"chain": "forward", "src-address": s.Subnet, "dst-address": dst, "action": "accept"}, s.ForwardAnchor)
	}
	rule(map[string]string{"chain": "forward", "src-address": s.Subnet, "action": "drop"}, s.ForwardAnchor)
	rule(map[string]string{"chain": "forward", "dst-address": s.Subnet, "connection-state": "established,related", "action": "accept"}, s.ForwardAnchor)
	rule(map[string]string{"chain": "forward", "dst-address": s.Subnet, "action": "drop"}, s.ForwardAnchor)
	p, _ := netip.ParsePrefix(s.Subnet)
	add("interface/ovpn-server/server", map[string]string{"name": s.Name, "certificate": server.Certificate, "default-profile": server.Profile, "port": strconv.Itoa(s.Port), "protocol": s.Transport, "mode": "ip", "netmask": strconv.Itoa(p.Bits()), "auth": "sha256", "cipher": "aes256-cbc", "tls-version": "only-1.2", "require-client-certificate": "yes", "redirect-gateway": "disabled", "disabled": "no"})
	return j
}
func UserPlan(id string, s domain.Server, u domain.VPNUser, password string) *domain.Job {
	marker := "mvm:" + id
	u.ID = id
	u.JobID = id
	u.Certificate = "mvm-" + id[:12] + "-client"
	return &domain.Job{ID: id, RouterID: u.RouterID, Kind: "user", State: "READY", User: &u, Steps: []domain.Step{
		{Path: "certificate", Method: "PUT", Values: map[string]string{"name": u.Certificate, "common-name": u.Username, "key-size": "2048", "days-valid": "365", "key-usage": "tls-client", "comment": marker}},
		{Path: "certificate/sign", Method: "POST", Values: map[string]string{"numbers": u.Certificate, "name": u.Certificate, "ca": s.CA}},
		{Path: "ppp/secret", Method: "PUT", Secret: true, Values: map[string]string{"name": u.Username, "password": password, "profile": s.Profile, "service": "ovpn", "remote-address": u.Address, "disabled": "no", "comment": marker}},
	}}
}
