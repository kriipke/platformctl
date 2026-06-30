package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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

	healthConfig := cfg.GetHealthCheckConfig()
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

	// Create app sync handler and consume app manifest commands
	appSyncHandler := NewAppSyncHandler(cfg)

	consumer := events.NewCommandConsumerWithBindings(rabbitmq, "gitops.app-sync.q", []string{"cmd.app.*"})
	if err := consumer.Start(appSyncHandler); err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to start command consumer")
	}

	logger.NewContextLogger(context.Background()).Info().Msg("App sync service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.NewContextLogger(context.Background()).Info().Msg("Shutting down app sync service")
	if err := consumer.Stop(); err != nil {
		logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Error stopping command consumer")
	}
	logger.NewContextLogger(context.Background()).Info().Msg("App sync service stopped")
}