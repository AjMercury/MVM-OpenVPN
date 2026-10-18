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

func TestTrustedProxyConfiguration(t *testing.T) {
	t.Setenv("MVM_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	t.Setenv("MVM_DEV_HTTP", "true")
	t.Setenv("MVM_LISTEN", "127.0.0.1:8080")
	t.Setenv("MVM_ORIGIN", "http://localhost:8080")
	for _, value := range []string{"0.0.0.0/0", "::/0", "proxy.invalid", "", "172.30.248.3", "10.0.0.0/24"} {
		t.Setenv("MVM_TRUSTED_PROXIES", value)
		_, err := Load()
		valid := value == "" || value == "172.30.248.3" || value == "10.0.0.0/24"
		if (err == nil) != valid {
			t.Fatalf("proxy %q: %v", value, err)
		}
	}
}

func TestDirectHTTPSAndExactOrigins(t *testing.T) {
	t.Setenv("MVM_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	t.Setenv("MVM_DEV_HTTP", "false")
	t.Setenv("MVM_LISTEN", "172.16.180.58:8443")
	t.Setenv("MVM_ORIGIN", "http://172.16.180.58:8443")
	if _, err := Load(); err == nil {
		t.Fatal("remote HTTP enabled implicitly")
	}
	t.Setenv("MVM_ORIGIN", "https://172.16.180.58:8443")
	t.Setenv("MVM_ADDITIONAL_ORIGINS", "https://panel.example.test:8443")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Dev || len(c.AdditionalOrigins) != 1 {
		t.Fatal("direct HTTPS config missing")
	}
	for _, alias := range []string{"http://panel.example.test:8443", "https://user:pass@panel.example.test:8443", "https://panel.example.test:8443/path", "https://panel.example.test:8443?"} {
		t.Setenv("MVM_ADDITIONAL_ORIGINS", alias)
		if _, err := Load(); err == nil {
			t.Fatal("unsafe alias", alias)
		}
	}
}

func TestExplicitNetworkHTTP(t *testing.T) {
	t.Setenv("MVM_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	t.Setenv("MVM_LISTEN", "0.0.0.0:8444")
	t.Setenv("MVM_ORIGIN", "http://192.0.2.45:8444")
	t.Setenv("MVM_ADDITIONAL_ORIGINS", "http://panel.example.test:8444")
	t.Setenv("MVM_ALLOW_HTTP", "true")
	c, err := Load()
	if err != nil || !c.AllowHTTP || c.Dev {
		t.Fatalf("explicit HTTP: %+v %v", c, err)
	}
	t.Setenv("MVM_TLS_CERT", "some.pem")
	if _, err := Load(); err == nil {
		t.Fatal("mixed TLS and HTTP accepted")
	}
	t.Setenv("MVM_TLS_CERT", "")
	t.Setenv("MVM_PROXY_TLS", "true")
	if _, err := Load(); err == nil {
		t.Fatal("mixed proxy TLS and HTTP accepted")
	}
	t.Setenv("MVM_PROXY_TLS", "false")
	t.Setenv("MVM_ORIGIN", "https://192.0.2.45:8444")
	if _, err := Load(); err == nil {
		t.Fatal("HTTPS origin accepted in HTTP mode")
	}
}
