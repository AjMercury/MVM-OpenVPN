package routers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mikrotik-vpn-manager/internal/domain"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Record = map[string]string
type API interface {
	Call(context.Context, string, string, any) ([]Record, error)
}
type Client struct {
	http                 *http.Client
	base, user, password string
}
type APIError struct{ Status int }

func (e *APIError) Error() string {
	return fmt.Sprintf("RouterOS rejected request (HTTP %d); inspect router permissions and configuration", e.Status)
}
func ValidHost(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\@?#%\r\n\t ") {
		return false
	}
	if a, e := netip.ParseAddr(host); e == nil {
		return a.Is4() || a.Is6()
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return true
}
func ValidateDestination(addr netip.Addr, allowed []netip.Prefix) error {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return errors.New("router destination is forbidden")
	}
	for _, p := range allowed {
		if p.Contains(addr) {
			return nil
		}
	}
	return errors.New("router destination is outside MVM_ROUTER_CIDRS")
}
func New(r domain.Router, allowed []netip.Prefix, timeout time.Duration) (*Client, error) {
	if !ValidHost(r.Host) || r.Port < 1 || r.Port > 65535 {
		return nil, errors.New("invalid router host or port")
	}
	roots, e := x509.SystemCertPool()
	if e != nil {
		roots = x509.NewCertPool()
	}
	if r.CA != "" && !roots.AppendCertsFromPEM([]byte(r.CA)) {
		return nil, errors.New("invalid private CA PEM")
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, Proxy: nil, MaxIdleConns: 2, IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil {
			return nil, errors.New("router DNS resolution failed")
		}
		if len(ips) == 0 {
			return nil, errors.New("router DNS returned no addresses")
		}
		for _, ip := range ips {
			if e := ValidateDestination(ip, allowed); e != nil {
				return nil, e
			}
		}
		// Dial the validated numeric address, preventing DNS rebinding between check and use.
		var last error
		for _, ip := range ips {
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}
	return &Client{http: &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("RouterOS redirects are forbidden") }}, base: "https://" + net.JoinHostPort(r.Host, strconv.Itoa(r.Port)) + "/rest/", user: r.Username, password: r.Password}, nil
}

// NewTestClient is deliberately not exported: tests in this package can attach a verified mock transport.
func (c *Client) Call(ctx context.Context, method, path string, body any) ([]Record, error) {
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return nil, errors.New("invalid REST resource")
	}
	var reader io.Reader
	if body != nil {
		data, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		reader = bytes.NewReader(data)
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.http.Do(req)
	if e != nil {
		if errors.Is(e, context.DeadlineExceeded) {
			return nil, errors.New("router request timed out; mutation outcome may be uncertain")
		}
		return nil, errors.New("router transport failed; check DNS, connectivity and verified TLS")
	}
	defer resp.Body.Close()
	data, e := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if e != nil {
		return nil, errors.New("router response interrupted")
	}
	if len(data) > 4*1024*1024 {
		return nil, errors.New("router response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{resp.StatusCode}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var items []Record
	if e = json.Unmarshal(data, &items); e == nil {
		return items, nil
	}
	var item Record
	if e = json.Unmarshal(data, &item); e == nil {
		return []Record{item}, nil
	}
	return nil, errors.New("unexpected RouterOS JSON response")
}
func List(ctx context.Context, c API, path string) ([]Record, error) {
	return c.Call(ctx, http.MethodGet, path, nil)
}
func Find(ctx context.Context, c API, path, key, value string) (Record, error) {
	items, e := List(ctx, c, path+"?"+url.Values{key: []string{value}}.Encode())
	if e != nil {
		return nil, e
	}
	if len(items) != 1 {
		return nil, errors.New("expected exactly one router resource")
	}
	return items[0], nil
}
func Version(s string) (major, minor, patch int, err error) {
	text := strings.Fields(s)
	if len(text) == 0 {
		return 0, 0, 0, errors.New("missing RouterOS version")
	}
	v := text[0]
	if strings.ContainsAny(v, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-") {
		return 0, 0, 0, errors.New("prerelease or unknown RouterOS version is not approved")
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, errors.New("invalid RouterOS version")
	}
	nums := []int{0, 0, 0}
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e != nil || n < 0 {
			return 0, 0, 0, errors.New("invalid RouterOS version")
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nil
}
func Discover(ctx context.Context, c API) (domain.Snapshot, error) {
	s := domain.Snapshot{Capabilities: map[string]bool{}}
	r, e := List(ctx, c, "system/resource")
	if e != nil {
		return s, e
	}
	if len(r) != 1 {
		return s, errors.New("missing system resource record")
	}
	s.Resources = r[0]
	s.Version = r[0]["version"]
	s.Architecture = r[0]["architecture-name"]
	s.Model = r[0]["board-name"]
	major, minor, _, e := Version(s.Version)
	baseline := e == nil && major == 7 && minor >= 17
	paths := []string{"interface/ovpn-server/server", "certificate", "ip/pool", "ppp/profile", "ppp/secret", "ip/firewall/filter", "ip/address", "ip/route"}
	observed := map[string][]Record{}
	for _, p := range paths {
		items, e := List(ctx, c, p)
		s.Capabilities[p] = e == nil
		if e != nil {
			s.Warnings = append(s.Warnings, "Unable to observe "+p+"; permissions or capability missing")
			continue
		}
		for _, item := range items {
			delete(item, "password")
			delete(item, "private-key")
		}
		observed[p] = items
	}
	s.Capabilities["openvpn_baseline"] = baseline && s.Capabilities[paths[0]] && s.Capabilities["certificate"]
	if !baseline {
		s.Warnings = append(s.Warnings, "Only stable RouterOS 7.17+ within major version 7 is eligible; lab interoperability remains unverified")
	}
	// The hash excludes telemetry and secrets; a change is a review signal, never an automatic overwrite.
	data, _ := json.Marshal(observed)
	sum := sha256.Sum256(data)
	s.ConfigHash = hex.EncodeToString(sum[:])
	return s, nil
}
func ReadFile(ctx context.Context, c API, name string) ([]byte, error) {
	f, e := Find(ctx, c, "file", "name", name)
	if e != nil {
		return nil, e
	}
	size, e := strconv.Atoi(f["size"])
	if e != nil || size <= 0 || size > 1024*1024 {
		return nil, errors.New("invalid exported file size")
	}
	var out []byte
	for offset := 0; offset < size; {
		length := min(32768, size-offset)
		rows, e := c.Call(ctx, "POST", "file/read", Record{"file": name, "offset": strconv.Itoa(offset), "chunk-size": strconv.Itoa(length)})
		if e != nil {
			return nil, e
		}
		if len(rows) != 1 || len(rows[0]["data"]) != length {
			return nil, errors.New("partial certificate file read; refusing incomplete material")
		}
		out = append(out, []byte(rows[0]["data"])...)
		offset += length
	}
	return out, nil
}
