package manager

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/registry"
	"github.com/justwaters/sitrep/internal/transport"
)

// ReportHandler serves POST /report on the mTLS report listener. The
// listener's tls.Config already guarantees a verified client certificate
// exists before this handler ever runs (tls.RequireAndVerifyClientCert);
// this handler additionally cross-checks the certificate's CommonName
// against the WorkerID claimed in the report body, so a worker cannot
// report under another worker's identity even with its own validly
// CA-signed certificate.
type ReportHandler struct {
	Registry *registry.Registry
	Config   *config.ManagerConfig
	Logger   *slog.Logger
}

func (h *ReportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		http.Error(w, "client certificate required", http.StatusUnauthorized)
		return
	}
	cn := r.TLS.PeerCertificates[0].Subject.CommonName

	var report transport.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if string(report.WorkerID) != cn {
		h.logger().Warn("report worker_id does not match client certificate",
			"cert_cn", cn, "claimed_worker_id", report.WorkerID)
		http.Error(w, "worker_id does not match client certificate", http.StatusForbidden)
		return
	}

	h.Registry.Upsert(&report)

	interval, checks := h.Config.Snapshot()
	ack := transport.ReportAck{
		AcceptedAt:      time.Now().Unix(),
		IntervalSeconds: interval,
		EnabledChecks:   checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ack)
}

func (h *ReportHandler) logger() *slog.Logger {
	return loggerOrDefault(h.Logger)
}
