// Package worker implements the worker agent's enrollment handshake and
// ongoing report loop.
package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/pki"
	"github.com/justwaters/sitrep/internal/transport"
)

// EnrollParams are the inputs the worker setup wizard collects from the
// operator to link this worker to a manager.
type EnrollParams struct {
	// ManagerAddr is the manager's enrollment endpoint, host:port.
	ManagerAddr string
	// Token is the one-time enrollment token from `sitrep manager token create`.
	Token string
	// CAFingerprint is the SHA-256 hex fingerprint of the manager's
	// server certificate, printed alongside the token, pinned for this
	// one bootstrap connection.
	CAFingerprint string
	// DataDir is where the resulting cert/key/config files are written.
	DataDir string
}

// Enroll performs the one-shot enrollment handshake: generates a local
// keypair + CSR (the private key never leaves this function), presents
// the token to the manager's enrollment endpoint, and persists the
// returned client certificate, the manager's CA cert, and the resulting
// WorkerConfig to p.DataDir.
func Enroll(ctx context.Context, p EnrollParams) (*config.WorkerConfig, error) {
	workerID, err := transport.NewWorkerID()
	if err != nil {
		return nil, err
	}

	keyDER, csrPEM, err := pki.GenerateKeyAndCSR(string(workerID))
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = string(workerID)
	}

	client := transport.NewEnrollClient(p.CAFingerprint)
	resp, err := transport.Enroll(ctx, client, p.ManagerAddr, transport.EnrollRequest{
		Token:    p.Token,
		Hostname: hostname,
		CSRPEM:   csrPEM,
	})
	if err != nil {
		return nil, fmt.Errorf("enroll with manager: %w", err)
	}

	if err := pki.WriteKeyPEM(config.WorkerClientKeyPath(p.DataDir), keyDER); err != nil {
		return nil, fmt.Errorf("save client key: %w", err)
	}
	if err := pki.WriteRawPEM(config.WorkerClientCertPath(p.DataDir), resp.ClientCertPEM); err != nil {
		return nil, fmt.Errorf("save client cert: %w", err)
	}
	if err := pki.WriteRawPEM(config.WorkerCACertPath(p.DataDir), resp.CACertPEM); err != nil {
		return nil, fmt.Errorf("save CA cert: %w", err)
	}

	cfg := &config.WorkerConfig{
		WorkerID:        resp.WorkerID,
		ManagerAddr:     resp.ManagerAddr,
		IntervalSeconds: resp.IntervalSeconds,
		EnabledChecks:   resp.EnabledChecks,
		DataDir:         p.DataDir,
	}
	if err := cfg.Save(config.WorkerConfigPath(p.DataDir)); err != nil {
		return nil, fmt.Errorf("save worker config: %w", err)
	}

	return cfg, nil
}
