package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"platformctl/internal/auth"
	"platformctl/internal/config"
	"platformctl/internal/handlers"
	"platformctl/internal/storage"
	"platformctl/internal/validation"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()

	db, err := storage.OpenPostgres(ctx, cfg.GetDatabaseConfig())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	store := storage.NewPostgresStore(db)
	validator := validation.NewValidator()

	contextHandler := handlers.NewGitOpsContextHandler(store, validator)
	applicationSetHandler := handlers.NewApplicationSetHandler()
	environmentHandler := handlers.NewEnvironmentHandler()
	vaultHandler := handlers.NewVaultValidationHandler()

	router := handlers.SetupGitOpsRouter(contextHandler, applicationSetHandler, environmentHandler, vaultHandler, auth.CustomerAuthMiddleware())

	server := &http.Server{
		Addr:         cfg.Port,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownCh
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("Starting GitOps API Gateway on %s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
