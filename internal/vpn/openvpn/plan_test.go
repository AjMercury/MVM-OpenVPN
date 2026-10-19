package openvpn

import (
	"mikrotik-vpn-manager/internal/domain"
	"testing"
)

func TestCertificateIdentityAndActivationOrder(t *testing.T) {
	for _, test := range []struct{ endpoint, san string }{{"vpn.example.com", "DNS:vpn.example.com"}, {"203.0.113.10", "IP:203.0.113.10"}, {"2001:db8::1", "IP:2001:db8::1"}} {
		t.Run(test.endpoint, func(t *testing.T) {
			job := Plan("0123456789abcdef", domain.ServerSpec{Endpoint: test.endpoint, Subnet: "10.80.0.0/24", Allowed: []string{"192.168.1.0/24"}})
			if job.Steps[0].Path != "certificate" || job.Steps[1].Path != "certificate/sign" || job.Steps[2].Values["subject-alt-name"] != test.san || job.Steps[3].Path != "certificate/sign" {
				t.Fatal("certificate identity or signing dependencies changed")
			}
			last := job.Steps[len(job.Steps)-1]
			if last.Path != "interface/ovpn-server/server" || last.Values["disabled"] != "no" {
				t.Fatal("server must activate after certificates, network and firewall")
			}
		})
	}
}
