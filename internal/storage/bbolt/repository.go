// Package bbolt exposes the compatible v0.1.0 encrypted repository.
package bbolt

import "mikrotik-vpn-manager/internal/storage"

var Open = storage.Open
var Restore = storage.Restore
