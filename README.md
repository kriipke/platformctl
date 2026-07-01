# Platformctl: GitOps-Optimized Application Monitoring Platform

**Status:** Development Complete - All Services Build Successfully  
**Version:** 1.1  
**Date:** 2026-01-23  
**Target Platform:** DigitalOcean Kubernetes  

---

## 🎯 What is Platformctl?

Platformctl is a **production-ready GitOps monitoring platform** that provides unified visibility across your ApplicationSets, Helm deployments, Vault secrets, and Kubernetes workloads. Built specifically for teams using ArgoCD ApplicationSets with customer branch isolation and multi-environment deployments.

### Key Features
- **🔄 ApplicationSet Deep Integration**: Monitors Bootstrap Applications → ApplicationSets → Generated Applications
- **🌍 Multi-Environment Correlation**: Unified dashboards across dev/qa/uat/prod environments  
- **🔐 Vault-Kubernetes Secret Bridge**: Real-time secret sync validation and pod environment correlation
- **🏢 Customer Branch Isolation**: Separate configurations and resource tracking per customer
- **📊 Application-Centric Dashboards**: Per-app views with environment tabs and cross-environment comparison
- **⚡ Event-Driven Architecture**: RabbitMQ-coordinated microservices with GitOps-aware messaging

---

## 🏗️ Current Architecture Status

### ✅ **Completed Services** (All Building Successfully)
- **platformctl-gateway** - API Gateway with Context CRUD and GitOps actions
- **platformctl-gitops-aggregator** - Multi-environment state aggregation and read model  
- **platformctl-app-sync-svc** - ApplicationSet monitoring and sync orchestration
- **platformctl-environment-validation-svc** - Cross-environment validation and compliance
- **platformctl-context-correlation-svc** - Context pairing and relationship management
- **platformctl-multi-environment-kube-svc** - Multi-cluster Kubernetes monitoring
- **platformctl-customer-git-branch-svc** - Customer branch tracking and values correlation

### 🔧 **Core Infrastructure**
- **PostgreSQL** - Context storage, read model, and audit logs
- **RabbitMQ** - GitOps command orchestration and result correlation  
- **Redis** - Application cache and cross-cluster data (optional)

---

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- DigitalOcean Kubernetes Cluster (DOKS)
- `doctl` CLI configured
- `kubectl` configured for your DOKS cluster

### 1. Clone and Build
```bash
git clone https://github.com/kriipke/platformctl
cd platformctl

# Build all services locally (verify everything works)
make build

# Build Docker images
make docker-build
```

### 2. Deploy with Helm

Platformctl deploys as a single Helm chart (`charts/platformctl`) with one values
file per environment. Everything runs on one DigitalOcean cluster, isolated by
namespace:

| Environment | Namespace           | Values file                            |
|-------------|---------------------|----------------------------------------|
| stage       | `platformctl-stage` | `charts/platformctl/values-stage.yaml` |
| prod        | `platformctl-prod`  | `charts/platformctl/values-prod.yaml`  |

Postgres is **external** (DigitalOcean managed); RabbitMQ is bundled in-cluster by
the chart (toggle with `rabbitmq.enabled`). The chart **references** two database
secrets per namespace but never creates them — set those up once (below).

#### One-time setup per environment

**a) Database roles and databases** — run on the managed Postgres as an admin:

```sql
CREATE ROLE platformctl_stage LOGIN PASSWORD '<PASSWORD>';
CREATE DATABASE platformctl_stage OWNER platformctl_stage;

CREATE ROLE platformctl_prod LOGIN PASSWORD '<PASSWORD>';
CREATE DATABASE platformctl_prod OWNER platformctl_prod;
```

DigitalOcean exposes two ports: `25060` (direct) and `25061` (PgBouncer pool). The
gateway runs schema migrations on startup and must use the **direct** connection;
every other service uses the **pool**.

**b) Database secrets** — create both secrets in each namespace. Fill in
`data.DATABASE_URL` with a base64-encoded connection string matching the commented
format, then `kubectl apply -f secrets.yaml`:

```yaml
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: platformctl-credentials
  namespace: platformctl-prod
data:
  # postgresql://platformctl_prod:<PASSWORD>@<SUBDOMAIN>.db.ondigitalocean.com:25060/platformctl_prod?sslmode=require
  DATABASE_URL: ''
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: platformctl-pool-credentials
  namespace: platformctl-prod
data:
  # postgresql://platformctl_prod:<PASSWORD>@<SUBDOMAIN>.db.ondigitalocean.com:25061/platformctl_prod_pool?sslmode=require
  DATABASE_URL: ''
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: platformctl-credentials
  namespace: platformctl-stage
data:
  # postgresql://platformctl_stage:<PASSWORD>@<SUBDOMAIN>.db.ondigitalocean.com:25060/platformctl_stage?sslmode=require
  DATABASE_URL: ''
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: platformctl-pool-credentials
  namespace: platformctl-stage
data:
  # postgresql://platformctl_stage:<PASSWORD>@<SUBDOMAIN>.db.ondigitalocean.com:25061/platformctl_stage_pool?sslmode=require
  DATABASE_URL: ''
```

> `platformctl-credentials` (direct :25060) is read only by the gateway;
> `platformctl-pool-credentials` (pool :25061) is read by every other service.
> The `_pool` names are DigitalOcean PgBouncer **pool names**, not separate
> databases — create the pool in the DO console (or `doctl databases pool create`)
> targeting the base database; you do not create them with SQL.

#### Deploy via CI/CD (GitHub Actions)

The `Deploy` workflow (`.github/workflows/deploy.yml`) runs `helm upgrade --install`:

| Trigger                   | Deploys to                              |
|---------------------------|-----------------------------------------|
| push to `main`            | **stage** (image tag `latest`)          |
| push tag `v*.*.*`         | **prod** (image tag = the git tag)      |
| **Run workflow** (manual) | choose `stage`/`prod` + optional tag    |

Images are built by the `Build and Push Container Images` workflow on the same
event; the deploy job waits for the gateway image before proceeding.

Required repository **Secrets**:

| Secret           | Purpose                                                                 |
|------------------|-------------------------------------------------------------------------|
| `KUBECONFIG_B64` | base64 kubeconfig (`doctl kubernetes cluster kubeconfig save <cluster>` then `base64 -w0 ~/.kube/config`) |
| `GHCR_PULL_PAT`  | PAT with `read:packages`; used to create the in-namespace `ghcr-pull` pull secret |

#### Deploy manually / from a laptop

```bash
# Point kubectl at the cluster
doctl kubernetes cluster kubeconfig save <cluster>

# Prod (immutable tag-style image)
helm upgrade --install platformctl charts/platformctl \
  --namespace platformctl-prod \
  -f charts/platformctl/values-prod.yaml \
  --set image.tag=v1.2.3 \
  --atomic --timeout 10m

# Render locally without touching the cluster
helm template platformctl charts/platformctl \
  -n platformctl-prod -f charts/platformctl/values-prod.yaml --set image.tag=v1.2.3
```

### 3. Verify Installation
```bash
# Check all pods are running
kubectl get pods -n platformctl-prod

# Check services
kubectl get svc -n platformctl-prod

# Gateway health (via port-forward)
kubectl port-forward svc/platformctl-gateway 8080:80 -n platformctl-prod
curl http://localhost:8080/health

# View gateway logs
kubectl logs -l app=gateway -n platformctl-prod
```

---

## 📋 Configuration

### Environment Variables
Create a `config.yaml` file for your environment:

```yaml
# Database Configuration
database:
  host: "postgres.platformctl.svc.cluster.local"
  port: 5432
  database: "platformctl"
  username: "${POSTGRES_USERNAME}"
  password: "${POSTGRES_PASSWORD}"
  ssl_mode: "require"

# RabbitMQ Configuration  
rabbitmq:
  url: "amqp://${RABBITMQ_USERNAME}:${RABBITMQ_PASSWORD}@rabbitmq.platformctl.svc.cluster.local:5672/"
  heartbeat: "10s"
  connection_retries: 5

# Observability
observability:
  log_level: "info"
  log_format: "json" 
  enable_console_log: false
  metrics_enabled: true
  metrics_port: "9090"
  health_check_port: "8081"

# GitOps Integration
argocd:
  enabled: true
  address: "https://argocd.yourdomain.com"
  
vault:
  enabled: true
  address: "https://vault.yourdomain.com"
  
# External Services
github:
  api_url: "https://api.github.com"
  
newrelic:
  api_url: "https://api.newrelic.com"
```

### Kubernetes Resources

#### Resource Requirements
```yaml
# Example resource requests/limits
resources:
  gateway:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m" 
      memory: "512Mi"
  
  aggregator:
    requests:
      cpu: "100m"
      memory: "256Mi"
    limits:
      cpu: "500m"
      memory: "1Gi"
      
  integration-services:
    requests:
      cpu: "50m"
      memory: "64Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"
```

---

## 🔧 Development

### Local Development Setup
```bash
# Start dependencies with Docker Compose
docker-compose up -d postgres rabbitmq

# Install dependencies
go mod download

# Run database migrations
make db-migrate

# Start services locally
make run-gateway &
make run-aggregator &
make run-app-sync-svc &
```

### Build Commands
```bash
# Build all services
make build

# Run tests
make test

# Build Docker images
make docker-build

# Run integration tests
make test-integration
```

### Service Architecture
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   API Gateway   │───▶│    RabbitMQ      │───▶│ Integration Services│
│   (Port 8080)   │    │  (Port 5672)     │    │  - app-sync-svc     │
└─────────────────┘    └──────────────────┘    │  - env-validation   │
         │                       │              │  - kube-svc         │
         ▼                       ▼              │  - git-branch-svc   │
┌─────────────────┐    ┌──────────────────┐    └─────────────────────┘
│   PostgreSQL    │◀───│ GitOps Aggregator│
│   (Port 5432)   │    │   (Port 8080)    │
└─────────────────┘    └──────────────────┘
```

---

## 📚 Usage

### CLI Commands (Future)
```bash
# Context Management
platformctl context create -f myapp-context.yaml
platformctl context list
platformctl context status myapp-dev
platformctl context delete myapp-dev

# GitOps Operations
platformctl refresh myapp-dev
platformctl validate myapp-dev
platformctl sync myapp-dev --environment prod

# Multi-Environment Views
platformctl status myapp-dev --all-environments
platformctl diff myapp-dev --from dev --to prod
```

### REST API Examples
```bash
# List contexts
curl http://<LOAD_BALANCER_IP>/api/v1/contexts

# Get context status
curl http://<LOAD_BALANCER_IP>/api/v1/contexts/myapp-dev/status

# Refresh context across all environments
curl -X POST http://<LOAD_BALANCER_IP>/api/v1/contexts/myapp-dev/actions/refresh

# Get multi-environment comparison
curl http://<LOAD_BALANCER_IP>/api/v1/contexts/myapp-dev/environments/comparison
```

---

## 🔍 Monitoring & Troubleshooting

### Health Checks
```bash
# Check service health
kubectl get pods -n platformctl
kubectl describe pod <pod-name> -n platformctl

# Check service logs
kubectl logs -f deployment/platformctl-gateway -n platformctl
kubectl logs -f deployment/platformctl-aggregator -n platformctl

# Check RabbitMQ queues
kubectl port-forward service/rabbitmq 15672:15672 -n platformctl
# Visit http://localhost:15672 (guest/guest)
```

### Common Issues

#### 1. Services Not Starting
```bash
# Check resource constraints
kubectl top pods -n platformctl

# Check events
kubectl get events -n platformctl --sort-by='.lastTimestamp'

# Check configuration
kubectl describe configmap platformctl-config -n platformctl
```

#### 2. Database Connection Issues
```bash
# Test database connectivity
kubectl run postgres-test --rm -i --tty --image postgres:14 -- bash
# Inside pod: psql -h postgres.platformctl.svc.cluster.local -U platformctl
```

#### 3. RabbitMQ Message Issues
```bash
# Check queue status
kubectl exec -it deployment/rabbitmq -n platformctl -- rabbitmqctl list_queues

# Check message flow
kubectl logs -f deployment/platformctl-app-sync-svc -n platformctl | grep correlation_id
```

---

## 🔒 Security

### RBAC Configuration
ServiceAccounts and the read-only GitOps-monitoring ClusterRoles ship with the
Helm chart (`charts/platformctl/templates/{serviceaccount,rbac}.yaml`) and are
created automatically on `helm upgrade --install`. ClusterRole/Binding names are
suffixed with the release namespace so stage and prod can coexist on one cluster.
Toggle with `rbac.create` in values. To inspect what will be applied:
```bash
helm template platformctl charts/platformctl \
  -n platformctl-prod -f charts/platformctl/values-prod.yaml \
  --show-only templates/rbac.yaml
```

### Secret Management
- All sensitive configuration stored in Kubernetes Secrets
- Integration with HashiCorp Vault for external secrets
- No secret values logged or exposed in APIs
- mTLS between services (future enhancement)

---

## 📈 Scaling

### Horizontal Pod Autoscaling
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: platformctl-gateway-hpa
  namespace: platformctl
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: platformctl-gateway
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Database Scaling
- Use DigitalOcean Managed PostgreSQL for production
- Configure connection pooling (PgBouncer)
- Implement read replicas for aggregator queries

---

## 🚀 Production Deployment Checklist

### Pre-Deployment
- [ ] DOKS cluster configured with appropriate node sizes
- [ ] DigitalOcean Container Registry set up
- [ ] DNS configured for LoadBalancer endpoints  
- [ ] SSL/TLS certificates provisioned
- [ ] Monitoring and alerting configured
- [ ] Backup strategy implemented

### Post-Deployment
- [ ] Health checks passing for all services
- [ ] RabbitMQ queues processing messages
- [ ] Database connections stable
- [ ] External integrations (ArgoCD, Vault) working
- [ ] Load testing completed
- [ ] Disaster recovery procedures tested

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

### Development Guidelines
- All services must build successfully (verified in CI)
- Follow Go coding standards and best practices
- Add tests for new functionality
- Update documentation for API changes
- Ensure security best practices are followed

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🆘 Support

- **Documentation**: See `docs/` directory for detailed guides
- **Issues**: Report bugs and feature requests via GitHub Issues  
- **Architecture**: See `CLAUDE.md` for comprehensive development guide
- **Roadmap**: See `ROADMAP.md` for planned features and timeline

---

**Built with ❤️ for DevOps teams using GitOps workflows**