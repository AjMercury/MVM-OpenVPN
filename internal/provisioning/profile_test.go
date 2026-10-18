package provisioning

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/youmark/pkcs8"
	"math/big"
	"mikrotik-vpn-manager/internal/domain"
	"strings"
	"testing"
	"time"
)

func testMaterial(t *testing.T) Material {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	ca, e = x509.ParseCertificate(caDER)
	if e != nil {
		t.Fatal(e)
	}
	client := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "alice"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	certDER, e := x509.CreateCertificate(rand.Reader, client, ca, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	block, e := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), []byte("separate passphrase"), x509.PEMCipherAES256)
	if e != nil {
		t.Fatal(e)
	}
	return Material{CA: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), Key: pem.EncodeToMemory(block), Passphrase: "separate passphrase"}
}

func TestEncryptedPKCS8AndKeyAssociation(t *testing.T) {
	m := testMaterial(t)
	block, _ := pem.Decode(m.Key)
	der, e := x509.DecryptPEMBlock(block, []byte(m.Passphrase))
	if e != nil {
		t.Fatal(e)
	}
	key, e := x509.ParsePKCS1PrivateKey(der)
	if e != nil {
		t.Fatal(e)
	}
	encrypted, e := pkcs8.MarshalPrivateKey(key, []byte(m.Passphrase), nil)
	if e != nil {
		t.Fatal(e)
	}
	m.Key = pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: encrypted})
	if e = ValidateMaterial(m, time.Now()); e != nil {
		t.Fatal(e)
	}
	m.Passphrase = "incorrect passphrase"
	if e = ValidateMaterial(m, time.Now()); e == nil {
		t.Fatal("wrong key passphrase accepted")
	}
	m = testMaterial(t)
	other := testMaterial(t)
	m.Key = other.Key
	if e = ValidateMaterial(m, time.Now()); e == nil {
		t.Fatal("mismatched private key accepted")
	}
	if e = boundedPKCS8([]byte("not DER")); e == nil {
		t.Fatal("invalid DER accepted")
	}
}
func TestGenerateProfile(t *testing.T) {
	m := testMaterial(t)
	s := domain.Server{Spec: domain.ServerSpec{Name: "office", Endpoint: "vpn.example.com", Port: 1194, Transport: "tcp", Subnet: "10.8.0.0/24", Start: "10.8.0.10", End: "10.8.0.200", Gateway: "10.8.0.1", DNS: []string{"10.0.0.53"}, Allowed: []string{"10.0.0.0/24"}, InputAnchor: "*A", ForwardAnchor: "*B"}}
	b, e := Generate(s, m)
	if e != nil {
		t.Fatal(e)
	}
	for _, expected := range []string{"proto tcp-client", "verify-x509-name vpn.example.com name", "remote-cert-tls server", "route 10.0.0.0 255.255.255.0", "<ca>", "<cert>", "<key>", "auth-user-pass", "route-nopull"} {
		if !bytes.Contains(b, []byte(expected)) {
			t.Fatal("missing", expected)
		}
	}
	if strings.Contains(string(b), "redirect-gateway") || strings.Contains(string(b), "separate passphrase") {
		t.Fatal("unsafe directive or embedded password")
	}
	s.Spec.Endpoint = "host\nscript-security 2"
	if _, e = Generate(s, m); e == nil {
		t.Fatal("config injection accepted")
	}
	m.Key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("plaintext")})
	if e = ValidateMaterial(m, time.Now()); e == nil {
		t.Fatal("unencrypted key accepted")
	}
}
func TestCertificateExpiry(t *testing.T) {
	m := testMaterial(t)
	if e := ValidateMaterial(m, time.Now().Add(48*time.Hour)); e == nil {
		t.Fatal("expired cert accepted")
	}
	m.Certificate = []byte("broken PEM")
	if e := ValidateMaterial(m, time.Now()); e == nil {
		t.Fatal("invalid certificate accepted")
	}
}
