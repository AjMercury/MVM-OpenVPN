package httpserver

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupAuthorizationAndClosure(t *testing.T) {
	h, r := testHTTP(t)
	token := strings.Repeat("a", 64)
	path := filepath.Join(t.TempDir(), "bootstrap.token")
	if e := os.WriteFile(path, []byte(token), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("MVM_BOOTSTRAP_TOKEN_FILE", path)
	for _, lang := range []string{"en", "fa"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/setup?lang="+lang, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `name="token"`) {
			t.Fatalf("setup page: %d", w.Code)
		}
		if lang == "fa" && !strings.Contains(w.Body.String(), `dir="rtl"`) {
			t.Fatal("setup RTL missing")
		}
	}
	for _, tc := range []struct {
		origin, token string
		want          int
	}{{"http://evil.test", token, 403}, {h.App.Config.Origin, "wrong", 403}, {h.App.Config.Origin, token, 303}} {
		v := url.Values{"token": {tc.token}, "username": {"administrator"}, "password": {"1"}, "timezone": {"UTC"}, "language": {"fa"}}
		req := httptest.NewRequest("POST", "/setup", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("setup status %d want %d: %s", w.Code, tc.want, w.Body)
		}
	}
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/setup", nil))
		if w.Code != 404 {
			t.Fatal("setup remained open")
		}
	}
}
