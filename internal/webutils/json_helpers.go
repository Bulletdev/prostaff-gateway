package webutils

import (
	"encoding/json"
	"net/http"
)

type jsonError struct {
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

// RawJSON writes a pre-encoded JSON byte slice directly (used for cache hits).
func RawJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data) //nolint:errcheck
}

func ErrorJSON(w http.ResponseWriter, err error, status int) {
	WriteJSON(w, status, jsonError{Error: err.Error()})
}
