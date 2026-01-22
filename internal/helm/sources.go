package helm

// SourceType enumerates supported Helm source types.
type SourceType string

const (
	SourceHelmRegistry SourceType = "helm-registry"
	SourceGit          SourceType = "git"
	SourceOCI          SourceType = "oci"
)

type Source struct {
	Type       SourceType `json:"type"`
	Registry   string     `json:"registry,omitempty"`
	Chart      string     `json:"chart,omitempty"`
	Version    string     `json:"version,omitempty"`
	Repository string     `json:"repository,omitempty"`
	Path       string     `json:"path,omitempty"`
	Ref        string     `json:"ref,omitempty"`
}
