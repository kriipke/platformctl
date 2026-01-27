package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/contextops/platformctl/pkg/api"
)

// ManifestFile represents a Kubernetes manifest file
type ManifestFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Filename string `json:"filename"`
}

type GitClient interface {
	FetchManifests(ctx context.Context, repoURL, branch, path string) ([]ManifestFile, error)
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

// FetchManifests clones a repository and extracts Kubernetes manifest files from the specified path
func (gc *GitClientImpl) FetchManifests(ctx context.Context, repoURL, branch, path string) ([]ManifestFile, error) {
	// Create an in-memory repository to avoid disk I/O
	storage := memory.NewStorage()
	
	// Clone the repository
	repo, err := git.CloneContext(ctx, storage, nil, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
		SingleBranch:  true,
		Depth:         1, // Shallow clone for better performance
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s: %w", repoURL, err)
	}

	// Get the repository worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the filesystem from the worktree
	fs := worktree.Filesystem

	var manifests []ManifestFile

	// Walk through the specified path to find manifest files
	err = gc.walkPath(fs, path, "", &manifests)
	if err != nil {
		return nil, fmt.Errorf("failed to walk path %s: %w", path, err)
	}

	return manifests, nil
}

// walkPath recursively walks through the filesystem to find Kubernetes manifest files
func (gc *GitClientImpl) walkPath(fs billy.Filesystem, rootPath, currentPath string, manifests *[]ManifestFile) error {
	// Combine root path with current path
	fullPath := filepath.Join(rootPath, currentPath)
	if fullPath == "." || fullPath == rootPath {
		fullPath = rootPath
	}

	// List files in the current directory
	files, err := fs.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist, return empty result
			return nil
		}
		return fmt.Errorf("failed to read directory %s: %w", fullPath, err)
	}

	for _, file := range files {
		filePath := filepath.Join(fullPath, file.Name())
		
		if file.IsDir() {
			// Recursively walk subdirectories
			err := gc.walkPath(fs, rootPath, filepath.Join(currentPath, file.Name()), manifests)
			if err != nil {
				return err
			}
		} else if gc.isKubernetesManifest(file.Name()) {
			// Read manifest file content
			content, err := gc.readFile(fs, filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			// Add to manifests list
			*manifests = append(*manifests, ManifestFile{
				Path:     filePath,
				Content:  content,
				Filename: file.Name(),
			})
		}
	}

	return nil
}

// isKubernetesManifest checks if a file is likely a Kubernetes manifest
func (gc *GitClientImpl) isKubernetesManifest(filename string) bool {
	// Check file extensions
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		return true
	}
	
	// Could add more sophisticated detection logic here
	// For example, checking file content for apiVersion and kind fields
	return false
}

// readFile reads the content of a file from the filesystem
func (gc *GitClientImpl) readFile(fs billy.Filesystem, path string) (string, error) {
	file, err := fs.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}