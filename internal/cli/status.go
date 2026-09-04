package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/sysd"
)

func newStatusCmd(short, unitName string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := sysd.Status(unitName)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", unitName, state)
			if err != nil && state != "active" {
				return fmt.Errorf("service is not active (see `journalctl -u %s` for details)", unitName)
			}
			return nil
		},
	}
}
