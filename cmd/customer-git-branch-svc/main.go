package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kriipke/platformctl/internal/config"
	"github.com/kriipke/platformctl/internal/events"
	"github.com/kriipke/platformctl/internal/storage"
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

	// Create the customer git branch handler and consume git/customer-branch commands
	gitHandler := NewCustomerGitBranchHandler(cfg)

	consumer := events.NewCommandConsumerWithBindings(messageBus, "gitops.customer-git-branch.q", []string{"cmd.git.*", "cmd.manifest.*"})
	if err := consumer.Start(gitHandler); err != nil {
		log.Fatalf("Failed to start command consumer: %v", err)
	}

	log.Println("Customer git branch service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down customer git branch service")
	if err := consumer.Stop(); err != nil {
		log.Printf("Error stopping command consumer: %v", err)
	}
}