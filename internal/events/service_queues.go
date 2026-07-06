package events

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ServiceQueueManager handles service-specific queue setup and binding
type ServiceQueueManager struct {
	messageBus *GitOpsMessageBus
}

func NewServiceQueueManager(mb *GitOpsMessageBus) *ServiceQueueManager {
	return &ServiceQueueManager{
		messageBus: mb,
	}
}

// SetupAllServiceQueues sets up queues and bindings for all Phase 1C services
func (sqm *ServiceQueueManager) SetupAllServiceQueues() error {
	services := map[string][]string{
		// Environment Manifest Validation Service
		"gitops.environment-validation.q": {
			"cmd.environment.*",
			"cmd.context.validate-environments",
		},

		// App Manifest Sync Service
		"gitops.app-sync.q": {
			"cmd.app.*",
			"cmd.context.sync-apps",
		},

		// Context Pairing Correlation Service
		"gitops.context-correlation.q": {
			"cmd.context.correlate-contexts",
			"cmd.context.inspect-manifests",
		},

		// Multi-Environment Kubernetes Service
		"gitops.multi-environment-kubernetes.q": {
			"cmd.context.correlate-context",
			"cmd.environment.correlate-context",
		},

		// Customer Git Branch Service
		"gitops.customer-git-branch.q": {
			"cmd.git.*",
			"cmd.context.sync-customer-branch",
		},
	}

	for queueName, routingKeys := range services {
		if err := sqm.setupServiceQueue(queueName, routingKeys); err != nil {
			return fmt.Errorf("failed to setup service queue %s: %w", queueName, err)
		}
	}

	return nil
}

// setupServiceQueue creates a queue and binds it to the specified routing keys
func (sqm *ServiceQueueManager) setupServiceQueue(queueName string, routingKeys []string) error {
	// Declare queue with service-specific settings
	_, err := sqm.messageBus.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-message-ttl":          600000, // 10 minutes TTL
			"x-max-priority":         10,
			"x-dead-letter-exchange": "gitops.dlx",
			"description":            fmt.Sprintf("GitOps service queue for %s", queueName),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	// Bind to routing keys
	for _, routingKey := range routingKeys {
		if err := sqm.messageBus.channel.QueueBind(
			queueName,
			routingKey,
			"gitops.commands",
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind queue %s to %s: %w", queueName, routingKey, err)
		}
	}

	return nil
}

// UpdateCommandConsumerBindings updates the command consumer to use service-specific bindings
func UpdateCommandConsumerBindings(consumer *CommandConsumer, serviceName string) error {
	// Service-specific binding configurations
	serviceBindings := map[string][]string{
		"environment-validation": {
			"cmd.environment.*",
			"cmd.context.validate-environments",
		},
		"app-sync": {
			"cmd.app.*",
			"cmd.context.sync-apps",
		},
		"context-correlation": {
			"cmd.context.correlate-contexts",
			"cmd.context.inspect-manifests",
		},
		"multi-environment-kubernetes": {
			"cmd.context.correlate-context",
			"cmd.environment.correlate-context",
		},
		"customer-git-branch": {
			"cmd.git.*",
			"cmd.context.sync-customer-branch",
		},
	}

	bindings, exists := serviceBindings[serviceName]
	if !exists {
		// Fallback to default bindings
		bindings = []string{
			"cmd.app.*",
			"cmd.environment.*",
			"cmd.context.*",
		}
	}

	// Update consumer bindings (this would be used in the consumer setup)
	_ = bindings // Implementation would update the consumer's binding configuration

	return nil
}
