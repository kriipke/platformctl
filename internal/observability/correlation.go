package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/kriipke/platformctl/pkg/api"
)

// CorrelationTracker provides end-to-end correlation tracking for GitOps operations
type CorrelationTracker struct {
	logger  *Logger
	metrics *Metrics
}

// NewCorrelationTracker creates a new correlation tracker
func NewCorrelationTracker(logger *Logger, metrics *Metrics) *CorrelationTracker {
	return &CorrelationTracker{
		logger:  logger,
		metrics: metrics,
	}
}

// CorrelationContext represents the context for a correlated operation
type CorrelationContext struct {
	CorrelationID string    `json:"correlation_id"`
	CustomerID    string    `json:"customer_id"`
	ContextName   string    `json:"context_name"`
	Action        string    `json:"action"`
	ManifestType  string    `json:"manifest_type"`
	RequestedBy   string    `json:"requested_by"`
	RequestedAt   time.Time `json:"requested_at"`
	Stage         string    `json:"stage"`
	ServiceName   string    `json:"service_name"`
	StartTime     time.Time `json:"start_time"`
}

// CorrelationEvent represents an event in the correlation chain
type CorrelationEvent struct {
	CorrelationID string                 `json:"correlation_id"`
	EventType     string                 `json:"event_type"` // request, command, result, error
	ServiceName   string                 `json:"service_name"`
	Stage         string                 `json:"stage"`
	Timestamp     time.Time              `json:"timestamp"`
	Duration      time.Duration          `json:"duration,omitempty"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// TrackHTTPRequest tracks the start of an HTTP request in the correlation chain
func (ct *CorrelationTracker) TrackHTTPRequest(ctx context.Context, method, endpoint string) *CorrelationContext {
	correlationID := GetCorrelationID(ctx)
	customerID := GetCustomerID(ctx)
	
	correlationCtx := &CorrelationContext{
		CorrelationID: correlationID,
		CustomerID:    customerID,
		Action:        method + " " + endpoint,
		Stage:         "http_request",
		ServiceName:   "api-gateway",
		StartTime:     time.Now(),
		RequestedAt:   time.Now(),
	}
	
	// Log the start of the correlation chain
	ct.logger.GitOpsEvent(ctx, "correlation_start").
		WithAction(correlationCtx.Action).
		WithStatus("started").
		Send("HTTP request correlation started")
	
	// Record metric
	ct.metrics.IncrementHTTPRequests(customerID, method, endpoint, 0) // Status will be updated later
	
	return correlationCtx
}

// TrackCommandPublished tracks when a GitOps command is published to RabbitMQ
func (ct *CorrelationTracker) TrackCommandPublished(ctx context.Context, command *api.GitOpsCommandMessage, exchange, routingKey string) {
	correlationID := GetCorrelationID(ctx)
	
	event := CorrelationEvent{
		CorrelationID: correlationID,
		EventType:     "command",
		ServiceName:   "api-gateway",
		Stage:         "command_published",
		Timestamp:     time.Now(),
		Status:        "published",
		Metadata: map[string]interface{}{
			"exchange":      exchange,
			"routing_key":   routingKey,
			"command_type":  command.CommandType,
			"manifest_type": command.ManifestType,
			"target_service": command.TargetService,
		},
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	// Record RabbitMQ metrics
	ct.metrics.IncrementMessagesPublished(command.CustomerID, exchange, routingKey, "command")
}

// TrackCommandReceived tracks when a GitOps service receives a command
func (ct *CorrelationTracker) TrackCommandReceived(ctx context.Context, serviceName string, message amqp.Delivery) context.Context {
	// Extract correlation context from message
	ctx = WithCorrelationFromMessage(ctx, message)
	correlationID := GetCorrelationID(ctx)
	
	event := CorrelationEvent{
		CorrelationID: correlationID,
		EventType:     "command",
		ServiceName:   serviceName,
		Stage:         "command_received",
		Timestamp:     time.Now(),
		Status:        "received",
		Metadata: map[string]interface{}{
			"queue":      message.RoutingKey,
			"message_id": message.MessageId,
		},
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	return ctx
}

// TrackServiceExecution tracks the execution of a GitOps service operation
func (ct *CorrelationTracker) TrackServiceExecution(ctx context.Context, serviceName, operation string) *ServiceExecutionTracker {
	correlationID := GetCorrelationID(ctx)
	customerID := GetCustomerID(ctx)
	
	tracker := &ServiceExecutionTracker{
		correlationTracker: ct,
		CorrelationID:      correlationID,
		CustomerID:         customerID,
		ServiceName:        serviceName,
		Operation:          operation,
		StartTime:          time.Now(),
		ctx:                ctx,
	}
	
	event := CorrelationEvent{
		CorrelationID: correlationID,
		EventType:     "execution",
		ServiceName:   serviceName,
		Stage:         "execution_started",
		Timestamp:     time.Now(),
		Status:        "started",
		Metadata: map[string]interface{}{
			"operation": operation,
		},
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	// Increment active processing metric
	contextName := ctx.Value("context_name")
	if contextNameStr, ok := contextName.(string); ok {
		manifestType := ctx.Value("manifest_type")
		if manifestTypeStr, ok := manifestType.(string); ok {
			ct.metrics.IncrementCommandsActiveProcessing(customerID, contextNameStr, manifestTypeStr)
		}
	}
	
	return tracker
}

// TrackExternalAPICall tracks calls to external services (ArgoCD, Vault, etc.)
func (ct *CorrelationTracker) TrackExternalAPICall(ctx context.Context, serviceName, api, endpoint string) *ExternalAPICallTracker {
	correlationID := GetCorrelationID(ctx)
	customerID := GetCustomerID(ctx)
	
	tracker := &ExternalAPICallTracker{
		correlationTracker: ct,
		CorrelationID:      correlationID,
		CustomerID:         customerID,
		ServiceName:        serviceName,
		API:                api,
		Endpoint:           endpoint,
		StartTime:          time.Now(),
		ctx:                ctx,
	}
	
	event := CorrelationEvent{
		CorrelationID: correlationID,
		EventType:     "external_api",
		ServiceName:   serviceName,
		Stage:         "api_call_started",
		Timestamp:     time.Now(),
		Status:        "started",
		Metadata: map[string]interface{}{
			"api":      api,
			"endpoint": endpoint,
		},
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	return tracker
}

// TrackResultPublished tracks when a service publishes a result
func (ct *CorrelationTracker) TrackResultPublished(ctx context.Context, result *api.GitOpsResultMessage, exchange, routingKey string) {
	correlationID := GetCorrelationID(ctx)
	
	event := CorrelationEvent{
		CorrelationID: correlationID,
		EventType:     "result",
		ServiceName:   result.ServiceName,
		Stage:         "result_published",
		Timestamp:     time.Now(),
		Status:        result.Status,
		Metadata: map[string]interface{}{
			"exchange":      exchange,
			"routing_key":   routingKey,
			"manifest_type": result.ManifestType,
		},
	}
	
	if result.ErrorMessage != "" {
		event.Error = result.ErrorMessage
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	// Record RabbitMQ metrics
	ct.metrics.IncrementMessagesPublished(result.CustomerID, exchange, routingKey, "result")
}

// TrackResultProcessed tracks when the aggregator processes a result
func (ct *CorrelationTracker) TrackResultProcessed(ctx context.Context, result *api.GitOpsResultMessage, processingDuration time.Duration) {
	correlationID := GetCorrelationID(ctx)
	
	event := CorrelationEvent{
		CorrelationID: correlationID,
		EventType:     "result",
		ServiceName:   "gitops-aggregator",
		Stage:         "result_processed",
		Timestamp:     time.Now(),
		Duration:      processingDuration,
		Status:        "processed",
		Metadata: map[string]interface{}{
			"source_service": result.ServiceName,
			"manifest_type":  result.ManifestType,
		},
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	// Record processing metrics
	ct.metrics.RecordMessageProcessingDuration(result.CustomerID, "results."+result.ServiceName, "result", processingDuration)
}

// TrackCorrelationComplete marks the end of a correlation chain
func (ct *CorrelationTracker) TrackCorrelationComplete(ctx context.Context, correlationCtx *CorrelationContext, status string, err error) {
	duration := time.Since(correlationCtx.StartTime)
	
	event := CorrelationEvent{
		CorrelationID: correlationCtx.CorrelationID,
		EventType:     "correlation_complete",
		ServiceName:   correlationCtx.ServiceName,
		Stage:         "completed",
		Timestamp:     time.Now(),
		Duration:      duration,
		Status:        status,
		Metadata: map[string]interface{}{
			"total_duration_ms": duration.Milliseconds(),
		},
	}
	
	if err != nil {
		event.Error = err.Error()
		event.Status = "failed"
	}
	
	ct.logCorrelationEvent(ctx, event)
	
	// Log correlation summary
	ct.logger.GitOpsEvent(ctx, "correlation_complete").
		WithContext(correlationCtx.ContextName).
		WithAction(correlationCtx.Action).
		WithStatus(status).
		WithDuration(duration).
		WithError(err).
		Send("GitOps operation correlation completed")
}

// ServiceExecutionTracker tracks the execution of a service operation
type ServiceExecutionTracker struct {
	correlationTracker *CorrelationTracker
	CorrelationID      string
	CustomerID         string
	ServiceName        string
	Operation          string
	StartTime          time.Time
	ctx                context.Context
}

// Complete marks the service execution as complete
func (set *ServiceExecutionTracker) Complete(status string, err error) {
	duration := time.Since(set.StartTime)
	
	event := CorrelationEvent{
		CorrelationID: set.CorrelationID,
		EventType:     "execution",
		ServiceName:   set.ServiceName,
		Stage:         "execution_completed",
		Timestamp:     time.Now(),
		Duration:      duration,
		Status:        status,
		Metadata: map[string]interface{}{
			"operation": set.Operation,
		},
	}
	
	if err != nil {
		event.Error = err.Error()
		if status == "" {
			event.Status = "failed"
		}
	}
	
	set.correlationTracker.logCorrelationEvent(set.ctx, event)
	
	// Record metrics
	contextName := set.ctx.Value("context_name")
	if contextNameStr, ok := contextName.(string); ok {
		manifestType := set.ctx.Value("manifest_type")
		if manifestTypeStr, ok := manifestType.(string); ok {
			set.correlationTracker.metrics.IncrementCommandsProcessed(set.CustomerID, contextNameStr, manifestTypeStr, status)
			set.correlationTracker.metrics.RecordCommandProcessingDuration(set.CustomerID, contextNameStr, manifestTypeStr, duration)
			set.correlationTracker.metrics.DecrementCommandsActiveProcessing(set.CustomerID, contextNameStr, manifestTypeStr)
		}
	}
}

// ExternalAPICallTracker tracks external API calls
type ExternalAPICallTracker struct {
	correlationTracker *CorrelationTracker
	CorrelationID      string
	CustomerID         string
	ServiceName        string
	API                string
	Endpoint           string
	StartTime          time.Time
	ctx                context.Context
}

// Complete marks the external API call as complete
func (eact *ExternalAPICallTracker) Complete(statusCode int, err error) {
	duration := time.Since(eact.StartTime)
	status := "success"
	
	if err != nil {
		status = "error"
	} else if statusCode >= 400 {
		status = "client_error"
		if statusCode >= 500 {
			status = "server_error"
		}
	}
	
	event := CorrelationEvent{
		CorrelationID: eact.CorrelationID,
		EventType:     "external_api",
		ServiceName:   eact.ServiceName,
		Stage:         "api_call_completed",
		Timestamp:     time.Now(),
		Duration:      duration,
		Status:        status,
		Metadata: map[string]interface{}{
			"api":         eact.API,
			"endpoint":    eact.Endpoint,
			"status_code": statusCode,
		},
	}
	
	if err != nil {
		event.Error = err.Error()
	}
	
	eact.correlationTracker.logCorrelationEvent(eact.ctx, event)
	
	// Record metrics
	eact.correlationTracker.metrics.IncrementExternalAPICalls(eact.CustomerID, eact.API, eact.Endpoint, statusCode)
	eact.correlationTracker.metrics.RecordExternalAPICallDuration(eact.CustomerID, eact.API, eact.Endpoint, duration)
	
	if err != nil {
		errorType := "network"
		if statusCode >= 400 {
			errorType = "http_error"
		}
		eact.correlationTracker.metrics.IncrementExternalAPICallErrors(eact.CustomerID, eact.API, eact.Endpoint, errorType)
	}
}

// Helper functions

// WithCorrelationFromMessage extracts correlation context from a RabbitMQ message
func WithCorrelationFromMessage(ctx context.Context, message amqp.Delivery) context.Context {
	// Extract correlation ID
	correlationID := message.CorrelationId
	if correlationID == "" && message.Headers != nil {
		if headerCorrelationID, ok := message.Headers["correlation_id"].(string); ok {
			correlationID = headerCorrelationID
		}
	}
	
	if correlationID != "" {
		ctx = WithCorrelationID(ctx, correlationID)
	}
	
	// Extract customer ID
	if message.Headers != nil {
		if customerID, ok := message.Headers["customer_id"].(string); ok {
			ctx = WithCustomerID(ctx, customerID)
		}
	}
	
	// Try to extract additional context from message body if it's a GitOps message
	if message.ContentType == "application/json" {
		var envelope api.GitOpsMessageEnvelope
		if err := json.Unmarshal(message.Body, &envelope); err == nil {
			ctx = WithCustomerID(ctx, envelope.CustomerID)
			ctx = WithCorrelationID(ctx, envelope.CorrelationID)
			ctx = context.WithValue(ctx, "context_name", envelope.ContextName)
			ctx = context.WithValue(ctx, "manifest_type", envelope.ManifestType)
		}
	}
	
	return ctx
}

// logCorrelationEvent logs a correlation event with structured data
func (ct *CorrelationTracker) logCorrelationEvent(ctx context.Context, event CorrelationEvent) {
	logEvent := ct.logger.GitOpsEvent(ctx, "correlation_event").
		WithStatus(event.Status).
		WithDuration(event.Duration)
	
	if event.Metadata != nil {
		for key, value := range event.Metadata {
			if str, ok := value.(string); ok {
				// Add string metadata as structured fields (simplified)
				switch key {
				case "operation":
					logEvent.WithAction(str)
				case "manifest_type":
					logEvent.WithManifest(str, "")
				}
			}
		}
	}
	
	if event.Error != "" {
		logEvent.WithError(fmt.Errorf("%s", event.Error))
	}
	
	logEvent.Send(fmt.Sprintf("%s: %s", event.ServiceName, event.Stage))
}

// Global correlation tracker instance
var globalCorrelationTracker *CorrelationTracker

// InitGlobalCorrelationTracker initializes the global correlation tracker
func InitGlobalCorrelationTracker(logger *Logger, metrics *Metrics) {
	globalCorrelationTracker = NewCorrelationTracker(logger, metrics)
}

// GetGlobalCorrelationTracker returns the global correlation tracker
func GetGlobalCorrelationTracker() *CorrelationTracker {
	return globalCorrelationTracker
}

// Global convenience functions

// TrackHTTPRequest tracks an HTTP request using the global correlation tracker
func TrackHTTPRequest(ctx context.Context, method, endpoint string) *CorrelationContext {
	if globalCorrelationTracker != nil {
		return globalCorrelationTracker.TrackHTTPRequest(ctx, method, endpoint)
	}
	return nil
}

// TrackServiceExecution tracks service execution using the global correlation tracker
func TrackServiceExecution(ctx context.Context, serviceName, operation string) *ServiceExecutionTracker {
	if globalCorrelationTracker != nil {
		return globalCorrelationTracker.TrackServiceExecution(ctx, serviceName, operation)
	}
	return nil
}

// TrackExternalAPICall tracks external API calls using the global correlation tracker
func TrackExternalAPICall(ctx context.Context, serviceName, api, endpoint string) *ExternalAPICallTracker {
	if globalCorrelationTracker != nil {
		return globalCorrelationTracker.TrackExternalAPICall(ctx, serviceName, api, endpoint)
	}
	return nil
}