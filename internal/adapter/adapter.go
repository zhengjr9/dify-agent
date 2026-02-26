package adapter

import (
	"context"
	"net/http"

	"github.com/zhengjr9/dify-agent/internal/dify"
)

// Adapter translates between a caller's API format and Dify's API.
type Adapter interface {
	// ExtractAPIKey pulls the caller's API key from the request.
	// Returns an error if the key is absent or malformed.
	ExtractAPIKey(r *http.Request) (string, error)

	// ToDifyRequest parses the incoming request body and returns a Dify ChatRequest.
	ToDifyRequest(ctx context.Context, r *http.Request) (*dify.ChatRequest, error)

	// WriteBlockingResponse encodes the Dify blocking response into the caller's format.
	WriteBlockingResponse(w http.ResponseWriter, resp *dify.BlockingResponse) error

	// WriteStreamingResponse consumes the StreamEvent channel and encodes each chunk
	// into the caller's streaming format, flushing after each write.
	WriteStreamingResponse(w http.ResponseWriter, stream <-chan dify.StreamEvent) error
}
