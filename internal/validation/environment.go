package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/contextops/platformctl/internal/models"
)

// ValidateEnvironment validates an Environment manifest
func ValidateEnvironment(env *models.Environment) error {
	if err := validateEnvironmentMetadata(&env.Metadata); err != nil {
		return fmt.Errorf("metadata validation failed: %w", err)
	}

	if err := validateEnvironmentSpec(&env.Spec); err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}

	return nil
}

// validateEnvironmentMetadata validates environment metadata
func validateEnvironmentMetadata(metadata *models.EnvironmentMetadata) error {
	if metadata.Name == "" {
		return errors.New("name is required")
	}

	// Validate DNS-compliant name
	if !isValidDNSName(metadata.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	return nil
}

// validateEnvironmentSpec validates environment specification
func validateEnvironmentSpec(spec *models.EnvironmentSpec) error {
	// Validate environment config
	if err := validateEnvironmentConfig(&spec.Environment); err != nil {
		return fmt.Errorf("environment config validation failed: %w", err)
	}

	// Validate Helm config
	if err := validateEnvironmentHelmConfig(&spec.Helm); err != nil {
		return fmt.Errorf("helm config validation failed: %w", err)
	}

	// Validate Vault config
	if err := validateEnvironmentVaultConfig(&spec.Vault); err != nil {
		return fmt.Errorf("vault config validation failed: %w", err)
	}

	// Validate datasources
	if len(spec.Datasources) == 0 {
		return errors.New("at least one datasource is required")
	}

	for name, datasource := range spec.Datasources {
		if err := validateVaultDatasource(&datasource); err != nil {
			return fmt.Errorf("datasource[%s] validation failed: %w", name, err)
		}
	}

	// Validate vault secrets
	if len(spec.VaultSecrets) == 0 {
		return errors.New("at least one vault secret is required")
	}

	for i, secret := range spec.VaultSecrets {
		if err := validateVaultStaticSecret(&secret); err != nil {
			return fmt.Errorf("vaultSecrets[%d] validation failed: %w", i, err)
		}
	}

	// Validate pod env validation config
	if err := validatePodEnvValidationConfig(&spec.PodEnvValidation); err != nil {
		return fmt.Errorf("podEnvValidation validation failed: %w", err)
	}

	return nil
}

// validateEnvironmentConfig validates environment configuration
func validateEnvironmentConfig(config *models.EnvironmentConfig) error {
	if config.Name == "" {
		return errors.New("name is required")
	}

	if config.Namespace == "" {
		return errors.New("namespace is required")
	}

	if !isValidDNSName(config.Namespace) {
		return errors.New("namespace must be a valid DNS-1123 label")
	}

	// Validate cluster config
	if err := validateClusterConfig(&config.Cluster); err != nil {
		return fmt.Errorf("cluster config validation failed: %w", err)
	}

	return nil
}

// validateClusterConfig validates cluster configuration
func validateClusterConfig(config *models.ClusterConfig) error {
	if err := validateVaultSecretRef(&config.KubeconfigSecretRef); err != nil {
		return fmt.Errorf("kubeconfigSecretRef validation failed: %w", err)
	}

	return nil
}

// validateVaultSecretRef validates vault secret reference
func validateVaultSecretRef(ref *models.VaultSecretRef) error {
	if ref.Vault == "" {
		return errors.New("vault path is required")
	}

	if !isValidVaultPath(ref.Vault) {
		return errors.New("vault path must be a valid vault path")
	}

	if ref.Key == "" {
		return errors.New("key is required")
	}

	return nil
}

// validateEnvironmentHelmConfig validates environment Helm configuration
func validateEnvironmentHelmConfig(config *models.EnvironmentHelmConfig) error {
	return validateHelmValuesSource(&config.ValuesSource)
}

// validateHelmValuesSource validates Helm values source
func validateHelmValuesSource(source *models.HelmValuesSource) error {
	if source.Type != "git" {
		return errors.New("type must be 'git'")
	}

	if source.Repository == "" {
		return errors.New("repository is required")
	}

	if !isValidURL(source.Repository) {
		return errors.New("repository must be a valid URL")
	}

	if source.Path == "" {
		return errors.New("path is required")
	}

	if source.Branch == "" {
		return errors.New("branch is required")
	}

	return nil
}

// validateEnvironmentVaultConfig validates environment Vault configuration
func validateEnvironmentVaultConfig(config *models.EnvironmentVaultConfig) error {
	if config.Address == "" {
		return errors.New("address is required")
	}

	if !isValidURL(config.Address) {
		return errors.New("address must be a valid URL")
	}

	return validateVaultAuthConfig(&config.Auth)
}

// validateVaultAuthConfig validates Vault authentication configuration
func validateVaultAuthConfig(config *models.VaultAuthConfig) error {
	validMethods := []string{"kubernetes", "token"}
	if !contains(validMethods, config.Method) {
		return fmt.Errorf("method must be one of: %s", strings.Join(validMethods, ", "))
	}

	switch config.Method {
	case "token":
		if config.Token == "" {
			return errors.New("token is required for token auth method")
		}
	case "kubernetes":
		if config.Kubernetes == nil {
			return errors.New("kubernetes configuration is required for kubernetes auth method")
		}
		if config.Kubernetes.Role == "" {
			return errors.New("kubernetes.role is required")
		}
	}

	return nil
}

// validateVaultDatasource validates vault datasource
func validateVaultDatasource(datasource *models.VaultDatasource) error {
	if datasource.Vault == "" {
		return errors.New("vault path is required")
	}

	if !isValidVaultPath(datasource.Vault) {
		return errors.New("vault path must be a valid vault path")
	}

	if len(datasource.Keys) == 0 {
		return errors.New("at least one key is required")
	}

	return nil
}

// validateVaultStaticSecret validates vault static secret
func validateVaultStaticSecret(secret *models.VaultStaticSecret) error {
	if secret.Name == "" {
		return errors.New("name is required")
	}

	if !isValidDNSName(secret.Name) {
		return errors.New("name must be a valid DNS-1123 label")
	}

	if secret.VaultPath == "" {
		return errors.New("vaultPath is required")
	}

	if !isValidVaultPath(secret.VaultPath) {
		return errors.New("vaultPath must be a valid vault path")
	}

	if secret.DestinationSecret == "" {
		return errors.New("destinationSecret is required")
	}

	if !isValidDNSName(secret.DestinationSecret) {
		return errors.New("destinationSecret must be a valid DNS-1123 label")
	}

	if len(secret.RequiredKeys) == 0 {
		return errors.New("at least one required key is specified")
	}

	return nil
}

// validatePodEnvValidationConfig validates pod environment validation configuration
func validatePodEnvValidationConfig(config *models.PodEnvValidationConfig) error {
	if config.Enabled {
		for i, envVar := range config.ExpectedEnvVars {
			if err := validateExpectedEnvVar(&envVar); err != nil {
				return fmt.Errorf("expectedEnvVars[%d] validation failed: %w", i, err)
			}
		}
	}

	return nil
}

// validateExpectedEnvVar validates expected environment variable
func validateExpectedEnvVar(envVar *models.ExpectedEnvVar) error {
	if envVar.Name == "" {
		return errors.New("name is required")
	}

	if envVar.SecretRef == "" {
		return errors.New("secretRef is required")
	}

	if envVar.Key == "" {
		return errors.New("key is required")
	}

	return nil
}

// isValidVaultPath validates a Vault path
func isValidVaultPath(path string) bool {
	// Basic vault path validation - should start with a mount path
	if len(path) == 0 {
		return false
	}
	
	// Vault paths should not start with / and should contain at least one /
	if strings.HasPrefix(path, "/") {
		return false
	}
	
	if !strings.Contains(path, "/") {
		return false
	}
	
	return true
}