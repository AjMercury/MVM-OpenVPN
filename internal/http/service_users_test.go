package httpserver

import (
	"context"
	"encoding/json"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/provisioning"
	"mikrotik-vpn-manager/internal/routers"
	"net/http/httptest"
	"strings"
	"testing"
)

type exportProfileAPI struct{ public string }

func (c exportProfileAPI) Call(_ context.Context, _, path string, _ any) ([]routers.Record, error) {
	if path == "interface/wireguard" || path == "interface/wireguard?name=wg-office" {
		return []routers.Record{{".id": "*1", "name": "wg-office", "public-key": c.public}}, nil
	}
	return []routers.Record{}, nil
}
func TestWireGuardExportReturnsNativeQR(t *testing.T) {
	h, handler := testHTTP(t)
	cookie, csrf := sessionFor(t, h, domain.Super)
	private, public, e := provisioning.WireGuardKeypair()
	if e != nil {
		t.Fatal(e)
	}
	_, serverPublic, e := provisioning.WireGuardKeypair()
	if e != nil {
		t.Fatal(e)
	}
	h.App.Connect = func(domain.Router) (routers.API, error) { return exportProfileAPI{serverPublic}, nil }
	server := domain.Server{ID: "server", JobID: "sj", Spec: domain.ServerSpec{Protocol: "wireguard", RouterID: "r", Name: "wg-office", Endpoint: "vpn.example.test", Port: 51820, DNS: []string{"1.1.1.1"}, Allowed: []string{"192.168.1.0/24"}, InputAnchor: "__empty__", ForwardAnchor: "__empty__"}}
	u := domain.VPNUser{ID: "wg", RouterID: "r", ServerID: server.ID, JobID: "uj", Protocol: "wireguard", Address: "10.80.0.10", PublicKey: public}
	for _, record := range []struct {
		bucket, id string
		value      any
	}{{"routers", "r", domain.Router{ID: "r"}}, {"servers", server.ID, server}, {"users", u.ID, u}, {"jobs", "sj", domain.Job{ID: "sj", State: "COMPLETED", Server: &server}}, {"jobs", "uj", domain.Job{ID: "uj", State: "COMPLETED"}}, {"wireguard_keys", u.ID, map[string]string{"private": private}}} {
		if e = h.App.Repo.Put(record.bucket, record.id, record.value); e != nil {
			t.Fatal(e)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/client-configs/wg/export", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("native QR export: %d %s", w.Code, w.Body)
	}
	var response struct {
		Mode string `json:"mode"`
		QR   string `json:"qr_png"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	if response.Mode != "wireguard_native_config" || !strings.HasPrefix(response.QR, "data:image/png;base64,") {
		t.Fatal("download-link QR returned instead of native configuration")
	}
	if strings.Contains(w.Body.String(), private) {
		t.Fatal("raw key leaked outside the authorized QR payload")
	}
}

func TestServiceUserMenusAndWireGuardDownload(t *testing.T) {
	h, handler := testHTTP(t)
	cookie, csrf := sessionFor(t, h, domain.Super)
	for _, u := range []domain.VPNUser{{ID: "ovpn", Username: "openvpn-only", RouterID: "r"}, {ID: "wg", Username: "wireguard-only", RouterID: "r", Protocol: "wireguard"}, {ID: "l2tp", Username: "l2tp-only", RouterID: "r", Protocol: "l2tp"}} {
		if e := h.App.Repo.Put("users", u.ID, u); e != nil {
			t.Fatal(e)
		}
	}
	req := httptest.NewRequest("GET", "/?page=users&protocol=wireguard", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "wireguard-only") || strings.Contains(body, "openvpn-only") || strings.Contains(body, "l2tp-only") || strings.Contains(body, `name="passphrase"`) {
		t.Fatal("service users or export controls mixed")
	}
	token, e := h.App.IssueDelivery(domain.Account{Role: domain.Super}, "wg", []byte("[Interface]\nPrivateKey = test\n"))
	if e != nil {
		t.Fatal(e)
	}
	req = httptest.NewRequest("POST", "/api/v1/client-configs/download", strings.NewReader("token="+token+"&csrf="+csrf))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://localhost:8080")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "client.conf") || !strings.Contains(w.Body.String(), "[Interface]") {
		t.Fatalf("WireGuard download: %d %s", w.Code, w.Body)
	}
}
