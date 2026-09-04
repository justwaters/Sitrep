// Package sysd renders and installs the systemd unit files that make
// both the manager and worker agents persistent background daemons
// surviving reboot, per the setup wizards' final step.
package sysd

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/justwaters/sitrep/internal/config"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

const (
	ManagerUnitTemplate = "sitrep-manager.service.tmpl"
	WorkerUnitTemplate  = "sitrep-worker.service.tmpl"

	ManagerUnitName = "sitrep-manager.service"
	WorkerUnitName  = "sitrep-worker.service"

	unitDir = "/etc/systemd/system"
)

// UnitParams fills in the embedded unit templates.
type UnitParams struct {
	BinaryPath string
	ConfigPath string
	DataDir    string
}

// RenderUnit renders the named embedded template (ManagerUnitTemplate or
// WorkerUnitTemplate) with params.
func RenderUnit(templateName string, params UnitParams) ([]byte, error) {
	tmplBytes, err := templatesFS.ReadFile("templates/" + templateName)
	if err != nil {
		return nil, fmt.Errorf("read embedded template %s: %w", templateName, err)
	}
	tmpl, err := template.New(templateName).Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", templateName, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("render template %s: %w", templateName, err)
	}
	return buf.Bytes(), nil
}

// EnsureSystemUser creates the dedicated, unprivileged `sitrep` system
// user/group both daemons run as, if it doesn't already exist.
func EnsureSystemUser() error {
	if _, err := user.Lookup(config.SystemUser); err == nil {
		return nil
	}
	cmd := exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", config.SystemUser)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("useradd %s: %w: %s", config.SystemUser, err, bytes.TrimSpace(out))
	}
	return nil
}

// ChownDataDir recursively chowns dataDir to the sitrep system user, so
// the unprivileged service process can read/write its own cert, key, and
// config files.
func ChownDataDir(dataDir string) error {
	u, err := user.Lookup(config.SystemUser)
	if err != nil {
		return fmt.Errorf("lookup user %s: %w", config.SystemUser, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("parse uid %q: %w", u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parse gid %q: %w", u.Gid, err)
	}
	return filepath.Walk(dataDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

// InstallUnit writes content to /etc/systemd/system/name, reloads the
// systemd unit cache, and enables+starts the unit so it's active now and
// on every future boot.
func InstallUnit(name string, content []byte) error {
	path := filepath.Join(unitDir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write unit file %s: %w", path, err)
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", name)
}

// Status returns the trimmed output of `systemctl is-active name` (e.g.
// "active", "inactive", "failed") alongside systemctl's error, which is
// non-nil for any non-active state — callers should inspect the returned
// string, not just the error.
func Status(name string) (string, error) {
	out, err := exec.Command("systemctl", "is-active", name).Output()
	return strings.TrimSpace(string(out)), err
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}
