package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/storage"
	"mikrotik-vpn-manager/internal/vpn/network"
	"strings"
	"sync"
	"time"
)

type Engine struct {
	Repo  storage.Repository
	locks sync.Map
}

func (e *Engine) Lock(router string) func() {
	m, _ := e.locks.LoadOrStore(router, &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	return m.(*sync.Mutex).Unlock
}
func (e *Engine) Save(j *domain.Job) error {
	if j.User != nil && j.User.Protocol == "wireguard" && (j.State == "EXPIRED" || j.State == "FAILED" || j.State == "ROLLED_BACK") {
		if err := e.Repo.Delete("wireguard_keys", j.User.ID); err != nil {
			return err
		}
	}
	if j.State == "COMPLETED" || j.State == "ROLLED_BACK" || j.State == "FAILED" || j.State == "EXPIRED" || j.State == "MANUAL_INTERVENTION" {
		for i := range j.Steps {
			delete(j.Steps[i].Values, "password")
			delete(j.Steps[i].Values, "ipsec-secret")
		}
	}
	j.Updated = time.Now().UTC()
	return e.Repo.Put("jobs", j.ID, j)
}
func (e *Engine) Prepare(j *domain.Job) error {
	var jobs []domain.Job
	if err := e.Repo.List("jobs", &jobs); err != nil {
		return err
	}
	for _, other := range jobs {
		if other.RouterID == j.RouterID && other.Key == j.Key {
			if other.Fingerprint != j.Fingerprint {
				return errors.New("idempotency key already belongs to a different request")
			}
			return errors.New("idempotent operation already exists: " + other.ID)
		}
	}
	j.Created = time.Now().UTC()
	return e.Save(j)
}
func (e *Engine) Recover() error {
	var jobs []domain.Job
	if err := e.Repo.List("jobs", &jobs); err != nil {
		return err
	}
	for _, j := range jobs {
		if j.State == "APPLYING" || j.State == "VERIFYING" || j.State == "ROLLING_BACK" {
			j.State = "MANUAL_INTERVENTION"
			j.Error = "Process interrupted; compare owned resources with RouterOS before any retry"
			if err := e.Save(&j); err != nil {
				return err
			}
		}
	}
	return nil
}
func (e *Engine) Blocked(router, except string) bool {
	var all []domain.Job
	if e.Repo.List("jobs", &all) != nil {
		return true
	}
	for _, j := range all {
		if j.RouterID == router && j.ID != except && (j.State == "MANUAL_INTERVENTION" || j.State == "APPLYING" || j.State == "VERIFYING" || j.State == "ROLLING_BACK") {
			return true
		}
	}
	return false
}
func (e *Engine) Apply(ctx context.Context, c routers.API, j *domain.Job) error {
	if j.State == "COMPLETED" {
		return nil
	}
	if j.State != "READY" {
		return errors.New("job is not READY")
	}
	if time.Since(j.Created) > 15*time.Minute {
		return errors.New("preview expired; prepare and review a new job")
	}
	if e.Blocked(j.RouterID, j.ID) {
		return errors.New("router has an unresolved operation")
	}
	// Recheck singleton settings before any write, so a preview cannot overwrite
	// settings changed by another administrator after review.
	for _, step := range j.Steps {
		if j.Network != nil && step.Path == "ip/firewall/nat" {
			if err := network.VerifySources(ctx, c, j); err != nil {
				j.State = "MANUAL_INTERVENTION"
				j.Error = "NAT source ranges could not be verified"
				_ = e.Save(j)
				return errors.New(j.Error)
			}
		}
		if step.Previous != nil {
			path := strings.TrimSuffix(step.Path, "/set")
			if err := verify(ctx, c, domain.Resource{Path: path, Desired: step.Previous}); err != nil {
				return errors.New("service settings changed after preview; prepare a new job")
			}
		}
	}
	j.State = "APPLYING"
	if err := e.Save(j); err != nil {
		return err
	}
	for _, step := range j.Steps {
		if j.Network != nil && step.Path == "ip/firewall/filter" && step.Values["src-address-list"] != "" {
			if err := network.VerifyOrder(ctx, c, j); err != nil {
				j.State = "MANUAL_INTERVENTION"
				j.Error = "NAT/firewall order could not be verified; outbound forwarding was blocked"
				_ = e.Save(j)
				return errors.New(j.Error)
			}
		}
		if j.Network != nil && step.Path == "ip/firewall/nat" {
			if err := network.VerifySources(ctx, c, j); err != nil {
				j.State = "MANUAL_INTERVENTION"
				j.Error = "NAT source ranges could not be verified"
				_ = e.Save(j)
				return errors.New(j.Error)
			}
		}
		if j.Server != nil && (step.Path == "interface/ovpn-server/server" || step.Path == "interface/wireguard" || step.Path == "interface/sstp-server/server/set" || step.Path == "interface/l2tp-server/server/set" || step.Path == "ip/ipsec/peer") {
			if err := e.verifyFirewallOrder(ctx, c, j); err != nil {
				j.State = "MANUAL_INTERVENTION"
				j.Error = "Firewall order could not be verified; server activation was blocked"
				_ = e.Save(j)
				return errors.New(j.Error)
			}
		}
		j.InFlight = true
		if err := e.Save(j); err != nil {
			return err
		}
		rows, err := c.Call(ctx, step.Method, step.Path, step.Values)
		if err != nil {
			j.Error = "Router step failed: " + step.Path
			j.State = "MANUAL_INTERVENTION"
			// Only a definite HTTP rejection is safe to compensate. Transport errors may have committed.
			var apiErr *routers.APIError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 408 {
				j.InFlight = false
				j.State = "ROLLING_BACK"
				if saveErr := e.Save(j); saveErr != nil {
					return saveErr
				}
				rollbackCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				rb := e.rollback(rollbackCtx, c, j)
				cancel()
				if rb != nil {
					j.State = "MANUAL_INTERVENTION"
				} else {
					j.State = "ROLLED_BACK"
				}
			}
			if saveErr := e.Save(j); saveErr != nil {
				return saveErr
			}
			return err
		}
		if step.Method == "PUT" {
			if len(rows) != 1 || !validID(rows[0][".id"]) {
				j.State = "MANUAL_INTERVENTION"
				j.Error = "Mutation returned no usable resource ID"
				_ = e.Save(j)
				return errors.New(j.Error)
			}
			desired := map[string]string{}
			for k, v := range step.Values {
				if k != "place-before" && k != "password" {
					desired[k] = v
				}
			}
			j.Owned = append(j.Owned, domain.Resource{Path: step.Path, ID: rows[0][".id"], Marker: "mvm:" + j.ID, Desired: desired})
		}
		if step.Previous != nil {
			desired := map[string]string{}
			for k, v := range step.Values {
				if k != "ipsec-secret" {
					desired[k] = v
				}
			}
			j.Owned = append(j.Owned, domain.Resource{Path: strings.TrimSuffix(step.Path, "/set"), Desired: desired, Previous: step.Previous})
		}
		j.InFlight = false
		if err := e.Save(j); err != nil {
			return err
		}
		if step.Method == "POST" && step.Path == "certificate/sign" {
			if err := waitSigned(ctx, c, step.Values["name"]); err != nil {
				j.State = "MANUAL_INTERVENTION"
				j.Error = "Certificate signing could not be verified; inspect certificate state"
				_ = e.Save(j)
				return errors.New(j.Error)
			}
		}
	}
	j.State = "VERIFYING"
	if err := e.Save(j); err != nil {
		return err
	}
	for _, r := range j.Owned {
		if err := verify(ctx, c, r); err != nil {
			j.State = "MANUAL_INTERVENTION"
			j.Error = "Read-back mismatch for " + r.Path
			_ = e.Save(j)
			return err
		}
	}
	if j.Server != nil {
		if err := e.verifyFirewallOrder(ctx, c, j); err != nil {
			j.State = "MANUAL_INTERVENTION"
			j.Error = "Firewall ordering differs from the approved policy"
			_ = e.Save(j)
			return err
		}
	}
	if j.Network != nil {
		if err := network.VerifyOrder(ctx, c, j); err != nil {
			j.State = "MANUAL_INTERVENTION"
			j.Error = "NAT/firewall order differs from the reviewed plan"
			_ = e.Save(j)
			return err
		}
	}
	if j.Server != nil {
		for _, r := range j.Owned {
			if r.Path == "interface/ovpn-server/server" || r.Path == "interface/wireguard" || r.Path == "ip/ipsec/peer" {
				j.Server.RemoteID = r.ID
			}
		}
		if err := e.Repo.Put("servers", j.Server.ID, j.Server); err != nil {
			return err
		}
	}
	if j.User != nil {
		for _, r := range j.Owned {
			if r.Path == "ppp/secret" || r.Path == "interface/wireguard/peers" {
				j.User.RemoteID = r.ID
			}
		}
		if err := e.Repo.Put("users", j.User.ID, j.User); err != nil {
			return err
		}
	}
	for i := range j.Steps {
		delete(j.Steps[i].Values, "password")
	}
	j.State = "COMPLETED"
	return e.Save(j)
}

func waitSigned(ctx context.Context, c routers.API, name string) error {
	waitCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		cert, err := routers.Find(waitCtx, c, "certificate", "name", name)
		if err == nil && cert["fingerprint"] != "" {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return errors.New("certificate readiness timeout")
		case <-ticker.C:
		}
	}
}

func (e *Engine) verifyFirewallOrder(ctx context.Context, c routers.API, j *domain.Job) error {
	var all []domain.Job
	if err := e.Repo.List("jobs", &all); err != nil {
		return err
	}
	others := []domain.Resource{}
	for _, other := range all {
		if other.ID == j.ID || other.RouterID != j.RouterID || other.State != "COMPLETED" || other.Server == nil && other.Network == nil {
			continue
		}
		if other.Network != nil {
			if err := network.VerifySources(ctx, c, &other); err != nil {
				return err
			}
		}
		for _, r := range other.Owned {
			if err := verify(ctx, c, r); err != nil {
				return errors.New("another managed network resource changed; review router state")
			}
			if r.Path == "ip/firewall/filter" {
				others = append(others, r)
			}
		}
	}
	return verifyFirewallOrder(ctx, c, j, others...)
}

func verifyFirewallOrder(ctx context.Context, c routers.API, j *domain.Job, others ...domain.Resource) error {
	rows, err := routers.List(ctx, c, "ip/firewall/filter")
	if err != nil {
		return err
	}
	for chain, anchor := range map[string]string{"input": j.Server.Spec.InputAnchor, "forward": j.Server.Spec.ForwardAnchor} {
		expected := []string{}
		for _, r := range j.Owned {
			if r.Path == "ip/firewall/filter" && r.Desired["chain"] == chain {
				expected = append(expected, r.ID)
			}
		}
		index := 0
		found := false
		for _, r := range rows {
			if r["chain"] != chain {
				continue
			}
			if index < len(expected) && r[".id"] == expected[index] {
				index++
				continue
			}
			if index == len(expected) && r[".id"] == anchor {
				found = true
				break
			}
			known := false
			for _, other := range others {
				if r[".id"] != other.ID {
					continue
				}
				known = true
				for key, value := range other.Desired {
					if !equivalent(key, value, r[key]) {
						return errors.New("managed firewall rule differs from its approved configuration")
					}
				}
				break
			}
			if known {
				continue
			}
			if index < len(expected) {
				return errors.New("managed firewall rules are not at the beginning of their chain")
			}
			if r[".id"] != anchor {
				return errors.New("firewall anchor moved")
			}
			found = true
			break
		}
		if !found {
			if anchor != "__empty__" || index != len(expected) {
				return errors.New("firewall anchor missing")
			}
		}
	}
	return nil
}

// VerifyOwned is a read-only guard used before expanding an existing server's identities.
func (e *Engine) VerifyOwned(ctx context.Context, c routers.API, jobID string) error {
	var j domain.Job
	if err := e.Repo.Get("jobs", jobID, &j); err != nil {
		return err
	}
	if j.State != "COMPLETED" {
		return errors.New("server provisioning is not complete")
	}
	for _, r := range j.Owned {
		if err := verify(ctx, c, r); err != nil {
			return err
		}
	}
	if j.Server != nil {
		return e.verifyFirewallOrder(ctx, c, &j)
	}
	return nil
}
func validID(id string) bool {
	if !strings.HasPrefix(id, "*") || len(id) < 2 {
		return false
	}
	for _, r := range id[1:] {
		if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'F' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func equivalent(k, want, got string) bool {
	if want == got {
		return true
	}
	if want == "yes" && got == "true" || want == "no" && got == "false" {
		return true
	}
	if k == "key-usage" {
		a := strings.Split(want, ",")
		b := strings.Split(got, ",")
		if len(a) != len(b) {
			return false
		}
		for _, x := range a {
			found := false
			for _, y := range b {
				if x == y {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	return false
}
func verify(ctx context.Context, c routers.API, r domain.Resource) error {
	path := r.Path
	if r.ID != "" {
		path += "/" + r.ID
	}
	rows, err := routers.List(ctx, c, path)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("owned resource not found")
	}
	for k, v := range r.Desired {
		if !equivalent(k, v, rows[0][k]) {
			return errors.New("resource differs from approved configuration")
		}
	}
	return nil
}
func (e *Engine) rollback(ctx context.Context, c routers.API, j *domain.Job) error {
	for i := len(j.Owned) - 1; i >= 0; i-- {
		r := j.Owned[i]
		if err := verify(ctx, c, r); err != nil {
			return err
		}
		if r.Previous != nil {
			if _, err := c.Call(ctx, "POST", r.Path+"/set", r.Previous); err != nil {
				return err
			}
			if err := verify(ctx, c, domain.Resource{Path: r.Path, Desired: r.Previous}); err != nil {
				return err
			}
			j.Owned = j.Owned[:i]
			if err := e.Save(j); err != nil {
				return err
			}
			continue
		}
		if err := noExternalDependencies(ctx, c, r, j.Owned); err != nil {
			return err
		}
		if _, err := c.Call(ctx, "DELETE", r.Path+"/"+r.ID, nil); err != nil {
			return err
		}
		j.Owned = j.Owned[:i]
		if err := e.Save(j); err != nil {
			return err
		}
	}
	return nil
}

func noExternalDependencies(ctx context.Context, c routers.API, r domain.Resource, owned []domain.Resource) error {
	name := r.Desired["name"]
	if r.Path == "ip/firewall/address-list" {
		name = r.Desired["list"]
	}
	if name == "" {
		return nil
	}
	refs := map[string][]string{}
	switch r.Path {
	case "ip/firewall/address-list":
		refs = map[string][]string{"ip/firewall/filter": {"src-address-list", "dst-address-list"}, "ip/firewall/nat": {"src-address-list", "dst-address-list"}, "ip/firewall/mangle": {"src-address-list", "dst-address-list"}, "ip/firewall/raw": {"src-address-list", "dst-address-list"}}
	case "ip/pool":
		refs = map[string][]string{"ppp/profile": {"remote-address"}, "ppp/secret": {"remote-address"}, "ip/dhcp-server": {"address-pool"}, "ip/ipsec/mode-config": {"address-pool"}}
	case "ppp/profile":
		refs = map[string][]string{"ppp/secret": {"profile"}, "interface/ovpn-server/server": {"default-profile"}, "interface/l2tp-server/server": {"default-profile"}, "interface/sstp-server/server": {"default-profile"}, "interface/pptp-server/server": {"default-profile"}}
	case "certificate":
		refs = map[string][]string{"ip/service": {"certificate"}, "interface/ovpn-server/server": {"certificate"}, "interface/sstp-server/server": {"certificate"}, "ip/ipsec/identity": {"certificate", "remote-certificate"}, "ip/hotspot/profile": {"ssl-certificate"}, "certificate": {"issuer"}}
	case "interface/wireguard":
		refs = map[string][]string{"interface/wireguard/peers": {"interface"}, "ip/address": {"interface"}, "ip/route": {"gateway"}, "interface/list/member": {"interface"}}
	case "ip/ipsec/profile":
		refs = map[string][]string{"ip/ipsec/peer": {"profile"}}
	case "ip/ipsec/proposal":
		refs = map[string][]string{"ip/ipsec/policy": {"proposal"}}
	case "ip/ipsec/mode-config":
		refs = map[string][]string{"ip/ipsec/identity": {"mode-config"}}
	case "ip/ipsec/policy/group":
		refs = map[string][]string{"ip/ipsec/policy": {"group"}, "ip/ipsec/identity": {"policy-template-group"}}
	case "ip/ipsec/peer":
		refs = map[string][]string{"ip/ipsec/identity": {"peer"}}
	default:
		return nil
	}
	for menu, fields := range refs {
		rows, err := routers.List(ctx, c, menu)
		if err != nil {
			return errors.New("cannot establish rollback dependency safety")
		}
		for _, row := range rows {
			isOwned := false
			for _, o := range owned {
				if o.Path == menu && o.ID == row[".id"] {
					isOwned = true
					break
				}
			}
			if isOwned {
				continue
			}
			if menu == "certificate" && strings.Contains(r.Desired["key-usage"], "key-cert-sign") {
				return errors.New("external certificates exist; CA dependency review required")
			}
			for _, field := range fields {
				for _, value := range strings.Split(row[field], ",") {
					if value == name || value == r.ID {
						return errors.New("external resource depends on this job's resource")
					}
				}
			}
		}
	}
	return nil
}
func Fingerprint(value any) string { b, _ := json.Marshal(value); return string(b) }
