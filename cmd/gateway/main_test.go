package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kriipke/platformctl/internal/auth"
	"github.com/kriipke/platformctl/internal/models"
)

// newAuthTestRouter builds a router with the real gateway auth middleware chain
// (basic auth + customer-context injection) applied to the /api/v1 group, and
// registers two representative routes that mirror how the real handlers read the
// customer:
//
//   - GET /api/v1/contexts exercises the wrapped http.HandlerFunc path
//     (auth.RequireCustomer on the request context), shared by every CRUD and
//     gitops-action handler.
//   - GET /api/v1/gitops/contexts/:contextName/status exercises the native-gin
//     path (c.Get("customer") -> *models.Customer), shared by the status handlers.
//
// The real handlers additionally query Postgres; these stand-ins stop at the auth
// gate so the test can assert the 401 -> 2xx fix without a live database.
func newAuthTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	apiGroup := router.Group("/api/v1")
	apiGroup.Use(ginBasicAuthMiddleware())
	apiGroup.Use(ginCustomerContextMiddleware())

	apiGroup.GET("/contexts", ginHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		customer, err := auth.RequireCustomer(r.Context())
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"customer_id": customer.CustomerID})
	}))

	gitopsGroup := apiGroup.Group("/gitops")
	gitopsGroup.GET("/contexts/:contextName/status", func(c *gin.Context) {
		customer, exists := c.Get("customer")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "customer not found in context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"customer_id": customer.(*models.Customer).ID.String()})
	})

	return router
}

// TestAPIRoutesRejectUnauthenticated confirms both the wrapped-handler and the
// native-gin route shapes still reject requests without credentials.
func TestAPIRoutesRejectUnauthenticated(t *testing.T) {
	router := newAuthTestRouter()

	for _, path := range []string{"/api/v1/contexts", "/api/v1/gitops/contexts/demo/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s without credentials: expected 401, got %d", path, rec.Code)
		}
	}
}

// TestAPIRoutesAuthenticatedReachHandlers is the regression test for the
// 401-on-every-authenticated-route bug: with valid basic-auth credentials both
// route shapes must reach their handler (2xx, not 401), and the customer id must
// be identical across the two representations so a tenant's writes (CRUD, string
// id) and reads (status, uuid id) address the same customer_id.
func TestAPIRoutesAuthenticatedReachHandlers(t *testing.T) {
	router := newAuthTestRouter()

	// CRUD path: wrapped http.HandlerFunc reading auth.RequireCustomer.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contexts", nil)
	req.SetBasicAuth("admin", "admin")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/contexts authenticated: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	crudCustomerID := decodeCustomerID(t, rec)
	if crudCustomerID == "" {
		t.Fatal("CRUD path: expected a non-empty customer id in the request context")
	}

	// Status path: native-gin handler reading c.Get("customer").
	req = httptest.NewRequest(http.MethodGet, "/api/v1/gitops/contexts/demo/status", nil)
	req.SetBasicAuth("admin", "admin")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/gitops/contexts/demo/status authenticated: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	statusCustomerID := decodeCustomerID(t, rec)

	if crudCustomerID != statusCustomerID {
		t.Fatalf("customer id mismatch between CRUD (%q) and status (%q) paths", crudCustomerID, statusCustomerID)
	}
}

// TestDeriveCustomerUUIDStable checks the identifier->UUID mapping the middleware
// relies on to keep the two customer representations consistent.
func TestDeriveCustomerUUIDStable(t *testing.T) {
	a := deriveCustomerUUID("admin")
	if a != deriveCustomerUUID("admin") {
		t.Fatal("deriveCustomerUUID is not stable for the same identifier")
	}
	if a == uuid.Nil {
		t.Fatal("deriveCustomerUUID returned the nil UUID")
	}
	if deriveCustomerUUID("other") == a {
		t.Fatal("different identifiers should derive different UUIDs")
	}

	// A caller-supplied UUID is used verbatim.
	explicit := "11111111-2222-4333-8444-555555555555"
	if got := deriveCustomerUUID(explicit).String(); got != explicit {
		t.Fatalf("expected explicit uuid passthrough %q, got %q", explicit, got)
	}
}

func decodeCustomerID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		CustomerID string `json:"customer_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body %q: %v", rec.Body.String(), err)
	}
	return body.CustomerID
}
