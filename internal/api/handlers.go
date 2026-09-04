package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/registry"
	"github.com/justwaters/sitrep/internal/token"
	"github.com/justwaters/sitrep/internal/transport"
)

// Handler wires the JSON API's routes to the manager's live worker
// registry, config, and enrollment token store.
type Handler struct {
	Registry              *registry.Registry
	Config                *config.ManagerConfig
	ConfigPath            string
	Tokens                *token.Store
	ServerCertFingerprint string
	Logger                *slog.Logger
}

// Mux builds the API's route table.
func (h *Handler) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workers", h.listWorkers)
	mux.HandleFunc("GET /v1/workers/{id}", h.getWorker)
	mux.HandleFunc("GET /v1/config", h.getConfig)
	mux.HandleFunc("PATCH /v1/config", h.patchConfig)
	mux.HandleFunc("POST /v1/tokens", h.createToken)
	return mux
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	var req TokenCreateRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	t, err := h.Tokens.Create(time.Duration(req.TTLSeconds) * time.Second)
	if err != nil {
		h.logger().Error("failed to create enrollment token", "err", err)
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	enrollAddr, err := h.Config.EnrollAdvertiseAddr()
	if err != nil {
		http.Error(w, "manager misconfigured: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, TokenCreateResponse{
		Token:                 t.Value,
		ExpiresAt:             t.ExpiresAt.Unix(),
		EnrollAddr:            enrollAddr,
		ServerCertFingerprint: h.ServerCertFingerprint,
	})
}

func (h *Handler) listWorkers(w http.ResponseWriter, r *http.Request) {
	entries := h.Registry.List()
	out := make([]WorkerSummary, 0, len(entries))
	for _, e := range entries {
		out = append(out, summaryOf(e))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getWorker(w http.ResponseWriter, r *http.Request) {
	id := transport.WorkerID(r.PathValue("id"))
	entry, ok := h.Registry.Get(id)
	if !ok {
		http.Error(w, "worker not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, WorkerDetail{
		WorkerSummary: summaryOf(entry),
		LastReport:    entry.LastReport,
	})
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.configResponse())
}

func (h *Handler) patchConfig(w http.ResponseWriter, r *http.Request) {
	var patch ConfigPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if patch.IntervalSeconds != nil && *patch.IntervalSeconds <= 0 {
		http.Error(w, "interval_seconds must be positive", http.StatusBadRequest)
		return
	}

	h.Config.Update(func(c *config.ManagerConfig) {
		if patch.IntervalSeconds != nil {
			c.IntervalSeconds = *patch.IntervalSeconds
		}
		if patch.EnabledChecks != nil {
			c.EnabledChecks = *patch.EnabledChecks
		}
	})

	// Persisting immediately (rather than only holding the change in
	// memory) means a manager restart doesn't silently revert an
	// operator's API-driven config change.
	if err := h.Config.Save(h.ConfigPath); err != nil {
		h.logger().Error("failed to persist config patch", "err", err)
		http.Error(w, "failed to persist config", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, h.configResponse())
}

func (h *Handler) configResponse() ConfigResponse {
	interval, checks := h.Config.Snapshot()
	return ConfigResponse{
		ListenAddr:      h.Config.ListenAddr,
		APIListenAddr:   h.Config.APIListenAddr,
		IntervalSeconds: interval,
		EnabledChecks:   checks,
	}
}

func summaryOf(e *registry.WorkerEntry) WorkerSummary {
	return WorkerSummary{
		ID:         string(e.ID),
		Hostname:   e.Hostname,
		EnrolledAt: e.EnrolledAt.Unix(),
		LastSeen:   e.LastSeen.Unix(),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}
