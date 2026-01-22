package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/contextops/platformctl/internal/models"
)

// ValidateContext validates a Context manifest
func ValidateContext(context *models.Context) error {
	if err := validateContextMetadata(&context.Metadata); err != nil {
		return fmt.Errorf("metadata validation failed: %w", err)
	}

	if err := validateContextSpec(&context.Spec); err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}

	return nil
}

// validateContextMetadata validates context metadata
func validateContextMetadata(metadata *models.ContextMetadata) error {
	if metadata.Name == "" {
		return errors.New("name is required")
	}

	// Validate DNS-compliant name
	if !isValidDNSName(metadata.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	return nil
}

// validateContextSpec validates context specification
func validateContextSpec(spec *models.ContextSpec) error {
	if spec.AppRef == "" {
		return errors.New("appRef is required")
	}

	// Validate deployments
	if len(spec.Deployments) == 0 {
		return errors.New("at least one deployment is required")
	}

	// Check that at least one deployment is active
	hasActiveDeployment := false
	for i, deployment := range spec.Deployments {
		if err := validateContextDeployment(&deployment); err != nil {
			return fmt.Errorf("deployments[%d] validation failed: %w", i, err)
		}
		if deployment.Active {
			hasActiveDeployment = true
		}
	}

	if !hasActiveDeployment {
		return errors.New("at least one deployment must be active")
	}

	// Validate GitOps config
	if err := validateContextGitOpsConfig(&spec.GitOps); err != nil {
		return fmt.Errorf("gitops config validation failed: %w", err)
	}

	return nil
}

// validateContextDeployment validates context deployment
func validateContextDeployment(deployment *models.ContextDeployment) error {
	if deployment.Environment == "" {
		return errors.New("environment is required")
	}

	if deployment.AppRef == "" {
		return errors.New("appRef is required")
	}

	if deployment.EnvironmentRef == "" {
		return errors.New("environmentRef is required")
	}

	// Validate environment names
	validEnvironments := []string{"dev", "qa", "uat", "prod", "staging", "production", "development", "testing"}
	if !containsIgnoreCase(validEnvironments, deployment.Environment) {
		return fmt.Errorf("environment '%s' is not a recognized environment name", deployment.Environment)
	}

	return nil
}

// validateContextGitOpsConfig validates GitOps configuration
func validateContextGitOpsConfig(config *models.ContextGitOpsConfig) error {
	// Validate customer branch config
	if err := validateCustomerBranchConfig(&config.CustomerBranch); err != nil {
		return fmt.Errorf("customer branch config validation failed: %w", err)
	}

	// Validate monitoring config
	if err := validateMonitoringConfig(&config.Monitoring); err != nil {
		return fmt.Errorf("monitoring config validation failed: %w", err)
	}

	return nil
}

// validateCustomerBranchConfig validates customer branch configuration
func validateCustomerBranchConfig(config *models.CustomerBranchConfig) error {
	if config.Enabled {
		if config.Branch == "" {
			return errors.New("branch is required when enabled")
		}

		if !isValidCustomerBranch(config.Branch) {
			return errors.New("branch must follow customer branch pattern (customer/{customer-name})")
		}
	}

	return nil
}

// validateMonitoringConfig validates monitoring configuration
func validateMonitoringConfig(config *models.MonitoringConfig) error {
	// No specific validation needed for monitoring config - all fields are optional booleans
	return nil
}

// isValidCustomerBranch validates customer branch pattern
func isValidCustomerBranch(branch string) bool {
	// Customer branch pattern: customer/{customer-name}
	customerBranchRegex := regexp.MustCompile(`^customer/[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`)
	return customerBranchRegex.MatchString(branch)
}

// containsIgnoreCase checks if a slice contains a string (case insensitive)
func containsIgnoreCase(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}