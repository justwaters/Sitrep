// Package config defines and (de)serializes the manager and worker
// on-disk configuration files.
package config

import "path/filepath"

const (
	// ManagerDataDir is the default directory holding the manager's CA,
	// server certificate, and config.
	ManagerDataDir = "/etc/sitrep/manager"
	// WorkerDataDir is the default directory holding the worker's client
	// certificate, the manager's CA cert, and config.
	WorkerDataDir = "/etc/sitrep/worker"

	// SystemUser is the unprivileged system user both daemons run as.
	SystemUser = "sitrep"
)

func ManagerConfigPath(dataDir string) string { return filepath.Join(dataDir, "config.yaml") }
func ManagerCAKeyPath(dataDir string) string  { return filepath.Join(dataDir, "ca.key.pem") }
func ManagerCACertPath(dataDir string) string { return filepath.Join(dataDir, "ca.crt.pem") }
func ManagerServerKeyPath(dataDir string) string {
	return filepath.Join(dataDir, "server.key.pem")
}
func ManagerServerCertPath(dataDir string) string {
	return filepath.Join(dataDir, "server.crt.pem")
}

func WorkerConfigPath(dataDir string) string     { return filepath.Join(dataDir, "config.yaml") }
func WorkerClientKeyPath(dataDir string) string  { return filepath.Join(dataDir, "client.key.pem") }
func WorkerClientCertPath(dataDir string) string { return filepath.Join(dataDir, "client.crt.pem") }
func WorkerCACertPath(dataDir string) string     { return filepath.Join(dataDir, "ca.crt.pem") }
