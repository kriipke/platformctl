package events

import (
	"testing"

	"github.com/kriipke/platformctl/pkg/api"
)

// TestGenerateRoutingKey documents the command routing keys per manifest type.
// "kubernetes" routes to cmd.kubernetes.* (the multi-environment service's
// binding) and "git" falls through to cmd.manifest.* (why the customer-git-branch
// service binds cmd.manifest.*).
func TestGenerateRoutingKey(t *testing.T) {
	p := &GitOpsCommandPublisher{}

	tests := []struct {
		manifestType string
		action       string
		want         string
	}{
		{"app", "sync-app", "cmd.app.sync-app"},
		{"environment", "validate-environment", "cmd.environment.validate-environment"},
		{"context", "correlate-context", "cmd.context.correlate-context"},
		{"kubernetes", "correlate-context", "cmd.kubernetes.correlate-context"},
		{"git", "sync-customer-branch", "cmd.manifest.sync-customer-branch"},
	}

	for _, tt := range tests {
		cmd := &api.GitOpsCommandMessage{
			GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
				ManifestType: tt.manifestType,
				Action:       tt.action,
			},
		}
		if got := p.generateRoutingKey(cmd); got != tt.want {
			t.Errorf("generateRoutingKey(%s/%s) = %q, want %q", tt.manifestType, tt.action, got, tt.want)
		}
	}
}
