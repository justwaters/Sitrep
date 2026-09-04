// Package pki implements Sitrep's self-contained certificate authority:
// the manager generates its own root CA on first startup and uses it to
// issue its own server certificate plus a client certificate for each
// enrolled worker. All key material is ECDSA P-256 (fast keygen, small
// certs — no keygen delay in interactive wizards).
package pki

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// keyFilePerm/certFilePerm match the design doc: private keys are
// root/service-account-only (0600), certs are world-readable (0644) since
// they contain no secret material.
const (
	keyFilePerm  os.FileMode = 0o600
	certFilePerm os.FileMode = 0o644
	dirPerm      os.FileMode = 0o700
)

func writePEMFile(path, blockType string, der []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	block := &pem.Block{Type: blockType, Bytes: der}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, block); err != nil {
		return fmt.Errorf("encode PEM to %s: %w", path, err)
	}
	return nil
}

// WriteCertPEM writes a DER-encoded certificate to path as PEM, 0644.
func WriteCertPEM(path string, der []byte) error {
	return writePEMFile(path, "CERTIFICATE", der, certFilePerm)
}

// WriteRawPEM writes already PEM-encoded bytes (e.g. a cert received over
// the wire in transport.EnrollResponse) to path verbatim, 0644.
func WriteRawPEM(path string, pemBytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	return os.WriteFile(path, pemBytes, certFilePerm)
}

// WriteKeyPEM writes a DER-encoded (PKCS#8) private key to path as PEM, 0600.
func WriteKeyPEM(path string, der []byte) error {
	return writePEMFile(path, "PRIVATE KEY", der, keyFilePerm)
}

// ReadCertPEM reads and parses a single PEM-encoded certificate from path,
// returning both the parsed certificate and the raw PEM bytes (the latter
// useful for embedding directly in wire messages like EnrollResponse).
func ReadCertPEM(path string) (*x509.Certificate, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("%s: not a PEM certificate", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate %s: %w", path, err)
	}
	return cert, data, nil
}

// ReadKeyPEM reads and parses a single PEM-encoded PKCS#8 ECDSA private
// key from path.
func ReadKeyPEM(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("%s: not a PEM private key", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s: %w", path, err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ECDSA")
	}
	return ecKey, nil
}

// LoadTLSCertificate loads a cert+key pair from disk as a tls.Certificate
// suitable for tls.Config.Certificates.
func LoadTLSCertificate(certPath, keyPath string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(certPath, keyPath)
}

// LoadCertPool builds an x509.CertPool containing a single CA certificate
// read from caCertPath.
func LoadCertPool(caCertPath string) (*x509.CertPool, error) {
	data, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("%s: no valid certificates found", caCertPath)
	}
	return pool, nil
}
