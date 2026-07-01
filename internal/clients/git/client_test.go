package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kriipke/platformctl/pkg/api"
)

func TestEnvironmentFromValuesFile(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"values.yaml", ""},
		{"values-dev.yaml", "dev"},
		{"values-staging.yml", "staging"},
		{"values-prod.yaml", "prod"},
	}
	for _, tt := range tests {
		if got := environmentFromValuesFile(tt.name); got != tt.want {
			t.Errorf("environmentFromValuesFile(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestIsValuesFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"values.yaml", true},
		{"values-dev.yaml", true},
		{"values-prod.yml", true},
		{"Chart.yaml", false},
		{"readme.md", false},
		{"valuesx", false},
	}
	for _, tt := range tests {
		if got := isValuesFile(tt.name); got != tt.want {
			t.Errorf("isValuesFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCustomerBranchPattern(t *testing.T) {
	if got := customerBranchPattern("acme"); got != "customer/acme" {
		t.Errorf("customerBranchPattern = %q, want customer/acme", got)
	}
}

func TestEnvironmentValidations(t *testing.T) {
	files := []api.HelmValuesFile{
		{FileName: "values.yaml", Environment: "", IsValid: true},
		{FileName: "values-dev.yaml", Environment: "dev", IsValid: true},
		{FileName: "values-prod.yaml", Environment: "prod", IsValid: true},
	}
	got := environmentValidations(files)
	if len(got) != 2 {
		t.Fatalf("expected 2 environment validations (base values excluded), got %d", len(got))
	}
	if got[0].Environment != "dev" || got[1].Environment != "prod" {
		t.Errorf("unexpected environments: %q, %q", got[0].Environment, got[1].Environment)
	}
}

func TestHeadCommit(t *testing.T) {
	hash := plumbing.NewHash("abc123def4560000000000000000000000000000")
	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("feature"), plumbing.NewHash("1111111111111111111111111111111111111111")),
		plumbing.NewHashReference(plumbing.HEAD, hash),
	}
	if got := headCommit(refs); got != hash.String() {
		t.Errorf("headCommit = %q, want %q", got, hash.String())
	}

	if got := headCommit(nil); got != "" {
		t.Errorf("headCommit(nil) = %q, want empty", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "x", "y"); got != "x" {
		t.Errorf("firstNonEmpty = %q, want x", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty(empties) = %q, want empty", got)
	}
}
