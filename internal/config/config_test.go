package config

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestConfigGuards(t *testing.T) {
	t.Setenv("MVM_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	t.Setenv("MVM_DEV_HTTP", "true")
	t.Setenv("MVM_LISTEN", "127.0.0.1:8080")
	t.Setenv("MVM_ORIGIN", "http://localhost:8080")
	t.Setenv("MVM_ROUTER_CIDRS", "10.0.0.1/32")
	if _, e := Load(); e != nil {
		t.Fatal(e)
	}
	t.Setenv("MVM_LISTEN", "0.0.0.0:8080")
	if _, e := Load(); e == nil {
		t.Fatal("public dev HTTP accepted")
	}
	t.Setenv("MVM_LISTEN", "127.0.0.1:8080")
	t.Setenv("MVM_ROUTER_TIMEOUT_SECONDS", "60")
	if _, e := Load(); e == nil {
		t.Fatal("unsafe timeout")
	}
	t.Setenv("MVM_ROUTER_TIMEOUT_SECONDS", "20")
	t.Setenv("MVM_MASTER_KEY", "invalid")
	if _, e := Load(); e == nil {
		t.Fatal("missing master key accepted")
	}
}
