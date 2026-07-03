package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kriipke/platformctl/internal/models"
	"github.com/kriipke/platformctl/pkg/api"
)

func testClient(baseURL string) *Client {
	return &Client{
		baseURL:  baseURL,
		http:     &http.Client{},
		username: "admin",
		password: "admin",
		logw:     io.Discard,
	}
}

func sampleContext(name string) models.Context {
	return models.Context{
		APIVersion: "platformctl/v1",
		Kind:       "Context",
		Metadata:   models.ContextMetadata{Name: name},
		Spec: models.ContextSpec{
			AppRef: "web-app",
			Deployments: []models.ContextDeployment{
				{Environment: "prod", AppRef: "web-app", EnvironmentRef: "web-app-prod-env", Active: true},
			},
		},
	}
}

func TestCreateContext(t *testing.T) {
	var gotPath, gotMethod, gotAuthUser string
	var gotReq api.CreateContextRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		u, _, _ := r.BasicAuth()
		gotAuthUser = u
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.CreateContextResponse{
			Success: true, Message: "Context created successfully", ContextName: gotReq.Context.Metadata.Name,
		})
	}))
	defer srv.Close()

	resp, err := testClient(srv.URL).CreateContext(sampleContext("web-app-prod"))
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/contexts" {
		t.Errorf("path = %s, want /api/v1/contexts", gotPath)
	}
	if gotAuthUser != "admin" {
		t.Errorf("basic-auth user = %q, want admin", gotAuthUser)
	}
	if gotReq.Context.Metadata.Name != "web-app-prod" {
		t.Errorf("request context name = %q, want web-app-prod", gotReq.Context.Metadata.Name)
	}
	if !resp.Success {
		t.Errorf("resp.Success = false, want true")
	}
}

func TestListContexts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/contexts" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(api.ListContextsResponse{
			Success:  true,
			Contexts: []models.Context{sampleContext("a"), sampleContext("b")},
			Count:    2,
		})
	}))
	defer srv.Close()

	resp, err := testClient(srv.URL).ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if resp.Count != 2 || len(resp.Contexts) != 2 {
		t.Errorf("got count=%d len=%d, want 2/2", resp.Count, len(resp.Contexts))
	}
}

func TestRunAction(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("type")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "correlation_id": "abc-123", "message": "ok",
		})
	}))
	defer srv.Close()

	resp, err := testClient(srv.URL).RunAction("web-app-prod", "inspect-manifests", "app")
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if gotPath != "/api/v1/contexts/web-app-prod/actions/inspect-manifests" {
		t.Errorf("path = %s", gotPath)
	}
	if gotQuery != "app" {
		t.Errorf("type query = %q, want app", gotQuery)
	}
	if resp["correlation_id"] != "abc-123" {
		t.Errorf("correlation_id = %v", resp["correlation_id"])
	}
}

func TestGetContextStatusPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"health_status": "healthy"})
	}))
	defer srv.Close()

	if _, err := testClient(srv.URL).GetContextStatus("web-app-prod"); err != nil {
		t.Fatalf("GetContextStatus: %v", err)
	}
	if gotPath != "/api/v1/gitops/contexts/web-app-prod/status" {
		t.Errorf("path = %s", gotPath)
	}
}

func TestValidationErrorDecoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(api.ValidationErrorResponse{
			Success: false,
			Error:   "Validation failed",
			Details: []api.ValidationError{{Field: "context", Message: "appRef is required"}},
		})
	}))
	defer srv.Close()

	_, err := testClient(srv.URL).CreateContext(sampleContext("x"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", apiErr.StatusCode)
	}
	if len(apiErr.Details) != 1 || apiErr.Details[0].Field != "context" {
		t.Errorf("details = %+v", apiErr.Details)
	}
}

func TestConnectionErrorIsFriendly(t *testing.T) {
	// Point at a closed port to force a dial error.
	c := testClient("http://127.0.0.1:1")
	_, err := c.ListContexts()
	if err == nil {
		t.Fatal("expected connection error")
	}
	if _, ok := err.(*APIError); ok {
		t.Fatalf("connection failure should not be an *APIError: %v", err)
	}
}
