package protocols

import (
	"context"
	"errors"
	"fmt"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"strings"
)

func l2tpPlan(ctx context.Context, c routers.API, j *domain.Job, s domain.ServerSpec, marker string) (*domain.Job, error) {
	if s.Port != 1701 || len(s.IPsecSecret) < 14 || len(s.IPsecSecret) > 128 || strings.ContainsAny(s.IPsecSecret, "\r\n") {
		return nil, errors.New("L2TP requires UDP 1701 and a 14–128 byte IPsec shared secret")
	}
	j.Server.Spec.IPsecSecret = ""
	j.Server.CA = ""
	j.Server.Certificate = ""
	addRule := func(v map[string]string) {
		v["comment"] = marker
		if s.InputAnchor != "__empty__" {
			v["place-before"] = s.InputAnchor
		}
		j.Steps = append(j.Steps, domain.Step{Path: "ip/firewall/filter", Method: "PUT", Values: v})
	}
	addRule(map[string]string{"chain": "input", "protocol": "ipsec-esp", "action": "accept"})
	addRule(map[string]string{"chain": "input", "protocol": "udp", "dst-port": "1701", "ipsec-policy": "in,ipsec", "action": "accept"})
	addRule(map[string]string{"chain": "input", "protocol": "udp", "dst-port": "1701", "action": "drop"})
	values := map[string]string{"enabled": "yes", "default-profile": j.Server.Profile, "authentication": "mschap2", "use-ipsec": "required", "ipsec-secret": s.IPsecSecret}
	rows, e := routers.List(ctx, c, "interface/l2tp-server/server")
	if e != nil || len(rows) != 1 {
		return nil, errors.New("cannot read L2TP settings for rollback")
	}
	oldSecret, ok := rows[0]["ipsec-secret"]
	if !ok || oldSecret != "" {
		return nil, errors.New("L2TP existing IPsec secret must be empty and observable before provisioning")
	}
	previous := map[string]string{}
	for key := range values {
		v, ok := rows[0][key]
		if !ok {
			return nil, fmt.Errorf("L2TP setting %s is not observable", key)
		}
		previous[key] = v
	}
	j.Steps = append(j.Steps, domain.Step{Path: "interface/l2tp-server/server/set", Method: "POST", Values: values, Previous: previous, Secret: true})
	return j, nil
}
