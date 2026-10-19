// Package protocols supplies distinct RouterOS server plans. Client delivery is
// provided by the OpenVPN and WireGuard workflows.
package protocols

import (
	"context"
	"errors"
	"fmt"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/vpn/openvpn"
	"net/netip"
	"strconv"
	"strings"
)

func Name(s domain.ServerSpec) string {
	if s.Protocol == "" {
		return "openvpn"
	}
	return s.Protocol
}

func Validate(s domain.ServerSpec) error {
	p := Name(s)
	if s.IPsecSecret != "" && p != "l2tp" {
		return errors.New("IPsec shared secrets are accepted only for L2TP/IPsec")
	}
	if p != "openvpn" && p != "wireguard" && p != "sstp" && p != "ikev2" && p != "l2tp" {
		return errors.New("protocol: select OpenVPN, WireGuard, SSTP, IKEv2 or L2TP/IPsec")
	}
	check := s
	// RouterOS does not impose a privileged-port restriction. SSTP uses 443;
	// IKEv2 uses the fixed IKE/NAT-T ports 500 and 4500.
	if p == "sstp" || p == "ikev2" || p == "l2tp" {
		check.Port = 1194
	}
	if err := openvpn.Validate(check); err != nil {
		return err
	}
	if s.Port < 1 || s.Port > 65535 {
		return errors.New("port: enter a value from 1 to 65535")
	}
	if p == "sstp" && s.Transport != "tcp" {
		return errors.New("transport: SSTP requires TCP")
	}
	if (p == "wireguard" || p == "ikev2" || p == "l2tp") && s.Transport != "udp" {
		return errors.New("transport: this protocol requires UDP")
	}
	if p == "ikev2" && s.Port != 500 {
		return errors.New("port: IKEv2 requires UDP 500 and 4500")
	}
	if p == "l2tp" && s.Port != 1701 {
		return errors.New("L2TP/IPsec requires UDP 1701, 500 and 4500")
	}
	if p == "l2tp" && s.IPsecSecret != "" && (len(s.IPsecSecret) < 14 || len(s.IPsecSecret) > 128 || strings.ContainsAny(s.IPsecSecret, "\r\n")) {
		return errors.New("IPsec shared secret must contain 14–128 bytes without newlines")
	}
	return nil
}

func Preflight(ctx context.Context, c routers.API, s domain.ServerSpec) error {
	if err := Validate(s); err != nil {
		return err
	}
	p := Name(s)
	if p == "openvpn" {
		return openvpn.Preflight(ctx, c, s)
	}
	resources, err := routers.List(ctx, c, "system/resource")
	if err != nil {
		return err
	}
	if len(resources) != 1 {
		return errors.New("cannot read RouterOS version")
	}
	major, minor, _, err := routers.Version(resources[0]["version"])
	if err != nil || major != 7 || minor < 17 {
		return errors.New("stable RouterOS 7.17+ required")
	}
	paths := []string{"interface/wireguard"}
	if p == "sstp" {
		paths = []string{"interface/sstp-server/server", "certificate", "ppp/profile", "ppp/secret"}
	}
	if p == "l2tp" {
		paths = []string{"interface/l2tp-server/server", "ppp/profile", "ppp/secret", "ip/ipsec/peer"}
	}
	if p == "ikev2" {
		paths = []string{"certificate", "ip/ipsec/profile", "ip/ipsec/proposal", "ip/ipsec/mode-config", "ip/ipsec/policy/group", "ip/ipsec/policy", "ip/ipsec/peer", "ip/ipsec/identity"}
	}
	for _, path := range paths {
		rows, err := routers.List(ctx, c, path)
		if err != nil {
			return fmt.Errorf("%s: required RouterOS menu unavailable: %w", path, err)
		}
		if path == "interface/sstp-server/server" && (len(rows) != 1 || rows[0]["enabled"] != "false" && rows[0]["enabled"] != "no") {
			return errors.New("SSTP is already enabled or its state is unknown; existing service will not be replaced")
		}
		if path == "ppp/secret" {
			for _, row := range rows {
				if row["disabled"] != "yes" && row["disabled"] != "true" && (row["service"] == p || row["service"] == "any") {
					return errors.New("existing PPP accounts could authenticate to SSTP; isolate them first")
				}
			}
		}
		if path == "ip/ipsec/peer" && len(rows) != 0 {
			return errors.New("existing IPsec peers require manual conflict review before creating IKEv2")
		}
		if path == "interface/l2tp-server/server" && (len(rows) != 1 || rows[0]["enabled"] != "false" && rows[0]["enabled"] != "no") {
			return errors.New("existing L2TP server must be reviewed before provisioning")
		}
		for _, row := range rows {
			if row["name"] == s.Name {
				return errors.New("name: an existing router resource already uses this name")
			}
		}
	}
	return openvpn.NetworkPreflight(ctx, c, s)
}

func Plan(ctx context.Context, c routers.API, id string, s domain.ServerSpec) (*domain.Job, error) {
	j := openvpn.Plan(id, s)
	p := Name(s)
	if p == "openvpn" {
		return j, nil
	}
	prefix, marker := "mvm-"+id[:12], "mvm:"+id
	base := j.Steps
	j.Steps = nil
	add := func(path string, v map[string]string) {
		j.Steps = append(j.Steps, domain.Step{Path: path, Method: "PUT", Values: v})
	}
	// Certificates are created and signed before anything references them.
	if p != "wireguard" && p != "l2tp" {
		j.Steps = append(j.Steps, base[:4]...)
	}
	if p == "sstp" || p == "l2tp" {
		j.Steps = append(j.Steps, base[4:6]...)
	}
	if p == "ikev2" {
		j.Steps = append(j.Steps, base[4])
		add("ip/ipsec/profile", map[string]string{"name": prefix, "hash-algorithm": "sha256", "enc-algorithm": "aes-256", "dh-group": "modp2048"})
		add("ip/ipsec/proposal", map[string]string{"name": prefix, "auth-algorithms": "sha256", "enc-algorithms": "aes-256-cbc", "pfs-group": "none"})
		add("ip/ipsec/mode-config", map[string]string{"name": prefix, "address-pool": j.Server.Pool, "address-prefix-length": "32", "split-include": strings.Join(s.Allowed, ","), "system-dns": "no", "static-dns": strings.Join(s.DNS, ",")})
		add("ip/ipsec/policy/group", map[string]string{"name": prefix})
		add("ip/ipsec/policy", map[string]string{"group": prefix, "src-address": "0.0.0.0/0", "dst-address": s.Subnet, "proposal": prefix, "template": "yes"})
	}
	// Install all access restrictions before creating a listening interface/peer.
	for _, step := range base[6 : len(base)-1] {
		if (p == "ikev2" || p == "l2tp") && step.Values["chain"] == "input" && step.Values["action"] == "accept" {
			step.Values["dst-port"] = "500,4500"
		}
		if p == "ikev2" && step.Values["chain"] == "forward" && step.Values["action"] == "accept" {
			if step.Values["src-address"] == s.Subnet {
				step.Values["ipsec-policy"] = "in,ipsec"
			} else {
				step.Values["ipsec-policy"] = "out,ipsec"
			}
		}
		j.Steps = append(j.Steps, step)
	}
	switch p {
	case "l2tp":
		return l2tpPlan(ctx, c, j, s, marker)
	case "wireguard":
		j.Server.CA, j.Server.Certificate, j.Server.Pool, j.Server.Profile = "", "", "", ""
		// RouterOS generates the keypair; the private key never enters the job.
		add("interface/wireguard", map[string]string{"name": s.Name, "listen-port": strconv.Itoa(s.Port), "mtu": "1420", "disabled": "no", "comment": marker})
		prefixNet, _ := netip.ParsePrefix(s.Subnet)
		add("ip/address", map[string]string{"address": s.Gateway + "/" + strconv.Itoa(prefixNet.Bits()), "interface": s.Name, "comment": marker})
	case "sstp":
		values := map[string]string{"enabled": "yes", "port": strconv.Itoa(s.Port), "certificate": j.Server.Certificate, "default-profile": j.Server.Profile, "authentication": "mschap2", "tls-version": "only-1.2", "verify-client-certificate": "no", "pfs": "yes"}
		rows, err := routers.List(ctx, c, "interface/sstp-server/server")
		if err != nil || len(rows) != 1 {
			return nil, errors.New("cannot read SSTP settings for rollback")
		}
		previous := map[string]string{}
		for key := range values {
			value, ok := rows[0][key]
			if !ok {
				return nil, fmt.Errorf("SSTP setting %s is not observable", key)
			}
			previous[key] = value
		}
		j.Steps = append(j.Steps, domain.Step{Path: "interface/sstp-server/server/set", Method: "POST", Values: values, Previous: previous})
	case "ikev2":
		// ESP is needed for clients without NAT-T.
		v := map[string]string{"chain": "input", "protocol": "ipsec-esp", "action": "accept", "comment": marker}
		if s.InputAnchor != "__empty__" {
			v["place-before"] = s.InputAnchor
		}
		add("ip/firewall/filter", v)
		add("ip/ipsec/peer", map[string]string{"name": s.Name, "exchange-mode": "ike2", "passive": "yes", "profile": prefix})
		// The identity starts disabled until client certificates have been enrolled.
		// This avoids authorizing unrelated certificates from any trusted CA.
		add("ip/ipsec/identity", map[string]string{"peer": s.Name, "auth-method": "digital-signature", "certificate": j.Server.Certificate, "generate-policy": "port-strict", "mode-config": prefix, "policy-template-group": prefix, "disabled": "yes", "comment": marker})
	}
	return j, nil
}
