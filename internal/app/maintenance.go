package app

import (
	"context"
	"mikrotik-vpn-manager/internal/domain"
	"time"
)

// Maintenance purges short-lived credentials locally; it never contacts a router.
func (s *Service) Maintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.PurgeExpired()
		}
	}
}
func (s *Service) PurgeExpired() error {
	now := time.Now()
	var deliveries []domain.Delivery
	if e := s.Repo.List("deliveries", &deliveries); e != nil {
		return e
	}
	for _, d := range deliveries {
		if now.After(d.Expires) || d.Consumed {
			if e := s.Repo.Delete("deliveries", d.ID); e != nil {
				return e
			}
		}
	}
	var jobs []domain.Job
	if e := s.Repo.List("jobs", &jobs); e != nil {
		return e
	}
	for _, j := range jobs {
		if j.State != "READY" || now.Sub(j.Created) < 15*time.Minute {
			continue
		}
		unlock := s.Engine.Lock(j.RouterID)
		var fresh domain.Job
		if e := s.Repo.Get("jobs", j.ID, &fresh); e != nil {
			unlock()
			return e
		}
		if fresh.State == "READY" {
			fresh.State = "FAILED"
			fresh.Error = "Preview expired without application"
			for i := range fresh.Steps {
				delete(fresh.Steps[i].Values, "password")
			}
			if e := s.Engine.Save(&fresh); e != nil {
				unlock()
				return e
			}
		}
		unlock()
	}
	return nil
}
