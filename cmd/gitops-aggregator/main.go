package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/contextops/platformctl/internal/aggregator"
	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/internal/database"
	"github.com/contextops/platformctl/internal/events"
	"github.com/contextops/platformctl/internal/observability"
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

	healthConfig := observability.HealthCheckConfig{
		Port:              cfg.Observability.HealthCheckPort,
		CheckTimeout:      5 * time.Second,
		EnableDeepChecks:  true,
	}
	healthManager := observability.NewHealthManager(healthConfig, "gitops-aggregator", "1.0.0")
	
	// Initialize database connection
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	
	// Run migrations
	if err := database.RunMigrations(cfg.DatabaseURL, "./migrations"); err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to run database migrations")
	}

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