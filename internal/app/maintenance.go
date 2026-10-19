package app

import (
	"context"
	"errors"
	"log/slog"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/storage"
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
			if err := s.PurgeExpired(); err != nil {
				slog.Error("maintenance failed", "error", err)
			}
		}
	}
}
func (s *Service) PurgeExpired() error {
	now := time.Now()
	if repo, ok := s.Repo.(interface {
		RecordIDs(string) ([]string, error)
	}); ok {
		ids, err := repo.RecordIDs("sessions")
		if err != nil {
			return err
		}
		for _, id := range ids {
			var session domain.Session
			if err := s.Repo.Get("sessions", id, &session); err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					continue
				}
				return err
			}
			if now.After(session.Expires) {
				if err := s.Repo.Delete("sessions", id); err != nil {
					return err
				}
			}
		}
	}
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
		legacyExpired := j.State == "FAILED" && j.Error == "Preview expired without application" && !j.InFlight && len(j.Owned) == 0
		hasTerminalSecret := false
		if j.State == "COMPLETED" || j.State == "ROLLED_BACK" || j.State == "FAILED" || j.State == "EXPIRED" || j.State == "MANUAL_INTERVENTION" {
			for _, step := range j.Steps {
				if _, ok := step.Values["password"]; ok {
					hasTerminalSecret = true
				}
				if _, ok := step.Values["ipsec-secret"]; ok {
					hasTerminalSecret = true
				}
			}
		}
		if !legacyExpired && !hasTerminalSecret && (j.State != "READY" || now.Sub(j.Created) < 15*time.Minute) {
			continue
		}
		unlock := s.Engine.Lock(j.RouterID)
		var fresh domain.Job
		if e := s.Repo.Get("jobs", j.ID, &fresh); e != nil {
			unlock()
			return e
		}
		if fresh.State == "READY" && now.Sub(fresh.Created) >= 15*time.Minute || fresh.State == "FAILED" && fresh.Error == "Preview expired without application" && !fresh.InFlight && len(fresh.Owned) == 0 {
			fresh.State = "EXPIRED"
			fresh.Error = "Preview expired without application"
			for i := range fresh.Steps {
				delete(fresh.Steps[i].Values, "password")
			}
		}
		if e := s.Engine.Save(&fresh); e != nil {
			unlock()
			return e
		}
		unlock()
	}
	return nil
}
