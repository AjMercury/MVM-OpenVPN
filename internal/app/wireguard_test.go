package app

import (
	"bytes"
	"context"
	"encoding/json"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/provisioning"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWireGuardEnrollmentAndDelivery(t *testing.T) {
	for _, edition := range []string{"bbolt", "sqlite"} {
		t.Run(edition, func(t *testing.T) {
			s := testService(t)
			if edition == "sqlite" {
				repo, e := sqlite.Open(filepath.Join(t.TempDir(), "wg.db"), bytes.Repeat([]byte{1}, 32))
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { repo.Close() })
				s = New(repo, s.Config)
			}
			s.Config.Mutations = true
			m := &mockREST{seq: 16, records: map[string][]routers.Record{"system/resource": {{"version": "7.20.1 (stable)"}}, "interface/wireguard": {}, "interface/wireguard/peers": {}, "ip/pool": {}, "ip/address": {}, "ip/route": {}, "ip/firewall/filter": {{".id": "*A", "chain": "input", "action": "drop"}, {".id": "*B", "chain": "forward", "action": "drop"}}}}
			srv := httptest.NewTLSServer(m)
			defer srv.Close()
			s.Connect = func(domain.Router) (routers.API, error) { return mockWire{srv}, nil }
			if e := s.Repo.Put("routers", "r", domain.Router{ID: "r"}); e != nil {
				t.Fatal(e)
			}
			a := domain.Account{ID: "admin", Role: domain.Super}
			ctx := context.Background()
			spec := domain.ServerSpec{Protocol: "wireguard", RouterID: "r", Name: "wg-office", Endpoint: "vpn.example.test", Port: 51820, Transport: "udp", Subnet: "10.80.0.0/24", Gateway: "10.80.0.1", Start: "10.80.0.10", End: "10.80.0.200", DNS: []string{"192.168.1.53"}, Allowed: []string{"192.168.1.0/24"}, InputAnchor: "*A", ForwardAnchor: "*B"}
			j, e := s.PrepareServer(ctx, a, spec, "wg-server-key")
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(ctx, a, j.ID, true); e != nil {
				t.Fatal(e)
			}
			_, serverPublic, e := provisioning.WireGuardKeypair()
			if e != nil {
				t.Fatal(e)
			}
			m.records["interface/wireguard"][0]["public-key"] = serverPublic
			var first domain.VPNUser
			for i, name := range []string{"alice", "bob"} {
				job, e := s.PrepareUser(ctx, a, UserRequest{ServerID: j.ID, Username: name, Key: "wg-user-" + name})
				if e != nil {
					t.Fatal(e)
				}
				var key wireGuardKey
				if e = s.Repo.Get("wireguard_keys", job.ID, &key); e != nil {
					t.Fatal(e)
				}
				raw, _ := json.Marshal(job)
				if strings.Contains(string(raw), key.Private) {
					t.Fatal("private key leaked into job")
				}
				if e = s.Apply(ctx, a, job.ID, true); e != nil {
					t.Fatal(e)
				}
				var user domain.VPNUser
				if e = s.Repo.Get("users", job.ID, &user); e != nil {
					t.Fatal(e)
				}
				if i == 0 {
					first = user
				}
				profile, e := s.Export(ctx, a, user.ID, "")
				if e != nil {
					t.Fatal(e)
				}
				if !strings.Contains(string(profile), "PrivateKey = "+key.Private) {
					t.Fatal("wrong private key export")
				}
				token, e := s.IssueDelivery(a, user.ID, profile)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = s.Consume(domain.Account{ID: "other", Role: domain.EndUser}, token); e == nil {
					t.Fatal("unauthorized download")
				}
				if _, e = s.Consume(a, token); e != nil {
					t.Fatal(e)
				}
				if _, e = s.Consume(a, token); e == nil {
					t.Fatal("token reused")
				}
			}
			if len(m.records["interface/wireguard/peers"]) != 2 {
				t.Fatal("missing peers")
			}
			m.records["interface/wireguard/peers"] = append(m.records["interface/wireguard/peers"], routers.Record{".id": "*ROGUE", "interface": spec.Name, "public-key": "unmanaged", "allowed-address": "0.0.0.0/0"})
			if _, e = s.Export(ctx, a, first.ID, ""); e == nil {
				t.Fatal("unreviewed peer scope exported")
			}
			m.records["interface/wireguard/peers"] = m.records["interface/wireguard/peers"][:2]
			preview, err := s.PrepareUser(ctx, a, UserRequest{ServerID: j.ID, Username: "charlie", Key: "wg-charlie-preview"})
			if err != nil {
				t.Fatal(err)
			}
			m.records["interface/wireguard/peers"] = append(m.records["interface/wireguard/peers"], routers.Record{".id": "*ROGUE", "interface": spec.Name, "public-key": "unmanaged", "allowed-address": "0.0.0.0/0"})
			if err = s.Apply(ctx, a, preview.ID, true); err == nil {
				t.Fatal("apply accepted unreviewed peer")
			}
			if len(m.records["interface/wireguard/peers"]) != 3 {
				t.Fatal("rejected preview mutated peers")
			}
			m.records["interface/wireguard/peers"] = m.records["interface/wireguard/peers"][:2]
			peer := m.records["interface/wireguard/peers"][0]
			originalAddress := peer["allowed-address"]
			peer["allowed-address"] = "0.0.0.0/0"
			if e = s.UserAction(ctx, a, first.ID, "disable"); e == nil {
				t.Fatal("modified peer scope accepted")
			}
			if peer["disabled"] == "yes" {
				t.Fatal("modified peer was mutated")
			}
			peer["allowed-address"] = originalAddress
			if e = s.UserAction(ctx, a, first.ID, "disable"); e != nil {
				t.Fatal(e)
			}
			if _, e = s.Export(ctx, a, first.ID, ""); e == nil {
				t.Fatal("disabled peer exported")
			}
		})
	}
}
