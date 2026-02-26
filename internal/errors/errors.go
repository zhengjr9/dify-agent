package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrMissingAPIKey  = errors.New("missing API key")
	ErrMalformedBody  = errors.New("malformed request body")
	ErrDifyBadGateway = errors.New("dify returned non-2xx response")
	ErrDifyTimeout    = errors.New("dify request timed out")
)

type jsonError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	body := jsonError{
		Error:   http.StatusText(statusCode),
		Message: message,
	}
	_ = json.NewEncoder(w).Encode(body)
}
