package app

import (
	"bytes"
	"context"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedNetworkApplyAndExistingVPNOwnership(t *testing.T) {
	for _, edition := range []string{"bbolt", "sqlite"} {
		t.Run(edition, func(t *testing.T) {
			s := testService(t)
			if edition == "sqlite" {
				repo, e := sqlite.Open(filepath.Join(t.TempDir(), "network.db"), bytes.Repeat([]byte{1}, 32))
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { repo.Close() })
				s = New(repo, s.Config)
			}
			s.Config.Mutations = true
			m := &mockREST{seq: 16, records: map[string][]routers.Record{"system/resource": {{"version": "7.20.1 (stable)"}}, "interface": {{".id": "*1", "name": "ether1", "type": "ether", "running": "yes"}, {".id": "*2", "name": "disabled-wan", "disabled": "yes"}}, "interface/ovpn-server/server": {}, "certificate": {}, "ppp/profile": {}, "ppp/secret": {}, "ip/pool": {}, "ip/address": {}, "ip/route": {}, "ip/firewall/nat": {}, "ip/firewall/address-list": {}, "ip/firewall/filter": {{".id": "*A", "chain": "input", "action": "drop"}, {".id": "*B", "chain": "forward", "action": "drop"}}}}
			srv := httptest.NewTLSServer(m)
			defer srv.Close()
			s.Connect = func(domain.Router) (routers.API, error) { return mockWire{srv}, nil }
			if e := s.Repo.Put("routers", "r", domain.Router{ID: "r"}); e != nil {
				t.Fatal(e)
			}
			ctx := context.Background()
			admin := domain.Account{ID: "admin", Role: domain.Super}
			serverSpec := domain.ServerSpec{RouterID: "r", Name: "office", Endpoint: "vpn.example.test", Port: 1194, Transport: "tcp", Subnet: "10.80.0.0/24", Start: "10.80.0.10", End: "10.80.0.200", Gateway: "10.80.0.1", DNS: []string{"192.168.10.53"}, Allowed: []string{"192.168.10.0/24"}, InputAnchor: "*A", ForwardAnchor: "*B"}
			serverJob, e := s.PrepareServer(ctx, admin, serverSpec, "server-before-nat")
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(ctx, admin, serverJob.ID, true); e != nil {
				t.Fatal(e)
			}
			options, e := s.NetworkOptions(ctx, admin, "r")
			if e != nil {
				t.Fatal(e)
			}
			if len(options.Interfaces) != 1 {
				t.Fatal("disabled interface was selectable")
			}
			spec := domain.NetworkSpec{RouterID: "r", SourceID: "server:" + serverJob.ID, OutInterfaceID: "*1", NATAnchor: options.NATAnchor, ForwardAnchor: options.ForwardAnchor, AllowForward: true}
			op := domain.Account{Role: domain.Operator, Routers: []string{"r"}}
			if _, e = s.PrepareNetwork(ctx, op, spec, "operator-nat"); e == nil {
				t.Fatal("operator could configure NAT")
			}
			j, e := s.PrepareNetwork(ctx, admin, spec, "network-idempotency")
			if e != nil {
				t.Fatal(e)
			}
			if len(m.records["ip/firewall/nat"]) != 0 {
				t.Fatal("preview changed router")
			}
			if e = s.Apply(ctx, admin, j.ID, false); e == nil {
				t.Fatal("unconfirmed NAT applied")
			}
			// Renaming the selected interface after review must reject the stale plan.
			m.records["interface"][0]["name"] = "renamed"
			if e = s.Apply(ctx, admin, j.ID, true); e == nil {
				t.Fatal("renamed interface silently applied")
			}
			m.records["interface"][0]["name"] = "ether1"
			if e = s.Apply(ctx, op, j.ID, true); e == nil {
				t.Fatal("operator applied an administrator network job")
			}
			if e = s.Apply(ctx, admin, j.ID, true); e != nil {
				t.Fatal(e)
			}
			nat := m.records["ip/firewall/nat"]
			if len(nat) != 1 || nat[0]["out-interface"] != "ether1" || nat[0]["ipsec-policy"] != "out,none" {
				t.Fatal("NAT selection not applied")
			}
			if _, e = s.PrepareUser(ctx, admin, UserRequest{ServerID: serverJob.ID, Username: "after-nat", Password: "long vpn password", Key: "user-after-nat"}); e != nil {
				t.Fatal("approved NAT broke VPN ownership checks", e)
			}
			// Unapproved rules ahead of the managed policy must still block enrollment.
			m.records["ip/firewall/filter"] = append([]routers.Record{{".id": "*BAD", "chain": "forward", "action": "accept"}}, m.records["ip/firewall/filter"]...)
			if _, e = s.PrepareUser(ctx, admin, UserRequest{ServerID: serverJob.ID, Username: "blocked-user", Password: "long vpn password", Key: "user-with-drift"}); e == nil || !strings.Contains(e.Error(), "managed firewall") {
				t.Fatal("unmanaged firewall drift was accepted", e)
			}
		})
	}
}
