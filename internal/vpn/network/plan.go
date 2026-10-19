package network

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"net/netip"
	"strings"
)

type Source struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Addresses []string `json:"addresses"`
}
type Interface struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Running bool   `json:"running"`
}
type Options struct {
	Interfaces    []Interface      `json:"interfaces"`
	Sources       []Source         `json:"sources"`
	Filter        []routers.Record `json:"filter"`
	NAT           []routers.Record `json:"nat"`
	ForwardAnchor string           `json:"forward_anchor"`
	NATAnchor     string           `json:"nat_anchor"`
}

func Anchor(rows []routers.Record, chain string) (string, error) {
	for _, r := range rows {
		if r["chain"] == chain {
			if r["dynamic"] == "true" || r["dynamic"] == "yes" || r[".id"] == "" {
				return "", fmt.Errorf("first %s rule is dynamic or has no ID; review its order before proceeding", chain)
			}
			return r[".id"], nil
		}
	}
	return "__empty__", nil
}

// PoolAddresses covers an existing pool exactly, without widening it to a subnet.
func PoolAddresses(raw string) ([]string, error) {
	result := []string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if p, e := netip.ParsePrefix(part); e == nil {
			if !p.Addr().Is4() || p.Bits() == 0 || p != p.Masked() {
				return nil, errors.New("invalid pool prefix")
			}
			result = append(result, p.String())
			continue
		}
		ends := strings.Split(part, "-")
		if len(ends) > 2 {
			return nil, errors.New("invalid pool range")
		}
		start, e := netip.ParseAddr(strings.TrimSpace(ends[0]))
		if e != nil || !start.Is4() || !start.IsGlobalUnicast() {
			return nil, errors.New("invalid pool start")
		}
		end := start
		if len(ends) == 2 {
			end, e = netip.ParseAddr(strings.TrimSpace(ends[1]))
			if e != nil || !end.Is4() || !end.IsGlobalUnicast() {
				return nil, errors.New("invalid pool end")
			}
		}
		a4, b4 := start.As4(), end.As4()
		a, b := uint64(binary.BigEndian.Uint32(a4[:])), uint64(binary.BigEndian.Uint32(b4[:]))
		if a > b {
			return nil, errors.New("reversed pool range")
		}
		for a <= b {
			power := bits.TrailingZeros32(uint32(a))
			for uint64(1)<<power > b-a+1 {
				power--
			}
			var addr [4]byte
			binary.BigEndian.PutUint32(addr[:], uint32(a))
			result = append(result, netip.PrefixFrom(netip.AddrFrom4(addr), 32-power).String())
			a += uint64(1) << power
			if len(result) > 128 {
				return nil, errors.New("pool has too many ranges")
			}
		}
	}
	if len(result) == 0 || len(result) > 128 {
		return nil, errors.New("pool has no usable IPv4 ranges")
	}
	return result, nil
}

func Read(ctx context.Context, c routers.API, servers []domain.Server, routerID string) (Options, error) {
	o := Options{Interfaces: []Interface{}, Sources: []Source{}}
	rows, e := routers.List(ctx, c, "interface")
	if e != nil {
		return o, e
	}
	for _, r := range rows {
		if r[".id"] != "" && r["name"] != "" && r["disabled"] != "yes" && r["disabled"] != "true" {
			o.Interfaces = append(o.Interfaces, Interface{ID: r[".id"], Name: r["name"], Type: r["type"], Running: r["running"] == "true" || r["running"] == "yes"})
		}
	}
	for _, s := range servers {
		if s.Spec.RouterID != routerID {
			continue
		}
		p, e := netip.ParsePrefix(s.Spec.Subnet)
		if e == nil && p.Addr().Is4() && p.Bits() > 0 && p == p.Masked() {
			o.Sources = append(o.Sources, Source{ID: "server:" + s.ID, Label: s.Spec.Name + " · " + p.String(), Addresses: []string{p.String()}})
		}
	}
	pools, e := routers.List(ctx, c, "ip/pool")
	if e != nil {
		return o, e
	}
	for _, r := range pools {
		addresses, e := PoolAddresses(r["ranges"])
		if e == nil && r[".id"] != "" {
			o.Sources = append(o.Sources, Source{ID: "pool:" + r[".id"], Label: r["name"] + " · " + r["ranges"], Addresses: addresses})
		}
	}
	addresses, e := routers.List(ctx, c, "ip/address")
	if e != nil {
		return o, e
	}
	for _, r := range addresses {
		p, e := netip.ParsePrefix(r["address"])
		if e == nil && p.Addr().Is4() && p.Bits() > 0 && r[".id"] != "" && r["disabled"] != "yes" && r["disabled"] != "true" {
			o.Sources = append(o.Sources, Source{ID: "address:" + r[".id"], Label: r["interface"] + " · " + p.Masked().String(), Addresses: []string{p.Masked().String()}})
		}
	}
	o.Filter, e = routers.List(ctx, c, "ip/firewall/filter")
	if e != nil {
		return o, e
	}
	o.NAT, e = routers.List(ctx, c, "ip/firewall/nat")
	if e != nil {
		return o, e
	}
	o.ForwardAnchor, e = Anchor(o.Filter, "forward")
	if e != nil {
		o.ForwardAnchor = ""
	}
	o.NATAnchor, e = Anchor(o.NAT, "srcnat")
	return o, e
}

func Resolve(o Options, s domain.NetworkSpec) (domain.NetworkSpec, error) {
	sourceFound, interfaceFound := false, false
	for _, source := range o.Sources {
		if source.ID == s.SourceID {
			s.SourceLabel = source.Label
			s.Addresses = append([]string{}, source.Addresses...)
			sourceFound = true
			break
		}
	}
	for _, iface := range o.Interfaces {
		if iface.ID == s.OutInterfaceID {
			s.OutInterface = iface.Name
			interfaceFound = true
			break
		}
	}
	if !sourceFound || !interfaceFound {
		return s, errors.New("select a current source range and an enabled interface from this router")
	}
	if s.AllowForward && o.ForwardAnchor == "" {
		return s, errors.New("forward chain begins with a dynamic rule; NAT-only is available, but forwarding requires manual order review")
	}
	if s.NATAnchor != o.NATAnchor || s.AllowForward && s.ForwardAnchor != o.ForwardAnchor {
		return s, errors.New("firewall or NAT order changed; reload the router selections")
	}
	return s, nil
}

func Plan(id string, s domain.NetworkSpec) *domain.Job {
	j := &domain.Job{ID: id, RouterID: s.RouterID, Kind: "network", State: "READY", Network: &s}
	list := "mvm-net-" + id[:12]
	marker := "mvm:" + id
	add := func(path string, v map[string]string, anchor string) {
		v["comment"] = marker
		v["disabled"] = "no"
		if anchor != "" && anchor != "__empty__" {
			v["place-before"] = anchor
		}
		j.Steps = append(j.Steps, domain.Step{Path: path, Method: "PUT", Values: v})
	}
	for _, address := range s.Addresses {
		add("ip/firewall/address-list", map[string]string{"list": list, "address": strings.TrimSuffix(address, "/32")}, "")
	}
	add("ip/firewall/nat", map[string]string{"chain": "srcnat", "action": "masquerade", "src-address-list": list, "out-interface": s.OutInterface, "ipsec-policy": "out,none"}, s.NATAnchor)
	if s.AllowForward {
		add("ip/firewall/filter", map[string]string{"chain": "forward", "action": "accept", "dst-address-list": list, "in-interface": s.OutInterface, "connection-state": "established,related"}, s.ForwardAnchor)
		add("ip/firewall/filter", map[string]string{"chain": "forward", "action": "accept", "src-address-list": list, "out-interface": s.OutInterface, "ipsec-policy": "out,none"}, s.ForwardAnchor)
	}
	return j
}

func VerifyOrder(ctx context.Context, c routers.API, j *domain.Job) error {
	if e := VerifySources(ctx, c, j); e != nil {
		return e
	}
	for _, menu := range []struct{ path, chain, anchor string }{{"ip/firewall/nat", "srcnat", j.Network.NATAnchor}, {"ip/firewall/filter", "forward", j.Network.ForwardAnchor}} {
		if menu.path == "ip/firewall/filter" && !j.Network.AllowForward {
			continue
		}
		rows, e := routers.List(ctx, c, menu.path)
		if e != nil {
			return e
		}
		expected := []string{}
		for _, r := range j.Owned {
			if r.Path == menu.path && r.Desired["chain"] == menu.chain {
				expected = append(expected, r.ID)
			}
		}
		index := 0
		found := false
		for _, row := range rows {
			if row["chain"] != menu.chain {
				continue
			}
			if index < len(expected) {
				if row[".id"] != expected[index] {
					return errors.New("network rules were not inserted in the reviewed order")
				}
				index++
				continue
			}
			if row[".id"] != menu.anchor {
				return errors.New("network firewall anchor changed")
			}
			found = true
			break
		}
		if !found && (menu.anchor != "__empty__" || index != len(expected)) {
			return errors.New("network firewall anchor is missing")
		}
	}
	return nil
}

func VerifySources(ctx context.Context, c routers.API, j *domain.Job) error {
	rows, e := routers.List(ctx, c, "ip/firewall/address-list")
	if e != nil {
		return e
	}
	list := "mvm-net-" + j.ID[:12]
	expected := map[string]domain.Resource{}
	for _, r := range j.Owned {
		if r.Path == "ip/firewall/address-list" {
			expected[r.ID] = r
		}
	}
	seen := 0
	for _, row := range rows {
		if row["list"] != list {
			continue
		}
		resource, ok := expected[row[".id"]]
		if !ok || row["address"] != resource.Desired["address"] || row["comment"] != resource.Desired["comment"] {
			return errors.New("selected NAT address list differs from the approved source ranges")
		}
		seen++
	}
	if seen != len(expected) {
		return errors.New("selected NAT source entries are missing")
	}
	return nil
}
