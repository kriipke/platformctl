package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/internal/database"
	"github.com/contextops/platformctl/internal/events"
	"github.com/contextops/platformctl/internal/observability"
	"github.com/contextops/platformctl/internal/services"
	"github.com/contextops/platformctl/internal/storage"
)

func main() {
	cfg := config.Load()

	// Initialize observability
	loggerConfig := observability.LoggerConfig{
		Level:         cfg.LogLevel,
		Format:        cfg.LogFormat,
		ServiceName:   "app-sync-service",
		EnableConsole: cfg.EnableConsoleLog,
	}
	logger := observability.NewLogger(loggerConfig)

	metricsConfig := observability.MetricsConfig{
		Enabled:     cfg.MetricsEnabled,
		Port:        cfg.MetricsPort,
		ServiceName: "app-sync-service",
		Namespace:   "contextops",
	}
	metrics := observability.NewMetrics(metricsConfig)

	healthConfig := observability.HealthCheckConfig{
		Port:              cfg.HealthCheckPort,
		CheckTimeout:      5 * time.Second,
		EnableDeepChecks:  true,
	}
	healthManager := observability.NewHealthManager(healthConfig, "app-sync-service", "1.0.0")

	// Database connection
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Register database health check
	healthManager.RegisterChecker(observability.NewDatabaseHealthChecker(db, "database"))

	// RabbitMQ connection
	rabbitmq, err := events.NewGitOpsRabbitMQ(cfg.RabbitMQURL, "app-sync-service")
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()

	// Register RabbitMQ health check
	if conn := rabbitmq.GetConnection(); conn != nil {
		healthManager.RegisterChecker(observability.NewRabbitMQHealthChecker(conn, "rabbitmq"))
	}

	// Start metrics server
	go func() {
		if err := observability.StartMetricsServer(metrics, metricsConfig); err != nil {
			logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Failed to start metrics server")
		}
	}()

	// Start health server
	go func() {
		if err := healthManager.StartHealthServer(); err != nil {
			logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Failed to start health server")
		}
	}()

	// Service setup
	appStore := storage.NewAppStore(db)
	environmentStore := storage.NewEnvironmentStore(db)
	contextStore := storage.NewContextStore(db)

	// Create app sync service with observability
	appSyncService := services.NewAppSyncService(
		db,
		rabbitmq,
		appStore,
		environmentStore,
		contextStore,
		logger,
		metrics,
	)

	// Start service
	ctx := context.Background()
	if err := appSyncService.Start(ctx); err != nil {
		logger.NewContextLogger(ctx).Fatal().Err(err).Msg("Failed to start app sync service")
	}

	logger.NewContextLogger(ctx).Info().Msg("App sync service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.NewContextLogger(context.Background()).Info().Msg("Shutting down app sync service")
	
	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := appSyncService.Stop(shutdownCtx); err != nil {
		logger.NewContextLogger(shutdownCtx).Error().Err(err).Msg("Error during service shutdown")
	}
	
	logger.NewContextLogger(context.Background()).Info().Msg("App sync service stopped")
}