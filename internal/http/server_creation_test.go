package httpserver

import (
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProtocolFormsAndMonitoring(t *testing.T) {
	h, handler := testHTTP(t)
	cookie, _ := sessionFor(t, h, domain.Super)
	for _, p := range []string{"openvpn", "wireguard", "sstp", "ikev2", "l2tp"} {
		request := httptest.NewRequest("GET", "/?page=new-server&protocol="+p+"&lang=fa", nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		body := response.Body.String()
		if response.Code != 200 || !strings.Contains(body, `name="protocol" value="`+p+`"`) || !strings.Contains(body, `id="server-errors"`) || !strings.Contains(body, `name="public_ip"`) || !strings.Contains(body, `class="wizard compact-wizard"`) {
			t.Fatalf("%s form: %d %s", p, response.Code, body)
		}
		if p == "ikev2" && !strings.Contains(body, h.locales["fa"]["ikev2Enrollment"]) {
			t.Fatal("missing enrollment limitation")
		}
	}
	for _, router := range []domain.Router{
		{ID: "visible", Name: "Visible router", Status: "online", LastSuccess: time.Now().Add(-time.Hour), Observed: domain.Snapshot{Resources: map[string]string{"cpu-load": "17", "uptime": "3d", "free-memory": "1000", "total-memory": "2000"}}},
		{ID: "hidden", Name: "Hidden router", Status: "online", Password: "do-not-leak"},
	} {
		if err := h.App.Repo.Put("routers", router.ID, router); err != nil {
			t.Fatal(err)
		}
	}
	// Change the session account to a scoped operator.
	var session domain.Session
	if err := h.App.Repo.Get("sessions", security.Digest(cookie.Value), &session); err != nil {
		t.Fatal(err)
	}
	if err := h.App.Repo.Put("accounts", session.AccountID, domain.Account{ID: session.AccountID, Role: domain.Operator, Routers: []string{"visible"}}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/?page=monitoring", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != 200 || !strings.Contains(body, "Visible router") || !strings.Contains(body, "stale") || !strings.Contains(body, "17%") || strings.Contains(body, "Hidden router") || strings.Contains(body, "do-not-leak") {
		t.Fatalf("monitor scope/sample: %d %s", response.Code, body)
	}
}
