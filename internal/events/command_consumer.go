package events

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/kriipke/platformctl/pkg/api"
	amqp "github.com/rabbitmq/amqp091-go"
)

type CommandConsumer struct {
	messageBus *GitOpsMessageBus
	queueName  string
	bindings   []string
	stopChan   chan struct{}
}

type CommandHandler interface {
	HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error)
}

// NewCommandConsumer creates a consumer with the default command bindings.
func NewCommandConsumer(mb *GitOpsMessageBus, queueName string) *CommandConsumer {
	return NewCommandConsumerWithBindings(mb, queueName, nil)
}

// NewCommandConsumerWithBindings creates a consumer that binds its queue only to
// the supplied routing keys on the gitops.commands exchange. Distinct bindings
// per service are required because the commands exchange is a topic exchange:
// overlapping bindings would deliver a copy of each command to every matching
// service queue and cause duplicate processing.
func NewCommandConsumerWithBindings(mb *GitOpsMessageBus, queueName string, bindings []string) *CommandConsumer {
	return &CommandConsumer{
		messageBus: mb,
		queueName:  queueName,
		bindings:   bindings,
		stopChan:   make(chan struct{}),
	}
}

func (c *CommandConsumer) Start(handler CommandHandler) error {
	// Declare service-specific queue
	_, err := c.messageBus.channel.QueueDeclare(
		c.queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind to command patterns (service-specific bindings)
	if err := c.bindToCommands(); err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Start consuming
	msgs, err := c.messageBus.channel.Consume(
		c.queueName,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		for {
			select {
			case msg := <-msgs:
				if err := c.processMessage(msg, handler); err != nil {
					log.Printf("Error processing message: %v", err)
					msg.Nack(false, true) // Requeue
				} else {
					msg.Ack(false)
				}
			case <-c.stopChan:
				return
			}
		}
	}()

	return nil
}

func (c *CommandConsumer) StartWithCustomerIsolation(handler interface{}) error {
	// For now, convert to CommandHandler interface
	if cmdHandler, ok := handler.(CommandHandler); ok {
		return c.Start(cmdHandler)
	}
	return fmt.Errorf("handler does not implement CommandHandler interface")
}

func (c *CommandConsumer) Stop() error {
	close(c.stopChan)
	return nil
}

func (c *CommandConsumer) bindToCommands() error {
	// Use the consumer's configured bindings; fall back to a broad default only
	// when none were supplied.
	bindings := c.bindings
	if len(bindings) == 0 {
		bindings = []string{
			"cmd.app.*",
			"cmd.environment.*",
			"cmd.context.*",
		}
	}

	for _, routingKey := range bindings {
		if err := c.messageBus.channel.QueueBind(
			c.queueName,
			routingKey,
			"gitops.commands",
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind queue %s to %s: %w", c.queueName, routingKey, err)
		}
	}

	return nil
}

func (c *CommandConsumer) processMessage(msg amqp.Delivery, handler CommandHandler) error {
	var cmd api.GitOpsCommandMessage
	if err := json.Unmarshal(msg.Body, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	result, err := handler.HandleCommand(&cmd)
	if err != nil {
		return fmt.Errorf("handler error: %w", err)
	}

	// Publish the result (including error-status results) to the results
	// exchange so the aggregator can materialize it into the read model.
	if result != nil {
		if err := c.publishResult(result); err != nil {
			return fmt.Errorf("failed to publish result: %w", err)
		}
		log.Printf("Published result for correlation ID %s (manifest=%s, status=%s)",
			result.CorrelationID, result.ManifestType, result.Status)
	}

	return nil
}

// publishResult marshals a result message and publishes it to the gitops.results
// topic exchange. The routing key is evt.<manifest-type>.<action>, matching the
// aggregator's evt.app.* / evt.environment.* / evt.context.* bindings.
func (c *CommandConsumer) publishResult(result *api.GitOpsResultMessage) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Clamp to the valid AMQP priority range before narrowing to uint8 so the
	// conversion cannot overflow on an out-of-range (negative or >255) value.
	p := result.Priority
	if p < 0 {
		p = 0
	}
	if p > 10 {
		p = 10
	}
	priority := uint8(p)

	return c.messageBus.channel.Publish(
		"gitops.results",         // exchange
		resultRoutingKey(result), // routing key
		false,                    // mandatory
		false,                    // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     result.MessageID,
			CorrelationId: result.CorrelationID,
			Timestamp:     result.CompletedAt,
			Priority:      priority,
			Body:          body,
			Headers: amqp.Table{
				"customer_id":   result.CustomerID,
				"context_name":  result.ContextName,
				"manifest_type": result.ManifestType,
				"service_name":  result.ServiceName,
				"status":        result.Status,
			},
		},
	)
}

// resultRoutingKey builds the gitops.results routing key for a result message.
func resultRoutingKey(result *api.GitOpsResultMessage) string {
	manifestType := result.ManifestType
	if manifestType == "" {
		manifestType = "manifest"
	}
	return fmt.Sprintf("evt.%s.%s", manifestType, result.Action)
}
