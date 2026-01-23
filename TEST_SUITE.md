# PlatformCTL Test Suite

This document describes the comprehensive test suite for the PlatformCTL GitOps platform implementing PHASE-1A requirements.

## Test Architecture

The test suite follows the testing pyramid pattern with emphasis on unit tests, comprehensive integration tests, and end-to-end workflow validation.

### Test Levels

1. **Unit Tests** - Fast, isolated tests for individual components
2. **Integration Tests** - Tests for component interactions with real dependencies
3. **End-to-End Tests** - Complete workflow tests demonstrating the three-manifest architecture

## Test Coverage

### PHASE-1A Requirements Coverage

✅ **Unit tests for App, Environment, and Context models**
- JSON marshaling/unmarshaling
- Field validation and constraints
- Complex nested structure handling
- Timestamp management

✅ **Integration test for complete App+Environment+Context CRUD flow**
- Complete lifecycle testing
- Relationship validation
- Cross-manifest dependencies
- Error handling and recovery

### Domain Models (`internal/models/*_test.go`)

#### App Model Tests (`app_test.go`)
- **TestAppMarshaling**: JSON serialization/deserialization
- **TestAppMetadataTimestamps**: Timestamp handling
- **TestHelmSourceTypes**: Multiple Helm source type support
- **TestApplicationSetGenerator**: ArgoCD ApplicationSet generators
- **TestAppEnvironmentRef**: Environment reference validation
- **TestBootstrapApplicationConfig**: Bootstrap application configuration
- **TestAppSpecValidStructure**: Complex nested structure validation

#### Environment Model Tests (`environment_test.go`)
- **TestEnvironmentMarshaling**: JSON serialization/deserialization
- **TestEnvironmentMetadataTimestamps**: Timestamp handling
- **TestVaultAuthConfig**: Multiple Vault authentication methods
- **TestVaultDatasource**: Vault datasource configuration
- **TestVaultStaticSecret**: Vault static secret management
- **TestPodEnvValidationConfig**: Pod environment variable validation
- **TestHelmValuesSource**: Git-based Helm values configuration
- **TestClusterConfig**: Kubernetes cluster configuration

#### Context Model Tests (`context_test.go`)
- **TestContextMarshaling**: JSON serialization/deserialization
- **TestContextMetadataTimestamps**: Timestamp handling
- **TestContextDeployment**: Deployment state management
- **TestCustomerBranchConfig**: Customer-specific branch configuration
- **TestMonitoringConfig**: GitOps monitoring configuration
- **TestContextSpecComplexStructure**: Complex deployment scenarios
- **TestContextManifestRelationships**: App-Environment relationships

### Validation Logic (`internal/validation/*_test.go`)

#### App Validation Tests (`app_test.go`)
- **TestValidateApp**: Complete app manifest validation
- **TestValidateAppMetadata**: Metadata validation (DNS names, labels)
- **TestValidateAppApplicationConfig**: Application configuration validation
- **TestValidateAppHelmConfig**: Helm source validation
- **TestValidateHelmSource**: Individual Helm source validation
- **TestValidateAppArgoCDConfig**: ArgoCD configuration validation
- **TestValidateApplicationSetConfig**: ApplicationSet validation
- **TestValidateApplicationSetGenerator**: Generator type validation
- **TestValidateGitGenerator**: Git generator validation
- **TestValidateAppEnvironmentRef**: Environment reference validation
- **Helper function tests**: DNS names, semver, email, URL validation

#### Environment Validation Tests (`environment_test.go`)
- **TestValidateEnvironment**: Complete environment manifest validation
- **TestValidateEnvironmentMetadata**: Metadata validation
- **TestValidateEnvironmentConfig**: Environment configuration validation
- **TestValidateClusterConfig**: Kubernetes cluster validation
- **TestValidateVaultSecretRef**: Vault secret reference validation
- **TestValidateHelmValuesSource**: Helm values source validation
- **TestValidateEnvironmentVaultConfig**: Vault configuration validation
- **TestValidateVaultAuthConfig**: Authentication method validation
- **TestValidateVaultDatasource**: Vault datasource validation
- **TestValidateVaultStaticSecret**: Static secret validation
- **TestValidatePodEnvValidationConfig**: Pod environment validation
- **TestIsValidVaultPath**: Vault path validation

#### Context Validation Tests (`context_test.go`)
- **TestValidateContext**: Complete context manifest validation
- **TestValidateContextMetadata**: Metadata validation
- **TestValidateContextSpec**: Context specification validation
- **TestValidateContextDeployment**: Deployment validation
- **TestValidateContextGitOpsConfig**: GitOps configuration validation
- **TestValidateCustomerBranchConfig**: Customer branch validation
- **TestValidateMonitoringConfig**: Monitoring configuration validation
- **TestIsValidCustomerBranch**: Customer branch pattern validation
- **TestContainsIgnoreCase**: Case-insensitive environment validation

### Storage Layer Integration Tests (`internal/storage/*_test.go`)

#### App Store Tests (`app_store_test.go`)
- **TestAppStore_Create**: App creation with various configurations
- **TestAppStore_Get**: App retrieval and not found scenarios
- **TestAppStore_Update**: App updates with relationship management
- **TestAppStore_Delete**: App deletion and cleanup
- **TestAppStore_List**: App listing with sorting
- **TestAppStore_ConcurrentOperations**: Race condition testing
- **TestAppStore_ComplexHelmSources**: Multiple Helm source management
- **TestAppStore_ComplexApplicationSets**: ApplicationSet configuration testing

#### Environment Store Tests (`environment_store_test.go`)
- **TestEnvironmentStore_Create**: Environment creation
- **TestEnvironmentStore_Get**: Environment retrieval
- **TestEnvironmentStore_Update**: Environment updates with Vault configuration
- **TestEnvironmentStore_Delete**: Environment deletion
- **TestEnvironmentStore_List**: Environment listing
- **TestEnvironmentStore_ConcurrentOperations**: Concurrent access testing
- **TestEnvironmentStore_ComplexVaultConfiguration**: Complex Vault setup testing
- **TestEnvironmentStore_DifferentAuthMethods**: Multiple authentication methods
- **TestEnvironmentStore_ClusterConfiguration**: Kubernetes cluster configuration

#### Context Store Tests (`context_store_test.go`)
- **TestContextStore_Create**: Context creation with deployments
- **TestContextStore_Get**: Context retrieval
- **TestContextStore_Update**: Context updates with deployment state changes
- **TestContextStore_Delete**: Context deletion
- **TestContextStore_List**: Context listing
- **TestContextStore_GetByAppAndEnvironment**: Relationship-based queries
- **TestContextStore_ConcurrentOperations**: Concurrent access testing
- **TestContextStore_ComplexDeploymentScenarios**: Multi-environment deployments
- **TestContextStore_CustomerBranchScenarios**: Customer branch configurations
- **TestContextStore_MonitoringConfiguration**: Monitoring setup testing

### End-to-End Integration Tests (`internal/integration/e2e_crud_test.go`)

#### Complete CRUD Workflow Test (`TestE2E_CompleteCRUDWorkflow`)
**Phase 1: Create Manifests**
- Create App manifest with multiple Helm sources and environments
- Create Environment manifests with different configurations (dev, staging, prod, canary)
- Create Context manifest pairing App and Environments
- Validate all manifest relationships

**Phase 2: Read and Verify Relationships**
- Retrieve all manifests and verify integrity
- Validate App-Environment references
- Validate Context-App-Environment relationships
- Verify environment-specific configurations
- Verify GitOps configurations

**Phase 3: Update Manifests and Relationships**
- Update App manifest (version bump, add environment)
- Create new Environment (canary)
- Update Context manifest (activate production, add canary deployment)
- Verify all updates were applied correctly

**Phase 4: List and Query Operations**
- List all manifests for customer
- Verify sorting and filtering
- Query contexts by app and environment references
- Test edge cases with non-existent references

**Phase 5: Complex Relationship Scenarios**
- Create second app sharing some environments
- Create context for second app
- Verify shared environment support
- Verify different app configurations

**Phase 6: Delete Operations and Cleanup**
- Delete contexts (dependents first)
- Delete apps
- Delete environments
- Verify complete cleanup and referential integrity

#### Validation Workflow Test (`TestE2E_ManifestValidationWorkflow`)
- Test invalid manifests are properly rejected
- Test valid manifests are accepted and stored
- Validate error messages are helpful and specific

#### Error Handling Test (`TestE2E_ErrorHandlingAndRecovery`)
- Test duplicate creation errors
- Test not found errors for all operations
- Test cross-customer isolation and security

## Test Utilities (`internal/testutil/`)

### Database Utilities (`fixtures.go`)
- **NewTestDB**: Creates isolated test database per test
- **Cleanup**: Removes test data between tests
- **AssertAppEqual**: App manifest comparison ignoring timestamps
- **AssertEnvironmentEqual**: Environment manifest comparison
- **AssertContextEqual**: Context manifest comparison

### Test Fixtures
- **CreateTestApp**: Creates valid test App manifests
- **CreateTestEnvironment**: Creates valid test Environment manifests
- **CreateTestContext**: Creates valid test Context manifests
- **CreateTestContextDeployments**: Creates test deployment configurations
- **CreateTestCustomerBranchContext**: Creates context with customer branch
- **Variations**: Multiple variations for different test scenarios

### HTTP Testing Utilities (`http.go`)
- **HTTPTestContext**: HTTP test server setup
- **Request helpers**: GET, POST, PUT, DELETE with authentication
- **Response assertions**: Status codes, JSON validation, error checking
- **MockCustomerAuth**: Authentication middleware for testing
- **APITestCase**: Structured API test case definition
- **Performance testing**: Concurrent request handling
- **Error response validation**: Structured error checking

## Running Tests

### Prerequisites

1. **PostgreSQL Database**
   ```bash
   # Install PostgreSQL (macOS)
   brew install postgresql
   brew services start postgresql
   
   # Create test database and user
   createdb platformctl_test
   psql -d platformctl_test -c "CREATE USER postgres WITH PASSWORD 'password';"
   ```

2. **Go Dependencies**
   ```bash
   go mod tidy
   ```

### Test Execution

#### Using Test Runner
```bash
# Run all tests
go run test/run_tests.go

# Run specific test types
go run test/run_tests.go unit
go run test/run_tests.go integration
go run test/run_tests.go e2e

# Run with verbose output
go run test/run_tests.go -v integration

# Run in short mode (skip integration tests)
go run test/run_tests.go -short unit
```

#### Using Go Test Directly
```bash
# Unit tests only
go test ./internal/models ./internal/validation

# Integration tests (requires database)
go test ./internal/storage ./internal/integration

# All tests with coverage
go test -cover -race ./...

# Specific test with verbose output
go test -v ./internal/integration -run TestE2E_CompleteCRUDWorkflow
```

### Environment Variables

- `TEST_DATABASE_URL`: PostgreSQL connection string
  - Default: `postgres://postgres:password@localhost:5432/platformctl_test?sslmode=disable`

## Test Performance

### Expected Performance Metrics

- **Unit Tests**: < 2 minutes total
- **Integration Tests**: < 5 minutes total  
- **End-to-End Tests**: < 10 minutes total
- **Database Operations**: < 100ms per CRUD operation
- **Concurrent Tests**: Support 10+ concurrent operations

### Test Data Management

- Each integration test uses an isolated database
- Test data is automatically cleaned up after each test
- No shared state between tests
- Deterministic test execution order

## Continuous Integration

### Test Pipeline Requirements

1. **Pre-commit Hooks**
   - Run unit tests
   - Code linting and formatting
   - Security scanning

2. **CI Pipeline**
   ```yaml
   # Example GitHub Actions configuration
   steps:
     - name: Setup PostgreSQL
       uses: harmon758/postgresql-action@v1
       with:
         postgresql version: '14'
         postgresql db: platformctl_test
         postgresql user: postgres
         postgresql password: password
   
     - name: Run Tests
       run: |
         go run test/run_tests.go -v
   ```

3. **Test Coverage Requirements**
   - Minimum 80% code coverage for all packages
   - 100% coverage for validation logic
   - All PHASE-1A success criteria must pass

## Success Criteria Validation

### PHASE-1A Requirements

✅ **Unit tests for App, Environment, and Context models**
- Comprehensive model testing with edge cases
- JSON marshaling/unmarshaling validation
- Field constraint validation
- Complex nested structure handling

✅ **Integration test for complete App+Environment+Context CRUD flow**
- End-to-end workflow testing
- Three-manifest architecture validation
- Relationship integrity testing
- Error handling and recovery scenarios

### Quality Metrics

- **Code Coverage**: >80% overall, 100% for validation
- **Test Execution Time**: <17 minutes total
- **Test Reliability**: Zero flaky tests
- **Documentation**: Complete test documentation with examples

## Troubleshooting

### Common Issues

1. **Database Connection Failures**
   ```bash
   # Check PostgreSQL is running
   pg_ctl status
   
   # Check connection
   psql -d platformctl_test -U postgres
   ```

2. **Test Timeouts**
   - Increase timeout values in test runner
   - Check database performance
   - Verify no resource leaks

3. **Race Conditions**
   - All tests include `-race` flag
   - Database operations use proper transactions
   - No shared global state

### Test Debugging

```bash
# Run specific failing test with verbose output
go test -v -run TestFailingTest ./internal/package

# Enable database query logging
export TEST_DATABASE_URL="postgres://postgres:password@localhost:5432/platformctl_test?sslmode=disable&log_statement=all"

# Run with timeout disabled for debugging
go test -timeout 0 -run TestDebugTest ./internal/package
```

## Contributing

When adding new tests:

1. **Follow naming conventions**: `Test[Component]_[Operation]`
2. **Use table-driven tests** for multiple scenarios
3. **Include both positive and negative test cases**
4. **Add integration tests** for new storage operations
5. **Update end-to-end tests** for new workflows
6. **Maintain test documentation** with clear descriptions

### Test Review Checklist

- [ ] Test covers happy path and edge cases
- [ ] Error conditions are tested
- [ ] Test data is isolated and cleaned up
- [ ] Test names clearly describe what is being tested
- [ ] Integration tests use real dependencies
- [ ] Documentation is updated
- [ ] Performance impact is considered