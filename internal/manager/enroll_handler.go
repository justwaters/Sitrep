// Package manager implements the manager agent's HTTP handlers: the
// enrollment endpoint (issues client certs to new workers) and the report
// endpoint (accepts mTLS status reports from enrolled workers).
package manager

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/pki"
	"github.com/justwaters/sitrep/internal/token"
	"github.com/justwaters/sitrep/internal/transport"
)

// EnrollHandler serves POST /enroll on the bootstrap (no-client-cert)
// listener: it validates a one-time token, signs the worker's CSR, and
// returns a client certificate plus the manager's CA cert and current
// reporting config.
type EnrollHandler struct {
	CA     *pki.CA
	Tokens *token.Store
	Config *config.ManagerConfig
	Logger *slog.Logger
}

func (h *EnrollHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req transport.EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := h.Tokens.Consume(req.Token); err != nil {
		h.logger().Warn("enrollment rejected: bad token", "name", req.Name, "err", err)
		http.Error(w, "invalid or expired enrollment token", http.StatusUnauthorized)
		return
	}

	csr, err := pki.ParseCSR(req.CSRPEM)
	if err != nil {
		http.Error(w, "invalid certificate signing request", http.StatusBadRequest)
		return
	}

	workerID := transport.WorkerID(csr.Subject.CommonName)
	if workerID == "" {
		http.Error(w, "CSR missing common name", http.StatusBadRequest)
		return
	}

	certDER, err := h.CA.IssueClientCertFromCSR(csr, csr.Subject.CommonName)
	if err != nil {
		h.logger().Error("failed to issue client certificate", "worker_id", workerID, "err", err)
		http.Error(w, "failed to issue certificate", http.StatusInternalServerError)
		return
	}

	reportAddr, err := h.Config.ReportAddr()
	if err != nil {
		h.logger().Error("failed to compute report address", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	interval, checks := h.Config.Snapshot()
	resp := transport.EnrollResponse{
		WorkerID:        workerID,
		ClientCertPEM:   pki.EncodeCertPEM(certDER),
		CACertPEM:       h.CA.CertPEM,
		IntervalSeconds: interval,
		EnabledChecks:   checks,
		ManagerAddr:     reportAddr,
	}

	h.logger().Info("worker enrolled", "worker_id", workerID, "name", req.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *EnrollHandler) logger() *slog.Logger {
	return loggerOrDefault(h.Logger)
}
