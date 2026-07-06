package events

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/kriipke/platformctl/pkg/api"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ResultConsumer struct {
	messageBus *GitOpsMessageBus
	handlers   map[string]ResultHandler
	stopChan   chan struct{}
}

type ResultHandler interface {
	HandleResult(result *api.GitOpsResultMessage) error
}

func NewResultConsumer(mb *GitOpsMessageBus) *ResultConsumer {
	return &ResultConsumer{
		messageBus: mb,
		handlers:   make(map[string]ResultHandler),
		stopChan:   make(chan struct{}),
	}
}

func (c *ResultConsumer) RegisterHandler(serviceName string, handler ResultHandler) {
	c.handlers[serviceName] = handler
}

func (c *ResultConsumer) Start() error {
	msgs, err := c.messageBus.channel.Consume(
		"gitops.aggregator.q", // queue
		"",                    // consumer
		false,                 // auto-ack (we'll manually ack)
		false,                 // exclusive
		false,                 // no-local
		false,                 // no-wait
		nil,                   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		for {
			select {
			case msg := <-msgs:
				if err := c.processMessage(msg); err != nil {
					log.Printf("Error processing message: %v", err)
					_ = msg.Nack(false, true) // Requeue on error
				} else {
					_ = msg.Ack(false)
				}
			case <-c.stopChan:
				return
			}
		}
	}()

	return nil
}

func (c *ResultConsumer) processMessage(msg amqp.Delivery) error {
	var result api.GitOpsResultMessage
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}

	handler, exists := c.handlers[result.ServiceName]
	if !exists {
		log.Printf("No handler registered for service: %s", result.ServiceName)
		return nil // Not an error, just no handler
	}

	return handler.HandleResult(&result)
}

func (c *ResultConsumer) Stop() {
	close(c.stopChan)
}

// Basic result handler implementation for logging
type LoggingResultHandler struct {
	logger *log.Logger
}

func NewLoggingResultHandler(logger *log.Logger) *LoggingResultHandler {
	return &LoggingResultHandler{logger: logger}
}

func (h *LoggingResultHandler) HandleResult(result *api.GitOpsResultMessage) error {
	h.logger.Printf("Received result from service %s: status=%s, correlation=%s, customer=%s",
		result.ServiceName, result.Status, result.CorrelationID, result.CustomerID)

	if result.ErrorMessage != "" {
		h.logger.Printf("Service error: %s", result.ErrorMessage)
	}

	return nil
}

// Database result handler for persisting results
type DatabaseResultHandler struct {
	// TODO: Add database storage interface when needed in Phase 1D
	logger *log.Logger
}

func NewDatabaseResultHandler(logger *log.Logger) *DatabaseResultHandler {
	return &DatabaseResultHandler{logger: logger}
}

func (h *DatabaseResultHandler) HandleResult(result *api.GitOpsResultMessage) error {
	// TODO: Persist result to database
	h.logger.Printf("Would persist result to database: %s", result.CorrelationID)
	return nil
}
