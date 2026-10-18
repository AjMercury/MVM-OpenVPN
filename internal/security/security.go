package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func Digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func HashPassword(password string) (string, error) {
	if len(password) < 14 || len(password) > 233 {
		return "", errors.New("password must contain 14–233 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 3, 32*1024, 2, 32)
	return fmt.Sprintf("argon2id$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func CheckPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "argon2id" || len(password) > 233 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[1])
	if e != nil || len(salt) != 16 {
		return false
	}
	expected, e := base64.RawStdEncoding.DecodeString(parts[2])
	if e != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 32*1024, 2, 32)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

type Vault struct{ aead cipher.AEAD }

func NewVault(key []byte) (*Vault, error) {
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(b)
	return &Vault{aead: a}, e
}
func (v *Vault) Seal(data []byte, context string) ([]byte, error) {
	n := make([]byte, v.aead.NonceSize())
	if _, e := rand.Read(n); e != nil {
		return nil, e
	}
	return v.aead.Seal(n, n, data, []byte(context)), nil
}
func (v *Vault) Open(data []byte, context string) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(data) < n {
		return nil, errors.New("invalid encrypted record")
	}
	return v.aead.Open(nil, data[:n], data[n:], []byte(context))
}
