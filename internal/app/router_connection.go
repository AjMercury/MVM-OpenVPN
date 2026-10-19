package app

import (
	"context"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"time"
)

// ProbeRouter observes an unsaved router without writing to RouterOS or storage.
func (s *Service) ProbeRouter(ctx context.Context, a domain.Account, r domain.Router) (domain.Router, error) {
	if a.Role != domain.Super {
		return r, errors.New("only super administrators can test new routers")
	}
	if !routers.ValidHost(r.Host) || r.Port < 1 || r.Port > 65535 || r.Username == "" || r.Password == "" || len(r.Password) > 1024 || len(r.CA) > 64*1024 {
		return r, errors.New("invalid router connection fields")
	}
	if _, err := routers.New(r, s.Config.Allowed, s.Config.Timeout); err != nil {
		return r, err
	}
	r.LastAttempt = time.Now().UTC()
	r.Status = "offline"
	c, err := s.Connect(r)
	if err == nil {
		r.Observed, err = routers.Discover(ctx, c)
	}
	if err != nil {
		r.LastError = err.Error()
		return r, err
	}
	r.LastSuccess = r.LastAttempt
	r.Status = "online"
	if len(r.Observed.Warnings) > 0 {
		r.Status = "degraded"
	}
	return r, nil
}
