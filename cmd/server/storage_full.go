//go:build !lite

package main

import (
	"errors"
	"mikrotik-vpn-manager/internal/storage"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"os"
	"path/filepath"
)

func openRepository(path string, key []byte) (storage.ManagedRepository, error) {
	if _, e := os.Stat(path); os.IsNotExist(e) {
		for _, old := range []string{"data/manager.db", filepath.Join(filepath.Dir(path), "manager.db")} {
			if old != path {
				if _, e := os.Stat(old); e == nil {
					return nil, errors.New("legacy database detected; explicit offline migration required before SQLite startup")
				}
			}
		}
	}
	return sqlite.Open(path, key)
}
func restoreRepository(path, backup string, key []byte) error {
	return sqlite.Restore(path, backup, key)
}
