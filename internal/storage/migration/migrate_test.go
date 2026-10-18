package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/storage"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyMigration(t *testing.T) {
	key := bytes.Repeat([]byte{2}, 32)
	dir := t.TempDir()
	src := filepath.Join(dir, "legacy.db")
	dst := filepath.Join(dir, "mvm.db")
	backup := filepath.Join(dir, "backup.db")
	s, e := storage.Open(src, key)
	if e != nil {
		t.Fatal(e)
	}
	s.Put("accounts", "stable", domain.Account{ID: "stable", Username: "admin", Hash: "secret"})
	s.Close()
	original, _ := os.ReadFile(src)
	if e = Migrate(src, dst, backup, bytes.Repeat([]byte{3}, 32)); e == nil {
		t.Fatal("wrong key accepted")
	}
	if _, e = os.Stat(dst); !os.IsNotExist(e) {
		t.Fatal("published on failure")
	}
	if e = Migrate(src, dst, backup, key); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(src)
	if !bytes.Equal(original, after) {
		t.Fatal("source modified")
	}
	db, e := sqlite.Open(dst, key)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var a domain.Account
	if e = db.Get("accounts", "stable", &a); e != nil || a.Hash != "secret" {
		t.Fatal(e)
	}
	if e = Migrate(src, dst, backup, key); e == nil {
		t.Fatal("unsafe repeat accepted")
	}
}
func TestInterruptionPreservesRecovery(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	dir := t.TempDir()
	src := filepath.Join(dir, "legacy.db")
	dst := filepath.Join(dir, "new.db")
	backup := filepath.Join(dir, "backup.db")
	s, e := storage.Open(src, key)
	if e != nil {
		t.Fatal(e)
	}
	s.Put("settings", "secret", "retain")
	s.Close()
	before, _ := os.ReadFile(src)
	e = migrate(src, dst, backup, key, func() error { return fmt.Errorf("simulated interruption before activation") })
	if e == nil {
		t.Fatal("fault ignored")
	}
	if _, e = os.Stat(dst); !os.IsNotExist(e) {
		t.Fatal("interrupted rehearsal activated destination")
	}
	after, _ := os.ReadFile(src)
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
	if e = storage.ReadLegacy(backup, key, func(string, string, []byte, json.RawMessage) error { return nil }); e != nil {
		t.Fatal("recovery backup invalid", e)
	}
}
func TestMigrationRefusesOrphan(t *testing.T) {
	key := bytes.Repeat([]byte{6}, 32)
	dir := t.TempDir()
	src := filepath.Join(dir, "legacy.db")
	dst := filepath.Join(dir, "new.db")
	backup := filepath.Join(dir, "backup.db")
	s, e := storage.Open(src, key)
	if e != nil {
		t.Fatal(e)
	}
	s.Put("sessions", "orphan", domain.Session{AccountID: "missing"})
	s.Close()
	if e = Migrate(src, dst, backup, key); e == nil {
		t.Fatal("orphan migrated")
	}
	if _, e = os.Stat(dst); !os.IsNotExist(e) {
		t.Fatal("invalid relationships activated")
	}
}

func TestRelatedRecordsPreserved(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	dir := t.TempDir()
	src := filepath.Join(dir, "old.db")
	dst := filepath.Join(dir, "new.db")
	s, e := storage.Open(src, key)
	if e != nil {
		t.Fatal(e)
	}
	fixtures := []struct {
		b, id string
		v     any
	}{
		{"routers", "r", domain.Router{ID: "r", Password: "router secret"}},
		{"accounts", "a", domain.Account{ID: "a", Username: "admin", Hash: "argon hash"}},
		{"jobs", "j", domain.Job{ID: "j", RouterID: "r", State: "MANUAL_INTERVENTION"}},
		{"servers", "s", domain.Server{ID: "s", Spec: domain.ServerSpec{RouterID: "r"}, JobID: "j"}},
		{"users", "u", domain.VPNUser{ID: "u", RouterID: "r", ServerID: "s", AccountID: "a"}},
		{"sessions", "session", domain.Session{AccountID: "a", CSRF: "secret csrf"}},
		{"deliveries", "d", domain.Delivery{ID: "d", AccountID: "a", UserID: "u", Profile: []byte("secret profile")}},
		{"future-cert-metadata", "cert", map[string]string{"id": "cert", "owner": "j"}},
	}
	for _, f := range fixtures {
		if e = s.Put(f.b, f.id, f.v); e != nil {
			t.Fatal(e)
		}
	}
	s.Close()
	if e = Migrate(src, dst, filepath.Join(dir, "backup.db"), key); e != nil {
		t.Fatal(e)
	}
	r, e := sqlite.Open(dst, key)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for _, f := range fixtures {
		var got any
		if e = r.Get(f.b, f.id, &got); e != nil {
			t.Fatal(f.b, e)
		}
		wantRaw, _ := json.Marshal(f.v)
		var want any
		json.Unmarshal(wantRaw, &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("changed record", f.b)
		}
	}
	if e = r.Check(); e != nil {
		t.Fatal(e)
	}
}
