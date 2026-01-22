package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"platformctl/internal/auth"
)

func SetupGitOpsRouter(
	contextHandler *GitOpsContextHandler,
	applicationSetHandler *ApplicationSetHandler,
	environmentHandler *EnvironmentHandler,
	vaultHandler *VaultValidationHandler,
	customerAuth func(http.Handler) http.Handler,
) *mux.Router {
	router := mux.NewRouter()
	router.Use(customerAuth)
	router.Use(auth.CustomerIsolationMiddleware())

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)

	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/contexts", contextHandler.CreateContext).Methods(http.MethodPost)
	api.HandleFunc("/contexts", contextHandler.ListContexts).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}", contextHandler.GetContext).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}", contextHandler.UpdateContext).Methods(http.MethodPut)
	api.HandleFunc("/contexts/{name}", contextHandler.DeleteContext).Methods(http.MethodDelete)
	api.HandleFunc("/contexts/{name}/validate", contextHandler.ValidateContextForGitOps).Methods(http.MethodPost)
	api.HandleFunc("/contexts/{name}/status", contextHandler.GetContextStatus).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}/applicationsets", applicationSetHandler.GetApplicationSets).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}/applicationsets/{appset}", applicationSetHandler.GetApplicationSetStatus).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}/applicationsets/{appset}/status", applicationSetHandler.UpdateApplicationSetStatus).Methods(http.MethodPut)

	api.HandleFunc("/contexts/{name}/environments", environmentHandler.ListEnvironmentStatuses).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}/environments/{env}", environmentHandler.GetEnvironmentStatus).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}/environments/{env}/status", environmentHandler.UpdateEnvironmentStatus).Methods(http.MethodPut)

	api.HandleFunc("/contexts/{name}/vault/validate", vaultHandler.ValidateSecrets).Methods(http.MethodPost)
	api.HandleFunc("/contexts/{name}/vault/secrets", vaultHandler.GetSecretValidationStatus).Methods(http.MethodGet)
	api.HandleFunc("/contexts/{name}/vault/pod-env-validation", vaultHandler.ValidatePodEnvVars).Methods(http.MethodPost)

	return router
}
