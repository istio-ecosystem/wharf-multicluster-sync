package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// RenderJSON outputs the given data as JSON
func RenderJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		RenderError(w, http.StatusInternalServerError, err)
	}
}

// RenderError outputs an error message
func RenderError(w http.ResponseWriter, statusCode int, err error) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%v", err)
}
