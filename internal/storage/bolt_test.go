package storage

import (
	"bytes"
	"mikrotik-vpn-manager/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistenceAndBackup(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	path := filepath.Join(t.TempDir(), "test.db")
	s, e := Open(path, key)
	if e != nil {
		t.Fatal(e)
	}
	a := domain.Account{ID: "one", Username: "admin", Hash: "secret-hash"}
	if e = s.Put("accounts", a.ID, a); e != nil {
		t.Fatal(e)
	}
	var backup bytes.Buffer
	if e = s.Backup(&backup); e != nil {
		t.Fatal(e)
	}
	if e = s.Check(); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("secret-hash")) {
		t.Fatal("database plaintext")
	}
	s, e = Open(path, key)
	if e != nil {
		t.Fatal(e)
	}
	var got domain.Account
	if e = s.Get("accounts", "one", &got); e != nil || got.Hash != a.Hash {
		t.Fatalf("lost credential: %v", e)
	}
	s.Close()
	if _, e = Open(path, bytes.Repeat([]byte{2}, 32)); e == nil {
		t.Fatal("wrong master key accepted")
	}
	restore := filepath.Join(t.TempDir(), "restore.db")
	if e = os.WriteFile(restore, backup.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	s, e = Open(restore, key)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Check(); e != nil {
		t.Fatal(e)
	}
	if e = s.Get("accounts", "one", &got); e != nil {
		t.Fatal(e)
	}
}
