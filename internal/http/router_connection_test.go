package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"net/http/httptest"
	"strings"
	"testing"
)

type connectionAPI struct {
	err   error
	calls int
}

func (c *connectionAPI) Call(_ context.Context, method, path string, _ any) ([]routers.Record, error) {
	c.calls++
	if method != "GET" {
		return nil, errors.New("unexpected mutation")
	}
	if c.err != nil {
		return nil, c.err
	}
	if path == "system/resource" {
		return []routers.Record{{"version": "7.20.1 (stable)", "board-name": "CHR"}}, nil
	}
	return []routers.Record{}, nil
}
func TestRouterConnectionBeforeAndDuringSave(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "connected", true: "failed"}[fail], func(t *testing.T) {
			h, handler := testHTTP(t)
			cookie, csrf := sessionFor(t, h, domain.Super)
			api := &connectionAPI{}
			if fail {
				api.err = &routers.APIError{Status: 401}
			}
			h.App.Connect = func(domain.Router) (routers.API, error) { return api, nil }
			request := func(path string) *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", path, strings.NewReader(`{"name":"office","host":"192.168.88.1","port":443,"username":"mvm","password":"private-test-secret"}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", "http://localhost:8080")
				req.Header.Set("X-CSRF-Token", csrf)
				req.AddCookie(cookie)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				return w
			}
			w := request("/api/v1/routers/test")
			if fail && w.Code != 400 || !fail && w.Code != 200 {
				t.Fatal(w.Code, w.Body)
			}
			var rows []domain.Router
			if err := h.App.Repo.List("routers", &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 0 {
				t.Fatal("test saved router")
			}
			w = request("/api/v1/routers")
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), "private-test-secret") {
				t.Fatal("credential leaked")
			}
			var router domain.Router
			if err := json.Unmarshal(w.Body.Bytes(), &router); err != nil {
				t.Fatal(err)
			}
			if router.LastAttempt.IsZero() || fail && (router.Status != "offline" || !strings.Contains(router.LastError, "authentication")) || !fail && router.Status != "online" {
				t.Fatal(router)
			}
			if api.calls < 2 {
				t.Fatal("save did not automatically test")
			}
		})
	}
}
func TestRouterProbeRequiresAdministrator(t *testing.T) {
	h, handler := testHTTP(t)
	cookie, csrf := sessionFor(t, h, domain.Operator)
	called := false
	h.App.Connect = func(domain.Router) (routers.API, error) { called = true; return &connectionAPI{}, nil }
	req := httptest.NewRequest("POST", "/api/v1/routers/test", strings.NewReader(`{"host":"192.168.88.1","port":443,"username":"mvm","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code == 200 || called {
		t.Fatal("unauthorized network probe")
	}
}
