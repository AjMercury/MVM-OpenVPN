package app

import (
	"context"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/vpn/protocols"
	"sort"
)

func userIdentity(u domain.VPNUser) string {
	if u.IdentityID != "" {
		return u.IdentityID
	}
	return u.ID
}

func (s *Service) blockSharedListener(routerID string) error {
	var users []domain.VPNUser
	if err := s.Repo.List("users", &users); err != nil {
		return err
	}
	for _, u := range users {
		if u.RouterID == routerID && !u.Disabled && len(u.SharedServers) > 0 {
			return errors.New("disable shared PPP users before adding another PPP server; its authentication scope must be reviewed")
		}
	}
	return nil
}

func (s *Service) verifyIdentity(a domain.Account, u domain.VPNUser) error {
	if u.IdentityID == "" {
		return nil
	}
	_, err := s.resolveIdentity(a, UserRequest{IdentityID: u.IdentityID, Username: u.Username, AccountID: u.AccountID}, u.RouterID)
	return err
}

func (s *Service) identityEnabled(u domain.VPNUser) bool {
	if u.IdentityID == "" {
		return true
	}
	var root domain.VPNUser
	return s.Repo.Get("users", u.IdentityID, &root) == nil && !root.Disabled && root.RouterID == u.RouterID && root.Username == u.Username && root.AccountID == u.AccountID
}

func (s *Service) IdentityAction(ctx context.Context, a domain.Account, id, action string) error {
	if action != "disable" && action != "disconnect" {
		return errors.New("unsupported user operation")
	}
	var all []domain.VPNUser
	if err := s.Repo.List("users", &all); err != nil {
		return err
	}
	members := []domain.VPNUser{}
	for _, u := range all {
		if userIdentity(u) == id {
			if !a.Can("vpn", u.RouterID) {
				return errors.New("permission denied")
			}
			members = append(members, u)
		}
	}
	if len(members) == 0 {
		return errors.New("shared identity unavailable")
	}
	var failures []error
	for _, u := range members {
		if action == "disconnect" && u.Protocol == "wireguard" {
			continue
		}
		if err := s.UserAction(ctx, a, u.ID, action); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return errors.New("some service actions could not complete; refresh user status and retry: " + errors.Join(failures...).Error())
	}
	return nil
}

// Linking requires an explicit existing identity on the same router and owner.
func (s *Service) resolveIdentity(a domain.Account, in UserRequest, routerID string) (string, error) {
	if in.IdentityID == "" {
		return "", nil
	}
	var existing domain.VPNUser
	if err := s.Repo.Get("users", in.IdentityID, &existing); err != nil {
		return "", errors.New("shared identity unavailable")
	}
	if !a.Can("vpn", existing.RouterID) || existing.RouterID != routerID || existing.Username != in.Username || existing.AccountID != in.AccountID || existing.Disabled {
		return "", errors.New("shared identity router, username or owner does not match")
	}
	return userIdentity(existing), nil
}

// RouterOS's service=any shares one PPP credential/profile. Refuse other PPP
// listeners, and require every active supported listener to be panel-owned.
func (s *Service) sharedPPPServers(ctx context.Context, c routers.API, routerID string) ([]string, error) {
	for _, path := range []string{"interface/pptp-server/server", "interface/pppoe-server/server"} {
		rows, err := routers.List(ctx, c, path)
		if err != nil {
			return nil, errors.New("cannot verify other PPP services before sharing credentials")
		}
		if path == "interface/pptp-server/server" && len(rows) != 1 {
			return nil, errors.New("cannot verify PPTP is disabled before sharing credentials")
		}
		for _, row := range rows {
			disabled := row["disabled"] == "yes" || row["disabled"] == "true" || row["enabled"] == "no" || row["enabled"] == "false"
			if !disabled {
				return nil, errors.New("disable unmanaged PPTP/PPPoE services before sharing PPP credentials")
			}
		}
	}
	var all []domain.Server
	if err := s.Repo.List("servers", &all); err != nil {
		return nil, err
	}
	ids := []string{}
	managed := map[string]bool{}
	for _, server := range all {
		p := protocols.Name(server.Spec)
		if server.Spec.RouterID != routerID || p == "wireguard" || p == "ikev2" {
			continue
		}
		if err := s.verifyServer(ctx, c, server); err != nil {
			return nil, err
		}
		managed[p] = true
		ids = append(ids, server.ID)
	}
	for _, p := range []string{"openvpn", "sstp", "l2tp"} {
		path := "interface/" + p + "-server/server"
		if p == "openvpn" {
			path = "interface/ovpn-server/server"
		}
		rows, err := routers.List(ctx, c, path)
		if err != nil {
			return nil, errors.New("cannot verify shared PPP listener scope")
		}
		if p != "openvpn" && len(rows) != 1 {
			return nil, errors.New("cannot verify shared PPP singleton settings")
		}
		for _, row := range rows {
			active := row["enabled"] == "yes" || row["enabled"] == "true" || row["disabled"] == "no" || row["disabled"] == "false"
			if active && !managed[p] {
				return nil, errors.New("unmanaged PPP listener must be reviewed before sharing credentials")
			}
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("create a managed PPP server first")
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *Service) verifySharedPPP(ctx context.Context, c routers.API, u domain.VPNUser) error {
	ids, err := s.sharedPPPServers(ctx, c, u.RouterID)
	if err != nil {
		return err
	}
	expected := append([]string{}, u.SharedServers...)
	sort.Strings(expected)
	if len(ids) != len(expected) {
		return errors.New("shared PPP services changed; review the user access scope")
	}
	for i := range ids {
		if ids[i] != expected[i] {
			return errors.New("shared PPP services changed; review the user access scope")
		}
	}
	return nil
}

type UserBinding struct {
	domain.VPNUser
	Protocols []string
}
type UserIdentity struct {
	ID, Username, RouterID, AccountID string
	Bindings                          []UserBinding
	AvailableServers                  []domain.Server
}

func GroupUsers(users []domain.VPNUser, servers []domain.Server) []UserIdentity {
	groups := []UserIdentity{}
	indices := map[string]int{}
	for _, u := range users {
		id := userIdentity(u)
		i, ok := indices[id]
		if !ok {
			i = len(groups)
			indices[id] = i
			groups = append(groups, UserIdentity{ID: id, Username: u.Username, RouterID: u.RouterID, AccountID: u.AccountID})
		}
		p := u.Protocol
		if p == "" {
			p = "openvpn"
		}
		ps := []string{p}
		if len(u.SharedServers) > 0 {
			ps = []string{}
			for _, id := range u.SharedServers {
				for _, server := range servers {
					if server.ID == id {
						ps = append(ps, protocols.Name(server.Spec))
					}
				}
			}
		}
		groups[i].Bindings = append(groups[i].Bindings, UserBinding{VPNUser: u, Protocols: ps})
	}
	for i := range groups {
		for _, server := range servers {
			p := protocols.Name(server.Spec)
			if server.Spec.RouterID != groups[i].RouterID || p == "ikev2" {
				continue
			}
			exists := false
			for _, binding := range groups[i].Bindings {
				if p == "wireguard" && binding.ServerID == server.ID || p != "wireguard" && binding.Protocol != "wireguard" {
					exists = true
					break
				}
			}
			if !exists {
				groups[i].AvailableServers = append(groups[i].AvailableServers, server)
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Username < groups[j].Username })
	return groups
}
