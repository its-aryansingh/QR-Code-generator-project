package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with status code and Content-Type application/json.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// ProblemJSON writes an RFC 7807 problem+json response.
func ProblemJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}
