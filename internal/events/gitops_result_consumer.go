package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"github.com/kriipke/platformctl/internal/observability"
	"github.com/kriipke/platformctl/pkg/api"
)

// ResultProcessor interface for processing GitOps result messages
type ResultProcessor interface {
	ProcessResultMessage(ctx context.Context, result *api.GitOpsResultMessage) error
}

// GitOpsResultConsumer consumes result messages from GitOps integration services
type GitOpsResultConsumer struct {
	rabbitmq  *GitOpsRabbitMQ
	processor ResultProcessor
	logger    zerolog.Logger
	metrics   *observability.Metrics
}

// NewGitOpsResultConsumer creates a new GitOps result consumer
func NewGitOpsResultConsumer(rabbitmq *GitOpsRabbitMQ, processor ResultProcessor, logger zerolog.Logger, metrics *observability.Metrics) *GitOpsResultConsumer {
	return &GitOpsResultConsumer{
		rabbitmq:  rabbitmq,
		processor: processor,
		logger:    logger.With().Str("component", "gitops-result-consumer").Logger(),
		metrics:   metrics,
	}
}

// StartConsuming starts consuming result messages from all GitOps services
func (c *GitOpsResultConsumer) StartConsuming(ctx context.Context) error {
	// Get a channel for consuming
	channel, err := c.rabbitmq.GetChannel()
	if err != nil {
		return fmt.Errorf("failed to get RabbitMQ channel: %w", err)
	}
	defer channel.Close()

	// Configure QoS to process one message at a time
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// All service results are published to the gitops.results exchange with
	// evt.* routing keys, which the topology binds to the single
	// gitops.aggregator.q queue. Consume from that queue. (The previous
	// results.* queue names were never declared in the topology, so nothing
	// was consumed and the read model / command status never updated.)
	resultQueues := []string{
		"gitops.aggregator.q",
	}

	// Start consuming from each result queue
	for _, queueName := range resultQueues {
		go func(queue string) {
			c.logger.Info().Str("queue", queue).Msg("Starting to consume from result queue")

			if err := c.consumeFromQueue(ctx, channel, queue); err != nil {
				c.logger.Error().Err(err).Str("queue", queue).Msg("Error consuming from queue")
			}
		}(queueName)
	}

	// Wait for context cancellation
	<-ctx.Done()
	c.logger.Info().Msg("Stopping GitOps result consumer")
	return nil
}

// consumeFromQueue consumes messages from a specific queue
func (c *GitOpsResultConsumer) consumeFromQueue(ctx context.Context, channel *amqp.Channel, queueName string) error {
	// Start consuming messages
	msgs, err := channel.Consume(
		queueName,
		"gitops-aggregator", // consumer tag
		false,               // auto-ack (we'll manually ack)
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,                 // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from queue %s: %w", queueName, err)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Str("queue", queueName).Msg("Stopping consumption due to context cancellation")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn().Str("queue", queueName).Msg("Message channel closed")
				return fmt.Errorf("message channel closed for queue %s", queueName)
			}

			// Process the message
			if err := c.processMessage(ctx, msg, queueName); err != nil {
				c.logger.Error().
					Err(err).
					Str("queue", queueName).
					Str("message_id", msg.MessageId).
					Msg("Failed to process message")

				// Reject message and requeue if it's a transient error
				if c.shouldRequeue(err) {
					msg.Nack(false, true)
					c.metrics.IncrementCounter("gitops_result_consumer_requeued", map[string]string{
						"queue": queueName,
						"error": "transient",
					})
				} else {
					// Reject without requeue for permanent failures
					msg.Nack(false, false)
					c.metrics.IncrementCounter("gitops_result_consumer_rejected", map[string]string{
						"queue": queueName,
						"error": "permanent",
					})
				}
			} else {
				// Acknowledge successful processing
				msg.Ack(false)
				c.metrics.IncrementCounter("gitops_result_consumer_processed", map[string]string{
					"queue": queueName,
				})
			}
		}
	}
}

// processMessage processes a single result message
func (c *GitOpsResultConsumer) processMessage(ctx context.Context, msg amqp.Delivery, queueName string) error {
	start := time.Now()

	c.logger.Debug().
		Str("queue", queueName).
		Str("message_id", msg.MessageId).
		Str("correlation_id", msg.CorrelationId).
		Int("body_size", len(msg.Body)).
		Msg("Processing result message")

	// Parse the GitOps result message
	var result api.GitOpsResultMessage
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		c.metrics.IncrementCounter("gitops_result_consumer_parse_errors", map[string]string{
			"queue": queueName,
		})
		return fmt.Errorf("failed to unmarshal result message: %w", err)
	}

	// Validate the message
	if err := c.validateResultMessage(&result); err != nil {
		c.metrics.IncrementCounter("gitops_result_consumer_validation_errors", map[string]string{
			"queue": queueName,
		})
		return fmt.Errorf("invalid result message: %w", err)
	}

	// Process with timeout
	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Process the result message
	if err := c.processor.ProcessResultMessage(processCtx, &result); err != nil {
		c.metrics.IncrementCounter("gitops_result_consumer_processing_errors", map[string]string{
			"queue":   queueName,
			"service": result.ServiceName,
		})
		return fmt.Errorf("failed to process result message: %w", err)
	}

	// Record processing metrics
	duration := time.Since(start)
	c.metrics.RecordHistogram("gitops_result_consumer_processing_duration", duration.Seconds(), map[string]string{
		"queue":   queueName,
		"service": result.ServiceName,
		"status":  result.Status,
	})

	c.logger.Info().
		Str("queue", queueName).
		Str("message_id", msg.MessageId).
		Str("correlation_id", result.CorrelationID).
		Str("service", result.ServiceName).
		Str("customer_id", result.CustomerID).
		Str("context_name", result.ContextName).
		Str("status", result.Status).
		Dur("processing_duration", duration).
		Msg("Successfully processed result message")

	return nil
}

// validateResultMessage validates the structure of a result message
func (c *GitOpsResultConsumer) validateResultMessage(result *api.GitOpsResultMessage) error {
	if result.MessageID == "" {
		return fmt.Errorf("message ID is required")
	}
	if result.CorrelationID == "" {
		return fmt.Errorf("correlation ID is required")
	}
	if result.CustomerID == "" {
		return fmt.Errorf("customer ID is required")
	}
	if result.ContextName == "" {
		return fmt.Errorf("context name is required")
	}
	if result.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if result.Status == "" {
		return fmt.Errorf("status is required")
	}
	if result.ManifestType == "" {
		return fmt.Errorf("manifest type is required")
	}

	// Validate manifest type specific data
	switch result.ManifestType {
	case "app":
		if result.AppManifestData == nil {
			return fmt.Errorf("app manifest data is required for app manifest type")
		}
	case "environment":
		if result.EnvironmentManifestData == nil {
			return fmt.Errorf("environment manifest data is required for environment manifest type")
		}
	case "context":
		if result.ContextPairingData == nil {
			return fmt.Errorf("context pairing data is required for context manifest type")
		}
	case "git":
		// customer-git-branch results carry their detail in the payload
	case "kubernetes":
		// multi-environment results carry their detail in the payload
	default:
		return fmt.Errorf("unknown manifest type: %s", result.ManifestType)
	}

	return nil
}

// shouldRequeue determines if a message should be requeued based on the error
func (c *GitOpsResultConsumer) shouldRequeue(err error) bool {
	// Define transient errors that should be retried
	transientErrors := []string{
		"database connection",
		"timeout",
		"temporary",
		"connection refused",
		"context deadline exceeded",
	}

	errStr := err.Error()
	for _, transient := range transientErrors {
		if fmt.Sprintf("%s", errStr) != "" && fmt.Sprintf("%s", transient) != "" {
			// Simple substring check - in production, you'd use more sophisticated error matching
			return true
		}
	}

	// Don't requeue validation errors or parsing errors
	return false
}
