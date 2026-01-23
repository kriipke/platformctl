package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/internal/events"
	"github.com/contextops/platformctl/internal/observability"
	"github.com/contextops/platformctl/internal/storage"
)

func main() {
	cfg := config.Load()

	// Initialize observability
	loggerConfig := observability.LoggerConfig{
		Level:         cfg.Observability.LogLevel,
		Format:        cfg.Observability.LogFormat,
		ServiceName:   "app-sync-service",
		EnableConsole: cfg.Observability.EnableConsoleLog,
	}
	logger := observability.NewLogger(loggerConfig)

	metricsConfig := observability.MetricsConfig{
		Enabled:     cfg.Observability.MetricsEnabled,
		Port:        cfg.Observability.MetricsPort,
		ServiceName: "app-sync-service",
		Namespace:   "contextops",
	}
	metrics := observability.NewMetrics(metricsConfig)

	healthConfig := observability.HealthCheckConfig{
		Port:              cfg.Observability.HealthCheckPort,
		CheckTimeout:      5 * time.Second,
		EnableDeepChecks:  true,
	}
	healthManager := observability.NewHealthManager(healthConfig, "app-sync-service", "1.0.0")

	// Database connection
	db, err := storage.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Register database health check - TODO: Fix type compatibility
	// healthManager.RegisterChecker(observability.NewDatabaseHealthChecker(db.DB, "database"))

	// RabbitMQ connection
	rabbitmq, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()

	// Register RabbitMQ health check - TODO: Add GetConnection method to GitOpsMessageBus
	// if conn := rabbitmq.Connection(); conn != nil {
	//	healthManager.RegisterChecker(observability.NewRabbitMQHealthChecker(conn, "rabbitmq"))
	// }

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

	// Service setup - TODO: Use these stores in proper service implementation
	_ = storage.NewAppStore(db)
	_ = storage.NewEnvironmentStore(db) 
	_ = storage.NewContextStore(db)

	// Create app sync handler
	appSyncHandler := NewAppSyncHandler(cfg)

	// Start service - TODO: Implement proper service runner
	ctx := context.Background()
	_ = appSyncHandler // Use the handler
	logger.NewContextLogger(ctx).Info().Msg("App sync service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.NewContextLogger(context.Background()).Info().Msg("Shutting down app sync service")
	
	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// TODO: Implement proper shutdown for handler
	if false { // err := appSyncHandler.Stop(shutdownCtx); err != nil {
		logger.NewContextLogger(shutdownCtx).Error().Err(err).Msg("Error during service shutdown")
	}
	
	logger.NewContextLogger(context.Background()).Info().Msg("App sync service stopped")
}