package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mikrotik-vpn-manager/internal/config"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/ipam"
	"mikrotik-vpn-manager/internal/jobs"
	"mikrotik-vpn-manager/internal/provisioning"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage"
	"mikrotik-vpn-manager/internal/vpn/openvpn"
	"net/netip"
	"strings"
	"time"
)

type Service struct {
	Repo    storage.Repository
	Engine  *jobs.Engine
	Config  config.Config
	Connect func(domain.Router) (routers.API, error)
}

func New(repo storage.Repository, c config.Config) *Service {
	s := &Service{Repo: repo, Engine: &jobs.Engine{Repo: repo}, Config: c}
	s.Connect = func(r domain.Router) (routers.API, error) { return routers.New(r, c.Allowed, c.Timeout) }
	return s
}
func ID() string { return security.Digest(security.Token())[:24] }
func (s *Service) Audit(actor, action, target, result string) error {
	a := domain.Audit{ID: ID(), At: time.Now().UTC(), Actor: actor, Action: action, Target: target, Result: result}
	return s.Repo.Put("audit", a.ID, a)
}
func (s *Service) Router(a domain.Account, id, action string) (domain.Router, error) {
	var r domain.Router
	if !a.Can(action, id) {
		return r, errors.New("permission denied")
	}
	e := s.Repo.Get("routers", id, &r)
	return r, e
}
func (s *Service) AddRouter(a domain.Account, r domain.Router) (domain.Router, error) {
	if a.Role != domain.Super {
		return r, errors.New("only super administrators can register routers")
	}
	if !routers.ValidHost(r.Host) || len(r.Name) < 1 || len(r.Name) > 80 || r.Port < 1 || r.Port > 65535 || r.Username == "" || r.Password == "" {
		return r, errors.New("invalid router inventory fields")
	}
	if len(r.CA) > 64*1024 || len(r.Password) > 1024 {
		return r, errors.New("router credential or CA too large")
	}
	if _, e := routers.New(r, s.Config.Allowed, s.Config.Timeout); e != nil {
		return r, e
	}
	r.ID = ID()
	r.Status = "unknown"
	if e := s.Audit(a.ID, "router.register", r.ID, "requested"); e != nil {
		return r, e
	}
	return r, s.Repo.Put("routers", r.ID, r)
}
func (s *Service) Sync(ctx context.Context, a domain.Account, id string) (domain.Router, error) {
	r, e := s.Router(a, id, "router")
	if e != nil {
		return r, e
	}
	unlock := s.Engine.Lock(id)
	defer unlock()
	c, e := s.Connect(r)
	if e != nil {
		return r, e
	}
	snapshot, e := routers.Discover(ctx, c)
	if e != nil {
		r.Status = "offline"
		_ = s.Repo.Put("routers", id, r)
		return r, e
	}
	r.Status = "online"
	if len(snapshot.Warnings) > 0 {
		r.Status = "degraded"
	}
	r.PreviousHash = r.Observed.ConfigHash
	r.Drift = r.PreviousHash != "" && r.PreviousHash != snapshot.ConfigHash
	r.Observed = snapshot
	r.LastSuccess = time.Now().UTC()
	if e = s.Repo.Put("routers", id, r); e != nil {
		return r, e
	}
	e = s.Audit(a.ID, "router.sync", id, "observed")
	return r, e
}
func (s *Service) PrepareServer(ctx context.Context, a domain.Account, spec domain.ServerSpec, key string) (*domain.Job, error) {
	r, e := s.Router(a, spec.RouterID, "vpn")
	if e != nil {
		return nil, e
	}
	unlock := s.Engine.Lock(r.ID)
	defer unlock()
	if len(key) < 8 || len(key) > 128 {
		return nil, errors.New("idempotency key must contain 8–128 characters")
	}
	c, e := s.Connect(r)
	if e != nil {
		return nil, e
	}
	if e = openvpn.Preflight(ctx, c, spec); e != nil {
		return nil, e
	}
	var servers []domain.Server
	if e = s.Repo.List("servers", &servers); e != nil {
		return nil, e
	}
	for _, v := range servers {
		if v.Spec.RouterID == r.ID {
			return nil, errors.New("one managed OpenVPN server per router")
		}
	}
	j := openvpn.Plan(ID(), spec)
	j.Actor = a.ID
	j.Key = key
	j.Fingerprint = security.Digest(jobs.Fingerprint(spec))
	if e = s.Engine.Prepare(j); e != nil {
		return nil, e
	}
	if e = s.Audit(a.ID, "server.preview", j.ID, "ready"); e != nil {
		return nil, e
	}
	return j, nil
}

type UserRequest struct {
	ServerID  string `json:"server_id"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Address   string `json:"address"`
	AccountID string `json:"account_id"`
	Key       string `json:"idempotency_key"`
}

func (s *Service) PrepareUser(ctx context.Context, a domain.Account, in UserRequest) (*domain.Job, error) {
	var server domain.Server
	if e := s.Repo.Get("servers", in.ServerID, &server); e != nil {
		return nil, e
	}
	r, e := s.Router(a, server.Spec.RouterID, "vpn")
	if e != nil {
		return nil, e
	}
	unlock := s.Engine.Lock(r.ID)
	defer unlock()
	if !openvpn.Identifier.MatchString(in.Username) || len(in.Password) < 14 || len(in.Password) > 233 || len(in.Key) < 8 || len(in.Key) > 128 {
		return nil, errors.New("invalid username, password length, or idempotency key")
	}
	if in.AccountID != "" {
		var owner domain.Account
		if e := s.Repo.Get("accounts", in.AccountID, &owner); e != nil || owner.Role != domain.EndUser {
			return nil, errors.New("profile owner must be an existing end-user account")
		}
	}
	c, e := s.Connect(r)
	if e != nil {
		return nil, e
	}
	if e = s.verifyServer(ctx, c, server); e != nil {
		return nil, e
	}
	secrets, e := routers.List(ctx, c, "ppp/secret")
	if e != nil {
		return nil, e
	}
	for _, u := range secrets {
		if u["name"] == in.Username {
			return nil, errors.New("username already exists in PPP scope")
		}
	}
	var users []domain.VPNUser
	if e = s.Repo.List("users", &users); e != nil {
		return nil, e
	}
	var allJobs []domain.Job
	if e = s.Repo.List("jobs", &allJobs); e != nil {
		return nil, e
	}
	used := map[string]bool{}
	for _, u := range users {
		if u.RouterID == r.ID {
			used[u.Address] = true
			if u.Username == in.Username {
				return nil, errors.New("username is already managed")
			}
		}
	}
	for _, j := range allJobs {
		if j.RouterID == r.ID && j.User != nil && j.State != "ROLLED_BACK" && j.State != "EXPIRED" {
			used[j.User.Address] = true
			if j.User.Username == in.Username {
				return nil, errors.New("username reserved by another job")
			}
		}
	}
	for _, u := range secrets {
		used[u["remote-address"]] = true
	}
	// Static reservations are used for both automatic and manual modes, making ownership deterministic.
	if in.Address == "" {
		start, _ := netip.ParseAddr(server.Spec.Start)
		end, _ := netip.ParseAddr(server.Spec.End)
		for p := start; p.IsValid() && p.Compare(end) <= 0; p = p.Next() {
			if !used[p.String()] {
				in.Address = p.String()
				break
			}
		}
	}
	if !ipam.Contains(server.Spec, in.Address) || used[in.Address] {
		return nil, errors.New("address unavailable or outside server pool")
	}
	u := domain.VPNUser{RouterID: r.ID, ServerID: server.ID, AccountID: in.AccountID, Username: in.Username, Address: in.Address}
	j := openvpn.UserPlan(ID(), server, u, in.Password)
	j.Actor = a.ID
	j.Key = in.Key
	j.Fingerprint = security.Digest(jobs.Fingerprint(in))
	if e = s.Engine.Prepare(j); e != nil {
		return nil, e
	}
	return j, s.Audit(a.ID, "user.preview", j.ID, "ready")
}
func (s *Service) Apply(ctx context.Context, a domain.Account, id string, confirmed bool) error {
	var j domain.Job
	if e := s.Repo.Get("jobs", id, &j); e != nil {
		return e
	}
	r, e := s.Router(a, j.RouterID, "vpn")
	if e != nil {
		return e
	}
	if !s.Config.Mutations || !confirmed {
		return errors.New("router mutations require MVM_ALLOW_MUTATIONS=true and explicit reviewed confirmation")
	}
	unlock := s.Engine.Lock(r.ID)
	defer unlock()
	if e = s.Repo.Get("jobs", id, &j); e != nil {
		return e
	}
	if j.State == "COMPLETED" {
		return nil
	}
	c, e := s.Connect(r)
	if e != nil {
		return e
	}
	if j.Server != nil {
		if e = openvpn.Preflight(ctx, c, j.Server.Spec); e != nil {
			return e
		}
	}
	if j.User != nil {
		var server domain.Server
		if e = s.Repo.Get("servers", j.User.ServerID, &server); e != nil {
			return e
		}
		if e = s.verifyServer(ctx, c, server); e != nil {
			return e
		}
		rows, e := routers.List(ctx, c, "ppp/secret")
		if e != nil {
			return e
		}
		for _, u := range rows {
			if u["name"] == j.User.Username || u["remote-address"] == j.User.Address {
				return errors.New("PPP identity or address changed since preview")
			}
		}
	}
	if e = s.Audit(a.ID, "job.apply", id, "approved"); e != nil {
		return e
	}
	e = s.Engine.Apply(ctx, c, &j)
	result := j.State
	if auditErr := s.Audit(a.ID, "job.result", id, result); auditErr != nil {
		return auditErr
	}
	return e
}

func (s *Service) verifyServer(ctx context.Context, c routers.API, server domain.Server) error {
	rows, e := routers.List(ctx, c, "interface/ovpn-server/server")
	if e != nil {
		return e
	}
	if len(rows) != 1 || rows[0][".id"] != server.RemoteID {
		return errors.New("OpenVPN server scope changed; manual review required")
	}
	secrets, e := routers.List(ctx, c, "ppp/secret")
	if e != nil {
		return e
	}
	var managed []domain.VPNUser
	if e = s.Repo.List("users", &managed); e != nil {
		return e
	}
	for _, secret := range secrets {
		if secret["disabled"] == "yes" || secret["disabled"] == "true" || secret["service"] != "ovpn" && secret["service"] != "any" {
			continue
		}
		known := false
		for _, user := range managed {
			if user.RouterID == server.Spec.RouterID && user.RemoteID == secret[".id"] && secret["comment"] == "mvm:"+user.JobID {
				known = true
				break
			}
		}
		if !known {
			return errors.New("unmanaged PPP account could authenticate to this server; review shared authentication scope")
		}
	}
	return s.Engine.VerifyOwned(ctx, c, server.JobID)
}
func (s *Service) UserAction(ctx context.Context, a domain.Account, id, action string) error {
	var u domain.VPNUser
	if e := s.Repo.Get("users", id, &u); e != nil {
		return e
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
		return errors.New("router has unresolved provisioning state")
	}
	c, e := s.Connect(r)
	if e != nil {
		return e
	}
	if action != "disable" && action != "disconnect" {
		return errors.New("unsupported user operation")
	}
	if e = s.Audit(a.ID, "user."+action, id, "approved"); e != nil {
		return e
	}
	if action == "disable" {
		row, e := routers.Find(ctx, c, "ppp/secret", "name", u.Username)
		if e != nil {
			return e
		}
		if row[".id"] != u.RemoteID || row["comment"] != "mvm:"+u.JobID {
			return errors.New("PPP ownership changed")
		}
		if _, e = c.Call(ctx, "PATCH", "ppp/secret/"+u.RemoteID, routers.Record{"disabled": "yes"}); e != nil {
			return e
		}
		row, e = routers.Find(ctx, c, "ppp/secret", "name", u.Username)
		if e != nil || row["disabled"] != "true" && row["disabled"] != "yes" {
			return errors.New("disable read-back failed")
		}
		u.Disabled = true
		if e = s.Repo.Put("users", id, u); e != nil {
			return e
		}
	}
	rows, e := routers.List(ctx, c, "ppp/active")
	if e != nil {
		return e
	}
	for _, row := range rows {
		if row["name"] == u.Username && row["service"] == "ovpn" {
			if _, e = c.Call(ctx, "DELETE", "ppp/active/"+row[".id"], nil); e != nil {
				return e
			}
		}
	}
	rows, e = routers.List(ctx, c, "ppp/active")
	if e != nil {
		return e
	}
	for _, row := range rows {
		if row["name"] == u.Username && row["service"] == "ovpn" {
			return errors.New("active session remains; review RouterOS")
		}
	}
	return s.Audit(a.ID, "user."+action, id, "verified")
}
func (s *Service) CanProfile(a domain.Account, u domain.VPNUser) bool {
	if a.Role == domain.EndUser {
		return u.AccountID == a.ID
	}
	return a.Can("vpn", u.RouterID)
}
func (s *Service) Export(ctx context.Context, a domain.Account, userID, passphrase string) ([]byte, error) {
	var u domain.VPNUser
	if e := s.Repo.Get("users", userID, &u); e != nil {
		return nil, e
	}
	if !s.CanProfile(a, u) || u.Disabled {
		return nil, errors.New("profile access denied")
	}
	if a.Role == domain.EndUser {
		return nil, errors.New("ask an authorized VPN operator to regenerate encrypted material")
	}
	if len(passphrase) < 14 || len(passphrase) > 128 {
		return nil, errors.New("export passphrase must contain 14–128 bytes")
	}
	if !s.Config.Mutations {
		return nil, errors.New("certificate export writes temporary router files; mutations disabled")
	}
	var server domain.Server
	if e := s.Repo.Get("servers", u.ServerID, &server); e != nil {
		return nil, e
	}
	var r domain.Router
	if e := s.Repo.Get("routers", u.RouterID, &r); e != nil {
		return nil, e
	}
	unlock := s.Engine.Lock(r.ID)
	defer unlock()
	if s.Engine.Blocked(r.ID, "") {
		return nil, errors.New("router has unresolved operation")
	}
	c, e := s.Connect(r)
	if e != nil {
		return nil, e
	}
	if e = s.Audit(a.ID, "profile.export", u.ID, "approved"); e != nil {
		return nil, e
	}
	prefix := "mvm-export-" + ID()
	files := []string{prefix + "-ca.crt", prefix + "-client.crt", prefix + "-client.key"}
	exportJob := &domain.Job{ID: ID(), RouterID: r.ID, Actor: a.ID, Kind: "export", State: "APPLYING", Created: time.Now().UTC(), InFlight: true, Steps: []domain.Step{{Path: "certificate/export-certificate", Method: "POST", Values: map[string]string{"temporary-prefix": prefix, "certificate": u.Certificate}}}}
	if e := s.Engine.Save(exportJob); e != nil {
		return nil, e
	}
	// Names are cryptographically unique. Cleanup is attempted even on a failed export.
	cleanup := func() error {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var errs []error
		for _, name := range files {
			rows, e := routers.List(cleanCtx, c, "file")
			if e != nil {
				errs = append(errs, e)
				continue
			}
			for _, f := range rows {
				if f["name"] == name {
					_, e = c.Call(cleanCtx, "DELETE", "file/"+f[".id"], nil)
					if e != nil {
						errs = append(errs, e)
					}
				}
			}
		}
		return errors.Join(errs...)
	}
	export := func() ([]byte, error) {
		if _, e := c.Call(ctx, "POST", "certificate/export-certificate", routers.Record{"numbers": server.CA, "type": "pem", "file-name": prefix + "-ca"}); e != nil {
			return nil, e
		}
		if _, e := c.Call(ctx, "POST", "certificate/export-certificate", routers.Record{"numbers": u.Certificate, "type": "pem", "file-name": prefix + "-client", "export-passphrase": passphrase}); e != nil {
			return nil, e
		}
		ca, e := routers.ReadFile(ctx, c, files[0])
		if e != nil {
			return nil, e
		}
		cert, e := routers.ReadFile(ctx, c, files[1])
		if e != nil {
			return nil, e
		}
		key, e := routers.ReadFile(ctx, c, files[2])
		if e != nil {
			return nil, e
		}
		return provisioning.Generate(server, provisioning.Material{CA: ca, Certificate: cert, Key: key, Passphrase: passphrase})
	}
	data, e := export()
	if cleanErr := cleanup(); cleanErr != nil {
		exportJob.State = "MANUAL_INTERVENTION"
		exportJob.Error = "Review temporary export files with prefix " + prefix
		_ = s.Engine.Save(exportJob)
		_ = s.Audit(a.ID, "profile.cleanup", u.ID, "manual_intervention")
		return nil, errors.New("temporary file cleanup failed; remove only the export files listed by prefix " + prefix)
	}
	if e != nil {
		exportJob.State = "FAILED"
		exportJob.InFlight = false
		exportJob.Error = "Export failed; temporary files cleaned"
		_ = s.Engine.Save(exportJob)
		return nil, e
	}
	exportJob.State = "COMPLETED"
	exportJob.InFlight = false
	if e := s.Engine.Save(exportJob); e != nil {
		return nil, e
	}
	return data, nil
}
func (s *Service) IssueDelivery(a domain.Account, userID string, profile []byte) (string, error) {
	var u domain.VPNUser
	if e := s.Repo.Get("users", userID, &u); e != nil {
		return "", e
	}
	if !s.CanProfile(a, u) {
		return "", errors.New("permission denied")
	}
	token := security.Token()
	d := domain.Delivery{ID: security.Digest(token), UserID: userID, AccountID: u.AccountID, Expires: time.Now().Add(5 * time.Minute), Profile: profile}
	return token, s.Repo.Put("deliveries", d.ID, d)
}
func (s *Service) Consume(a domain.Account, token string) ([]byte, error) {
	var initial domain.Delivery
	if e := s.Repo.Get("deliveries", security.Digest(token), &initial); e != nil {
		return nil, errors.New("download unavailable")
	}
	var u domain.VPNUser
	if e := s.Repo.Get("users", initial.UserID, &u); e != nil {
		return nil, e
	}
	unlock := s.Engine.Lock(u.RouterID)
	defer unlock()
	if e := s.Repo.Get("users", initial.UserID, &u); e != nil {
		return nil, e
	}
	var profile []byte
	e := s.Repo.Update("deliveries", security.Digest(token), func(raw []byte) ([]byte, error) {
		var d domain.Delivery
		if len(raw) == 0 || json.Unmarshal(raw, &d) != nil {
			return nil, errors.New("download unavailable")
		}
		if !s.CanProfile(a, u) || u.Disabled || d.Consumed || time.Now().After(d.Expires) {
			return nil, errors.New("download expired, consumed, revoked or unauthorized")
		}
		profile = d.Profile
		d.Profile = nil
		d.Consumed = true
		return json.Marshal(d)
	})
	return profile, e
}

func (s *Service) RevokeDelivery(a domain.Account, token string) error {
	var initial domain.Delivery
	if e := s.Repo.Get("deliveries", security.Digest(token), &initial); e != nil {
		return errors.New("delivery unavailable")
	}
	var user domain.VPNUser
	if e := s.Repo.Get("users", initial.UserID, &user); e != nil {
		return e
	}
	if !s.CanProfile(a, user) {
		return errors.New("permission denied")
	}
	return s.Repo.Update("deliveries", security.Digest(token), func(raw []byte) ([]byte, error) {
		var d domain.Delivery
		if len(raw) == 0 || json.Unmarshal(raw, &d) != nil || d.UserID != user.ID {
			return nil, errors.New("delivery unavailable")
		}
		d.Consumed = true
		d.Profile = nil
		return json.Marshal(d)
	})
}
func (s *Service) Bootstrap(username, password string) error {
	unlock := s.Engine.Lock("accounts")
	defer unlock()
	var accounts []domain.Account
	if e := s.Repo.List("accounts", &accounts); e != nil {
		return e
	}
	if len(accounts) > 0 {
		return errors.New("administrator already initialized")
	}
	if !openvpn.Identifier.MatchString(username) {
		return errors.New("invalid administrator username")
	}
	hash, e := security.HashPassword(password)
	if e != nil {
		return e
	}
	a := domain.Account{ID: ID(), Username: username, Hash: hash, Role: domain.Super}
	return s.Repo.Put("accounts", a.ID, a)
}
func (s *Service) AddAccount(a domain.Account, username, password string, role domain.Role, scopes []string) error {
	unlock := s.Engine.Lock("accounts")
	defer unlock()
	if a.Role != domain.Super {
		return errors.New("permission denied")
	}
	switch role {
	case domain.Super, domain.RouterAdmin, domain.Operator, domain.Auditor, domain.EndUser:
	default:
		return errors.New("invalid role")
	}
	var accounts []domain.Account
	if e := s.Repo.List("accounts", &accounts); e != nil {
		return e
	}
	if !openvpn.Identifier.MatchString(username) {
		return errors.New("invalid account username")
	}
	for _, v := range accounts {
		if strings.EqualFold(v.Username, username) {
			return errors.New("duplicate account")
		}
	}
	for _, id := range scopes {
		var r domain.Router
		if e := s.Repo.Get("routers", id, &r); e != nil {
			return errors.New("invalid router scope")
		}
	}
	hash, e := security.HashPassword(password)
	if e != nil {
		return e
	}
	account := domain.Account{ID: ID(), Username: username, Hash: hash, Role: role, Routers: scopes}
	if e = s.Audit(a.ID, "account.create", account.ID, fmt.Sprint(role)); e != nil {
		return e
	}
	return s.Repo.Put("accounts", account.ID, account)
}
