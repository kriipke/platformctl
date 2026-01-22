package config

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

type Config struct {
	Port         string        `env:"PORT" envDefault:":8080"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
	LogLevel     string        `env:"LOG_LEVEL" envDefault:"info"`

	DatabaseURL      string        `env:"DATABASE_URL" envDefault:"postgres://localhost/contextops?sslmode=disable"`
	MaxDBConnections int           `env:"MAX_DB_CONNECTIONS" envDefault:"25"`
	DBTimeout        time.Duration `env:"DB_TIMEOUT" envDefault:"10s"`

	ArgoCD      ArgoCDConfig      `envPrefix:"ARGOCD_"`
	Vault       VaultConfig       `envPrefix:"VAULT_"`
	Helm        HelmConfig        `envPrefix:"HELM_"`
	MultiTenant MultiTenantConfig `envPrefix:"TENANT_"`

	DevMode bool `env:"DEV_MODE" envDefault:"false"`
}

type ArgoCDConfig struct {
	Enabled   bool   `env:"ENABLED" envDefault:"true"`
	ServerURL string `env:"SERVER_URL" envDefault:"https://argocd.example.com"`
	AuthToken string `env:"AUTH_TOKEN"`
	Namespace string `env:"NAMESPACE" envDefault:"argocd"`
	Insecure  bool   `env:"INSECURE" envDefault:"false"`
}

type VaultConfig struct {
	Enabled   bool   `env:"ENABLED" envDefault:"true"`
	Address   string `env:"ADDRESS" envDefault:"https://vault.example.com"`
	AuthPath  string `env:"AUTH_PATH" envDefault:"kubernetes"`
	Role      string `env:"ROLE" envDefault:"contextops"`
	Namespace string `env:"NAMESPACE"`
}

type HelmConfig struct {
	Enabled     bool   `env:"ENABLED" envDefault:"true"`
	RegistryURL string `env:"REGISTRY_URL" envDefault:"https://charts.example.com"`
	Username    string `env:"USERNAME"`
	Password    string `env:"PASSWORD"`
}

type MultiTenantConfig struct {
	Enabled           bool          `env:"ENABLED" envDefault:"true"`
	DefaultCustomerID string        `env:"DEFAULT_CUSTOMER_ID" envDefault:"system"`
	IsolationMode     string        `env:"ISOLATION_MODE" envDefault:"strict"`
	CustomerHeader    string        `env:"CUSTOMER_HEADER" envDefault:"X-Customer-ID"`
	UserHeader        string        `env:"USER_HEADER" envDefault:"X-User-ID"`
	SessionTimeout    time.Duration `env:"SESSION_TIMEOUT" envDefault:"24h"`
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal("failed to parse config: ", err)
	}

	if err := validateGitOpsConfig(cfg); err != nil {
		log.Fatal("invalid GitOps configuration: ", err)
	}

	return cfg
}

func validateGitOpsConfig(cfg *Config) error {
	if cfg.ArgoCD.Enabled && cfg.ArgoCD.ServerURL == "" {
		return errors.New("ArgoCD server URL is required when ArgoCD is enabled")
	}

	if cfg.Vault.Enabled && cfg.Vault.Address == "" {
		return errors.New("Vault address is required when Vault is enabled")
	}

	if cfg.MultiTenant.Enabled && cfg.MultiTenant.IsolationMode != "strict" && cfg.MultiTenant.IsolationMode != "permissive" {
		return errors.New("tenant isolation mode must be 'strict' or 'permissive'")
	}

	return nil
}

func (cfg *Config) IsDevelopment() bool {
	return cfg.DevMode || cfg.LogLevel == "debug"
}

func (cfg *Config) GetDatabaseConfig() string {
	return fmt.Sprintf("%s?max_connections=%d&connect_timeout=%ds",
		cfg.DatabaseURL, cfg.MaxDBConnections, int(cfg.DBTimeout.Seconds()))
}
