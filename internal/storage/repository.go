package storage

import "io"

// ManagedRepository adds operational capabilities without coupling domain logic to an engine.
type ManagedRepository interface {
	Repository
	Close() error
	Check() error
	Backup(io.Writer) error
}
