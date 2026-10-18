package httpserver

import (
	"encoding/json"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDirectIPAndDomainLoginAndCSRF(t *testing.T) {
	for _, plain := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTPS", true: "HTTP"}[plain], func(t *testing.T) {
			h, r := testHTTP(t)
			h.App.Config.Dev = false
			h.App.Config.AllowHTTP = plain
			h.App.Config.Origin = "https://172.16.180.58:8443"
			h.App.Config.AdditionalOrigins = []string{"https://panel.example.test:8443"}
			if plain {
				h.App.Config.Origin = strings.Replace(h.App.Config.Origin, "https:", "http:", 1)
				h.App.Config.AdditionalOrigins[0] = strings.Replace(h.App.Config.AdditionalOrigins[0], "https:", "http:", 1)
			}
			hash, err := security.HashPassword("long-direct-password")
			if err != nil {
				t.Fatal(err)
			}
			if err := h.App.Repo.Put("accounts", "a", domain.Account{ID: "a", Username: "admin", Hash: hash, Role: domain.Super}); err != nil {
				t.Fatal(err)
			}
			for _, origin := range []string{h.App.Config.Origin, h.App.Config.AdditionalOrigins[0]} {
				req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"long-direct-password"}`))
				req.Header.Set("Origin", origin)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != 200 {
					t.Fatal(origin, w.Code, w.Body.String())
				}
				cookies := w.Result().Cookies()
				if len(cookies) == 0 || cookies[0].Secure == plain || !cookies[0].HttpOnly {
					t.Fatal("HTTPS session cookie not secure")
				}
				if (w.Header().Get("Strict-Transport-Security") == "") != plain {
					t.Fatal("HTTPS missing HSTS")
				}
				var body struct {
					CSRF string `json:"csrf"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				for _, allowed := range []bool{false, true} {
					req := httptest.NewRequest("POST", "/api/v1/logout", nil)
					req.AddCookie(cookies[0])
					req.Header.Set("X-CSRF-Token", body.CSRF)
					if allowed {
						req.Header.Set("Origin", origin)
					} else {
						req.Header.Set("Origin", "http://evil.example:8443")
					}
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					if !allowed && w.Code != 403 {
						t.Fatal("untrusted origin accepted", w.Code)
					}
					if allowed && w.Code != 303 {
						t.Fatal("configured origin rejected", w.Code)
					}
				}
			}

		})
	}
}
