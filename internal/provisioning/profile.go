package provisioning

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/youmark/pkcs8"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/vpn/openvpn"
	"net/netip"
	"strings"
	"time"
)

type Material struct {
	CA, Certificate, Key []byte
	Passphrase           string
}

func parseCert(raw []byte) (*x509.Certificate, error) {
	b, rest := pem.Decode(raw)
	if b == nil || b.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) > 0 {
		return nil, errors.New("expected one PEM certificate")
	}
	return x509.ParseCertificate(b.Bytes)
}
func ValidateMaterial(m Material, now time.Time) error {
	ca, e := parseCert(m.CA)
	if e != nil {
		return e
	}
	cert, e := parseCert(m.Certificate)
	if e != nil {
		return e
	}
	if !ca.IsCA || now.Before(ca.NotBefore) || now.After(ca.NotAfter) {
		return errors.New("invalid or expired CA")
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, e := cert.Verify(x509.VerifyOptions{Roots: pool, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); e != nil {
		return errors.New("client certificate validation failed")
	}
	b, rest := pem.Decode(m.Key)
	if b == nil || len(bytes.TrimSpace(rest)) > 0 || b.Type != "ENCRYPTED PRIVATE KEY" && !(b.Type == "RSA PRIVATE KEY" && x509.IsEncryptedPEMBlock(b)) {
		return errors.New("an encrypted client private key is required")
	}
	var private any
	if b.Type == "ENCRYPTED PRIVATE KEY" {
		if e = boundedPKCS8(b.Bytes); e != nil {
			return e
		}
		private, e = pkcs8.ParsePKCS8PrivateKey(b.Bytes, []byte(m.Passphrase))
	} else {
		var der []byte
		der, e = x509.DecryptPEMBlock(b, []byte(m.Passphrase))
		if e == nil {
			private, e = x509.ParsePKCS1PrivateKey(der)
		}
		for i := range der {
			der[i] = 0
		}
	}
	if e != nil {
		return errors.New("client private key decryption failed")
	}
	signer, ok := private.(crypto.Signer)
	if !ok {
		return errors.New("unsupported client key type")
	}
	public, e := x509.MarshalPKIXPublicKey(signer.Public())
	if e != nil {
		return errors.New("invalid client public key")
	}
	if !bytes.Equal(public, cert.RawSubjectPublicKeyInfo) {
		return errors.New("client certificate and private key do not match")
	}
	return nil
}
func Generate(s domain.Server, m Material) ([]byte, error) {
	if e := openvpn.Validate(s.Spec); e != nil {
		return nil, e
	}
	if e := ValidateMaterial(m, time.Now()); e != nil {
		return nil, e
	}
	proto := "tcp-client"
	if s.Spec.Transport == "udp" {
		proto = "udp"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "client\ndev tun\nproto %s\nremote %s %d\nresolv-retry infinite\nnobind\npersist-key\npersist-tun\nauth-user-pass\nauth-nocache\nremote-cert-tls server\nverify-x509-name %s name\ntls-version-min 1.2\ncipher AES-256-CBC\ndata-ciphers AES-256-CBC\nauth SHA256\nroute-nopull\nverb 3\n", proto, s.Spec.Endpoint, s.Spec.Port, s.Spec.Endpoint)
	for _, raw := range s.Spec.Allowed {
		p, _ := netip.ParsePrefix(raw)
		mask := uint32(0xffffffff) << uint(32-p.Bits())
		fmt.Fprintf(&b, "route %s %d.%d.%d.%d\n", p.Addr(), byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask))
	}
	for _, dns := range s.Spec.DNS {
		fmt.Fprintf(&b, "dhcp-option DNS %s\n", dns)
	}
	for _, part := range []struct {
		name string
		data []byte
	}{{"ca", m.CA}, {"cert", m.Certificate}, {"key", m.Key}} {
		fmt.Fprintf(&b, "<%s>\n%s\n</%s>\n", part.name, bytes.TrimSpace(part.data), part.name)
	}
	return []byte(b.String()), nil
}
func Bundle(profile []byte, m Material) ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, f := range []struct {
		name string
		data []byte
	}{{"client.ovpn", profile}, {"ca.crt", m.CA}, {"client.crt", m.Certificate}, {"client.key", m.Key}} {
		w, e := z.Create(f.name)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(f.data); e != nil {
			return nil, e
		}
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}
