package routers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	for _, v := range []string{"7.17", "7.17.2 (stable)", "7.22.1"} {
		if major, _, _, e := Version(v); e != nil || major != 7 {
			t.Fatal(v, e)
		}
	}
	for _, v := range []string{"", "7", "7.17rc3", "7.17beta5", "invalid", "7.-1"} {
		if _, _, _, e := Version(v); e == nil {
			t.Fatal("invalid accepted", v)
		}
	}
}
func TestSSRF(t *testing.T) {
	allowed := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.0/8")}
	for _, ip := range []string{"127.0.0.1", "169.254.169.254", "8.8.8.8", "::1", "::ffff:127.0.0.1"} {
		if ValidateDestination(netip.MustParseAddr(ip), allowed) == nil {
			t.Fatal("forbidden", ip)
		}
	}
	if e := ValidateDestination(netip.MustParseAddr("10.0.0.1"), allowed); e != nil {
		t.Fatal(e)
	}
	for _, host := range []string{"user@host", "http://router", "router/path", "router\n", "a..b", "-router"} {
		if ValidHost(host) {
			t.Fatal(host)
		}
	}
}
func TestVerifiedMockREST(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wantErr    bool
	}{{"object", `{"version":"7.17"}`, 200, false}, {"array", `[{".id":"*1","name":"pool"}]`, 200, false}, {"auth", `{"password":"do-not-leak"}`, 401, true}, {"permission", `{}`, 403, true}, {"rate", `{}`, 429, true}, {"unexpected", `<html>wrong</html>`, 200, true}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, p, ok := r.BasicAuth()
				if !ok || u != "api" || p != "secret" {
					t.Error("missing basic auth")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			c := &Client{http: srv.Client(), base: srv.URL + "/rest/", user: "api", password: "secret"}
			_, e := c.Call(context.Background(), "GET", "system/resource", nil)
			if (e != nil) != tc.wantErr {
				t.Fatal(e)
			}
			if e != nil && strings.Contains(e.Error(), "do-not-leak") {
				t.Fatal("response leaked")
			}
		})
	}
}
func TestTLSAndTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(30 * time.Millisecond); fmt.Fprint(w, `{}`) }))
	defer srv.Close()
	c := &Client{http: &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}, base: srv.URL + "/rest/"}
	if _, e := c.Call(context.Background(), "GET", "system/resource", nil); e == nil {
		t.Fatal("untrusted TLS accepted")
	}
	c.http = srv.Client()
	c.http.Timeout = time.Millisecond
	if _, e := c.Call(context.Background(), "GET", "system/resource", nil); e == nil {
		t.Fatal("timeout ignored")
	}
}

type fileAPI struct {
	data    string
	partial bool
}

func (f fileAPI) Call(_ context.Context, method, path string, body any) ([]Record, error) {
	if method == "GET" {
		return []Record{{"name": "client.crt", "size": fmt.Sprint(len(f.data))}}, nil
	}
	v := body.(Record)
	var offset, n int
	fmt.Sscan(v["offset"], &offset)
	fmt.Sscan(v["chunk-size"], &n)
	if f.partial {
		n--
	}
	return []Record{{"data": f.data[offset : offset+n]}}, nil
}
func TestChunkedFile(t *testing.T) {
	raw := strings.Repeat("x", 70000)
	b, e := ReadFile(context.Background(), fileAPI{data: raw}, "client.crt")
	if e != nil || string(b) != raw {
		t.Fatal("chunk retrieval", e)
	}
	if _, e = ReadFile(context.Background(), fileAPI{data: raw, partial: true}, "client.crt"); e == nil {
		t.Fatal("partial accepted")
	}
}
