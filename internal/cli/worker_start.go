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
	"github.com/justwaters/sitrep/internal/sysd"
	"github.com/justwaters/sitrep/internal/worker"
)

func newWorkerStartCmd() *cobra.Command {
	var dataDir string
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the worker agent (runs the setup wizard on first launch)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkerStart(cmd, dataDir, foreground)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", config.WorkerDataDir, "directory holding the worker's client cert and config")
	cmd.Flags().BoolVar(&foreground, "foreground", false, "run without the interactive wizard, even on first launch (used by the systemd unit)")
	return cmd
}

func runWorkerStart(cmd *cobra.Command, dataDir string, foreground bool) error {
	configPath := config.WorkerConfigPath(dataDir)
	out := cmd.OutOrStdout()

	cfg, err := config.LoadWorkerConfig(configPath)
	freshInstall := errors.Is(err, os.ErrNotExist)
	if err != nil && !freshInstall {
		return fmt.Errorf("load worker config: %w", err)
	}

	if freshInstall {
		if foreground {
			return fmt.Errorf("no config found at %s and --foreground was set; run `sitrep worker start` interactively first", configPath)
		}
		if _, err := runWorkerWizard(cmd, dataDir, configPath); err != nil {
			return err
		}
		return nil
	}

	if !foreground {
		status, _ := sysd.Status(sysd.WorkerUnitName)
		fmt.Fprintf(out, "Worker is already enrolled and configured at %s.\n", configPath)
		fmt.Fprintf(out, "systemd service %s status: %s\n", sysd.WorkerUnitName, status)
		return nil
	}

	logger := slog.New(slog.NewTextHandler(out, nil))
	svc, err := worker.NewService(cfg, configPath, logger)
	if err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("worker starting",
		"worker_id", cfg.WorkerID, "manager_addr", cfg.ManagerAddr, "interval_seconds", cfg.IntervalSeconds)
	svc.Run(ctx)
	return nil
}
