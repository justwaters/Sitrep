package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/justwaters/sitrep/internal/transport"
)

// WorkerConfig is the worker's on-disk configuration. IntervalSeconds and
// EnabledChecks are a cache of the last value pushed by the manager (via
// EnrollResponse or ReportAck): on restart the worker resumes reporting
// with the last-known-good config before it has re-contacted the manager.
type WorkerConfig struct {
	// Name is what this machine is called, chosen by the operator during
	// setup and reported to the manager on every check-in — see
	// transport.Report.Name.
	Name            string             `yaml:"name"`
	WorkerID        transport.WorkerID `yaml:"worker_id"`
	ManagerAddr     string             `yaml:"manager_addr"`
	IntervalSeconds int                `yaml:"interval_seconds"`
	EnabledChecks   []string           `yaml:"enabled_checks"`
	DataDir         string             `yaml:"data_dir"`
}

// LoadWorkerConfig reads and parses a worker config.yaml.
func LoadWorkerConfig(path string) (*WorkerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &WorkerConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse worker config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config to path as YAML, creating parent directories with
// 0700 permissions if needed.
func (c *WorkerConfig) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal worker config: %w", err)
	}
	return writeConfigFile(path, data)
}
