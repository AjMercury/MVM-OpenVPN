package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"

	"mikrotik-vpn-manager/internal/storage"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Migrate requires an offline source and absent destination/backup. No source writes occur.
// A verified consistent backup is mandatory; the complete temporary import is the rehearsal.
func Migrate(source, destination, backup string, key []byte) error {
	return migrate(source, destination, backup, key, nil)
}

// afterImport is an internal fault-injection hook, never exposed by the CLI.
func migrate(source, destination, backup string, key []byte, afterImport func() error) error {
	for _, p := range []string{destination, backup} {
		if _, e := os.Lstat(p); !os.IsNotExist(e) {
			return errors.New("destination and backup must be new paths")
		}
	}
	noop := func(string, string, []byte, json.RawMessage) error { return nil }
	if e := storage.ReadLegacy(source, key, noop); e != nil {
		return e
	}
	db, e := bolt.Open(source, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if e != nil {
		return e
	}
	defer db.Close()
	f, e := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	e = (&storage.Store{DB: db}).Backup(f)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = storage.ReadLegacy(backup, key, noop); e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(destination), 0700); e != nil {
		return e
	}
	dir, e := os.MkdirTemp(filepath.Dir(destination), ".migration-*")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	staged := filepath.Join(dir, "mvm.db")
	repo, e := sqlite.Open(staged, key)
	if e != nil {
		return e
	}
	defer repo.Close()
	count := 0
	type record struct {
		b, id string
		raw   json.RawMessage
	}
	records := []record{}
	e = storage.ReadLegacy(backup, key, func(b, id string, _ []byte, raw json.RawMessage) error {
		records = append(records, record{b, id, append(json.RawMessage{}, raw...)})
		return nil
	})
	if e != nil {
		return e
	}
	rank := map[string]int{"settings": 0, "routers": 1, "accounts": 2, "jobs": 3, "servers": 4, "users": 5, "sessions": 6, "deliveries": 7}
	sort.SliceStable(records, func(i, j int) bool { return rank[records[i].b] < rank[records[j].b] })
	for _, r := range records {
		if e = repo.Put(r.b, r.id, r.raw); e != nil {
			return e
		}
		count++
	}
	if afterImport != nil {
		if e = afterImport(); e != nil {
			return e
		}
	}
	// Compare every stable identifier and canonical JSON value after re-encryption.
	checked := 0
	e = storage.ReadLegacy(backup, key, func(b, id string, _ []byte, raw json.RawMessage) error {
		var got json.RawMessage
		if e := repo.Get(b, id, &got); e != nil {
			return e
		}
		var a, c bytes.Buffer
		if e := json.Compact(&a, raw); e != nil {
			return e
		}
		if e := json.Compact(&c, got); e != nil {
			return e
		}
		if !bytes.Equal(a.Bytes(), c.Bytes()) {
			return errors.New("migration record mismatch")
		}
		checked++
		return nil
	})
	if e != nil {
		return e
	}
	actual, e := repo.CountRecords()
	if e != nil {
		return e
	}
	if checked != count || actual != count {
		return errors.New("migration count mismatch")
	}
	if e = repo.Check(); e != nil {
		return e
	}
	// Supported SQLite snapshot removes the WAL publication problem.
	out, e := os.OpenFile(filepath.Join(dir, "activated.db"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	e = repo.Backup(out)
	if e == nil {
		e = out.Sync()
	}
	ce = out.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = sqlite.Restore(destination, filepath.Join(dir, "activated.db"), key); e != nil {
		return e
	}
	return nil
}
