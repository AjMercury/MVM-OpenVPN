package protocols

import (
	"context"
	"mikrotik-vpn-manager/internal/routers"
	"testing"
)

type l2tpSettings struct{}

func (l2tpSettings) Call(context.Context, string, string, any) ([]routers.Record, error) {
	return []routers.Record{{"enabled": "no", "default-profile": "default", "authentication": "mschap1,mschap2", "use-ipsec": "no", "ipsec-secret": ""}}, nil
}
func TestL2TPRequiresIPsecAndRestrictsPlaintext(t *testing.T) {
	s := specFor("l2tp")
	s.Port = 1701
	s.Transport = "udp"
	s.IPsecSecret = "long-private-ipsec-secret"
	if e := Validate(s); e != nil {
		t.Fatal(e)
	}
	j, e := Plan(context.Background(), l2tpSettings{}, "0123456789abcdef", s)
	if e != nil {
		t.Fatal(e)
	}
	if j.Server.Spec.IPsecSecret != "" || j.Server.Certificate != "" {
		t.Fatal("L2TP must not persist PSK in server metadata or create TLS certificates")
	}
	secure, drop, listener := -1, -1, -1
	for i, step := range j.Steps {
		if step.Path == "certificate" {
			t.Fatal("unexpected certificate")
		}
		if step.Path == "ip/firewall/filter" && step.Values["dst-port"] == "1701" {
			if step.Values["action"] == "accept" && step.Values["ipsec-policy"] == "in,ipsec" {
				secure = i
			}
			if step.Values["action"] == "drop" {
				drop = i
			}
		}
		if step.Path == "interface/l2tp-server/server/set" {
			listener = i
			if step.Values["use-ipsec"] != "required" || step.Values["authentication"] != "mschap2" || step.Values["ipsec-secret"] != s.IPsecSecret || step.Previous["enabled"] != "no" {
				t.Fatal("L2TP service settings invalid")
			}
		}
	}
	if secure < 0 || drop <= secure || listener <= drop {
		t.Fatal("plaintext L2TP is not blocked before service activation")
	}
	s.IPsecSecret = "short"
	if _, e = Plan(context.Background(), l2tpSettings{}, "0123456789abcdef", s); e == nil {
		t.Fatal("short IPsec secret accepted")
	}
}
