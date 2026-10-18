package sqlite

import (
	"bytes"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"path/filepath"
	"testing"
)

func TestBootstrapAdmin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.db")
	key := bytes.Repeat([]byte{7}, 32)
	s, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, password := range []string{"short", string(bytes.Repeat([]byte{'a'}, 234))} {
		if s.BootstrapAdmin("admin", password, "UTC", "en") == nil {
			t.Fatal("weak/oversized password accepted")
		}
	}
	if s.BootstrapAdmin("bad user", "long unique passphrase", "UTC", "en") == nil {
		t.Fatal("invalid username accepted")
	}
	if s.BootstrapAdmin("admin", "long unique passphrase", "bad/zone", "en") == nil {
		t.Fatal("invalid timezone accepted")
	}
	if err = s.BootstrapAdmin("owner", "long unique passphrase", "Asia/Tehran", "fa"); err != nil {
		t.Fatal(err)
	}
	if s.BootstrapAdmin("attacker", "another long passphrase", "UTC", "en") == nil {
		t.Fatal("existing administrator overwritten")
	}
	var accounts []domain.Account
	if err = s.List("accounts", &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Role != domain.Super || !security.CheckPassword(accounts[0].Hash, "long unique passphrase") {
		t.Fatal("invalid administrator")
	}
	var prefs map[string]string
	if err = s.Get("settings", "panel", &prefs); err != nil || prefs["language"] != "fa" {
		t.Fatal("preferences missing", err)
	}
	s.Close()
	s, err = Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if s.BootstrapAdmin("retry", "another long passphrase", "UTC", "en") == nil {
		t.Fatal("retry overwrote administrator")
	}
	s.Close()
}
