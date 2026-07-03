// Package cli implements the platformctl command-line interface. Commands are
// thin wrappers around the API Gateway HTTP endpoints (see internal/cli/client.go).
package cli

import (
	"github.com/spf13/cobra"
)

// BuildInfo carries version metadata injected at build time.
type BuildInfo struct {
	Version   string
	CommitSHA string
	BuildDate string
}

// GlobalFlags holds the values of the persistent (global) flags. After the
// root command's PersistentPreRunE runs, every field holds its final, resolved
// value (flag > environment variable > config file > built-in default).
type GlobalFlags struct {
	ConfigFile string
	Output     string
	Verbose    bool
	Server     string
	Username   string
	Password   string
	CustomerID string
	Token      string
	Insecure   bool
}

// NewRootCommand builds the root `platformctl` command and wires up all
// subcommands.
func NewRootCommand(info BuildInfo) *cobra.Command {
	flags := &GlobalFlags{}

	rootCmd := &cobra.Command{
		Use:   "platformctl",
		Short: "Manage Platformctl contexts and GitOps actions",
		Long: `Platformctl CLI manages application contexts and triggers GitOps
operations (sync, validate, correlate, inspect) against the Platformctl API
Gateway, and queries aggregated context status.

Configuration precedence (highest first):
  1. command-line flags
  2. environment variables (PLATFORMCTL_SERVER, PLATFORMCTL_OUTPUT,
     PLATFORMCTL_USERNAME, PLATFORMCTL_PASSWORD, PLATFORMCTL_CUSTOMER_ID,
     PLATFORMCTL_TOKEN, PLATFORMCTL_INSECURE)
  3. config file (default $HOME/.platformctl.yaml, or --config)
  4. built-in defaults`,
		Version:       info.Version,
		SilenceUsage:  true, // don't dump usage on a runtime (non-flag) error
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return flags.resolve(cmd)
		},
	}

	// A concise, informative version template.
	rootCmd.SetVersionTemplate(
		"platformctl {{.Version}} (commit " + info.CommitSHA + ", built " + info.BuildDate + ")\n",
	)

	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&flags.ConfigFile, "config", "c", "", "config file (default is $HOME/.platformctl.yaml)")
	pf.StringVarP(&flags.Output, "output", "o", defaultOutput, "output format (table|json|yaml)")
	pf.BoolVarP(&flags.Verbose, "verbose", "v", false, "verbose output (log HTTP requests to stderr)")
	pf.StringVar(&flags.Server, "server", defaultServer, "Platformctl API Gateway base URL")
	pf.StringVar(&flags.Username, "username", defaultUsername, "basic-auth username")
	pf.StringVar(&flags.Password, "password", defaultPassword, "basic-auth password")
	pf.StringVar(&flags.CustomerID, "customer-id", "", "tenant/customer id (sent as X-Customer-ID)")
	pf.StringVar(&flags.Token, "token", "", "bearer token for JWT auth (planned); overrides basic auth when set")
	pf.BoolVar(&flags.Insecure, "insecure", false, "skip TLS certificate verification")

	rootCmd.AddCommand(
		newContextCommand(flags),
		newCompletionCommand(),
		newVersionCommand(info),
	)

	return rootCmd
}
