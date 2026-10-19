package app

import (
	"context"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/jobs"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/vpn/network"
	"reflect"
	"time"
)

func (s *Service) NetworkOptions(ctx context.Context, a domain.Account, routerID string) (network.Options, error) {
	r, e := s.Router(a, routerID, "read")
	if e != nil {
		return network.Options{}, e
	}
	c, e := s.Connect(r)
	if e != nil {
		return network.Options{}, e
	}
	return s.networkOptions(ctx, c, r.ID)
}
func (s *Service) networkOptions(ctx context.Context, c routers.API, routerID string) (network.Options, error) {
	var servers []domain.Server
	if e := s.Repo.List("servers", &servers); e != nil {
		return network.Options{}, e
	}
	return network.Read(ctx, c, servers, routerID)
}
func (s *Service) PrepareNetwork(ctx context.Context, a domain.Account, spec domain.NetworkSpec, key string) (*domain.Job, error) {
	r, e := s.Router(a, spec.RouterID, "router")
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
	options, e := s.networkOptions(ctx, c, r.ID)
	if e != nil {
		return nil, e
	}
	spec, e = network.Resolve(options, spec)
	if e != nil {
		return nil, e
	}
	var existing []domain.Job
	if e = s.Repo.List("jobs", &existing); e != nil {
		return nil, e
	}
	for _, j := range existing {
		if j.RouterID == r.ID && j.Network != nil && j.Network.SourceID == spec.SourceID && j.Network.OutInterfaceID == spec.OutInterfaceID && j.State != "FAILED" && j.State != "EXPIRED" && j.State != "ROLLED_BACK" {
			if j.State == "READY" && !j.InFlight && len(j.Owned) == 0 && (time.Since(j.Created) > 15*time.Minute || !reflect.DeepEqual(*j.Network, spec)) {
				j.State = "EXPIRED"
				j.Error = "Superseded by a refreshed network preview"
				if e = s.Engine.Save(&j); e != nil {
					return nil, e
				}
				continue
			}
			return nil, errors.New("a network job already exists for this source and output interface; review it before creating another")
		}
	}
	j := network.Plan(ID(), spec)
	j.Actor = a.ID
	j.Key = key
	j.Fingerprint = security.Digest(jobs.Fingerprint(spec))
	if e = s.Engine.Prepare(j); e != nil {
		return nil, e
	}
	return j, s.Audit(a.ID, "network.preview", j.ID, "ready")
}
func (s *Service) networkPreflight(ctx context.Context, c routers.API, spec domain.NetworkSpec) error {
	options, e := s.networkOptions(ctx, c, spec.RouterID)
	if e != nil {
		return e
	}
	current, e := network.Resolve(options, spec)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(current, spec) {
		return errors.New("selected interface or source range changed after preview; prepare a new job")
	}
	return nil
}
