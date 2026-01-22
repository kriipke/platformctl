package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/internal/events"
	"github.com/contextops/platformctl/internal/services"
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
	contextHandler := NewContextPairingHandler(cfg)
	service := services.NewManifestBaseService("context-correlation", messageBus, appStore, environmentStore, contextStore)

	// Start service
	if err := service.Start(contextHandler); err != nil {
		log.Fatal("Failed to start context correlation service:", err)
	}

	log.Println("Context correlation service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down context correlation service")
	service.Stop()
}