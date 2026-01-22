module github.com/contextops/platformctl

go 1.23.0

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0

	// GitOps integrations
	github.com/argoproj/argo-cd/v2 v2.8.4
	github.com/caarlos0/env/v9 v9.0.0

	// JSON/YAML processing for GitOps configurations
	github.com/ghodss/yaml v1.0.0

	// Validation and configuration
	github.com/go-playground/validator/v10 v10.27.0

	// Authentication and authorization
	github.com/golang-jwt/jwt/v5 v5.2.0
	github.com/golang-migrate/migrate/v4 v4.16.2

	// Utilities
	github.com/google/uuid v1.6.0
	github.com/gorilla/handlers v1.5.1
	// Core web framework and routing
	github.com/gorilla/mux v1.8.1
	github.com/hashicorp/vault/api v1.10.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/joho/godotenv v1.4.0

	// Database and migrations
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.23.2

	// Message queue for GitOps events
	github.com/rabbitmq/amqp091-go v1.9.0

	// Logging and observability
	github.com/sirupsen/logrus v1.9.3

	// Testing and development
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.24.1
	github.com/tidwall/gjson v1.16.0
	go.uber.org/zap v1.26.0
	golang.org/x/crypto v0.41.0
	helm.sh/helm/v3 v3.12.3
	k8s.io/api v0.28.2
	k8s.io/apimachinery v0.28.2

	// Kubernetes client libraries
	k8s.io/client-go v0.28.2
	sigs.k8s.io/controller-runtime v0.16.2
)

require (
	github.com/bytedance/sonic v1.14.0 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v24.0.7+incompatible // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/gin-gonic/gin v1.11.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.54.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/arch v0.20.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
