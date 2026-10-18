package config

import (
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Version                              string
	Address, Database, Origin, Cert, Key string
	Master                               []byte
	Allowed                              []netip.Prefix
	TrustedProxies                       []string
	AdditionalOrigins                    []string
	Mutations, Dev, ProxyTLS             bool
	AllowHTTP                            bool
	TLSCAFile                            string
	Timeout                              time.Duration
}

func Load() (Config, error) {
	c := Config{Address: env("MVM_LISTEN", "127.0.0.1:8080"), Database: env("MVM_DATABASE", defaultDatabase()), Origin: env("MVM_ORIGIN", "https://localhost:8443"), Cert: os.Getenv("MVM_TLS_CERT"), Key: os.Getenv("MVM_TLS_KEY"), Mutations: os.Getenv("MVM_ALLOW_MUTATIONS") == "true", Dev: os.Getenv("MVM_DEV_HTTP") == "true", Timeout: 20 * time.Second}
	var e error
	c.ProxyTLS = os.Getenv("MVM_PROXY_TLS") == "true"
	c.AllowHTTP = os.Getenv("MVM_ALLOW_HTTP") == "true"
	if c.AllowHTTP && (c.Cert != "" || c.Key != "" || c.ProxyTLS) {
		return c, errors.New("explicit HTTP mode cannot be combined with TLS")
	}
	c.TLSCAFile = os.Getenv("MVM_TLS_CA_FILE")
	if raw := os.Getenv("MVM_TRUSTED_PROXIES"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if _, err := netip.ParseAddr(value); err != nil {
				p, err := netip.ParsePrefix(value)
				if err != nil || p.Bits() == 0 {
					return c, errors.New("MVM_TRUSTED_PROXIES requires explicit IPs or restricted CIDRs")
				}
			}
			c.TrustedProxies = append(c.TrustedProxies, value)
		}
	}
	if (c.Cert == "") != (c.Key == "") {
		return c, errors.New("provide both MVM_TLS_CERT and MVM_TLS_KEY")
	}
	master := os.Getenv("MVM_MASTER_KEY")
	if path := os.Getenv("MVM_MASTER_KEY_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return c, err
		}
		master = strings.TrimSpace(string(raw))
	}
	c.Master, e = base64.StdEncoding.DecodeString(master)
	if e != nil || len(c.Master) != 32 {
		return c, errors.New("MVM_MASTER_KEY must be a base64-encoded random 32-byte key")
	}
	u, e := url.Parse(c.Origin)
	if e != nil || !validOrigin(u) || u.Scheme != "https" && !((c.Dev || c.AllowHTTP) && u.Scheme == "http") {
		return c, errors.New("MVM_ORIGIN must use HTTPS, explicitly enabled HTTP, or loopback development HTTP")
	}
	if c.AllowHTTP && u.Scheme != "http" {
		return c, errors.New("explicit HTTP mode requires an HTTP origin")
	}
	if raw := os.Getenv("MVM_ADDITIONAL_ORIGINS"); raw != "" {
		for _, origin := range strings.Split(raw, ",") {
			origin = strings.TrimSpace(origin)
			alias, err := url.Parse(origin)
			if err != nil || !validOrigin(alias) || alias.Scheme != u.Scheme {
				return c, errors.New("additional origins must be exact origins using the panel scheme")
			}
			c.AdditionalOrigins = append(c.AdditionalOrigins, origin)
		}
	}
	if c.Dev {
		h, _, e := net.SplitHostPort(c.Address)
		if e != nil || h != "127.0.0.1" && h != "::1" {
			return c, errors.New("development HTTP must listen on loopback")
		}
	}
	if raw := os.Getenv("MVM_ROUTER_CIDRS"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			prefix, e := netip.ParsePrefix(strings.TrimSpace(p))
			if e != nil {
				return c, errors.New("invalid MVM_ROUTER_CIDRS")
			}
			c.Allowed = append(c.Allowed, prefix)
		}
	}
	if raw := os.Getenv("MVM_ROUTER_TIMEOUT_SECONDS"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 55 {
			return c, errors.New("router timeout must be 1–55 seconds")
		}
		c.Timeout = time.Duration(n) * time.Second
	}
	return c, nil
}
func validOrigin(u *url.URL) bool {
	return u != nil && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.Path == "" && u.Opaque == "" && (u.Scheme == "http" || u.Scheme == "https")
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
