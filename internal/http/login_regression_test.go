package httpserver

import (
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuccessfulLoginsDoNotExhaustSharedIP(t *testing.T) {
	h, r := testHTTP(t)
	hash, err := security.HashPassword("long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.App.Repo.Put("accounts", "admin", domain.Account{ID: "admin", Username: "admin", Hash: hash, Role: domain.Super}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"long-test-password"}`))
		req.Header.Set("Origin", h.App.Config.Origin)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("login %d: %d %s", i, w.Code, w.Body.String())
		}
	}
}

func TestTrustedProxyDoesNotTrustDirectSpoofedHeaders(t *testing.T) {
	h, _ := testHTTP(t)
	h.App.Config.TrustedProxies = []string{"10.0.0.3"}
	r := h.Routes()
	for i, source := range []string{"10.0.0.3:1234", "10.0.0.4:1234"} {
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"unknown","password":"wrong"}`))
		req.RemoteAddr = source
		req.Header.Set("Origin", h.App.Config.Origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "203.0.113.8")
		r.ServeHTTP(httptest.NewRecorder(), req)
		key := "ip:203.0.113.8"
		if i == 1 {
			key = "ip:10.0.0.4"
		}
		if h.attempts[key].Count != 1 {
			t.Fatal("incorrect proxy trust", key)
		}
	}
}

type failedServerInventory struct{ storage.Repository }

func (s failedServerInventory) List(bucket string, out any) error {
	if bucket == "servers" {
		return errors.New("injected inventory failure")
	}
	return s.Repository.List(bucket, out)
}
func TestRouterRemovalStopsOnInventoryReadFailure(t *testing.T) {
	h, r := testHTTP(t)
	cookie, csrf := sessionFor(t, h, domain.Super)
	if err := h.App.Repo.Put("routers", "r", domain.Router{ID: "r"}); err != nil {
		t.Fatal(err)
	}
	h.App.Repo = failedServerInventory{h.App.Repo}
	req := httptest.NewRequest("DELETE", "/api/v1/routers/r", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", h.App.Config.Origin)
	req.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatal(w.Code)
	}
	var router domain.Router
	if err := h.App.Repo.Get("routers", "r", &router); err != nil {
		t.Fatal("router removed despite failed dependency check", err)
	}
}

func TestAccountThrottleAcrossAddresses(t *testing.T) {
	h, r := testHTTP(t)
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"target","password":"wrong-password"}`))
		req.RemoteAddr = string(rune('1'+i)) + ".0.0.1:1234"
		req.Header.Set("Origin", h.App.Config.Origin)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		want := 401
		if i == 5 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
}
