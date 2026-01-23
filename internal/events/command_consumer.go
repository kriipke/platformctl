package events

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/contextops/platformctl/pkg/api"
	amqp "github.com/rabbitmq/amqp091-go"
)

type CommandConsumer struct {
	messageBus *GitOpsMessageBus
	queueName  string
	stopChan   chan struct{}
}

type CommandHandler interface {
	HandleCommand(cmd *api.GitOpsCommandMessage) (*api.GitOpsResultMessage, error)
}

func NewCommandConsumer(mb *GitOpsMessageBus, queueName string) *CommandConsumer {
	return &CommandConsumer{
		messageBus: mb,
		queueName:  queueName,
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
	// Default binding - services should override this for specific bindings
	bindings := []string{
		"cmd.app.*",
		"cmd.environment.*",
		"cmd.context.*",
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

	// Publish result if handler returned one
	if result != nil {
		// TODO: Publish result to results exchange
		log.Printf("Would publish result for correlation ID: %s", result.CorrelationID)
	}

	return nil
}