package main

import (
	"log"
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

	// Database connection
	db, err := storage.NewDB(cfg.GetDatabaseConfig())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// RabbitMQ connection
	messageBus, err := events.NewGitOpsMessageBus(cfg.RabbitMQURL, cfg)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
	}
	defer messageBus.Close()

	// Service setup
	appStore := storage.NewAppStore(db)
	environmentStore := storage.NewEnvironmentStore(db)
	contextStore := storage.NewContextStore(db)
	gitHandler := NewCustomerGitBranchHandler(cfg)
	// TODO: Implement proper service framework
	_ = gitHandler
	_ = messageBus
	_ = appStore 
	_ = environmentStore
	_ = contextStore

	// Initialize health manager
	healthConfig := cfg.GetHealthCheckConfig()
	healthManager := observability.NewHealthManager(healthConfig, "customer-git-branch-service", "1.0.0")

	// Start health server
	go func() {
		if err := healthManager.StartHealthServer(); err != nil {
			log.Printf("Failed to start health server: %v", err)
		}
	}()

	log.Println("Customer git branch service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down customer git branch service")
	// TODO: Implement proper service shutdown
}