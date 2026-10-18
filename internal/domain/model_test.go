package domain

import "testing"

func TestRBAC(t *testing.T) {
	for _, v := range []struct {
		a              Account
		action, router string
		want           bool
	}{{Account{Role: Super}, "router", "x", true}, {Account{Role: Operator, Routers: []string{"x"}}, "vpn", "x", true}, {Account{Role: Operator, Routers: []string{"x"}}, "router", "x", false}, {Account{Role: Operator, Routers: []string{"x"}}, "vpn", "y", false}, {Account{Role: Auditor, Routers: []string{"x"}}, "read", "x", true}, {Account{Role: Auditor, Routers: []string{"x"}}, "vpn", "x", false}, {Account{Role: EndUser}, "read", "x", false}} {
		if got := v.a.Can(v.action, v.router); got != v.want {
			t.Fatalf("%+v: %v", v, got)
		}
	}
}
