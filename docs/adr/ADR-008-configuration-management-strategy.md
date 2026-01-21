# ADR-008: Configuration Management Strategy

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 1B - APIs and Messaging Infrastructure  

---

## Context

ContextOps services require configuration for database connections, RabbitMQ URLs, external service endpoints, logging levels, and various operational parameters. A consistent configuration strategy is needed across all services while maintaining security, flexibility, and operational simplicity.

### Problem Statement

Configuration management challenges include:

1. **Security:** Database passwords, API keys, and other secrets mixed with regular config
2. **Environment parity:** Different configurations for dev, staging, production
3. **Service coordination:** Shared configuration values across multiple services  
4. **Operational flexibility:** Ability to change configuration without redeployment
5. **Validation:** Ensuring configuration correctness before service startup
6. **Observability:** Understanding current configuration state and changes

### Requirements

- **12-Factor compliance:** Configuration stored in environment variables
- **Secret separation:** Clear separation between configuration and secrets
- **Environment-specific:** Support for different environments without code changes
- **Validation:** Configuration validation at startup with clear error messages
- **Hot reload:** Ability to reload non-critical configuration without restart
- **Audit trail:** Track configuration changes for troubleshooting and compliance

---

## Decision

We will use **environment variables** as the primary configuration mechanism, with **structured validation** and **hierarchical defaults**, following 12-Factor App principles.

### Configuration Hierarchy
1. **Command-line flags** (highest priority)
2. **Environment variables**  
3. **Configuration files** (.env files for local development)
4. **Default values** (lowest priority)

### Implementation Strategy
```go
type Config struct {
    // Server configuration
    Port     string `env:"PORT" envDefault:":8080" validate:"required"`
    LogLevel string `env:"LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
    
    // Database configuration
    DatabaseURL      string `env:"DATABASE_URL" validate:"required,url"`
    DatabaseMaxConns int    `env:"DATABASE_MAX_CONNS" envDefault:"25" validate:"min=1,max=100"`
    
    // RabbitMQ configuration
    RabbitMQURL         string `env:"RABBITMQ_URL" validate:"required,url"`
    RabbitMQMaxRetries  int    `env:"RABBITMQ_MAX_RETRIES" envDefault:"3" validate:"min=1,max=10"`
    
    // External service endpoints
    VaultAddr    string `env:"VAULT_ADDR" validate:"omitempty,url"`
    ArgoCDAddr   string `env:"ARGOCD_ADDR" validate:"omitempty,url"`
    NewRelicAddr string `env:"NEWRELIC_ADDR" envDefault:"https://api.newrelic.com"`
    
    // Feature flags
    EnableMetrics     bool `env:"ENABLE_METRICS" envDefault:"true"`
    EnableTracing     bool `env:"ENABLE_TRACING" envDefault:"false"`
    EnableDebugMode   bool `env:"ENABLE_DEBUG_MODE" envDefault:"false"`
}
```

---

## Consequences

### Positive

1. **12-Factor Compliance**
   - Clear separation between code and configuration
   - Environment-specific configuration without code changes
   - Portable across deployment platforms

2. **Security**
   - Secrets managed separately from regular configuration
   - No secrets in code repositories or configuration files
   - Environment variable scoping provides isolation

3. **Operational Simplicity**
   - Standard environment variable patterns familiar to operators
   - Easy integration with container orchestration platforms
   - Simple configuration injection in CI/CD pipelines

### Negative

1. **Environment Variable Sprawl**
   - Large number of environment variables to manage
   - Potential for configuration drift between environments
   - Limited structure validation compared to configuration files

2. **Debugging Complexity**
   - Configuration values scattered across environment
   - Difficult to see complete configuration picture
   - Environment variable precedence can be confusing

---

## Implementation Guidelines

### Configuration Structure per Service
```go
// Service-specific configuration embedding common config
type GatewayConfig struct {
    Common    CommonConfig    `envPrefix:""`
    Auth      AuthConfig      `envPrefix:"AUTH_"`
    RateLimit RateLimitConfig `envPrefix:"RATE_LIMIT_"`
}

type CommonConfig struct {
    Port        string `env:"PORT" envDefault:":8080"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
    DatabaseURL string `env:"DATABASE_URL" validate:"required"`
    RabbitMQURL string `env:"RABBITMQ_URL" validate:"required"`
}
```

### Validation and Error Handling
```go
func LoadConfig() (*Config, error) {
    var cfg Config
    
    // Parse environment variables
    if err := env.Parse(&cfg); err != nil {
        return nil, fmt.Errorf("failed to parse configuration: %w", err)
    }
    
    // Validate configuration
    validate := validator.New()
    if err := validate.Struct(&cfg); err != nil {
        return nil, fmt.Errorf("configuration validation failed: %w", err)
    }
    
    // Post-process and validate complex rules
    if err := cfg.PostValidate(); err != nil {
        return nil, fmt.Errorf("configuration post-validation failed: %w", err)
    }
    
    return &cfg, nil
}
```

---

## Related ADRs

- ADR-003: Secrets posture - Defines how secrets are separated from configuration
- ADR-001: Event-driven integration workflows - Services need RabbitMQ configuration
- ADR-006: Circuit breaker and retry strategy - Circuit breaker settings configured via environment