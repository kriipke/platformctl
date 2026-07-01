package aggregator

import (
	"testing"

	"github.com/kriipke/platformctl/pkg/api"
)

// TestCustomerBranchFields verifies extraction of the customer-git-branch
// read-model fields from a result payload, including the metadata fallback for
// the branch and the compliance mapping.
func TestCustomerBranchFields(t *testing.T) {
	tests := []struct {
		name           string
		payload        map[string]interface{}
		metadataBranch string
		status         string
		wantRepo       string
		wantBranch     string
		wantCompliance string
	}{
		{
			name: "valid sync result is compliant",
			payload: map[string]interface{}{
				"repository_url":    "https://github.com/acme/configs",
				"customer_branch":   "customer/acme",
				"validation_status": "valid",
			},
			status:         "healthy",
			wantRepo:       "https://github.com/acme/configs",
			wantBranch:     "customer/acme",
			wantCompliance: "compliant",
		},
		{
			name: "error result is non-compliant",
			payload: map[string]interface{}{
				"repository_url":    "https://github.com/acme/configs",
				"customer_branch":   "customer/acme",
				"validation_status": "invalid",
			},
			status:         "error",
			wantRepo:       "https://github.com/acme/configs",
			wantBranch:     "customer/acme",
			wantCompliance: "non_compliant",
		},
		{
			name:           "branch falls back to metadata",
			payload:        map[string]interface{}{},
			metadataBranch: "customer/beta",
			status:         "healthy",
			wantRepo:       "",
			wantBranch:     "customer/beta",
			wantCompliance: "compliant",
		},
		{
			name:           "missing validation status is unknown",
			payload:        map[string]interface{}{"customer_branch": "customer/x"},
			status:         "degraded",
			wantRepo:       "",
			wantBranch:     "customer/x",
			wantCompliance: "unknown",
		},
		{
			name:           "nil payload uses metadata and is unknown",
			payload:        nil,
			metadataBranch: "customer/y",
			status:         "degraded",
			wantRepo:       "",
			wantBranch:     "customer/y",
			wantCompliance: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &api.GitOpsResultMessage{
				GitOpsMessageEnvelope: api.GitOpsMessageEnvelope{
					Payload:          tt.payload,
					ManifestMetadata: api.ManifestMetadata{CustomerBranch: tt.metadataBranch},
				},
				Status: tt.status,
			}

			repo, branch, compliance := customerBranchFields(result)
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", branch, tt.wantBranch)
			}
			if compliance != tt.wantCompliance {
				t.Errorf("compliance = %q, want %q", compliance, tt.wantCompliance)
			}
		})
	}
}
