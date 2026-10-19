package provisioning

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	qrcode "github.com/skip2/go-qrcode"
	"mikrotik-vpn-manager/internal/domain"
	"net"
	"strconv"
	"strings"
)

func WireGuardKeypair() (private, public string, err error) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(key.Bytes()), base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()), nil
}

func WireGuardProfile(s domain.Server, u domain.VPNUser, private, serverPublic string) ([]byte, error) {
	decode := func(raw string) ([]byte, error) {
		b, e := base64.StdEncoding.DecodeString(raw)
		if e != nil || len(b) != 32 {
			return nil, errors.New("invalid WireGuard key")
		}
		return b, nil
	}
	keyBytes, e := decode(private)
	if e != nil {
		return nil, e
	}
	key, e := ecdh.X25519().NewPrivateKey(keyBytes)
	if e != nil {
		return nil, e
	}
	if base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()) != u.PublicKey {
		return nil, errors.New("WireGuard client key does not match enrolled peer")
	}
	if _, e = decode(serverPublic); e != nil {
		return nil, e
	}
	if strings.ContainsAny(s.Spec.Endpoint+u.Address+strings.Join(s.Spec.DNS, "")+strings.Join(s.Spec.Allowed, ""), "\r\n") {
		return nil, errors.New("invalid WireGuard profile values")
	}
	allowed := append([]string{}, s.Spec.Allowed...)
	return []byte(fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = %s\n\n[Peer]\nPublicKey = %s\nAllowedIPs = %s\nEndpoint = %s\nPersistentKeepalive = 25\n", private, u.Address, strings.Join(s.Spec.DNS, ", "), serverPublic, strings.Join(allowed, ", "), net.JoinHostPort(s.Spec.Endpoint, strconv.Itoa(s.Spec.Port)))), nil
}

// WireGuardQR encodes the native configuration, so WireGuard can import it directly.
func WireGuardQR(profile []byte) ([]byte, error) {
	if len(profile) > 4096 || !strings.HasPrefix(string(profile), "[Interface]\n") {
		return nil, errors.New("invalid or oversized WireGuard configuration")
	}
	return qrcode.Encode(string(profile), qrcode.Medium, 512)
}
