package provisioning

import (
	"bytes"
	"mikrotik-vpn-manager/internal/domain"
	"strings"
	"testing"
)

func TestWireGuardNativeProfile(t *testing.T) {
	private, public, e := WireGuardKeypair()
	if e != nil {
		t.Fatal(e)
	}
	_, serverPublic, e := WireGuardKeypair()
	if e != nil {
		t.Fatal(e)
	}
	s := domain.Server{Spec: domain.ServerSpec{Endpoint: "2001:db8::1", Port: 51820, DNS: []string{"192.168.1.53"}, Allowed: []string{"192.168.1.0/24"}}}
	u := domain.VPNUser{Address: "10.80.0.10", PublicKey: public}
	profile, e := WireGuardProfile(s, u, private, serverPublic)
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"[Interface]\nPrivateKey = " + private, "Address = 10.80.0.10/32", "PublicKey = " + serverPublic, "AllowedIPs = 192.168.1.0/24", "Endpoint = [2001:db8::1]:51820"} {
		if !strings.Contains(string(profile), want) {
			t.Fatal("profile missing", want)
		}
	}
	if strings.Contains(string(profile), "0.0.0.0/0") {
		t.Fatal("unexpected full tunnel")
	}
	png, e := WireGuardQR(profile)
	if e != nil || !bytes.HasPrefix(png, []byte{137, 80, 78, 71}) {
		t.Fatal("native QR failed", e)
	}
	u.PublicKey = serverPublic
	if _, e = WireGuardProfile(s, u, private, serverPublic); e == nil {
		t.Fatal("mismatched private key accepted")
	}
}
