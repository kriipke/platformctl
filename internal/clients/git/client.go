package git

import (
	"fmt"
	"time"

	"github.com/contextops/platformctl/pkg/api"
)

type GitClient interface {
	ValidateValuesFileExists(gitRepo, filePath string) (bool, error)
	GetFileContent(gitRepo, filePath string) ([]byte, error)
	GetLastModifiedTime(gitRepo, filePath string) (time.Time, error)
	ValidateCustomerBranch(customerID, branch, repositoryURL string) (*api.CustomerBranchValidation, error)
	ValidateGitSources(customerID, appName string) ([]api.GitSourceStatus, error)
}

type GitClientImpl struct{}

func NewGitClient() *GitClientImpl {
	return &GitClientImpl{}
}

func (gc *GitClientImpl) ValidateValuesFileExists(gitRepo, filePath string) (bool, error) {
	// For Phase 1C, simulate file existence check
	// In a real implementation, this would:
	// 1. Clone or access the Git repository
	// 2. Check if the file exists at the specified path
	// 3. Return existence status

	// Simulate that common files exist
	commonFiles := []string{
		"values.yaml",
		"values-dev.yaml",
		"values-staging.yaml",
		"values-prod.yaml",
		"helm/values.yaml",
	}

	for _, file := range commonFiles {
		if file == filePath {
			return true, nil
		}
	}

	return false, nil
}

func (gc *GitClientImpl) GetFileContent(gitRepo, filePath string) ([]byte, error) {
	// For Phase 1C, simulate file content retrieval
	// In a real implementation, this would:
	// 1. Access the Git repository
	// 2. Read the file content
	// 3. Return the raw bytes

	// Simulate YAML content based on file type
	if filePath == "values.yaml" || filePath == "helm/values.yaml" {
		return []byte(`
image:
  repository: myapp
  tag: "latest"
  pullPolicy: IfNotPresent

replicas: 3

service:
  type: ClusterIP
  port: 80

ingress:
  enabled: false

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 250m
    memory: 256Mi
`), nil
	}

	if filePath == "values-dev.yaml" {
		return []byte(`
image:
  tag: "dev-latest"

replicas: 1

ingress:
  enabled: true
  hosts:
    - host: myapp-dev.example.com
      paths: ["/"]
`), nil
	}

	return []byte{}, fmt.Errorf("file not found: %s", filePath)
}

func (gc *GitClientImpl) GetLastModifiedTime(gitRepo, filePath string) (time.Time, error) {
	// For Phase 1C, simulate last modified time
	// In a real implementation, this would:
	// 1. Get the Git commit history for the file
	// 2. Return the timestamp of the last commit that modified the file

	// Simulate different modification times
	switch filePath {
	case "values.yaml", "helm/values.yaml":
		return time.Now().Add(-48 * time.Hour), nil
	case "values-dev.yaml":
		return time.Now().Add(-24 * time.Hour), nil
	case "values-staging.yaml":
		return time.Now().Add(-72 * time.Hour), nil
	case "values-prod.yaml":
		return time.Now().Add(-168 * time.Hour), nil
	default:
		return time.Now().Add(-24 * time.Hour), nil
	}
}

func (gc *GitClientImpl) ValidateCustomerBranch(customerID, branch, repositoryURL string) (*api.CustomerBranchValidation, error) {
	// For Phase 1C, simulate customer branch validation
	// In a real implementation, this would:
	// 1. Check if the branch exists in the repository
	// 2. Validate that it follows the customer branch pattern
	// 3. Get branch metadata and recent commits

	expectedPattern := fmt.Sprintf("customer/%s", customerID)
	patternCompliant := (branch == expectedPattern)

	validation := &api.CustomerBranchValidation{
		CustomerID:       customerID,
		BranchName:       branch,
		BranchPattern:    expectedPattern,
		PatternCompliant: patternCompliant,
		BranchExists:     true, // Simulate that branch exists
		LastCommit: &api.GitCommit{
			SHA:       "abc123def456",
			Message:   "Update application configuration",
			Author:    "customer-user",
			Timestamp: time.Now().Add(-6 * time.Hour),
		},
		HelmValuesFiles: []api.HelmValuesFile{
			{
				FileName:     "values-dev.yaml",
				FilePath:     "helm/values-dev.yaml",
				Environment:  "dev",
				FileSize:     1024,
				LastModified: time.Now().Add(-24 * time.Hour),
				SHA:          "def456ghi789",
				IsValid:      true,
				Errors:       []string{},
			},
			{
				FileName:     "values-prod.yaml",
				FilePath:     "helm/values-prod.yaml",
				Environment:  "prod",
				FileSize:     2048,
				LastModified: time.Now().Add(-168 * time.Hour),
				SHA:          "ghi789jkl012",
				IsValid:      true,
				Errors:       []string{},
			},
		},
		EnvironmentFiles: []api.EnvironmentValuesValidation{
			{
				Environment:      "dev",
				ExpectedFileName: "values-dev.yaml",
				ActualFileName:   "values-dev.yaml",
				FileExists:       true,
				FileValid:        true,
				ValidationErrors: []string{},
				RequiredKeys:     []string{"image.repository", "image.tag", "replicas"},
				MissingKeys:      []string{},
			},
			{
				Environment:      "prod",
				ExpectedFileName: "values-prod.yaml",
				ActualFileName:   "values-prod.yaml",
				FileExists:       true,
				FileValid:        true,
				ValidationErrors: []string{},
				RequiredKeys:     []string{"image.repository", "image.tag", "replicas"},
				MissingKeys:      []string{},
			},
		},
		ValidationStatus: "valid",
	}

	if !patternCompliant {
		validation.ValidationStatus = "invalid"
		validation.ErrorMessage = fmt.Sprintf("Branch name '%s' does not follow pattern '%s'", branch, expectedPattern)
	}

	return validation, nil
}

func (gc *GitClientImpl) ValidateGitSources(customerID, appName string) ([]api.GitSourceStatus, error) {
	var sources []api.GitSourceStatus

	// Simulate Git source validation for the app
	source := api.GitSourceStatus{
		URL:         fmt.Sprintf("https://github.com/%s/%s", customerID, appName),
		Path:        "helm",
		Revision:    "main",
		Status:      "available",
		LastCommit:  "abc123def456",
		LastChecked: timePtr(time.Now().Add(-10 * time.Minute)),
	}

	sources = append(sources, source)
	return sources, nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}