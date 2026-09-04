package pki

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestCAIssueAndVerifyServerCert(t *testing.T) {
	ca, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	certDER, keyDER, err := ca.IssueServerCert([]string{"127.0.0.1", "localhost"}, "")
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("verify chain: %v", err)
	}

	if _, err := x509.ParsePKCS8PrivateKey(keyDER); err != nil {
		t.Fatalf("parse key: %v", err)
	}
}

func TestCAIssueClientCertFromCSR(t *testing.T) {
	ca, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	_, csrPEM, err := GenerateKeyAndCSR("wkr_test123")
	if err != nil {
		t.Fatalf("GenerateKeyAndCSR: %v", err)
	}

	csr, err := ParseCSR(csrPEM)
	if err != nil {
		t.Fatalf("ParseCSR: %v", err)
	}

	certDER, err := ca.IssueClientCertFromCSR(csr, csr.Subject.CommonName)
	if err != nil {
		t.Fatalf("IssueClientCertFromCSR: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if cert.Subject.CommonName != "wkr_test123" {
		t.Errorf("CN = %q, want wkr_test123", cert.Subject.CommonName)
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("verify chain: %v", err)
	}
}

func TestCAIssueClientCertFromCSR_InvalidSignature(t *testing.T) {
	ca, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	_, csrPEM, err := GenerateKeyAndCSR("wkr_a")
	if err != nil {
		t.Fatalf("GenerateKeyAndCSR: %v", err)
	}
	csrA, err := ParseCSR(csrPEM)
	if err != nil {
		t.Fatalf("ParseCSR: %v", err)
	}

	_, csrPEM2, err := GenerateKeyAndCSR("wkr_b")
	if err != nil {
		t.Fatalf("GenerateKeyAndCSR: %v", err)
	}
	csrB, err := ParseCSR(csrPEM2)
	if err != nil {
		t.Fatalf("ParseCSR: %v", err)
	}

	// Swap in another CSR's signature so csrA's declared public key no
	// longer matches what actually signed it — CheckSignature must catch
	// this rather than trusting an unverified self-assertion.
	csrA.Signature = csrB.Signature
	if _, err := ca.IssueClientCertFromCSR(csrA, csrA.Subject.CommonName); err == nil {
		t.Fatal("expected signature verification to fail, got nil error")
	}
}

func TestVerifyRejectsWrongCA(t *testing.T) {
	ca1, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	ca2, err := GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	certDER, _, err := ca1.IssueServerCert([]string{"127.0.0.1"}, "")
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca2.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err == nil {
		t.Fatal("expected verification against the wrong CA to fail")
	}
}

func TestNeedsRenewal(t *testing.T) {
	now := time.Now()

	fresh := &x509.Certificate{NotAfter: now.Add(ServerValidity)}
	if NeedsRenewal(fresh) {
		t.Error("fresh cert should not need renewal")
	}

	expiringSoon := &x509.Certificate{NotAfter: now.Add(RenewalWindow / 2)}
	if !NeedsRenewal(expiringSoon) {
		t.Error("cert within the renewal window should need renewal")
	}

	expired := &x509.Certificate{NotAfter: now.Add(-time.Hour)}
	if !NeedsRenewal(expired) {
		t.Error("expired cert should need renewal")
	}
}
