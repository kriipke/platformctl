package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/contextops/platformctl/internal/config"
	"github.com/contextops/platformctl/internal/events"
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

	// Create the context pairing handler and consume context commands
	contextHandler := NewContextPairingHandler(cfg)

	consumer := events.NewCommandConsumerWithBindings(messageBus, "gitops.context-correlation.q", []string{"cmd.context.*"})
	if err := consumer.Start(contextHandler); err != nil {
		log.Fatalf("Failed to start command consumer: %v", err)
	}

	log.Println("Context correlation service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down context correlation service")
	if err := consumer.Stop(); err != nil {
		log.Printf("Error stopping command consumer: %v", err)
	}
}