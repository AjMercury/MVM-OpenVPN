package provisioning

import (
	"bytes"
	"image/png"
	"net/url"
	"strings"
	"testing"
)

func TestDownloadQRPayload(t *testing.T) {
	token := strings.Repeat("A", 43)
	link, data, e := DownloadQR("https://admin.example.com", token)
	if e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(link)
	if e != nil || u.Fragment != token || u.Query().Get("page") != "delivery" || strings.Contains(u.RawQuery, token) {
		t.Fatal("unsafe QR payload")
	}
	image, e := png.Decode(bytes.NewReader(data))
	if e != nil || image.Bounds().Dx() != 320 {
		t.Fatal("invalid local QR image")
	}
	if _, _, e = DownloadQR("https://admin.example.com/path", token); e == nil {
		t.Fatal("invalid origin accepted")
	}
}
