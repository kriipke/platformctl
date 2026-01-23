-- ContextOps Sample Data Generation Script
-- This script populates the ContextOps database with realistic sample data
-- representing three customer organizations with multi-environment GitOps deployments
--
-- Usage: psql -h localhost -U contextops -d contextops -f generate-sample-data.sql
--
-- Customer Scenarios:
-- - ACME Corp: Enterprise SaaS (dev, qa, prod)  
-- - TechStart.io: Early-stage startup (dev, staging, prod)
-- - Global Bank: Financial institution (dev, test, prod)

BEGIN;

-- Clear existing sample data (be careful in production!)
-- TRUNCATE TABLE pod_env_validations CASCADE;
-- TRUNCATE TABLE context_deployments CASCADE; 
-- TRUNCATE TABLE customer_branches CASCADE;
-- TRUNCATE TABLE vault_static_secrets CASCADE;
-- TRUNCATE TABLE vault_sources CASCADE;
-- TRUNCATE TABLE cluster_configs CASCADE;
-- TRUNCATE TABLE helm_sources CASCADE;
-- TRUNCATE TABLE applicationsets CASCADE;
-- TRUNCATE TABLE contexts CASCADE;
-- TRUNCATE TABLE apps CASCADE;
-- TRUNCATE TABLE environments CASCADE;

-- ============================================================================
-- 1. ENVIRONMENTS - Different environments per customer organization
-- ============================================================================

INSERT INTO environments (name, customer_id, spec) VALUES 
-- ACME Corp - Traditional enterprise environments
('dev', 'acme-corp', '{
  "description": "Development environment for ACME Corp",
  "cluster": {
    "name": "acme-dev-cluster", 
    "endpoint": "https://k8s-dev.acme.com",
    "region": "us-east-1"
  },
  "namespace": "acme-dev",
  "vault": {
    "address": "https://vault-dev.acme.com",
    "path": "acme/dev",
    "role": "acme-dev-role"
  },
  "argocd": {
    "server": "https://argocd-dev.acme.com",
    "project": "acme-dev",
    "repo": "https://git.acme.com/infra"
  },
  "monitoring": {
    "newrelic_account": "dev-account-123"
  }
}'),

('qa', 'acme-corp', '{
  "description": "Quality Assurance environment for ACME Corp",
  "cluster": {
    "name": "acme-qa-cluster",
    "endpoint": "https://k8s-qa.acme.com",
    "region": "us-east-1"
  },
  "namespace": "acme-qa",
  "vault": {
    "address": "https://vault-qa.acme.com",
    "path": "acme/qa",
    "role": "acme-qa-role"
  },
  "argocd": {
    "server": "https://argocd-qa.acme.com",
    "project": "acme-qa",
    "repo": "https://git.acme.com/infra"
  },
  "monitoring": {
    "newrelic_account": "qa-account-456"
  }
}'),

('prod', 'acme-corp', '{
  "description": "Production environment for ACME Corp",
  "cluster": {
    "name": "acme-prod-cluster",
    "endpoint": "https://k8s-prod.acme.com",
    "region": "us-east-1"
  },
  "namespace": "acme-prod",
  "vault": {
    "address": "https://vault.acme.com",
    "path": "acme/prod",
    "role": "acme-prod-role"
  },
  "argocd": {
    "server": "https://argocd.acme.com",
    "project": "acme-prod",
    "repo": "https://git.acme.com/infra"
  },
  "monitoring": {
    "newrelic_account": "prod-account-789"
  }
}'),

-- TechStart.io - Startup-friendly environments
('dev', 'techstart-io', '{
  "description": "Development environment for TechStart.io",
  "cluster": {
    "name": "techstart-dev",
    "endpoint": "https://dev.k8s.techstart.io",
    "region": "us-central1",
    "provider": "GKE"
  },
  "namespace": "techstart-dev",
  "vault": {
    "address": "https://vault.techstart.io",
    "path": "techstart/dev",
    "role": "dev-reader"
  },
  "argocd": {
    "server": "https://argocd.techstart.io",
    "project": "techstart-dev",
    "repo": "https://github.com/techstart/gitops"
  }
}'),

('staging', 'techstart-io', '{
  "description": "Staging environment for TechStart.io",
  "cluster": {
    "name": "techstart-staging",
    "endpoint": "https://staging.k8s.techstart.io",
    "region": "us-central1",
    "provider": "GKE"
  },
  "namespace": "techstart-staging",
  "vault": {
    "address": "https://vault.techstart.io",
    "path": "techstart/staging",
    "role": "staging-reader"
  },
  "argocd": {
    "server": "https://argocd.techstart.io",
    "project": "techstart-staging",
    "repo": "https://github.com/techstart/gitops"
  }
}'),

('prod', 'techstart-io', '{
  "description": "Production environment for TechStart.io",
  "cluster": {
    "name": "techstart-prod",
    "endpoint": "https://prod.k8s.techstart.io",
    "region": "us-central1",
    "provider": "GKE"
  },
  "namespace": "techstart-prod",
  "vault": {
    "address": "https://vault.techstart.io",
    "path": "techstart/prod",
    "role": "prod-reader"
  },
  "argocd": {
    "server": "https://argocd.techstart.io",
    "project": "techstart-prod",
    "repo": "https://github.com/techstart/gitops"
  }
}'),

-- Global Bank - Banking/Financial environments with compliance
('dev', 'global-bank', '{
  "description": "Development environment for Global Bank",
  "cluster": {
    "name": "gbank-dev-us-east",
    "endpoint": "https://dev-k8s.globalbank.com",
    "region": "us-east-1"
  },
  "namespace": "gbank-dev",
  "vault": {
    "address": "https://vault-dev.globalbank.com",
    "path": "globalbank/dev",
    "role": "dev-service"
  },
  "argocd": {
    "server": "https://argocd-dev.globalbank.com",
    "project": "gbank-dev",
    "repo": "https://git.globalbank.com/platform"
  },
  "compliance": ["SOX", "PCI-DSS"],
  "monitoring": {
    "newrelic_account": "gbank-dev-001"
  }
}'),

('test', 'global-bank', '{
  "description": "Test environment for Global Bank",
  "cluster": {
    "name": "gbank-test-us-east",
    "endpoint": "https://test-k8s.globalbank.com",
    "region": "us-east-1"
  },
  "namespace": "gbank-test",
  "vault": {
    "address": "https://vault-test.globalbank.com",
    "path": "globalbank/test",
    "role": "test-service"
  },
  "argocd": {
    "server": "https://argocd-test.globalbank.com",
    "project": "gbank-test",
    "repo": "https://git.globalbank.com/platform"
  },
  "compliance": ["SOX", "PCI-DSS"],
  "monitoring": {
    "newrelic_account": "gbank-test-002"
  }
}'),

('prod', 'global-bank', '{
  "description": "Production environment for Global Bank",
  "cluster": {
    "name": "gbank-prod-multi",
    "endpoint": "https://prod-k8s.globalbank.com",
    "region": "multi-region",
    "ha": true
  },
  "namespace": "gbank-prod",
  "vault": {
    "address": "https://vault.globalbank.com",
    "path": "globalbank/prod",
    "role": "prod-service"
  },
  "argocd": {
    "server": "https://argocd.globalbank.com",
    "project": "gbank-prod",
    "repo": "https://git.globalbank.com/platform"
  },
  "compliance": ["SOX", "PCI-DSS", "FIPS-140-2"],
  "monitoring": {
    "newrelic_account": "gbank-prod-003"
  }
}');

-- ============================================================================
-- 2. APPLICATIONS - Different applications per customer
-- ============================================================================

INSERT INTO apps (name, customer_id, spec) VALUES 
-- ACME Corp Applications
('user-service', 'acme-corp', '{
  "description": "User authentication and profile management service",
  "repository": "https://git.acme.com/services/user-service",
  "language": "Go",
  "framework": "Gin",
  "helm": {
    "chart": "microservice",
    "version": "1.2.0",
    "repository": "https://charts.acme.com"
  },
  "dependencies": ["database", "redis", "vault"],
  "team": "platform",
  "owner": "platform-team@acme.com",
  "criticality": "high",
  "sla": "99.9%"
}'),

('payment-service', 'acme-corp', '{
  "description": "Payment processing and billing service",
  "repository": "https://git.acme.com/services/payment-service",
  "language": "Go", 
  "framework": "Gin",
  "helm": {
    "chart": "microservice",
    "version": "1.2.0",
    "repository": "https://charts.acme.com"
  },
  "dependencies": ["database", "vault", "stripe-api"],
  "team": "fintech",
  "owner": "fintech-team@acme.com",
  "criticality": "critical",
  "sla": "99.95%",
  "pci_compliant": true
}'),

('notification-service', 'acme-corp', '{
  "description": "Email and SMS notification service",
  "repository": "https://git.acme.com/services/notification-service",
  "language": "Python",
  "framework": "FastAPI",
  "helm": {
    "chart": "python-service",
    "version": "1.1.5",
    "repository": "https://charts.acme.com"
  },
  "dependencies": ["rabbitmq", "sendgrid", "twilio"],
  "team": "platform",
  "owner": "platform-team@acme.com",
  "criticality": "medium",
  "sla": "99.5%"
}'),

('api-gateway', 'acme-corp', '{
  "description": "Main API gateway and load balancer",
  "repository": "https://git.acme.com/infrastructure/api-gateway",
  "technology": "NGINX Ingress",
  "helm": {
    "chart": "nginx-ingress",
    "version": "2.1.0",
    "repository": "https://kubernetes.github.io/ingress-nginx"
  },
  "dependencies": [],
  "team": "devops",
  "owner": "devops-team@acme.com",
  "criticality": "critical",
  "sla": "99.99%"
}'),

-- TechStart.io Applications
('web-app', 'techstart-io', '{
  "description": "Main React web application frontend",
  "repository": "https://github.com/techstart/web-app",
  "language": "JavaScript",
  "framework": "React",
  "helm": {
    "chart": "webapp",
    "version": "0.8.2",
    "repository": "https://charts.bitnami.com/bitnami"
  },
  "dependencies": ["api-backend", "cdn"],
  "team": "frontend",
  "owner": "frontend@techstart.io",
  "criticality": "high",
  "sla": "99.5%"
}'),

('api-backend', 'techstart-io', '{
  "description": "Node.js API backend service",
  "repository": "https://github.com/techstart/api-backend",
  "language": "JavaScript",
  "framework": "Express.js",
  "helm": {
    "chart": "nodejs-app",
    "version": "0.5.1",
    "repository": "https://charts.bitnami.com/bitnami"
  },
  "dependencies": ["postgresql", "redis", "stripe-api"],
  "team": "backend",
  "owner": "backend@techstart.io",
  "criticality": "critical",
  "sla": "99.9%"
}'),

('analytics-service', 'techstart-io', '{
  "description": "Analytics and metrics collection service",
  "repository": "https://github.com/techstart/analytics",
  "language": "Python",
  "framework": "Django",
  "helm": {
    "chart": "analytics",
    "version": "0.3.0",
    "repository": "https://charts.bitnami.com/bitnami"
  },
  "dependencies": ["clickhouse", "kafka"],
  "team": "data",
  "owner": "data@techstart.io",
  "criticality": "medium",
  "sla": "99.0%"
}'),

-- Global Bank Applications
('account-service', 'global-bank', '{
  "description": "Core account management system",
  "repository": "https://git.globalbank.com/core/account-service",
  "language": "Java",
  "framework": "Spring Boot",
  "helm": {
    "chart": "java-service",
    "version": "2.5.1",
    "repository": "https://charts.globalbank.com/internal"
  },
  "dependencies": ["oracle-db", "vault", "hsm"],
  "team": "core-banking",
  "owner": "core-banking@globalbank.com",
  "criticality": "critical",
  "sla": "99.99%",
  "compliance": ["SOX", "PCI-DSS", "FIPS-140-2"]
}'),

('transaction-processor', 'global-bank', '{
  "description": "Real-time transaction processing engine",
  "repository": "https://git.globalbank.com/core/transaction-processor",
  "language": "Java",
  "framework": "Spring Boot",
  "helm": {
    "chart": "java-service",
    "version": "2.5.1", 
    "repository": "https://charts.globalbank.com/internal"
  },
  "dependencies": ["kafka", "oracle-db", "redis", "vault"],
  "team": "payments",
  "owner": "payments@globalbank.com",
  "criticality": "critical",
  "sla": "99.99%",
  "compliance": ["SOX", "PCI-DSS"]
}'),

('customer-portal', 'global-bank', '{
  "description": "Customer-facing web portal and mobile API",
  "repository": "https://git.globalbank.com/frontend/customer-portal",
  "language": "JavaScript",
  "framework": "React",
  "helm": {
    "chart": "webapp",
    "version": "1.8.0",
    "repository": "https://charts.globalbank.com/internal"
  },
  "dependencies": ["account-service", "transaction-processor"],
  "team": "digital",
  "owner": "digital@globalbank.com",
  "criticality": "high",
  "sla": "99.9%"
}'),

('compliance-service', 'global-bank', '{
  "description": "Regulatory compliance and reporting service",
  "repository": "https://git.globalbank.com/compliance/compliance-service",
  "language": "Java",
  "framework": "Spring Boot",
  "helm": {
    "chart": "java-service",
    "version": "1.4.2",
    "repository": "https://charts.globalbank.com/internal"
  },
  "dependencies": ["oracle-db", "kafka", "vault"],
  "team": "compliance",
  "owner": "compliance@globalbank.com",
  "criticality": "critical",
  "sla": "99.95%",
  "compliance": ["SOX", "GDPR", "CCPA"]
}');

-- ============================================================================
-- 3. CONTEXTS - Link applications to environments with deployment configs
-- ============================================================================

INSERT INTO contexts (name, customer_id, app_reference, environment_reference, spec) VALUES 
-- ACME Corp Contexts - User Service across all environments
('user-service-dev', 'acme-corp', 'user-service', 'dev', '{
  "version": "1.2.3",
  "image": "user-service:1.2.3",
  "replicas": 2,
  "resources": {
    "requests": {"cpu": "200m", "memory": "256Mi"},
    "limits": {"cpu": "500m", "memory": "512Mi"}
  },
  "config": {
    "debug": true,
    "log_level": "debug",
    "db_pool_size": 5,
    "cache_ttl": 300
  },
  "secrets": ["db-credentials", "jwt-secret", "redis-password"],
  "health_checks": {
    "readiness_path": "/ready",
    "liveness_path": "/health"
  }
}'),

('user-service-qa', 'acme-corp', 'user-service', 'qa', '{
  "version": "1.2.2",
  "image": "user-service:1.2.2",
  "replicas": 2,
  "resources": {
    "requests": {"cpu": "300m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  },
  "config": {
    "debug": false,
    "log_level": "info",
    "db_pool_size": 10,
    "cache_ttl": 600
  },
  "secrets": ["db-credentials", "jwt-secret", "redis-password"],
  "health_checks": {
    "readiness_path": "/ready",
    "liveness_path": "/health"
  }
}'),

('user-service-prod', 'acme-corp', 'user-service', 'prod', '{
  "version": "1.2.0",
  "image": "user-service:1.2.0",
  "replicas": 5,
  "resources": {
    "requests": {"cpu": "1000m", "memory": "2Gi"},
    "limits": {"cpu": "2000m", "memory": "4Gi"}
  },
  "config": {
    "debug": false,
    "log_level": "warn",
    "db_pool_size": 25,
    "cache_ttl": 3600
  },
  "secrets": ["db-credentials", "jwt-secret", "redis-password"],
  "health_checks": {
    "readiness_path": "/ready",
    "liveness_path": "/health"
  }
}'),

-- ACME Corp Contexts - Payment Service
('payment-service-dev', 'acme-corp', 'payment-service', 'dev', '{
  "version": "2.1.5",
  "image": "payment-service:2.1.5",
  "replicas": 1,
  "resources": {
    "requests": {"cpu": "300m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  },
  "config": {
    "debug": true,
    "rate_limit": 100,
    "stripe_mode": "test",
    "encryption_enabled": false
  },
  "secrets": ["db-credentials", "stripe-key", "vault-token"],
  "pci_compliant": true
}'),

('payment-service-qa', 'acme-corp', 'payment-service', 'qa', '{
  "version": "2.1.4", 
  "image": "payment-service:2.1.4",
  "replicas": 2,
  "resources": {
    "requests": {"cpu": "500m", "memory": "1Gi"},
    "limits": {"cpu": "1500m", "memory": "2Gi"}
  },
  "config": {
    "debug": false,
    "rate_limit": 500,
    "stripe_mode": "test",
    "encryption_enabled": true
  },
  "secrets": ["db-credentials", "stripe-key", "vault-token"],
  "pci_compliant": true
}'),

('payment-service-prod', 'acme-corp', 'payment-service', 'prod', '{
  "version": "2.1.2",
  "image": "payment-service:2.1.2",
  "replicas": 8,
  "resources": {
    "requests": {"cpu": "1500m", "memory": "3Gi"},
    "limits": {"cpu": "3000m", "memory": "6Gi"}
  },
  "config": {
    "debug": false,
    "rate_limit": 5000,
    "stripe_mode": "live",
    "encryption_enabled": true
  },
  "secrets": ["db-credentials", "stripe-key", "vault-token"],
  "pci_compliant": true
}'),

-- ACME Corp API Gateway
('api-gateway-dev', 'acme-corp', 'api-gateway', 'dev', '{
  "version": "1.0.8",
  "replicas": 1,
  "resources": {
    "requests": {"cpu": "100m", "memory": "128Mi"},
    "limits": {"cpu": "500m", "memory": "256Mi"}
  },
  "config": {
    "ssl_enabled": false,
    "rate_limit": 1000,
    "cors_enabled": true
  },
  "secrets": []
}'),

('api-gateway-prod', 'acme-corp', 'api-gateway', 'prod', '{
  "version": "1.0.5",
  "replicas": 6,
  "resources": {
    "requests": {"cpu": "800m", "memory": "1Gi"},
    "limits": {"cpu": "2000m", "memory": "2Gi"}
  },
  "config": {
    "ssl_enabled": true,
    "rate_limit": 10000,
    "cors_enabled": false
  },
  "secrets": ["ssl-cert", "ssl-key"]
}'),

-- TechStart.io Contexts
('web-app-dev', 'techstart-io', 'web-app', 'dev', '{
  "version": "0.8.3",
  "image": "web-app:0.8.3",
  "replicas": 1,
  "resources": {
    "requests": {"cpu": "100m", "memory": "128Mi"},
    "limits": {"cpu": "300m", "memory": "256Mi"}
  },
  "config": {
    "api_url": "http://api-backend-dev.techstart-dev.svc.cluster.local",
    "analytics_enabled": true,
    "debug_mode": true
  },
  "secrets": []
}'),

('web-app-staging', 'techstart-io', 'web-app', 'staging', '{
  "version": "0.8.2",
  "image": "web-app:0.8.2",
  "replicas": 3,
  "resources": {
    "requests": {"cpu": "200m", "memory": "256Mi"},
    "limits": {"cpu": "500m", "memory": "512Mi"}
  },
  "config": {
    "api_url": "https://api-staging.techstart.io",
    "analytics_enabled": true,
    "debug_mode": false
  },
  "secrets": ["ssl-cert"]
}'),

('web-app-prod', 'techstart-io', 'web-app', 'prod', '{
  "version": "0.8.1",
  "image": "web-app:0.8.1",
  "replicas": 5,
  "resources": {
    "requests": {"cpu": "300m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  },
  "config": {
    "api_url": "https://api.techstart.io",
    "analytics_enabled": true,
    "debug_mode": false
  },
  "secrets": ["ssl-cert"]
}'),

('api-backend-dev', 'techstart-io', 'api-backend', 'dev', '{
  "version": "0.5.2",
  "image": "api-backend:0.5.2",
  "replicas": 1,
  "resources": {
    "requests": {"cpu": "200m", "memory": "256Mi"},
    "limits": {"cpu": "500m", "memory": "512Mi"}
  },
  "config": {
    "db_pool_size": 5,
    "debug": true,
    "stripe_mode": "test"
  },
  "secrets": ["db-password", "jwt-secret"]
}'),

('api-backend-staging', 'techstart-io', 'api-backend', 'staging', '{
  "version": "0.5.1",
  "image": "api-backend:0.5.1",
  "replicas": 3,
  "resources": {
    "requests": {"cpu": "400m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  },
  "config": {
    "db_pool_size": 10,
    "debug": false,
    "stripe_mode": "test"
  },
  "secrets": ["db-password", "jwt-secret", "stripe-key"]
}'),

('api-backend-prod', 'techstart-io', 'api-backend', 'prod', '{
  "version": "0.5.0",
  "image": "api-backend:0.5.0",
  "replicas": 6,
  "resources": {
    "requests": {"cpu": "800m", "memory": "1Gi"},
    "limits": {"cpu": "2000m", "memory": "2Gi"}
  },
  "config": {
    "db_pool_size": 20,
    "debug": false,
    "stripe_mode": "live"
  },
  "secrets": ["db-password", "jwt-secret", "stripe-key"]
}'),

-- Global Bank Contexts
('account-service-dev', 'global-bank', 'account-service', 'dev', '{
  "version": "2.5.2",
  "image": "account-service:2.5.2",
  "replicas": 2,
  "resources": {
    "requests": {"cpu": "500m", "memory": "1Gi"},
    "limits": {"cpu": "1000m", "memory": "2Gi"}
  },
  "config": {
    "db_timeout": 30,
    "audit_level": "debug",
    "encryption_enabled": true
  },
  "secrets": ["db-credentials", "encryption-key"]
}'),

('account-service-test', 'global-bank', 'account-service', 'test', '{
  "version": "2.5.1",
  "image": "account-service:2.5.1",
  "replicas": 4,
  "resources": {
    "requests": {"cpu": "800m", "memory": "2Gi"},
    "limits": {"cpu": "2000m", "memory": "4Gi"}
  },
  "config": {
    "db_timeout": 10,
    "audit_level": "info",
    "encryption_enabled": true
  },
  "secrets": ["db-credentials", "encryption-key"]
}'),

('account-service-prod', 'global-bank', 'account-service', 'prod', '{
  "version": "2.5.0",
  "image": "account-service:2.5.0",
  "replicas": 12,
  "resources": {
    "requests": {"cpu": "2000m", "memory": "4Gi"},
    "limits": {"cpu": "4000m", "memory": "8Gi"}
  },
  "config": {
    "db_timeout": 5,
    "audit_level": "warn",
    "encryption_enabled": true
  },
  "secrets": ["db-credentials", "encryption-key", "hsm-key"]
}'),

('transaction-processor-dev', 'global-bank', 'transaction-processor', 'dev', '{
  "version": "1.8.3",
  "image": "transaction-processor:1.8.3",
  "replicas": 1,
  "resources": {
    "requests": {"cpu": "400m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  },
  "config": {
    "batch_size": 100,
    "debug": true,
    "kafka_partitions": 3
  },
  "secrets": ["kafka-credentials", "db-credentials"]
}'),

('transaction-processor-test', 'global-bank', 'transaction-processor', 'test', '{
  "version": "1.8.2",
  "image": "transaction-processor:1.8.2",
  "replicas": 3,
  "resources": {
    "requests": {"cpu": "1000m", "memory": "2Gi"},
    "limits": {"cpu": "2000m", "memory": "4Gi"}
  },
  "config": {
    "batch_size": 500,
    "debug": false,
    "kafka_partitions": 6
  },
  "secrets": ["kafka-credentials", "db-credentials"]
}'),

('transaction-processor-prod', 'global-bank', 'transaction-processor', 'prod', '{
  "version": "1.8.1",
  "image": "transaction-processor:1.8.1",
  "replicas": 20,
  "resources": {
    "requests": {"cpu": "2500m", "memory": "8Gi"},
    "limits": {"cpu": "5000m", "memory": "16Gi"}
  },
  "config": {
    "batch_size": 1000,
    "debug": false,
    "kafka_partitions": 12
  },
  "secrets": ["kafka-credentials", "db-credentials", "encryption-key"]
}');

-- ============================================================================
-- 4. VAULT STATIC SECRETS - Secrets management across environments
-- ============================================================================

INSERT INTO vault_static_secrets (environment_name, customer_id, name, vault_path, destination_secret, required_keys, validation_status, last_validated) VALUES 
-- ACME Corp Secrets
('dev', 'acme-corp', 'database-credentials', 'acme/dev/database', 'user-service-db', '{"username", "password", "host", "port"}', 'valid', NOW() - INTERVAL '1 hour'),
('dev', 'acme-corp', 'jwt-signing-key', 'acme/dev/jwt', 'user-service-jwt', '{"private_key", "public_key", "algorithm"}', 'valid', NOW() - INTERVAL '2 hours'),
('dev', 'acme-corp', 'stripe-api-key', 'acme/dev/stripe', 'payment-service-stripe', '{"secret_key", "publishable_key"}', 'valid', NOW() - INTERVAL '30 minutes'),
('dev', 'acme-corp', 'redis-credentials', 'acme/dev/redis', 'redis-auth', '{"password"}', 'valid', NOW() - INTERVAL '45 minutes'),

('qa', 'acme-corp', 'database-credentials', 'acme/qa/database', 'user-service-db', '{"username", "password", "host", "port"}', 'valid', NOW() - INTERVAL '3 hours'),
('qa', 'acme-corp', 'jwt-signing-key', 'acme/qa/jwt', 'user-service-jwt', '{"private_key", "public_key", "algorithm"}', 'valid', NOW() - INTERVAL '4 hours'),
('qa', 'acme-corp', 'stripe-api-key', 'acme/qa/stripe', 'payment-service-stripe', '{"secret_key", "publishable_key"}', 'valid', NOW() - INTERVAL '2 hours'),

('prod', 'acme-corp', 'database-credentials', 'acme/prod/database', 'user-service-db', '{"username", "password", "host", "port"}', 'valid', NOW() - INTERVAL '15 minutes'),
('prod', 'acme-corp', 'jwt-signing-key', 'acme/prod/jwt', 'user-service-jwt', '{"private_key", "public_key", "algorithm"}', 'valid', NOW() - INTERVAL '30 minutes'),
('prod', 'acme-corp', 'stripe-api-key', 'acme/prod/stripe', 'payment-service-stripe', '{"secret_key", "publishable_key"}', 'stale', NOW() - INTERVAL '6 hours'),
('prod', 'acme-corp', 'vault-token', 'acme/prod/vault-token', 'payment-service-vault', '{"token", "role_id"}', 'valid', NOW() - INTERVAL '1 hour'),
('prod', 'acme-corp', 'ssl-certificate', 'acme/prod/ssl', 'api-gateway-ssl', '{"cert", "key", "ca"}', 'valid', NOW() - INTERVAL '2 hours'),

-- TechStart.io Secrets
('dev', 'techstart-io', 'postgres-credentials', 'techstart/dev/postgres', 'api-backend-db', '{"username", "password", "database"}', 'valid', NOW() - INTERVAL '2 hours'),
('dev', 'techstart-io', 'jwt-secret', 'techstart/dev/jwt', 'api-backend-jwt', '{"secret"}', 'valid', NOW() - INTERVAL '1 hour'),

('staging', 'techstart-io', 'postgres-credentials', 'techstart/staging/postgres', 'api-backend-db', '{"username", "password", "database"}', 'valid', NOW() - INTERVAL '45 minutes'),
('staging', 'techstart-io', 'jwt-secret', 'techstart/staging/jwt', 'api-backend-jwt', '{"secret"}', 'valid', NOW() - INTERVAL '1 hour'),
('staging', 'techstart-io', 'stripe-key', 'techstart/staging/stripe', 'api-backend-stripe', '{"secret_key", "publishable_key"}', 'valid', NOW() - INTERVAL '3 hours'),
('staging', 'techstart-io', 'ssl-certificate', 'techstart/staging/ssl', 'web-app-ssl', '{"cert", "key"}', 'valid', NOW() - INTERVAL '2 hours'),

('prod', 'techstart-io', 'postgres-credentials', 'techstart/prod/postgres', 'api-backend-db', '{"username", "password", "database"}', 'valid', NOW() - INTERVAL '20 minutes'),
('prod', 'techstart-io', 'jwt-secret', 'techstart/prod/jwt', 'api-backend-jwt', '{"secret"}', 'valid', NOW() - INTERVAL '25 minutes'),
('prod', 'techstart-io', 'stripe-key', 'techstart/prod/stripe', 'api-backend-stripe', '{"secret_key", "publishable_key"}', 'valid', NOW() - INTERVAL '1 hour'),
('prod', 'techstart-io', 'ssl-certificate', 'techstart/prod/ssl', 'web-app-ssl', '{"cert", "key"}', 'valid', NOW() - INTERVAL '90 minutes'),

-- Global Bank Secrets
('dev', 'global-bank', 'oracle-credentials', 'globalbank/dev/oracle', 'account-service-db', '{"username", "password", "sid", "host"}', 'valid', NOW() - INTERVAL '30 minutes'),
('dev', 'global-bank', 'encryption-key', 'globalbank/dev/encryption', 'account-service-encrypt', '{"master_key", "salt"}', 'valid', NOW() - INTERVAL '1 hour'),
('dev', 'global-bank', 'kafka-credentials', 'globalbank/dev/kafka', 'transaction-processor-kafka', '{"username", "password", "sasl_mechanism"}', 'valid', NOW() - INTERVAL '45 minutes'),

('test', 'global-bank', 'oracle-credentials', 'globalbank/test/oracle', 'account-service-db', '{"username", "password", "sid", "host"}', 'valid', NOW() - INTERVAL '45 minutes'),
('test', 'global-bank', 'encryption-key', 'globalbank/test/encryption', 'account-service-encrypt', '{"master_key", "salt"}', 'valid', NOW() - INTERVAL '1 hour'),
('test', 'global-bank', 'kafka-credentials', 'globalbank/test/kafka', 'transaction-processor-kafka', '{"username", "password", "sasl_mechanism"}', 'valid', NOW() - INTERVAL '2 hours'),

('prod', 'global-bank', 'oracle-credentials', 'globalbank/prod/oracle', 'account-service-db', '{"username", "password", "sid", "host"}', 'valid', NOW() - INTERVAL '10 minutes'),
('prod', 'global-bank', 'hsm-encryption-key', 'globalbank/prod/hsm', 'account-service-hsm', '{"key_id", "key_data", "algorithm"}', 'valid', NOW() - INTERVAL '30 minutes'),
('prod', 'global-bank', 'kafka-credentials', 'globalbank/prod/kafka', 'transaction-processor-kafka', '{"username", "password", "sasl_mechanism"}', 'sync_failed', NOW() - INTERVAL '8 hours'),
('prod', 'global-bank', 'encryption-key', 'globalbank/prod/encryption', 'transaction-processor-encrypt', '{"master_key", "salt"}', 'valid', NOW() - INTERVAL '15 minutes');

-- ============================================================================
-- 5. ADDITIONAL GITOPS DATA (ApplicationSets, Helm Sources, etc.)
-- ============================================================================

-- ApplicationSets
INSERT INTO applicationsets (name, customer_id, app_name, spec, created_at) VALUES 
('user-service-apps', 'acme-corp', 'user-service', '{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "ApplicationSet",
  "generators": [{
    "clusters": {
      "selector": {
        "matchLabels": {"env": ["dev", "qa", "prod"]}
      }
    }
  }],
  "template": {
    "metadata": {"name": "user-service-{{name}}"},
    "spec": {
      "source": {
        "repoURL": "https://git.acme.com/services/user-service",
        "path": "helm",
        "helm": {"valueFiles": ["values-{{name}}.yaml"]}
      },
      "destination": {
        "server": "{{server}}",
        "namespace": "acme-{{name}}"
      }
    }
  }
}', NOW() - INTERVAL '5 days'),

('payment-service-apps', 'acme-corp', 'payment-service', '{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "ApplicationSet",
  "generators": [{
    "clusters": {
      "selector": {
        "matchLabels": {"env": ["dev", "qa", "prod"]}
      }
    }
  }],
  "template": {
    "metadata": {"name": "payment-service-{{name}}"},
    "spec": {
      "source": {
        "repoURL": "https://git.acme.com/services/payment-service",
        "path": "helm",
        "helm": {"valueFiles": ["values-{{name}}.yaml"]}
      },
      "destination": {
        "server": "{{server}}",
        "namespace": "acme-{{name}}"
      }
    }
  }
}', NOW() - INTERVAL '3 days'),

('web-app-apps', 'techstart-io', 'web-app', '{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "ApplicationSet",
  "generators": [{
    "clusters": {
      "selector": {
        "matchLabels": {"env": ["dev", "staging", "prod"]}
      }
    }
  }],
  "template": {
    "metadata": {"name": "web-app-{{name}}"},
    "spec": {
      "source": {
        "repoURL": "https://github.com/techstart/web-app",
        "path": "k8s/{{name}}"
      },
      "destination": {
        "server": "{{server}}",
        "namespace": "techstart-{{name}}"
      }
    }
  }
}', NOW() - INTERVAL '1 day'),

('account-service-apps', 'global-bank', 'account-service', '{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "ApplicationSet",
  "generators": [{
    "clusters": {
      "selector": {
        "matchLabels": {"env": ["dev", "test", "prod"]}
      }
    }
  }],
  "template": {
    "metadata": {"name": "account-service-{{name}}"},
    "spec": {
      "source": {
        "repoURL": "https://git.globalbank.com/core/account-service",
        "path": "deploy/{{name}}"
      },
      "destination": {
        "server": "{{server}}",
        "namespace": "gbank-{{name}}"
      }
    }
  }
}', NOW() - INTERVAL '4 days');

-- Helm Sources
INSERT INTO helm_sources (chart_name, chart_version, repository_url, customer_id, app_name, values_overrides, created_at) VALUES 
-- ACME Corp Helm Sources
('microservice', '1.2.0', 'https://charts.acme.com', 'acme-corp', 'user-service', '{
  "image": {"tag": "1.2.3", "pullPolicy": "Always"},
  "service": {"port": 8080, "type": "ClusterIP"},
  "ingress": {"enabled": true, "host": "user-service-dev.acme.com"},
  "resources": {
    "requests": {"cpu": "200m", "memory": "256Mi"},
    "limits": {"cpu": "500m", "memory": "512Mi"}
  },
  "autoscaling": {"enabled": false},
  "secrets": {"vault": {"enabled": true, "role": "user-service"}}
}', NOW() - INTERVAL '2 days'),

('microservice', '1.2.0', 'https://charts.acme.com', 'acme-corp', 'payment-service', '{
  "image": {"tag": "2.1.5", "pullPolicy": "Always"},
  "service": {"port": 8080, "type": "ClusterIP"},
  "vault": {"enabled": true, "role": "payment-service"},
  "pci": {"enabled": true, "network_policies": true},
  "resources": {
    "requests": {"cpu": "300m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  }
}', NOW() - INTERVAL '1 day'),

-- TechStart.io Helm Sources
('webapp', '0.8.2', 'https://charts.bitnami.com/bitnami', 'techstart-io', 'web-app', '{
  "image": {"tag": "0.8.3", "pullPolicy": "IfNotPresent"},
  "service": {"type": "ClusterIP", "port": 80},
  "ingress": {"enabled": true, "className": "nginx"},
  "resources": {"limits": {"memory": "256Mi", "cpu": "300m"}},
  "nodeSelector": {"workload": "frontend"}
}', NOW() - INTERVAL '6 hours'),

('nodejs-app', '0.5.1', 'https://charts.bitnami.com/bitnami', 'techstart-io', 'api-backend', '{
  "image": {"tag": "0.5.2", "pullPolicy": "Always"},
  "postgresql": {"enabled": true, "auth": {"database": "techstart"}},
  "redis": {"enabled": true, "auth": {"enabled": false}},
  "resources": {
    "requests": {"cpu": "200m", "memory": "256Mi"},
    "limits": {"cpu": "500m", "memory": "512Mi"}
  }
}', NOW() - INTERVAL '12 hours'),

-- Global Bank Helm Sources
('java-service', '2.5.1', 'https://charts.globalbank.com/internal', 'global-bank', 'account-service', '{
  "image": {"tag": "2.5.2", "pullPolicy": "Always"},
  "database": {"type": "oracle", "ssl": true},
  "security": {
    "enabled": true,
    "encryption": true,
    "hsm_integration": true
  },
  "compliance": {"sox": true, "pci_dss": true},
  "monitoring": {"newrelic": true, "prometheus": true}
}', NOW() - INTERVAL '1 day'),

('java-service', '2.5.1', 'https://charts.globalbank.com/internal', 'global-bank', 'transaction-processor', '{
  "image": {"tag": "1.8.3", "pullPolicy": "Always"},
  "kafka": {
    "enabled": true,
    "partitions": 12,
    "replication_factor": 3,
    "security_protocol": "SASL_SSL"
  },
  "monitoring": {"enabled": true, "alerts": true},
  "resources": {
    "requests": {"cpu": "400m", "memory": "512Mi"},
    "limits": {"cpu": "1000m", "memory": "1Gi"}
  }
}', NOW() - INTERVAL '2 hours');

-- Cluster Configs
INSERT INTO cluster_configs (cluster_name, customer_id, environment_name, endpoint_url, auth_config, metadata, created_at) VALUES 
-- ACME Corp Clusters
('acme-dev-cluster', 'acme-corp', 'dev', 'https://k8s-dev.acme.com', '{
  "type": "service_account",
  "token_path": "/var/run/secrets/kubernetes.io/serviceaccount/token",
  "ca_cert_path": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
}', '{
  "region": "us-east-1",
  "node_count": 3,
  "version": "1.28.5",
  "instance_types": ["t3.medium"],
  "provider": "EKS"
}', NOW() - INTERVAL '5 days'),

('acme-qa-cluster', 'acme-corp', 'qa', 'https://k8s-qa.acme.com', '{
  "type": "service_account",
  "token_path": "/var/run/secrets/kubernetes.io/serviceaccount/token",
  "ca_cert_path": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
}', '{
  "region": "us-east-1",
  "node_count": 5,
  "version": "1.28.5",
  "instance_types": ["t3.large"],
  "provider": "EKS"
}', NOW() - INTERVAL '4 days'),

('acme-prod-cluster', 'acme-corp', 'prod', 'https://k8s-prod.acme.com', '{
  "type": "service_account",
  "token_path": "/var/run/secrets/kubernetes.io/serviceaccount/token",
  "ca_cert_path": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
}', '{
  "region": "us-east-1",
  "node_count": 20,
  "version": "1.28.3",
  "instance_types": ["t3.xlarge", "t3.2xlarge"],
  "provider": "EKS"
}', NOW() - INTERVAL '10 days'),

-- TechStart.io Clusters  
('techstart-dev', 'techstart-io', 'dev', 'https://dev.k8s.techstart.io', '{
  "type": "kubeconfig",
  "context": "dev-cluster"
}', '{
  "provider": "GKE",
  "region": "us-central1",
  "node_count": 2,
  "version": "1.29.1",
  "machine_type": "e2-standard-2"
}', NOW() - INTERVAL '2 days'),

('techstart-staging', 'techstart-io', 'staging', 'https://staging.k8s.techstart.io', '{
  "type": "kubeconfig",
  "context": "staging-cluster"
}', '{
  "provider": "GKE",
  "region": "us-central1",
  "node_count": 4,
  "version": "1.29.1",
  "machine_type": "e2-standard-4"
}', NOW() - INTERVAL '1 day'),

('techstart-prod', 'techstart-io', 'prod', 'https://prod.k8s.techstart.io', '{
  "type": "kubeconfig", 
  "context": "prod-cluster"
}', '{
  "provider": "GKE",
  "region": "us-central1",
  "node_count": 8,
  "version": "1.29.0",
  "machine_type": "e2-standard-8"
}', NOW() - INTERVAL '7 days'),

-- Global Bank Clusters
('gbank-dev-us-east', 'global-bank', 'dev', 'https://dev-k8s.globalbank.com', '{
  "type": "cert_auth",
  "cert_path": "/etc/kubernetes/pki/client.crt",
  "key_path": "/etc/kubernetes/pki/client.key",
  "ca_cert_path": "/etc/kubernetes/pki/ca.crt"
}', '{
  "region": "us-east-1",
  "node_count": 6,
  "version": "1.27.8",
  "instance_types": ["m5.large"],
  "compliance": ["SOX"],
  "provider": "self-managed"
}', NOW() - INTERVAL '3 days'),

('gbank-test-us-east', 'global-bank', 'test', 'https://test-k8s.globalbank.com', '{
  "type": "cert_auth",
  "cert_path": "/etc/kubernetes/pki/client.crt",
  "key_path": "/etc/kubernetes/pki/client.key",
  "ca_cert_path": "/etc/kubernetes/pki/ca.crt"
}', '{
  "region": "us-east-1",
  "node_count": 12,
  "version": "1.27.8",
  "instance_types": ["m5.xlarge"],
  "compliance": ["SOX"],
  "provider": "self-managed"
}', NOW() - INTERVAL '2 days'),

('gbank-prod-multi', 'global-bank', 'prod', 'https://prod-k8s.globalbank.com', '{
  "type": "cert_auth",
  "cert_path": "/etc/kubernetes/pki/client.crt",
  "key_path": "/etc/kubernetes/pki/client.key",
  "ca_cert_path": "/etc/kubernetes/pki/ca.crt"
}', '{
  "regions": ["us-east-1", "us-west-2"],
  "node_count": 50,
  "version": "1.27.8",
  "instance_types": ["m5.2xlarge", "m5.4xlarge"],
  "compliance": ["SOX", "PCI-DSS", "FIPS-140-2"],
  "ha": true,
  "provider": "self-managed"
}', NOW() - INTERVAL '14 days');

-- Customer Branches (Git repository branches for customer isolation)
INSERT INTO customer_branches (customer_id, repository_url, branch_name, branch_status, last_commit_sha, last_updated, created_at) VALUES 
('acme-corp', 'https://git.acme.com/infra', 'customer/acme-corp', 'active', 'a1b2c3d4e5f6789012345678901234567890abcd', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '30 days'),
('acme-corp', 'https://git.acme.com/services/user-service', 'customer/acme-corp', 'active', 'b2c3d4e5f6789012345678901234567890abcdef', NOW() - INTERVAL '4 hours', NOW() - INTERVAL '25 days'),
('acme-corp', 'https://git.acme.com/services/payment-service', 'customer/acme-corp', 'active', 'c3d4e5f6789012345678901234567890abcdef12', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '20 days'),

('techstart-io', 'https://github.com/techstart/gitops', 'main', 'active', 'd4e5f6789012345678901234567890abcdef1234', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '15 days'),
('techstart-io', 'https://github.com/techstart/web-app', 'main', 'active', 'e5f6789012345678901234567890abcdef123456', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '12 days'),
('techstart-io', 'https://github.com/techstart/api-backend', 'develop', 'active', 'f6789012345678901234567890abcdef12345678', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '10 days'),

('global-bank', 'https://git.globalbank.com/platform', 'customer/global-bank', 'active', '1234567890abcdef123456789012345678901234', NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '60 days'),
('global-bank', 'https://git.globalbank.com/core/account-service', 'customer/global-bank', 'active', '234567890abcdef1234567890123456789012345', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '45 days'),
('global-bank', 'https://git.globalbank.com/core/transaction-processor', 'hotfix/global-bank', 'active', '34567890abcdef12345678901234567890123456', NOW() - INTERVAL '5 minutes', NOW() - INTERVAL '2 days');

-- Context Deployments (current deployment status)
INSERT INTO context_deployments (context_name, customer_id, deployment_status, argocd_app_name, helm_release_name, kubernetes_namespace, last_deployed, sync_status, health_status, created_at) VALUES 
-- ACME Corp Deployments
('user-service-dev', 'acme-corp', 'deployed', 'user-service-dev', 'user-service', 'acme-dev', NOW() - INTERVAL '2 hours', 'Synced', 'Healthy', NOW() - INTERVAL '1 day'),
('user-service-qa', 'acme-corp', 'deployed', 'user-service-qa', 'user-service', 'acme-qa', NOW() - INTERVAL '6 hours', 'Synced', 'Healthy', NOW() - INTERVAL '2 days'),
('user-service-prod', 'acme-corp', 'deployed', 'user-service-prod', 'user-service', 'acme-prod', NOW() - INTERVAL '4 hours', 'Synced', 'Healthy', NOW() - INTERVAL '5 days'),

('payment-service-dev', 'acme-corp', 'deployed', 'payment-service-dev', 'payment-service', 'acme-dev', NOW() - INTERVAL '1 hour', 'Synced', 'Healthy', NOW() - INTERVAL '1 day'),
('payment-service-qa', 'acme-corp', 'deployed', 'payment-service-qa', 'payment-service', 'acme-qa', NOW() - INTERVAL '8 hours', 'Synced', 'Healthy', NOW() - INTERVAL '2 days'),
('payment-service-prod', 'acme-corp', 'deployed', 'payment-service-prod', 'payment-service', 'acme-prod', NOW() - INTERVAL '6 hours', 'Synced', 'Healthy', NOW() - INTERVAL '5 days'),

('api-gateway-dev', 'acme-corp', 'deployed', 'api-gateway-dev', 'nginx-ingress', 'acme-dev', NOW() - INTERVAL '3 hours', 'Synced', 'Healthy', NOW() - INTERVAL '2 days'),
('api-gateway-prod', 'acme-corp', 'deployed', 'api-gateway-prod', 'nginx-ingress', 'acme-prod', NOW() - INTERVAL '1 hour', 'Synced', 'Healthy', NOW() - INTERVAL '7 days'),

-- TechStart.io Deployments
('web-app-dev', 'techstart-io', 'deployed', 'web-app-dev', 'webapp', 'techstart-dev', NOW() - INTERVAL '30 minutes', 'Synced', 'Healthy', NOW() - INTERVAL '6 hours'),
('web-app-staging', 'techstart-io', 'deployed', 'web-app-staging', 'webapp', 'techstart-staging', NOW() - INTERVAL '4 hours', 'Synced', 'Healthy', NOW() - INTERVAL '12 hours'),
('web-app-prod', 'techstart-io', 'deployed', 'web-app-prod', 'webapp', 'techstart-prod', NOW() - INTERVAL '8 hours', 'Synced', 'Healthy', NOW() - INTERVAL '1 day'),

('api-backend-dev', 'techstart-io', 'deployed', 'api-backend-dev', 'nodejs-app', 'techstart-dev', NOW() - INTERVAL '45 minutes', 'Synced', 'Healthy', NOW() - INTERVAL '6 hours'),
('api-backend-staging', 'techstart-io', 'failed', 'api-backend-staging', 'nodejs-app', 'techstart-staging', NOW() - INTERVAL '2 hours', 'OutOfSync', 'Unhealthy', NOW() - INTERVAL '12 hours'),
('api-backend-prod', 'techstart-io', 'deployed', 'api-backend-prod', 'nodejs-app', 'techstart-prod', NOW() - INTERVAL '3 hours', 'Synced', 'Healthy', NOW() - INTERVAL '1 day'),

-- Global Bank Deployments
('account-service-dev', 'global-bank', 'deployed', 'account-service-dev', 'java-service', 'gbank-dev', NOW() - INTERVAL '1 hour', 'Synced', 'Healthy', NOW() - INTERVAL '2 days'),
('account-service-test', 'global-bank', 'deployed', 'account-service-test', 'java-service', 'gbank-test', NOW() - INTERVAL '3 hours', 'Synced', 'Healthy', NOW() - INTERVAL '1 day'),
('account-service-prod', 'global-bank', 'deployed', 'account-service-prod', 'java-service', 'gbank-prod', NOW() - INTERVAL '30 minutes', 'Synced', 'Healthy', NOW() - INTERVAL '3 days'),

('transaction-processor-dev', 'global-bank', 'deployed', 'transaction-processor-dev', 'java-service', 'gbank-dev', NOW() - INTERVAL '2 hours', 'Synced', 'Healthy', NOW() - INTERVAL '1 day'),
('transaction-processor-test', 'global-bank', 'deployed', 'transaction-processor-test', 'java-service', 'gbank-test', NOW() - INTERVAL '45 minutes', 'Synced', 'Degraded', NOW() - INTERVAL '8 hours'),
('transaction-processor-prod', 'global-bank', 'deployed', 'transaction-processor-prod', 'java-service', 'gbank-prod', NOW() - INTERVAL '15 minutes', 'Synced', 'Healthy', NOW() - INTERVAL '2 hours');

-- Pod Environment Validations (Vault secret correlation with running pods)
INSERT INTO pod_env_validations (context_name, customer_id, pod_name, environment_variable, vault_secret_ref, validation_status, last_validated, created_at) VALUES 
-- ACME Corp Pod Environment Validations
('user-service-dev', 'acme-corp', 'user-service-7b8c9d-abc12', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '1 day'),
('user-service-dev', 'acme-corp', 'user-service-7b8c9d-abc12', 'JWT_SECRET', 'jwt-signing-key', 'valid', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '1 day'),
('user-service-dev', 'acme-corp', 'user-service-7b8c9d-def34', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '1 day'),
('user-service-dev', 'acme-corp', 'user-service-7b8c9d-def34', 'REDIS_PASSWORD', 'redis-credentials', 'valid', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '1 day'),

('payment-service-dev', 'acme-corp', 'payment-service-5a6b7c-xyz89', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '2 days'),
('payment-service-dev', 'acme-corp', 'payment-service-5a6b7c-xyz89', 'STRIPE_SECRET_KEY', 'stripe-api-key', 'valid', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '2 days'),
('payment-service-dev', 'acme-corp', 'payment-service-5a6b7c-xyz89', 'VAULT_TOKEN', 'vault-token', 'valid', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '2 days'),

('user-service-qa', 'acme-corp', 'user-service-9d1e2f-ghi56', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '3 days'),
('user-service-qa', 'acme-corp', 'user-service-9d1e2f-ghi56', 'JWT_SECRET', 'jwt-signing-key', 'valid', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '3 days'),
('user-service-qa', 'acme-corp', 'user-service-9d1e2f-klm78', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '3 days'),

('payment-service-prod', 'acme-corp', 'payment-service-3c4d5e-mno78', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '5 days'),
('payment-service-prod', 'acme-corp', 'payment-service-3c4d5e-mno78', 'STRIPE_SECRET_KEY', 'stripe-api-key', 'stale', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '5 days'),
('payment-service-prod', 'acme-corp', 'payment-service-3c4d5e-pqr90', 'VAULT_TOKEN', 'vault-token', 'valid', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '5 days'),
('payment-service-prod', 'acme-corp', 'payment-service-3c4d5e-stu12', 'DB_PASSWORD', 'database-credentials', 'valid', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '5 days'),

-- TechStart.io Pod Environment Validations
('api-backend-dev', 'techstart-io', 'api-backend-1f2g3h-stu12', 'DATABASE_PASSWORD', 'postgres-credentials', 'valid', NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '6 hours'),
('api-backend-dev', 'techstart-io', 'api-backend-1f2g3h-stu12', 'JWT_SECRET', 'jwt-secret', 'valid', NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '6 hours'),

('api-backend-staging', 'techstart-io', 'api-backend-4h5i6j-vwx34', 'DATABASE_PASSWORD', 'postgres-credentials', 'invalid', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '12 hours'),
('api-backend-staging', 'techstart-io', 'api-backend-4h5i6j-vwx34', 'STRIPE_PUBLISHABLE_KEY', 'stripe-key', 'valid', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '12 hours'),
('api-backend-staging', 'techstart-io', 'api-backend-4h5i6j-yza56', 'JWT_SECRET', 'jwt-secret', 'valid', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '12 hours'),

('api-backend-prod', 'techstart-io', 'api-backend-7j8k9l-yza56', 'DATABASE_PASSWORD', 'postgres-credentials', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day'),
('api-backend-prod', 'techstart-io', 'api-backend-7j8k9l-yza56', 'STRIPE_SECRET_KEY', 'stripe-key', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day'),
('api-backend-prod', 'techstart-io', 'api-backend-7j8k9l-bcd78', 'DATABASE_PASSWORD', 'postgres-credentials', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day'),
('api-backend-prod', 'techstart-io', 'api-backend-7j8k9l-efg90', 'JWT_SECRET', 'jwt-secret', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day'),

-- Global Bank Pod Environment Validations
('account-service-dev', 'global-bank', 'account-service-9m0n1o-efg90', 'ORACLE_PASSWORD', 'oracle-credentials', 'valid', NOW() - INTERVAL '25 minutes', NOW() - INTERVAL '2 days'),
('account-service-dev', 'global-bank', 'account-service-9m0n1o-efg90', 'ENCRYPTION_KEY', 'encryption-key', 'valid', NOW() - INTERVAL '25 minutes', NOW() - INTERVAL '2 days'),
('account-service-dev', 'global-bank', 'account-service-9m0n1o-hij12', 'ORACLE_PASSWORD', 'oracle-credentials', 'valid', NOW() - INTERVAL '25 minutes', NOW() - INTERVAL '2 days'),

('transaction-processor-test', 'global-bank', 'tx-processor-2o3p4q-hij12', 'ORACLE_PASSWORD', 'oracle-credentials', 'valid', NOW() - INTERVAL '40 minutes', NOW() - INTERVAL '1 day'),
('transaction-processor-test', 'global-bank', 'tx-processor-2o3p4q-hij12', 'KAFKA_PASSWORD', 'kafka-credentials', 'valid', NOW() - INTERVAL '40 minutes', NOW() - INTERVAL '1 day'),
('transaction-processor-test', 'global-bank', 'tx-processor-2o3p4q-klm34', 'ENCRYPTION_KEY', 'encryption-key', 'valid', NOW() - INTERVAL '40 minutes', NOW() - INTERVAL '1 day'),

('account-service-prod', 'global-bank', 'account-service-5q6r7s-klm34', 'ORACLE_PASSWORD', 'oracle-credentials', 'valid', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '3 days'),
('account-service-prod', 'global-bank', 'account-service-5q6r7s-klm34', 'HSM_KEY_ID', 'hsm-encryption-key', 'valid', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '3 days'),
('account-service-prod', 'global-bank', 'account-service-5q6r7s-nop56', 'ENCRYPTION_KEY', 'encryption-key', 'valid', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '3 days'),
('account-service-prod', 'global-bank', 'account-service-5q6r7s-pqr78', 'ORACLE_PASSWORD', 'oracle-credentials', 'valid', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '3 days'),

('transaction-processor-prod', 'global-bank', 'tx-processor-8s9t0u-qrs78', 'KAFKA_PASSWORD', 'kafka-credentials', 'failed', NOW() - INTERVAL '8 hours', NOW() - INTERVAL '1 day'),
('transaction-processor-prod', 'global-bank', 'tx-processor-8s9t0u-qrs78', 'ENCRYPTION_KEY', 'encryption-key', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day'),
('transaction-processor-prod', 'global-bank', 'tx-processor-8s9t0u-stu90', 'ORACLE_PASSWORD', 'oracle-credentials', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day'),
('transaction-processor-prod', 'global-bank', 'tx-processor-8s9t0u-vwx12', 'ENCRYPTION_KEY', 'encryption-key', 'valid', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '1 day');

COMMIT;

-- ============================================================================
-- VERIFICATION QUERIES - Run these to verify sample data was created
-- ============================================================================

-- Summary by customer
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
    COUNT(c.name) as deployed_environments,
    string_agg(c.environment_reference, ', ' ORDER BY c.environment_reference) as environments
FROM apps a
JOIN contexts c ON c.app_reference = a.name AND c.customer_id = a.customer_id
GROUP BY a.customer_id, a.name 
ORDER BY a.customer_id, a.name;

-- Vault secrets by environment
SELECT 
    customer_id,
    environment_name,
    COUNT(*) as secrets_count,
    COUNT(CASE WHEN validation_status = 'valid' THEN 1 END) as valid_secrets,
    COUNT(CASE WHEN validation_status != 'valid' THEN 1 END) as invalid_secrets
FROM vault_static_secrets
GROUP BY customer_id, environment_name
ORDER BY customer_id, environment_name;

-- Context versions and replicas
SELECT 
    c.customer_id,
    c.app_reference as application,
    c.environment_reference as environment,
    c.spec->>'version' as version,
    c.spec->>'replicas' as replicas
FROM contexts c
ORDER BY c.customer_id, c.app_reference, c.environment_reference;

-- Sample data totals
SELECT 
    'environments' as table_name, COUNT(*) as records FROM environments
UNION ALL
SELECT 
    'apps' as table_name, COUNT(*) as records FROM apps
UNION ALL
SELECT 
    'contexts' as table_name, COUNT(*) as records FROM contexts
UNION ALL
SELECT 
    'vault_static_secrets' as table_name, COUNT(*) as records FROM vault_static_secrets
UNION ALL
SELECT 
    'context_deployments' as table_name, COUNT(*) as records FROM context_deployments
UNION ALL
SELECT 
    'pod_env_validations' as table_name, COUNT(*) as records FROM pod_env_validations;

-- Done! ContextOps sample data generation complete.