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
	logger := observability.NewLogger(cfg.Service.Name, cfg.Log.Level)
	metrics := observability.NewMetrics(cfg.Service.Name, cfg.Metrics.Enabled)

	// Initialize database connection
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	migrationsPath := getEnv("MIGRATIONS_PATH", "./migrations")
	if err := database.RunMigrations(cfg.DatabaseURL, migrationsPath); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Initialize RabbitMQ connection
	rabbitmq, err := events.NewGitOpsRabbitMQ(cfg.RabbitMQURL, "api-gateway")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()

	// Initialize stores
	appStore := storage.NewAppStore(db)
	environmentStore := storage.NewEnvironmentStore(db)
	contextStore := storage.NewContextStore(db)

	// Initialize GitOps read model store
	gitopsStore := readmodel.NewGitOpsStore(db)

	// Initialize GitOps components
	publisher := events.NewGitOpsPublisher(rabbitmq)

	// Initialize handlers
	appHandler := handlers.NewAppHandler(appStore)
	environmentHandler := handlers.NewEnvironmentHandler(environmentStore)
	contextHandler := handlers.NewContextHandler(contextStore)
	actionHandler := handlers.NewGitOpsActionHandler(appStore, environmentStore, contextStore, publisher)
	statusHandler := handlers.NewGitOpsStatusHandler(gitopsStore, *logger)

	// Setup Gin router with observability middleware
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	
	// Apply observability middleware stack
	middlewareStack := observability.NewObservabilityMiddlewareStack(logger, metrics, "X-Correlation-ID")
	middlewareStack.ApplyToGin(router)

	// Setup API routes
	setupAPIRoutes(router, appHandler, environmentHandler, contextHandler, actionHandler, statusHandler)

	// Initialize health checker
	healthChecker := observability.NewHealthChecker(logger)
	healthChecker.AddDatabaseCheck("postgres", db)
	healthChecker.AddRabbitMQCheck("rabbitmq", rabbitmq.Connection())

	// Start observability server for health and metrics
	obsServer := observability.StartObservabilityServer(cfg.Health.Port, logger, metrics, healthChecker)
	defer obsServer.Shutdown(context.Background())

	// Start main server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown handling
	go func() {
		logger.Info().
			Str("port", cfg.Port).
			Str("health_port", cfg.Health.Port).
			Msg("Starting GitOps API Gateway")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server exited")
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
	gitopsGroup.GET("/contexts/status", ginHandlerWrapper(statusHandler.ListContextStatuses))
	gitopsGroup.GET("/contexts/:contextName/status", ginHandlerWrapper(statusHandler.GetContextStatus))
	gitopsGroup.GET("/contexts/:contextName/health", ginHandlerWrapper(statusHandler.GetContextHealth))
	
	// App manifest status routes
	gitopsGroup.GET("/contexts/:contextName/apps/:appName/status", ginHandlerWrapper(statusHandler.GetAppManifestStatus))
	gitopsGroup.GET("/contexts/:contextName/apps/:appName/environments/status", ginHandlerWrapper(statusHandler.GetMultiEnvironmentAppStatus))
	
	// Environment manifest status routes
	gitopsGroup.GET("/contexts/:contextName/environments/:environmentName/status", ginHandlerWrapper(statusHandler.GetEnvironmentManifestStatus))
	gitopsGroup.GET("/contexts/:contextName/environments/:environmentName/vault/status", ginHandlerWrapper(statusHandler.GetVaultValidationDetails))
	
	// System health overview
	gitopsGroup.GET("/health/overview", ginHandlerWrapper(statusHandler.GetSystemHealthOverview))

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
			RabbitMQ bool `json:"rabbitmq,omitempty"`
		}{
			Database: true,
			Storage:  true,
			RabbitMQ: true,
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