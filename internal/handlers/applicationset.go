package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

type ApplicationSetHandler struct{}

func NewApplicationSetHandler() *ApplicationSetHandler {
	return &ApplicationSetHandler{}
}

func (h *ApplicationSetHandler) GetApplicationSets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

func (h *ApplicationSetHandler) GetApplicationSetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	writeJSON(w, http.StatusOK, map[string]string{"applicationset": vars["appset"], "status": "unknown"})
}

func (h *ApplicationSetHandler) UpdateApplicationSetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	writeJSON(w, http.StatusOK, map[string]string{"applicationset": vars["appset"], "status": "updated"})
}
