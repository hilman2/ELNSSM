package api

import (
	"encoding/json"
	"net/http"
)

// Response is the standard API response envelope.
type Response struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Total  *int   `json:"total,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// Error represents an API error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Status: "ok",
		Data:   data,
	})
}

func writeJSONList(w http.ResponseWriter, data any, total int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Status: "ok",
		Data:   data,
		Total:  &total,
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Status: "error",
		Error: &Error{
			Code:    code,
			Message: message,
		},
	})
}
