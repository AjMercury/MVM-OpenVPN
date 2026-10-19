package app

import (
	"context"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/ipam"
	"mikrotik-vpn-manager/internal/jobs"
	"mikrotik-vpn-manager/internal/provisioning"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/vpn/openvpn"
	"net/netip"
)

type wireGuardKey struct {
	Private string `json:"private"`
}

func (s *Service) prepareWireGuardUser(ctx context.Context, a domain.Account, server domain.Server, in UserRequest) (*domain.Job, error) {
	r, e := s.Router(a, server.Spec.RouterID, "vpn")
	if e != nil {
		return nil, e
	}
	unlock := s.Engine.Lock(r.ID)
	defer unlock()
	if !openvpn.Identifier.MatchString(in.Username) || len(in.Key) < 8 || len(in.Key) > 128 {
		return nil, errors.New("invalid peer name or idempotency key")
	}
	if in.AccountID != "" {
		var owner domain.Account
		if e = s.Repo.Get("accounts", in.AccountID, &owner); e != nil || owner.Role != domain.EndUser {
			return nil, errors.New("profile owner must be an existing end-user account")
		}
	}
	c, e := s.Connect(r)
	if e != nil {
		return nil, e
	}
	if e = s.Engine.VerifyOwned(ctx, c, server.JobID); e != nil {
		return nil, e
	}
	peers, e := routers.List(ctx, c, "interface/wireguard/peers")
	if e != nil {
		return nil, e
	}
	var users []domain.VPNUser
	var allJobs []domain.Job
	if e = s.Repo.List("users", &users); e != nil {
		return nil, e
	}
	if e = s.Repo.List("jobs", &allJobs); e != nil {
		return nil, e
	}
	used := map[string]bool{}
	for _, u := range users {
		if u.RouterID == r.ID {
			used[u.Address] = true
			if u.ServerID == server.ID && u.Username == in.Username {
				return nil, errors.New("peer name is already managed")
			}
		}
	}
	for _, j := range allJobs {
		if j.RouterID == r.ID && j.User != nil && j.State != "EXPIRED" && j.State != "ROLLED_BACK" {
			used[j.User.Address] = true
			if j.User.ServerID == server.ID && j.User.Username == in.Username {
				return nil, errors.New("peer name reserved by another job")
			}
		}
	}
	if e = checkWireGuardScope(server, peers, users); e != nil {
		return nil, e
	}
	if in.Address == "" {
		start, _ := netip.ParseAddr(server.Spec.Start)
		end, _ := netip.ParseAddr(server.Spec.End)
		for addr := start; addr.IsValid() && addr.Compare(end) <= 0; addr = addr.Next() {
			if !used[addr.String()] {
				in.Address = addr.String()
				break
			}
		}
	}
	if !ipam.Contains(server.Spec, in.Address) || used[in.Address] {
		return nil, errors.New("address unavailable or outside server pool")
	}
	private, public, e := provisioning.WireGuardKeypair()
	if e != nil {
		return nil, e
	}
	id := ID()
	u := domain.VPNUser{ID: id, JobID: id, Protocol: "wireguard", RouterID: r.ID, ServerID: server.ID, AccountID: in.AccountID, Username: in.Username, Address: in.Address, PublicKey: public}
	j := &domain.Job{ID: id, RouterID: r.ID, Actor: a.ID, Kind: "user", Key: in.Key, Fingerprint: security.Digest(jobs.Fingerprint(in)), State: "READY", User: &u, Steps: []domain.Step{{Path: "interface/wireguard/peers", Method: "PUT", Values: map[string]string{"interface": server.Spec.Name, "public-key": public, "allowed-address": in.Address + "/32", "disabled": "no", "comment": "mvm:" + id}}}}
	// This collection is encrypted by both repositories and never returned by an API.
	if e = s.Repo.Put("wireguard_keys", id, wireGuardKey{Private: private}); e != nil {
		return nil, e
	}
	if e = s.Engine.Prepare(j); e != nil {
		_ = s.Repo.Delete("wireguard_keys", id)
		return nil, e
	}
	return j, s.Audit(a.ID, "wireguard.user.preview", id, "ready")
}

func (s *Service) exportWireGuard(ctx context.Context, a domain.Account, u domain.VPNUser) ([]byte, error) {
	if !s.CanProfile(a, u) || u.Disabled {
		return nil, errors.New("profile access denied")
	}
	unlock := s.Engine.Lock(u.RouterID)
	defer unlock()
	if e := s.Repo.Get("users", u.ID, &u); e != nil || u.Disabled || !s.CanProfile(a, u) {
		return nil, errors.New("profile unavailable")
	}
	var server domain.Server
	if e := s.Repo.Get("servers", u.ServerID, &server); e != nil {
		return nil, e
	}
	var r domain.Router
	e := s.Repo.Get("routers", u.RouterID, &r)
	if e != nil {
		return nil, e
	}
	c, e := s.Connect(r)
	if e != nil {
		return nil, e
	}
	if e = s.Engine.VerifyOwned(ctx, c, u.JobID); e != nil {
		return nil, e
	}
	if e = s.Engine.VerifyOwned(ctx, c, server.JobID); e != nil {
		return nil, e
	}
	peers, e := routers.List(ctx, c, "interface/wireguard/peers")
	if e != nil {
		return nil, e
	}
	if e = s.verifyWireGuardScope(ctx, c, server, peers); e != nil {
		return nil, e
	}
	row, e := routers.Find(ctx, c, "interface/wireguard", "name", server.Spec.Name)
	if e != nil {
		return nil, e
	}
	var key wireGuardKey
	if e = s.Repo.Get("wireguard_keys", u.ID, &key); e != nil {
		return nil, e
	}
	profile, e := provisioning.WireGuardProfile(server, u, key.Private, row["public-key"])
	if e != nil {
		return nil, e
	}
	if e = s.Audit(a.ID, "wireguard.profile.export", u.ID, "verified"); e != nil {
		return nil, e
	}
	return profile, nil
}

func (s *Service) wireGuardAction(ctx context.Context, a domain.Account, u domain.VPNUser, action string) error {
	if action != "disable" {
		return errors.New("WireGuard peers support disable; there is no PPP session to disconnect")
	}
	r, e := s.Router(a, u.RouterID, "vpn")
	if e != nil {
		return e
	}
	if !s.Config.Mutations {
		return errors.New("router mutations disabled")
	}
	unlock := s.Engine.Lock(r.ID)
	defer unlock()
	if s.Engine.Blocked(r.ID, "") {
		return errors.New("router has unresolved operation")
	}
	c, e := s.Connect(r)
	if e != nil {
		return e
	}
	rows, e := routers.List(ctx, c, "interface/wireguard/peers/"+u.RemoteID)
	if e != nil {
		return e
	}
	if len(rows) != 1 || rows[0]["public-key"] != u.PublicKey || rows[0]["comment"] != "mvm:"+u.JobID {
		return errors.New("WireGuard peer ownership changed")
	}
	var server domain.Server
	if e = s.Repo.Get("servers", u.ServerID, &server); e != nil {
		return e
	}
	if rows[0]["interface"] != server.Spec.Name || rows[0]["allowed-address"] != u.Address+"/32" {
		return errors.New("WireGuard peer scope changed")
	}
	if e = s.Audit(a.ID, "wireguard.user.disable", u.ID, "approved"); e != nil {
		return e
	}
	if _, e = c.Call(ctx, "PATCH", "interface/wireguard/peers/"+u.RemoteID, routers.Record{"disabled": "yes"}); e != nil {
		return e
	}
	rows, e = routers.List(ctx, c, "interface/wireguard/peers/"+u.RemoteID)
	if e != nil || len(rows) != 1 || rows[0]["disabled"] != "yes" && rows[0]["disabled"] != "true" {
		return errors.New("peer disable read-back failed")
	}
	u.Disabled = true
	if e = s.Repo.Put("users", u.ID, u); e != nil {
		return e
	}
	return s.Audit(a.ID, "wireguard.user.disable", u.ID, "verified")
}

func (s *Service) verifyWireGuardScope(ctx context.Context, c routers.API, server domain.Server, peers []routers.Record) error {
	var users []domain.VPNUser
	if e := s.Repo.List("users", &users); e != nil {
		return e
	}
	return checkWireGuardScope(server, peers, users)
}
func checkWireGuardScope(server domain.Server, peers []routers.Record, users []domain.VPNUser) error {
	for _, peer := range peers {
		if peer["interface"] != server.Spec.Name {
			continue
		}
		known := false
		for _, user := range users {
			if user.ServerID == server.ID && user.RemoteID == peer[".id"] && user.PublicKey == peer["public-key"] && peer["allowed-address"] == user.Address+"/32" && peer["comment"] == "mvm:"+user.JobID {
				known = true
				break
			}
		}
		if !known {
			return errors.New("existing peers require isolation review before enrollment")
		}
	}
	return nil
}
