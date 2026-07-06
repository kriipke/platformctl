package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kriipke/platformctl/internal/models"
)

// ValidateApp validates an App manifest
func ValidateApp(app *models.App) error {
	if err := validateAppMetadata(&app.Metadata); err != nil {
		return fmt.Errorf("metadata validation failed: %w", err)
	}

	if err := validateAppSpec(&app.Spec); err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}

	return nil
}

// validateAppMetadata validates app metadata
func validateAppMetadata(metadata *models.AppMetadata) error {
	if metadata.Name == "" {
		return errors.New("name is required")
	}

	// Validate DNS-compliant name
	if !isValidDNSName(metadata.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	return nil
}

// validateAppSpec validates app specification
func validateAppSpec(spec *models.AppSpec) error {
	// Validate application config
	if err := validateAppApplicationConfig(&spec.Application); err != nil {
		return fmt.Errorf("application config validation failed: %w", err)
	}

	// Validate Helm config
	if err := validateAppHelmConfig(&spec.Helm); err != nil {
		return fmt.Errorf("helm config validation failed: %w", err)
	}

	// Validate ArgoCD config
	if err := validateAppArgoCDConfig(&spec.ArgoCD); err != nil {
		return fmt.Errorf("argoCD config validation failed: %w", err)
	}

	// Validate environments
	if len(spec.Environments) == 0 {
		return errors.New("at least one environment reference is required")
	}

	for i, env := range spec.Environments {
		if err := validateAppEnvironmentRef(&env); err != nil {
			return fmt.Errorf("environment[%d] validation failed: %w", i, err)
		}
	}

	return nil
}

// validateAppApplicationConfig validates application configuration
func validateAppApplicationConfig(config *models.AppApplicationConfig) error {
	if config.Name == "" {
		return errors.New("name is required")
	}

	if !isValidDNSName(config.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	if config.Version == "" {
		return errors.New("version is required")
	}

	if !isValidSemver(config.Version) {
		return errors.New("version must be a valid semantic version")
	}

	if config.Maintainer == "" {
		return errors.New("maintainer is required")
	}

	if !isValidEmail(config.Maintainer) {
		return errors.New("maintainer must be a valid email address")
	}

	return nil
}

// validateAppHelmConfig validates Helm configuration
func validateAppHelmConfig(config *models.AppHelmConfig) error {
	if len(config.Sources) == 0 {
		return errors.New("at least one Helm source is required")
	}

	if config.DefaultSource >= len(config.Sources) {
		return errors.New("defaultSource index is out of range")
	}

	for i, source := range config.Sources {
		if err := validateHelmSource(&source); err != nil {
			return fmt.Errorf("source[%d] validation failed: %w", i, err)
		}
	}

	return nil
}

// validateHelmSource validates a Helm source
func validateHelmSource(source *models.HelmSource) error {
	validTypes := []string{"helm-registry", "git", "oci"}
	if !contains(validTypes, source.Type) {
		return fmt.Errorf("type must be one of: %s", strings.Join(validTypes, ", "))
	}

	if source.Chart == "" {
		return errors.New("chart is required")
	}

	// Type-specific validation
	switch source.Type {
	case "helm-registry":
		if source.Registry == "" {
			return errors.New("registry is required for helm-registry type")
		}
	case "git":
		if source.Repository == "" {
			return errors.New("repository is required for git type")
		}
		if !isValidURL(source.Repository) {
			return errors.New("repository must be a valid URL")
		}
	case "oci":
		if source.Registry == "" {
			return errors.New("registry is required for oci type")
		}
	}

	return nil
}

// validateAppArgoCDConfig validates ArgoCD configuration
func validateAppArgoCDConfig(config *models.AppArgoCDConfig) error {
	if len(config.ApplicationSets) == 0 {
		return errors.New("at least one ApplicationSet is required")
	}

	for i, appSet := range config.ApplicationSets {
		if err := validateApplicationSetConfig(&appSet); err != nil {
			return fmt.Errorf("applicationSets[%d] validation failed: %w", i, err)
		}
	}

	if config.BootstrapApplication != nil {
		if err := validateBootstrapApplicationConfig(config.BootstrapApplication); err != nil {
			return fmt.Errorf("bootstrapApplication validation failed: %w", err)
		}
	}

	return nil
}

// validateApplicationSetConfig validates ApplicationSet configuration
func validateApplicationSetConfig(config *models.ApplicationSetConfig) error {
	if config.Name == "" {
		return errors.New("name is required")
	}

	if !isValidDNSName(config.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	if config.Namespace == "" {
		return errors.New("namespace is required")
	}

	if !isValidDNSName(config.Namespace) {
		return errors.New("namespace must be a valid DNS-1123 label")
	}

	if err := validateApplicationSetGenerator(&config.Generator); err != nil {
		return fmt.Errorf("generator validation failed: %w", err)
	}

	if err := validateApplicationSetTemplate(&config.Template); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	return nil
}

// validateApplicationSetGenerator validates ApplicationSet generator
func validateApplicationSetGenerator(generator *models.ApplicationSetGenerator) error {
	validTypes := []string{"git", "clusters", "list"}
	if !contains(validTypes, generator.Type) {
		return fmt.Errorf("type must be one of: %s", strings.Join(validTypes, ", "))
	}

	switch generator.Type {
	case "git":
		if generator.Git == nil {
			return errors.New("git generator configuration is required")
		}
		if err := validateGitGenerator(generator.Git); err != nil {
			return fmt.Errorf("git generator validation failed: %w", err)
		}
	case "list":
		if generator.List == nil {
			return errors.New("list generator configuration is required")
		}
		if len(generator.List.Elements) == 0 {
			return errors.New("list generator must have at least one element")
		}
	case "clusters":
		if generator.Clusters == nil {
			return errors.New("clusters generator configuration is required")
		}
	}

	return nil
}

// validateGitGenerator validates Git generator configuration
func validateGitGenerator(git *models.GitGenerator) error {
	if git.RepoURL == "" {
		return errors.New("repoURL is required")
	}

	if !isValidURL(git.RepoURL) {
		return errors.New("repoURL must be a valid URL")
	}

	if git.Revision == "" {
		return errors.New("revision is required")
	}

	return nil
}

// validateApplicationSetTemplate validates ApplicationSet template
func validateApplicationSetTemplate(template *models.ApplicationSetTemplate) error {
	if template.Metadata.Name == "" {
		return errors.New("metadata.name is required")
	}

	return nil
}

// validateBootstrapApplicationConfig validates bootstrap application configuration
func validateBootstrapApplicationConfig(config *models.BootstrapApplicationConfig) error {
	if config.Name == "" {
		return errors.New("name is required")
	}

	if !isValidDNSName(config.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	if config.Namespace == "" {
		return errors.New("namespace is required")
	}

	if !isValidDNSName(config.Namespace) {
		return errors.New("namespace must be a valid DNS-1123 label")
	}

	return nil
}

// validateAppEnvironmentRef validates environment reference
func validateAppEnvironmentRef(envRef *models.AppEnvironmentRef) error {
	if envRef.Name == "" {
		return errors.New("name is required")
	}

	if envRef.EnvironmentRef == "" {
		return errors.New("environmentRef is required")
	}

	return nil
}

// Helper validation functions
func isValidDNSName(name string) bool {
	// DNS-1123 label validation
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	dnsNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	return dnsNameRegex.MatchString(name)
}

func isValidSemver(version string) bool {
	// Basic semver validation
	semverRegex := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[a-zA-Z0-9\-.]+)?(\+[a-zA-Z0-9\-.]+)?$`)
	return semverRegex.MatchString(version)
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "git://")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
