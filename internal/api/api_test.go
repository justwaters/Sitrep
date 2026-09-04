package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/justwaters/sitrep/internal/api"
	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/registry"
	"github.com/justwaters/sitrep/internal/token"
	"github.com/justwaters/sitrep/internal/transport"
)

func newTestHandler(t *testing.T) (*api.Handler, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	return &api.Handler{
		Registry:              registry.New(),
		Config:                config.DefaultManagerConfig(),
		ConfigPath:            configPath,
		Tokens:                token.NewStore(),
		ServerCertFingerprint: "deadbeef",
	}, configPath
}

func TestListWorkers(t *testing.T) {
	h, _ := newTestHandler(t)
	h.Registry.Upsert(&transport.Report{WorkerID: "wkr_1", Name: "host1", Timestamp: 1})

	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/workers")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var workers []api.WorkerSummary
	if err := json.NewDecoder(resp.Body).Decode(&workers); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(workers) != 1 || workers[0].ID != "wkr_1" {
		t.Errorf("workers = %+v", workers)
	}
}

func TestGetWorker_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/workers/wkr_missing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetWorker_Found(t *testing.T) {
	h, _ := newTestHandler(t)
	h.Registry.Upsert(&transport.Report{WorkerID: "wkr_1", Name: "host1", Timestamp: 42})

	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/workers/wkr_1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var detail api.WorkerDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.LastReport == nil || detail.LastReport.Timestamp != 42 {
		t.Errorf("detail = %+v", detail)
	}
}

func TestGetConfig(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/config")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var cfg api.ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.IntervalSeconds != h.Config.IntervalSeconds {
		t.Errorf("IntervalSeconds = %d, want %d", cfg.IntervalSeconds, h.Config.IntervalSeconds)
	}
}

func TestPatchConfig(t *testing.T) {
	h, configPath := newTestHandler(t)
	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	newInterval := 99
	body, _ := json.Marshal(api.ConfigPatch{IntervalSeconds: &newInterval})
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/v1/config", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var cfg api.ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.IntervalSeconds != 99 {
		t.Errorf("IntervalSeconds = %d, want 99", cfg.IntervalSeconds)
	}

	loaded, err := config.LoadManagerConfig(configPath)
	if err != nil {
		t.Fatalf("reload persisted config: %v", err)
	}
	if loaded.IntervalSeconds != 99 {
		t.Errorf("persisted IntervalSeconds = %d, want 99", loaded.IntervalSeconds)
	}
}

func TestPatchConfig_RejectsNonPositiveInterval(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	zero := 0
	body, _ := json.Marshal(api.ConfigPatch{IntervalSeconds: &zero})
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/v1/config", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateToken(t *testing.T) {
	h, _ := newTestHandler(t)
	srv := httptest.NewServer(h.Mux())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/tokens", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var out api.TokenCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Token == "" {
		t.Error("expected non-empty token")
	}
	if out.ServerCertFingerprint != "deadbeef" {
		t.Errorf("ServerCertFingerprint = %q, want deadbeef", out.ServerCertFingerprint)
	}

	if _, err := h.Tokens.Consume(out.Token); err != nil {
		t.Errorf("token from response should be consumable from the same store: %v", err)
	}
}
