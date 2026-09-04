package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/pki"
	"github.com/justwaters/sitrep/internal/stats"
	"github.com/justwaters/sitrep/internal/transport"
)

// Service wires together an already-enrolled worker's mTLS report client
// and its collect-and-report loop.
type Service struct {
	Loop *ReportLoop
}

// NewService loads the worker's client certificate and the manager's CA
// cert from cfg.DataDir (written there by Enroll) and builds the report
// loop ready to run.
func NewService(cfg *config.WorkerConfig, configPath string, logger *slog.Logger) (*Service, error) {
	clientCert, err := pki.LoadTLSCertificate(
		config.WorkerClientCertPath(cfg.DataDir),
		config.WorkerClientKeyPath(cfg.DataDir),
	)
	if err != nil {
		return nil, fmt.Errorf("load client certificate (has this worker been enrolled?): %w", err)
	}

	caPool, err := pki.LoadCertPool(config.WorkerCACertPath(cfg.DataDir))
	if err != nil {
		return nil, fmt.Errorf("load manager CA certificate: %w", err)
	}

	client := transport.NewReportClient(clientCert, caPool)
	loop := NewReportLoop(cfg, configPath, client, stats.NewDefaultRegistry(), logger)

	return &Service{Loop: loop}, nil
}

// Run blocks, reporting on the manager-controlled interval, until ctx is
// done.
func (s *Service) Run(ctx context.Context) {
	s.Loop.Run(ctx)
}
