# PHASE 1A: Core Foundation

**Duration:** 3-4 days  
**Prerequisites:** Go 1.21+, PostgreSQL 15+, Git  
**Deliverable:** Basic API Gateway with Context CRUD operations and database persistence

---

## Overview

Establish the foundational components of ContextOps: core data models, database schema, API Gateway skeleton, and basic Context CRUD operations. This phase creates the backbone that all other services will build upon.

## Success Criteria

✅ Context model implemented with full validation  
✅ PostgreSQL schema created and migrations working  
✅ API Gateway serving Context CRUD endpoints  
✅ Basic authentication middleware in place  
✅ All endpoints return proper JSON responses  
✅ Unit tests for models and validation logic  
✅ Basic integration test for Context CRUD flow  

---

## Implementation Tasks

### Task 1: Project Structure Setup

Create the foundational directory structure:

```bash
mkdir -p {cmd/gateway,internal/{contexts,storage,auth},pkg/{api,schemas}}
cd /path/to/contextops
```

**Files to create:**
- `go.mod` - Go module definition
- `cmd/gateway/main.go` - API Gateway entry point
- `internal/contexts/models.go` - Context domain models
- `internal/contexts/validation.go` - Context validation logic
- `internal/storage/postgres.go` - Database connection and queries
- `pkg/api/types.go` - Shared API types and DTOs
- `pkg/schemas/context.go` - JSON schema validation
- `migrations/001_initial_schema.up.sql` - Database schema
- `migrations/001_initial_schema.down.sql` - Schema rollback

### Task 2: Context Domain Model

**File: `internal/contexts/models.go`**

Implement the Context model based on the YAML schema from README.md:

```go
// Core Context struct with all nested types
type Context struct {
    APIVersion string             `json:"apiVersion" validate:"required,eq=contextops/v1"`
    Kind       string             `json:"kind" validate:"required,eq=Context"`
    Metadata   ContextMetadata    `json:"metadata" validate:"required"`
    Spec       ContextSpec        `json:"spec" validate:"required"`
}

type ContextMetadata struct {
    Name string `json:"name" validate:"required,contextname"`
}

type ContextSpec struct {
    App    AppConfig    `json:"app" validate:"required"`
    Policy PolicyConfig `json:"policy" validate:"required"`
    // ... all other config sections
}
```

**Key Implementation Points:**
- Use struct tags for JSON marshaling and validation
- Implement custom validation for context names (`^[a-z0-9-]+$`)
- Add helper methods for common operations
- Ensure all secret fields are references, never values

### Task 3: Database Schema and Persistence

**File: `migrations/001_initial_schema.up.sql`**

```sql
-- Contexts table with JSONB for flexible schema evolution
CREATE TABLE contexts (
    name VARCHAR(255) PRIMARY KEY,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_contexts_app_env ON contexts USING GIN ((spec->'app'));
CREATE INDEX idx_contexts_updated_at ON contexts (updated_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for automatic updated_at
CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE ON contexts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**File: `internal/storage/postgres.go`**

```go
type ContextStore struct {
    db *sql.DB
}

func NewContextStore(db *sql.DB) *ContextStore {
    return &ContextStore{db: db}
}

func (s *ContextStore) Create(ctx context.Context, context *contexts.Context) error
func (s *ContextStore) Get(ctx context.Context, name string) (*contexts.Context, error)
func (s *ContextStore) List(ctx context.Context) ([]*contexts.Context, error)
func (s *ContextStore) Update(ctx context.Context, context *contexts.Context) error
func (s *ContextStore) Delete(ctx context.Context, name string) error
```

**Implementation Notes:**
- Use JSONB for context specs to support schema evolution
- Implement proper connection pooling
- Add context timeout handling
- Use transactions for multi-step operations

### Task 4: Context Validation

**File: `internal/contexts/validation.go`**

Implement comprehensive validation:

```go
func ValidateContext(ctx *Context) error {
    // 1. Struct tag validation using validator package
    if err := validator.New().Struct(ctx); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // 2. Business rule validation
    if err := validateBusinessRules(ctx); err != nil {
        return fmt.Errorf("business rule validation failed: %w", err)
    }
    
    // 3. Secret reference validation
    if err := validateSecretReferences(ctx); err != nil {
        return fmt.Errorf("secret validation failed: %w", err)
    }
    
    return nil
}

// Custom validation functions
func validateContextName(name string) bool
func validateSecretReferences(ctx *Context) error
func validateKubernetesPolicy(policy *PolicyConfig) error
```

### Task 5: API Gateway Foundation

**File: `cmd/gateway/main.go`**

```go
func main() {
    // Configuration loading
    cfg := loadConfig()
    
    // Database connection
    db := setupDatabase(cfg.DatabaseURL)
    defer db.Close()
    
    // Dependencies
    contextStore := storage.NewContextStore(db)
    contextHandler := handlers.NewContextHandler(contextStore)
    
    // Router setup
    router := setupRouter(contextHandler)
    
    // Server startup
    server := &http.Server{
        Addr:    cfg.Port,
        Handler: router,
    }
    
    log.Fatal(server.ListenAndServe())
}
```

**API Endpoints to implement:**

```go
// File: internal/handlers/context.go
func (h *ContextHandler) CreateContext(w http.ResponseWriter, r *http.Request)
func (h *ContextHandler) GetContext(w http.ResponseWriter, r *http.Request)
func (h *ContextHandler) ListContexts(w http.ResponseWriter, r *http.Request)
func (h *ContextHandler) UpdateContext(w http.ResponseWriter, r *http.Request)
func (h *ContextHandler) DeleteContext(w http.ResponseWriter, r *http.Request)
```

**Routes:**
- `POST /contexts` → CreateContext
- `GET /contexts` → ListContexts  
- `GET /contexts/{name}` → GetContext
- `PUT /contexts/{name}` → UpdateContext
- `DELETE /contexts/{name}` → DeleteContext

### Task 6: Basic Authentication Middleware

**File: `internal/auth/middleware.go`**

Implement placeholder authentication for now:

```go
func BasicAuthMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // For Phase 1A: Allow all requests
            // TODO: Implement proper authentication in Phase 1B
            
            // Set basic user context
            ctx := r.Context()
            ctx = context.WithValue(ctx, "user", "system")
            
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Task 7: Configuration Management

**File: `internal/config/config.go`**

```go
type Config struct {
    Port        string `env:"PORT" envDefault:":8080"`
    DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://localhost/contextops?sslmode=disable"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
}

func Load() *Config {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        log.Fatal("failed to parse config: ", err)
    }
    return cfg
}
```

---

## Testing Requirements

### Unit Tests

**File: `internal/contexts/models_test.go`**
- Test Context struct marshaling/unmarshaling
- Test validation rules for each field
- Test custom validation functions

**File: `internal/contexts/validation_test.go`**
- Test all validation scenarios (valid/invalid contexts)
- Test edge cases and boundary conditions
- Test custom validator functions

**File: `internal/storage/postgres_test.go`**
- Test all CRUD operations
- Test error conditions (duplicate names, not found, etc.)
- Test database connection handling

### Integration Tests

**File: `cmd/gateway/integration_test.go`**
- Test complete Context CRUD flow via HTTP endpoints
- Test JSON request/response formatting
- Test error responses and status codes
- Test concurrent access scenarios

---

## Dependencies

Add to `go.mod`:
```
require (
    github.com/gorilla/mux v1.8.0
    github.com/lib/pq v1.10.9
    github.com/go-playground/validator/v10 v10.15.5
    github.com/caarlos0/env/v9 v9.0.0
    github.com/golang-migrate/migrate/v4 v4.16.2
)
```

---

## Validation Checklist

Before marking Phase 1A complete:

- [ ] `go build ./cmd/gateway` compiles successfully
- [ ] Database migrations run without errors
- [ ] All unit tests pass: `go test ./internal/...`
- [ ] Integration tests pass: `go test ./cmd/gateway/...`
- [ ] Can create, read, update, delete contexts via API endpoints
- [ ] Invalid context specs are properly rejected with meaningful errors
- [ ] All endpoints return proper HTTP status codes
- [ ] JSON responses are well-formatted and consistent
- [ ] Database persists context data correctly
- [ ] Context name validation works (`^[a-z0-9-]+$`)
- [ ] Secret validation prevents inline secret values

---

## Next Steps

Upon completion, Phase 1A provides:
- Working Context CRUD API
- Robust data validation 
- Database persistence
- Foundation for message publishing (Phase 1B)

**Handoff to Phase 1B:** The API Gateway can accept and validate contexts. Next phase will add RabbitMQ integration for command publishing and result consumption.