package httpserver

import (
	"html"
	"mikrotik-vpn-manager/internal/domain"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLanguageTogglePreservesServiceContext(t *testing.T) {
	h, handler := testHTTP(t)
	cookie, _ := sessionFor(t, h, domain.Super)
	req := httptest.NewRequest("GET", "/?page=users&protocol=wireguard&router_id=router-one&id=user-one&lang=en&token=secret-token&password=secret-password", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	body := html.UnescapeString(w.Body.String())
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	want := `href="/?id=user-one&lang=fa&page=users&protocol=wireguard&router_id=router-one"`
	if !strings.Contains(body, want) {
		t.Fatal("language toggle lost page context")
	}
	if strings.Contains(body, "secret-token") || strings.Contains(body, "secret-password") {
		t.Fatal("sensitive query copied into page")
	}
}
