package config

import (
	"os"
	"path/filepath"
)

// writeConfigFile writes data to path, creating the parent directory with
// 0700 permissions if it doesn't already exist, and the file itself 0600
// since config files may reference cert/key paths and are operationally
// sensitive.
func writeConfigFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
