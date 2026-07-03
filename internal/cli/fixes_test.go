package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseBoolLenient(t *testing.T) {
	truthy := []string{"1", "t", "true", "TRUE", "y", "Yes", "on", " on "}
	for _, s := range truthy {
		if v, ok := parseBoolLenient(s); !ok || !v {
			t.Errorf("parseBoolLenient(%q) = (%v,%v), want (true,true)", s, v, ok)
		}
	}
	falsy := []string{"0", "f", "false", "n", "no", "off", "OFF"}
	for _, s := range falsy {
		if v, ok := parseBoolLenient(s); !ok || v {
			t.Errorf("parseBoolLenient(%q) = (%v,%v), want (false,true)", s, v, ok)
		}
	}
	for _, s := range []string{"maybe", "2", ""} {
		if _, ok := parseBoolLenient(s); ok {
			t.Errorf("parseBoolLenient(%q) should be unrecognized", s)
		}
	}
}

func TestResolveBoolEnvPrecedenceAndError(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("insecure", false, "")

	// env "off" overrides fileVal=true -> false, no error
	v, err := resolveBool(cmd, "insecure", "PLATFORMCTL_INSECURE", "off", false, true)
	if err != nil || v {
		t.Errorf("resolveBool(off, file=true) = (%v,%v), want (false,nil)", v, err)
	}
	// unrecognized env value is a hard error (no silent drop)
	if _, err := resolveBool(cmd, "insecure", "PLATFORMCTL_INSECURE", "garbage", false, true); err == nil {
		t.Errorf("resolveBool(garbage) should return an error")
	}
	// empty env falls through to fileVal
	v, err = resolveBool(cmd, "insecure", "PLATFORMCTL_INSECURE", "", false, true)
	if err != nil || !v {
		t.Errorf("resolveBool(empty, file=true) = (%v,%v), want (true,nil)", v, err)
	}
}

// A manifest that omits metadata.name should decode fine and only fail
// validation until the name is injected (the `update NAME` path).
func TestDecodeThenNameInjection(t *testing.T) {
	manifest := `
apiVersion: platformctl/v1
kind: Context
spec:
  appRef: web-app
  deployments:
    - environment: prod
      appRef: web-app
      environmentRef: web-app-prod-env
      active: true
  gitops:
    customerBranch:
      enabled: false
    monitoring:
      applicationSets: true
`
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(manifest))

	ctx, err := decodeContextManifest(cmd, "-")
	if err != nil {
		t.Fatalf("decodeContextManifest: %v", err)
	}
	if ctx.Metadata.Name != "" {
		t.Fatalf("expected empty name, got %q", ctx.Metadata.Name)
	}
	// Without a name, validation must fail...
	if err := validateManifest(ctx); err == nil {
		t.Errorf("expected validation error for missing name")
	}
	// ...but injecting the NAME argument makes it valid (the update flow).
	ctx.Metadata.Name = "web-app-prod"
	if err := validateManifest(ctx); err != nil {
		t.Errorf("after name injection, validateManifest failed: %v", err)
	}
}

// Watch mode with a non-positive interval must return a clean error, not panic.
func TestStatusWatchRejectsNonPositiveInterval(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // avoid picking up a real ~/.platformctl.yaml
	root := NewRootCommand(BuildInfo{Version: "test"})
	root.SetArgs([]string{"context", "status", "foo", "--watch", "--interval", "0"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error for --interval 0")
	}
	if !strings.Contains(err.Error(), "interval must be positive") {
		t.Errorf("error = %v, want interval-positive message", err)
	}
}
