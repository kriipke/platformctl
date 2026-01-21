# PHASE 1B: APIs and Messaging Infrastructure

**Duration:** 2-3 days  
**Prerequisites:** Phase 1A completed, RabbitMQ instance available  
**Deliverable:** Event-driven command/result system with action endpoints

---

## Overview

Extend the API Gateway with RabbitMQ integration for event-driven workflows. Add action endpoints that publish command events and implement the basic message envelope structure. This phase establishes the async communication backbone for integration services.

## Success Criteria

✅ RabbitMQ connection and topology established  
✅ Command publishing system working  
✅ Result consumption system working  
✅ Action endpoints implemented (refresh, validate, inspect)  
✅ Message envelope structure implemented  
✅ Correlation ID tracking working  
✅ Basic error handling and DLQ setup  
✅ Integration tests for message flow  

---

## Implementation Tasks

### Task 1: RabbitMQ Integration Setup

**File: `internal/events/rabbitmq.go`**

```go
type MessageBus struct {
    conn    *amqp.Connection
    channel *amqp.Channel
}

func NewMessageBus(url string) (*MessageBus, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
    }
    
    channel, err := conn.Channel()
    if err != nil {
        return nil, fmt.Errorf("failed to open channel: %w", err)
    }
    
    mb := &MessageBus{
        conn:    conn,
        channel: channel,
    }
    
    if err := mb.setupTopology(); err != nil {
        return nil, fmt.Errorf("failed to setup topology: %w", err)
    }
    
    return mb, nil
}

func (mb *MessageBus) setupTopology() error {
    // Declare exchanges
    if err := mb.channel.ExchangeDeclare(
        "contextops.commands", // name
        "topic",              // type
        true,                 // durable
        false,                // auto-deleted
        false,                // internal
        false,                // no-wait
        nil,                  // arguments
    ); err != nil {
        return err
    }
    
    if err := mb.channel.ExchangeDeclare(
        "contextops.results", // name
        "topic",             // type
        true,                // durable
        false,               // auto-deleted
        false,               // internal
        false,               // no-wait
        nil,                 // arguments
    ); err != nil {
        return err
    }
    
    // Declare aggregator queue (we'll use this in Phase 1D)
    _, err := mb.channel.QueueDeclare(
        "aggregator.q", // name
        true,          // durable
        false,         // delete when unused
        false,         // exclusive
        false,         // no-wait
        nil,           // arguments
    )
    if err != nil {
        return err
    }
    
    // Bind aggregator queue to results
    if err := mb.channel.QueueBind(
        "aggregator.q",              // queue name
        "evt.context.result.*",      // routing key
        "contextops.results",        // exchange
        false,
        nil,
    ); err != nil {
        return err
    }
    
    return nil
}
```

### Task 2: Message Envelope Structure

**File: `pkg/api/messages.go`**

```go
type MessageEnvelope struct {
    SchemaVersion  int                    `json:"schema_version"`
    MessageID      string                 `json:"message_id"`
    CorrelationID  string                 `json:"correlation_id"`
    ContextName    string                 `json:"context_name"`
    Action         string                 `json:"action"`
    RequestedBy    string                 `json:"requested_by"`
    RequestedAt    time.Time              `json:"requested_at"`
    Payload        map[string]interface{} `json:"payload"`
}

type CommandMessage struct {
    MessageEnvelope
    // Command-specific fields can be added here
}

type ResultMessage struct {
    MessageEnvelope
    ServiceName   string      `json:"service_name"`
    Status        string      `json:"status"` // ok, degraded, error
    CompletedAt   time.Time   `json:"completed_at"`
    ErrorMessage  string      `json:"error_message,omitempty"`
    ResultPayload interface{} `json:"result_payload"`
}

// Message creation helpers
func NewCommandMessage(contextName, action, user string) *CommandMessage {
    return &CommandMessage{
        MessageEnvelope: MessageEnvelope{
            SchemaVersion: 1,
            MessageID:     generateUUID(),
            CorrelationID: generateUUID(),
            ContextName:   contextName,
            Action:        action,
            RequestedBy:   user,
            RequestedAt:   time.Now().UTC(),
            Payload:       make(map[string]interface{}),
        },
    }
}
```

### Task 3: Command Publishing Service

**File: `internal/events/publisher.go`**

```go
type CommandPublisher struct {
    messageBus *MessageBus
}

func NewCommandPublisher(mb *MessageBus) *CommandPublisher {
    return &CommandPublisher{messageBus: mb}
}

func (p *CommandPublisher) PublishCommand(cmd *api.CommandMessage) error {
    body, err := json.Marshal(cmd)
    if err != nil {
        return fmt.Errorf("failed to marshal command: %w", err)
    }
    
    routingKey := fmt.Sprintf("cmd.context.%s", cmd.Action)
    
    err = p.messageBus.channel.Publish(
        "contextops.commands", // exchange
        routingKey,            // routing key
        false,                 // mandatory
        false,                 // immediate
        amqp.Publishing{
            ContentType:   "application/json",
            DeliveryMode:  amqp.Persistent,
            MessageId:     cmd.MessageID,
            CorrelationId: cmd.CorrelationID,
            Timestamp:     cmd.RequestedAt,
            Body:          body,
            Headers: amqp.Table{
                "context_name": cmd.ContextName,
                "action":       cmd.Action,
                "requested_by": cmd.RequestedBy,
            },
        },
    )
    
    if err != nil {
        return fmt.Errorf("failed to publish command: %w", err)
    }
    
    return nil
}

// Publish specific command types
func (p *CommandPublisher) PublishRefresh(contextName, user string) (*api.CommandMessage, error) {
    cmd := api.NewCommandMessage(contextName, "refresh", user)
    return cmd, p.PublishCommand(cmd)
}

func (p *CommandPublisher) PublishValidate(contextName, user string) (*api.CommandMessage, error) {
    cmd := api.NewCommandMessage(contextName, "validate", user)
    return cmd, p.PublishCommand(cmd)
}

func (p *CommandPublisher) PublishInspect(contextName, user string) (*api.CommandMessage, error) {
    cmd := api.NewCommandMessage(contextName, "inspect", user)
    return cmd, p.PublishCommand(cmd)
}
```

### Task 4: Action Endpoints Implementation

**File: `internal/handlers/actions.go`**

```go
type ActionHandler struct {
    contextStore *storage.ContextStore
    publisher    *events.CommandPublisher
}

func NewActionHandler(store *storage.ContextStore, pub *events.CommandPublisher) *ActionHandler {
    return &ActionHandler{
        contextStore: store,
        publisher:    pub,
    }
}

type ActionResponse struct {
    Success       bool   `json:"success"`
    CorrelationID string `json:"correlation_id"`
    Message       string `json:"message"`
    Action        string `json:"action"`
}

func (h *ActionHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
    contextName := mux.Vars(r)["name"]
    user := getUserFromContext(r.Context())
    
    // Verify context exists
    _, err := h.contextStore.Get(r.Context(), contextName)
    if err != nil {
        if err == storage.ErrContextNotFound {
            http.Error(w, "Context not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to verify context", http.StatusInternalServerError)
        return
    }
    
    // Publish refresh command
    cmd, err := h.publisher.PublishRefresh(contextName, user)
    if err != nil {
        http.Error(w, "Failed to publish refresh command", http.StatusInternalServerError)
        return
    }
    
    response := ActionResponse{
        Success:       true,
        CorrelationID: cmd.CorrelationID,
        Message:       "Refresh command published successfully",
        Action:        "refresh",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (h *ActionHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
    // Similar implementation for validate
}

func (h *ActionHandler) HandleInspect(w http.ResponseWriter, r *http.Request) {
    // Similar implementation for inspect  
}

// Note: Sync endpoint will be added in Phase 2A after security hardening
```

### Task 5: Result Consumer Framework

**File: `internal/events/consumer.go`**

```go
type ResultConsumer struct {
    messageBus   *MessageBus
    handlers     map[string]ResultHandler
    stopChan     chan struct{}
}

type ResultHandler interface {
    HandleResult(result *api.ResultMessage) error
}

func NewResultConsumer(mb *MessageBus) *ResultConsumer {
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
        "aggregator.q", // queue
        "",            // consumer
        false,         // auto-ack (we'll manually ack)
        false,         // exclusive
        false,         // no-local
        false,         // no-wait
        nil,           // args
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
                    msg.Nack(false, true) // Requeue on error
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

func (c *ResultConsumer) processMessage(msg amqp.Delivery) error {
    var result api.ResultMessage
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
```

### Task 6: Enhanced API Gateway Main

**File: `cmd/gateway/main.go`** (update existing)

```go
func main() {
    // Configuration loading
    cfg := loadConfig()
    
    // Database connection
    db := setupDatabase(cfg.DatabaseURL)
    defer db.Close()
    
    // RabbitMQ connection
    messageBus, err := events.NewMessageBus(cfg.RabbitMQURL)
    if err != nil {
        log.Fatal("Failed to connect to RabbitMQ:", err)
    }
    defer messageBus.Close()
    
    // Dependencies
    contextStore := storage.NewContextStore(db)
    publisher := events.NewCommandPublisher(messageBus)
    
    // Handlers
    contextHandler := handlers.NewContextHandler(contextStore)
    actionHandler := handlers.NewActionHandler(contextStore, publisher)
    
    // Router setup with new action endpoints
    router := setupRouter(contextHandler, actionHandler)
    
    // Start result consumer (basic setup for Phase 1D)
    consumer := events.NewResultConsumer(messageBus)
    if err := consumer.Start(); err != nil {
        log.Fatal("Failed to start consumer:", err)
    }
    defer consumer.Stop()
    
    // Server startup
    server := &http.Server{
        Addr:    cfg.Port,
        Handler: router,
    }
    
    log.Printf("Server starting on %s", cfg.Port)
    log.Fatal(server.ListenAndServe())
}

func setupRouter(contextHandler *handlers.ContextHandler, actionHandler *handlers.ActionHandler) *mux.Router {
    r := mux.NewRouter()
    
    // Middleware
    r.Use(auth.BasicAuthMiddleware())
    r.Use(loggingMiddleware())
    
    // Context CRUD routes (from Phase 1A)
    r.HandleFunc("/contexts", contextHandler.CreateContext).Methods("POST")
    r.HandleFunc("/contexts", contextHandler.ListContexts).Methods("GET")
    r.HandleFunc("/contexts/{name}", contextHandler.GetContext).Methods("GET")
    r.HandleFunc("/contexts/{name}", contextHandler.UpdateContext).Methods("PUT")
    r.HandleFunc("/contexts/{name}", contextHandler.DeleteContext).Methods("DELETE")
    
    // New action routes (Phase 1B)
    r.HandleFunc("/contexts/{name}/actions/refresh", actionHandler.HandleRefresh).Methods("POST")
    r.HandleFunc("/contexts/{name}/actions/validate", actionHandler.HandleValidate).Methods("POST")
    r.HandleFunc("/contexts/{name}/actions/inspect", actionHandler.HandleInspect).Methods("POST")
    
    // Health check
    r.HandleFunc("/health", healthCheck).Methods("GET")
    
    return r
}
```

### Task 7: Configuration Updates

**File: `internal/config/config.go`** (update existing)

```go
type Config struct {
    Port         string `env:"PORT" envDefault:":8080"`
    DatabaseURL  string `env:"DATABASE_URL" envDefault:"postgres://localhost/contextops?sslmode=disable"`
    RabbitMQURL  string `env:"RABBITMQ_URL" envDefault:"amqp://localhost:5672/"`
    LogLevel     string `env:"LOG_LEVEL" envDefault:"info"`
}
```

### Task 8: Run Tracking Database Schema

**File: `migrations/002_run_tracking.up.sql`**

```sql
-- Table to track command runs and their correlation IDs
CREATE TABLE command_runs (
    correlation_id UUID PRIMARY KEY,
    context_name VARCHAR(255) NOT NULL REFERENCES contexts(name) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL,
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- pending, in_progress, completed, failed
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_command_runs_context_action ON command_runs (context_name, action);
CREATE INDEX idx_command_runs_status ON command_runs (status);
CREATE INDEX idx_command_runs_requested_at ON command_runs (requested_at DESC);
```

---

## Testing Requirements

### Unit Tests

**File: `internal/events/publisher_test.go`**
- Test message envelope creation
- Test command publishing with different actions
- Test error handling for RabbitMQ connection issues

**File: `internal/events/consumer_test.go`**  
- Test message consumption and handler registration
- Test error handling and message requeuing
- Test graceful shutdown

### Integration Tests

**File: `cmd/gateway/actions_integration_test.go`**
- Test complete command publishing flow
- Test action endpoints return proper responses
- Test correlation ID tracking
- Test error scenarios (context not found, RabbitMQ down)

**File: `internal/events/rabbitmq_integration_test.go`**
- Test RabbitMQ topology creation
- Test message round-trip (publish → consume)
- Test exchange and queue bindings

---

## Dependencies

Add to `go.mod`:
```
require (
    github.com/streadway/amqp v1.1.0
    github.com/google/uuid v1.3.1
)
```

---

## Validation Checklist

Before marking Phase 1B complete:

- [ ] RabbitMQ topology (exchanges, queues) created correctly
- [ ] Action endpoints publish commands to correct routing keys
- [ ] Message envelopes include all required fields
- [ ] Correlation IDs are properly generated and tracked
- [ ] Consumer can receive and process result messages
- [ ] All action endpoints return proper JSON responses
- [ ] Database migrations for run tracking applied successfully
- [ ] Integration tests pass for message flow
- [ ] Error handling works for RabbitMQ connection failures
- [ ] Health check endpoint indicates RabbitMQ connectivity

---

## Message Flow Verification

Test the complete flow:

1. **POST** `/contexts/{name}/actions/refresh`
2. Verify command published to `contextops.commands` exchange
3. Verify routing key is `cmd.context.refresh`
4. Verify message envelope structure is correct
5. Verify correlation ID is returned in response

---

## Next Steps

Upon completion, Phase 1B provides:
- Event-driven command publishing system
- RabbitMQ topology ready for integration services
- Action endpoints for triggering workflows
- Foundation for result aggregation

**Handoff to Phase 1C:** Integration services can now consume commands from RabbitMQ and publish results. The message envelope structure is established and correlation tracking is in place.