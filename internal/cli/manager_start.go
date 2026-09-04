package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/manager"
	"github.com/justwaters/sitrep/internal/sysd"
)

func newManagerStartCmd() *cobra.Command {
	var dataDir string
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the manager agent (runs the setup wizard on first launch)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagerStart(cmd, dataDir, foreground)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", config.ManagerDataDir, "directory holding the manager's CA, server cert, and config")
	cmd.Flags().BoolVar(&foreground, "foreground", false, "run without the interactive wizard, even on first launch (used by the systemd unit)")
	return cmd
}

func runManagerStart(cmd *cobra.Command, dataDir string, foreground bool) error {
	configPath := config.ManagerConfigPath(dataDir)
	out := cmd.OutOrStdout()

	cfg, err := config.LoadManagerConfig(configPath)
	freshInstall := errors.Is(err, os.ErrNotExist)
	if err != nil && !freshInstall {
		return fmt.Errorf("load manager config: %w", err)
	}

	if freshInstall {
		if foreground {
			return fmt.Errorf("no config found at %s and --foreground was set; run `sitrep manager start` interactively first", configPath)
		}
		if _, err := runManagerWizard(cmd, dataDir, configPath); err != nil {
			return err
		}
		return nil
	}

	if !foreground {
		status, _ := sysd.Status(sysd.ManagerUnitName)
		fmt.Fprintf(out, "Manager %q is already configured at %s.\n", cfg.Name, configPath)
		fmt.Fprintf(out, "systemd service %s status: %s\n", sysd.ManagerUnitName, status)
		fmt.Fprintf(out, "Run `sitrep manager token create` to enroll a new worker.\n")
		return nil
	}

	// --foreground: this is the actual daemon entrypoint, invoked by the
	// systemd unit's ExecStart (or directly, for manual/debug runs).
	logger := slog.New(slog.NewTextHandler(out, nil))
	svc, err := manager.NewService(cfg, configPath, logger)
	if err != nil {
		return fmt.Errorf("start manager: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("manager starting",
		"name", cfg.Name, "report_addr", cfg.ListenAddr, "enroll_addr", cfg.EnrollAddr, "api_addr", cfg.APIListenAddr)
	return svc.Run(ctx)
}
