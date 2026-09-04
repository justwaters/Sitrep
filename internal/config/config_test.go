package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justwaters/sitrep/internal/transport"
)

func TestManagerConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultManagerConfig()
	cfg.DataDir = dir
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadManagerConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ListenAddr != cfg.ListenAddr {
		t.Errorf("ListenAddr = %q, want %q", loaded.ListenAddr, cfg.ListenAddr)
	}
	if loaded.IntervalSeconds != cfg.IntervalSeconds {
		t.Errorf("IntervalSeconds = %d, want %d", loaded.IntervalSeconds, cfg.IntervalSeconds)
	}
	if len(loaded.EnabledChecks) != len(cfg.EnabledChecks) {
		t.Errorf("EnabledChecks length = %d, want %d", len(loaded.EnabledChecks), len(cfg.EnabledChecks))
	}
}

func TestLoadManagerConfigNotExist(t *testing.T) {
	_, err := LoadManagerConfig(filepath.Join(t.TempDir(), "nonexistent", "config.yaml"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist-wrapped error, got %v", err)
	}
}

func TestWorkerConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &WorkerConfig{
		WorkerID:        transport.WorkerID("wkr_1"),
		ManagerAddr:     "1.2.3.4:8443",
		IntervalSeconds: 30,
		EnabledChecks:   []string{"disk"},
		DataDir:         dir,
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadWorkerConfig(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.WorkerID != cfg.WorkerID {
		t.Errorf("WorkerID = %q, want %q", loaded.WorkerID, cfg.WorkerID)
	}
	if loaded.ManagerAddr != cfg.ManagerAddr {
		t.Errorf("ManagerAddr = %q, want %q", loaded.ManagerAddr, cfg.ManagerAddr)
	}
}

func TestReportAddr(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.AdvertiseHost = "example.com"
	cfg.ListenAddr = "0.0.0.0:8443"

	addr, err := cfg.ReportAddr()
	if err != nil {
		t.Fatalf("ReportAddr: %v", err)
	}
	if addr != "example.com:8443" {
		t.Errorf("ReportAddr() = %q, want example.com:8443", addr)
	}
}

func TestEnrollAdvertiseAddr(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.AdvertiseHost = "example.com"
	cfg.EnrollAddr = "0.0.0.0:8444"

	addr, err := cfg.EnrollAdvertiseAddr()
	if err != nil {
		t.Fatalf("EnrollAdvertiseAddr: %v", err)
	}
	if addr != "example.com:8444" {
		t.Errorf("EnrollAdvertiseAddr() = %q, want example.com:8444", addr)
	}
}

func TestManagerConfigUpdateAndSnapshot(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.Update(func(c *ManagerConfig) {
		c.IntervalSeconds = 120
		c.EnabledChecks = []string{"cpu"}
	})

	interval, checks := cfg.Snapshot()
	if interval != 120 {
		t.Errorf("IntervalSeconds = %d, want 120", interval)
	}
	if len(checks) != 1 || checks[0] != "cpu" {
		t.Errorf("EnabledChecks = %v, want [cpu]", checks)
	}
}
