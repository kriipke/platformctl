# ContextOps Sample Data Scripts

This directory contains scripts to generate and load comprehensive sample data for the ContextOps GitOps monitoring platform.

## Files

- **`generate-sample-data.sql`** - Complete SQL script with realistic sample data
- **`load-sample-data.sh`** - Shell script to execute the SQL and load data
- **`README.md`** - This documentation file

## Sample Data Overview

The sample data represents three realistic customer organizations:

### 🏢 Customer Organizations

1. **ACME Corp** - Enterprise SaaS company
   - Environments: `dev`, `qa`, `prod`
   - Applications: `user-service`, `payment-service`, `notification-service`, `api-gateway`
   - Tech Stack: Go, PostgreSQL, Redis, Vault, Stripe

2. **TechStart.io** - Early-stage startup
   - Environments: `dev`, `staging`, `prod`
   - Applications: `web-app`, `api-backend`, `analytics-service`
   - Tech Stack: React, Node.js, PostgreSQL, Redis, Google Cloud

3. **Global Bank** - Financial institution
   - Environments: `dev`, `test`, `prod`
   - Applications: `account-service`, `transaction-processor`, `customer-portal`, `compliance-service`
   - Tech Stack: Java Spring Boot, Oracle, Kafka, HSM, SOX/PCI compliance

### 📊 Data Breakdown

| Resource Type | Count | Description |
|---------------|-------|-------------|
| Environments | 9 | 3 environments per customer organization |
| Applications | 11 | Microservices and web applications |
| Contexts | 18 | App-environment deployment combinations |
| Vault Secrets | 30+ | Database credentials, API keys, certificates |
| ApplicationSets | 4+ | ArgoCD ApplicationSet configurations |
| Helm Sources | 7+ | Helm chart configurations with overrides |
| Cluster Configs | 9 | Kubernetes cluster connection details |
| Deployments | 17+ | Current deployment status tracking |
| Pod Validations | 40+ | Vault secret-to-pod environment correlations |

## Quick Start

### Option 1: Load via kubectl (Kubernetes deployment)

```bash
# Auto-detect PostgreSQL pod
./load-sample-data.sh --kubernetes --pod $(kubectl get pods -n contextops -l app=postgres -o name | head -1 | cut -d/ -f2)

# Or specify pod manually
./load-sample-data.sh --kubernetes --pod postgres-55c8b6d6ff-krc2d --namespace contextops
```

### Option 2: Load via direct psql connection

```bash
# Default local connection
./load-sample-data.sh

# Custom database connection
./load-sample-data.sh --host your-db-host --user contextops --database contextops
```

### Option 3: Manual SQL execution

```bash
# Using psql directly
psql -h localhost -U contextops -d contextops -f generate-sample-data.sql

# Using kubectl exec
kubectl exec -n contextops postgres-pod -- psql -h localhost -U contextops -d contextops -f - < generate-sample-data.sql
```

## Sample Data Features

### 🚀 GitOps Scenarios

- **Multi-Environment Deployments**: Each application deployed across 3 environments
- **Version Progression**: Realistic version patterns (higher versions in dev, stable in prod)
- **Resource Scaling**: Appropriate replica counts per environment (1-2 dev → 5-20 prod)
- **Configuration Drift**: Different configs per environment (debug modes, resource limits)

### 🔐 Vault Integration

- **Secret Types**: Database credentials, JWT keys, API keys, SSL certificates, HSM keys
- **Environment Isolation**: Separate Vault paths per customer and environment
- **Validation Status**: Realistic secret sync statuses (valid, stale, failed, pending)
- **Pod Correlation**: Environment variables mapped to Vault secrets

### 🏗️ Infrastructure Patterns

- **Multi-Cloud**: AWS EKS, Google GKE, self-managed clusters
- **Compliance**: Banking-specific compliance requirements (SOX, PCI-DSS, FIPS)
- **High Availability**: Production clusters with multi-region setup
- **Security**: Different authentication methods per customer

### 📈 Monitoring & Observability

- **Deployment Status**: ArgoCD sync status (Synced, OutOfSync, Progressing)
- **Health Status**: Application health (Healthy, Degraded, Unhealthy)
- **Secret Validation**: Real-time Vault secret validation status
- **Resource Tracking**: CPU/memory requests and limits per environment

## Verification Queries

After loading the data, run these queries to verify everything was created:

```sql
-- Customer overview
SELECT 
    customer_id,
    COUNT(DISTINCT e.name) as environments,
    (SELECT COUNT(*) FROM apps a WHERE a.customer_id = e.customer_id) as applications,
    (SELECT COUNT(*) FROM contexts c WHERE c.customer_id = e.customer_id) as contexts
FROM environments e
GROUP BY customer_id 
ORDER BY customer_id;

-- Multi-environment applications
SELECT 
    a.customer_id,
    a.name as application,
    COUNT(c.name) as deployed_environments
FROM apps a
JOIN contexts c ON c.app_reference = a.name AND c.customer_id = a.customer_id
GROUP BY a.customer_id, a.name 
ORDER BY a.customer_id, a.name;

-- Vault secrets by environment
SELECT 
    customer_id,
    environment_name,
    COUNT(*) as secrets_count,
    COUNT(CASE WHEN validation_status = 'valid' THEN 1 END) as valid_secrets
FROM vault_static_secrets
GROUP BY customer_id, environment_name
ORDER BY customer_id, environment_name;
```

## API Testing

Once the sample data is loaded, you can test the ContextOps API endpoints:

```bash
# Test health endpoint (public)
curl http://your-gateway-ip/health

# Test contexts API (requires authentication)
curl -u admin:admin http://your-gateway-ip/api/v1/contexts

# Test environments API
curl -u admin:admin http://your-gateway-ip/api/v1/environments

# Test applications API  
curl -u admin:admin http://your-gateway-ip/api/v1/apps
```

## Use Cases Demonstrated

The sample data enables testing of these ContextOps use cases:

1. **Multi-Tenant Management**: Separate customer organizations with isolated data
2. **Multi-Environment Tracking**: Applications deployed across dev/staging/prod
3. **Version Management**: Different application versions per environment
4. **Secret Synchronization**: Vault secrets correlated with pod environment variables
5. **Compliance Tracking**: Banking-specific compliance requirements
6. **GitOps Workflows**: ArgoCD ApplicationSets with Helm chart management
7. **Infrastructure Diversity**: Multiple cloud providers and cluster types
8. **Scaling Patterns**: Resource allocation scaling with environment criticality
9. **Health Monitoring**: Deployment and application health status tracking
10. **Cross-Environment Analysis**: Comparing configurations across environments

## Cleanup

To remove sample data (if supported by your schema):

```sql
-- WARNING: This will delete all sample data
TRUNCATE TABLE pod_env_validations CASCADE;
TRUNCATE TABLE context_deployments CASCADE; 
TRUNCATE TABLE customer_branches CASCADE;
TRUNCATE TABLE vault_static_secrets CASCADE;
TRUNCATE TABLE cluster_configs CASCADE;
TRUNCATE TABLE helm_sources CASCADE;
TRUNCATE TABLE applicationsets CASCADE;
TRUNCATE TABLE contexts CASCADE;
TRUNCATE TABLE apps CASCADE;
TRUNCATE TABLE environments CASCADE;
```

## Contributing

To add more sample data scenarios:

1. Follow the existing naming conventions
2. Maintain realistic relationships between entities
3. Include proper timestamps for created/updated fields
4. Add verification queries for new data types
5. Update this README with new use cases

## Support

For issues with the sample data scripts:

1. Check database connection and permissions
2. Verify all required tables exist (run migrations first)
3. Check for foreign key constraint errors
4. Review the PostgreSQL logs for detailed error messages