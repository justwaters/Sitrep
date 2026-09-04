package transport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const requestTimeout = 15 * time.Second

// Fingerprint is the SHA-256 digest of a DER-encoded certificate,
// formatted as lowercase hex. It's what `sitrep manager token create`
// prints for the operator to hand to the worker wizard, and what the
// worker's enrollment dial pins against — establishing trust in the
// manager's server certificate for that one bootstrap connection without
// requiring the worker to already possess the manager's CA certificate.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// NewEnrollClient builds an HTTP client for the one-shot enrollment
// connection. It trusts exactly one certificate: the one whose SHA-256
// fingerprint matches wantFingerprint. This is a TOFU/pinning verification
// (like SSH host key pinning) rather than full chain validation, which is
// appropriate here since the fingerprint itself was already communicated
// out-of-band alongside the enrollment token.
func NewEnrollClient(wantFingerprint string) *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // verified manually below via pinned fingerprint
				VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					for _, raw := range rawCerts {
						if Fingerprint(raw) == wantFingerprint {
							return nil
						}
					}
					return fmt.Errorf("manager certificate does not match pinned fingerprint %s", wantFingerprint)
				},
			},
		},
	}
}

// Enroll POSTs req to the manager's /enroll endpoint and returns the
// parsed response. managerAddr is host:port.
func Enroll(ctx context.Context, client *http.Client, managerAddr string, req EnrollRequest) (*EnrollResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal enroll request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+managerAddr+"/enroll", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("enroll request to %s: %w", managerAddr, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read enroll response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enroll rejected (%s): %s", resp.Status, bytes.TrimSpace(respBody))
	}

	var out EnrollResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("parse enroll response: %w", err)
	}
	return &out, nil
}

// NewReportClient builds an HTTP client for the worker's ongoing report
// traffic: full mutual TLS, presenting the worker's own client
// certificate and verifying the manager's server certificate against the
// manager's CA pool (not a pinned leaf fingerprint), so it keeps working
// across the manager's own server-cert auto-renewal.
func NewReportClient(clientCert tls.Certificate, managerCAs *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      managerCAs,
				MinVersion:   tls.VersionTLS12,
			},
		},
	}
}

// SendReport POSTs report to the manager's /report endpoint and returns
// the parsed acknowledgement (which carries the manager's current
// interval/enabled-checks for the worker to self-adjust to).
func SendReport(ctx context.Context, client *http.Client, managerAddr string, report Report) (*ReportAck, error) {
	body, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+managerAddr+"/report", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("report request to %s: %w", managerAddr, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read report response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("report rejected (%s): %s", resp.Status, bytes.TrimSpace(respBody))
	}

	var ack ReportAck
	if err := json.Unmarshal(respBody, &ack); err != nil {
		return nil, fmt.Errorf("parse report ack: %w", err)
	}
	return &ack, nil
}
