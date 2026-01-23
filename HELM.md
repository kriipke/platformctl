# ContextOps Helm Chart Installation Guide

This guide provides instructions for deploying ContextOps using the Helm chart instead of the original kustomize manifests.

## Quick Start

### Prerequisites

- Kubernetes cluster (1.19+)
- Helm 3.2.0+
- kubectl configured to access your cluster

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/kriipke/platformctl.git
   cd platformctl
   ```

2. **Install with default values:**
   ```bash
   helm install contextops ./helm/contextops
   ```

3. **Check deployment status:**
   ```bash
   kubectl get pods -n contextops
   ```

4. **Access the application:**
   ```bash
   # Get the external LoadBalancer IP (if enabled)
   kubectl get svc contextops-external-lb -n contextops
   
   # Or use port-forward for local access
   kubectl port-forward -n contextops svc/contextops-gateway 8080:80
   ```

## Production Deployment

For production deployments, create a custom values file:

```yaml
# production-values.yaml
secrets:
  database:
    username: "contextops_prod"
    password: "CHANGE_THIS_PASSWORD"
  rabbitmq:
    username: "contextops_prod"
    password: "CHANGE_THIS_PASSWORD"
  jwt:
    secret: "your-jwt-secret-here"

postgres:
  persistence:
    size: "50Gi"
    storageClass: "fast-ssd"
  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "1000m"

rabbitmq:
  persistence:
    size: "20Gi"
    storageClass: "fast-ssd"

gateway:
  replicaCount: 3
  resources:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "1Gi"
      cpu: "1000m"

loadBalancer:
  enabled: true
  provider: "aws"  # or azure, gcp, digitalocean
  aws:
    type: "nlb"
    scheme: "internet-facing"
```

Deploy with custom values:
```bash
helm install contextops ./helm/contextops -f production-values.yaml
```

## Cloud Provider Specific Configurations

### AWS

```yaml
# aws-values.yaml
loadBalancer:
  provider: aws
  aws:
    type: nlb
    scheme: internet-facing
    crossZoneLoadBalancing: "true"
    targetType: instance

postgres:
  persistence:
    storageClass: "gp3"

rabbitmq:
  persistence:
    storageClass: "gp3"
```

### Azure

```yaml
# azure-values.yaml
loadBalancer:
  provider: azure
  azure:
    internal: "false"

postgres:
  persistence:
    storageClass: "managed-premium"

rabbitmq:
  persistence:
    storageClass: "managed-premium"
```

### Google Cloud Platform

```yaml
# gcp-values.yaml
loadBalancer:
  provider: gcp
  gcp:
    type: "External"

postgres:
  persistence:
    storageClass: "ssd"

rabbitmq:
  persistence:
    storageClass: "ssd"
```

## External Dependencies

### Using External PostgreSQL

For production, you might want to use a managed PostgreSQL service:

```yaml
# external-postgres-values.yaml
postgres:
  enabled: false
  external:
    enabled: true
    connectionString: "postgres://user:pass@your-postgres-host:5432/contextops"
```

### Using External RabbitMQ

Similarly, you can use a managed RabbitMQ service:

```yaml
# external-rabbitmq-values.yaml
rabbitmq:
  enabled: false
  external:
    enabled: true
    connectionString: "amqp://user:pass@your-rabbitmq-host:5672/"
```

## Migration from Kustomize

If you're currently using the kustomize deployment, follow these steps:

1. **Backup your current data:**
   ```bash
   # Backup PostgreSQL
   kubectl exec -n contextops postgres-pod -- pg_dump -U contextops contextops > backup.sql
   
   # Backup RabbitMQ definitions (if needed)
   kubectl exec -n contextops rabbitmq-pod -- rabbitmqctl export_definitions /tmp/definitions.json
   ```

2. **Uninstall kustomize deployment:**
   ```bash
   kubectl delete -k deployments/base/
   ```

3. **Install Helm chart:**
   ```bash
   helm install contextops ./helm/contextops -f your-values.yaml
   ```

4. **Restore data:**
   ```bash
   # Restore PostgreSQL
   kubectl exec -i -n contextops contextops-postgres-pod -- psql -U contextops contextops < backup.sql
   ```

## Customization

### Disabling Services

You can disable individual services:

```yaml
integrationServices:
  app-sync-svc:
    enabled: false
  environment-validation-svc:
    enabled: true
  context-correlation-svc:
    enabled: true
  multi-environment-kube-svc:
    enabled: false
  customer-git-branch-svc:
    enabled: true

gitopsAggregator:
  enabled: true

kubernetesIntegration:
  enabled: false
```

### Adding Custom Environment Variables

```yaml
gateway:
  extraEnv:
    CUSTOM_VAR: "custom-value"
    ANOTHER_VAR: "another-value"

config:
  extraEnv:
    GLOBAL_CUSTOM_VAR: "global-value"
```

## Monitoring and Observability

The chart includes built-in support for metrics and health checks:

- Prometheus metrics are exposed on port 9090 for all services
- Health checks are available on port 8081
- Structured JSON logging is configured by default

To scrape metrics with Prometheus:

```yaml
# Enable metrics LoadBalancer (optional)
metricsLoadBalancer:
  enabled: true
```

## Troubleshooting

### Common Issues

1. **Pods stuck in Pending state:**
   - Check if PVCs can be provisioned
   - Verify storage class exists
   - Check resource limits

2. **Database connection issues:**
   - Verify PostgreSQL is running
   - Check database credentials in secrets
   - Review database URL construction

3. **RabbitMQ connection issues:**
   - Check RabbitMQ pod status
   - Verify RabbitMQ credentials
   - Check service discovery

### Debugging Commands

```bash
# Check all resources
kubectl get all -n contextops

# Check events
kubectl get events -n contextops --sort-by='.lastTimestamp'

# Check logs
kubectl logs -n contextops -l app.kubernetes.io/name=contextops -f

# Describe problematic pods
kubectl describe pod -n contextops <pod-name>
```

## Upgrading

To upgrade the Helm release:

```bash
helm upgrade contextops ./helm/contextops -f your-values.yaml
```

To check what would change before upgrading:

```bash
helm diff upgrade contextops ./helm/contextops -f your-values.yaml
```

## Uninstalling

To completely remove ContextOps:

```bash
helm uninstall contextops
```

**Note:** This will not delete persistent volumes. To delete them:

```bash
kubectl delete pvc -n contextops contextops-postgres-pvc
kubectl delete pvc -n contextops contextops-rabbitmq-pvc
kubectl delete namespace contextops
```

## Support

For more information and support:

- [Chart Documentation](./helm/contextops/README.md)
- [Main Project Documentation](./README.md)
- [GitHub Issues](https://github.com/kriipke/platformctl/issues)