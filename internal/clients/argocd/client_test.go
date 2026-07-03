package argocd

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestRestBaseURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"argocd-server.argocd.svc.cluster.local", "https://argocd-server.argocd.svc.cluster.local"},
		{"https://argocd.example.com", "https://argocd.example.com"},
		{"http://argocd.example.com/", "http://argocd.example.com"},
		{"argocd.example.com:443", "https://argocd.example.com:443"},
		{"https://argocd.example.com:8080/", "https://argocd.example.com:8080"},
	}
	for _, tt := range tests {
		if got := restBaseURL(tt.in); got != tt.want {
			t.Errorf("restBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGeneratorType(t *testing.T) {
	tests := []struct {
		name string
		gen  v1alpha1.ApplicationSetGenerator
		want string
	}{
		{"git", v1alpha1.ApplicationSetGenerator{Git: &v1alpha1.GitGenerator{}}, "git"},
		{"clusters", v1alpha1.ApplicationSetGenerator{Clusters: &v1alpha1.ClusterGenerator{}}, "clusters"},
		{"list", v1alpha1.ApplicationSetGenerator{List: &v1alpha1.ListGenerator{}}, "list"},
		{"empty", v1alpha1.ApplicationSetGenerator{}, "unknown"},
	}
	for _, tt := range tests {
		if got := generatorType(tt.gen); got != tt.want {
			t.Errorf("generatorType(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAppSetSyncStatus(t *testing.T) {
	healthy := &v1alpha1.ApplicationSet{
		Status: v1alpha1.ApplicationSetStatus{
			ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{Application: "a", Status: "Healthy"},
				{Application: "b", Status: "Synced"},
			},
		},
	}
	if got := appSetSyncStatus(healthy); got != "synced" {
		t.Errorf("appSetSyncStatus(healthy) = %q, want synced", got)
	}

	degraded := &v1alpha1.ApplicationSet{
		Status: v1alpha1.ApplicationSetStatus{
			ApplicationStatus: []v1alpha1.ApplicationSetApplicationStatus{
				{Application: "a", Status: "Progressing"},
			},
		},
	}
	if got := appSetSyncStatus(degraded); got != "out_of_sync" {
		t.Errorf("appSetSyncStatus(degraded) = %q, want out_of_sync", got)
	}

	empty := &v1alpha1.ApplicationSet{}
	if got := appSetSyncStatus(empty); got != "unknown" {
		t.Errorf("appSetSyncStatus(empty) = %q, want unknown", got)
	}
}
