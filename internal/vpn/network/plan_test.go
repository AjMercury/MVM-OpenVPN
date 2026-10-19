package network

import (
	"fmt"
	"mikrotik-vpn-manager/internal/domain"
	"net/netip"
	"testing"
)

func TestPoolAddressesDoNotExpandRange(t *testing.T) {
	addresses, e := PoolAddresses("10.80.0.10-10.80.0.200,10.80.1.7")
	if e != nil {
		t.Fatal(e)
	}
	for octet := 0; octet < 256; octet++ {
		a := netip.MustParseAddr(fmt.Sprintf("10.80.0.%d", octet))
		count := 0
		for _, raw := range addresses {
			if netip.MustParsePrefix(raw).Contains(a) {
				count++
			}
		}
		want := 0
		if octet >= 10 && octet <= 200 {
			want = 1
		}
		if count != want {
			t.Fatalf("%s covered %d times", a, count)
		}
	}
	for _, raw := range []string{"", "10.80.0.200-10.80.0.10", "0.0.0.0/0", "10.80.0.7/24", "2001:db8::/64"} {
		if _, e = PoolAddresses(raw); e == nil {
			t.Fatal("invalid pool accepted", raw)
		}
	}
}
func TestResolveUsesOnlyExistingChoices(t *testing.T) {
	o := Options{Interfaces: []Interface{{ID: "*1", Name: "ether1"}}, Sources: []Source{{ID: "server:vpn", Label: "Office VPN", Addresses: []string{"10.80.0.0/24"}}}, NATAnchor: "*A", ForwardAnchor: "*B"}
	spec := domain.NetworkSpec{SourceID: "server:vpn", OutInterfaceID: "*1", OutInterface: "forged", Addresses: []string{"0.0.0.0/0"}, NATAnchor: "*A", ForwardAnchor: "*B", AllowForward: true}
	got, e := Resolve(o, spec)
	if e != nil {
		t.Fatal(e)
	}
	if got.OutInterface != "ether1" || len(got.Addresses) != 1 || got.Addresses[0] != "10.80.0.0/24" {
		t.Fatal("client controlled addresses or interface name")
	}
	spec.SourceID = "server:other-router"
	if _, e = Resolve(o, spec); e == nil {
		t.Fatal("unknown range accepted")
	}
	spec.SourceID = "server:vpn"
	spec.OutInterfaceID = "*2"
	if _, e = Resolve(o, spec); e == nil {
		t.Fatal("unknown interface accepted")
	}
	spec.OutInterfaceID = "*1"
	spec.NATAnchor = "*OLD"
	if _, e = Resolve(o, spec); e == nil {
		t.Fatal("stale NAT anchor accepted")
	}
	spec.NATAnchor = "*A"
	o.ForwardAnchor = ""
	spec.ForwardAnchor = ""
	if _, e = Resolve(o, spec); e == nil {
		t.Fatal("dynamic forward chain accepted")
	}
	spec.AllowForward = false
	if _, e = Resolve(o, spec); e != nil {
		t.Fatal("NAT-only incorrectly requires a forward anchor", e)
	}
}
func TestPlanSeparatesNATFromForwardPermissions(t *testing.T) {
	s := domain.NetworkSpec{RouterID: "r", OutInterface: "pppoe-out1", Addresses: []string{"10.80.0.10/32", "10.80.0.12/30"}, NATAnchor: "*A", ForwardAnchor: "*B"}
	j := Plan("0123456789abcdef", s)
	if len(j.Steps) != 3 {
		t.Fatal("NAT-only created forwarding rules")
	}
	if j.Steps[0].Values["address"] != "10.80.0.10" {
		t.Fatal("single IP must use canonical RouterOS address-list representation")
	}
	nat := j.Steps[2]
	if nat.Path != "ip/firewall/nat" || nat.Values["out-interface"] != "pppoe-out1" || nat.Values["ipsec-policy"] != "out,none" || nat.Values["place-before"] != "*A" {
		t.Fatal("unscoped NAT plan")
	}
	s.AllowForward = true
	j = Plan("0123456789abcdef", s)
	if len(j.Steps) != 5 {
		t.Fatal("missing forwarding pair")
	}
	if j.Steps[3].Values["connection-state"] != "established,related" || j.Steps[4].Values["out-interface"] != "pppoe-out1" {
		t.Fatal("return and output policies are not scoped")
	}
}
