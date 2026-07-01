# PHASE 1H: Basic CLI

**Duration:** 2-3 days  
**Prerequisites:** Phase 1G completed  
**Deliverable:** Command-line interface for essential Platformctl operations

---

## Overview

Implement a command-line interface that provides essential operations for managing contexts and triggering actions. The CLI should offer both interactive and scriptable interfaces suitable for developer workflows and automation.

## Success Criteria

✅ CLI binary with proper command structure  
✅ Context CRUD operations via CLI  
✅ Action triggering (refresh, validate, inspect)  
✅ Status querying with formatted output  
✅ Configuration file support  
✅ Output formatting options (JSON, YAML, table)  
✅ Error handling and user-friendly messages  
✅ Shell completion support  

---

## Implementation Tasks

### Task 1: CLI Framework Setup

**File: `cmd/cli/main.go`**

```go
package main

import (
    "os"
    
    "github.com/spf13/cobra"
    "platformctl/internal/cli"
)

var version = "dev" // Set by build process

func main() {
    rootCmd := cli.NewRootCommand(version)
    
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

**File: `internal/cli/root.go`**

```go
package cli

import (
    "fmt"
    "os"
    
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

type GlobalFlags struct {
    ConfigFile string
    Output     string // json, yaml, table
    Verbose    bool
    ServerURL  string
}

func NewRootCommand(version string) *cobra.Command {
    var globalFlags GlobalFlags
    
    rootCmd := &cobra.Command{
        Use:   "platformctl",
        Short: "Platformctl CLI - Manage application contexts and infrastructure",
        Long: `Platformctl CLI provides commands to manage application contexts,
trigger infrastructure operations, and query system status.`,
        Version: version,
        PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
            return initConfig(&globalFlags)
        },
    }
    
    // Global flags
    rootCmd.PersistentFlags().StringVarP(&globalFlags.ConfigFile, "config", "c", "", "config file (default is $HOME/.platformctl.yaml)")
    rootCmd.PersistentFlags().StringVarP(&globalFlags.Output, "output", "o", "table", "output format (json|yaml|table)")
    rootCmd.PersistentFlags().BoolVarP(&globalFlags.Verbose, "verbose", "v", false, "verbose output")
    rootCmd.PersistentFlags().StringVar(&globalFlags.ServerURL, "server", "", "Platformctl server URL")
    
    // Add subcommands
    rootCmd.AddCommand(
        newContextCommand(&globalFlags),
        newStatusCommand(&globalFlags),
        newActionCommand(&globalFlags),
        newCompletionCommand(),
        newVersionCommand(version),
    )
    
    return rootCmd
}

func initConfig(flags *GlobalFlags) error {
    if flags.ConfigFile != "" {
        viper.SetConfigFile(flags.ConfigFile)
    } else {
        home, err := os.UserHomeDir()
        if err != nil {
            return fmt.Errorf("could not find home directory: %w", err)
        }
        
        viper.AddConfigPath(home)
        viper.AddConfigPath(".")
        viper.SetConfigType("yaml")
        viper.SetConfigName(".platformctl")
    }
    
    // Environment variable support
    viper.SetEnvPrefix("PLATFORMCTL")
    viper.AutomaticEnv()
    
    // Set defaults
    viper.SetDefault("server_url", "http://localhost:8080")
    viper.SetDefault("output", "table")
    
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return fmt.Errorf("error reading config file: %w", err)
        }
    }
    
    // Override with command line flags
    if flags.ServerURL != "" {
        viper.Set("server_url", flags.ServerURL)
    }
    if flags.Output != "table" {
        viper.Set("output", flags.Output)
    }
    
    return nil
}
```

### Task 2: Context Management Commands

**File: `internal/cli/context.go`**

```go
package cli

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

func newContextCommand(globalFlags *GlobalFlags) *cobra.Command {
    contextCmd := &cobra.Command{
        Use:   "context",
        Short: "Manage contexts",
        Long:  "Create, read, update, and delete Platformctl contexts",
    }
    
    contextCmd.AddCommand(
        newContextCreateCommand(globalFlags),
        newContextGetCommand(globalFlags),
        newContextListCommand(globalFlags),
        newContextUpdateCommand(globalFlags),
        newContextDeleteCommand(globalFlags),
    )
    
    return contextCmd
}

func newContextCreateCommand(globalFlags *GlobalFlags) *cobra.Command {
    var file string
    
    cmd := &cobra.Command{
        Use:   "create [NAME]",
        Short: "Create a new context",
        Long:  "Create a new context from a YAML or JSON file",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            var contextData []byte
            var err error
            
            if file == "-" || file == "" {
                // Read from stdin
                contextData, err = io.ReadAll(os.Stdin)
            } else {
                // Read from file
                contextData, err = os.ReadFile(file)
            }
            
            if err != nil {
                return fmt.Errorf("failed to read context data: %w", err)
            }
            
            // Parse to determine if it's YAML or JSON
            var context interface{}
            if err := yaml.Unmarshal(contextData, &context); err != nil {
                return fmt.Errorf("failed to parse context data: %w", err)
            }
            
            // Convert to JSON for API
            jsonData, err := json.Marshal(context)
            if err != nil {
                return fmt.Errorf("failed to convert to JSON: %w", err)
            }
            
            result, err := client.CreateContext(jsonData)
            if err != nil {
                return fmt.Errorf("failed to create context: %w", err)
            }
            
            return outputResult(result, globalFlags.Output)
        },
    }
    
    cmd.Flags().StringVarP(&file, "file", "f", "", "context file (YAML or JSON), use '-' for stdin")
    
    return cmd
}

func newContextGetCommand(globalFlags *GlobalFlags) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "get NAME",
        Short: "Get a context by name",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            result, err := client.GetContext(args[0])
            if err != nil {
                return fmt.Errorf("failed to get context: %w", err)
            }
            
            return outputResult(result, globalFlags.Output)
        },
    }
    
    return cmd
}

func newContextListCommand(globalFlags *GlobalFlags) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "list",
        Short: "List all contexts",
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            result, err := client.ListContexts()
            if err != nil {
                return fmt.Errorf("failed to list contexts: %w", err)
            }
            
            if globalFlags.Output == "table" {
                return outputContextTable(result)
            }
            
            return outputResult(result, globalFlags.Output)
        },
    }
    
    return cmd
}

func newContextUpdateCommand(globalFlags *GlobalFlags) *cobra.Command {
    var file string
    
    cmd := &cobra.Command{
        Use:   "update NAME",
        Short: "Update an existing context",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            var contextData []byte
            var err error
            
            if file == "-" {
                contextData, err = io.ReadAll(os.Stdin)
            } else {
                contextData, err = os.ReadFile(file)
            }
            
            if err != nil {
                return fmt.Errorf("failed to read context data: %w", err)
            }
            
            var context interface{}
            if err := yaml.Unmarshal(contextData, &context); err != nil {
                return fmt.Errorf("failed to parse context data: %w", err)
            }
            
            jsonData, err := json.Marshal(context)
            if err != nil {
                return fmt.Errorf("failed to convert to JSON: %w", err)
            }
            
            result, err := client.UpdateContext(args[0], jsonData)
            if err != nil {
                return fmt.Errorf("failed to update context: %w", err)
            }
            
            return outputResult(result, globalFlags.Output)
        },
    }
    
    cmd.Flags().StringVarP(&file, "file", "f", "", "context file (YAML or JSON), use '-' for stdin")
    cmd.MarkFlagRequired("file")
    
    return cmd
}

func newContextDeleteCommand(globalFlags *GlobalFlags) *cobra.Command {
    var force bool
    
    cmd := &cobra.Command{
        Use:   "delete NAME",
        Short: "Delete a context",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if !force {
                fmt.Printf("Are you sure you want to delete context '%s'? (y/N): ", args[0])
                var response string
                fmt.Scanln(&response)
                if response != "y" && response != "Y" {
                    fmt.Println("Delete cancelled")
                    return nil
                }
            }
            
            client := NewAPIClient(globalFlags)
            
            err := client.DeleteContext(args[0])
            if err != nil {
                return fmt.Errorf("failed to delete context: %w", err)
            }
            
            fmt.Printf("Context '%s' deleted successfully\n", args[0])
            return nil
        },
    }
    
    cmd.Flags().BoolVarP(&force, "force", "", false, "skip confirmation prompt")
    
    return cmd
}
```

### Task 3: Action Commands

**File: `internal/cli/actions.go`**

```go
package cli

import (
    "fmt"
    "time"
    
    "github.com/spf13/cobra"
)

func newActionCommand(globalFlags *GlobalFlags) *cobra.Command {
    actionCmd := &cobra.Command{
        Use:   "action",
        Short: "Trigger context actions",
        Long:  "Execute actions like refresh, validate, and inspect on contexts",
    }
    
    actionCmd.AddCommand(
        newRefreshCommand(globalFlags),
        newValidateCommand(globalFlags),
        newInspectCommand(globalFlags),
    )
    
    return actionCmd
}

func newRefreshCommand(globalFlags *GlobalFlags) *cobra.Command {
    var wait bool
    var timeout time.Duration
    
    cmd := &cobra.Command{
        Use:   "refresh CONTEXT_NAME",
        Short: "Refresh context data from all integration services",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            result, err := client.TriggerAction(args[0], "refresh")
            if err != nil {
                return fmt.Errorf("failed to trigger refresh: %w", err)
            }
            
            correlationID := result["correlation_id"].(string)
            fmt.Printf("Refresh triggered for context '%s' (correlation ID: %s)\n", args[0], correlationID)
            
            if wait {
                fmt.Printf("Waiting for refresh to complete (timeout: %s)...\n", timeout)
                return waitForCompletion(client, args[0], correlationID, timeout)
            }
            
            return nil
        },
    }
    
    cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait for action to complete")
    cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "timeout for waiting")
    
    return cmd
}

func newValidateCommand(globalFlags *GlobalFlags) *cobra.Command {
    var wait bool
    var timeout time.Duration
    
    cmd := &cobra.Command{
        Use:   "validate CONTEXT_NAME",
        Short: "Validate context configuration",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            result, err := client.TriggerAction(args[0], "validate")
            if err != nil {
                return fmt.Errorf("failed to trigger validation: %w", err)
            }
            
            correlationID := result["correlation_id"].(string)
            fmt.Printf("Validation triggered for context '%s' (correlation ID: %s)\n", args[0], correlationID)
            
            if wait {
                fmt.Printf("Waiting for validation to complete (timeout: %s)...\n", timeout)
                return waitForCompletion(client, args[0], correlationID, timeout)
            }
            
            return nil
        },
    }
    
    cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait for action to complete")
    cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "timeout for waiting")
    
    return cmd
}

func newInspectCommand(globalFlags *GlobalFlags) *cobra.Command {
    var wait bool
    var timeout time.Duration
    
    cmd := &cobra.Command{
        Use:   "inspect CONTEXT_NAME",
        Short: "Inspect context infrastructure state",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            result, err := client.TriggerAction(args[0], "inspect")
            if err != nil {
                return fmt.Errorf("failed to trigger inspection: %w", err)
            }
            
            correlationID := result["correlation_id"].(string)
            fmt.Printf("Inspection triggered for context '%s' (correlation ID: %s)\n", args[0], correlationID)
            
            if wait {
                fmt.Printf("Waiting for inspection to complete (timeout: %s)...\n", timeout)
                return waitForCompletion(client, args[0], correlationID, timeout)
            }
            
            return nil
        },
    }
    
    cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait for action to complete")
    cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "timeout for waiting")
    
    return cmd
}

func waitForCompletion(client *APIClient, contextName, correlationID string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    
    for time.Now().Before(deadline) {
        // Check run history for completion
        history, err := client.GetRunHistory(contextName, 10)
        if err != nil {
            return fmt.Errorf("failed to check completion status: %w", err)
        }
        
        runs := history["runs"].([]interface{})
        for _, runInterface := range runs {
            run := runInterface.(map[string]interface{})
            if run["correlation_id"].(string) == correlationID {
                status := run["overall_status"].(string)
                
                switch status {
                case "ok":
                    fmt.Println("✅ Action completed successfully")
                    return nil
                case "error":
                    fmt.Println("❌ Action completed with errors")
                    return fmt.Errorf("action failed")
                case "degraded":
                    fmt.Println("⚠️  Action completed with warnings")
                    return nil
                }
            }
        }
        
        time.Sleep(2 * time.Second)
    }
    
    return fmt.Errorf("action did not complete within timeout")
}
```

### Task 4: Status Commands

**File: `internal/cli/status.go`**

```go
package cli

import (
    "fmt"
    "strings"
    "time"
    
    "github.com/spf13/cobra"
)

func newStatusCommand(globalFlags *GlobalFlags) *cobra.Command {
    statusCmd := &cobra.Command{
        Use:   "status",
        Short: "Query context status and history",
    }
    
    statusCmd.AddCommand(
        newStatusShowCommand(globalFlags),
        newStatusHistoryCommand(globalFlags),
    )
    
    return statusCmd
}

func newStatusShowCommand(globalFlags *GlobalFlags) *cobra.Command {
    var watch bool
    var watchInterval time.Duration
    
    cmd := &cobra.Command{
        Use:   "show CONTEXT_NAME",
        Short: "Show current status of a context",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            if watch {
                return watchStatus(client, args[0], watchInterval, globalFlags)
            }
            
            return showStatus(client, args[0], globalFlags)
        },
    }
    
    cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch status updates")
    cmd.Flags().DurationVar(&watchInterval, "interval", 5*time.Second, "watch interval")
    
    return cmd
}

func newStatusHistoryCommand(globalFlags *GlobalFlags) *cobra.Command {
    var limit int
    
    cmd := &cobra.Command{
        Use:   "history CONTEXT_NAME",
        Short: "Show run history for a context",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client := NewAPIClient(globalFlags)
            
            result, err := client.GetRunHistory(args[0], limit)
            if err != nil {
                return fmt.Errorf("failed to get run history: %w", err)
            }
            
            if globalFlags.Output == "table" {
                return outputHistoryTable(result)
            }
            
            return outputResult(result, globalFlags.Output)
        },
    }
    
    cmd.Flags().IntVarP(&limit, "limit", "l", 20, "maximum number of runs to show")
    
    return cmd
}

func showStatus(client *APIClient, contextName string, flags *GlobalFlags) error {
    result, err := client.GetContextStatus(contextName)
    if err != nil {
        return fmt.Errorf("failed to get context status: %w", err)
    }
    
    if flags.Output == "table" {
        return outputStatusTable(result)
    }
    
    return outputResult(result, flags.Output)
}

func watchStatus(client *APIClient, contextName string, interval time.Duration, flags *GlobalFlags) error {
    fmt.Printf("Watching status for context '%s' (interval: %s)\n", contextName, interval)
    fmt.Println("Press Ctrl+C to stop")
    fmt.Println()
    
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    // Show initial status
    if err := showStatus(client, contextName, flags); err != nil {
        return err
    }
    
    for range ticker.C {
        // Clear screen and show updated status
        fmt.Print("\033[H\033[2J") // ANSI clear screen
        fmt.Printf("Last updated: %s\n\n", time.Now().Format("15:04:05"))
        
        if err := showStatus(client, contextName, flags); err != nil {
            fmt.Printf("Error updating status: %v\n", err)
        }
    }
    
    return nil
}
```

### Task 5: API Client

**File: `internal/cli/client.go`**

```go
package cli

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
    
    "github.com/spf13/viper"
)

type APIClient struct {
    BaseURL    string
    HTTPClient *http.Client
    Verbose    bool
}

func NewAPIClient(flags *GlobalFlags) *APIClient {
    baseURL := viper.GetString("server_url")
    if flags.ServerURL != "" {
        baseURL = flags.ServerURL
    }
    
    return &APIClient{
        BaseURL: baseURL,
        HTTPClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        Verbose: flags.Verbose,
    }
}

func (c *APIClient) CreateContext(contextData []byte) (map[string]interface{}, error) {
    return c.makeRequest("POST", "/contexts", contextData)
}

func (c *APIClient) GetContext(name string) (map[string]interface{}, error) {
    return c.makeRequest("GET", fmt.Sprintf("/contexts/%s", name), nil)
}

func (c *APIClient) ListContexts() ([]interface{}, error) {
    result, err := c.makeRequest("GET", "/contexts", nil)
    if err != nil {
        return nil, err
    }
    
    // Convert to slice
    if contexts, ok := result["contexts"].([]interface{}); ok {
        return contexts, nil
    }
    
    return []interface{}{}, nil
}

func (c *APIClient) UpdateContext(name string, contextData []byte) (map[string]interface{}, error) {
    return c.makeRequest("PUT", fmt.Sprintf("/contexts/%s", name), contextData)
}

func (c *APIClient) DeleteContext(name string) error {
    _, err := c.makeRequest("DELETE", fmt.Sprintf("/contexts/%s", name), nil)
    return err
}

func (c *APIClient) TriggerAction(contextName, action string) (map[string]interface{}, error) {
    return c.makeRequest("POST", fmt.Sprintf("/contexts/%s/actions/%s", contextName, action), nil)
}

func (c *APIClient) GetContextStatus(name string) (map[string]interface{}, error) {
    return c.makeRequest("GET", fmt.Sprintf("/contexts/%s/status", name), nil)
}

func (c *APIClient) GetRunHistory(name string, limit int) (map[string]interface{}, error) {
    url := fmt.Sprintf("/contexts/%s/runs?limit=%d", name, limit)
    return c.makeRequest("GET", url, nil)
}

func (c *APIClient) makeRequest(method, path string, body []byte) (map[string]interface{}, error) {
    url := c.BaseURL + path
    
    var reqBody io.Reader
    if body != nil {
        reqBody = bytes.NewReader(body)
    }
    
    req, err := http.NewRequest(method, url, reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    
    if c.Verbose {
        fmt.Printf("→ %s %s\n", method, url)
    }
    
    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()
    
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }
    
    if c.Verbose {
        fmt.Printf("← %d %s\n", resp.StatusCode, string(respBody))
    }
    
    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
    }
    
    if len(respBody) == 0 {
        return map[string]interface{}{}, nil
    }
    
    var result map[string]interface{}
    if err := json.Unmarshal(respBody, &result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }
    
    return result, nil
}
```

### Task 6: Output Formatting

**File: `internal/cli/output.go`**

```go
package cli

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "text/tabwriter"
    "time"
    
    "gopkg.in/yaml.v3"
)

func outputResult(data interface{}, format string) error {
    switch format {
    case "json":
        return outputJSON(data)
    case "yaml":
        return outputYAML(data)
    case "table":
        return outputJSON(data) // Default to JSON for generic data
    default:
        return fmt.Errorf("unsupported output format: %s", format)
    }
}

func outputJSON(data interface{}) error {
    encoder := json.NewEncoder(os.Stdout)
    encoder.SetIndent("", "  ")
    return encoder.Encode(data)
}

func outputYAML(data interface{}) error {
    encoder := yaml.NewEncoder(os.Stdout)
    defer encoder.Close()
    return encoder.Encode(data)
}

func outputContextTable(contexts []interface{}) error {
    if len(contexts) == 0 {
        fmt.Println("No contexts found")
        return nil
    }
    
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "NAME\tAPP\tENVIRONMENT\tCREATED")
    fmt.Fprintln(w, "----\t---\t-----------\t-------")
    
    for _, ctxInterface := range contexts {
        ctx := ctxInterface.(map[string]interface{})
        
        name := ctx["metadata"].(map[string]interface{})["name"].(string)
        spec := ctx["spec"].(map[string]interface{})
        app := spec["app"].(map[string]interface{})
        
        appName := app["name"].(string)
        environment := app["environment"].(string)
        
        // Parse created timestamp if available
        created := "unknown"
        if createdAt, ok := ctx["created_at"].(string); ok {
            if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
                created = t.Format("2006-01-02 15:04")
            }
        }
        
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, appName, environment, created)
    }
    
    return w.Flush()
}

func outputStatusTable(status map[string]interface{}) error {
    contextName := status["context_name"].(string)
    overallHealth := status["overall_health"].(string)
    updatedAt := status["updated_at"].(string)
    stalenessSeconds := int(status["staleness_seconds"].(float64))
    
    // Parse timestamp
    updatedTime, _ := time.Parse(time.RFC3339, updatedAt)
    
    fmt.Printf("Context: %s\n", contextName)
    fmt.Printf("Overall Health: %s\n", formatHealthStatus(overallHealth))
    fmt.Printf("Last Updated: %s (%s ago)\n", 
        updatedTime.Format("2006-01-02 15:04:05"),
        formatDuration(time.Duration(stalenessSeconds)*time.Second))
    fmt.Println()
    
    // Service status table
    summary := status["summary"].(map[string]interface{})
    
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "SERVICE\tSTATUS\tLAST UPDATE")
    fmt.Fprintln(w, "-------\t------\t-----------")
    
    services := []string{"vault", "argocd", "newrelic", "kubernetes", "git"}
    for _, service := range services {
        if serviceStatus, ok := summary[service].(string); ok {
            lastUpdate := "unknown"
            
            // Try to get last update time from details
            if details, ok := status["details"].(map[string]interface{}); ok {
                if serviceData, ok := details[service].(map[string]interface{}); ok {
                    if updateTime, ok := serviceData["updated_at"].(string); ok {
                        if t, err := time.Parse(time.RFC3339, updateTime); err == nil {
                            lastUpdate = formatDuration(time.Since(t)) + " ago"
                        }
                    }
                }
            }
            
            fmt.Fprintf(w, "%s\t%s\t%s\n", 
                service, 
                formatHealthStatus(serviceStatus),
                lastUpdate)
        }
    }
    
    return w.Flush()
}

func outputHistoryTable(history map[string]interface{}) error {
    runs := history["runs"].([]interface{})
    
    if len(runs) == 0 {
        fmt.Println("No run history found")
        return nil
    }
    
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "CORRELATION ID\tACTION\tSTATUS\tDURATION\tREQUESTED AT")
    fmt.Fprintln(w, "--------------\t------\t------\t--------\t------------")
    
    for _, runInterface := range runs {
        run := runInterface.(map[string]interface{})
        
        correlationID := run["correlation_id"].(string)[:8] // Show first 8 chars
        action := run["action"].(string)
        status := run["overall_status"].(string)
        latencyMs := int(run["latency_ms"].(float64))
        requestedAt := run["requested_at"].(string)
        
        // Parse timestamp
        reqTime, _ := time.Parse(time.RFC3339, requestedAt)
        
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
            correlationID,
            action,
            formatHealthStatus(status),
            fmt.Sprintf("%dms", latencyMs),
            reqTime.Format("2006-01-02 15:04"))
    }
    
    return w.Flush()
}

func formatHealthStatus(status string) string {
    switch status {
    case "ok":
        return "✅ OK"
    case "degraded":
        return "⚠️  DEGRADED"
    case "error":
        return "❌ ERROR"
    case "unknown":
        return "❓ UNKNOWN"
    default:
        return strings.ToUpper(status)
    }
}

func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return fmt.Sprintf("%ds", int(d.Seconds()))
    } else if d < time.Hour {
        return fmt.Sprintf("%dm", int(d.Minutes()))
    } else if d < 24*time.Hour {
        return fmt.Sprintf("%dh", int(d.Hours()))
    } else {
        return fmt.Sprintf("%dd", int(d.Hours()/24))
    }
}
```

### Task 7: Shell Completion and Version Commands

**File: `internal/cli/completion.go`**

```go
package cli

import (
    "os"
    
    "github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "completion [bash|zsh|fish|powershell]",
        Short: "Generate completion script",
        Long: `To load completions:

Bash:
  $ source <(platformctl completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ platformctl completion bash > /etc/bash_completion.d/platformctl
  # macOS:
  $ platformctl completion bash > /usr/local/etc/bash_completion.d/platformctl

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ platformctl completion zsh > "${fpath[1]}/_platformctl"

  # You will need to start a new shell for this setup to take effect.

fish:
  $ platformctl completion fish | source

  # To load completions for each session, execute once:
  $ platformctl completion fish > ~/.config/fish/completions/platformctl.fish

PowerShell:
  PS> platformctl completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> platformctl completion powershell > platformctl.ps1
  # and source this file from your PowerShell profile.
`,
        DisableFlagsInUseLine: true,
        ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
        Args:                  cobra.ExactValidArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            switch args[0] {
            case "bash":
                cmd.Root().GenBashCompletion(os.Stdout)
            case "zsh":
                cmd.Root().GenZshCompletion(os.Stdout)
            case "fish":
                cmd.Root().GenFishCompletion(os.Stdout, true)
            case "powershell":
                cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
            }
        },
    }
    
    return cmd
}

func newVersionCommand(version string) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "version",
        Short: "Print the version number",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Printf("platformctl version %s\n", version)
        },
    }
    
    return cmd
}
```

### Task 8: Configuration File Support

**File: `internal/cli/config.go`**

```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "gopkg.in/yaml.v3"
)

type Config struct {
    ServerURL string `yaml:"server_url"`
    Output    string `yaml:"output"`
    Profiles  map[string]Profile `yaml:"profiles,omitempty"`
}

type Profile struct {
    ServerURL string `yaml:"server_url"`
    Output    string `yaml:"output,omitempty"`
}

func newConfigCommand(globalFlags *GlobalFlags) *cobra.Command {
    configCmd := &cobra.Command{
        Use:   "config",
        Short: "Manage CLI configuration",
    }
    
    configCmd.AddCommand(
        newConfigInitCommand(),
        newConfigSetCommand(),
        newConfigGetCommand(),
        newConfigViewCommand(),
    )
    
    return configCmd
}

func newConfigInitCommand() *cobra.Command {
    var force bool
    
    cmd := &cobra.Command{
        Use:   "init",
        Short: "Initialize configuration file",
        RunE: func(cmd *cobra.Command, args []string) error {
            home, err := os.UserHomeDir()
            if err != nil {
                return fmt.Errorf("could not find home directory: %w", err)
            }
            
            configFile := filepath.Join(home, ".platformctl.yaml")
            
            if _, err := os.Stat(configFile); err == nil && !force {
                return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configFile)
            }
            
            config := Config{
                ServerURL: "http://localhost:8080",
                Output:    "table",
                Profiles: map[string]Profile{
                    "development": {
                        ServerURL: "http://localhost:8080",
                        Output:    "table",
                    },
                    "production": {
                        ServerURL: "https://platformctl.example.com",
                        Output:    "json",
                    },
                },
            }
            
            data, err := yaml.Marshal(config)
            if err != nil {
                return fmt.Errorf("failed to marshal config: %w", err)
            }
            
            if err := os.WriteFile(configFile, data, 0600); err != nil {
                return fmt.Errorf("failed to write config file: %w", err)
            }
            
            fmt.Printf("Configuration file created at %s\n", configFile)
            return nil
        },
    }
    
    cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")
    
    return cmd
}

func newConfigSetCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "set KEY VALUE",
        Short: "Set a configuration value",
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            key := args[0]
            value := args[1]
            
            viper.Set(key, value)
            
            if err := viper.WriteConfig(); err != nil {
                return fmt.Errorf("failed to write config: %w", err)
            }
            
            fmt.Printf("Set %s = %s\n", key, value)
            return nil
        },
    }
    
    return cmd
}

func newConfigGetCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "get [KEY]",
        Short: "Get configuration value(s)",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if len(args) == 0 {
                // Show all config
                return newConfigViewCommand().RunE(cmd, args)
            }
            
            key := args[0]
            value := viper.Get(key)
            
            if value == nil {
                fmt.Printf("%s is not set\n", key)
            } else {
                fmt.Printf("%s = %v\n", key, value)
            }
            
            return nil
        },
    }
    
    return cmd
}

func newConfigViewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "view",
        Short: "View current configuration",
        RunE: func(cmd *cobra.Command, args []string) error {
            config := viper.AllSettings()
            
            data, err := yaml.Marshal(config)
            if err != nil {
                return fmt.Errorf("failed to marshal config: %w", err)
            }
            
            fmt.Print(string(data))
            return nil
        },
    }
    
    return cmd
}
```

---

## Dependencies

Add to `go.mod`:
```
require (
    github.com/spf13/cobra v1.7.0
    github.com/spf13/viper v1.16.0
    gopkg.in/yaml.v3 v3.0.1
)
```

---

## Validation Checklist

Before marking Phase 1H complete:

**Core Functionality:**
- [ ] CLI binary builds and runs without errors
- [ ] Context CRUD operations work via CLI
- [ ] Action commands trigger operations successfully  
- [ ] Status commands display current state
- [ ] Run history commands show execution logs

**User Experience:**
- [ ] Command structure is intuitive and follows conventions
- [ ] Help text is comprehensive and accurate
- [ ] Error messages are user-friendly and actionable
- [ ] Output formats (JSON, YAML, table) work correctly
- [ ] Shell completion functions properly

**Configuration:**
- [ ] Configuration file support working
- [ ] Environment variable override working
- [ ] Multiple output format support
- [ ] Server URL configuration working
- [ ] Verbose mode provides useful debug information

**Integration:**
- [ ] API client handles all HTTP methods correctly
- [ ] Error handling for network failures
- [ ] Timeout handling for long operations
- [ ] Authentication integration (if implemented)

**Examples and Documentation:**
- [ ] CLI help examples are correct
- [ ] Common workflows documented
- [ ] Installation instructions clear
- [ ] Shell completion setup instructions accurate

---

## Next Steps

Upon completion, Phase 1H provides:
- Complete command-line interface for Platformctl
- User-friendly developer experience
- Scriptable automation capabilities
- Foundation for operator workflows

**MVP Complete:** With Phase 1H, Platformctl has a fully functional MVP including:
- Core context management
- Event-driven integration services  
- Aggregated status views
- Comprehensive observability
- Kubernetes deployment
- Complete test coverage
- Command-line interface

The system is now ready for initial production use and can serve as the foundation for Phase 2 security enhancements and Phase 3 performance optimizations.