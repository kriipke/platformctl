package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

// HTTPTestContext provides utilities for HTTP handler testing
type HTTPTestContext struct {
	Server     *httptest.Server
	Client     *http.Client
	Router     *mux.Router
	CustomerID string
}

// NewHTTPTestContext creates a new HTTP test context
func NewHTTPTestContext(t *testing.T) *HTTPTestContext {
	t.Helper()

	router := mux.NewRouter()
	server := httptest.NewServer(router)
	client := server.Client()

	return &HTTPTestContext{
		Server:     server,
		Client:     client,
		Router:     router,
		CustomerID: "test-customer-123",
	}
}

// Close closes the HTTP test context
func (htc *HTTPTestContext) Close() {
	htc.Server.Close()
}

// Request makes an HTTP request with optional body and returns response
func (htc *HTTPTestContext) Request(t *testing.T, method, path string, body interface{}) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(jsonBody)
	}

	url := htc.Server.URL + path
	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err)

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add customer context header for authentication
	req.Header.Set("X-Customer-ID", htc.CustomerID)

	resp, err := htc.Client.Do(req)
	require.NoError(t, err)

	return resp
}

// GET makes a GET request
func (htc *HTTPTestContext) GET(t *testing.T, path string) *http.Response {
	t.Helper()
	return htc.Request(t, http.MethodGet, path, nil)
}

// POST makes a POST request with JSON body
func (htc *HTTPTestContext) POST(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	return htc.Request(t, http.MethodPost, path, body)
}

// PUT makes a PUT request with JSON body
func (htc *HTTPTestContext) PUT(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	return htc.Request(t, http.MethodPut, path, body)
}

// DELETE makes a DELETE request
func (htc *HTTPTestContext) DELETE(t *testing.T, path string) *http.Response {
	t.Helper()
	return htc.Request(t, http.MethodDelete, path, nil)
}

// AssertStatusCode asserts that the response has the expected status code
func (htc *HTTPTestContext) AssertStatusCode(t *testing.T, resp *http.Response, expectedCode int) {
	t.Helper()
	if resp.StatusCode != expectedCode {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status code %d, got %d. Response body: %s", expectedCode, resp.StatusCode, string(body))
	}
}

// AssertJSON reads and unmarshals JSON response body into target
func (htc *HTTPTestContext) AssertJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = json.Unmarshal(body, target)
	require.NoError(t, err, "Failed to unmarshal response body: %s", string(body))
}

// ReadResponseBody reads the entire response body as string
func (htc *HTTPTestContext) ReadResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

// MockCustomerAuth is a middleware that adds customer context for testing
func MockCustomerAuth(customerID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create customer context
			ctx := context.WithValue(r.Context(), "customer", struct {
				CustomerID string
			}{
				CustomerID: customerID,
			})
			
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JSONRequest represents a generic JSON request for testing
type JSONRequest map[string]interface{}

// JSONResponse represents a generic JSON response for testing
type JSONResponse map[string]interface{}

// APITestCase represents a test case for API endpoints
type APITestCase struct {
	Name           string
	Method         string
	Path           string
	Body           interface{}
	ExpectedStatus int
	ExpectedBody   interface{}
	SetupFunc      func(t *testing.T, htc *HTTPTestContext)
	AssertFunc     func(t *testing.T, resp *http.Response, body []byte)
}

// RunAPITestCases runs a series of API test cases
func RunAPITestCases(t *testing.T, htc *HTTPTestContext, testCases []APITestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Run setup function if provided
			if tc.SetupFunc != nil {
				tc.SetupFunc(t, htc)
			}

			// Make the request
			resp := htc.Request(t, tc.Method, tc.Path, tc.Body)
			defer resp.Body.Close()

			// Assert status code
			htc.AssertStatusCode(t, resp, tc.ExpectedStatus)

			// Read response body
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Run custom assertion if provided
			if tc.AssertFunc != nil {
				tc.AssertFunc(t, resp, body)
				return
			}

			// Default JSON assertion if expected body is provided
			if tc.ExpectedBody != nil {
				var actualBody interface{}
				err = json.Unmarshal(body, &actualBody)
				require.NoError(t, err)
				
				expectedJSON, err := json.Marshal(tc.ExpectedBody)
				require.NoError(t, err)
				
				var expectedBody interface{}
				err = json.Unmarshal(expectedJSON, &expectedBody)
				require.NoError(t, err)
				
				require.Equal(t, expectedBody, actualBody)
			}
		})
	}
}

// CreateValidationErrorResponse creates a validation error response for testing
func CreateValidationErrorResponse(field, message string) map[string]interface{} {
	return map[string]interface{}{
		"success": false,
		"error":   "Validation failed",
		"details": []map[string]interface{}{
			{
				"field":   field,
				"message": message,
			},
		},
	}
}

// CreateErrorResponse creates a simple error response for testing
func CreateErrorResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"success": false,
		"error":   message,
	}
}

// CreateSuccessResponse creates a simple success response for testing
func CreateSuccessResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"success": true,
		"message": message,
	}
}

// AssertErrorResponse asserts that the response is an error with the expected message
func AssertErrorResponse(t *testing.T, resp *http.Response, expectedMessage string) {
	t.Helper()

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	require.NoError(t, err)

	require.False(t, errorResp["success"].(bool), "Expected error response but got success")
	require.Contains(t, errorResp["error"].(string), expectedMessage)
}

// AssertSuccessResponse asserts that the response is successful with the expected message
func AssertSuccessResponse(t *testing.T, resp *http.Response, expectedMessage string) {
	t.Helper()

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var successResp map[string]interface{}
	err = json.Unmarshal(body, &successResp)
	require.NoError(t, err)

	require.True(t, successResp["success"].(bool), "Expected success response but got error")
	if expectedMessage != "" {
		require.Equal(t, expectedMessage, successResp["message"].(string))
	}
}

// MockRequestWithCustomer creates an HTTP request with customer context
func MockRequestWithCustomer(method, url, customerID string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("X-Customer-ID", customerID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	// Add customer context
	ctx := context.WithValue(req.Context(), "customer", struct {
		CustomerID string
	}{
		CustomerID: customerID,
	})
	
	return req.WithContext(ctx)
}

// CreateMockResponseWriter creates a mock response writer for testing
func CreateMockResponseWriter() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// AssertJSONContains asserts that JSON response contains expected fields
func AssertJSONContains(t *testing.T, resp *http.Response, expectedFields map[string]interface{}) {
	t.Helper()

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var jsonResp map[string]interface{}
	err = json.Unmarshal(body, &jsonResp)
	require.NoError(t, err, "Failed to parse JSON response: %s", string(body))

	for field, expectedValue := range expectedFields {
		actualValue, exists := jsonResp[field]
		require.True(t, exists, "Field '%s' not found in response", field)
		require.Equal(t, expectedValue, actualValue, "Field '%s' has unexpected value", field)
	}
}

// ConcurrentTestRunner runs tests concurrently to check for race conditions
func ConcurrentTestRunner(t *testing.T, testFunc func(t *testing.T), numGoroutines int) {
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("goroutine %d panicked: %v", routineID, r)
				}
				done <- true
			}()

			// Create a sub-test for each goroutine
			t.Run(fmt.Sprintf("concurrent_%d", routineID), testFunc)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Goroutine completed
		case err := <-errors:
			t.Fatalf("Concurrent test failed: %v", err)
		}
	}
}

// PerformanceTimer measures execution time of operations
type PerformanceTimer struct {
	operations map[string][]time.Duration
}

// NewPerformanceTimer creates a new performance timer
func NewPerformanceTimer() *PerformanceTimer {
	return &PerformanceTimer{
		operations: make(map[string][]time.Duration),
	}
}

// Time measures the execution time of an operation
func (pt *PerformanceTimer) Time(operation string, fn func()) {
	start := time.Now()
	fn()
	duration := time.Since(start)
	pt.operations[operation] = append(pt.operations[operation], duration)
}

// Report reports performance statistics
func (pt *PerformanceTimer) Report(t *testing.T) {
	t.Helper()

	for operation, durations := range pt.operations {
		if len(durations) == 0 {
			continue
		}

		var total time.Duration
		min := durations[0]
		max := durations[0]

		for _, d := range durations {
			total += d
			if d < min {
				min = d
			}
			if d > max {
				max = d
			}
		}

		avg := total / time.Duration(len(durations))
		t.Logf("Operation: %s, Count: %d, Avg: %v, Min: %v, Max: %v, Total: %v",
			operation, len(durations), avg, min, max, total)
	}
}