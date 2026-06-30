# ContextOps: GitOps-Optimized Application Monitoring Platform

**Status:** Development Complete - All Services Build Successfully  
**Version:** 1.1  
**Date:** 2026-01-23  
**Target Platform:** DigitalOcean Kubernetes  

---

## 🎯 What is ContextOps?

ContextOps is a **production-ready GitOps monitoring platform** that provides unified visibility across your ApplicationSets, Helm deployments, Vault secrets, and Kubernetes workloads. Built specifically for teams using ArgoCD ApplicationSets with customer branch isolation and multi-environment deployments.

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

### 2. Deploy to DigitalOcean Kubernetes

#### Step 1: Create DOKS Cluster (if needed)
```bash
# Create a DOKS cluster
doctl kubernetes cluster create contextops-cluster \
  --region nyc3 \
  --size s-2vcpu-2gb \
  --count 3 \
  --tag contextops \
  --wait

# Get cluster credentials
doctl kubernetes cluster kubeconfig save contextops-cluster
```

#### Step 2: Configure Container Registry
```bash
# Create DigitalOcean Container Registry (if needed)
doctl registry create contextops-registry

# Configure Docker authentication
doctl registry login

# Tag and push images
docker tag platformctl-gateway:latest registry.digitalocean.com/contextops-registry/platformctl-gateway:latest
docker tag platformctl-gitops-aggregator:latest registry.digitalocean.com/contextops-registry/platformctl-gitops-aggregator:latest

# Push all images
docker push registry.digitalocean.com/contextops-registry/platformctl-gateway:latest
docker push registry.digitalocean.com/contextops-registry/platformctl-gitops-aggregator:latest
# ... repeat for all services
```

#### Step 3: Deploy Infrastructure Dependencies
```bash
# Create namespace
kubectl create namespace contextops

# Deploy PostgreSQL
kubectl apply -f deployments/base/postgres.yaml -n contextops

# Deploy RabbitMQ  
kubectl apply -f deployments/base/rabbitmq.yaml -n contextops

# Wait for dependencies to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n contextops --timeout=300s
kubectl wait --for=condition=ready pod -l app=rabbitmq -n contextops --timeout=300s
```

#### Step 4: Configure Secrets
```bash
# Create database secret
kubectl create secret generic postgres-secret \
  --from-literal=username=contextops \
  --from-literal=password=$(openssl rand -base64 32) \
  --from-literal=database=contextops \
  -n contextops

# Create RabbitMQ secret
kubectl create secret generic rabbitmq-secret \
  --from-literal=username=contextops \
  --from-literal=password=$(openssl rand -base64 32) \
  -n contextops

# Create application config
kubectl create configmap contextops-config \
  --from-file=deployments/base/config.yaml \
  -n contextops
```

#### Step 5: Deploy ContextOps Services
```bash
# Update image references in kustomization.yaml
cd deployments/base
kustomize edit set image platformctl-gateway=registry.digitalocean.com/contextops-registry/platformctl-gateway:latest
kustomize edit set image platformctl-gitops-aggregator=registry.digitalocean.com/contextops-registry/platformctl-gitops-aggregator:latest

# Deploy all services
kubectl apply -k deployments/base -n contextops

# Verify deployment
kubectl get pods -n contextops
kubectl get services -n contextops
```

#### Step 6: Expose Services (DigitalOcean LoadBalancer)
```bash
# Create LoadBalancer service for API Gateway
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: contextops-gateway-lb
  namespace: contextops
  annotations:
    service.beta.kubernetes.io/do-loadbalancer-name: "contextops-gateway"
    service.beta.kubernetes.io/do-loadbalancer-protocol: "http"
    service.beta.kubernetes.io/do-loadbalancer-healthcheck-path: "/health"
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
  selector:
    app: contextops-gateway
EOF

# Get LoadBalancer IP
kubectl get service contextops-gateway-lb -n contextops
```

### 3. Verify Installation
```bash
# Check all pods are running
kubectl get pods -n contextops

# Check services
kubectl get svc -n contextops

# Test API endpoint (replace <LOAD_BALANCER_IP> with actual IP)
curl http://<LOAD_BALANCER_IP>/health

# View logs
kubectl logs -l app=contextops-gateway -n contextops
```

---

## 📋 Configuration

### Environment Variables
Create a `config.yaml` file for your environment:

```yaml
# Database Configuration
database:
  host: "postgres.contextops.svc.cluster.local"
  port: 5432
  database: "contextops"
  username: "${POSTGRES_USERNAME}"
  password: "${POSTGRES_PASSWORD}"
  ssl_mode: "require"

# RabbitMQ Configuration  
rabbitmq:
  url: "amqp://${RABBITMQ_USERNAME}:${RABBITMQ_PASSWORD}@rabbitmq.contextops.svc.cluster.local:5672/"
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
kubectl get pods -n contextops
kubectl describe pod <pod-name> -n contextops

# Check service logs
kubectl logs -f deployment/contextops-gateway -n contextops
kubectl logs -f deployment/contextops-aggregator -n contextops

# Check RabbitMQ queues
kubectl port-forward service/rabbitmq 15672:15672 -n contextops
# Visit http://localhost:15672 (guest/guest)
```

### Common Issues

#### 1. Services Not Starting
```bash
# Check resource constraints
kubectl top pods -n contextops

# Check events
kubectl get events -n contextops --sort-by='.lastTimestamp'

# Check configuration
kubectl describe configmap contextops-config -n contextops
```

#### 2. Database Connection Issues
```bash
# Test database connectivity
kubectl run postgres-test --rm -i --tty --image postgres:14 -- bash
# Inside pod: psql -h postgres.contextops.svc.cluster.local -U contextops
```

#### 3. RabbitMQ Message Issues
```bash
# Check queue status
kubectl exec -it deployment/rabbitmq -n contextops -- rabbitmqctl list_queues

# Check message flow
kubectl logs -f deployment/contextops-app-sync-svc -n contextops | grep correlation_id
```

---

## 🔒 Security

### RBAC Configuration
```bash
# Create service account
kubectl create serviceaccount contextops-sa -n contextops

# Apply RBAC manifests
kubectl apply -f deployments/base/rbac.yaml -n contextops
```

### Network Policies
```bash
# Apply network policies for micro-segmentation
kubectl apply -f deployments/base/network-policies.yaml -n contextops
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
  name: contextops-gateway-hpa
  namespace: contextops
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: contextops-gateway
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