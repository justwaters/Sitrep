// Command sitrep runs the Sitrep manager or worker agent.
package main

import (
	"os"

	"github.com/justwaters/sitrep/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
