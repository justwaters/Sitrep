package pki

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadCertPEM(t *testing.T) {
	ca, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt.pem")
	if err := WriteCertPEM(certPath, ca.Cert.Raw); err != nil {
		t.Fatalf("WriteCertPEM: %v", err)
	}

	cert, pemBytes, err := ReadCertPEM(certPath)
	if err != nil {
		t.Fatalf("ReadCertPEM: %v", err)
	}
	if cert.SerialNumber.Cmp(ca.Cert.SerialNumber) != 0 {
		t.Error("serial number mismatch after round trip")
	}
	if len(pemBytes) == 0 {
		t.Error("expected non-empty PEM bytes")
	}

	info, err := os.Stat(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != certFilePerm {
		t.Errorf("perm = %v, want %v", info.Mode().Perm(), certFilePerm)
	}
}

func TestWriteReadKeyPEM(t *testing.T) {
	ca, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key.pem")
	keyDER, err := x509.MarshalPKCS8PrivateKey(ca.Key)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteKeyPEM(keyPath, keyDER); err != nil {
		t.Fatalf("WriteKeyPEM: %v", err)
	}

	key, err := ReadKeyPEM(keyPath)
	if err != nil {
		t.Fatalf("ReadKeyPEM: %v", err)
	}
	if !key.Equal(ca.Key) {
		t.Error("round-tripped key does not match original")
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != keyFilePerm {
		t.Errorf("perm = %v, want %v", info.Mode().Perm(), keyFilePerm)
	}
}

func TestLoadOrCreateCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt.pem")
	keyPath := filepath.Join(dir, "ca.key.pem")

	ca1, err := LoadOrCreateCA(certPath, keyPath, "test-manager")
	if err != nil {
		t.Fatalf("LoadOrCreateCA (create): %v", err)
	}

	ca2, err := LoadOrCreateCA(certPath, keyPath, "test-manager")
	if err != nil {
		t.Fatalf("LoadOrCreateCA (load): %v", err)
	}

	if ca1.Cert.SerialNumber.Cmp(ca2.Cert.SerialNumber) != 0 {
		t.Error("second call should load the existing CA, not generate a new one")
	}
}

func TestLoadCertPool(t *testing.T) {
	ca, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt.pem")
	if err := WriteCertPEM(certPath, ca.Cert.Raw); err != nil {
		t.Fatalf("WriteCertPEM: %v", err)
	}

	pool, err := LoadCertPool(certPath)
	if err != nil {
		t.Fatalf("LoadCertPool: %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}
