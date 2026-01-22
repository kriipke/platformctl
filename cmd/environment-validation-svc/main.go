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
		ServiceName:   "environment-validation-service",
		EnableConsole: cfg.EnableConsoleLog,
	}
	logger := observability.NewLogger(loggerConfig)

	metricsConfig := observability.MetricsConfig{
		Enabled:     cfg.MetricsEnabled,
		Port:        cfg.MetricsPort,
		ServiceName: "environment-validation-service",
		Namespace:   "contextops",
	}
	metrics := observability.NewMetrics(metricsConfig)

	healthConfig := observability.HealthCheckConfig{
		Port:              cfg.HealthCheckPort,
		CheckTimeout:      5 * time.Second,
		EnableDeepChecks:  true,
	}
	healthManager := observability.NewHealthManager(healthConfig, "environment-validation-service", "1.0.0")

	// Database connection
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.NewContextLogger(context.Background()).Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	healthManager.RegisterChecker(observability.NewDatabaseHealthChecker(db, "database"))

	// RabbitMQ connection
	rabbitmq, err := events.NewGitOpsRabbitMQ(cfg.RabbitMQURL, "environment-validation-service")
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

	// Service setup
	appStore := storage.NewAppStore(db)
	environmentStore := storage.NewEnvironmentStore(db)
	contextStore := storage.NewContextStore(db)

	// Create environment validation service with observability
	environmentValidationService := services.NewEnvironmentValidationService(
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
	if err := environmentValidationService.Start(ctx); err != nil {
		logger.NewContextLogger(ctx).Fatal().Err(err).Msg("Failed to start environment validation service")
	}

	logger.NewContextLogger(ctx).Info().Msg("Environment validation service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.NewContextLogger(context.Background()).Info().Msg("Shutting down environment validation service")
	
	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := environmentValidationService.Stop(shutdownCtx); err != nil {
		logger.NewContextLogger(shutdownCtx).Error().Err(err).Msg("Error during service shutdown")
	}
	
	logger.NewContextLogger(context.Background()).Info().Msg("Environment validation service stopped")
}