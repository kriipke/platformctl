package handlers

import (
	"net/http"
)

type VaultValidationHandler struct{}

func NewVaultValidationHandler() *VaultValidationHandler {
	return &VaultValidationHandler{}
}

func (h *VaultValidationHandler) ValidateSecrets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
}

func (h *VaultValidationHandler) GetSecretValidationStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

func (h *VaultValidationHandler) ValidatePodEnvVars(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
}
