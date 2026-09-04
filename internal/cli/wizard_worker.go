package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/stats"
	"github.com/justwaters/sitrep/internal/sysd"
	"github.com/justwaters/sitrep/internal/transport"
	"github.com/justwaters/sitrep/internal/worker"
)

const enrollTimeout = 30 * time.Second

// runWorkerWizard checks for missing check dependencies (offering to
// install them), collects the manager address/token/fingerprint from the
// operator, performs the enrollment handshake, and installs the worker as
// a persistent systemd service.
func runWorkerWizard(cmd *cobra.Command, dataDir, configPath string) (*config.WorkerConfig, error) {
	out := cmd.OutOrStdout()

	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("run `sitrep worker start` as root (e.g. via sudo) — first-run setup installs a systemd service")
	}

	fmt.Fprintln(out, "Sitrep worker setup")
	fmt.Fprintln(out, "====================")

	// The manager decides which checks are enabled only after
	// enrollment, so check dependencies for every known check up front —
	// better to flag them now than have a check silently fail later.
	if missing := stats.Missing(transport.AllChecks); len(missing) > 0 {
		if err := offerDependencyInstall(out, missing); err != nil {
			return nil, err
		}
	}

	name := defaultMachineName()
	var managerAddr, tok, fingerprint string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What do you want to call this machine?").
				Description("This is how the manager will identify it in reports.").
				Value(&name).
				Validate(requireNonEmpty),
		),
		huh.NewGroup(
			huh.NewNote().
				Title("Link to a manager").
				Description("Get these values from `sitrep manager token create` run on the manager machine."),
			huh.NewInput().Title("Manager enrollment address (host:port)").Value(&managerAddr),
			huh.NewInput().Title("Enrollment token").Value(&tok),
			huh.NewInput().Title("Manager server certificate fingerprint (sha256 hex)").Value(&fingerprint),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("setup wizard: %w", err)
	}

	fmt.Fprintln(out, "\nEnrolling with manager...")
	ctx, cancel := context.WithTimeout(context.Background(), enrollTimeout)
	defer cancel()

	cfg, err := worker.Enroll(ctx, worker.EnrollParams{
		Name:          name,
		ManagerAddr:   managerAddr,
		Token:         tok,
		CAFingerprint: fingerprint,
		DataDir:       dataDir,
	})
	if err != nil {
		return nil, fmt.Errorf("enrollment failed: %w", err)
	}
	fmt.Fprintf(out, "Enrolled as %q (worker %s).\n", cfg.Name, cfg.WorkerID)

	fmt.Fprintln(out, "\nInstalling the sitrep-worker systemd service...")
	if err := sysd.EnsureSystemUser(); err != nil {
		return nil, fmt.Errorf("create sitrep system user: %w", err)
	}
	if err := sysd.ChownDataDir(dataDir); err != nil {
		return nil, fmt.Errorf("set data directory ownership: %w", err)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve sitrep binary path: %w", err)
	}

	unitContent, err := sysd.RenderUnit(sysd.WorkerUnitTemplate, sysd.UnitParams{
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		DataDir:    dataDir,
	})
	if err != nil {
		return nil, err
	}
	if err := sysd.InstallUnit(sysd.WorkerUnitName, unitContent); err != nil {
		return nil, fmt.Errorf("install systemd service: %w", err)
	}

	fmt.Fprintf(out, "\nDone. %q is now reporting in the background as %s.\n", cfg.Name, sysd.WorkerUnitName)
	fmt.Fprintf(out, "Check its status any time with: systemctl status %s\n", sysd.WorkerUnitName)

	return cfg, nil
}
