package sqlite

import (
	"bytes"
	"mikrotik-vpn-manager/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEncryptionBackupRestoreConstraints(t *testing.T) {
	key := bytes.Repeat([]byte{8}, 32)
	dir := t.TempDir()
	path := filepath.Join(dir, "mvm.db")
	s, e := Open(path, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Put("accounts", "a", domain.Account{ID: "a", Username: "admin", Hash: "sensitive-hash"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Put("accounts", "b", domain.Account{ID: "b", Username: "admin"}); e == nil {
		t.Fatal("duplicate accepted")
	}
	if e = s.Put("sessions", "x", domain.Session{AccountID: "missing"}); e == nil {
		t.Fatal("orphan accepted")
	}
	var snapshot bytes.Buffer
	if e = s.Backup(&snapshot); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(snapshot.Bytes(), []byte("sensitive-hash")) {
		t.Fatal("plaintext in snapshot")
	}
	backup := filepath.Join(dir, "backup.db")
	os.WriteFile(backup, snapshot.Bytes(), 0600)
	restored := filepath.Join(dir, "restored.db")
	if e = Restore(restored, backup, bytes.Repeat([]byte{9}, 32)); e == nil {
		t.Fatal("wrong key accepted")
	}
	if _, e = os.Stat(restored); !os.IsNotExist(e) {
		t.Fatal("failed restore published")
	}
	if e = Restore(restored, backup, key); e != nil {
		t.Fatal(e)
	}
	if e = Restore(restored, backup, key); e == nil {
		t.Fatal("restore overwrote")
	}
	s.Close()
	if _, e = Open(path, bytes.Repeat([]byte{9}, 32)); e == nil {
		t.Fatal("wrong key accepted")
	}
}
func TestRefuseUnknownAndLegacy(t *testing.T) {
	p := filepath.Join(t.TempDir(), "db")
	os.WriteFile(p, []byte("legacy bytes"), 0600)
	if _, e := Open(p, bytes.Repeat([]byte{1}, 32)); e == nil {
		t.Fatal("legacy accepted")
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != "legacy bytes" {
		t.Fatal("legacy modified")
	}
}
func TestCrashRecovery(t *testing.T) {
	if os.Getenv("MVM_SQLITE_CRASH_HELPER") == "1" {
		s, e := Open(os.Getenv("MVM_SQLITE_CRASH_DB"), bytes.Repeat([]byte{4}, 32))
		if e != nil {
			os.Exit(9)
		}
		if e = s.Put("settings", "committed", "durable"); e != nil {
			os.Exit(10)
		}
		tx, e := s.db.Begin()
		if e != nil {
			os.Exit(11)
		}
		if _, e = tx.Exec("DELETE FROM records WHERE collection='settings' AND id='committed'"); e != nil {
			os.Exit(12)
		}
		os.Exit(88) // Intentionally skip rollback/Close to simulate process death.
	}
	path := filepath.Join(t.TempDir(), "crash.db")
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(exe, "-test.run=^TestCrashRecovery$")
	cmd.Env = append(os.Environ(), "MVM_SQLITE_CRASH_HELPER=1", "MVM_SQLITE_CRASH_DB="+path)
	if e = cmd.Run(); e == nil {
		t.Fatal("helper did not crash")
	} else if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 88 {
		t.Fatal(e)
	}
	s, e := Open(path, bytes.Repeat([]byte{4}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var value string
	if e = s.Get("settings", "committed", &value); e != nil || value != "durable" {
		t.Fatal("lost committed value", e)
	}
	if e = s.Check(); e != nil {
		t.Fatal(e)
	}
}

func TestUnknownSchemaRejectedWithoutReset(t *testing.T) {
	p := filepath.Join(t.TempDir(), "schema.db")
	key := bytes.Repeat([]byte{3}, 32)
	s, e := Open(p, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("PRAGMA user_version=99"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	before, _ := os.ReadFile(p)
	if _, e = Open(p, key); e == nil {
		t.Fatal("future schema opened")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("future schema reset/modified")
	}
}
func TestUniqueIPAndIdentity(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "db"), bytes.Repeat([]byte{2}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Put("routers", "r", domain.Router{ID: "r"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Put("users", "u", domain.VPNUser{ID: "u", RouterID: "r", Username: "alice", Address: "10.0.0.2"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Put("users", "v", domain.VPNUser{ID: "v", RouterID: "r", Username: "bob", Address: "10.0.0.2"}); e == nil {
		t.Fatal("duplicate IP accepted")
	}
	if e = s.Put("users", "w", domain.VPNUser{ID: "w", RouterID: "r", Username: "alice", Address: "10.0.0.3"}); e == nil {
		t.Fatal("duplicate identity accepted")
	}
}
