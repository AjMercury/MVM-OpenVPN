package storage_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"mikrotik-vpn-manager/internal/storage"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"path/filepath"
	"sync"
	"testing"
)

func TestRepositoryContract(t *testing.T) {
	for name, open := range map[string]func(string, []byte) (storage.ManagedRepository, error){
		"bbolt":  func(p string, k []byte) (storage.ManagedRepository, error) { return storage.Open(p, k) },
		"sqlite": func(p string, k []byte) (storage.ManagedRepository, error) { return sqlite.Open(p, k) },
	} {
		t.Run(name, func(t *testing.T) {
			key := bytes.Repeat([]byte{5}, 32)
			s, e := open(filepath.Join(t.TempDir(), "db"), key)
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			if e = s.Put("settings", "counter", 0); e != nil {
				t.Fatal(e)
			}
			var wg sync.WaitGroup
			for range 20 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if e := s.Update("settings", "counter", func(raw []byte) ([]byte, error) {
						var n int
						if e := json.Unmarshal(raw, &n); e != nil {
							return nil, e
						}
						return json.Marshal(n + 1)
					}); e != nil {
						t.Error(e)
					}
				}()
			}
			wg.Wait()
			var n int
			if e = s.Get("settings", "counter", &n); e != nil || n != 20 {
				t.Fatalf("counter=%d err=%v", n, e)
			}
			sentinel := errors.New("abort")
			if e = s.Update("settings", "counter", func([]byte) ([]byte, error) { return nil, sentinel }); !errors.Is(e, sentinel) {
				t.Fatal(e)
			}
			s.Get("settings", "counter", &n)
			if n != 20 {
				t.Fatal("transaction committed on failure")
			}
			var values []int
			if e = s.List("unknown", &values); e != nil || len(values) != 0 {
				t.Fatal(e)
			}
			if e = s.Delete("settings", "counter"); e != nil {
				t.Fatal(e)
			}
			if e = s.Get("settings", "counter", &n); !errors.Is(e, storage.ErrNotFound) {
				t.Fatal(e)
			}
			if e = s.Check(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
