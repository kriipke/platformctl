package cli

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/pkg/api"
)

// apiPrefix is the versioned base path the gateway serves all resources under.
const apiPrefix = "/api/v1"

// Client is a thin, typed HTTP client for the Platformctl API Gateway.
type Client struct {
	baseURL    string
	http       *http.Client
	username   string
	password   string
	customerID string
	token      string
	verbose    bool
	logw       io.Writer // where verbose request/response lines go (stderr)
}

// NewClient builds a Client from the resolved global flags.
func NewClient(f *GlobalFlags, logw io.Writer) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if f.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- opt-in via --insecure
	}
	return &Client{
		baseURL:    f.Server,
		http:       &http.Client{Timeout: 30 * time.Second, Transport: transport},
		username:   f.Username,
		password:   f.Password,
		customerID: f.CustomerID,
		token:      f.Token,
		verbose:    f.Verbose,
		logw:       logw,
	}
}

// APIError represents a non-2xx response from the gateway.
type APIError struct {
	StatusCode int
	Message    string
	Details    []api.ValidationError
	Raw        string
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "server returned %d", e.StatusCode)
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	}
	for _, d := range e.Details {
		fmt.Fprintf(&b, "\n  - %s: %s", d.Field, d.Message)
	}
	return b.String()
}

// --- Context CRUD --------------------------------------------------------

func (c *Client) CreateContext(ctx models.Context) (*api.CreateContextResponse, error) {
	var out api.CreateContextResponse
	err := c.do(http.MethodPost, "/contexts", api.CreateContextRequest{Context: ctx}, &out)
	return &out, err
}

func (c *Client) GetContext(name string) (*api.GetContextResponse, error) {
	var out api.GetContextResponse
	err := c.do(http.MethodGet, "/contexts/"+url.PathEscape(name), nil, &out)
	return &out, err
}

func (c *Client) ListContexts() (*api.ListContextsResponse, error) {
	var out api.ListContextsResponse
	err := c.do(http.MethodGet, "/contexts", nil, &out)
	return &out, err
}

func (c *Client) UpdateContext(name string, ctx models.Context) (*api.UpdateContextResponse, error) {
	var out api.UpdateContextResponse
	err := c.do(http.MethodPut, "/contexts/"+url.PathEscape(name), api.UpdateContextRequest{Context: ctx}, &out)
	return &out, err
}

func (c *Client) DeleteContext(name string) (*api.DeleteResponse, error) {
	var out api.DeleteResponse
	err := c.do(http.MethodDelete, "/contexts/"+url.PathEscape(name), nil, &out)
	return &out, err
}

// --- GitOps actions & status --------------------------------------------

// RunAction triggers a GitOps action on a context. queryType, when non-empty,
// is passed as the `type` query parameter (used by inspect-manifests). The
// response shape varies per action, so it is returned as a generic map.
func (c *Client) RunAction(name, action, queryType string) (map[string]interface{}, error) {
	path := "/contexts/" + url.PathEscape(name) + "/actions/" + url.PathEscape(action)
	if queryType != "" {
		path += "?type=" + url.QueryEscape(queryType)
	}
	var out map[string]interface{}
	err := c.do(http.MethodPost, path, nil, &out)
	return out, err
}

// GetContextStatus returns the aggregated read-model status for a context. The
// payload is dynamic, so it is decoded into a generic map.
func (c *Client) GetContextStatus(name string) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.do(http.MethodGet, "/gitops/contexts/"+url.PathEscape(name)+"/status", nil, &out)
	return out, err
}

// --- transport -----------------------------------------------------------

// do performs a request against apiPrefix+path, marshalling body (if non-nil)
// as JSON and decoding a successful response into out (if non-nil).
func (c *Client) do(method, path string, body, out interface{}) error {
	fullURL := c.baseURL + apiPrefix + path

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, fullURL, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyAuth(req)

	if c.verbose {
		fmt.Fprintf(c.logw, "→ %s %s\n", method, fullURL)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Platformctl server at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if c.verbose {
		fmt.Fprintf(c.logw, "← %d %s\n", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return parseAPIError(resp.StatusCode, respBody)
	}

	if out == nil || len(bytes.TrimSpace(respBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// applyAuth sets the auth + tenant headers. A bearer token takes precedence
// over basic auth. The X-Customer-ID / X-User-ID tenant headers are sent when a
// customer id is configured.
func (c *Client) applyAuth(req *http.Request) {
	switch {
	case c.token != "":
		req.Header.Set("Authorization", "Bearer "+c.token)
	case c.username != "":
		req.SetBasicAuth(c.username, c.password)
	}
	if c.customerID != "" {
		req.Header.Set("X-Customer-ID", c.customerID)
		if c.username != "" {
			req.Header.Set("X-User-ID", c.username)
		}
	}
}

// parseAPIError turns an error response body into an *APIError, best-effort
// decoding the gateway's ErrorResponse / ValidationErrorResponse shapes.
func parseAPIError(status int, body []byte) error {
	apiErr := &APIError{StatusCode: status, Raw: strings.TrimSpace(string(body))}

	// Validation errors carry a details array.
	var verr api.ValidationErrorResponse
	if err := json.Unmarshal(body, &verr); err == nil && len(verr.Details) > 0 {
		apiErr.Message = verr.Error
		apiErr.Details = verr.Details
		return apiErr
	}

	// Generic {"error": "..."} shape (used by both the wrapped handlers and the
	// gin gitops handlers).
	var generic struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &generic); err == nil && generic.Error != "" {
		apiErr.Message = generic.Error
		return apiErr
	}

	// Fall back to the raw body (e.g. http.Error plain-text responses).
	apiErr.Message = apiErr.Raw
	return apiErr
}
