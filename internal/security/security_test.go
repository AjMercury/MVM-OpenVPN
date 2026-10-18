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
	if _, e = HashPassword("short"); e == nil {
		t.Fatal("short password accepted")
	}
}
