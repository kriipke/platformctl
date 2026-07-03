package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// actionSpec describes a GitOps action: the gateway endpoint segment and the
// default `type` query parameter (only meaningful for inspect-manifests).
type actionSpec struct {
	endpoint  string
	queryType string
}

// canonicalActions maps the gateway's real action endpoints
// (POST /api/v1/contexts/{name}/actions/{endpoint}).
var canonicalActions = map[string]actionSpec{
	"sync-apps":                   {endpoint: "sync-apps"},
	"validate-environments":       {endpoint: "validate-environments"},
	"correlate-contexts":          {endpoint: "correlate-contexts"},
	"correlate-multi-environment": {endpoint: "correlate-multi-environment"},
	"inspect-manifests":           {endpoint: "inspect-manifests"},
}

// actionAliases maps friendly verbs to canonical action names.
var actionAliases = map[string]string{
	"sync":      "sync-apps",
	"refresh":   "sync-apps",
	"validate":  "validate-environments",
	"correlate": "correlate-contexts",
	"inspect":   "inspect-manifests",
}

// resolveAction returns the actionSpec for a user-supplied action name,
// accepting both canonical names and aliases.
func resolveAction(name string) (actionSpec, bool) {
	if canonical, ok := actionAliases[name]; ok {
		name = canonical
	}
	spec, ok := canonicalActions[name]
	return spec, ok
}

// actionNames returns the sorted set of accepted action tokens (canonical
// names plus aliases) for help text and shell completion.
func actionNames() []string {
	seen := map[string]struct{}{}
	for k := range canonicalActions {
		seen[k] = struct{}{}
	}
	for k := range actionAliases {
		seen[k] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func newContextRunCommand(flags *GlobalFlags) *cobra.Command {
	var manifestType string
	cmd := &cobra.Command{
		Use:   "run NAME ACTION",
		Short: "Trigger a GitOps action on a context",
		Long: fmt.Sprintf(`Trigger a GitOps action on a context. The command publishes a
command message and returns a correlation id to track it.

Actions:
  sync-apps                    synchronize ArgoCD ApplicationSets (alias: sync, refresh)
  validate-environments        validate environment manifests / Vault secrets (alias: validate)
  correlate-contexts           correlate the app+environment pairing (alias: correlate)
  correlate-multi-environment  correlate workloads across environments
  inspect-manifests            inspect manifests (alias: inspect); use --type to scope

Accepted tokens: %s`, strings.Join(actionNames(), ", ")),
		Example: `  platformctl context run web-app-prod sync-apps
  platformctl context run web-app-prod validate
  platformctl context run web-app-prod inspect-manifests --type app`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Complete only the second positional (the action).
			if len(args) != 1 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return actionNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, action := args[0], args[1]
			spec, ok := resolveAction(action)
			if !ok {
				return fmt.Errorf("unknown action %q (accepted: %s)", action, strings.Join(actionNames(), ", "))
			}
			queryType := spec.queryType
			if cmd.Flags().Changed("type") {
				queryType = manifestType
			}

			client := NewClient(flags, cmd.ErrOrStderr())
			resp, err := client.RunAction(name, spec.endpoint, queryType)
			if err != nil {
				return err
			}
			return renderAction(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&manifestType, "type", "", "manifest type filter for inspect-manifests (app|environment|context|all)")
	return cmd
}
