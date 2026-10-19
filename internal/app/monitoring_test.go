package app

import (
	"context"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"testing"
	"time"
)

type monitorAPI struct{}

func (monitorAPI) Call(_ context.Context, method, path string, _ any) ([]routers.Record, error) {
	switch path {
	case "ppp/active":
		return []routers.Record{{"name": "client"}}, nil
	case "interface/wireguard/peers":
		return []routers.Record{{"last-handshake": "20s", "rx": "100"}, {"last-handshake": "0s", "rx": "0", "tx": "0"}, {"last-handshake": "4m"}, {"disabled": "yes", "last-handshake": "1s"}}, nil
	case "ip/ipsec/active-peers":
		return nil, errors.New("unsupported")
	case "interface/print":
		return []routers.Record{{"type": "ether", "rx-byte": "1000", "tx-byte": "2000"}, {"type": "bridge", "rx-byte": "99999", "tx-byte": "99999"}}, nil
	}
	return nil, errors.New("unexpected menu")
}
func TestObservedMonitorHistoryAndScope(t *testing.T) {
	s := testService(t)
	r := domain.Router{ID: "r", Name: "visible", Status: "online", LastSuccess: time.Now(), Observed: domain.Snapshot{Resources: map[string]string{"cpu-load": "17", "free-memory": "250", "total-memory": "1000", "free-hdd-space": "800", "total-hdd-space": "1000"}}}
	if e := s.Repo.Put("routers", r.ID, r); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 125; i++ {
		if e := s.recordMonitor(context.Background(), monitorAPI{}, r); e != nil {
			t.Fatal(e)
		}
	}
	rows, e := s.Monitoring(domain.Account{Role: domain.Operator, Routers: []string{"r"}})
	if e != nil || len(rows) != 1 {
		t.Fatal(e)
	}
	history := rows[0].History
	if len(history) != 120 {
		t.Fatal("unbounded history")
	}
	last := history[len(history)-1]
	if *last.CPU != 17 || *last.Memory != 75 || *last.WireGuard != 1 || *last.PPP != 1 || last.IPsec != nil || *last.RX != 1000 || len(last.Warnings) != 1 {
		t.Fatalf("incorrect observed sample: %+v", last)
	}
	rows, e = s.Monitoring(domain.Account{Role: domain.Operator, Routers: []string{"other"}})
	if e != nil || len(rows) != 0 {
		t.Fatal("scope leak", e)
	}
	r.Status = "offline"
	if e = s.recordMonitor(context.Background(), nil, r); e != nil {
		t.Fatal(e)
	}
	rows, e = s.Monitoring(domain.Account{Role: domain.Super})
	if e != nil {
		t.Fatal(e)
	}
	last = rows[0].History[len(rows[0].History)-1]
	if last.Online || last.CPU != nil || last.PPP != nil {
		t.Fatal("offline sample reused stale metrics")
	}
}
