package stats

import (
	"os/exec"
	"sort"

	"github.com/justwaters/sitrep/internal/transport"
)

// RequiredBinaries maps each check name to the external binaries its
// Checker shells out to. Checks not listed here (dram, cpu) rely only on
// gopsutil and need no external binary.
var RequiredBinaries = map[string][]string{
	transport.CheckInternet: {"ping"},
	transport.CheckGitAuth:  {"gh"},
}

// Missing returns, for the given set of enabled check names, which
// required external binaries are absent from $PATH — sorted, de-duplicated.
func Missing(enabledChecks []string) []string {
	seen := make(map[string]bool)
	var missing []string
	for _, check := range enabledChecks {
		for _, bin := range RequiredBinaries[check] {
			if seen[bin] {
				continue
			}
			seen[bin] = true
			if _, err := exec.LookPath(bin); err != nil {
				missing = append(missing, bin)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// PackageManager detects which of the supported package managers is
// present on this host, in preference order, or "" if none are found.
func PackageManager() string {
	for _, pm := range []string{"apt-get", "dnf", "pacman"} {
		if _, err := exec.LookPath(pm); err == nil {
			return pm
		}
	}
	return ""
}

// InstallCommand returns the exact shell command an operator should run
// to install binary via package manager pm, or "" if the combination
// isn't recognized. The wizard shows this command and only executes it on
// explicit operator confirmation — Sitrep never installs packages
// silently.
func InstallCommand(pm, binary string) string {
	pkg := packageNameFor(pm, binary)
	if pkg == "" {
		return ""
	}
	switch pm {
	case "apt-get":
		return "sudo apt-get install -y " + pkg
	case "dnf":
		return "sudo dnf install -y " + pkg
	case "pacman":
		return "sudo pacman -S --noconfirm " + pkg
	default:
		return ""
	}
}

// InstallNotes returns extra operator guidance for binaries whose install
// isn't a plain one-line package install, or "" if none is needed.
func InstallNotes(binary string) string {
	if binary == "gh" {
		return "the GitHub CLI isn't in every distro's default repos; " +
			"if the install command fails, add GitHub's own apt/dnf/pacman repo first — " +
			"see https://github.com/cli/cli/blob/trunk/docs/install_linux.md"
	}
	return ""
}

func packageNameFor(pm, binary string) string {
	switch binary {
	case "ping":
		switch pm {
		case "apt-get":
			return "iputils-ping"
		case "dnf", "pacman":
			return "iputils"
		}
	case "gh":
		switch pm {
		case "apt-get", "dnf":
			return "gh"
		case "pacman":
			return "github-cli"
		}
	}
	return ""
}
