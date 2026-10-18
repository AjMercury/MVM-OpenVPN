package provisioning

import (
	"errors"
	qrcode "github.com/skip2/go-qrcode"
	"net/url"
	"regexp"
)

var deliveryToken = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// DownloadQR is intentionally a URL mode, never native OpenVPN configuration.
func DownloadQR(origin, token string) (string, []byte, error) {
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" && u.Scheme != "http" || !deliveryToken.MatchString(token) {
		return "", nil, errors.New("invalid download QR origin or token")
	}
	u.Path = "/"
	u.RawQuery = "page=delivery"
	u.Fragment = token
	link := u.String()
	png, e := qrcode.Encode(link, qrcode.Medium, 320)
	return link, png, e
}
