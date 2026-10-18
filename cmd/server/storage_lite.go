//go:build lite

package main

import "mikrotik-vpn-manager/internal/storage"

func openRepository(path string, key []byte) (storage.ManagedRepository, error) {
	return storage.Open(path, key)
}
func restoreRepository(path, backup string, key []byte) error {
	return storage.Restore(path, backup, key)
}
