package events

import (
	"testing"

	"github.com/contextops/platformctl/pkg/api"
)

// TestResultRoutingKey verifies the gitops.results routing keys produced for
// service results. The aggregator queue binds evt.app.* / evt.environment.* /
// evt.context.*, so app/environment/context results must use those namespaces;
// other manifest types are namespaced consistently but are not currently
// consumed by the aggregator.
func TestResultRoutingKey(t *testing.T) {
	tests := []struct {
		name         string
		manifestType string
		action       string
		want         string
	}{
		{"app", "app", "sync-app", "evt.app.sync-app"},
		{"environment", "environment", "validate-environment", "evt.environment.validate-environment"},
		{"context", "context", "correlate-context", "evt.context.correlate-context"},
		{"app inspection", "app", "inspect-manifests", "evt.app.inspect-manifests"},
		{"git", "git", "sync-customer-branch", "evt.git.sync-customer-branch"},
		{"empty type defaults to manifest", "", "inspect-manifests", "evt.manifest.inspect-manifests"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &api.GitOpsResultMessage{
				GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
					ManifestType: tt.manifestType,
					Action:       tt.action,
				},
			}
			if got := resultRoutingKey(result); got != tt.want {
				t.Errorf("resultRoutingKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
