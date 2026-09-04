package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
)

// GenerateKeyAndCSR generates a fresh ECDSA P-256 key pair and a PEM-encoded
// certificate signing request for commonName (the worker's WorkerID). The
// private key never leaves the caller — only the CSR (containing the
// public key) is meant to be sent to the manager.
func GenerateKeyAndCSR(commonName string) (keyDER []byte, csrPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate worker key: %w", err)
	}

	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: commonName},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create CSR: %w", err)
	}

	keyDER, err = x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal worker key: %w", err)
	}

	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	return keyDER, csrPEM, nil
}

// ParseCSR decodes and parses a PEM-encoded certificate signing request.
func ParseCSR(csrPEM []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("not a PEM certificate request")
	}
	return x509.ParseCertificateRequest(block.Bytes)
}

// EncodeCertPEM PEM-encodes a DER-encoded certificate, for embedding
// directly in wire messages like transport.EnrollResponse.
func EncodeCertPEM(der []byte) []byte {
	return certToPEM(der)
}

// EncodeKeyPEM PEM-encodes a DER-encoded PKCS#8 private key.
func EncodeKeyPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func certToPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
