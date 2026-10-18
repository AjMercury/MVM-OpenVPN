package storage

import (
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"io"
	"mikrotik-vpn-manager/internal/security"
	"os"
	"path/filepath"
	"time"
)

var ErrNotFound = errors.New("record not found")

type Repository interface {
	Get(string, string, any) error
	Put(string, string, any) error
	Delete(string, string) error
	List(string, any) error
	Update(string, string, func([]byte) ([]byte, error)) error
}
type Store struct {
	DB    *bolt.DB
	vault *security.Vault
}

func Open(path string, key []byte) (*Store, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	v, e := security.NewVault(key)
	if e != nil {
		return nil, e
	}
	db, e := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if e != nil {
		return nil, e
	}
	s := &Store{db, v}
	e = db.Update(func(tx *bolt.Tx) error {
		b, e := tx.CreateBucketIfNotExists([]byte("meta"))
		if e != nil {
			return e
		}
		old := b.Get([]byte("schema"))
		if len(old) > 0 && string(old) != "1" {
			return errors.New("unsupported database schema")
		}
		if e = b.Put([]byte("schema"), []byte("1")); e != nil {
			return e
		}
		return nil
	})
	if e != nil {
		db.Close()
		return nil, e
	}
	// A sentinel detects a wrong recovery key before any writes.
	var sentinel string
	e = s.Get("settings", "key_check", &sentinel)
	if errors.Is(e, ErrNotFound) {
		e = s.Put("settings", "key_check", "verified")
	}
	if e != nil {
		db.Close()
		return nil, errors.New("cannot decrypt database: check master key")
	}
	return s, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Get(bucket, id string, out any) error {
	return s.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return ErrNotFound
		}
		p := b.Get([]byte(id))
		if p == nil {
			return ErrNotFound
		}
		data, e := s.vault.Open(p, bucket+":"+id)
		if e != nil {
			return e
		}
		return json.Unmarshal(data, out)
	})
}
func (s *Store) Put(bucket, id string, value any) error {
	data, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return s.Update(bucket, id, func([]byte) ([]byte, error) { return data, nil })
}
func (s *Store) Update(bucket, id string, fn func([]byte) ([]byte, error)) error {
	return s.DB.Update(func(tx *bolt.Tx) error {
		b, e := tx.CreateBucketIfNotExists([]byte(bucket))
		if e != nil {
			return e
		}
		var data []byte
		if p := b.Get([]byte(id)); p != nil {
			data, e = s.vault.Open(p, bucket+":"+id)
			if e != nil {
				return e
			}
		}
		data, e = fn(data)
		if e != nil {
			return e
		}
		sealed, e := s.vault.Seal(data, bucket+":"+id)
		if e != nil {
			return e
		}
		return b.Put([]byte(id), sealed)
	})
}
func (s *Store) Delete(bucket, id string) error {
	return s.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}
func (s *Store) List(bucket string, out any) error {
	items := []json.RawMessage{}
	e := s.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			data, e := s.vault.Open(v, bucket+":"+string(k))
			if e != nil {
				return e
			}
			items = append(items, json.RawMessage(data))
			return nil
		})
	})
	if e != nil {
		return e
	}
	data, e := json.Marshal(items)
	if e != nil {
		return e
	}
	return json.Unmarshal(data, out)
}
func (s *Store) RecordIDs(bucket string) ([]string, error) {
	ids := []string{}
	err := s.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if v != nil {
				ids = append(ids, string(k))
			}
			return nil
		})
	})
	return ids, err
}
func (s *Store) Backup(w io.Writer) error {
	return s.DB.View(func(tx *bolt.Tx) error { _, e := tx.WriteTo(w); return e })
}
func (s *Store) Check() error {
	return s.DB.View(func(tx *bolt.Tx) error {
		for e := range tx.Check() {
			if e != nil {
				return e
			}
		}
		return nil
	})
}
