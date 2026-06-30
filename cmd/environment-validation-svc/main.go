package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/observability"
	"github.com/kriipke/platformctl/internal/storage"
)

func main() {
	cfg := config.Load()

	// Initialize observability
	loggerConfig := observability.LoggerConfig{
		Level:         cfg.Observability.LogLevel,
		Format:        cfg.Observability.LogFormat,
		ServiceName:   "environment-validation-service",
		EnableConsole: cfg.Observability.EnableConsoleLog,
	}
	logger := observability.NewLogger(loggerConfig)

	metricsConfig := observability.MetricsConfig{
		Enabled:     cfg.Observability.MetricsEnabled,
		Port:        cfg.Observability.MetricsPort,
		ServiceName: "environment-validation-service",
		Namespace:   "contextops",
	}
	metrics := observability.NewMetrics(metricsConfig)

	healthConfig := cfg.GetHealthCheckConfig()
	healthManager := observability.NewHealthManager(healthConfig, "environment-validation-service", "1.0.0")

	// Database connection
	db, err := storage.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	// healthManager.RegisterChecker(observability.NewDatabaseHealthChecker(db.DB, "database")) // TODO: Fix type compatibility

	// RabbitMQ connection
	rabbitmq, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()
	// TODO: Add GetConnection method to GitOpsMessageBus
	// healthManager.RegisterChecker(observability.NewRabbitMQHealthChecker(conn, "rabbitmq"))

	// Start observability servers
	go func() {
		if err := observability.StartMetricsServer(metrics, metricsConfig); err != nil {
			logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Failed to start metrics server")
		}
	}()
	go func() {
		if err := healthManager.StartHealthServer(); err != nil {
			logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Failed to start health server")
		}
	}()

	// Create the environment validation handler and consume environment commands
	envHandler := NewEnvironmentValidationHandler(cfg)

	consumer := events.NewCommandConsumerWithBindings(rabbitmq, "gitops.environment-validation.q", []string{"cmd.environment.*"})
	if err := consumer.Start(envHandler); err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to start command consumer")
	}

	logger.NewContextLogger(context.Background()).Info().Msg("Environment validation service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.NewContextLogger(context.Background()).Info().Msg("Shutting down environment validation service")
	if err := consumer.Stop(); err != nil {
		logger.NewContextLogger(context.Background()).Error().Err(err).Msg("Error stopping command consumer")
	}
	logger.NewContextLogger(context.Background()).Info().Msg("Environment validation service stopped")
}