package security

import (
	"bytes"
	"testing"
)

func TestVaultAuthenticatedContext(t *testing.T) {
	v, e := NewVault(bytes.Repeat([]byte{7}, 32))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := v.Seal([]byte("router-password"), "routers:one")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("router-password")) {
		t.Fatal("plaintext")
	}
	if _, e = v.Open(raw, "routers:two"); e == nil {
		t.Fatal("context substitution accepted")
	}
	raw[len(raw)-1] ^= 1
	if _, e = v.Open(raw, "routers:one"); e == nil {
		t.Fatal("tampering accepted")
	}
}
func TestPassword(t *testing.T) {
	h, e := HashPassword("a very long password 123")
	if e != nil {
		t.Fatal(e)
	}
	if !CheckPassword(h, "a very long password 123") || CheckPassword(h, "incorrect password") {
		t.Fatal("password verification")
	}
	short, e := HashPassword("1")
	if e != nil || !CheckPassword(short, "1") || CheckPassword(short, "2") {
		t.Fatal("user-selected short password was not supported")
	}
	if _, e = HashPassword(""); e == nil {
		t.Fatal("empty password accepted")
	}
	if _, e = HashPassword(string(bytes.Repeat([]byte{'a'}, 234))); e == nil {
		t.Fatal("oversized password accepted")
	}
}
