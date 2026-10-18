package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mikrotik-vpn-manager/internal/app"
	"mikrotik-vpn-manager/internal/config"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testHTTP(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	repo, e := storage.Open(filepath.Join(t.TempDir(), "test.db"), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { repo.Close() })
	s := app.New(repo, config.Config{Dev: true, Origin: "http://localhost:8080"})
	h, e := New(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if e != nil {
		t.Fatal(e)
	}
	return h, h.Routes()
}
func sessionFor(t *testing.T, h *Server, role domain.Role) (*http.Cookie, string) {
	t.Helper()
	a := domain.Account{ID: app.ID(), Username: "test", Role: role}
	if e := h.App.Repo.Put("accounts", a.ID, a); e != nil {
		t.Fatal(e)
	}
	token := security.Token()
	csrf := security.Token()
	if e := h.App.Repo.Put("sessions", security.Digest(token), domain.Session{AccountID: a.ID, CSRF: csrf, Expires: time.Now().Add(time.Hour)}); e != nil {
		t.Fatal(e)
	}
	return &http.Cookie{Name: "mvm_session", Value: token}, csrf
}
func TestAuthenticationAndCSRF(t *testing.T) {
	h, r := testHTTP(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/routers", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	cookie, csrf := sessionFor(t, h, domain.Super)
	for _, v := range []struct {
		origin, csrf string
		want         int
	}{{"http://evil.invalid", csrf, 403}, {"http://localhost:8080", "wrong", 403}, {"http://localhost:8080", csrf, 400}} {
		req := httptest.NewRequest("POST", "/api/v1/routers", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", v.origin)
		req.Header.Set("X-CSRF-Token", v.csrf)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != v.want {
			t.Fatalf("%d: %s", w.Code, w.Body)
		}
	}
}
func TestPagesAndXSS(t *testing.T) {
	h, r := testHTTP(t)
	cookie, _ := sessionFor(t, h, domain.Super)
	router := domain.Router{ID: "r", Name: `<script>alert(1)</script>`, Password: "private-router-password", Status: "unknown"}
	_ = h.App.Repo.Put("routers", router.ID, router)
	for _, page := range []string{"dashboard", "routers", "new-router", "servers", "new-server", "users", "new-user", "jobs", "audit", "portal", "settings", "accounts"} {
		req := httptest.NewRequest("GET", "/?page="+page, nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 || strings.Contains(w.Body.String(), `<script>alert(1)</script>`) || strings.Contains(w.Body.String(), "private-router-password") {
			t.Fatalf("page %s: %d", page, w.Code)
		}
	}
	req := httptest.NewRequest("GET", "/?lang=fa", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), `dir="rtl"`) {
		t.Fatal("RTL absent")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("sensitive caching")
	}
}
func TestPrivilegeAndSecretLeak(t *testing.T) {
	h, r := testHTTP(t)
	cookie, csrf := sessionFor(t, h, domain.EndUser)
	req := httptest.NewRequest("POST", "/api/v1/accounts", strings.NewReader(`{"username":"bad","password":"a very long password","role":"super_admin"}`))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == 200 {
		t.Fatal("privilege escalation")
	}
	admin, _ := sessionFor(t, h, domain.Super)
	router := domain.Router{ID: "r", Password: "super-secret"}
	_ = h.App.Repo.Put("routers", "r", router)
	req = httptest.NewRequest("GET", "/api/v1/routers", nil)
	req.AddCookie(admin)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "super-secret") {
		t.Fatal("secret leaked")
	}
	var body map[string]any
	if json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatal("invalid response")
	}
}
func TestSessionExpiration(t *testing.T) {
	h, r := testHTTP(t)
	cookie, _ := sessionFor(t, h, domain.Super)
	_ = h.App.Repo.Put("sessions", security.Digest(cookie.Value), domain.Session{Expires: time.Now().Add(-time.Minute)})
	req := httptest.NewRequest("GET", "/api/v1/routers", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("expired session accepted")
	}
}
