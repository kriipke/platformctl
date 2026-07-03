package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "platformctl %s\n", info.Version)
			fmt.Fprintf(w, "  commit:     %s\n", info.CommitSHA)
			fmt.Fprintf(w, "  build date: %s\n", info.BuildDate)
			return nil
		},
	}
}
