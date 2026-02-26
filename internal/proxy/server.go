package proxy

import (
	"context"
	"net/http"
	"time"

	"github.com/zhengjr9/dify-agent/internal/adapter/anthropic"
	"github.com/zhengjr9/dify-agent/internal/adapter/gemini"
	"github.com/zhengjr9/dify-agent/internal/adapter/openai"
	"github.com/zhengjr9/dify-agent/internal/config"
	"github.com/zhengjr9/dify-agent/internal/dify"
)

// Server is the reverse proxy HTTP server.
type Server struct {
	httpServer *http.Server
}

// New constructs a Server from the given config.
func New(cfg *config.Config) *Server {
	client := dify.NewClient(cfg.DifyBaseURL, cfg.RequestTimeout, cfg.DifyProxyURL)

	oaHandler := openai.NewHandler(client, cfg.DefaultUser, cfg.RequestTimeout)
	anHandler := anthropic.NewHandler(client, cfg.DefaultUser, cfg.RequestTimeout)
	gmHandler := gemini.NewHandler(client, cfg.DefaultUser, cfg.RequestTimeout)

	mux := http.NewServeMux()

	// OpenAI
	mux.Handle("POST /v1/chat/completions", oaHandler)

	// Anthropic
	mux.Handle("POST /v1/messages", anHandler)

	// Gemini: ServeMux wildcards cannot be mixed with literal suffixes in the same
	// segment (e.g. "{model}:generateContent" is invalid). Use a prefix catch-all
	// and dispatch to blocking vs streaming by path suffix inside the handler.
	mux.HandleFunc("POST /v1beta/models/", gmHandler.Dispatch)

	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: cfg.RequestTimeout + 10*time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start begins listening and blocks until the server is stopped.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Handler returns the underlying http.Handler (for use in tests with httptest.NewServer).
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
