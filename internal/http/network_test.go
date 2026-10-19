package httpserver

import (
	"context"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"net/http/httptest"
	"strings"
	"testing"
)

type networkChoicesAPI struct{}

func (networkChoicesAPI) Call(_ context.Context, _, path string, _ any) ([]routers.Record, error) {
	switch path {
	case "interface":
		return []routers.Record{{".id": "*1", "name": "WAN <script>bad</script>", "type": "ether", "running": "yes"}}, nil
	case "ip/pool":
		return []routers.Record{{".id": "*2", "name": "office-pool", "ranges": "10.80.0.10-10.80.0.200"}}, nil
	case "ip/address":
		return []routers.Record{{".id": "*3", "interface": "LAN", "address": "192.168.88.1/24"}}, nil
	case "ip/firewall/filter":
		return []routers.Record{{".id": "*A", "chain": "forward", "action": "drop"}}, nil
	}
	return []routers.Record{}, nil
}
func TestNetworkPageSelectsExistingScopedResources(t *testing.T) {
	h, handler := testHTTP(t)
	cookie, _ := sessionFor(t, h, domain.Super)
	h.App.Connect = func(domain.Router) (routers.API, error) { return networkChoicesAPI{}, nil }
	for _, id := range []string{"one", "two"} {
		if e := h.App.Repo.Put("routers", id, domain.Router{ID: id, Name: id}); e != nil {
			t.Fatal(e)
		}
	}
	for _, s := range []domain.Server{{ID: "local", Spec: domain.ServerSpec{RouterID: "one", Name: "local-vpn", Subnet: "10.80.0.0/24"}}, {ID: "foreign", Spec: domain.ServerSpec{RouterID: "two", Name: "foreign-vpn", Subnet: "10.81.0.0/24"}}} {
		if e := h.App.Repo.Put("servers", s.ID, s); e != nil {
			t.Fatal(e)
		}
	}
	request := httptest.NewRequest("GET", "/?page=network&router_id=one&lang=fa", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != 200 {
		t.Fatal(response.Code, body)
	}
	for _, want := range []string{`<select name="source_id" required>`, `value="server:local"`, `value="pool:*2"`, `value="address:*3"`, `<select name="out_interface_id" required>`, `value="*1"`, `name="allow_forward"`, `id="network-errors"`} {
		if !strings.Contains(body, want) {
			t.Fatal("missing selection", want)
		}
	}
	if strings.Contains(body, "foreign-vpn") || strings.Contains(body, "<script>bad</script>") || strings.Contains(body, `<input name="source_id"`) {
		t.Fatal("foreign resources, injection, or free-text source input")
	}
}
