package cli

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/justwaters/sitrep/internal/stats"
)

// offerDependencyInstall shows the operator the exact install command for
// each missing binary and only runs it on explicit confirmation — Sitrep
// never installs packages silently.
func offerDependencyInstall(out io.Writer, missing []string) error {
	pm := stats.PackageManager()
	fmt.Fprintf(out, "\nMissing dependencies: %s\n", strings.Join(missing, ", "))
	if pm == "" {
		fmt.Fprintln(out, "Could not detect a supported package manager (apt/dnf/pacman) to suggest an install command.")
		fmt.Fprintln(out, "Install these manually before continuing — checks depending on them will report errors until then.")
		return nil
	}

	for _, bin := range missing {
		installCmd := stats.InstallCommand(pm, bin)
		if installCmd == "" {
			fmt.Fprintf(out, "\n  %s: no known install command for %s; install it manually.\n", bin, pm)
			continue
		}

		fmt.Fprintf(out, "\n  %s is required and was not found in PATH.\n  Install command: %s\n", bin, installCmd)
		if notes := stats.InstallNotes(bin); notes != "" {
			fmt.Fprintf(out, "  Note: %s\n", notes)
		}

		var confirm bool
		confirmForm := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Run this command now? (%s)", installCmd)).
				Value(&confirm),
		))
		if err := confirmForm.Run(); err != nil {
			return fmt.Errorf("dependency prompt: %w", err)
		}
		if !confirm {
			fmt.Fprintf(out, "  Skipping. Checks requiring %q will report errors until it's installed.\n", bin)
			continue
		}

		parts := strings.Fields(installCmd)
		runCmd := exec.Command(parts[0], parts[1:]...)
		runCmd.Stdout = out
		runCmd.Stderr = out
		if err := runCmd.Run(); err != nil {
			fmt.Fprintf(out, "  Install failed: %v\n", err)
		}
	}
	return nil
}
