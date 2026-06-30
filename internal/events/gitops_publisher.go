package events

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/contextops/platformctl/pkg/api"
	amqp "github.com/rabbitmq/amqp091-go"
)

type GitOpsCommandPublisher struct {
	messageBus *GitOpsMessageBus
	logger     *log.Logger
}

func NewGitOpsCommandPublisher(gmb *GitOpsMessageBus) *GitOpsCommandPublisher {
	return &GitOpsCommandPublisher{
		messageBus: gmb,
		logger:     log.New(os.Stdout, "[GitOpsPublisher] ", log.LstdFlags),
	}
}

func (p *GitOpsCommandPublisher) PublishGitOpsCommand(cmd *api.GitOpsCommandMessage) error {
	body, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal GitOps command: %w", err)
	}

	routingKey := p.generateRoutingKey(cmd)

	// Set message priority and properties based on GitOps type
	priority := uint8(cmd.Priority)
	if priority > 10 {
		priority = 10
	}

	err = p.messageBus.channel.Publish(
		"gitops.commands", // exchange
		routingKey,        // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     cmd.MessageID,
			CorrelationId: cmd.CorrelationID,
			Timestamp:     cmd.RequestedAt,
			Priority:      priority,
			Body:          body,
			Headers: amqp.Table{
				"customer_id":           cmd.CustomerID,
				"context_name":          cmd.ContextName,
				"action":               cmd.Action,
				"manifest_type":        cmd.ManifestType,
				"app_name":             cmd.AppName,
				"environment_name":     cmd.EnvironmentName,
				"requested_by":         cmd.RequestedBy,
				"target_service":       cmd.TargetService,
				"app_reference":        cmd.ManifestMetadata.AppReference,
				"environment_reference": cmd.ManifestMetadata.EnvironmentReference,
				"customer_branch":      cmd.ManifestMetadata.CustomerBranch,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish GitOps command: %w", err)
	}

	p.logger.Printf("Published GitOps command: %s (manifest: %s, customer: %s, correlation: %s)",
		cmd.Action, cmd.ManifestType, cmd.CustomerID, cmd.CorrelationID)

	return nil
}

func (p *GitOpsCommandPublisher) generateRoutingKey(cmd *api.GitOpsCommandMessage) string {
	switch cmd.ManifestType {
	case "app":
		return fmt.Sprintf("cmd.app.%s", cmd.Action)
	case "environment":
		return fmt.Sprintf("cmd.environment.%s", cmd.Action)
	case "context":
		return fmt.Sprintf("cmd.context.%s", cmd.Action)
	case "kubernetes":
		return fmt.Sprintf("cmd.kubernetes.%s", cmd.Action)
	default:
		return fmt.Sprintf("cmd.manifest.%s", cmd.Action)
	}
}

// Manifest-specific command publishing methods
func (p *GitOpsCommandPublisher) PublishAppSync(customerID, contextName, appName, user string) (*api.GitOpsCommandMessage, error) {
	cmd := api.NewAppManifestCommandMessage(customerID, contextName, appName, user)
	cmd.Action = "sync-app"
	cmd.CommandType = "sync"

	// Add App manifest specific payload
	cmd.Payload["sync_applicationset"] = true
	cmd.Payload["validate_helm_sources"] = true
	cmd.Payload["check_git_sources"] = true

	return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishEnvironmentValidation(customerID, contextName, environmentName, user string) (*api.GitOpsCommandMessage, error) {
	cmd := api.NewEnvironmentManifestCommandMessage(customerID, contextName, environmentName, user)
	cmd.Action = "validate-environment"
	cmd.CommandType = "validate"

	// Add Environment manifest specific payload
	cmd.Payload["validate_vault_sources"] = true
	cmd.Payload["validate_cluster_configs"] = true
	cmd.Payload["validate_values_files"] = true
	cmd.Payload["check_pod_env"] = true

	return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishContextCorrelation(customerID, contextName, user string, appReference, environmentReference string) (*api.GitOpsCommandMessage, error) {
	cmd := api.NewContextPairingCommandMessage(customerID, contextName, user)
	cmd.Action = "correlate-context"
	cmd.CommandType = "correlate"
	cmd.ManifestMetadata.AppReference = appReference
	cmd.ManifestMetadata.EnvironmentReference = environmentReference

	// Add Context pairing correlation payload
	cmd.Payload["app_reference"] = appReference
	cmd.Payload["environment_reference"] = environmentReference
	cmd.Payload["validate_pairing"] = true
	cmd.Payload["sync_after_correlation"] = true

	return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishManifestInspection(customerID, contextName, user string, manifestType string) (*api.GitOpsCommandMessage, error) {
	cmd := api.NewGitOpsCommandMessage(customerID, contextName, "inspect-manifests", manifestType, user)
	cmd.TargetService = fmt.Sprintf("%s-manifest-inspector", manifestType)
	cmd.CommandType = "inspect"
	cmd.Priority = 6 // Medium-high priority for manifest inspection

	// Add manifest inspection specific payload
	cmd.Payload["manifest_type"] = manifestType
	cmd.Payload["deep_inspection"] = true
	cmd.Payload["validate_references"] = true
	cmd.Payload["include_performance_metrics"] = true

	return cmd, p.PublishGitOpsCommand(cmd)
}

func (p *GitOpsCommandPublisher) PublishCustomerBranchSync(customerID, contextName, customerBranch, user string) (*api.GitOpsCommandMessage, error) {
	cmd := api.NewGitOpsCommandMessage(customerID, contextName, "sync-customer-branch", "git", user)
	cmd.TargetService = "git-sync-service"
	cmd.ManifestMetadata.CustomerBranch = customerBranch
	cmd.Priority = 7 // High priority for customer branch changes

	// Add customer branch sync payload
	cmd.Payload["customer_branch"] = customerBranch
	cmd.Payload["sync_all_environments"] = true
	cmd.Payload["validate_after_sync"] = true

	return cmd, p.PublishGitOpsCommand(cmd)
}

// PublishMultiEnvironmentCorrelation publishes a command for the
// multi-environment Kubernetes service. It uses the "kubernetes" manifest type
// so it routes to cmd.kubernetes.* (the multi-environment service's binding)
// rather than competing with the context-correlation service on cmd.context.*.
func (p *GitOpsCommandPublisher) PublishMultiEnvironmentCorrelation(customerID, contextName, user string) (*api.GitOpsCommandMessage, error) {
	cmd := api.NewGitOpsCommandMessage(customerID, contextName, "correlate-context", "kubernetes", user)
	cmd.TargetService = "multi-environment-kubernetes"
	cmd.CommandType = "correlate"
	cmd.Priority = 6

	cmd.Payload["correlate_all_environments"] = true
	cmd.Payload["include_workload_status"] = true

	return cmd, p.PublishGitOpsCommand(cmd)
}

// Batch operations for multiple Context pairing commands
func (p *GitOpsCommandPublisher) PublishMultiContextCorrelation(customerID, contextName, user string, contextPairings []struct{ AppRef, EnvRef string }) ([]*api.GitOpsCommandMessage, error) {
	var commands []*api.GitOpsCommandMessage
	var errors []error

	for _, pairing := range contextPairings {
		cmd, err := p.PublishContextCorrelation(customerID, contextName, user, pairing.AppRef, pairing.EnvRef)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to publish command for context pairing %s+%s: %w", pairing.AppRef, pairing.EnvRef, err))
			continue
		}
		commands = append(commands, cmd)
	}

	if len(errors) > 0 {
		return commands, fmt.Errorf("some context pairing commands failed: %v", errors)
	}

	return commands, nil
}