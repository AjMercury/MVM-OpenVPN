package sqlite

import (
	"encoding/json"
	"errors"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/vpn/openvpn"
	"time"
	_ "time/tzdata"
)

// BootstrapAdmin is offline-only. The account guard and preferences commit together.
// Any account disables browser setup, including when an old token file remains.
func (s *Store) BootstrapAdmin(username, password, zone, language string) error {
	if !openvpn.Identifier.MatchString(username) {
		return errors.New("invalid administrator username")
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return errors.New("invalid timezone")
	}
	if language != "en" && language != "fa" {
		return errors.New("invalid language")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Acquire SQLite's write lock before reading to serialize concurrent bootstraps.
	if _, err = tx.Exec("INSERT INTO collections(name) VALUES('accounts') ON CONFLICT DO NOTHING"); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM records WHERE collection='accounts'").Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("administrator already initialized")
	}
	account := domain.Account{ID: "initial-super", Username: username, Hash: hash, Role: domain.Super}
	for _, record := range []struct {
		bucket, id string
		value      any
	}{
		{"accounts", account.ID, account},
		{"settings", "panel", map[string]string{"timezone": zone, "language": language}},
	} {
		raw, err := json.Marshal(record.value)
		if err != nil {
			return err
		}
		sealed, err := s.vault.Seal(raw, record.bucket+":"+record.id)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO collections(name) VALUES(?) ON CONFLICT DO NOTHING", record.bucket); err != nil {
			return err
		}
		var name any
		if record.bucket == "accounts" {
			name = security.Digest(username)
		}
		if _, err = tx.Exec("INSERT INTO records(collection,id,payload,unique_name) VALUES(?,?,?,?)", record.bucket, record.id, sealed, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}
