# Troubleshooting Database Migrations

This guide covers common database migration issues in ContextOps deployments and their solutions.

## Overview

ContextOps uses database migrations to manage schema changes and updates. The migration system tracks the current database version and ensures all changes are applied in the correct order. However, sometimes migrations can fail or be interrupted, leaving the database in an inconsistent "dirty" state.

## Common Migration Errors

### "Dirty Database Version" Error

The most common migration error is:

```
Failed to run migrations: failed to run migrations: Dirty database version X. Fix and force version.
```

This error occurs when a migration was started but not completed successfully, leaving the migration system in an inconsistent state.

## Root Causes

### 1. Resource Constraints
- **Pod restarts** due to insufficient memory or CPU
- **Node evictions** during cluster maintenance
- **Timeout errors** when migrations take too long

### 2. Database Issues
- **Connection timeouts** to PostgreSQL
- **Lock timeouts** when multiple services try to migrate simultaneously
- **SQL errors** in migration scripts

### 3. Missing Dependencies
- **Trigger functions** not created before table creation
- **Extensions** not installed
- **Foreign key constraints** referencing missing tables

## Diagnosis Steps

### Step 1: Check Migration State

```bash
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c "SELECT * FROM schema_migrations;"
```

Expected output:
```
 version | dirty 
---------+-------
       4 | f
```

If `dirty = t`, the migration is stuck.

### Step 2: Check Gateway Logs

```bash
kubectl logs -n contextops deployment/contextops-gateway --tail=20
```

Look for specific error messages that indicate which migration failed.

### Step 3: Verify Database Tables

```bash
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c "\dt"
```

Compare the existing tables with what should exist after the current migration version.

## Resolution Steps

### Method 1: Clear Dirty State (Most Common)

**Step 1: Identify the stuck version**
```bash
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c "SELECT * FROM schema_migrations;"
```

**Step 2: Clear the dirty flag**
```bash
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c \
  "UPDATE schema_migrations SET dirty = false WHERE version = X;"
```
Replace `X` with the actual version number from Step 1.

**Step 3: Create missing dependencies**

For migration version 4 and above, ensure the trigger function exists:
```bash
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c \
  "CREATE OR REPLACE FUNCTION update_updated_at_column() 
   RETURNS TRIGGER AS \$\$ 
   BEGIN 
     NEW.last_updated = NOW(); 
     RETURN NEW; 
   END; 
   \$\$ LANGUAGE plpgsql;"
```

**Step 4: Restart the gateway**
```bash
kubectl rollout restart deployment/contextops-gateway -n contextops
```

**Step 5: Verify the fix**
```bash
# Check migration state
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c "SELECT * FROM schema_migrations;"

# Check gateway status
kubectl get pods -n contextops | grep gateway

# Check logs for successful startup
kubectl logs -n contextops deployment/contextops-gateway --tail=5
```

Expected log output:
```
2026/01/27 01:12:19 Starting GitOps API Gateway on port 8080 (health: 8081)
```

### Method 2: Manual Migration Recovery

If clearing the dirty state doesn't work, manually complete the migration:

**Step 1: Check which tables are missing**
```bash
# List current tables
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c "\dt"

# Check migration 4 tables specifically
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c \
  "SELECT tablename FROM pg_tables WHERE tablename IN 
   ('context_pairing_status', 'app_manifest_correlation', 
    'environment_manifest_validation', 'context_pairing_operations',
    'gitops_vault_validation_status', 'multi_environment_app_status',
    'customer_git_branch_correlation');"
```

**Step 2: Create missing tables manually**

For context_pairing_status table:
```bash
kubectl exec -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops -c \
  "CREATE TABLE IF NOT EXISTS context_pairing_status (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    context_name VARCHAR(255) NOT NULL,
    app_reference VARCHAR(255) NOT NULL,
    environment_reference VARCHAR(255) NOT NULL,
    pairing_status VARCHAR(50) DEFAULT 'unknown',
    sync_status VARCHAR(50) DEFAULT 'unknown',
    health_status VARCHAR(50) DEFAULT 'unknown',
    resource_count INTEGER DEFAULT 0,
    last_sync_time TIMESTAMP WITH TIME ZONE,
    last_deployment_time TIMESTAMP WITH TIME ZONE,
    correlation_data JSONB,
    validation_errors TEXT[],
    git_commit VARCHAR(40),
    helm_revision VARCHAR(100),
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(customer_id, context_name, app_reference, environment_reference)
  );"
```

**Step 3: Clear dirty state and restart**
Follow steps 2-5 from Method 1.

### Method 3: Complete Reset (Last Resort)

⚠️ **Warning**: This method will lose all data. Only use in development environments.

**Step 1: Scale down all services**
```bash
kubectl scale deployment --all --replicas=0 -n contextops
```

**Step 2: Reset database**
```bash
# Connect to postgres
kubectl exec -it -n contextops deployment/contextops-postgres -- \
  psql -U contextops -d contextops

# Drop and recreate database
DROP DATABASE contextops;
CREATE DATABASE contextops OWNER contextops;
\q
```

**Step 3: Scale services back up**
```bash
kubectl scale deployment --replicas=1 -n contextops contextops-gateway
kubectl scale deployment --replicas=1 -n contextops contextops-gitops-aggregator
# Scale other services as needed
```

## Prevention Strategies

### 1. Resource Management

**Ensure adequate resources:**
```yaml
# In values.yaml or values-production.yaml
gateway:
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi" 
      cpu: "500m"
```

**Enable resource monitoring:**
```bash
kubectl top pods -n contextops
```

### 2. Migration Timeouts

**Increase migration timeouts in gateway configuration:**
```yaml
config:
  performance:
    requestTimeout: "60s"  # Increase from 30s
    dbMaxOpenConns: 10     # Reduce from 25 to avoid locks
```

### 3. Sequential Deployments

**Avoid concurrent deployments:**
```bash
# Use --wait flag for helm installations
helm upgrade contextops . --wait --timeout=10m -n contextops
```

### 4. Health Checks

**Monitor migration health:**
```bash
# Check gateway readiness
kubectl get pods -n contextops -l app=contextops-gateway

# Check database connectivity
kubectl exec -n contextops deployment/contextops-postgres -- \
  pg_isready -U contextops -d contextops
```

## Migration Version Reference

| Version | Description | Key Tables Added |
|---------|-------------|------------------|
| 1 | Core application schema | `apps`, `environments`, `contexts` |
| 2 | Security audit schema | `audit_logs`, `security_events` |
| 3 | Run tracking | `command_runs`, `service_runs` |
| 4 | GitOps read model | `context_pairing_status`, `app_manifest_correlation` |

## Common SQL Functions

### Trigger Function for Updated Timestamps
```sql
CREATE OR REPLACE FUNCTION update_updated_at_column() 
RETURNS TRIGGER AS $$ 
BEGIN 
  NEW.last_updated = NOW(); 
  RETURN NEW; 
END; 
$$ LANGUAGE plpgsql;
```

### Check Migration Dependencies
```sql
-- Check if trigger function exists
SELECT routine_name 
FROM information_schema.routines 
WHERE routine_name = 'update_updated_at_column';

-- Check foreign key constraints
SELECT conname, conrelid::regclass, confrelid::regclass
FROM pg_constraint 
WHERE contype = 'f';
```

## Troubleshooting Checklist

- [ ] Check migration state (`dirty` flag)
- [ ] Review gateway/postgres logs for errors  
- [ ] Verify database connectivity
- [ ] Check resource availability (CPU/memory)
- [ ] Ensure trigger functions exist
- [ ] Validate table dependencies
- [ ] Clear dirty state if safe
- [ ] Restart affected services
- [ ] Verify successful startup
- [ ] Test basic functionality

## Getting Help

If these solutions don't resolve your migration issues:

1. **Collect diagnostics:**
   ```bash
   kubectl logs -n contextops deployment/contextops-gateway > gateway.log
   kubectl logs -n contextops deployment/contextops-postgres > postgres.log
   kubectl describe pods -n contextops > pod-status.txt
   ```

2. **Check resource usage:**
   ```bash
   kubectl top nodes
   kubectl top pods -n contextops
   ```

3. **Review the migration files:**
   - Location: `migrations/` directory in the platformctl repository
   - Check for complex dependencies or long-running operations

4. **Consider temporary workarounds:**
   - Disable migration checks temporarily (development only)
   - Use read-only mode until migrations are fixed
   - Scale down to single replica to avoid concurrent access

---

**Last Updated**: January 2026  
**Applies to**: ContextOps v0.1.0+