package events

import (
	"fmt"
	"log"

	"github.com/contextops/platformctl/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

// GitOpsRabbitMQ is an alias for GitOpsMessageBus for backward compatibility
type GitOpsRabbitMQ = GitOpsMessageBus

type GitOpsMessageBus struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.Config
}

func NewGitOpsMessageBus(url string, cfg *config.Config) (*GitOpsMessageBus, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	gmb := &GitOpsMessageBus{
		conn:    conn,
		channel: channel,
		config:  cfg,
	}

	if err := gmb.setupGitOpsTopology(); err != nil {
		return nil, fmt.Errorf("failed to setup GitOps topology: %w", err)
	}

	return gmb, nil
}

func (gmb *GitOpsMessageBus) setupGitOpsTopology() error {
	// GitOps Commands Exchange
	if err := gmb.channel.ExchangeDeclare(
		"gitops.commands", // name
		"topic",          // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		amqp.Table{
			"description": "GitOps commands for ApplicationSet monitoring, Vault validation, and environment correlation",
		},
	); err != nil {
		return err
	}

	// GitOps Results Exchange
	if err := gmb.channel.ExchangeDeclare(
		"gitops.results", // name
		"topic",         // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		amqp.Table{
			"description": "GitOps results from ApplicationSet monitoring, Vault validation, and environment correlation",
		},
	); err != nil {
		return err
	}

	// ApplicationSet Monitoring Queue
	_, err := gmb.channel.QueueDeclare(
		"gitops.applicationset-monitor.q", // name
		true,                              // durable
		false,                             // delete when unused
		false,                             // exclusive
		false,                             // no-wait
		amqp.Table{
			"x-message-ttl":            300000, // 5 minutes TTL
			"x-max-priority":           10,
			"description":              "ApplicationSet monitoring commands",
		},
	)
	if err != nil {
		return err
	}

	// Vault Secrets Validation Queue
	_, err = gmb.channel.QueueDeclare(
		"gitops.vault-validation.q", // name
		true,                        // durable
		false,                       // delete when unused
		false,                       // exclusive
		false,                       // no-wait
		amqp.Table{
			"x-message-ttl":   600000, // 10 minutes TTL
			"x-max-priority":  10,
			"description":     "Vault secret validation commands",
		},
	)
	if err != nil {
		return err
	}

	// Environment Correlation Queue
	_, err = gmb.channel.QueueDeclare(
		"gitops.environment-correlation.q", // name
		true,                               // durable
		false,                              // delete when unused
		false,                              // exclusive
		false,                              // no-wait
		amqp.Table{
			"x-message-ttl":   180000, // 3 minutes TTL
			"x-max-priority":  5,
			"description":     "Multi-environment status correlation commands",
		},
	)
	if err != nil {
		return err
	}

	// Aggregator Queue (customer-aware)
	_, err = gmb.channel.QueueDeclare(
		"gitops.aggregator.q", // name
		true,                  // durable
		false,                 // delete when unused
		false,                 // exclusive
		false,                 // no-wait
		amqp.Table{
			"x-message-ttl":   900000, // 15 minutes TTL
			"description":     "GitOps results aggregation queue",
		},
	)
	if err != nil {
		return err
	}

	// Bind queues to exchanges with GitOps-specific routing keys
	bindings := []struct {
		queue      string
		exchange   string
		routingKey string
	}{
		// App manifest synchronization bindings
		{"gitops.applicationset-monitor.q", "gitops.commands", "cmd.app.*"},
		{"gitops.applicationset-monitor.q", "gitops.commands", "cmd.applicationset.*"},

		// Environment manifest validation bindings
		{"gitops.vault-validation.q", "gitops.commands", "cmd.environment.*"},
		{"gitops.vault-validation.q", "gitops.commands", "cmd.vault.*"},

		// Context pairing correlation bindings
		{"gitops.environment-correlation.q", "gitops.commands", "cmd.context.*"},
		{"gitops.environment-correlation.q", "gitops.commands", "cmd.correlate.*"},

		// Aggregator bindings for all manifest results
		{"gitops.aggregator.q", "gitops.results", "evt.app.*"},
		{"gitops.aggregator.q", "gitops.results", "evt.environment.*"},
		{"gitops.aggregator.q", "gitops.results", "evt.context.*"},
		{"gitops.aggregator.q", "gitops.results", "evt.correlation.*"},
	}

	for _, binding := range bindings {
		if err := gmb.channel.QueueBind(
			binding.queue,      // queue name
			binding.routingKey, // routing key
			binding.exchange,   // exchange
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s with key %s: %w",
				binding.queue, binding.exchange, binding.routingKey, err)
		}
	}

	// Setup Dead Letter Queues for GitOps failures
	if err := gmb.setupGitOpsDLQ(); err != nil {
		return fmt.Errorf("failed to setup GitOps DLQ: %w", err)
	}

	return nil
}

func (gmb *GitOpsMessageBus) setupGitOpsDLQ() error {
	// GitOps Dead Letter Exchange
	if err := gmb.channel.ExchangeDeclare(
		"gitops.dlx", // name
		"topic",     // type
		true,        // durable
		false,       // auto-deleted
		false,       // internal
		false,       // no-wait
		amqp.Table{
			"description": "GitOps Dead Letter Exchange for failed commands",
		},
	); err != nil {
		return err
	}

	// GitOps Dead Letter Queue
	_, err := gmb.channel.QueueDeclare(
		"gitops.dlq", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		amqp.Table{
			"description": "GitOps Dead Letter Queue",
		},
	)
	if err != nil {
		return err
	}

	// Bind DLQ to DLX
	return gmb.channel.QueueBind(
		"gitops.dlq", // queue name
		"#",          // routing key (catch all)
		"gitops.dlx", // exchange
		false,
		nil,
	)
}

func (gmb *GitOpsMessageBus) Close() error {
	if gmb.channel != nil {
		if err := gmb.channel.Close(); err != nil {
			log.Printf("Error closing RabbitMQ channel: %v", err)
		}
	}
	if gmb.conn != nil {
		return gmb.conn.Close()
	}
	return nil
}

func (gmb *GitOpsMessageBus) GetChannel() (*amqp.Channel, error) {
	if gmb.channel == nil || gmb.channel.IsClosed() {
		return nil, fmt.Errorf("channel is closed or nil")
	}
	return gmb.channel, nil
}

func (gmb *GitOpsMessageBus) IsConnected() bool {
	return gmb.conn != nil && !gmb.conn.IsClosed()
}