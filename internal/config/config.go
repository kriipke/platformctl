package config

import (
	"errors"
	"log"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	"github.com/kriipke/platformctl/internal/observability"
)

type Config struct {
	// Server configuration
	Port         string        `env:"PORT" envDefault:":8080"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
	LogLevel     string        `env:"LOG_LEVEL" envDefault:"info"`

	// Gateway admin basic-auth credentials (used only by cmd/gateway). Sourced
	// from a Kubernetes Secret in real deploys. AdminPassword has NO default:
	// when it is empty the gateway fails closed (all /api/v1 routes return 503)
	// rather than falling back to a well-known password. Rotate by editing the
	// Secret and restarting the gateway (secretKeyRef env is read only at boot).
	AdminUser     string `env:"GATEWAY_ADMIN_USER" envDefault:"admin"`
	AdminPassword string `env:"GATEWAY_ADMIN_PASSWORD"`

	// Database configuration
	DatabaseURL      string        `env:"DATABASE_URL" envDefault:"postgres://localhost/platformctl?sslmode=disable"`
	MaxDBConnections int           `env:"MAX_DB_CONNECTIONS" envDefault:"25"`
	DBTimeout        time.Duration `env:"DB_TIMEOUT" envDefault:"10s"`

	// RabbitMQ configuration
	RabbitMQURL               string        `env:"RABBITMQ_URL" envDefault:"amqp://localhost:5672/"`
	RabbitMQConnectionRetries int           `env:"RABBITMQ_CONNECTION_RETRIES" envDefault:"5"`
	RabbitMQRetryDelay        time.Duration `env:"RABBITMQ_RETRY_DELAY" envDefault:"5s"`
	RabbitMQHeartbeat         time.Duration `env:"RABBITMQ_HEARTBEAT" envDefault:"10s"`

	// Observability configuration
	Observability ObservabilityConfig `envPrefix:"OBS_"`

	// GitOps integration configuration
	ArgoCD ArgoCDConfig `envPrefix:"ARGOCD_"`
	Vault  VaultConfig  `envPrefix:"VAULT_"`
	Helm   HelmConfig   `envPrefix:"HELM_"`

	// Multi-tenant configuration
	MultiTenant MultiTenantConfig `envPrefix:"TENANT_"`

	// Development configuration
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
	Role      string `env:"ROLE" envDefault:"platformctl"`
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
	IsolationMode     string        `env:"ISOLATION_MODE" envDefault:"strict"` // strict, permissive
	CustomerHeader    string        `env:"CUSTOMER_HEADER" envDefault:"X-Customer-ID"`
	UserHeader        string        `env:"USER_HEADER" envDefault:"X-User-ID"`
	SessionTimeout    time.Duration `env:"SESSION_TIMEOUT" envDefault:"24h"`
}

type ObservabilityConfig struct {
	// Logging configuration
	LogLevel         string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat        string `env:"LOG_FORMAT" envDefault:"json"`
	EnableConsoleLog bool   `env:"ENABLE_CONSOLE_LOG" envDefault:"false"`

	// Metrics configuration
	MetricsEnabled   bool   `env:"METRICS_ENABLED" envDefault:"true"`
	MetricsPort      string `env:"METRICS_PORT" envDefault:"9090"`
	MetricsPath      string `env:"METRICS_PATH" envDefault:"/metrics"`
	MetricsNamespace string `env:"METRICS_NAMESPACE" envDefault:"platformctl"`

	// Health check configuration
	HealthCheckPort     string        `env:"HEALTH_CHECK_PORT" envDefault:"8081"`
	ReadinessPath       string        `env:"READINESS_PATH" envDefault:"/ready"`
	LivenessPath        string        `env:"LIVENESS_PATH" envDefault:"/health"`
	HealthCheckInterval time.Duration `env:"HEALTH_CHECK_INTERVAL" envDefault:"30s"`
	HealthCheckTimeout  time.Duration `env:"HEALTH_CHECK_TIMEOUT" envDefault:"5s"`
	EnableDeepChecks    bool          `env:"ENABLE_DEEP_HEALTH_CHECKS" envDefault:"true"`

	// Correlation configuration
	CorrelationHeader string `env:"CORRELATION_HEADER" envDefault:"X-Correlation-ID"`

	// Tracing configuration (for future use)
	TracingEnabled bool   `env:"TRACING_ENABLED" envDefault:"false"`
	TracingURL     string `env:"TRACING_URL"`
}

func Load() *Config {
	// Load .env file if it exists (for development)
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal("failed to parse config: ", err)
	}

	// Validate critical GitOps configuration
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
		return errors.New("vault address is required when Vault is enabled")
	}

	if cfg.MultiTenant.Enabled && cfg.MultiTenant.IsolationMode != "strict" && cfg.MultiTenant.IsolationMode != "permissive" {
		return errors.New("tenant isolation mode must be 'strict' or 'permissive'")
	}

	if cfg.RabbitMQURL == "" {
		return errors.New("RabbitMQ URL is required")
	}

	return nil
}

func (cfg *Config) IsDevelopment() bool {
	return cfg.DevMode || cfg.LogLevel == "debug"
}

func (cfg *Config) GetDatabaseConfig() string {
	return cfg.DatabaseURL
}

func (cfg *Config) GetRabbitMQConfig() RabbitMQConfig {
	return RabbitMQConfig{
		URL:               cfg.RabbitMQURL,
		ConnectionRetries: cfg.RabbitMQConnectionRetries,
		RetryDelay:        cfg.RabbitMQRetryDelay,
		Heartbeat:         cfg.RabbitMQHeartbeat,
	}
}

type RabbitMQConfig struct {
	URL               string
	ConnectionRetries int
	RetryDelay        time.Duration
	Heartbeat         time.Duration
}

// Convenience methods for observability configuration access

func (cfg *Config) GetLoggerConfig(serviceName string) LoggerConfig {
	return LoggerConfig{
		Level:         cfg.Observability.LogLevel,
		Format:        cfg.Observability.LogFormat,
		ServiceName:   serviceName,
		EnableConsole: cfg.Observability.EnableConsoleLog,
	}
}

func (cfg *Config) GetMetricsConfig(serviceName string) MetricsConfig {
	return MetricsConfig{
		Enabled:     cfg.Observability.MetricsEnabled,
		Port:        cfg.Observability.MetricsPort,
		Path:        cfg.Observability.MetricsPath,
		ServiceName: serviceName,
		Namespace:   cfg.Observability.MetricsNamespace,
	}
}

func (cfg *Config) GetHealthCheckConfig() observability.HealthCheckConfig {
	return observability.HealthCheckConfig{
		Port:             cfg.Observability.HealthCheckPort,
		ReadinessPath:    cfg.Observability.ReadinessPath,
		LivenessPath:     cfg.Observability.LivenessPath,
		CheckInterval:    cfg.Observability.HealthCheckInterval,
		CheckTimeout:     cfg.Observability.HealthCheckTimeout,
		EnableDeepChecks: cfg.Observability.EnableDeepChecks,
	}
}

func (cfg *Config) GetCorrelationHeader() string {
	return cfg.Observability.CorrelationHeader
}

// Configuration structs for observability components

type LoggerConfig struct {
	Level         string
	Format        string
	ServiceName   string
	EnableConsole bool
}

type MetricsConfig struct {
	Enabled     bool
	Port        string
	Path        string
	ServiceName string
	Namespace   string
}
