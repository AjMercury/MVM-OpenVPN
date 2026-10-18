package ipam

import (
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"net/netip"
	"strings"
)

func Range(subnet, start, end, gateway string) error {
	p, e := netip.ParsePrefix(subnet)
	if e != nil || !p.Addr().Is4() || p.Bits() < 16 || p.Bits() > 30 || p != p.Masked() {
		return errors.New("use a canonical IPv4 /16–/30 VPN subnet")
	}
	a, e1 := netip.ParseAddr(start)
	b, e2 := netip.ParseAddr(end)
	g, e3 := netip.ParseAddr(gateway)
	if e1 != nil || e2 != nil || e3 != nil || !a.Is4() || !b.Is4() || !g.Is4() || !p.Contains(a) || !p.Contains(b) || !p.Contains(g) || a.Compare(b) > 0 {
		return errors.New("invalid pool range or gateway outside subnet")
	}
	last := p.Addr()
	raw := last.As4()
	for bit := p.Bits(); bit < 32; bit++ {
		raw[bit/8] |= 1 << uint(7-bit%8)
	}
	broadcast := netip.AddrFrom4(raw)
	if a == p.Addr() || b == broadcast || g == p.Addr() || g == broadcast || g.Compare(a) >= 0 && g.Compare(b) <= 0 {
		return errors.New("pool must exclude network, broadcast and gateway")
	}
	return nil
}
func Contains(s domain.ServerSpec, address string) bool {
	a, e := netip.ParseAddr(address)
	start, _ := netip.ParseAddr(s.Start)
	end, _ := netip.ParseAddr(s.End)
	return e == nil && a.Is4() && a.Compare(start) >= 0 && a.Compare(end) <= 0
}
func Overlap(subnet string, addresses, routes, pools []routers.Record) error {
	p, _ := netip.ParsePrefix(subnet)
	for _, rows := range [][]routers.Record{addresses, routes} {
		for _, r := range rows {
			raw := r["address"]
			if raw == "" {
				raw = r["dst-address"]
			}
			q, e := netip.ParsePrefix(raw)
			if e != nil {
				return errors.New("cannot parse existing router network; manual review required")
			}
			if q.Bits() == 0 {
				continue
			}
			if p.Overlaps(q) {
				return errors.New("VPN subnet overlaps an existing router address or route")
			}
		}
	}
	for _, r := range pools {
		for _, part := range strings.Split(r["ranges"], ",") {
			parts := strings.Split(strings.TrimSpace(part), "-")
			if len(parts) != 2 {
				return errors.New("unrecognized existing pool range; manual review required")
			}
			a, e := netip.ParseAddr(parts[0])
			b, e2 := netip.ParseAddr(parts[1])
			if e != nil || e2 != nil || !a.Is4() || !b.Is4() {
				return errors.New("invalid existing pool range")
			}
			if p.Contains(a) || p.Contains(b) || a.Compare(p.Addr()) <= 0 && b.Compare(p.Addr()) >= 0 {
				return errors.New("VPN subnet overlaps existing address pool")
			}
		}
	}
	return nil
}
