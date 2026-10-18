package storage

import (
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"mikrotik-vpn-manager/internal/security"
	"time"
)

// ReadLegacy opens only an existing database, read-only, and takes the bbolt file lock.
// The callback sees ciphertext; plaintext is verified locally and never written to disk.
func ReadLegacy(path string, key []byte, visit func(string, string, []byte, json.RawMessage) error) error {
	v, e := security.NewVault(key)
	if e != nil {
		return e
	}
	db, e := bolt.Open(path, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if e != nil {
		return e
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte("meta"))
		if meta == nil || string(meta.Get([]byte("schema"))) != "1" {
			return errors.New("unsupported legacy schema")
		}
		settings := tx.Bucket([]byte("settings"))
		if settings == nil {
			return errors.New("missing master key sentinel")
		}
		p, e := v.Open(settings.Get([]byte("key_check")), "settings:key_check")
		if e != nil || string(p) != `"verified"` {
			return errors.New("legacy master key verification failed")
		}
		for e := range tx.Check() {
			if e != nil {
				return e
			}
		}
		return tx.ForEach(func(b []byte, bucket *bolt.Bucket) error {
			if string(b) == "meta" {
				return nil
			}
			return bucket.ForEach(func(id, sealed []byte) error {
				raw, e := v.Open(sealed, string(b)+":"+string(id))
				if e != nil {
					return e
				}
				if !json.Valid(raw) {
					return errors.New("invalid legacy record")
				}
				return visit(string(b), string(id), sealed, raw)
			})
		})
	})
}
