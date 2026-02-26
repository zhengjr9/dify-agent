package httputil

import (
	"net/http"
	"strings"
)

// SetSSEHeaders sets the standard headers for a Server-Sent Events response.
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

// Credentials holds the Dify API key and user extracted from a request.
type Credentials struct {
	APIKey string
	User   string
}

// ExtractCredentials reads Dify credentials from the request using the following priority:
//
//  1. X-Dify-Api-Key header  → apiKey
//  2. Authorization: Bearer  → apiKey (fallback)
//  3. X-Dify-User header     → user   (overrides defaultUser when present)
//
// Returns an empty APIKey when no key is found; callers must validate.
func ExtractCredentials(r *http.Request, defaultUser string) Credentials {
	apiKey := strings.TrimSpace(r.Header.Get("X-Dify-Api-Key"))
	if apiKey == "" {
		auth := r.Header.Get("Authorization")
		if rest, ok := strings.CutPrefix(auth, "Bearer "); ok {
			apiKey = strings.TrimSpace(rest)
		}
	}

	user := strings.TrimSpace(r.Header.Get("X-Dify-User"))
	if user == "" {
		user = defaultUser
	}

	return Credentials{APIKey: apiKey, User: user}
}
