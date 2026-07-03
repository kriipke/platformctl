// Command platformctl is the command-line interface for the Platformctl
// GitOps monitoring platform. It talks to the API Gateway over HTTP to manage
// contexts and trigger GitOps actions.
package main

import (
	"os"

	"github.com/kriipke/platformctl/internal/cli"
)

// Build metadata, injected at link time via -ldflags (see the Makefile).
// The variable names match those used by the services so the same LDFLAGS work.
var (
	Version   = "dev"
	CommitSHA = "unknown"
	BuildDate = "unknown"
)

func main() {
	root := cli.NewRootCommand(cli.BuildInfo{
		Version:   Version,
		CommitSHA: CommitSHA,
		BuildDate: BuildDate,
	})

	// Cobra prints the error (RunE) to stderr itself; we only need to signal a
	// non-zero exit status to the shell.
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
