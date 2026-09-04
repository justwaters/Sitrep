package cli

import (
	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/sysd"
)

func newManagerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Run and manage the Sitrep manager agent",
	}
	cmd.AddCommand(newManagerStartCmd())
	cmd.AddCommand(newManagerTokenCmd())
	cmd.AddCommand(newStatusCmd("Show the manager service's systemd status", sysd.ManagerUnitName))
	return cmd
}
