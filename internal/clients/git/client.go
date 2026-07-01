package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	gogit "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/kriipke/platformctl/internal/circuitbreaker"
	"github.com/kriipke/platformctl/internal/clients/resilience"
	"github.com/kriipke/platformctl/pkg/api"
)

type GitClient interface {
	ValidateValuesFileExists(gitRepo, filePath string) (bool, error)
	GetFileContent(gitRepo, filePath string) ([]byte, error)
	GetLastModifiedTime(gitRepo, filePath string) (time.Time, error)
	ValidateCustomerBranch(customerID, branch, repositoryURL string) (*api.CustomerBranchValidation, error)
	ValidateGitSources(customerID, appName string) ([]api.GitSourceStatus, error)
}

// GitClientImpl talks to remote Git repositories using go-git. External calls
// are routed through a circuit breaker. A token (GIT_TOKEN or GITHUB_TOKEN) is
// used for HTTP basic auth against private repositories when present.
type GitClientImpl struct {
	token string
	cb    *circuitbreaker.CircuitBreaker
}

func NewGitClient() *GitClientImpl {
	return &GitClientImpl{
		token: firstNonEmpty(os.Getenv("GIT_TOKEN"), os.Getenv("GITHUB_TOKEN")),
		cb:    resilience.New("git"),
	}
}

func (gc *GitClientImpl) ValidateValuesFileExists(gitRepo, filePath string) (bool, error) {
	return resilience.Run(gc.cb, func() (bool, error) {
		return gc.validateValuesFileExists(gitRepo, filePath)
	})
}

func (gc *GitClientImpl) GetFileContent(gitRepo, filePath string) ([]byte, error) {
	return resilience.Run(gc.cb, func() ([]byte, error) {
		return gc.getFileContent(gitRepo, filePath)
	})
}

func (gc *GitClientImpl) GetLastModifiedTime(gitRepo, filePath string) (time.Time, error) {
	return resilience.Run(gc.cb, func() (time.Time, error) {
		return gc.getLastModifiedTime(gitRepo, filePath)
	})
}

func (gc *GitClientImpl) ValidateCustomerBranch(customerID, branch, repositoryURL string) (*api.CustomerBranchValidation, error) {
	return resilience.Run(gc.cb, func() (*api.CustomerBranchValidation, error) {
		return gc.validateCustomerBranch(customerID, branch, repositoryURL)
	})
}

func (gc *GitClientImpl) ValidateGitSources(customerID, appName string) ([]api.GitSourceStatus, error) {
	return resilience.Run(gc.cb, func() ([]api.GitSourceStatus, error) {
		return gc.validateGitSources(customerID, appName)
	})
}

// --- real implementations ---

func (gc *GitClientImpl) validateValuesFileExists(gitRepo, filePath string) (bool, error) {
	_, fs, err := gc.clone(gitRepo, "", false)
	if err != nil {
		return false, fmt.Errorf("clone %s: %w", gitRepo, err)
	}
	if _, err := fs.Stat(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (gc *GitClientImpl) getFileContent(gitRepo, filePath string) ([]byte, error) {
	_, fs, err := gc.clone(gitRepo, "", false)
	if err != nil {
		return nil, fmt.Errorf("clone %s: %w", gitRepo, err)
	}
	f, err := fs.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (gc *GitClientImpl) getLastModifiedTime(gitRepo, filePath string) (time.Time, error) {
	// Full history is required to find the last commit that touched the file.
	repo, _, err := gc.clone(gitRepo, "", true)
	if err != nil {
		return time.Time{}, fmt.Errorf("clone %s: %w", gitRepo, err)
	}
	iter, err := repo.Log(&gogit.LogOptions{FileName: &filePath})
	if err != nil {
		return time.Time{}, err
	}
	defer iter.Close()
	commit, err := iter.Next()
	if err != nil {
		return time.Time{}, fmt.Errorf("no history for %s: %w", filePath, err)
	}
	return commit.Author.When, nil
}

func (gc *GitClientImpl) validateCustomerBranch(customerID, branch, repositoryURL string) (*api.CustomerBranchValidation, error) {
	expectedPattern := customerBranchPattern(customerID)
	validation := &api.CustomerBranchValidation{
		CustomerID:       customerID,
		BranchName:       branch,
		BranchPattern:    expectedPattern,
		PatternCompliant: branch == expectedPattern,
		HelmValuesFiles:  []api.HelmValuesFile{},
		EnvironmentFiles: []api.EnvironmentValuesValidation{},
	}

	refs, err := gc.lsRemote(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("ls-remote %s: %w", repositoryURL, err)
	}

	branchRef := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == branchRef {
			validation.BranchExists = true
			break
		}
	}

	if !validation.BranchExists {
		validation.ValidationStatus = "invalid"
		validation.ErrorMessage = fmt.Sprintf("branch %q not found in %s", branch, repositoryURL)
		return validation, nil
	}

	repo, fs, err := gc.clone(repositoryURL, branchRef, true)
	if err != nil {
		return nil, fmt.Errorf("clone branch %s: %w", branch, err)
	}

	if head, err := repo.Head(); err == nil {
		if c, err := repo.CommitObject(head.Hash()); err == nil {
			validation.LastCommit = &api.GitCommit{
				SHA:       c.Hash.String(),
				Message:   strings.TrimSpace(c.Message),
				Author:    c.Author.Name,
				Timestamp: c.Author.When,
			}
		}
	}

	validation.HelmValuesFiles = collectValuesFiles(fs)
	validation.EnvironmentFiles = environmentValidations(validation.HelmValuesFiles)

	if validation.PatternCompliant {
		validation.ValidationStatus = "valid"
	} else {
		validation.ValidationStatus = "invalid"
		validation.ErrorMessage = fmt.Sprintf("branch %q does not follow pattern %q", branch, expectedPattern)
	}

	return validation, nil
}

func (gc *GitClientImpl) validateGitSources(customerID, appName string) ([]api.GitSourceStatus, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s", customerID, appName)
	now := time.Now()

	refs, err := gc.lsRemote(repoURL)
	if err != nil {
		return nil, fmt.Errorf("ls-remote %s: %w", repoURL, err)
	}

	source := api.GitSourceStatus{
		URL:         repoURL,
		Path:        "helm",
		Revision:    "main",
		Status:      "available",
		LastCommit:  headCommit(refs),
		LastChecked: &now,
	}
	return []api.GitSourceStatus{source}, nil
}

// --- go-git helpers ---

func (gc *GitClientImpl) auth() transport.AuthMethod {
	if gc.token == "" {
		return nil
	}
	// Username is ignored by GitHub for token auth but must be non-empty.
	return &githttp.BasicAuth{Username: "git", Password: gc.token}
}

// clone performs an in-memory clone. When ref is non-empty only that branch is
// fetched; when full is false the clone is shallow (depth 1).
func (gc *GitClientImpl) clone(repoURL string, ref plumbing.ReferenceName, full bool) (*gogit.Repository, billy.Filesystem, error) {
	fs := memfs.New()
	opts := &gogit.CloneOptions{
		URL:  repoURL,
		Auth: gc.auth(),
	}
	if ref != "" {
		opts.ReferenceName = ref
		opts.SingleBranch = true
	}
	if !full {
		opts.Depth = 1
	}
	repo, err := gogit.Clone(memory.NewStorage(), fs, opts)
	if err != nil {
		return nil, nil, err
	}
	return repo, fs, nil
}

func (gc *GitClientImpl) lsRemote(repoURL string) ([]*plumbing.Reference, error) {
	remote := gogit.NewRemote(memory.NewStorage(), &gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})
	return remote.List(&gogit.ListOptions{Auth: gc.auth()})
}

func collectValuesFiles(fs billy.Filesystem) []api.HelmValuesFile {
	out := []api.HelmValuesFile{}
	for _, dir := range []string{".", "helm"} {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !isValuesFile(e.Name()) {
				continue
			}
			path := e.Name()
			if dir != "." {
				path = dir + "/" + e.Name()
			}
			out = append(out, api.HelmValuesFile{
				FileName:     e.Name(),
				FilePath:     path,
				Environment:  environmentFromValuesFile(e.Name()),
				FileSize:     e.Size(),
				LastModified: e.ModTime(),
				IsValid:      true,
				Errors:       []string{},
			})
		}
	}
	return out
}

func headCommit(refs []*plumbing.Reference) string {
	candidates := []plumbing.ReferenceName{
		plumbing.HEAD,
		plumbing.NewBranchReferenceName("main"),
		plumbing.Master,
	}
	for _, want := range candidates {
		for _, ref := range refs {
			if ref.Name() == want && !ref.Hash().IsZero() {
				return ref.Hash().String()
			}
		}
	}
	return ""
}

// --- pure helpers (unit tested) ---

func customerBranchPattern(customerID string) string {
	return fmt.Sprintf("customer/%s", customerID)
}

func isValuesFile(name string) bool {
	return strings.HasPrefix(name, "values") &&
		(strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml"))
}

// environmentFromValuesFile maps a values file name to its environment.
// "values-dev.yaml" -> "dev"; "values.yaml" -> "" (base values).
func environmentFromValuesFile(name string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
	if base == "values" {
		return ""
	}
	return strings.TrimPrefix(base, "values-")
}

func environmentValidations(files []api.HelmValuesFile) []api.EnvironmentValuesValidation {
	out := []api.EnvironmentValuesValidation{}
	for _, f := range files {
		if f.Environment == "" {
			continue
		}
		out = append(out, api.EnvironmentValuesValidation{
			Environment:      f.Environment,
			ExpectedFileName: f.FileName,
			ActualFileName:   f.FileName,
			FileExists:       true,
			FileValid:        f.IsValid,
			ValidationErrors: []string{},
			RequiredKeys:     []string{},
			MissingKeys:      []string{},
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
