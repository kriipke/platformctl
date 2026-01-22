package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"platformctl/internal/auth"
	"platformctl/internal/models"
	"platformctl/internal/storage"
	"platformctl/internal/validation"
)

type GitOpsContextHandler struct {
	store     *storage.GitOpsContextStore
	validator *validation.Validator
}

func NewGitOpsContextHandler(store *storage.GitOpsContextStore, validator *validation.Validator) *GitOpsContextHandler {
	return &GitOpsContextHandler{store: store, validator: validator}
}

func (h *GitOpsContextHandler) CreateContext(w http.ResponseWriter, r *http.Request) {
	var contextModel models.Context
	if err := json.NewDecoder(r.Body).Decode(&contextModel); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateContext(h.validator, &contextModel); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.store.CreateContext(r.Context(), customer.CustomerID, &contextModel); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, contextModel)
}

func (h *GitOpsContextHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contextModel, err := h.store.GetContext(r.Context(), customer.CustomerID, name)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "context not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, contextModel)
}

func (h *GitOpsContextHandler) ListContexts(w http.ResponseWriter, r *http.Request) {
	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contexts, err := h.store.ListContexts(r.Context(), customer.CustomerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, contexts)
}

func (h *GitOpsContextHandler) UpdateContext(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var contextModel models.Context
	if err := json.NewDecoder(r.Body).Decode(&contextModel); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	contextModel.Metadata.Name = name
	if err := validation.ValidateContext(h.validator, &contextModel); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.store.UpdateContext(r.Context(), customer.CustomerID, &contextModel); err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "context not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, contextModel)
}

func (h *GitOpsContextHandler) DeleteContext(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.store.DeleteContext(r.Context(), customer.CustomerID, name); err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "context not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *GitOpsContextHandler) ValidateContextForGitOps(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contextModel, err := h.store.GetContext(r.Context(), customer.CustomerID, name)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "context not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := validation.ValidateContext(h.validator, contextModel); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
}

func (h *GitOpsContextHandler) GetContextStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
}

func (h *GitOpsContextHandler) GetContextEnvironments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	customer, err := auth.GetCustomerFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contextModel, err := h.store.GetContext(r.Context(), customer.CustomerID, name)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "context not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, contextModel.Spec.Deployments)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
