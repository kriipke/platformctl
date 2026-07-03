package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kriipke/platformctl/internal/auth"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/internal/database"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/handlers"
	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/internal/observability"
	"github.com/kriipke/platformctl/internal/readmodel"
	"github.com/kriipke/platformctl/internal/storage"
	"github.com/kriipke/platformctl/pkg/api"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize observability
	loggerConfig := observability.LoggerConfig{
		Level:         cfg.Observability.LogLevel,
		Format:        cfg.Observability.LogFormat,
		ServiceName:   "gateway",
		EnableConsole: cfg.Observability.EnableConsoleLog,
	}
	logger := observability.NewLogger(loggerConfig)

	metricsConfig := observability.MetricsConfig{
		Enabled:   cfg.Observability.MetricsEnabled,
		Port:      cfg.Observability.MetricsPort,
		Path:      cfg.Observability.MetricsPath,
		Namespace: cfg.Observability.MetricsNamespace,
	}
	metrics := observability.NewMetrics(metricsConfig)

	// Initialize database connection
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Also create storage.DB for other stores
	storageDB, err := storage.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to storage database: %v", err)
	}
	defer storageDB.Close()

	// Run migrations
	migrationsPath := getEnv("MIGRATIONS_PATH", "./migrations")
	if err := database.RunMigrations(cfg.DatabaseURL, migrationsPath); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize RabbitMQ connection
	rabbitmq, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmq.Close()

	// Initialize stores
	appStore := storage.NewAppStore(storageDB)
	environmentStore := storage.NewEnvironmentStore(storageDB)
	contextStore := storage.NewContextStore(storageDB)

	// Initialize GitOps read model store
	gitopsStore := readmodel.NewGitOpsStore(db)

	// Initialize GitOps components
	publisher := events.NewGitOpsCommandPublisher(rabbitmq)

	// Initialize handlers
	appHandler := handlers.NewAppHandler(appStore)
	environmentHandler := handlers.NewEnvironmentHandler(environmentStore)
	contextHandler := handlers.NewContextHandler(contextStore)
	actionHandler := handlers.NewGitOpsActionHandler(appStore, environmentStore, contextStore, publisher)
	// Create a simple zerolog logger for the status handler
	statusLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	statusHandler := handlers.NewGitOpsStatusHandler(gitopsStore, statusLogger)

	// Setup Gin router with observability middleware
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Apply observability middleware stack
	middlewareStack := observability.NewObservabilityMiddlewareStack(logger, metrics, "X-Correlation-ID")
	middlewareStack.ApplyToGin(router)

	// Setup API routes
	setupAPIRoutes(router, appHandler, environmentHandler, contextHandler, actionHandler, statusHandler)

	// Start the health/readiness server on the dedicated health port. Kubernetes
	// probes liveness (/health) and readiness (/ready) on this port (8081); the
	// gin API server below only listens on cfg.Port (8080), so without this the
	// probes get connection-refused and the kubelet kills the pod.
	healthManager := observability.NewHealthManager(cfg.GetHealthCheckConfig(), "gateway", "1.0.0")
	go func() {
		if err := healthManager.StartHealthServer(); err != nil && err != http.ErrServerClosed {
			logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Health server failed")
		}
	}()

	// Start main server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown handling
	go func() {
		log.Printf("Starting GitOps API Gateway on port %s (health: %s)", cfg.Port, cfg.Observability.HealthCheckPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Use the underlying zerolog logger for shutdown message
	logger.NewContextLogger(context.Background()).Info().Msg("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupAPIRoutes(router *gin.Engine, appHandler *handlers.AppHandler, environmentHandler *handlers.EnvironmentHandler, contextHandler *handlers.ContextHandler, actionHandler *handlers.GitOpsActionHandler, statusHandler *handlers.GitOpsStatusHandler) {
	// API routes with authentication
	apiGroup := router.Group("/api/v1")
	apiGroup.Use(ginBasicAuthMiddleware())
	// Populate the per-request customer/tenant context that the downstream
	// handlers read. Without this, every /api/v1 route returns 401 even after
	// basic auth succeeds. Must run after ginBasicAuthMiddleware so the
	// authenticated identity is available.
	apiGroup.Use(ginCustomerContextMiddleware())

	// App routes
	apiGroup.POST("/apps", ginHandlerWrapper(appHandler.CreateApp))
	apiGroup.GET("/apps", ginHandlerWrapper(appHandler.ListApps))
	apiGroup.GET("/apps/:name", ginHandlerWrapper(appHandler.GetApp))
	apiGroup.PUT("/apps/:name", ginHandlerWrapper(appHandler.UpdateApp))
	apiGroup.DELETE("/apps/:name", ginHandlerWrapper(appHandler.DeleteApp))

	// Environment routes
	apiGroup.POST("/environments", ginHandlerWrapper(environmentHandler.CreateEnvironment))
	apiGroup.GET("/environments", ginHandlerWrapper(environmentHandler.ListEnvironments))
	apiGroup.GET("/environments/:name", ginHandlerWrapper(environmentHandler.GetEnvironment))
	apiGroup.PUT("/environments/:name", ginHandlerWrapper(environmentHandler.UpdateEnvironment))
	apiGroup.DELETE("/environments/:name", ginHandlerWrapper(environmentHandler.DeleteEnvironment))

	// Context routes
	apiGroup.POST("/contexts", ginHandlerWrapper(contextHandler.CreateContext))
	apiGroup.GET("/contexts", ginHandlerWrapper(contextHandler.ListContexts))
	apiGroup.GET("/contexts/:name", ginHandlerWrapper(contextHandler.GetContext))
	apiGroup.PUT("/contexts/:name", ginHandlerWrapper(contextHandler.UpdateContext))
	apiGroup.DELETE("/contexts/:name", ginHandlerWrapper(contextHandler.DeleteContext))

	// GitOps Action routes (Phase 1B)
	apiGroup.POST("/contexts/:name/actions/sync-apps", ginHandlerWrapper(actionHandler.HandleSyncApps))
	apiGroup.POST("/contexts/:name/actions/validate-environments", ginHandlerWrapper(actionHandler.HandleValidateEnvironments))
	apiGroup.POST("/contexts/:name/actions/correlate-contexts", ginHandlerWrapper(actionHandler.HandleCorrelateContexts))
	apiGroup.POST("/contexts/:name/actions/correlate-multi-environment", ginHandlerWrapper(actionHandler.HandleCorrelateMultiEnvironment))
	apiGroup.POST("/contexts/:name/actions/inspect-manifests", ginHandlerWrapper(actionHandler.HandleInspectManifests))

	// GitOps Status routes (Phase 1D)
	gitopsGroup := apiGroup.Group("/gitops")

	// Context status routes
	gitopsGroup.GET("/contexts/status", statusHandler.ListContextStatuses)
	gitopsGroup.GET("/contexts/:contextName/status", statusHandler.GetContextStatus)
	gitopsGroup.GET("/contexts/:contextName/health", statusHandler.GetContextHealth)

	// App manifest status routes
	gitopsGroup.GET("/contexts/:contextName/apps/:appName/status", statusHandler.GetAppManifestStatus)
	gitopsGroup.GET("/contexts/:contextName/apps/:appName/environments/status", statusHandler.GetMultiEnvironmentAppStatus)

	// Environment manifest status routes
	gitopsGroup.GET("/contexts/:contextName/environments/:environmentName/status", statusHandler.GetEnvironmentManifestStatus)
	gitopsGroup.GET("/contexts/:contextName/environments/:environmentName/vault/status", statusHandler.GetVaultValidationDetails)

	// System health overview
	gitopsGroup.GET("/health/overview", statusHandler.GetSystemHealthOverview)

	// Health check (no auth required)
	router.GET("/health", ginHealthHandler)
}

// Gin-compatible health handler
func ginHealthHandler(c *gin.Context) {
	response := api.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Services: struct {
			Database bool `json:"database"`
			Storage  bool `json:"storage"`
		}{
			Database: true,
			Storage:  true,
		},
	}
	c.JSON(http.StatusOK, response)
}

// Gin-compatible basic auth middleware
func ginBasicAuthMiddleware() gin.HandlerFunc {
	return gin.BasicAuth(gin.Accounts{
		"admin": "admin", // TODO: Use proper authentication
	})
}

// customerNamespaceUUID is a fixed namespace used to derive a stable, deterministic
// customer UUID from a customer identifier string. It lets the string customer id
// used by the CRUD handlers (auth.Customer.CustomerID) and the uuid used by the
// status handlers (models.Customer.ID) resolve to the same value for a given tenant,
// so a tenant's writes and reads address the same customer_id.
var customerNamespaceUUID = uuid.MustParse("a3c9f1e2-7b4d-4e6a-9c1f-2d3e4f5a6b7c")

// deriveCustomerUUID maps a customer identifier to a stable UUID. If the identifier
// is already a UUID it is used verbatim; otherwise a deterministic v5 UUID is derived
// from it so the same identifier always yields the same customer id.
func deriveCustomerUUID(customerKey string) uuid.UUID {
	if id, err := uuid.Parse(customerKey); err == nil {
		return id
	}
	return uuid.NewSHA1(customerNamespaceUUID, []byte(customerKey))
}

// ginCustomerContextMiddleware derives the authenticated tenant and publishes it in
// BOTH shapes the downstream handlers expect:
//
//   - c.Set("customer", *models.Customer) for the native-gin status handlers
//     (internal/handlers/gitops_status.go), which read c.Get("customer").
//   - auth.CustomerContextKey in the request context for the wrapped
//     http.HandlerFunc handlers (context/app/environment CRUD and gitops_actions.go),
//     which read auth.GetCustomerFromContext / auth.RequireCustomer.
//
// The string customer id exposed to the CRUD path is exactly the string form of the
// uuid exposed to the status path, so a tenant's writes and reads line up. The tenant
// is taken from the basic-auth identity, optionally overridden by the X-Customer-ID /
// X-User-ID headers so API clients (e.g. the platformctl CLI) can select a tenant.
// Must run after ginBasicAuthMiddleware so the authenticated username is available.
func ginCustomerContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetString(gin.AuthUserKey)

		customerKey := c.GetHeader("X-Customer-ID")
		if customerKey == "" {
			customerKey = username
		}
		if hdrUser := c.GetHeader("X-User-ID"); hdrUser != "" {
			username = hdrUser
		}

		customerUUID := deriveCustomerUUID(customerKey)

		// For native-gin handlers that read c.Get("customer").
		c.Set("customer", &models.Customer{
			ID:       customerUUID,
			Username: username,
		})

		// For wrapped http.HandlerFunc handlers that read auth.GetCustomerFromContext.
		ctx := context.WithValue(c.Request.Context(), auth.CustomerContextKey, &auth.Customer{
			CustomerID: customerUUID.String(),
			Username:   username,
			Roles:      []string{"user"},
		})
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// Wrapper to convert http.HandlerFunc to gin.HandlerFunc
func ginHandlerWrapper(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return gin.WrapF(handler)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
