package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage/migration"
	"mikrotik-vpn-manager/internal/storage/sqlite"
	"os"
	"strings"
)

var version = "1.0.3-dev"

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(version + "; SQLite schema 1; RouterOS automation experimental")
		return nil
	}
	raw, e := os.ReadFile(os.Getenv("MVM_MASTER_KEY_FILE"))
	if e != nil {
		return e
	}
	key, e := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if e != nil {
		return e
	}
	if len(os.Args) == 6 && os.Args[1] == "bootstrap-admin" {
		if _, e = os.Stat(os.Args[2]); e != nil {
			return e
		}
		input, err := io.ReadAll(io.LimitReader(os.Stdin, 235))
		if err != nil {
			return err
		}
		if len(input) > 233 {
			return errors.New("password exceeds 233 bytes")
		}
		s, err := sqlite.Open(os.Args[2], key)
		if err != nil {
			return err
		}
		defer s.Close()
		return s.BootstrapAdmin(os.Args[3], string(input), os.Args[4], os.Args[5])
	}
	if len(os.Args) == 4 && os.Args[1] == "reset-admin" {
		if _, e = os.Stat(os.Args[2]); e != nil {
			return e
		}
		raw, e := io.ReadAll(io.LimitReader(os.Stdin, 235))
		if e != nil {
			return e
		}
		password := strings.TrimSuffix(string(raw), "\n")
		hash, e := security.HashPassword(password)
		if e != nil {
			return e
		}
		s, e := sqlite.Open(os.Args[2], key)
		if e != nil {
			return e
		}
		defer s.Close()
		var accounts []domain.Account
		if e = s.List("accounts", &accounts); e != nil {
			return e
		}
		for _, a := range accounts {
			if a.Username == os.Args[3] && a.Role == domain.Super {
				if e = s.Update("accounts", a.ID, func(raw []byte) ([]byte, error) {
					var fresh domain.Account
					if e := json.Unmarshal(raw, &fresh); e != nil {
						return nil, e
					}
					fresh.Hash = hash
					return json.Marshal(fresh)
				}); e != nil {
					return e
				}
				// Invalidate all existing sessions. Session IDs are digests, not values in the records.
				return s.DeleteCollection("sessions")
			}
		}
		return errors.New("existing super administrator not found")
	}
	if len(os.Args) == 5 && os.Args[1] == "migrate-bbolt" {
		return migration.Migrate(os.Args[2], os.Args[3], os.Args[4], key)
	}
	if len(os.Args) == 4 && os.Args[1] == "restore" {
		return sqlite.Restore(os.Args[2], os.Args[3], key)
	}
	if len(os.Args) == 3 && (os.Args[1] == "check" || os.Args[1] == "init") {
		if os.Args[1] == "check" {
			if _, e = os.Stat(os.Args[2]); e != nil {
				return e
			}
		}
		s, e := sqlite.Open(os.Args[2], key)
		if e != nil {
			return e
		}
		defer s.Close()
		return s.Check()
	}
	if len(os.Args) == 4 && os.Args[1] == "backup" {
		if _, e = os.Stat(os.Args[2]); e != nil {
			return e
		}
		s, e := sqlite.Open(os.Args[2], key)
		if e != nil {
			return e
		}
		defer s.Close()
		if e = s.Check(); e != nil {
			return e
		}
		f, e := os.OpenFile(os.Args[3], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		e = s.Backup(f)
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
		verify, e := os.CreateTemp("", "mvm-verify-*")
		if e != nil {
			return e
		}
		p := verify.Name()
		verify.Close()
		os.Remove(p)
		defer os.Remove(p)
		return sqlite.Restore(p, os.Args[3], key)
	}
	return errors.New("usage: mvmctl bootstrap-admin DB USERNAME TIMEZONE en|fa (password on protected stdin) | reset-admin DB USERNAME | version | init DB | check DB | backup DB NEW_BACKUP | restore NEW_DB BACKUP | migrate-bbolt SOURCE NEW_SQLITE NEW_BACKUP; requires MVM_MASTER_KEY_FILE")
}
