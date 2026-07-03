package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kriipke/platformctl/internal/models"
	"sigs.k8s.io/yaml"
)

// outputJSON writes data as indented JSON.
func outputJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// outputYAML writes data as YAML (encoded through JSON tags).
func outputYAML(w io.Writer, data interface{}) error {
	raw, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}

// renderContextList renders a list of contexts as a table (default) or as
// JSON/YAML.
func renderContextList(w io.Writer, format string, contexts []models.Context) error {
	switch format {
	case "json":
		return outputJSON(w, contexts)
	case "yaml":
		return outputYAML(w, contexts)
	}

	if len(contexts) == 0 {
		fmt.Fprintln(w, "No contexts found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tAPP\tENVIRONMENTS\tCUSTOMER BRANCH\tCREATED")
	for _, c := range contexts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			dash(c.Metadata.Name),
			dash(c.Spec.AppRef),
			dash(deploymentEnvironments(c)),
			customerBranch(c),
			formatTimePtr(c.Metadata.CreatedAt),
		)
	}
	return tw.Flush()
}

// renderContext renders a single context. The table form prints a summary plus
// a deployments sub-table.
func renderContext(w io.Writer, format string, c models.Context) error {
	switch format {
	case "json":
		return outputJSON(w, c)
	case "yaml":
		return outputYAML(w, c)
	}

	fmt.Fprintf(w, "Name:            %s\n", dash(c.Metadata.Name))
	fmt.Fprintf(w, "App:             %s\n", dash(c.Spec.AppRef))
	fmt.Fprintf(w, "Customer Branch: %s\n", customerBranch(c))
	fmt.Fprintf(w, "Monitoring:      %s\n", dash(monitoringSummary(c.Spec.GitOps.Monitoring)))
	fmt.Fprintf(w, "Created:         %s\n", formatTimePtr(c.Metadata.CreatedAt))
	fmt.Fprintf(w, "Updated:         %s\n", formatTimePtr(c.Metadata.UpdatedAt))

	if len(c.Spec.Deployments) > 0 {
		fmt.Fprintln(w, "\nDeployments:")
		tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "  ENVIRONMENT\tAPP REF\tENVIRONMENT REF\tACTIVE")
		for _, d := range c.Spec.Deployments {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%t\n", dash(d.Environment), dash(d.AppRef), dash(d.EnvironmentRef), d.Active)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// renderCreateUpdate renders a create/update response. The table form just
// prints the server's message; JSON/YAML echo the full response.
func renderCreateUpdate(w io.Writer, format string, resp interface{}, message string) error {
	switch format {
	case "json":
		return outputJSON(w, resp)
	case "yaml":
		return outputYAML(w, resp)
	}
	if message == "" {
		message = "OK"
	}
	fmt.Fprintln(w, message)
	return nil
}

// renderAction renders a GitOps action response (a dynamic map).
func renderAction(w io.Writer, format string, resp map[string]interface{}) error {
	switch format {
	case "json":
		return outputJSON(w, resp)
	case "yaml":
		return outputYAML(w, resp)
	}

	if msg, ok := resp["message"].(string); ok && msg != "" {
		fmt.Fprintln(w, msg)
	}
	if cid, ok := resp["correlation_id"].(string); ok && cid != "" {
		fmt.Fprintf(w, "Correlation ID: %s\n", cid)
	}
	// Surface any string-slice fields (app_names, applicationsets, ...).
	for _, key := range sortedKeys(resp) {
		if items, ok := toStringSlice(resp[key]); ok && len(items) > 0 {
			fmt.Fprintf(w, "%s: %s\n", humanizeKey(key), strings.Join(items, ", "))
		}
	}
	return nil
}

// renderStatus renders a context status response (a dynamic map).
func renderStatus(w io.Writer, format, name string, status map[string]interface{}) error {
	switch format {
	case "json":
		return outputJSON(w, status)
	case "yaml":
		return outputYAML(w, status)
	}

	fmt.Fprintf(w, "Context: %s\n", name)
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, key := range sortedKeys(status) {
		fmt.Fprintf(tw, "  %s\t%s\n", humanizeKey(key), scalarString(status[key]))
	}
	return tw.Flush()
}

// --- helpers -------------------------------------------------------------

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func deploymentEnvironments(c models.Context) string {
	envs := make([]string, 0, len(c.Spec.Deployments))
	for _, d := range c.Spec.Deployments {
		env := d.Environment
		if !d.Active {
			env += " (inactive)"
		}
		envs = append(envs, env)
	}
	return strings.Join(envs, ", ")
}

func customerBranch(c models.Context) string {
	cb := c.Spec.GitOps.CustomerBranch
	if cb.Enabled && cb.Branch != "" {
		return cb.Branch
	}
	return "-"
}

func monitoringSummary(m models.MonitoringConfig) string {
	var on []string
	if m.ApplicationSets {
		on = append(on, "applicationSets")
	}
	if m.VaultSecrets {
		on = append(on, "vaultSecrets")
	}
	if m.HelmValues {
		on = append(on, "helmValues")
	}
	if m.CrossEnvironmentDrift {
		on = append(on, "crossEnvironmentDrift")
	}
	if len(on) == 0 {
		return ""
	}
	return strings.Join(on, ", ")
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// humanizeKey turns "context_name" / "correlationId" into "Context Name".
func humanizeKey(key string) string {
	key = strings.ReplaceAll(key, "_", " ")
	// Split camelCase boundaries into separate words.
	var b strings.Builder
	for i, r := range key {
		if i > 0 && r >= 'A' && r <= 'Z' && key[i-1] >= 'a' && key[i-1] <= 'z' {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	// Title-case each space-separated word (ASCII).
	words := strings.Fields(b.String())
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
	}
	return strings.Join(words, " ")
}

// scalarString renders a value for the status table. Scalars are shown inline;
// nested objects/arrays are shown as compact JSON.
func scalarString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "-"
	case string:
		return dash(t)
	case bool:
		return fmt.Sprintf("%t", t)
	case float64:
		// JSON numbers decode to float64; render integers without a decimal.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(raw)
	}
}

func toStringSlice(v interface{}) ([]string, bool) {
	items, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
