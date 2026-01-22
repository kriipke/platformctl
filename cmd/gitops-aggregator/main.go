package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"platformctl/internal/aggregator"
	"platformctl/internal/config"
	"platformctl/internal/database"
	"platformctl/internal/events"
	"platformctl/internal/observability"
)

func main() {
	// Initialize configuration
	cfg := config.Load()
	
	// Initialize observability
	loggerConfig := observability.LoggerConfig{
		Level:         cfg.LogLevel,
		Format:        cfg.LogFormat,
		ServiceName:   "gitops-aggregator",
		EnableConsole: cfg.EnableConsoleLog,
	}
	logger := observability.NewLogger(loggerConfig)

	metricsConfig := observability.MetricsConfig{
		Enabled:     cfg.MetricsEnabled,
		Port:        cfg.MetricsPort,
		ServiceName: "gitops-aggregator",
		Namespace:   "contextops",
	}
	metrics := observability.NewMetrics(metricsConfig)

	healthConfig := observability.HealthCheckConfig{
		Port:              cfg.HealthCheckPort,
		CheckTimeout:      5 * time.Second,
		EnableDeepChecks:  true,
	}
	healthManager := observability.NewHealthManager(healthConfig, "gitops-aggregator", "1.0.0")
	
	// Initialize database connection
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	healthManager.RegisterChecker(observability.NewDatabaseHealthChecker(db, "database"))

	// Initialize RabbitMQ connection for result consumption
	rabbitmq, err := events.NewGitOpsRabbitMQ(cfg.RabbitMQURL, "gitops-aggregator")
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()
	if conn := rabbitmq.GetConnection(); conn != nil {
		healthManager.RegisterChecker(observability.NewRabbitMQHealthChecker(conn, "rabbitmq"))
	}

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

	// Initialize the GitOps aggregator service
	aggregatorService := aggregator.NewGitOpsAggregator(db, *logger, metrics)

	// Initialize result consumer for all GitOps services
	consumer := events.NewGitOpsResultConsumer(rabbitmq, aggregatorService, *logger, metrics)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start the result consumer
	go func() {
		logger.NewContextLogger(context.Background()).Info().Msg("Starting GitOps Aggregator Service")
		if err := consumer.StartConsuming(ctx); err != nil {
			logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to start result consumer")
		}
	}()

	// Wait for shutdown signal
	<-sigCh
	logger.NewContextLogger(context.Background()).Info().Msg("Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel() // Stop the consumer

	logger.NewContextLogger(context.Background()).Info().Msg("GitOps Aggregator Service stopped")
}