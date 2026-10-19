package protocols

import (
	"context"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"testing"
)

type settingsAPI struct{}

func (settingsAPI) Call(_ context.Context, _, _ string, _ any) ([]routers.Record, error) {
	return []routers.Record{{"enabled": "no", "port": "443", "certificate": "none", "default-profile": "default", "authentication": "pap,chap,mschap1,mschap2", "tls-version": "any", "verify-client-certificate": "no", "pfs": "no"}}, nil
}
func specFor(p string) domain.ServerSpec {
	port, transport := 1194, "tcp"
	if p == "wireguard" {
		port, transport = 51820, "udp"
	}
	if p == "sstp" {
		port = 443
	}
	if p == "ikev2" {
		port, transport = 500, "udp"
	}
	return domain.ServerSpec{Protocol: p, RouterID: "router", Name: "vpn-test", Endpoint: "vpn.example.com", Port: port, Transport: transport, Subnet: "10.80.0.0/24", Gateway: "10.80.0.1", Start: "10.80.0.10", End: "10.80.0.200", DNS: []string{"1.1.1.1"}, Allowed: []string{"192.168.20.0/24"}, InputAnchor: "*1", ForwardAnchor: "*2"}
}
func TestSeparateServerPlans(t *testing.T) {
	for _, p := range []string{"openvpn", "wireguard", "sstp", "ikev2"} {
		t.Run(p, func(t *testing.T) {
			s := specFor(p)
			if err := Validate(s); err != nil {
				t.Fatal(err)
			}
			j, err := Plan(context.Background(), settingsAPI{}, "0123456789abcdef", s)
			if err != nil {
				t.Fatal(err)
			}
			lastFirewall, activation := -1, -1
			for i, step := range j.Steps {
				if step.Path == "ip/firewall/filter" {
					lastFirewall = i
				}
				if p != "openvpn" && step.Path == "interface/ovpn-server/server" {
					t.Fatal("wrong protocol server")
				}
				if p == "wireguard" && step.Path == "certificate" {
					t.Fatal("WireGuard must not create certificates")
				}
				if step.Path == "interface/wireguard" || step.Path == "interface/sstp-server/server/set" || step.Path == "ip/ipsec/peer" || step.Path == "interface/ovpn-server/server" {
					activation = i
				}
			}
			if activation <= lastFirewall || lastFirewall < 0 {
				t.Fatal("listener must follow all firewall restrictions")
			}
			if p == "sstp" {
				last := j.Steps[len(j.Steps)-1]
				if last.Previous["enabled"] != "no" || last.Values["authentication"] != "mschap2" {
					t.Fatal("SSTP rollback/authentication missing")
				}
			}
			if p == "ikev2" {
				last := j.Steps[len(j.Steps)-1]
				if last.Values["disabled"] != "yes" {
					t.Fatal("unenrolled clients must not be authorized")
				}
			}
		})
	}
}
func TestProtocolValidation(t *testing.T) {
	for _, p := range []string{"wireguard", "sstp", "ikev2"} {
		s := specFor(p)
		s.Transport = "bad"
		if Validate(s) == nil {
			t.Fatal("invalid transport accepted", p)
		}
	}
	s := specFor("ikev2")
	s.Port = 1194
	if Validate(s) == nil {
		t.Fatal("custom IKE port accepted")
	}
	s = specFor("pptp")
	if Validate(s) == nil {
		t.Fatal("unsupported protocol accepted")
	}
}
