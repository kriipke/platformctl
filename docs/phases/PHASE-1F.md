# PHASE 1F: Basic Deployment

**Duration:** 2-3 days  
**Prerequisites:** Phase 1E completed  
**Deliverable:** Containerized services with Kubernetes manifests and basic networking

---

## Overview

Create Docker containers for all services and Kubernetes deployment manifests. Establish the foundation for running ContextOps in Kubernetes with proper service discovery, health checks, and basic networking.

## Success Criteria

✅ Docker images built for all services  
✅ Multi-stage builds optimized for Go applications  
✅ Kubernetes manifests for all components  
✅ ConfigMaps and Secrets properly configured  
✅ Service discovery working between components  
✅ Health checks integrated with Kubernetes probes  
✅ Basic ingress configuration  
✅ Development and production overlays  

---

## Implementation Tasks

### Task 1: Docker Images

**File: `Dockerfile`**

```dockerfile
# Multi-stage build for Go applications
FROM golang:1.21-alpine AS builder

# Install required packages
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/${SERVICE_NAME} ./cmd/${SERVICE_NAME}

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -s /bin/sh contextops

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
ARG SERVICE_NAME
COPY --from=builder /app/bin/${SERVICE_NAME} /app/contextops

# Copy migration files if they exist
COPY migrations/ /app/migrations/

# Change ownership
RUN chown -R contextops:contextops /app

# Switch to non-root user
USER contextops

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/contextops", "healthcheck"] || exit 1

# Command to run
ENTRYPOINT ["/app/contextops"]
```

**File: `build/docker/build.sh`**

```bash
#!/bin/bash

set -e

REGISTRY="${REGISTRY:-contextops}"
TAG="${TAG:-latest}"

# List of services to build
SERVICES=(
    "gateway"
    "aggregator" 
    "vault-svc"
    "argocd-svc"
    "newrelic-svc"
    "kube-svc"
    "git-svc"
    "cli"
)

echo "Building ContextOps Docker images..."

for service in "${SERVICES[@]}"; do
    echo "Building ${service}..."
    
    docker build \
        --build-arg SERVICE_NAME=${service} \
        --tag ${REGISTRY}/${service}:${TAG} \
        --file Dockerfile \
        .
        
    echo "Built ${REGISTRY}/${service}:${TAG}"
done

echo "All images built successfully!"

# Optional: Push to registry
if [ "$PUSH" = "true" ]; then
    for service in "${SERVICES[@]}"; do
        echo "Pushing ${REGISTRY}/${service}:${TAG}..."
        docker push ${REGISTRY}/${service}:${TAG}
    done
fi
```

### Task 2: Kubernetes Base Manifests

**File: `deploy/k8s/namespace.yaml`**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: contextops
  labels:
    app.kubernetes.io/name: contextops
    app.kubernetes.io/version: "1.0"
```

**File: `deploy/k8s/configmap.yaml`**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: contextops-config
  namespace: contextops
data:
  LOG_LEVEL: "info"
  DATABASE_URL: "postgres://contextops:password@postgres:5432/contextops?sslmode=disable"
  RABBITMQ_URL: "amqp://contextops:password@rabbitmq:5672/"
  PORT: ":8080"
```

**File: `deploy/k8s/secrets.yaml`**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: contextops-secrets
  namespace: contextops
type: Opaque
data:
  # Base64 encoded values - replace in production
  DATABASE_PASSWORD: cGFzc3dvcmQ=  # password
  RABBITMQ_PASSWORD: cGFzc3dvcmQ=  # password
```

### Task 3: Service Deployments

**File: `deploy/k8s/gateway-deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: contextops
  labels:
    app.kubernetes.io/name: gateway
    app.kubernetes.io/component: api-gateway
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: gateway
  template:
    metadata:
      labels:
        app.kubernetes.io/name: gateway
        app.kubernetes.io/component: api-gateway
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: contextops-gateway
      containers:
      - name: gateway
        image: contextops/gateway:latest
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: PORT
          value: ":8080"
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: LOG_LEVEL
        - name: DATABASE_URL
          valueFrom:
            configMapKeyRef:
              name: contextops-config  
              key: DATABASE_URL
        - name: RABBITMQ_URL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: RABBITMQ_URL
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
---
apiVersion: v1
kind: Service
metadata:
  name: gateway
  namespace: contextops
  labels:
    app.kubernetes.io/name: gateway
spec:
  selector:
    app.kubernetes.io/name: gateway
  ports:
  - name: http
    port: 80
    targetPort: 8080
  type: ClusterIP
```

**File: `deploy/k8s/aggregator-deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aggregator
  namespace: contextops
  labels:
    app.kubernetes.io/name: aggregator
    app.kubernetes.io/component: aggregator
spec:
  replicas: 1  # Single replica for now to avoid concurrency issues
  selector:
    matchLabels:
      app.kubernetes.io/name: aggregator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: aggregator
        app.kubernetes.io/component: aggregator
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: contextops-aggregator
      containers:
      - name: aggregator
        image: contextops/aggregator:latest
        ports:
        - containerPort: 8080
          name: metrics
        env:
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: LOG_LEVEL
        - name: DATABASE_URL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: DATABASE_URL
        - name: RABBITMQ_URL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: RABBITMQ_URL
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /health  
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 256Mi
```

### Task 4: Integration Service Deployments

**File: `deploy/k8s/vault-svc-deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vault-svc
  namespace: contextops
  labels:
    app.kubernetes.io/name: vault-svc
    app.kubernetes.io/component: integration-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: vault-svc
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vault-svc
        app.kubernetes.io/component: integration-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080" 
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: contextops-vault
      containers:
      - name: vault-svc
        image: contextops/vault-svc:latest
        ports:
        - containerPort: 8080
          name: metrics
        env:
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: LOG_LEVEL
        - name: DATABASE_URL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: DATABASE_URL
        - name: RABBITMQ_URL
          valueFrom:
            configMapKeyRef:
              name: contextops-config
              key: RABBITMQ_URL
        # Mount service account token for Vault Kubernetes auth
        volumeMounts:
        - name: service-account-token
          mountPath: /var/run/secrets/kubernetes.io/serviceaccount
          readOnly: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 256Mi
      volumes:
      - name: service-account-token
        projected:
          sources:
          - serviceAccountToken:
              path: token
              expirationSeconds: 7200
```

### Task 5: RBAC Configuration

**File: `deploy/k8s/rbac.yaml`**

```yaml
# Gateway service account
apiVersion: v1
kind: ServiceAccount
metadata:
  name: contextops-gateway
  namespace: contextops
---
# Aggregator service account
apiVersion: v1
kind: ServiceAccount
metadata:
  name: contextops-aggregator
  namespace: contextops
---
# Vault service account (for Vault Kubernetes auth)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: contextops-vault
  namespace: contextops
---
# Kubernetes service account (for kubeconfig-less operation)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: contextops-kube
  namespace: contextops
---
# ClusterRole for Kubernetes service (namespace-scoped access)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: contextops-kube-reader
rules:
- apiGroups: [""]
  resources: ["pods", "services", "endpoints", "events", "configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list", "watch"]
---
# ClusterRoleBinding for Kubernetes service
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: contextops-kube-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: contextops-kube-reader
subjects:
- kind: ServiceAccount
  name: contextops-kube
  namespace: contextops
```

### Task 6: Infrastructure Dependencies

**File: `deploy/k8s/postgres.yaml`**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: contextops
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: contextops
        - name: POSTGRES_USER
          value: contextops
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: contextops-secrets
              key: DATABASE_PASSWORD
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 1Gi
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: contextops
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
  clusterIP: None
```

**File: `deploy/k8s/rabbitmq.yaml`**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
  namespace: contextops
spec:
  serviceName: rabbitmq
  replicas: 1
  selector:
    matchLabels:
      app: rabbitmq
  template:
    metadata:
      labels:
        app: rabbitmq
    spec:
      containers:
      - name: rabbitmq
        image: rabbitmq:3.11-management
        ports:
        - containerPort: 5672
          name: amqp
        - containerPort: 15672
          name: management
        env:
        - name: RABBITMQ_DEFAULT_USER
          value: contextops
        - name: RABBITMQ_DEFAULT_PASS
          valueFrom:
            secretKeyRef:
              name: contextops-secrets
              key: RABBITMQ_PASSWORD
        volumeMounts:
        - name: rabbitmq-data
          mountPath: /var/lib/rabbitmq
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 1Gi
  volumeClaimTemplates:
  - metadata:
      name: rabbitmq-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 5Gi
---
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq
  namespace: contextops
spec:
  selector:
    app: rabbitmq
  ports:
  - name: amqp
    port: 5672
    targetPort: 5672
  - name: management
    port: 15672
    targetPort: 15672
```

### Task 7: Ingress Configuration

**File: `deploy/k8s/ingress.yaml`**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: contextops-ingress
  namespace: contextops
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
    nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  ingressClassName: nginx
  rules:
  - host: contextops.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gateway
            port:
              number: 80
      - path: /rabbitmq
        pathType: Prefix
        backend:
          service:
            name: rabbitmq
            port:
              number: 15672
```

### Task 8: Deployment Scripts

**File: `deploy/scripts/deploy.sh`**

```bash
#!/bin/bash

set -e

NAMESPACE="${NAMESPACE:-contextops}"
KUBECTL="${KUBECTL:-kubectl}"

echo "Deploying ContextOps to Kubernetes..."

# Apply namespace
echo "Creating namespace..."
${KUBECTL} apply -f deploy/k8s/namespace.yaml

# Apply RBAC
echo "Setting up RBAC..."
${KUBECTL} apply -f deploy/k8s/rbac.yaml

# Apply configuration
echo "Applying configuration..."
${KUBECTL} apply -f deploy/k8s/configmap.yaml
${KUBECTL} apply -f deploy/k8s/secrets.yaml

# Deploy infrastructure
echo "Deploying infrastructure..."
${KUBECTL} apply -f deploy/k8s/postgres.yaml
${KUBECTL} apply -f deploy/k8s/rabbitmq.yaml

# Wait for infrastructure
echo "Waiting for infrastructure to be ready..."
${KUBECTL} wait --for=condition=ready pod -l app=postgres -n ${NAMESPACE} --timeout=300s
${KUBECTL} wait --for=condition=ready pod -l app=rabbitmq -n ${NAMESPACE} --timeout=300s

# Run database migrations
echo "Running database migrations..."
${KUBECTL} run migration-job --image=contextops/gateway:latest --rm -it --restart=Never \
  -n ${NAMESPACE} -- /app/contextops migrate up

# Deploy services
echo "Deploying services..."
${KUBECTL} apply -f deploy/k8s/gateway-deployment.yaml
${KUBECTL} apply -f deploy/k8s/aggregator-deployment.yaml
${KUBECTL} apply -f deploy/k8s/vault-svc-deployment.yaml
# Apply other service deployments...

# Deploy ingress
echo "Deploying ingress..."
${KUBECTL} apply -f deploy/k8s/ingress.yaml

# Wait for deployments
echo "Waiting for services to be ready..."
${KUBECTL} wait --for=condition=available deployment -l app.kubernetes.io/component=api-gateway -n ${NAMESPACE} --timeout=300s
${KUBECTL} wait --for=condition=available deployment -l app.kubernetes.io/component=aggregator -n ${NAMESPACE} --timeout=300s

echo "ContextOps deployment complete!"
echo "Access the application at: http://contextops.local"
echo "RabbitMQ management: http://contextops.local/rabbitmq"
```

### Task 9: Environment Overlays with Kustomize

**File: `deploy/overlays/development/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: contextops-dev

resources:
- ../../k8s

namePrefix: dev-

replicas:
- name: gateway
  count: 1
- name: aggregator  
  count: 1
- name: vault-svc
  count: 1

configMapGenerator:
- name: contextops-config
  behavior: replace
  literals:
  - LOG_LEVEL=debug
  - DATABASE_URL=postgres://contextops:password@postgres:5432/contextops_dev?sslmode=disable

images:
- name: contextops/gateway
  newTag: dev
- name: contextops/aggregator
  newTag: dev
```

**File: `deploy/overlays/production/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: contextops

resources:
- ../../k8s

replicas:
- name: gateway
  count: 3
- name: aggregator
  count: 2
- name: vault-svc
  count: 3

configMapGenerator:
- name: contextops-config
  behavior: replace
  literals:
  - LOG_LEVEL=info
  - DATABASE_URL=postgres://contextops:password@postgres:5432/contextops?sslmode=disable

# Resource limits for production
patchesStrategicMerge:
- production-resources.yaml

images:
- name: contextops/gateway
  newTag: v1.0.0
- name: contextops/aggregator
  newTag: v1.0.0
```

---

## Validation Checklist

Before marking Phase 1F complete:

**Docker Images:**
- [ ] All service images build successfully
- [ ] Images use multi-stage builds for optimization
- [ ] Images run as non-root user
- [ ] Health check commands work in containers

**Kubernetes Manifests:**
- [ ] All services deploy without errors
- [ ] Service discovery works between components
- [ ] Health checks integrate with Kubernetes probes
- [ ] Resource limits and requests configured
- [ ] RBAC permissions are minimal and functional

**Infrastructure:**
- [ ] PostgreSQL statefulset runs and persists data
- [ ] RabbitMQ statefulset runs and maintains queues
- [ ] Database migrations complete successfully
- [ ] Services can connect to infrastructure components

**Networking:**
- [ ] Internal service communication works
- [ ] Ingress provides external access to gateway
- [ ] Prometheus metrics endpoints accessible
- [ ] Health check endpoints respond correctly

**Configuration:**
- [ ] ConfigMaps and Secrets applied correctly
- [ ] Environment variables loaded in containers
- [ ] Service accounts have appropriate permissions
- [ ] Namespace isolation working

---

## Next Steps

Upon completion, Phase 1F provides:
- Complete Kubernetes deployment for ContextOps
- Containerized services ready for orchestration  
- Infrastructure dependencies properly managed
- Foundation for production operations

**Handoff to Phase 1G:** The testing framework can now include integration tests against the Kubernetes deployment and validate the complete system behavior.