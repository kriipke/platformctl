package vault

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/contextops/platformctl/internal/config"
	vaultapi "github.com/contextops/platformctl/pkg/api"
)

type VaultClient interface {
	ValidateVaultSources(customerID, environmentName string) ([]vaultapi.VaultValidationResult, error)
	ValidatePodEnvironmentVariables(customerID, environmentName string, vaultValidations []vaultapi.VaultValidationResult) ([]vaultapi.PodEnvValidationResult, error)
	GetSecretFromVaultSource(vaultPath, key string) (string, error)
}

type HashiCorpVaultClient struct {
	client *api.Client
	config *config.VaultConfig
}

func NewHashiCorpVaultClient(cfg config.VaultConfig) *HashiCorpVaultClient {
	config := api.DefaultConfig()
	if cfg.Address != "" {
		config.Address = cfg.Address
	}
	
	client, err := api.NewClient(config)
	if err != nil {
		log.Printf("Failed to create Vault client: %v", err)
		return &HashiCorpVaultClient{client: nil, config: &cfg}
	}

	return &HashiCorpVaultClient{
		client: client,
		config: &cfg,
	}
}

func (vc *HashiCorpVaultClient) ValidateVaultSources(customerID, environmentName string) ([]vaultapi.VaultValidationResult, error) {
	var validations []vaultapi.VaultValidationResult

	// For Phase 1C, we'll simulate Vault source validation
	// In a real implementation, this would:
	// 1. Get Environment manifest for the customer
	// 2. Extract VaultStaticSecret configurations
	// 3. Validate each secret path and required keys
	// 4. Check pod environment variable correlations

	// Simulate vault source validation
	validation := vaultapi.VaultValidationResult{
		VaultPath:         fmt.Sprintf("secret/%s/%s", customerID, environmentName),
		SecretName:        fmt.Sprintf("%s-secrets", environmentName),
		ValidationStatus:  "valid",
		MissingKeys:       []string{},
		ExtraKeys:         []string{},
		PodEnvValidations: []vaultapi.PodEnvValidationResult{},
		LastValidated:     time.Now().UTC(),
	}

	// Simulate some validation logic
	if vc.client != nil {
		// Try to authenticate and validate
		if err := vc.validateAuth(); err != nil {
			validation.ValidationStatus = "error"
			log.Printf("Vault auth failed: %v", err)
		} else {
			// Try to read a test secret
			_, err := vc.client.Logical().Read(validation.VaultPath)
			if err != nil {
				validation.ValidationStatus = "missing"
				log.Printf("Vault secret not found: %v", err)
			}
		}
	} else {
		validation.ValidationStatus = "error"
	}

	validations = append(validations, validation)
	return validations, nil
}

func (vc *HashiCorpVaultClient) ValidatePodEnvironmentVariables(customerID, environmentName string, vaultValidations []vaultapi.VaultValidationResult) ([]vaultapi.PodEnvValidationResult, error) {
	var podEnvValidations []vaultapi.PodEnvValidationResult

	// For Phase 1C, simulate pod environment variable validation
	// In a real implementation, this would:
	// 1. Get pods in the environment's namespace
	// 2. Check their environment variables
	// 3. Correlate with VaultStaticSecret configurations
	// 4. Validate that secrets are properly mounted

	for _, vaultValidation := range vaultValidations {
		podValidation := vaultapi.PodEnvValidationResult{
			PodName:          fmt.Sprintf("app-pod-%s", environmentName),
			Namespace:        fmt.Sprintf("%s-%s", customerID, environmentName),
			EnvVarName:       "DATABASE_URL",
			SecretRef:        vaultValidation.SecretName,
			SecretKey:        "database-url",
			ValidationStatus: "match",
			ErrorMessage:     "",
		}

		podEnvValidations = append(podEnvValidations, podValidation)
	}

	return podEnvValidations, nil
}

func (vc *HashiCorpVaultClient) GetSecretFromVaultSource(vaultPath, key string) (string, error) {
	if vc.client == nil {
		return "", fmt.Errorf("vault client not initialized")
	}

	resp, err := vc.client.Logical().Read(vaultPath)
	if err != nil {
		return "", fmt.Errorf("failed to read secret: %w", err)
	}

	if resp == nil {
		return "", fmt.Errorf("secret not found")
	}

	data := resp.Data
	if kv2Data, ok := data["data"].(map[string]interface{}); ok {
		data = kv2Data // Handle KV v2
	}

	if value, exists := data[key]; exists {
		if stringValue, ok := value.(string); ok {
			return stringValue, nil
		}
		return fmt.Sprintf("%v", value), nil
	}

	return "", fmt.Errorf("key %s not found in secret", key)
}

func (vc *HashiCorpVaultClient) validateAuth() error {
	if vc.client == nil {
		return fmt.Errorf("vault client not initialized")
	}

	// Set Vault address and namespace
	if vc.config.Address != "" {
		vc.client.SetAddress(vc.config.Address)
	}
	if vc.config.Namespace != "" {
		vc.client.SetNamespace(vc.config.Namespace)
	}

	switch vc.config.AuthPath {
	case "kubernetes":
		return vc.validateKubernetesAuth()
	case "token":
		return vc.validateTokenAuth()
	default:
		return fmt.Errorf("unsupported auth method: %s", vc.config.AuthPath)
	}
}

func (vc *HashiCorpVaultClient) validateKubernetesAuth() error {
	// Read service account token
	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		// In development, this file might not exist
		log.Printf("Warning: could not read service account token: %v", err)
		return nil // Don't fail in development
	}

	// Authenticate with Vault
	data := map[string]interface{}{
		"role": vc.config.Role,
		"jwt":  string(tokenBytes),
	}

	resp, err := vc.client.Logical().Write("auth/kubernetes/login", data)
	if err != nil {
		return fmt.Errorf("kubernetes auth failed: %w", err)
	}

	if resp.Auth == nil {
		return fmt.Errorf("no auth info returned")
	}

	// Set token for subsequent requests
	vc.client.SetToken(resp.Auth.ClientToken)
	return nil
}

func (vc *HashiCorpVaultClient) validateTokenAuth() error {
	// Token should be set via environment variable or config
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		return fmt.Errorf("VAULT_TOKEN not set")
	}

	vc.client.SetToken(token)
	
	// Validate token by making a test call
	_, err := vc.client.Auth().Token().LookupSelf()
	return err
}