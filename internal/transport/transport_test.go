package transport_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/justwaters/sitrep/internal/pki"
	"github.com/justwaters/sitrep/internal/transport"
)

func issueServerCert(t *testing.T, ca *pki.CA) tls.Certificate {
	t.Helper()
	certDER, keyDER, err := ca.IssueServerCert([]string{"127.0.0.1"}, "")
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	cert, err := tls.X509KeyPair(pki.EncodeCertPEM(certDER), pki.EncodeKeyPEM(keyDER))
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	return cert
}

func shutdownSoon(t *testing.T, srv *transport.Server) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
}

func TestEnrollServer_NoClientCertRequired(t *testing.T) {
	ca, err := pki.GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverCert := issueServerCert(t, ca)

	srv, err := transport.NewEnrollServer("127.0.0.1:0", serverCert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("NewEnrollServer: %v", err)
	}
	shutdownSoon(t, srv)
	go srv.Serve()

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   5 * time.Second,
	}
	resp, err := client.Get("https://" + srv.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET without client cert should succeed on the enrollment listener: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReportServer_RequiresClientCert(t *testing.T) {
	ca, err := pki.GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverCert := issueServerCert(t, ca)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca.Cert)

	srv, err := transport.NewReportServer("127.0.0.1:0", serverCert, clientCAs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("NewReportServer: %v", err)
	}
	shutdownSoon(t, srv)
	go srv.Serve()

	noCertClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   5 * time.Second,
	}
	if _, err := noCertClient.Get("https://" + srv.Addr().String() + "/"); err == nil {
		t.Error("expected the TLS handshake to fail without a client certificate")
	}

	keyDER, csrPEM, err := pki.GenerateKeyAndCSR("wkr_test")
	if err != nil {
		t.Fatalf("GenerateKeyAndCSR: %v", err)
	}
	csr, err := pki.ParseCSR(csrPEM)
	if err != nil {
		t.Fatalf("ParseCSR: %v", err)
	}
	clientCertDER, err := ca.IssueClientCertFromCSR(csr, csr.Subject.CommonName)
	if err != nil {
		t.Fatalf("IssueClientCertFromCSR: %v", err)
	}
	clientCert, err := tls.X509KeyPair(pki.EncodeCertPEM(clientCertDER), pki.EncodeKeyPEM(keyDER))
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}

	withCertClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{clientCert},
			InsecureSkipVerify: true,
		}},
		Timeout: 5 * time.Second,
	}
	resp, err := withCertClient.Get("https://" + srv.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET with a validly CA-signed client cert should succeed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestEnrollClient_FingerprintPinning(t *testing.T) {
	ca, err := pki.GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverCert := issueServerCert(t, ca)
	fingerprint := transport.Fingerprint(serverCert.Certificate[0])

	var gotReq transport.EnrollRequest
	srv, err := transport.NewEnrollServer("127.0.0.1:0", serverCert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode(transport.EnrollResponse{WorkerID: "wkr_x", CACertPEM: ca.CertPEM})
	}))
	if err != nil {
		t.Fatalf("NewEnrollServer: %v", err)
	}
	shutdownSoon(t, srv)
	go srv.Serve()

	client := transport.NewEnrollClient(fingerprint)
	resp, err := transport.Enroll(context.Background(), client, srv.Addr().String(), transport.EnrollRequest{
		Token: "tok", Name: "h", CSRPEM: []byte("csr"),
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if resp.WorkerID != "wkr_x" {
		t.Errorf("WorkerID = %q, want wkr_x", resp.WorkerID)
	}
	if gotReq.Token != "tok" {
		t.Error("server did not receive the expected token")
	}
}

func TestEnrollClient_WrongFingerprintRejected(t *testing.T) {
	ca, err := pki.GenerateCA("")
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverCert := issueServerCert(t, ca)

	srv, err := transport.NewEnrollServer("127.0.0.1:0", serverCert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("NewEnrollServer: %v", err)
	}
	shutdownSoon(t, srv)
	go srv.Serve()

	client := transport.NewEnrollClient("0000000000000000000000000000000000000000000000000000000000000000")
	if _, err := transport.Enroll(context.Background(), client, srv.Addr().String(), transport.EnrollRequest{}); err == nil {
		t.Error("expected enrollment to fail against a certificate that doesn't match the pinned fingerprint")
	}
}
