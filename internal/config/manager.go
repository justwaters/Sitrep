package config

import (
	"fmt"
	"net"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// ManagerConfig is the manager's on-disk configuration. It is also kept
// live in memory behind a mutex (see ManagerConfig.Mu) so the local API's
// PATCH /v1/config can change it at runtime without a restart.
type ManagerConfig struct {
	// Name is what this manager is called, chosen by the operator during
	// setup. It's embedded in the CA and server certificate common names
	// and shown wherever the manager identifies itself (e.g. `token create`).
	Name string `yaml:"name"`
	// ListenAddr is the mTLS report listener address, e.g. "0.0.0.0:8443".
	ListenAddr string `yaml:"listen_addr"`
	// EnrollAddr is the bootstrap (server-auth-only) enrollment listener
	// address, e.g. "0.0.0.0:8444".
	EnrollAddr string `yaml:"enroll_addr"`
	// AdvertiseHost is the hostname or IP workers use to reach this
	// manager (distinct from ListenAddr's bind address, which may be
	// 0.0.0.0). It's embedded as a SAN in the manager's server certificate
	// and is what `token create` prints as the manager address.
	AdvertiseHost string `yaml:"advertise_host"`
	// APIListenAddr is the local JSON query API address. Must resolve to a
	// loopback address; enforced at startup.
	APIListenAddr string `yaml:"api_listen_addr"`
	// IntervalSeconds is the reporting interval pushed to workers.
	IntervalSeconds int `yaml:"interval_seconds"`
	// EnabledChecks is the set of checks (see transport.AllChecks) workers
	// should collect and report.
	EnabledChecks []string `yaml:"enabled_checks"`
	// DataDir is where the CA, server cert, and this file itself live.
	DataDir string `yaml:"data_dir"`

	mu sync.RWMutex `yaml:"-"`
}

// DefaultManagerConfig returns sane defaults for a fresh install.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ListenAddr:      "0.0.0.0:8443",
		EnrollAddr:      "0.0.0.0:8444",
		AdvertiseHost:   "127.0.0.1",
		APIListenAddr:   "127.0.0.1:8080",
		IntervalSeconds: 60,
		EnabledChecks: []string{
			"internet", "disk", "dram", "cpu", "gitauth",
		},
		DataDir: ManagerDataDir,
	}
}

// Snapshot returns a copy of the fields the report/enrollment handlers and
// API need to read concurrently, safe to call from any goroutine.
func (c *ManagerConfig) Snapshot() (intervalSeconds int, enabledChecks []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	checks := make([]string, len(c.EnabledChecks))
	copy(checks, c.EnabledChecks)
	return c.IntervalSeconds, checks
}

// Update mutates the live config under lock. The caller is responsible for
// persisting via Save afterward if the change should survive a restart.
func (c *ManagerConfig) Update(fn func(*ManagerConfig)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}

// LoadManagerConfig reads and parses a manager config.yaml. It returns
// os.ErrNotExist (wrapped) if the file doesn't exist yet, which callers use
// to detect a fresh install and trigger the setup wizard.
func LoadManagerConfig(path string) (*ManagerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &ManagerConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse manager config %s: %w", path, err)
	}
	return cfg, nil
}

// ReportAddr returns the host:port workers should dial for the mTLS
// report endpoint: AdvertiseHost combined with ListenAddr's port.
func (c *ManagerConfig) ReportAddr() (string, error) {
	return combineAdvertiseAddr(c.AdvertiseHost, c.ListenAddr)
}

// EnrollAdvertiseAddr returns the host:port workers should dial for the
// enrollment endpoint: AdvertiseHost combined with EnrollAddr's port.
func (c *ManagerConfig) EnrollAdvertiseAddr() (string, error) {
	return combineAdvertiseAddr(c.AdvertiseHost, c.EnrollAddr)
}

func combineAdvertiseAddr(host, listenAddr string) (string, error) {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", fmt.Errorf("parse port from %q: %w", listenAddr, err)
	}
	return net.JoinHostPort(host, port), nil
}

// Save writes the config to path as YAML, creating parent directories with
// 0700 permissions if needed.
func (c *ManagerConfig) Save(path string) error {
	c.mu.RLock()
	data, err := yaml.Marshal(c)
	c.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal manager config: %w", err)
	}
	return writeConfigFile(path, data)
}
