package manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/justwaters/sitrep/internal/api"
	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/pki"
	"github.com/justwaters/sitrep/internal/registry"
	"github.com/justwaters/sitrep/internal/token"
	"github.com/justwaters/sitrep/internal/transport"
)

const shutdownTimeout = 10 * time.Second

// Service wires together everything the manager agent needs to run: its
// CA and server certificate, the enrollment and report mTLS listeners,
// and the local JSON query API.
type Service struct {
	Registry *registry.Registry
	Tokens   *token.Store
	Logger   *slog.Logger

	enrollSrv *transport.Server
	reportSrv *transport.Server
	apiSrv    *api.Server
}

// NewService loads (or bootstraps on first run) the manager's CA and
// server certificate under cfg.DataDir, then builds the enrollment,
// report, and local API servers ready to run.
func NewService(cfg *config.ManagerConfig, configPath string, logger *slog.Logger) (*Service, error) {
	ca, err := pki.LoadOrCreateCA(config.ManagerCACertPath(cfg.DataDir), config.ManagerCAKeyPath(cfg.DataDir), cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("load/create CA: %w", err)
	}

	serverCert, err := loadOrIssueServerCert(ca, cfg)
	if err != nil {
		return nil, fmt.Errorf("load/issue server certificate: %w", err)
	}
	serverCertFingerprint := transport.Fingerprint(serverCert.Certificate[0])

	reg := registry.New()
	tokens := token.NewStore()

	enrollHandler := &EnrollHandler{CA: ca, Tokens: tokens, Config: cfg, Logger: logger}
	reportHandler := &ReportHandler{Registry: reg, Config: cfg, Logger: logger}

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca.Cert)

	enrollSrv, err := transport.NewEnrollServer(cfg.EnrollAddr, serverCert, enrollHandler)
	if err != nil {
		return nil, fmt.Errorf("bind enrollment listener: %w", err)
	}
	reportSrv, err := transport.NewReportServer(cfg.ListenAddr, serverCert, clientCAs, reportHandler)
	if err != nil {
		return nil, fmt.Errorf("bind report listener: %w", err)
	}

	apiHandler := &api.Handler{
		Registry:              reg,
		Config:                cfg,
		ConfigPath:            configPath,
		Tokens:                tokens,
		ServerCertFingerprint: serverCertFingerprint,
		Logger:                logger,
	}
	apiSrv, err := api.NewServer(cfg.APIListenAddr, apiHandler.Mux())
	if err != nil {
		return nil, fmt.Errorf("build local API server: %w", err)
	}

	return &Service{
		Registry:  reg,
		Tokens:    tokens,
		Logger:    logger,
		enrollSrv: enrollSrv,
		reportSrv: reportSrv,
		apiSrv:    apiSrv,
	}, nil
}

// Run starts all three listeners and blocks until ctx is done, then
// gracefully shuts everything down.
func (s *Service) Run(ctx context.Context) error {
	errCh := make(chan error, 3)
	go func() { errCh <- s.enrollSrv.Serve() }()
	go func() { errCh <- s.reportSrv.Serve() }()
	go func() { errCh <- s.apiSrv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		s.logger().Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return errors.Join(
			s.enrollSrv.Shutdown(shutdownCtx),
			s.reportSrv.Shutdown(shutdownCtx),
			s.apiSrv.Shutdown(shutdownCtx),
		)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listener failed: %w", err)
		}
		<-ctx.Done()
		return nil
	}
}

func (s *Service) logger() *slog.Logger {
	return loggerOrDefault(s.Logger)
}

// loadOrIssueServerCert loads the manager's server certificate from disk,
// issuing (or re-issuing, if the existing one is within its renewal
// window) a fresh one signed by ca as needed.
func loadOrIssueServerCert(ca *pki.CA, cfg *config.ManagerConfig) (tls.Certificate, error) {
	certPath := config.ManagerServerCertPath(cfg.DataDir)
	keyPath := config.ManagerServerKeyPath(cfg.DataDir)

	if cert, _, err := pki.ReadCertPEM(certPath); err == nil && !pki.NeedsRenewal(cert) {
		return pki.LoadTLSCertificate(certPath, keyPath)
	}

	certDER, keyDER, err := ca.IssueServerCert(serverCertHosts(cfg), cfg.Name)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("issue server certificate: %w", err)
	}
	if err := pki.WriteCertPEM(certPath, certDER); err != nil {
		return tls.Certificate{}, err
	}
	if err := pki.WriteKeyPEM(keyPath, keyDER); err != nil {
		return tls.Certificate{}, err
	}
	return pki.LoadTLSCertificate(certPath, keyPath)
}

func serverCertHosts(cfg *config.ManagerConfig) []string {
	hosts := []string{cfg.AdvertiseHost, "127.0.0.1", "::1", "localhost"}
	seen := make(map[string]bool, len(hosts))
	out := hosts[:0]
	for _, h := range hosts {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}
