package ipam

import (
	"mikrotik-vpn-manager/internal/routers"
	"testing"
)

func TestRanges(t *testing.T) {
	for _, v := range []struct {
		start, end, gateway string
		want                bool
	}{{"10.8.0.10", "10.8.0.200", "10.8.0.1", true}, {"10.8.0.10", "10.8.0.200", "10.8.0.20", false}, {"10.8.0.0", "10.8.0.200", "10.8.0.1", false}, {"10.8.0.10", "10.8.0.255", "10.8.0.1", false}, {"10.8.1.1", "10.8.1.2", "10.8.0.1", false}, {"10.8.0.200", "10.8.0.10", "10.8.0.1", false}} {
		if (Range("10.8.0.0/24", v.start, v.end, v.gateway) == nil) != v.want {
			t.Fatal(v)
		}
	}
}
func TestOverlap(t *testing.T) {
	if e := Overlap("10.8.0.0/24", []routers.Record{{"address": "10.8.0.1/24"}}, nil, nil); e == nil {
		t.Fatal("overlap accepted")
	}
	if e := Overlap("10.8.0.0/24", nil, []routers.Record{{"dst-address": "0.0.0.0/0"}}, nil); e != nil {
		t.Fatal(e)
	}
	if e := Overlap("10.8.0.0/24", nil, nil, []routers.Record{{"ranges": "10.7.255.1-10.9.0.1"}}); e == nil {
		t.Fatal("enclosing pool accepted")
	}
}
