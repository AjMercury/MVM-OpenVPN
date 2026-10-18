package app

import (
	"bytes"
	"mikrotik-vpn-manager/internal/config"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage"
	"path/filepath"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	repo, e := storage.Open(filepath.Join(t.TempDir(), "test.db"), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { repo.Close() })
	return New(repo, config.Config{})
}
func TestSecureDelivery(t *testing.T) {
	s := testService(t)
	u := domain.VPNUser{ID: "vpn-user", RouterID: "router", AccountID: "alice"}
	if e := s.Repo.Put("users", u.ID, u); e != nil {
		t.Fatal(e)
	}
	admin := domain.Account{ID: "admin", Role: domain.Super}
	token, e := s.IssueDelivery(admin, u.ID, []byte("sensitive-profile"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Consume(domain.Account{ID: "bob", Role: domain.EndUser}, token); e == nil {
		t.Fatal("cross-user download")
	}
	data, e := s.Consume(domain.Account{ID: "alice", Role: domain.EndUser}, token)
	if e != nil || string(data) != "sensitive-profile" {
		t.Fatal(e)
	}
	if _, e = s.Consume(admin, token); e == nil {
		t.Fatal("reused token")
	}
	token, e = s.IssueDelivery(admin, u.ID, []byte("profile"))
	if e != nil {
		t.Fatal(e)
	}
	var d domain.Delivery
	if e = s.Repo.Get("deliveries", security.Digest(token), &d); e != nil {
		t.Fatal(e)
	}
	d.Expires = time.Now().Add(-time.Minute)
	_ = s.Repo.Put("deliveries", d.ID, d)
	if _, e = s.Consume(admin, token); e == nil {
		t.Fatal("expired token")
	}
}
func TestDisabledUserCannotDownload(t *testing.T) {
	s := testService(t)
	u := domain.VPNUser{ID: "u", RouterID: "r"}
	_ = s.Repo.Put("users", u.ID, u)
	a := domain.Account{Role: domain.Super}
	token, e := s.IssueDelivery(a, u.ID, []byte("profile"))
	if e != nil {
		t.Fatal(e)
	}
	u.Disabled = true
	_ = s.Repo.Put("users", u.ID, u)
	if _, e = s.Consume(a, token); e == nil {
		t.Fatal("disabled profile delivered")
	}
}

func TestDeliveryRevocationUsesCurrentScope(t *testing.T) {
	s := testService(t)
	user := domain.VPNUser{ID: "u", RouterID: "router", AccountID: "alice"}
	_ = s.Repo.Put("users", user.ID, user)
	a := domain.Account{ID: "operator", Role: domain.Operator, Routers: []string{"router"}}
	token, e := s.IssueDelivery(a, user.ID, []byte("profile"))
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeDelivery(domain.Account{ID: "bob", Role: domain.EndUser}, token); e == nil {
		t.Fatal("cross-user revocation")
	}
	if e = s.RevokeDelivery(a, token); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Consume(domain.Account{ID: "alice", Role: domain.EndUser}, token); e == nil {
		t.Fatal("revoked download")
	}
}
