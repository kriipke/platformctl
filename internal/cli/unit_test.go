package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kriipke/platformctl/internal/models"
)

func TestResolveAction(t *testing.T) {
	cases := map[string]string{
		"sync-apps": "sync-apps",
		"sync":      "sync-apps",
		"refresh":   "sync-apps",
		"validate":  "validate-environments",
		"correlate": "correlate-contexts",
		"inspect":   "inspect-manifests",
	}
	for in, wantEndpoint := range cases {
		spec, ok := resolveAction(in)
		if !ok {
			t.Errorf("resolveAction(%q) not found", in)
			continue
		}
		if spec.endpoint != wantEndpoint {
			t.Errorf("resolveAction(%q).endpoint = %q, want %q", in, spec.endpoint, wantEndpoint)
		}
	}
	if _, ok := resolveAction("bogus"); ok {
		t.Errorf("resolveAction(bogus) should be unknown")
	}
}

func TestRenderContextListTable(t *testing.T) {
	var buf bytes.Buffer
	contexts := []models.Context{sampleContext("web-app-prod")}
	if err := renderContextList(&buf, "table", contexts); err != nil {
		t.Fatalf("renderContextList: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"NAME", "web-app-prod", "web-app", "prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q; got:\n%s", want, out)
		}
	}
}

func TestRenderContextListEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := renderContextList(&buf, "table", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No contexts found") {
		t.Errorf("want empty message, got %q", buf.String())
	}
}

func TestRenderContextListJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := renderContextList(&buf, "json", []models.Context{sampleContext("x")}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\"apiVersion\": \"platformctl/v1\"") {
		t.Errorf("json output missing apiVersion; got:\n%s", buf.String())
	}
}

func TestLoadFileConfigMissingDefaultIsOK(t *testing.T) {
	// An empty explicit path with no home file present should not error; simulate
	// by pointing HOME at an empty temp dir.
	t.Setenv("HOME", t.TempDir())
	fc, err := loadFileConfig("")
	if err != nil {
		t.Fatalf("loadFileConfig: %v", err)
	}
	if fc.Server != "" {
		t.Errorf("expected zero config, got %+v", fc)
	}
}

func TestLoadFileConfigExplicitMissingErrors(t *testing.T) {
	_, err := loadFileConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing explicit config file")
	}
}

func TestLoadFileConfigParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("server: https://gw.example.com\noutput: json\ncustomerId: acme\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fc, err := loadFileConfig(path)
	if err != nil {
		t.Fatalf("loadFileConfig: %v", err)
	}
	if fc.Server != "https://gw.example.com" || fc.Output != "json" || fc.CustomerID != "acme" {
		t.Errorf("parsed config = %+v", fc)
	}
}
