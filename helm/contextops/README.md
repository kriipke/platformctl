# ContextOps Helm Chart

A GitOps-optimized application monitoring platform designed for DevOps engineers managing applications deployed via ArgoCD ApplicationSets, Helm umbrella charts, and Vault-secured secrets across multiple environments and customers.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- PV provisioner support in the underlying infrastructure (if persistence is enabled)

## Installing the Chart

To install the chart with the release name `contextops`:

```bash
helm install contextops ./helm/contextops
```

The command deploys ContextOps on the Kubernetes cluster with the default configuration. The [Parameters](#parameters) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `contextops` deployment:

```bash
helm uninstall contextops
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the ContextOps chart and their default values.

### Global Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `global.imageRegistry` | Global Docker image registry | `""` |
| `global.imagePullSecrets` | Global Docker registry secret names as an array | `[]` |

### Common Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `namespace` | Namespace to install ContextOps | `contextops` |
| `nameOverride` | String to partially override contextops.fullname | `""` |
| `fullnameOverride` | String to fully override contextops.fullname | `""` |

### Service Account Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `serviceAccount.create` | Specifies whether a service account should be created | `true` |
| `serviceAccount.annotations` | Annotations to add to the service account | `{}` |
| `serviceAccount.name` | The name of the service account to use | `""` |

### RBAC Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `rbac.create` | Specifies whether RBAC resources should be created | `true` |
| `rbac.rules.argocd` | Enable ArgoCD RBAC rules | `true` |
| `rbac.rules.helm` | Enable Helm RBAC rules | `true` |
| `rbac.rules.externalSecrets` | Enable External Secrets RBAC rules | `true` |
| `rbac.rules.vault` | Enable Vault RBAC rules | `true` |

### Gateway Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `gateway.enabled` | Enable Gateway service | `true` |
| `gateway.replicaCount` | Number of Gateway replicas | `2` |
| `gateway.image.repository` | Gateway image repository | `ghcr.io/kriipke/platformctl-gateway` |
| `gateway.image.pullPolicy` | Gateway image pull policy | `Always` |
| `gateway.image.tag` | Override the image tag | `""` |
| `gateway.service.type` | Gateway service type | `ClusterIP` |
| `gateway.service.port` | Gateway service port | `8080` |

### GitOps Aggregator Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `gitopsAggregator.enabled` | Enable GitOps Aggregator service | `true` |
| `gitopsAggregator.replicaCount` | Number of GitOps Aggregator replicas | `1` |
| `gitopsAggregator.image.repository` | GitOps Aggregator image repository | `ghcr.io/kriipke/platformctl-gitops-aggregator` |

### Integration Services Parameters

Each integration service (app-sync-svc, environment-validation-svc, context-correlation-svc, multi-environment-kube-svc, customer-git-branch-svc) supports the following parameters:

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `integrationServices.<service>.enabled` | Enable the integration service | `true` |
| `integrationServices.<service>.replicaCount` | Number of replicas | `2` |
| `integrationServices.<service>.image.repository` | Image repository | `ghcr.io/kriipke/platformctl-<service>` |

### PostgreSQL Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `postgres.enabled` | Deploy PostgreSQL as part of the release | `true` |
| `postgres.auth.username` | PostgreSQL username | `contextops` |
| `postgres.auth.password` | PostgreSQL password | `contextops` |
| `postgres.auth.database` | PostgreSQL database name | `contextops` |
| `postgres.persistence.size` | PostgreSQL Persistent Volume size | `10Gi` |
| `postgres.persistence.storageClass` | PostgreSQL storage class | `do-block-storage` |

### RabbitMQ Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `rabbitmq.enabled` | Deploy RabbitMQ as part of the release | `true` |
| `rabbitmq.auth.username` | RabbitMQ username | `contextops` |
| `rabbitmq.auth.password` | RabbitMQ password | `contextops` |
| `rabbitmq.persistence.size` | RabbitMQ Persistent Volume size | `5Gi` |
| `rabbitmq.persistence.storageClass` | RabbitMQ storage class | `do-block-storage` |

### Load Balancer Parameters

| Name               | Description                     | Value |
| ------------------ | ------------------------------- | ----- |
| `loadBalancer.enabled` | Enable LoadBalancer service | `true` |
| `loadBalancer.provider` | Cloud provider (aws, azure, gcp, digitalocean) | `digitalocean` |
| `loadBalancer.externalTrafficPolicy` | External traffic policy | `Local` |

## Example Configurations

### Using External PostgreSQL

```yaml
postgres:
  enabled: false
  external:
    enabled: true
    connectionString: "postgres://user:pass@host:5432/dbname"
```

### Using External RabbitMQ

```yaml
rabbitmq:
  enabled: false
  external:
    enabled: true
    connectionString: "amqp://user:pass@host:5672/"
```

### AWS Load Balancer Configuration

```yaml
loadBalancer:
  provider: aws
  aws:
    type: nlb
    scheme: internet-facing
    crossZoneLoadBalancing: "true"
    targetType: instance
```

### Custom Resource Limits

```yaml
gateway:
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 500m
      memory: 512Mi
```

## Upgrading

To upgrade the chart:

```bash
helm upgrade contextops ./helm/contextops
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n contextops
```

### View Logs

```bash
kubectl logs -n contextops -l app.kubernetes.io/name=contextops
```

### Access RabbitMQ Management UI

```bash
kubectl port-forward -n contextops svc/contextops-rabbitmq 15672:15672
```

Then visit http://localhost:15672

### Access PostgreSQL

```bash
kubectl port-forward -n contextops svc/contextops-postgres 5432:5432
```

Then connect using: `psql -h localhost -U contextops -d contextops`

## Contributing

Please read the [main project README](../../README.md) for information about contributing to ContextOps.

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](../../LICENSE) file for details.