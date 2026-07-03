package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/validation"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func newContextCommand(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx", "contexts"},
		Short:   "Manage contexts",
		Long:    "Create, read, update and delete contexts, query their status, and run GitOps actions.",
	}
	cmd.AddCommand(
		newContextCreateCommand(flags),
		newContextGetCommand(flags),
		newContextListCommand(flags),
		newContextUpdateCommand(flags),
		newContextDeleteCommand(flags),
		newContextStatusCommand(flags),
		newContextRunCommand(flags),
	)
	return cmd
}

func newContextCreateCommand(flags *GlobalFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create [FILE]",
		Short: "Create a context from a YAML or JSON manifest",
		Long: `Create a context from a manifest file.

The manifest may be given as a positional argument, via --file, or piped on
stdin ("-"). YAML and JSON are both accepted.`,
		Example: `  platformctl context create context.yaml
  platformctl context create -f context.yaml
  cat context.yaml | platformctl context create -`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := decodeContextManifest(cmd, source(file, args))
			if err != nil {
				return err
			}
			if err := validateManifest(ctx); err != nil {
				return err
			}
			client := NewClient(flags, cmd.ErrOrStderr())
			resp, err := client.CreateContext(*ctx)
			if err != nil {
				return err
			}
			return renderCreateUpdate(cmd.OutOrStdout(), flags.Output, resp, resp.Message)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", `manifest file (YAML or JSON); "-" for stdin`)
	return cmd
}

func newContextGetCommand(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get NAME",
		Short: "Get a single context by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := NewClient(flags, cmd.ErrOrStderr())
			resp, err := client.GetContext(args[0])
			if err != nil {
				return err
			}
			return renderContext(cmd.OutOrStdout(), flags.Output, resp.Context)
		},
	}
	return cmd
}

func newContextListCommand(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all contexts",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := NewClient(flags, cmd.ErrOrStderr())
			resp, err := client.ListContexts()
			if err != nil {
				return err
			}
			return renderContextList(cmd.OutOrStdout(), flags.Output, resp.Contexts)
		},
	}
	return cmd
}

func newContextUpdateCommand(flags *GlobalFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update NAME [FILE]",
		Short: "Update an existing context from a manifest",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ctx, err := decodeContextManifest(cmd, source(file, args[1:]))
			if err != nil {
				return err
			}
			// Reconcile the name from the NAME argument BEFORE validating, so a
			// manifest that omits metadata.name can rely on NAME to supply it.
			if ctx.Metadata.Name != "" && ctx.Metadata.Name != name {
				return fmt.Errorf("manifest name %q does not match NAME argument %q", ctx.Metadata.Name, name)
			}
			ctx.Metadata.Name = name
			if err := validateManifest(ctx); err != nil {
				return err
			}
			client := NewClient(flags, cmd.ErrOrStderr())
			resp, err := client.UpdateContext(name, *ctx)
			if err != nil {
				return err
			}
			return renderCreateUpdate(cmd.OutOrStdout(), flags.Output, resp, resp.Message)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", `manifest file (YAML or JSON); "-" for stdin`)
	return cmd
}

func newContextDeleteCommand(flags *GlobalFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "delete NAME",
		Aliases: []string{"rm"},
		Short:   "Delete a context",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !force {
				ok, err := confirm(cmd, fmt.Sprintf("Delete context %q?", name))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			client := NewClient(flags, cmd.ErrOrStderr())
			if _, err := client.DeleteContext(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Context %q deleted.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation prompt")
	return cmd
}

// source picks the manifest source: the --file flag wins, otherwise the first
// positional argument, otherwise "-" (stdin).
func source(file string, args []string) string {
	if file != "" {
		return file
	}
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	return "-"
}

// decodeContextManifest reads a manifest from path ("-" means stdin) and
// decodes it (YAML or JSON) into a models.Context. It does NOT validate, so
// callers can reconcile fields (e.g. inject the name from a NAME argument)
// before validation.
func decodeContextManifest(cmd *cobra.Command, path string) (*models.Context, error) {
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(cmd.InOrStdin())
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("manifest is empty")
	}

	var ctx models.Context
	if err := yaml.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &ctx, nil
}

// validateManifest runs the same client-side validation the gateway applies, so
// the user gets fast feedback before the request is sent.
func validateManifest(ctx *models.Context) error {
	if err := validation.ValidateContext(ctx); err != nil {
		return fmt.Errorf("invalid context manifest: %w", err)
	}
	return nil
}

// confirm prompts on stdout and reads a yes/no answer from stdin.
func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", prompt)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
