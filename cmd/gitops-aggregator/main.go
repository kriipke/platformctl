package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/kriipke/platformctl/internal/aggregator"
	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/observability"
	"github.com/kriipke/platformctl/internal/storage"
	_ "github.com/lib/pq"
)

func main() {
	// Initialize configuration
	cfg := config.Load()

	// Initialize observability
	loggerConfig := observability.LoggerConfig{
		Level:         cfg.Observability.LogLevel,
		Format:        cfg.Observability.LogFormat,
		ServiceName:   "gitops-aggregator",
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

	healthConfig := cfg.GetHealthCheckConfig()
	healthManager := observability.NewHealthManager(healthConfig, "gitops-aggregator", "1.0.0")

	// Initialize database connection
	dbConn, err := storage.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbConn.Close()

	// Convert to sqlx for aggregator compatibility
	db := sqlx.NewDb(dbConn.DB, "postgres")

	// Initialize RabbitMQ connection for result consumption
	rabbitmq, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()

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
	zerologger := logger.NewContextLogger(context.Background())
	aggregatorService := aggregator.NewGitOpsAggregator(db, *zerologger, metrics)

	// Initialize result consumer for all GitOps services
	consumer := events.NewGitOpsResultConsumer(rabbitmq, aggregatorService, *zerologger, metrics)

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

	// Use shutdown context for any cleanup operations
	select {
	case <-shutdownCtx.Done():
		logger.NewContextLogger(context.Background()).Warn().Msg("Shutdown timeout exceeded")
	case <-time.After(1 * time.Second):
		// Allow brief time for cleanup
	}

	logger.NewContextLogger(context.Background()).Info().Msg("GitOps Aggregator Service stopped")
}
