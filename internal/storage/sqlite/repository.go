package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"io"
	"mikrotik-vpn-manager/internal/security"
	"mikrotik-vpn-manager/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store serializes transactions on one connection. WAL permits consistent snapshot backup.
type Store struct {
	db    *sql.DB
	vault *security.Vault
	mu    sync.Mutex
}

var _ storage.ManagedRepository = (*Store)(nil)

func Open(path string, key []byte) (*Store, error) {
	v, e := security.NewVault(key)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	// Reject other formats before SQLite can modify them.
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	fresh := e == nil
	if fresh {
		f.Close()
	} else if !os.IsExist(e) {
		return nil, e
	} else {
		f, e = os.Open(path)
		if e != nil {
			return nil, e
		}
		var magic [16]byte
		_, e = io.ReadFull(f, magic[:])
		f.Close()
		if e != nil || string(magic[:]) != "SQLite format 3\x00" {
			return nil, errors.New("existing database is not SQLite; explicit offline migration required")
		}
	}
	db, e := driver.Open(path, func(c *sqlite3.Conn) error {
		return c.Exec("PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON; PRAGMA synchronous=FULL;")
	})
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, vault: v}
	fail := func(e error) (*Store, error) { db.Close(); return nil, e }
	for _, q := range []string{"PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON"} {
		if _, e = db.Exec(q); e != nil {
			return fail(e)
		}
	}
	var schema int
	if e = db.QueryRow("PRAGMA user_version").Scan(&schema); e != nil {
		return fail(e)
	}
	if !fresh && schema != 1 {
		return fail(errors.New("unsupported SQLite schema"))
	}
	if fresh {
		tx, e := db.Begin()
		if e != nil {
			return fail(e)
		}
		_, e = tx.Exec(`CREATE TABLE collections(name TEXT PRIMARY KEY);
CREATE TABLE records(collection TEXT NOT NULL REFERENCES collections(name), id TEXT NOT NULL, payload BLOB NOT NULL, unique_name TEXT, identity TEXT, allocation TEXT, PRIMARY KEY(collection,id));
CREATE TABLE record_references(source_collection TEXT NOT NULL, source_id TEXT NOT NULL, target_collection TEXT NOT NULL, target_id TEXT NOT NULL,
PRIMARY KEY(source_collection,source_id,target_collection,target_id),
FOREIGN KEY(source_collection,source_id) REFERENCES records(collection,id) ON DELETE CASCADE,
FOREIGN KEY(target_collection,target_id) REFERENCES records(collection,id));
CREATE INDEX reference_targets ON record_references(target_collection,target_id);
CREATE UNIQUE INDEX records_unique_name ON records(collection,unique_name) WHERE unique_name IS NOT NULL;
CREATE UNIQUE INDEX records_identity ON records(collection,identity) WHERE identity IS NOT NULL;
CREATE UNIQUE INDEX records_allocation ON records(collection,allocation) WHERE allocation IS NOT NULL;
PRAGMA user_version=1;`)
		if e != nil {
			tx.Rollback()
			return fail(e)
		}
		if e = tx.Commit(); e != nil {
			return fail(e)
		}
		if e = s.Put("settings", "key_check", "verified"); e != nil {
			return fail(e)
		}
	} else {
		var check string
		if e = s.Get("settings", "key_check", &check); e != nil || check != "verified" {
			return fail(errors.New("SQLite master key verification failed"))
		}
	}
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL"} {
		if _, e = db.Exec(q); e != nil {
			return fail(e)
		}
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Get(b, id string, out any) error {
	var sealed []byte
	e := s.db.QueryRow("SELECT payload FROM records WHERE collection=? AND id=?", b, id).Scan(&sealed)
	if errors.Is(e, sql.ErrNoRows) {
		return storage.ErrNotFound
	}
	if e != nil {
		return e
	}
	raw, e := s.vault.Open(sealed, b+":"+id)
	if e != nil {
		return e
	}
	return json.Unmarshal(raw, out)
}
func (s *Store) Put(b, id string, v any) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return s.Update(b, id, func([]byte) ([]byte, error) { return raw, nil })
}
func (s *Store) Update(b, id string, fn func([]byte) ([]byte, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var sealed, raw []byte
	e = tx.QueryRow("SELECT payload FROM records WHERE collection=? AND id=?", b, id).Scan(&sealed)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if e == nil {
		raw, e = s.vault.Open(sealed, b+":"+id)
		if e != nil {
			return e
		}
	}
	raw, e = fn(raw)
	if e != nil {
		return e
	}
	if !json.Valid(raw) {
		return errors.New("invalid JSON record")
	}
	sealed, e = s.vault.Seal(raw, b+":"+id)
	if e != nil {
		return e
	}
	var name, identity, allocation any
	var projected struct {
		RouterID string `json:"router_id"`
		Protocol string `json:"protocol"`
		Username string `json:"username"`
		Address  string `json:"address"`
		Key      string `json:"key"`
		Spec     struct {
			RouterID string `json:"router_id"`
			Protocol string `json:"protocol"`
		} `json:"spec"`
	}
	if e = json.Unmarshal(raw, &projected); e != nil && (b == "users" || b == "jobs" || b == "servers") {
		return e
	}
	switch b {
	case "users":
		if projected.RouterID != "" && projected.Username != "" {
			identity = security.Digest(projected.RouterID + ":" + projected.Username)
			if projected.Protocol == "wireguard" {
				identity = security.Digest(projected.RouterID + ":wireguard:" + projected.Username)
			}
		}
		if projected.RouterID != "" && projected.Address != "" {
			allocation = security.Digest(projected.RouterID + ":" + projected.Address)
		}
	case "jobs":
		if projected.RouterID != "" && projected.Key != "" {
			identity = security.Digest(projected.RouterID + ":" + projected.Key)
		}
	case "servers":
		if projected.Spec.RouterID != "" {
			identity = security.Digest(projected.Spec.RouterID)
			if projected.Spec.Protocol != "" && projected.Spec.Protocol != "openvpn" {
				identity = security.Digest(projected.Spec.RouterID + ":" + projected.Spec.Protocol)
			}
		}
	}
	if b == "accounts" {
		var a struct {
			Username string `json:"username"`
		}
		if e = json.Unmarshal(raw, &a); e != nil {
			return e
		}
		if a.Username != "" {
			name = security.Digest(a.Username)
		}
	}
	if _, e = tx.Exec("INSERT INTO collections(name) VALUES(?) ON CONFLICT DO NOTHING", b); e != nil {
		return e
	}
	_, e = tx.Exec("INSERT INTO records(collection,id,payload,unique_name,identity,allocation) VALUES(?,?,?,?,?,?) ON CONFLICT(collection,id) DO UPDATE SET payload=excluded.payload,unique_name=excluded.unique_name,identity=excluded.identity,allocation=excluded.allocation", b, id, sealed, name, identity, allocation)
	if e != nil {
		return e
	}
	if _, e = tx.Exec("DELETE FROM record_references WHERE source_collection=? AND source_id=?", b, id); e != nil {
		return e
	}
	for _, ref := range references(b, raw) {
		if _, e = tx.Exec("INSERT INTO record_references VALUES(?,?,?,?)", b, id, ref.bucket, ref.id); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) Delete(b, id string) error {
	_, e := s.db.Exec("DELETE FROM records WHERE collection=? AND id=?", b, id)
	return e
}
func (s *Store) List(b string, out any) error {
	rows, e := s.db.Query("SELECT id,payload FROM records WHERE collection=? ORDER BY id", b)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var id string
		var sealed []byte
		if e = rows.Scan(&id, &sealed); e != nil {
			return e
		}
		raw, e := s.vault.Open(sealed, b+":"+id)
		if e != nil {
			return e
		}
		items = append(items, raw)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	raw, e := json.Marshal(items)
	if e != nil {
		return e
	}
	return json.Unmarshal(raw, out)
}
func (s *Store) RecordIDs(bucket string) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM records WHERE collection=? ORDER BY id", bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *Store) Check() error {
	var result string
	if e := s.db.QueryRow("PRAGMA integrity_check").Scan(&result); e != nil {
		return e
	}
	if result != "ok" {
		return errors.New("SQLite integrity check failed")
	}
	rows, e := s.db.Query("PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	bad := rows.Next()
	queryErr := rows.Err()
	rows.Close()
	if queryErr != nil {
		return queryErr
	}
	if bad {
		return errors.New("foreign key check failed")
	}
	rows, e = s.db.Query("SELECT collection,id,payload FROM records")
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var b, id string
		var p []byte
		if e = rows.Scan(&b, &id, &p); e != nil {
			return e
		}
		raw, e := s.vault.Open(p, b+":"+id)
		if e != nil {
			return e
		}
		if !json.Valid(raw) {
			return errors.New("invalid encrypted JSON")
		}
	}
	return rows.Err()
}

// Backup uses SQLite VACUUM INTO, including committed WAL content in a consistent snapshot.
func (s *Store) Backup(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, e := os.MkdirTemp("", "mvm-snapshot-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "snapshot.db")
	if _, e = s.db.Exec("VACUUM INTO '" + strings.ReplaceAll(path, "'", "''") + "'"); e != nil {
		return e
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = io.Copy(w, f)
	return e
}

// Restore verifies a private copy before atomic no-replace publication. Destination must be absent.
func Restore(destination, backup string, key []byte) error {
	if _, e := os.Lstat(destination); !os.IsNotExist(e) {
		return errors.New("restore destination must be absent")
	}
	if e := os.MkdirAll(filepath.Dir(destination), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(destination), ".restore-*")
	if e != nil {
		return e
	}
	p := f.Name()
	defer os.Remove(p)
	src, e := os.Open(backup)
	if e != nil {
		f.Close()
		return e
	}
	_, e = io.Copy(f, src)
	src.Close()
	if e == nil {
		e = f.Sync()
	}
	f.Close()
	if e != nil {
		return e
	}
	s, e := Open(p, key)
	if e != nil {
		return e
	}
	e = s.Check()
	ce := s.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Link(p, destination)
}

// DeleteCollection is an offline credential-recovery operation, outside business workflows.
func (s *Store) DeleteCollection(name string) error {
	_, e := s.db.Exec("DELETE FROM records WHERE collection=?", name)
	return e
}

// CountRecords supports complete offline migration count comparison, including unknown collections.
func (s *Store) CountRecords() (int, error) {
	var n int
	e := s.db.QueryRow("SELECT count(*) FROM records").Scan(&n)
	return n, e
}
