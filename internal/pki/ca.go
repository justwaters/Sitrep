package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

// Validity periods, per the design's cert-rotation policy: long-dated
// certs keep manual re-enrollment (the documented escape hatch for expired
// worker certs) rare in practice without needing an automated renewal
// protocol between long-lived daemons.
const (
	CAValidity     = 10 * 365 * 24 * time.Hour
	ServerValidity = 2 * 365 * 24 * time.Hour
	ClientValidity = 365 * 24 * time.Hour

	// RenewalWindow: the manager auto-regenerates its server cert on
	// startup if less than this much validity remains.
	RenewalWindow = 30 * 24 * time.Hour
)

// CA is Sitrep's self-signed root certificate authority. The manager
// generates one on first startup and uses it to issue its own server cert
// and a client cert for every enrolled worker.
type CA struct {
	Cert    *x509.Certificate
	CertPEM []byte
	Key     *ecdsa.PrivateKey
}

// GenerateCA creates a new self-signed ECDSA P-256 root CA.
func GenerateCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := newSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Sitrep Root CA", Organization: []string{"Sitrep"}},
		NotBefore:             now.Add(-5 * time.Minute), // tolerate minor clock skew
		NotAfter:              now.Add(CAValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse generated CA certificate: %w", err)
	}

	return &CA{Cert: cert, CertPEM: certToPEM(der), Key: key}, nil
}

// Save writes the CA's certificate and private key to disk.
func (ca *CA) Save(certPath, keyPath string) error {
	if err := WriteCertPEM(certPath, ca.Cert.Raw); err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(ca.Key)
	if err != nil {
		return fmt.Errorf("marshal CA key: %w", err)
	}
	return WriteKeyPEM(keyPath, keyDER)
}

// LoadCA reads an existing CA cert+key pair from disk.
func LoadCA(certPath, keyPath string) (*CA, error) {
	cert, certPEM, err := ReadCertPEM(certPath)
	if err != nil {
		return nil, err
	}
	key, err := ReadKeyPEM(keyPath)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, CertPEM: certPEM, Key: key}, nil
}

// LoadOrCreateCA loads the CA at certPath/keyPath if present, or generates
// and persists a new one if not. This is what the manager calls on every
// startup.
func LoadOrCreateCA(certPath, keyPath string) (*CA, error) {
	ca, err := LoadCA(certPath, keyPath)
	if err == nil {
		return ca, nil
	}
	ca, err = GenerateCA()
	if err != nil {
		return nil, err
	}
	if err := ca.Save(certPath, keyPath); err != nil {
		return nil, fmt.Errorf("save new CA: %w", err)
	}
	return ca, nil
}

// IssueServerCert generates a fresh ECDSA key and issues a server
// certificate for it, signed by ca, valid for the given hosts (DNS names
// and/or IP addresses).
func (ca *CA) IssueServerCert(hosts []string) (certDER []byte, keyDER []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate server key: %w", err)
	}

	serial, err := newSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "sitrep-manager"},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(ServerValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("create server certificate: %w", err)
	}
	keyDER, err = x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal server key: %w", err)
	}
	return der, keyDER, nil
}

// IssueClientCertFromCSR verifies csrPEM's self-signature and issues a
// client certificate binding its public key to commonName (the worker's
// WorkerID), signed by ca. The worker's private key is never seen by the
// manager — only the public key embedded in the CSR.
func (ca *CA) IssueClientCertFromCSR(csr *x509.CertificateRequest, commonName string) (certDER []byte, err error) {
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("invalid CSR signature: %w", err)
	}

	serial, err := newSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(ClientValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, csr.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("create client certificate: %w", err)
	}
	return der, nil
}

// NeedsRenewal reports whether cert has less than RenewalWindow validity
// remaining (or is already expired).
func NeedsRenewal(cert *x509.Certificate) bool {
	return time.Until(cert.NotAfter) < RenewalWindow
}

func newSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}
	return serial, nil
}
