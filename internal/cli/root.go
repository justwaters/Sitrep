// Package cli wires up the sitrep command tree (manager and worker
// subcommands) and their interactive setup wizards.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/buildinfo"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sitrep",
		Short: "Sitrep — remote systems status reporting over mTLS",
		// Runtime/domain errors (e.g. "not root", "enrollment failed")
		// aren't flag-usage mistakes; don't dump the usage block for them.
		SilenceUsage: true,
	}
	root.AddCommand(newManagerCmd())
	root.AddCommand(newWorkerCmd())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the sitrep version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "sitrep %s (commit %s, built %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
			return nil
		},
	})
	return root
}

// Execute runs the sitrep CLI.
func Execute() error {
	return newRootCmd().Execute()
}
