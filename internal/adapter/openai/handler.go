package openai

import (
	"context"
	"net/http"
	"strings"
	"time"

	apierrors "github.com/zhengjr9/dify-agent/internal/errors"
	"github.com/zhengjr9/dify-agent/internal/dify"
	"github.com/zhengjr9/dify-agent/internal/httputil"
)

// Handler implements the OpenAI chat completions endpoint.
type Handler struct {
	client      *dify.Client
	defaultUser string
	timeout     time.Duration
}

// NewHandler constructs a Handler.
func NewHandler(client *dify.Client, defaultUser string, timeout time.Duration) *Handler {
	return &Handler{client: client, defaultUser: defaultUser, timeout: timeout}
}

// ServeHTTP handles POST /v1/chat/completions.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	creds := httputil.ExtractCredentials(r, h.defaultUser)
	if creds.APIKey == "" {
		apierrors.WriteJSONError(w, http.StatusUnauthorized, "missing API key: provide X-Dify-Api-Key header or Authorization: Bearer <key>")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	difyReq, streaming, err := ToDifyRequest(ctx, r, creds.User)
	if err != nil {
		apierrors.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	model := "dify"

	if streaming {
		stream, err := h.client.SendStreaming(ctx, creds.APIKey, difyReq)
		if err != nil {
			writeUpstreamError(w, err)
			return
		}
		httputil.SetSSEHeaders(w)
		if err := WriteStreamingResponse(w, stream, model); err != nil {
			return
		}
		return
	}

	resp, err := h.client.SendBlocking(ctx, creds.APIKey, difyReq)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if err := WriteBlockingResponse(w, resp, model); err != nil {
		apierrors.WriteJSONError(w, http.StatusInternalServerError, "failed to write response")
	}
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "timeout") {
		apierrors.WriteJSONError(w, http.StatusGatewayTimeout, "upstream timeout")
		return
	}
	apierrors.WriteJSONError(w, http.StatusBadGateway, "upstream error: "+msg)
}
