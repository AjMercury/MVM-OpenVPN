package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineRestore(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup.db")
	s, e := Open(backup, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Put("settings", "example", "persisted"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	dest := filepath.Join(dir, "restored.db")
	if e = Restore(dest, backup, bytes.Repeat([]byte{8}, 32)); e == nil {
		t.Fatal("wrong restore key accepted")
	}
	if _, e = os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("failed restore created destination")
	}
	if e = Restore(dest, backup, key); e != nil {
		t.Fatal(e)
	}
	s, e = Open(dest, key)
	if e != nil {
		t.Fatal(e)
	}
	var got string
	e = s.Get("settings", "example", &got)
	s.Close()
	if e != nil || got != "persisted" {
		t.Fatal(e)
	}
	if e = Restore(dest, backup, key); e == nil {
		t.Fatal("restore overwrote existing database")
	}
}
