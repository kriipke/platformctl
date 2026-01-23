package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/contextops/platformctl/internal/auth"
	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/internal/database"
	"github.com/contextops/platformctl/internal/events"
	"github.com/contextops/platformctl/internal/handlers"
	"github.com/contextops/platformctl/internal/observability"
	"github.com/contextops/platformctl/internal/readmodel"
	"github.com/contextops/platformctl/internal/storage"
	"github.com/contextops/platformctl/pkg/api"
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

	// Initialize health manager
	healthConfig := cfg.GetHealthCheckConfig()
	healthManager := observability.NewHealthManager(healthConfig, "gateway", "1.0.0")

	// Start health server
	go func() {
		if err := healthManager.StartHealthServer(); err != nil {
			log.Printf("Failed to start health server: %v", err)
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

// Gin-compatible basic auth middleware that sets customer context
func ginBasicAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract basic auth header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authorization header required",
				"code":    401,
			})
			c.Abort()
			return
		}

		// Parse basic auth
		const basicScheme = "Basic "
		if !strings.HasPrefix(authHeader, basicScheme) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid authorization scheme",
				"code":    401,
			})
			c.Abort()
			return
		}

		encodedCredentials := authHeader[len(basicScheme):]
		decodedCredentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid authorization header",
				"code":    401,
			})
			c.Abort()
			return
		}

		credentials := strings.SplitN(string(decodedCredentials), ":", 2)
		if len(credentials) != 2 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid credentials format",
				"code":    401,
			})
			c.Abort()
			return
		}

		username, password := credentials[0], credentials[1]

		// Validate credentials (simple validation for admin:admin)
		if username != "admin" || password != "admin" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid credentials",
				"code":    401,
			})
			c.Abort()
			return
		}

		// Set customer in Gin context (use acme-corp for demo data access)
		c.Set("customer_id", "acme-corp")
		c.Set("username", username)

		// Continue to next handler
		c.Next()
	}
}

// Wrapper to convert http.HandlerFunc to gin.HandlerFunc with proper auth context
func ginHandlerWrapper(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get customer info from Gin context (set by auth middleware)
		customerID, exists := c.Get("customer_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "No customer context",
				"code":    401,
			})
			return
		}

		username, _ := c.Get("username")

		// Create customer object for the handlers
		customer := &auth.Customer{
			CustomerID: customerID.(string),
			Username:   username.(string),
			Roles:      []string{"user"},
		}

		// Add customer to request context for the handlers
		ctx := context.WithValue(c.Request.Context(), auth.CustomerContextKey, customer)
		r := c.Request.WithContext(ctx)

		// Call the original handler with the modified request
		handler(c.Writer, r)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}