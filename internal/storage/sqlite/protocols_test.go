package sqlite

import (
	"bytes"
	"mikrotik-vpn-manager/internal/domain"
	"path/filepath"
	"testing"
)

func TestDistinctProtocolsShareRouterWithoutIdentityCollisions(t *testing.T) {
	repo, e := Open(filepath.Join(t.TempDir(), "protocols.db"), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer repo.Close()
	if e = repo.Put("routers", "r", domain.Router{ID: "r"}); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"openvpn", "wireguard", "sstp", "l2tp", "ikev2"} {
		if e = repo.Put("servers", p, domain.Server{ID: p, Spec: domain.ServerSpec{RouterID: "r", Protocol: p}}); e != nil {
			t.Fatalf("%s: %v", p, e)
		}
	}
	if e = repo.Put("servers", "duplicate", domain.Server{ID: "duplicate", Spec: domain.ServerSpec{RouterID: "r", Protocol: "wireguard"}}); e == nil {
		t.Fatal("duplicate same-protocol server accepted")
	}
	for _, u := range []domain.VPNUser{{ID: "ppp", RouterID: "r", ServerID: "openvpn", Username: "alice", Address: "10.80.0.10"}, {ID: "wg", RouterID: "r", ServerID: "wireguard", Protocol: "wireguard", Username: "alice", Address: "10.81.0.10"}} {
		if e = repo.Put("users", u.ID, u); e != nil {
			t.Fatal("WireGuard and PPP identity collision", e)
		}
	}
}
