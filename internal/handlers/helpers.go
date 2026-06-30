package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kriipke/platformctl/pkg/api"
)

// writeJSONResponse writes a JSON response with the given status code
func writeJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeErrorResponse writes an error response
func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := api.ErrorResponse{
		Success: false,
		Error:   message,
		Code:    statusCode,
	}
	writeJSONResponse(w, response, statusCode)
}

// writeValidationErrorResponse writes a validation error response
func writeValidationErrorResponse(w http.ResponseWriter, message string, details []api.ValidationError) {
	response := api.ValidationErrorResponse{
		Success: false,
		Error:   message,
		Details: details,
	}
	writeJSONResponse(w, response, http.StatusBadRequest)
}