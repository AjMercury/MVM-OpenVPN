package provisioning

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
)

// Bound attacker-controlled KDF work before passing encrypted DER to the parser.
func boundedPKCS8(der []byte) error {
	if len(der) > 65536 {
		return errors.New("encrypted key exceeds size limit")
	}
	var encrypted struct {
		Algorithm pkix.AlgorithmIdentifier
		Data      []byte
	}
	rest, e := asn1.Unmarshal(der, &encrypted)
	if e != nil || len(rest) > 0 || !encrypted.Algorithm.Algorithm.Equal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}) {
		return errors.New("only bounded PBES2 encrypted keys are accepted")
	}
	var pbes struct {
		KDF    pkix.AlgorithmIdentifier
		Cipher pkix.AlgorithmIdentifier
	}
	if _, e = asn1.Unmarshal(encrypted.Algorithm.Parameters.FullBytes, &pbes); e != nil {
		return errors.New("invalid PBES2 key parameters")
	}
	switch pbes.KDF.Algorithm.String() {
	case "1.2.840.113549.1.5.12":
		var kdf struct {
			Salt       []byte
			Iterations int
			KeyLength  int                      `asn1:"optional"`
			PRF        pkix.AlgorithmIdentifier `asn1:"optional"`
		}
		if _, e = asn1.Unmarshal(pbes.KDF.Parameters.FullBytes, &kdf); e != nil || len(kdf.Salt) < 8 || len(kdf.Salt) > 128 || kdf.Iterations < 1 || kdf.Iterations > 1000000 || kdf.KeyLength < 0 || kdf.KeyLength > 64 {
			return errors.New("PBKDF2 work parameters exceed policy")
		}
	case "1.3.6.1.4.1.11591.4.11":
		var kdf struct {
			Salt      []byte
			N, R, P   int
			KeyLength int `asn1:"optional"`
		}
		if _, e = asn1.Unmarshal(pbes.KDF.Parameters.FullBytes, &kdf); e != nil || len(kdf.Salt) < 8 || len(kdf.Salt) > 128 || kdf.N < 2 || kdf.N > 32768 || kdf.N&(kdf.N-1) != 0 || kdf.R < 1 || kdf.R > 8 || kdf.P < 1 || kdf.P > 2 || kdf.N*kdf.R*kdf.P > 262144 || kdf.KeyLength < 0 || kdf.KeyLength > 64 {
			return errors.New("scrypt work parameters exceed policy")
		}
	default:
		return errors.New("unsupported encrypted-key KDF")
	}
	return nil
}
