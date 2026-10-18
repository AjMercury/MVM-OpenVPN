package main

import (
	"bytes"
	"encoding/base64"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineCLIBackupRestoreReset(t *testing.T) {
	dir := t.TempDir()
	key := bytes.Repeat([]byte{9}, 32)
	keyfile := filepath.Join(dir, "master.key")
	os.WriteFile(keyfile, []byte(base64.StdEncoding.EncodeToString(key)), 0600)
	t.Setenv("MVM_MASTER_KEY_FILE", keyfile)
	originalArgs, originalInput := os.Args, os.Stdin
	defer func() { os.Args = originalArgs; os.Stdin = originalInput }()
	command := func(args ...string) error { os.Args = append([]string{"mvmctl"}, args...); return run() }
	db := filepath.Join(dir, "mvm.db")
	if e := command("init", db); e != nil {
		t.Fatal(e)
	}
	repo, e := sqlite.Open(db, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = repo.Put("accounts", "a", domain.Account{ID: "a", Username: "admin", Role: domain.Super, Hash: "old"}); e != nil {
		t.Fatal(e)
	}
	if e = repo.Put("sessions", "s", domain.Session{AccountID: "a"}); e != nil {
		t.Fatal(e)
	}
	repo.Close()
	backup := filepath.Join(dir, "backup.db")
	if e = command("backup", db, backup); e != nil {
		t.Fatal(e)
	}
	if e = command("backup", db, backup); e == nil {
		t.Fatal("backup overwritten")
	}
	restored := filepath.Join(dir, "restored.db")
	if e = command("restore", restored, backup); e != nil {
		t.Fatal(e)
	}
	password := filepath.Join(dir, "stdin")
	os.WriteFile(password, []byte("a secure reset password"), 0600)
	input, e := os.Open(password)
	if e != nil {
		t.Fatal(e)
	}
	defer input.Close()
	os.Stdin = input
	if e = command("reset-admin", db, "admin"); e != nil {
		t.Fatal(e)
	}
	repo, e = sqlite.Open(db, key)
	if e != nil {
		t.Fatal(e)
	}
	defer repo.Close()
	var account domain.Account
	if e = repo.Get("accounts", "a", &account); e != nil || !security.CheckPassword(account.Hash, "a secure reset password") {
		t.Fatal("reset failed", e)
	}
	var sessions []domain.Session
	if e = repo.List("sessions", &sessions); e != nil || len(sessions) != 0 {
		t.Fatal("sessions not invalidated", e)
	}
}

func TestOfflineBootstrap(t *testing.T) {
	dir := t.TempDir()
	key := bytes.Repeat([]byte{4}, 32)
	keyfile := filepath.Join(dir, "key")
	if err := os.WriteFile(keyfile, []byte(base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MVM_MASTER_KEY_FILE", keyfile)
	args, stdin := os.Args, os.Stdin
	defer func() { os.Args = args; os.Stdin = stdin }()
	db := filepath.Join(dir, "panel.db")
	os.Args = []string{"mvmctl", "init", db}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	for index, password := range []string{"short", "secure unique passphrase", "replacement passphrase"} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(password)); err != nil {
			t.Fatal(err)
		}
		w.Close()
		os.Stdin = r
		os.Args = []string{"mvmctl", "bootstrap-admin", db, "owner", "UTC", "en"}
		err = run()
		r.Close()
		if (index == 1) != (err == nil) {
			t.Fatalf("bootstrap case %d: %v", index, err)
		}
	}
	repo, err := sqlite.Open(db, key)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	var accounts []domain.Account
	if err = repo.List("accounts", &accounts); err != nil || len(accounts) != 1 || !security.CheckPassword(accounts[0].Hash, "secure unique passphrase") {
		t.Fatal("bootstrap preservation failed", err)
	}
}
