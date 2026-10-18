package storage

import (
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"mikrotik-vpn-manager/internal/security"
	"os"
	"path/filepath"
	"time"
)

// Restore refuses to overwrite any target. Stop the service and preserve its old
// database elsewhere before invoking this offline command.
func Restore(destination, backup string, key []byte) error {
	src, e := filepath.Abs(backup)
	if e != nil {
		return e
	}
	dst, e := filepath.Abs(destination)
	if e != nil {
		return e
	}
	if src == dst {
		return errors.New("backup and destination must differ")
	}
	v, e := security.NewVault(key)
	if e != nil {
		return e
	}
	db, e := bolt.Open(src, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if e != nil {
		return e
	}
	defer db.Close()
	e = db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte("meta"))
		if meta == nil || string(meta.Get([]byte("schema"))) != "1" {
			return errors.New("backup schema unsupported")
		}
		settings := tx.Bucket([]byte("settings"))
		if settings == nil {
			return errors.New("backup missing key sentinel")
		}
		if _, e := v.Open(settings.Get([]byte("key_check")), "settings:key_check"); e != nil {
			return errors.New("backup master key verification failed")
		}
		for e := range tx.Check() {
			if e != nil {
				return e
			}
		}
		if e := tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			if string(name) == "meta" {
				return nil
			}
			return b.ForEach(func(k, record []byte) error {
				if record == nil {
					return errors.New("unexpected nested bucket in backup")
				}
				raw, e := v.Open(record, string(name)+":"+string(k))
				if e != nil || !json.Valid(raw) {
					return errors.New("backup contains an invalid encrypted record")
				}
				return nil
			})
		}); e != nil {
			return e
		}
		return nil
	})
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
		return e
	}
	temp, e := os.CreateTemp(filepath.Dir(dst), ".restore-*")
	if e != nil {
		return e
	}
	name := temp.Name()
	defer os.Remove(name)
	if e = temp.Chmod(0600); e != nil {
		temp.Close()
		return e
	}
	e = db.View(func(tx *bolt.Tx) error { _, e := tx.WriteTo(temp); return e })
	if e == nil {
		e = temp.Sync()
	}
	closeErr := temp.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	// A hard link publishes the complete file atomically and fails if dst exists.
	if e = os.Link(name, dst); e != nil {
		return errors.New("restore target must be absent and filesystem must support atomic hard-link publication: " + e.Error())
	}
	return nil
}
