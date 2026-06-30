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

	// Create the multi-environment kubernetes handler and consume its commands.
	// NOTE: no publisher currently emits cmd.kubernetes.* — this runner is wired
	// and ready, but the gateway/publisher must target this routing key (or this
	// binding be changed) for it to receive work. A distinct key is used so it
	// does not duplicate the context-correlation service's cmd.context.* stream.
	kubernetesHandler := NewMultiEnvironmentKubernetesHandler(cfg)

	consumer := events.NewCommandConsumerWithBindings(messageBus, "gitops.multi-environment-kubernetes.q", []string{"cmd.kubernetes.*"})
	if err := consumer.Start(kubernetesHandler); err != nil {
		log.Fatalf("Failed to start command consumer: %v", err)
	}

	log.Println("Multi-environment kubernetes service started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down multi-environment kubernetes service")
	if err := consumer.Stop(); err != nil {
		log.Printf("Error stopping command consumer: %v", err)
	}
}