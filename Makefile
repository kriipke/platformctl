# Platformctl Makefile for building and deploying GitOps monitoring platform

# Variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
REGISTRY ?= platformctl
GHCR_REGISTRY ?= ghcr.io/kriipke
NAMESPACE ?= platformctl-stage
IMAGE_TAG ?= develop
DOCKER_BUILDKIT ?= 1

# Go variables
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

# Services to build
SERVICES = gateway gitops-aggregator app-sync-svc environment-validation-svc context-correlation-svc multi-environment-kube-svc customer-git-branch-svc

# Kubernetes variables
KUBECTL ?= kubectl
HELM ?= helm

# Build flags
LDFLAGS = -w -s \
	-X main.Version=$(VERSION) \
	-X main.CommitSHA=$(COMMIT_SHA) \
	-X main.BuildDate=$(BUILD_DATE)

.PHONY: help
help: ## Display this help message
	@echo "Platformctl GitOps Monitoring Platform"
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development targets
.PHONY: deps
deps: ## Download and verify dependencies
	go mod download
	go mod verify
	go mod tidy

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...
	goimports -w -local platformctl .

.PHONY: lint
lint: ## Run linters
	golangci-lint run
	
.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: test
test: ## Run tests
	go test -race -cover ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -tags=integration -race ./test/integration/...

# Build targets
.PHONY: build
build: $(SERVICES) ## Build all services

.PHONY: build-local
build-local: ## Build all services for local architecture
	@for service in $(SERVICES); do \
		echo "Building $$service locally..."; \
		go build -ldflags="$(LDFLAGS)" -o bin/$$service ./cmd/$$service/; \
	done

.PHONY: $(SERVICES)
$(SERVICES): ## Build individual service (e.g., make gateway)
	@echo "Building $@..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -ldflags="$(LDFLAGS)" -a -installsuffix cgo \
		-o bin/$@ ./cmd/$@/

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	docker system prune -f
	docker image prune -f

# Docker targets
.PHONY: docker-build
docker-build: ## Build Docker images for all services
	@for service in $(SERVICES); do \
		echo "Building Docker image for $$service..."; \
		docker build \
			--build-arg SERVICE_NAME=$$service \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT_SHA=$(COMMIT_SHA) \
			--build-arg BUILD_DATE=$(BUILD_DATE) \
			-t $(REGISTRY)/$$service:$(VERSION) \
			-t $(REGISTRY)/$$service:latest \
			.; \
	done

.PHONY: docker-build-%
docker-build-%: ## Build Docker image for specific service (e.g., make docker-build-gateway)
	@service_name=$(subst docker-build-,,$@); \
	echo "Building Docker image for $$service_name..."; \
	docker build \
		--build-arg SERVICE_NAME=$$service_name \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_SHA=$(COMMIT_SHA) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(REGISTRY)/$$service_name:$(VERSION) \
		-t $(REGISTRY)/$$service_name:latest \
		.

.PHONY: docker-push
docker-push: ## Push Docker images to registry
	@for service in $(SERVICES); do \
		echo "Pushing $$service:$(VERSION)..."; \
		docker push $(REGISTRY)/$$service:$(VERSION); \
		docker push $(REGISTRY)/$$service:latest; \
	done

.PHONY: docker-run-%
docker-run-%: ## Run specific service in Docker (e.g., make docker-run-gateway)
	@service_name=$(subst docker-run-,,$@); \
	echo "Running $$service_name in Docker..."; \
	docker run -it --rm \
		-p 8080:8080 -p 8081:8081 -p 9090:9090 \
		-e DATABASE_URL=postgres://platformctl:platformctl@host.docker.internal:5432/platformctl \
		-e RABBITMQ_URL=amqp://platformctl:platformctl@host.docker.internal:5672/ \
		$(REGISTRY)/$$service_name:latest

# Database targets
.PHONY: db-migrate
db-migrate: ## Run database migrations
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	migrate -path ./migrations -database "$(DATABASE_URL)" up

.PHONY: db-migrate-down
db-migrate-down: ## Rollback database migrations
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	migrate -path ./migrations -database "$(DATABASE_URL)" down

.PHONY: db-reset
db-reset: ## Reset database (drop and recreate)
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	migrate -path ./migrations -database "$(DATABASE_URL)" drop -f
	migrate -path ./migrations -database "$(DATABASE_URL)" up

# Kubernetes targets
.PHONY: k8s-namespace
k8s-namespace: ## Create Kubernetes namespace
	$(KUBECTL) create namespace $(NAMESPACE) --dry-run=client -o yaml | $(KUBECTL) apply -f -

.PHONY: k8s-lint
k8s-lint: ## Lint + render the Helm chart for both environments
	$(HELM) lint charts/platformctl -f charts/platformctl/values-stage.yaml
	$(HELM) lint charts/platformctl -f charts/platformctl/values-prod.yaml

.PHONY: k8s-deploy-stage
k8s-deploy-stage: ## Deploy to stage via Helm (namespace platformctl-stage). Override IMAGE_TAG=...
	$(HELM) upgrade --install platformctl charts/platformctl \
		--namespace platformctl-stage --create-namespace \
		-f charts/platformctl/values-stage.yaml \
		--set image.tag=$(IMAGE_TAG) --atomic --timeout 10m

.PHONY: k8s-deploy-prod
k8s-deploy-prod: ## Deploy to prod via Helm (namespace platformctl-prod). Override IMAGE_TAG=...
	$(HELM) upgrade --install platformctl charts/platformctl \
		--namespace platformctl-prod --create-namespace \
		-f charts/platformctl/values-prod.yaml \
		--set image.tag=$(IMAGE_TAG) --atomic --timeout 10m

.PHONY: k8s-deploy
k8s-deploy: k8s-deploy-stage ## Deploy to default (stage) environment

.PHONY: k8s-delete
k8s-delete: ## Uninstall the Helm release (NAMESPACE selects the target namespace)
	$(HELM) uninstall platformctl --namespace $(NAMESPACE) || true

.PHONY: k8s-logs-%
k8s-logs-%: ## View logs for specific service (e.g., make k8s-logs-gateway)
	@service_name=$(subst k8s-logs-,,$@); \
	$(KUBECTL) logs -l app=$$service_name -n $(NAMESPACE) -f

.PHONY: k8s-status
k8s-status: ## Show Kubernetes deployment status
	$(KUBECTL) get all -n $(NAMESPACE)
	$(KUBECTL) get ingress -n $(NAMESPACE)
	$(KUBECTL) get configmap -n $(NAMESPACE)
	$(KUBECTL) get secret -n $(NAMESPACE)

.PHONY: k8s-port-forward-%
k8s-port-forward-%: ## Port forward to specific service (e.g., make k8s-port-forward-gateway)
	@service_name=$(subst k8s-port-forward-,,$@); \
	port=$$(case $$service_name in \
		gateway) echo "8080:80";; \
		*) echo "9090:9090";; \
	esac); \
	echo "Port forwarding $$service_name on $$port..."; \
	$(KUBECTL) port-forward -n $(NAMESPACE) service/platformctl-$$service_name $$port

.PHONY: k8s-shell-%
k8s-shell-%: ## Get shell in specific service pod (e.g., make k8s-shell-gateway)
	@service_name=$(subst k8s-shell-,,$@); \
	pod=$$($(KUBECTL) get pods -l app=$$service_name -n $(NAMESPACE) -o jsonpath='{.items[0].metadata.name}'); \
	$(KUBECTL) exec -it $$pod -n $(NAMESPACE) -- /bin/sh

# Development environment targets
.PHONY: dev-up
dev-up: ## Start development environment (PostgreSQL, RabbitMQ)
	docker-compose -f deployments/docker-compose.dev.yml up -d

.PHONY: dev-down
dev-down: ## Stop development environment
	docker-compose -f deployments/docker-compose.dev.yml down

.PHONY: dev-logs
dev-logs: ## View development environment logs
	docker-compose -f deployments/docker-compose.dev.yml logs -f

# CI/CD targets
.PHONY: ci
ci: deps fmt vet lint test ## Run CI pipeline

.PHONY: release
release: ci docker-build docker-push ## Build and push release

# GitHub Container Registry targets
.PHONY: docker-build-ghcr
docker-build-ghcr: ## Build and tag images for GitHub Container Registry
	@for service in $(SERVICES); do \
		echo "Building $$service for GHCR..."; \
		DOCKER_BUILDKIT=$(DOCKER_BUILDKIT) docker build \
			--build-arg SERVICE_NAME=$$service \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT_SHA=$(COMMIT_SHA) \
			--build-arg BUILD_DATE=$(BUILD_DATE) \
			-t $(GHCR_REGISTRY)/platformctl-$$service:$(VERSION) \
			-t $(GHCR_REGISTRY)/platformctl-$$service:latest \
			.; \
	done

.PHONY: docker-push-ghcr
docker-push-ghcr: ## Push images to GitHub Container Registry
	@for service in $(SERVICES); do \
		echo "Pushing $$service to GHCR..."; \
		docker push $(GHCR_REGISTRY)/platformctl-$$service:$(VERSION); \
		docker push $(GHCR_REGISTRY)/platformctl-$$service:latest; \
	done

.PHONY: docker-login-ghcr
docker-login-ghcr: ## Login to GitHub Container Registry
	@echo "Logging into GitHub Container Registry..."
	@echo "$(GITHUB_TOKEN)" | docker login ghcr.io -u $(GITHUB_ACTOR) --password-stdin

.PHONY: ghcr-release
ghcr-release: ci docker-build-ghcr docker-push-ghcr ## Build and push release to GHCR

# Monitoring targets
.PHONY: metrics
metrics: ## View Prometheus metrics
	@echo "Gateway metrics:"
	@curl -s http://localhost:9090/metrics | grep platformctl_http_requests_total | head -5
	@echo "\nAggregator metrics:"
	@curl -s http://localhost:9091/metrics | grep platformctl_commands_processed_total | head -5

.PHONY: health
health: ## Check service health
	@for port in 8080 8081 8082 8083 8084 8085 8086; do \
		echo -n "Port $$port: "; \
		curl -s http://localhost:$$port/health | jq -r '.status // "unhealthy"' 2>/dev/null || echo "unreachable"; \
	done

# Documentation targets
.PHONY: docs
docs: ## Generate documentation
	@echo "Generating API documentation..."
	@swagger generate spec -o ./docs/api/swagger.json

.PHONY: readme
readme: ## Update README with current service information
	@echo "Current services:" > README.services.md
	@for service in $(SERVICES); do \
		echo "- $$service" >> README.services.md; \
	done

# Security targets
.PHONY: security-scan
security-scan: ## Run security scans
	@echo "Running security scans..."
	govulncheck ./...
	docker scout quickview $(REGISTRY)/gateway:latest || true

# Environment setup targets
.PHONY: install-tools
install-tools: ## Install required development tools
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/go-swagger/go-swagger/cmd/swagger@latest

.PHONY: setup-dev
setup-dev: install-tools deps dev-up ## Complete development environment setup
	@echo "Development environment setup complete!"
	@echo "Services will be available at:"
	@echo "- Gateway: http://localhost:8080"
	@echo "- PostgreSQL: localhost:5432"
	@echo "- RabbitMQ Management: http://localhost:15672"

# Default target
.DEFAULT_GOAL := help