# PHASE 1G: Foundation Testing

**Duration:** 2-3 days  
**Prerequisites:** Phase 1F completed  
**Deliverable:** Comprehensive test suite covering unit, integration, and system-level testing

---

## Overview

Implement a complete testing strategy with unit tests, integration tests, and end-to-end tests. Establish testing infrastructure, mocking strategies, and CI-friendly test execution.

## Success Criteria

✅ Unit tests for all core business logic  
✅ Integration tests for database and messaging  
✅ API endpoint tests covering all scenarios  
✅ Service integration tests with real external dependencies  
✅ End-to-end workflow tests  
✅ Test data management and cleanup  
✅ CI pipeline test execution  
✅ Code coverage reporting  

---

## Implementation Tasks

### Task 1: Test Infrastructure Setup

**File: `internal/testutil/database.go`**

```go
package testutil

import (
    "database/sql"
    "fmt"
    "os"
    "testing"
    
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/lib/pq"
)

// TestDatabase manages a test database instance
type TestDatabase struct {
    DB   *sql.DB
    Name string
}

// SetupTestDatabase creates a test database and runs migrations
func SetupTestDatabase(t *testing.T) *TestDatabase {
    if testing.Short() {
        t.Skip("Skipping database tests in short mode")
    }
    
    // Generate unique database name
    dbName := fmt.Sprintf("platformctl_test_%s", randomString(8))
    
    // Connect to postgres to create test database
    adminDB := connectToAdmin(t)
    defer adminDB.Close()
    
    _, err := adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(dbName)))
    if err != nil {
        t.Fatalf("Failed to create test database: %v", err)
    }
    
    // Connect to test database
    testDB := connectToTestDB(t, dbName)
    
    // Run migrations
    if err := runMigrations(testDB, dbName); err != nil {
        t.Fatalf("Failed to run migrations: %v", err)
    }
    
    testDatabase := &TestDatabase{
        DB:   testDB,
        Name: dbName,
    }
    
    // Register cleanup
    t.Cleanup(func() {
        testDatabase.Cleanup(t)
    })
    
    return testDatabase
}

func (td *TestDatabase) Cleanup(t *testing.T) {
    if td.DB != nil {
        td.DB.Close()
    }
    
    // Drop test database
    adminDB := connectToAdmin(t)
    defer adminDB.Close()
    
    _, err := adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pq.QuoteIdentifier(td.Name)))
    if err != nil {
        t.Logf("Warning: Failed to drop test database %s: %v", td.Name, err)
    }
}

func connectToAdmin(t *testing.T) *sql.DB {
    dbURL := getTestDatabaseURL()
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        t.Fatalf("Failed to connect to admin database: %v", err)
    }
    
    if err := db.Ping(); err != nil {
        t.Fatalf("Failed to ping admin database: %v", err)
    }
    
    return db
}

func connectToTestDB(t *testing.T, dbName string) *sql.DB {
    dbURL := getTestDatabaseURL() + "/" + dbName + "?sslmode=disable"
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        t.Fatalf("Failed to connect to test database: %v", err)
    }
    
    if err := db.Ping(); err != nil {
        t.Fatalf("Failed to ping test database: %v", err)
    }
    
    return db
}

func getTestDatabaseURL() string {
    dbURL := os.Getenv("TEST_DATABASE_URL")
    if dbURL == "" {
        dbURL = "postgres://postgres:password@localhost:5432"
    }
    return dbURL
}

func runMigrations(db *sql.DB, dbName string) error {
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        return fmt.Errorf("failed to create migration driver: %w", err)
    }
    
    m, err := migrate.NewWithDatabaseInstance(
        "file://migrations",
        dbName,
        driver,
    )
    if err != nil {
        return fmt.Errorf("failed to create migration instance: %w", err)
    }
    
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("failed to run migrations: %w", err)
    }
    
    return nil
}
```

**File: `internal/testutil/rabbitmq.go`**

```go
package testutil

import (
    "fmt"
    "os"
    "testing"
    "time"
    
    "github.com/streadway/amqp"
)

// TestRabbitMQ manages a test RabbitMQ connection
type TestRabbitMQ struct {
    Connection *amqp.Connection
    Channel    *amqp.Channel
    QueueNames []string
}

func SetupTestRabbitMQ(t *testing.T) *TestRabbitMQ {
    if testing.Short() {
        t.Skip("Skipping RabbitMQ tests in short mode")
    }
    
    rabbitURL := getTestRabbitMQURL()
    
    // Wait for RabbitMQ to be ready
    var conn *amqp.Connection
    var err error
    
    for i := 0; i < 10; i++ {
        conn, err = amqp.Dial(rabbitURL)
        if err == nil {
            break
        }
        time.Sleep(time.Second)
    }
    
    if err != nil {
        t.Fatalf("Failed to connect to test RabbitMQ: %v", err)
    }
    
    channel, err := conn.Channel()
    if err != nil {
        conn.Close()
        t.Fatalf("Failed to open RabbitMQ channel: %v", err)
    }
    
    testRabbit := &TestRabbitMQ{
        Connection: conn,
        Channel:    channel,
    }
    
    // Register cleanup
    t.Cleanup(func() {
        testRabbit.Cleanup(t)
    })
    
    return testRabbit
}

func (tr *TestRabbitMQ) CreateTestQueue(t *testing.T, name string) string {
    queueName := fmt.Sprintf("test_%s_%s", name, randomString(6))
    
    _, err := tr.Channel.QueueDeclare(
        queueName,
        false, // durable
        true,  // delete when unused
        false, // exclusive
        false, // no-wait
        nil,   // arguments
    )
    
    if err != nil {
        t.Fatalf("Failed to declare test queue: %v", err)
    }
    
    tr.QueueNames = append(tr.QueueNames, queueName)
    return queueName
}

func (tr *TestRabbitMQ) Cleanup(t *testing.T) {
    // Clean up queues
    for _, queueName := range tr.QueueNames {
        tr.Channel.QueueDelete(queueName, false, false, false)
    }
    
    if tr.Channel != nil {
        tr.Channel.Close()
    }
    
    if tr.Connection != nil {
        tr.Connection.Close()
    }
}

func getTestRabbitMQURL() string {
    rabbitURL := os.Getenv("TEST_RABBITMQ_URL")
    if rabbitURL == "" {
        rabbitURL = "amqp://guest:guest@localhost:5672/"
    }
    return rabbitURL
}
```

### Task 2: Unit Tests

**File: `internal/contexts/validation_test.go`**

```go
func TestValidateContext(t *testing.T) {
    tests := []struct {
        name    string
        context *Context
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid context",
            context: &Context{
                APIVersion: "platformctl/v1",
                Kind:       "Context",
                Metadata: ContextMetadata{
                    Name: "test-app-dev",
                },
                Spec: validContextSpec(),
            },
            wantErr: false,
        },
        {
            name: "invalid name format",
            context: &Context{
                APIVersion: "platformctl/v1",
                Kind:       "Context",
                Metadata: ContextMetadata{
                    Name: "Test_App_Dev", // invalid characters
                },
                Spec: validContextSpec(),
            },
            wantErr: true,
            errMsg:  "name validation",
        },
        {
            name: "inline secret forbidden",
            context: &Context{
                APIVersion: "platformctl/v1",
                Kind:       "Context",
                Metadata: ContextMetadata{
                    Name: "test-app-dev",
                },
                Spec: contextSpecWithInlineSecret(),
            },
            wantErr: true,
            errMsg:  "secret validation",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateContext(tt.context)
            
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestValidateContextName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
    }{
        {"valid lowercase", "test-app-dev", true},
        {"valid with numbers", "app1-prod", true},
        {"invalid uppercase", "Test-App", false},
        {"invalid underscore", "test_app", false},
        {"invalid special chars", "test@app", false},
        {"empty", "", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := validateContextName(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func validContextSpec() ContextSpec {
    return ContextSpec{
        App: AppConfig{
            Name:        "test-app",
            Environment: "dev",
        },
        Policy: PolicyConfig{
            AllowedActions: []string{"refresh", "validate"},
        },
        Vault: VaultConfig{
            Address:   "https://vault.example.com",
            Namespace: "test",
            Auth: VaultAuthConfig{
                Method: "token",
            },
        },
    }
}
```

**File: `internal/storage/postgres_test.go`**

```go
func TestContextStore_Create(t *testing.T) {
    testDB := testutil.SetupTestDatabase(t)
    store := storage.NewContextStore(testDB.DB)
    
    ctx := context.Background()
    testContext := createTestContext("test-app-dev")
    
    err := store.Create(ctx, testContext)
    assert.NoError(t, err)
    
    // Verify context was created
    retrieved, err := store.Get(ctx, "test-app-dev")
    assert.NoError(t, err)
    assert.Equal(t, testContext.Metadata.Name, retrieved.Metadata.Name)
    assert.Equal(t, testContext.Spec.App.Name, retrieved.Spec.App.Name)
}

func TestContextStore_Get_NotFound(t *testing.T) {
    testDB := testutil.SetupTestDatabase(t)
    store := storage.NewContextStore(testDB.DB)
    
    ctx := context.Background()
    
    _, err := store.Get(ctx, "nonexistent")
    assert.Error(t, err)
    assert.Equal(t, storage.ErrContextNotFound, err)
}

func TestContextStore_List(t *testing.T) {
    testDB := testutil.SetupTestDatabase(t)
    store := storage.NewContextStore(testDB.DB)
    
    ctx := context.Background()
    
    // Create test contexts
    contexts := []*Context{
        createTestContext("app1-dev"),
        createTestContext("app1-prod"),
        createTestContext("app2-dev"),
    }
    
    for _, c := range contexts {
        err := store.Create(ctx, c)
        assert.NoError(t, err)
    }
    
    // List contexts
    retrieved, err := store.List(ctx)
    assert.NoError(t, err)
    assert.Len(t, retrieved, 3)
    
    // Verify order (should be sorted by name)
    names := make([]string, len(retrieved))
    for i, c := range retrieved {
        names[i] = c.Metadata.Name
    }
    assert.Equal(t, []string{"app1-dev", "app1-prod", "app2-dev"}, names)
}

func TestContextStore_Update(t *testing.T) {
    testDB := testutil.SetupTestDatabase(t)
    store := storage.NewContextStore(testDB.DB)
    
    ctx := context.Background()
    testContext := createTestContext("test-app-dev")
    
    // Create initial context
    err := store.Create(ctx, testContext)
    assert.NoError(t, err)
    
    // Update context
    testContext.Spec.App.Environment = "staging"
    err = store.Update(ctx, testContext)
    assert.NoError(t, err)
    
    // Verify update
    retrieved, err := store.Get(ctx, "test-app-dev")
    assert.NoError(t, err)
    assert.Equal(t, "staging", retrieved.Spec.App.Environment)
}
```

### Task 3: API Integration Tests

**File: `cmd/gateway/integration_test.go`**

```go
func TestContextCRUD_Integration(t *testing.T) {
    // Setup test environment
    testDB := testutil.SetupTestDatabase(t)
    testRabbit := testutil.SetupTestRabbitMQ(t)
    
    // Create test server
    server := setupTestServer(t, testDB.DB, testRabbit.Connection)
    defer server.Close()
    
    client := &http.Client{Timeout: 10 * time.Second}
    
    t.Run("CreateContext", func(t *testing.T) {
        context := createTestContextPayload()
        
        resp, err := client.Post(
            server.URL+"/contexts",
            "application/json",
            strings.NewReader(context),
        )
        assert.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        var created map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&created)
        assert.NoError(t, err)
        assert.Equal(t, "test-app-dev", created["metadata"].(map[string]interface{})["name"])
    })
    
    t.Run("GetContext", func(t *testing.T) {
        resp, err := client.Get(server.URL + "/contexts/test-app-dev")
        assert.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        
        var retrieved map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&retrieved)
        assert.NoError(t, err)
        assert.Equal(t, "test-app-dev", retrieved["metadata"].(map[string]interface{})["name"])
    })
    
    t.Run("ListContexts", func(t *testing.T) {
        resp, err := client.Get(server.URL + "/contexts")
        assert.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        
        var contexts []map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&contexts)
        assert.NoError(t, err)
        assert.Len(t, contexts, 1)
    })
    
    t.Run("RefreshAction", func(t *testing.T) {
        resp, err := client.Post(server.URL+"/contexts/test-app-dev/actions/refresh", "application/json", nil)
        assert.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        
        var result map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&result)
        assert.NoError(t, err)
        assert.True(t, result["success"].(bool))
        assert.NotEmpty(t, result["correlation_id"])
    })
}

func TestContextValidation_Integration(t *testing.T) {
    testDB := testutil.SetupTestDatabase(t)
    testRabbit := testutil.SetupTestRabbitMQ(t)
    server := setupTestServer(t, testDB.DB, testRabbit.Connection)
    defer server.Close()
    
    client := &http.Client{Timeout: 10 * time.Second}
    
    tests := []struct {
        name           string
        payload        string
        expectedStatus int
        errorContains  string
    }{
        {
            name:           "invalid name format",
            payload:        createInvalidContextPayload("Invalid_Name"),
            expectedStatus: http.StatusBadRequest,
            errorContains:  "validation failed",
        },
        {
            name:           "missing required fields",
            payload:        `{"apiVersion": "platformctl/v1"}`,
            expectedStatus: http.StatusBadRequest,
            errorContains:  "validation failed",
        },
        {
            name:           "inline secret forbidden",
            payload:        createContextWithInlineSecret(),
            expectedStatus: http.StatusBadRequest,
            errorContains:  "secret validation",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resp, err := client.Post(
                server.URL+"/contexts",
                "application/json",
                strings.NewReader(tt.payload),
            )
            assert.NoError(t, err)
            defer resp.Body.Close()
            
            assert.Equal(t, tt.expectedStatus, resp.StatusCode)
            
            if tt.errorContains != "" {
                body, err := io.ReadAll(resp.Body)
                assert.NoError(t, err)
                assert.Contains(t, string(body), tt.errorContains)
            }
        })
    }
}
```

### Task 4: Service Integration Tests

**File: `cmd/vault-svc/integration_test.go`**

```go
func TestVaultService_Integration(t *testing.T) {
    if os.Getenv("VAULT_ADDR") == "" {
        t.Skip("VAULT_ADDR not set, skipping Vault integration tests")
    }
    
    testDB := testutil.SetupTestDatabase(t)
    testRabbit := testutil.SetupTestRabbitMQ(t)
    
    // Setup test context in database
    store := storage.NewContextStore(testDB.DB)
    testContext := createTestContextWithVault()
    err := store.Create(context.Background(), testContext)
    assert.NoError(t, err)
    
    // Setup vault service
    vaultHandler := NewVaultHandler()
    
    // Create test command
    cmd := &api.CommandMessage{
        MessageEnvelope: api.MessageEnvelope{
            MessageID:     "test-message-id",
            CorrelationID: "test-correlation-id",
            ContextName:   "test-vault-context",
            Action:        "validate",
            RequestedBy:   "test-user",
            RequestedAt:   time.Now(),
        },
    }
    
    // Execute command
    result, err := vaultHandler.HandleCommand(cmd)
    assert.NoError(t, err)
    assert.NotNil(t, result)
    
    // Verify result structure
    assert.Equal(t, "vault", result.ServiceName)
    assert.Equal(t, cmd.CorrelationID, result.CorrelationID)
    assert.Equal(t, cmd.ContextName, result.ContextName)
    assert.NotEmpty(t, result.CompletedAt)
    
    // Verify payload contains expected fields
    payload, ok := result.ResultPayload.(map[string]interface{})
    assert.True(t, ok)
    assert.Contains(t, payload, "auth_status")
    assert.Contains(t, payload, "secret_validations")
    assert.Contains(t, payload, "latency_ms")
}

func TestVaultService_AuthFailure(t *testing.T) {
    testDB := testutil.SetupTestDatabase(t)
    
    // Setup test context with invalid Vault config
    store := storage.NewContextStore(testDB.DB)
    testContext := createTestContextWithInvalidVault()
    err := store.Create(context.Background(), testContext)
    assert.NoError(t, err)
    
    vaultHandler := NewVaultHandler()
    
    cmd := &api.CommandMessage{
        MessageEnvelope: api.MessageEnvelope{
            ContextName: "test-invalid-vault",
            Action:      "validate",
        },
    }
    
    result, err := vaultHandler.HandleCommand(cmd)
    assert.NoError(t, err) // Handler shouldn't return error
    
    // But result should indicate failure
    assert.Equal(t, "error", result.Status)
    assert.NotEmpty(t, result.ErrorMessage)
    assert.Contains(t, result.ErrorMessage, "auth validation failed")
}
```

### Task 5: End-to-End Workflow Tests

**File: `test/e2e/workflow_test.go`**

```go
func TestCompleteRefreshWorkflow_E2E(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E tests in short mode")
    }
    
    // Setup complete test environment
    env := setupE2EEnvironment(t)
    defer env.Cleanup()
    
    // Create a test context
    context := createE2ETestContext()
    createResp := env.CreateContext(t, context)
    assert.Equal(t, http.StatusCreated, createResp.StatusCode)
    
    // Wait for context to be ready
    time.Sleep(2 * time.Second)
    
    // Trigger refresh action
    refreshResp := env.TriggerAction(t, "test-e2e-app", "refresh")
    assert.Equal(t, http.StatusOK, refreshResp.StatusCode)
    
    var refreshResult map[string]interface{}
    err := json.NewDecoder(refreshResp.Body).Decode(&refreshResult)
    assert.NoError(t, err)
    correlationID := refreshResult["correlation_id"].(string)
    assert.NotEmpty(t, correlationID)
    
    // Wait for processing to complete
    waitForWorkflowCompletion(t, env, correlationID, 30*time.Second)
    
    // Verify aggregated status
    statusResp := env.GetContextStatus(t, "test-e2e-app")
    assert.Equal(t, http.StatusOK, statusResp.StatusCode)
    
    var status map[string]interface{}
    err = json.NewDecoder(statusResp.Body).Decode(&status)
    assert.NoError(t, err)
    
    // Verify all services processed
    summary := status["summary"].(map[string]interface{})
    assert.Contains(t, summary, "vault")
    assert.Contains(t, summary, "kubernetes")
    assert.Contains(t, summary, "git")
    
    // Verify overall health is calculated
    assert.Contains(t, status, "overall_health")
    assert.NotEqual(t, "unknown", status["overall_health"])
    
    // Verify run history
    historyResp := env.GetRunHistory(t, "test-e2e-app")
    assert.Equal(t, http.StatusOK, historyResp.StatusCode)
    
    var history map[string]interface{}
    err = json.NewDecoder(historyResp.Body).Decode(&history)
    assert.NoError(t, err)
    
    runs := history["runs"].([]interface{})
    assert.Len(t, runs, 1)
    
    run := runs[0].(map[string]interface{})
    assert.Equal(t, correlationID, run["correlation_id"])
    assert.Equal(t, "refresh", run["action"])
    assert.Contains(t, run, "services")
}

type E2EEnvironment struct {
    GatewayURL string
    Database   *testutil.TestDatabase  
    RabbitMQ   *testutil.TestRabbitMQ
    Services   []*ServiceProcess
}

func setupE2EEnvironment(t *testing.T) *E2EEnvironment {
    // Setup infrastructure
    testDB := testutil.SetupTestDatabase(t)
    testRabbit := testutil.SetupTestRabbitMQ(t)
    
    // Start services in background
    services := startE2EServices(t, testDB, testRabbit)
    
    // Wait for services to be ready
    waitForServicesReady(t, services)
    
    env := &E2EEnvironment{
        GatewayURL: "http://localhost:8080",
        Database:   testDB,
        RabbitMQ:   testRabbit,
        Services:   services,
    }
    
    return env
}

func waitForWorkflowCompletion(t *testing.T, env *E2EEnvironment, correlationID string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    
    for time.Now().Before(deadline) {
        // Check if all expected services have reported results
        var count int
        err := env.Database.DB.QueryRow(`
            SELECT COUNT(DISTINCT service_name) 
            FROM result_events 
            WHERE correlation_id = $1
        `, correlationID).Scan(&count)
        
        if err == nil && count >= 3 { // Expect at least vault, kube, git
            return
        }
        
        time.Sleep(500 * time.Millisecond)
    }
    
    t.Fatalf("Workflow did not complete within timeout")
}
```

### Task 6: Test Data Management

**File: `internal/testutil/fixtures.go`**

```go
package testutil

import (
    "encoding/json"
    "fmt"
    "time"
)

// Test context fixtures
func CreateTestContext(name string) *contexts.Context {
    return &contexts.Context{
        APIVersion: "platformctl/v1",
        Kind:       "Context",
        Metadata: contexts.ContextMetadata{
            Name: name,
        },
        Spec: CreateTestContextSpec(),
    }
}

func CreateTestContextSpec() contexts.ContextSpec {
    return contexts.ContextSpec{
        App: contexts.AppConfig{
            Name:        "test-app",
            Environment: "test",
        },
        Policy: contexts.PolicyConfig{
            AllowedActions:        []string{"refresh", "validate", "inspect"},
            RequireMfaForActions: []string{},
        },
        Vault: contexts.VaultConfig{
            Address:   "https://vault.test.local",
            Namespace: "test",
            Auth: contexts.VaultAuthConfig{
                Method: "token",
                Token: contexts.VaultTokenAuth{
                    TokenRef: contexts.SecretReference{
                        VaultPath: "secret/test",
                        Key:      "token",
                    },
                },
            },
            Secrets: []contexts.VaultSecret{
                {
                    LogicalName:  "test-secret",
                    Path:         "secret/test/app",
                    RequiredKeys: []string{"api_key"},
                },
            },
        },
        Kubernetes: contexts.KubernetesConfig{
            Kubeconfig: contexts.KubeconfigConfig{
                Path: "/tmp/test-kubeconfig",
            },
        },
        Git: contexts.GitConfig{
            Provider: "github",
            Auth: contexts.GitAuthConfig{
                Method: "pat",
                SecretRef: contexts.SecretReference{
                    VaultPath: "secret/test/github", 
                    Key:      "token",
                },
            },
            Browse: contexts.GitBrowseConfig{
                DefaultOrg:      "test-org",
                CacheTTLSeconds: 300,
            },
        },
    }
}

// Test message fixtures
func CreateTestCommandMessage(contextName, action string) *api.CommandMessage {
    return &api.CommandMessage{
        MessageEnvelope: api.MessageEnvelope{
            SchemaVersion: 1,
            MessageID:     RandomUUID(),
            CorrelationID: RandomUUID(),
            ContextName:   contextName,
            Action:        action,
            RequestedBy:   "test-user",
            RequestedAt:   time.Now().UTC(),
            Payload:       make(map[string]interface{}),
        },
    }
}

func CreateTestResultMessage(serviceName, contextName, correlationID string) *api.ResultMessage {
    return &api.ResultMessage{
        MessageEnvelope: api.MessageEnvelope{
            SchemaVersion: 1,
            MessageID:     RandomUUID(),
            CorrelationID: correlationID,
            ContextName:   contextName,
            Action:        "refresh",
            RequestedBy:   "test-user",
            RequestedAt:   time.Now().UTC().Add(-5 * time.Second),
        },
        ServiceName: serviceName,
        Status:      "ok",
        CompletedAt: time.Now().UTC(),
        ResultPayload: map[string]interface{}{
            "latency_ms": 250,
            "test_data":  true,
        },
    }
}

// Database seed data
func SeedTestData(db *sql.DB) error {
    contexts := []*contexts.Context{
        CreateTestContext("app1-dev"),
        CreateTestContext("app1-prod"), 
        CreateTestContext("app2-dev"),
    }
    
    store := storage.NewContextStore(db)
    
    for _, ctx := range contexts {
        if err := store.Create(context.Background(), ctx); err != nil {
            return fmt.Errorf("failed to seed context %s: %w", ctx.Metadata.Name, err)
        }
    }
    
    return nil
}
```

### Task 7: CI Pipeline Integration

**File: `.github/workflows/test.yml`**

```yaml
name: Test

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432
          
      rabbitmq:
        image: rabbitmq:3.11
        env:
          RABBITMQ_DEFAULT_USER: guest
          RABBITMQ_DEFAULT_PASS: guest
        options: >-
          --health-cmd "rabbitmq-diagnostics -q ping"
          --health-interval 30s
          --health-timeout 30s
          --health-retries 3
        ports:
          - 5672:5672

    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21
        
    - name: Cache dependencies
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
          
    - name: Install dependencies
      run: go mod download
      
    - name: Run unit tests
      env:
        TEST_DATABASE_URL: postgres://postgres:postgres@localhost:5432
        TEST_RABBITMQ_URL: amqp://guest:guest@localhost:5672/
      run: go test -v -race -coverprofile=coverage.out ./internal/...
      
    - name: Run integration tests
      env:
        TEST_DATABASE_URL: postgres://postgres:postgres@localhost:5432
        TEST_RABBITMQ_URL: amqp://guest:guest@localhost:5672/
      run: go test -v -race -tags=integration ./cmd/...
      
    - name: Upload coverage reports
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
        flags: unittests
        name: codecov-umbrella
```

---

## Validation Checklist

Before marking Phase 1G complete:

**Unit Tests:**
- [ ] All core business logic has unit test coverage
- [ ] Context validation logic thoroughly tested
- [ ] Database operations tested with test database
- [ ] Message handling logic tested with mocks
- [ ] Error scenarios covered

**Integration Tests:**
- [ ] API endpoints tested with real database
- [ ] RabbitMQ message flow tested
- [ ] Service command handling tested
- [ ] Database migrations tested
- [ ] Authentication/authorization tested

**System Tests:**
- [ ] Complete workflow tests (command → processing → result → aggregation)
- [ ] Multiple service coordination tested
- [ ] Error propagation and handling tested
- [ ] Performance under load tested
- [ ] Concurrent operation testing

**Test Infrastructure:**
- [ ] Test database setup/cleanup working
- [ ] Test RabbitMQ setup/cleanup working
- [ ] Test fixtures provide realistic data
- [ ] CI pipeline executes all test types
- [ ] Code coverage reporting functional

**Quality Metrics:**
- [ ] Code coverage above 80% for core logic
- [ ] All tests pass consistently
- [ ] No flaky tests in CI
- [ ] Test execution time reasonable (< 5 minutes)

---

## Next Steps

Upon completion, Phase 1G provides:
- Comprehensive test coverage for confidence in changes
- Automated testing in CI pipeline
- Foundation for test-driven development
- Quality assurance for production deployment

**Handoff to Phase 1H:** The CLI implementation can now include tests from the beginning and leverage the existing test infrastructure for validation.