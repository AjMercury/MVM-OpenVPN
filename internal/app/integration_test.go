package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// This is explicitly a test double, never a production RouterOS transport.
type mockREST struct {
	mu      sync.Mutex
	seq     int
	records map[string][]routers.Record
}

func (m *mockREST) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := strings.TrimPrefix(r.URL.Path, "/rest/")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" && p == "certificate/sign" {
		var v routers.Record
		_ = json.NewDecoder(r.Body).Decode(&v)
		for _, c := range m.records["certificate"] {
			if c["name"] == v["numbers"] {
				c["fingerprint"] = "signed-fingerprint"
			}
		}
		json.NewEncoder(w).Encode([]routers.Record{})
		return
	}
	switch r.Method {
	case "GET":
		if rows, ok := m.records[p]; ok {
			out := []routers.Record{}
			for _, row := range rows {
				match := true
				for key, values := range r.URL.Query() {
					if row[key] != values[0] {
						match = false
					}
				}
				if match {
					out = append(out, row)
				}
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		menu, id := path.Split(p)
		for _, row := range m.records[strings.TrimSuffix(menu, "/")] {
			if row[".id"] == id {
				json.NewEncoder(w).Encode(row)
				return
			}
		}
		http.Error(w, `{}`, 404)
	case "PUT":
		var row routers.Record
		if json.NewDecoder(r.Body).Decode(&row) != nil {
			w.WriteHeader(400)
			return
		}
		m.seq++
		row[".id"] = fmt.Sprintf("*%X", m.seq)
		anchor := row["place-before"]
		delete(row, "place-before")
		rows := m.records[p]
		inserted := false
		for i, item := range rows {
			if item[".id"] == anchor {
				rows = append(rows, nil)
				copy(rows[i+1:], rows[i:])
				rows[i] = row
				inserted = true
				break
			}
		}
		if !inserted {
			rows = append(rows, row)
		}
		m.records[p] = rows
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(row)
	case "PATCH":
		menu, id := path.Split(p)
		var values routers.Record
		_ = json.NewDecoder(r.Body).Decode(&values)
		for _, row := range m.records[strings.TrimSuffix(menu, "/")] {
			if row[".id"] == id {
				for k, v := range values {
					row[k] = v
				}
				json.NewEncoder(w).Encode(row)
				return
			}
		}
		w.WriteHeader(404)
	case "DELETE":
		menu, id := path.Split(p)
		menu = strings.TrimSuffix(menu, "/")
		for i, row := range m.records[menu] {
			if row[".id"] == id {
				m.records[menu] = append(m.records[menu][:i], m.records[menu][i+1:]...)
				w.WriteHeader(204)
				return
			}
		}
		w.WriteHeader(404)
	default:
		w.WriteHeader(405)
	}
}

type mockWire struct{ srv *httptest.Server }

func (c mockWire) Call(ctx context.Context, method, p string, body any) ([]routers.Record, error) {
	b, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, method, c.srv.URL+"/rest/"+p, bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	resp, e := c.srv.Client().Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(resp.Body)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode >= 400 {
		return nil, &routers.APIError{Status: resp.StatusCode}
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var rows []routers.Record
	if json.Unmarshal(raw, &rows) == nil {
		return rows, nil
	}
	var row routers.Record
	if e = json.Unmarshal(raw, &row); e != nil {
		return nil, e
	}
	return []routers.Record{row}, nil
}
func TestOpenVPNVerticalSliceMockREST(t *testing.T) {
	for _, edition := range []string{"bbolt", "sqlite"} {
		t.Run(edition, func(t *testing.T) {
			s := testService(t)
			if edition == "sqlite" {
				repo, e := sqlite.Open(filepath.Join(t.TempDir(), "mvm.db"), bytes.Repeat([]byte{1}, 32))
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { repo.Close() })
				s = New(repo, s.Config)
			}
			verticalSlice(t, s)
		})
	}
}
func verticalSlice(t *testing.T, s *Service) {
	s.Config.Mutations = true
	m := &mockREST{seq: 16, records: map[string][]routers.Record{"system/resource": {{"version": "7.17.2 (stable)", "architecture-name": "x86_64", "board-name": "CHR"}}, "interface/ovpn-server/server": {}, "certificate": {}, "ip/pool": {}, "ppp/profile": {}, "ppp/secret": {}, "ip/address": {}, "ip/route": {}, "ppp/active": {}, "ip/firewall/filter": {{".id": "*A", "chain": "input", "action": "drop"}, {".id": "*B", "chain": "forward", "action": "drop"}}}}
	srv := httptest.NewTLSServer(m)
	defer srv.Close()
	s.Connect = func(domain.Router) (routers.API, error) { return mockWire{srv}, nil }
	r := domain.Router{ID: "r", Name: "lab"}
	if e := s.Repo.Put("routers", r.ID, r); e != nil {
		t.Fatal(e)
	}
	a := domain.Account{ID: "admin", Role: domain.Super}
	ctx := context.Background()
	spec := domain.ServerSpec{RouterID: "r", Name: "office", Endpoint: "vpn.example.test", Port: 1194, Transport: "tcp", Subnet: "10.80.0.0/24", Start: "10.80.0.10", End: "10.80.0.200", Gateway: "10.80.0.1", DNS: []string{"192.168.10.53"}, Allowed: []string{"192.168.10.0/24"}, InputAnchor: "*A", ForwardAnchor: "*B"}
	j, e := s.PrepareServer(ctx, a, spec, "server-idempotency")
	if e != nil {
		t.Fatal(e)
	}
	if len(m.records["certificate"]) != 0 {
		t.Fatal("preview mutated router")
	}
	if e = s.Apply(ctx, a, j.ID, false); e == nil {
		t.Fatal("unconfirmed write")
	}
	if e = s.Apply(ctx, a, j.ID, true); e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(ctx, a, j.ID, true); e != nil {
		t.Fatal("idempotent apply", e)
	}
	var server domain.Server
	if e = s.Repo.Get("servers", j.ID, &server); e != nil {
		t.Fatal(e)
	}
	user, e := s.PrepareUser(ctx, a, UserRequest{ServerID: server.ID, Username: "alice", Password: "long user password 123", Key: "user-idempotency"})
	if e != nil {
		t.Fatal(e)
	}
	if user.User.Address != "10.80.0.10" {
		t.Fatal("bad automatic reservation")
	}
	if e = s.Apply(ctx, a, user.ID, true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PrepareUser(ctx, a, UserRequest{ServerID: server.ID, Username: "alice", Password: "long user password 123", Key: "second-user-key"}); e == nil {
		t.Fatal("duplicate identity")
	}
	if _, e = s.PrepareUser(ctx, a, UserRequest{ServerID: server.ID, Username: "bob", Password: "long user password 123", Address: "10.90.0.1", Key: "second-user-key"}); e == nil {
		t.Fatal("out-of-pool static address")
	}
	preview, e := s.PrepareUser(ctx, a, UserRequest{ServerID: server.ID, Username: "bob", Password: "long user password 123", Key: "expired-preview-key"})
	if e != nil {
		t.Fatal(e)
	}
	preview.Created = time.Now().Add(-time.Hour)
	if e := s.Repo.Put("jobs", preview.ID, preview); e != nil {
		t.Fatal(e)
	}
	if e := s.PurgeExpired(); e != nil {
		t.Fatal(e)
	}
	replacement, e := s.PrepareUser(ctx, a, UserRequest{ServerID: server.ID, Username: "bob", Password: "long user password 123", Key: "replacement-preview-key"})
	if e != nil {
		t.Fatal("expired preview still reserves username", e)
	}
	if replacement.User.Address != preview.User.Address {
		t.Fatal("expired address still reserved")
	}
	m.mu.Lock()
	m.records["ppp/active"] = []routers.Record{{".id": "*FF", "name": "alice", "service": "ovpn"}}
	m.mu.Unlock()
	if e = s.UserAction(ctx, a, user.ID, "disable"); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.records["ppp/active"]) != 0 || m.records["ppp/secret"][0]["disabled"] != "yes" {
		t.Fatal("disable/disconnect failed")
	}
}
