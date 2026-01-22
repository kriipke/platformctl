package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

type EnvironmentHandler struct{}

func NewEnvironmentHandler() *EnvironmentHandler {
	return &EnvironmentHandler{}
}

func (h *EnvironmentHandler) GetEnvironmentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	writeJSON(w, http.StatusOK, map[string]string{"environment": vars["env"], "status": "unknown"})
}

func (h *EnvironmentHandler) ListEnvironmentStatuses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

func (h *EnvironmentHandler) UpdateEnvironmentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	writeJSON(w, http.StatusOK, map[string]string{"environment": vars["env"], "status": "updated"})
}
