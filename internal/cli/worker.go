package cli

import (
	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/sysd"
)

func newWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run and manage the Sitrep worker agent",
	}
	cmd.AddCommand(newWorkerStartCmd())
	cmd.AddCommand(newStatusCmd("Show the worker service's systemd status", sysd.WorkerUnitName))
	return cmd
}
