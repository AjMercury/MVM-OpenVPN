package app

import (
	"context"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"strings"
	"testing"
	"time"
)

func TestIdentityLinkScopeAndDisabledParent(t *testing.T) {
	s := testService(t)
	a := domain.Account{Role: domain.Super}
	root := domain.VPNUser{ID: "root", RouterID: "r", Username: "alice", AccountID: "owner"}
	if err := s.Repo.Put("users", root.ID, root); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, router, username, owner string
		valid                         bool
	}{{"same", "r", "alice", "owner", true}, {"other router", "other", "alice", "owner", false}, {"other name", "r", "bob", "owner", false}, {"other owner", "r", "alice", "other", false}} {
		_, err := s.resolveIdentity(a, UserRequest{IdentityID: root.ID, Username: tc.username, AccountID: tc.owner}, tc.router)
		if (err == nil) != tc.valid {
			t.Fatal(tc.name, err)
		}
	}
	root.Disabled = true
	if err := s.Repo.Put("users", root.ID, root); err != nil {
		t.Fatal(err)
	}
	s.Config.Mutations = true
	s.Connect = func(domain.Router) (routers.API, error) { return nil, nil }
	if err := s.Repo.Put("routers", "r", domain.Router{ID: "r"}); err != nil {
		t.Fatal(err)
	}
	child := domain.VPNUser{ID: "child", IdentityID: root.ID, RouterID: "r", Username: "alice", AccountID: "owner", Protocol: "wireguard"}
	job := domain.Job{ID: "child", RouterID: "r", State: "READY", Created: time.Now(), User: &child}
	if err := s.Repo.Put("jobs", job.ID, job); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(context.Background(), a, job.ID, true); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatal("disabled parent enrollment accepted", err)
	}
	if err := s.Repo.Put("users", child.ID, child); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Export(context.Background(), a, child.ID, ""); err == nil {
		t.Fatal("disabled parent profile exported")
	}
}

func TestUserGroupingAndAvailableServices(t *testing.T) {
	users := []domain.VPNUser{{ID: "root", Username: "alice", RouterID: "r", Protocol: "openvpn", ServerID: "ovpn", SharedServers: []string{"ovpn", "sstp"}}, {ID: "peer", IdentityID: "root", Username: "alice", RouterID: "r", Protocol: "wireguard", ServerID: "wg"}}
	servers := []domain.Server{{ID: "ovpn", Spec: domain.ServerSpec{RouterID: "r", Protocol: "openvpn"}}, {ID: "sstp", Spec: domain.ServerSpec{RouterID: "r", Protocol: "sstp"}}, {ID: "wg", Spec: domain.ServerSpec{RouterID: "r", Protocol: "wireguard"}}, {ID: "other", Spec: domain.ServerSpec{RouterID: "other", Protocol: "wireguard"}}}
	groups := GroupUsers(users, servers)
	if len(groups) != 1 || len(groups[0].Bindings) != 2 || len(groups[0].AvailableServers) != 0 {
		t.Fatal("duplicate identity or already-linked service offered", groups)
	}
}
