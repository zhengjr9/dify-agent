package gemini

import (
	"context"
	"net/http"
	"strings"
	"time"

	apierrors "github.com/zhengjr9/dify-agent/internal/errors"
	"github.com/zhengjr9/dify-agent/internal/dify"
	"github.com/zhengjr9/dify-agent/internal/httputil"
)

// Handler implements the Gemini generateContent / streamGenerateContent endpoints.
type Handler struct {
	client      *dify.Client
	defaultUser string
	timeout     time.Duration
}

// NewHandler constructs a Handler.
func NewHandler(client *dify.Client, defaultUser string, timeout time.Duration) *Handler {
	return &Handler{client: client, defaultUser: defaultUser, timeout: timeout}
}

// serveHTTP handles both generateContent and streamGenerateContent.
func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request, streaming bool) {
	creds := httputil.ExtractCredentials(r, h.defaultUser)
	if creds.APIKey == "" {
		apierrors.WriteJSONError(w, http.StatusUnauthorized, "missing API key: provide X-Dify-Api-Key header or Authorization: Bearer <key>")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	difyReq, isStream, err := ToDifyRequest(ctx, r, creds.User, streaming)
	if err != nil {
		apierrors.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if isStream {
		stream, err := h.client.SendStreaming(ctx, creds.APIKey, difyReq)
		if err != nil {
			writeUpstreamError(w, err)
			return
		}
		httputil.SetSSEHeaders(w)
		if err := WriteStreamingResponse(w, stream); err != nil {
			return
		}
		return
	}

	resp, err := h.client.SendBlocking(ctx, creds.APIKey, difyReq)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if err := WriteBlockingResponse(w, resp); err != nil {
		apierrors.WriteJSONError(w, http.StatusInternalServerError, "failed to write response")
	}
}

// Dispatch routes to blocking or streaming based on the URL path suffix.
func (h *Handler) Dispatch(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, ":streamGenerateContent"):
		h.serveHTTP(w, r, true)
	case strings.HasSuffix(path, ":generateContent"):
		h.serveHTTP(w, r, false)
	default:
		http.NotFound(w, r)
	}
}

// HandleBlocking serves POST /v1beta/models/{model}:generateContent.
func (h *Handler) HandleBlocking(w http.ResponseWriter, r *http.Request) {
	h.serveHTTP(w, r, false)
}

// HandleStreaming serves POST /v1beta/models/{model}:streamGenerateContent.
func (h *Handler) HandleStreaming(w http.ResponseWriter, r *http.Request) {
	h.serveHTTP(w, r, true)
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "timeout") {
		apierrors.WriteJSONError(w, http.StatusGatewayTimeout, "upstream timeout")
		return
	}
	apierrors.WriteJSONError(w, http.StatusBadGateway, "upstream error: "+msg)
}
