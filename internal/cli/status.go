package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newContextStatusCommand(flags *GlobalFlags) *cobra.Command {
	var (
		watch    bool
		interval time.Duration
	)
	cmd := &cobra.Command{
		Use:   "status NAME",
		Short: "Show the aggregated GitOps status of a context",
		Long:  "Query the read-model status for a context (health, sync and pairing state).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if watch && interval <= 0 {
				return fmt.Errorf("--interval must be positive, got %s", interval)
			}
			client := NewClient(flags, cmd.ErrOrStderr())

			showOnce := func() error {
				status, err := client.GetContextStatus(name)
				if err != nil {
					return err
				}
				return renderStatus(cmd.OutOrStdout(), flags.Output, name, status)
			}

			if !watch {
				return showOnce()
			}

			// Watch mode: clear the screen and re-render on each tick until the
			// user interrupts (Ctrl-C).
			out := cmd.OutOrStdout()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				fmt.Fprint(out, "\033[H\033[2J") // ANSI: cursor home + clear screen
				fmt.Fprintf(out, "Watching %q every %s (Ctrl-C to stop) — %s\n\n",
					name, interval, time.Now().Format("15:04:05"))
				if err := showOnce(); err != nil {
					fmt.Fprintf(out, "error: %v\n", err)
				}
				<-ticker.C
			}
		},
	}
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "continuously refresh the status")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "refresh interval in watch mode")
	return cmd
}
