package app

import (
	"mikrotik-vpn-manager/internal/domain"
	"testing"
	"time"
)

func TestMaintenanceExpiresPreviewsAndLegacySessions(t *testing.T) {
	s := testService(t)
	for id, expires := range map[string]time.Time{"old": time.Now().Add(-time.Hour), "active": time.Now().Add(time.Hour)} {
		if err := s.Repo.Put("sessions", id, domain.Session{Expires: expires}); err != nil {
			t.Fatal(err)
		}
	}
	for _, j := range []domain.Job{
		{ID: "preview", RouterID: "r", State: "READY", Created: time.Now().Add(-time.Hour)},
		{ID: "legacy", RouterID: "r", State: "FAILED", Error: "Preview expired without application"},
		{ID: "uncertain", RouterID: "r", State: "MANUAL_INTERVENTION", InFlight: true},
	} {
		j.Steps = []domain.Step{{Values: map[string]string{"password": "must-be-erased"}}}
		if err := s.Repo.Put("jobs", j.ID, j); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PurgeExpired(); err != nil {
		t.Fatal(err)
	}
	var session domain.Session
	if err := s.Repo.Get("sessions", "old", &session); err == nil {
		t.Fatal("expired legacy session retained")
	}
	if err := s.Repo.Get("sessions", "active", &session); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"preview", "legacy", "uncertain"} {
		var j domain.Job
		if err := s.Repo.Get("jobs", id, &j); err != nil {
			t.Fatal(err)
		}
		if _, ok := j.Steps[0].Values["password"]; ok {
			t.Fatal("secret retained", id)
		}
		if id != "uncertain" && j.State != "EXPIRED" {
			t.Fatal("reservation not expired", id, j.State)
		}
		if id == "uncertain" && j.State != "MANUAL_INTERVENTION" {
			t.Fatal("uncertain reservation released")
		}
	}
}

func TestMaintenanceScrubsTerminalIPsecSecret(t *testing.T) {
	s := testService(t)
	j := domain.Job{ID: "old-ipsec", RouterID: "r", State: "COMPLETED", Steps: []domain.Step{{Values: map[string]string{"ipsec-secret": "legacy-secret"}}}}
	if err := s.Repo.Put("jobs", j.ID, j); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeExpired(); err != nil {
		t.Fatal(err)
	}
	j = domain.Job{ID: j.ID}
	if err := s.Repo.Get("jobs", j.ID, &j); err != nil {
		t.Fatal(err)
	}
	if _, ok := j.Steps[0].Values["ipsec-secret"]; ok {
		t.Fatal("terminal IPsec secret retained")
	}
}
