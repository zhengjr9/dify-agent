package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/volcengine/veadk-go/apps"
	"github.com/volcengine/veadk-go/apps/a2a_app"
	"google.golang.org/adk/agent"

	"github.com/zhengjr9/dify-agent/internal/a2a"
	"github.com/zhengjr9/dify-agent/internal/config"
	"github.com/zhengjr9/dify-agent/internal/dify"
	"github.com/zhengjr9/dify-agent/internal/proxy"
)

func main() {
	cfg := config.Load()

	slog.Info("starting dify-agent",
		"listen", cfg.ListenAddr,
		"dify_base_url", cfg.DifyBaseURL,
		"a2a_enabled", cfg.A2AEnabled,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Always start the proxy server.
	srv := proxy.New(cfg)
	proxyErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			proxyErr <- err
		}
	}()

	// Optionally start the A2A server.
	a2aErr := make(chan error, 1)
	if cfg.A2AEnabled {
		difyClient := dify.NewClient(cfg.DifyBaseURL, cfg.RequestTimeout, cfg.DifyProxyURL)
		difyAgent, err := a2a.New(a2a.AgentConfig{
			Name:        cfg.AgentName,
			Description: cfg.AgentDesc,
			DifyClient:  difyClient,
			APIKey:      cfg.DifyAPIKey, // optional: fallback when caller omits Authorization
			DefaultUser: cfg.DefaultUser,
		})
		if err != nil {
			slog.Error("failed to create A2A agent", "error", err)
			os.Exit(1)
		}

		slog.Info("starting A2A server", "port", cfg.A2APort, "agent_name", cfg.AgentName)

		// Wrap the standard A2A app to inject an HTTP middleware that extracts
		// the caller's Bearer token and stores it in the request context before
		// the JSON-RPC handler sees the request.
		inner := a2a_app.NewAgentkitA2AServerApp(
			apps.DefaultApiConfig().SetPort(cfg.A2APort),
		)
		wrapped := &authMiddlewareApp{BasicApp: inner}

		go func() {
			if err := wrapped.Run(ctx, &apps.RunConfig{
				AgentLoader: agent.NewSingleLoader(difyAgent),
			}); err != nil {
				a2aErr <- err
			}
		}()
	}

	select {
	case <-ctx.Done():
		slog.Info("shutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*cfg.RequestTimeout/120)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Error("proxy shutdown error", "error", err)
		}
	case err := <-proxyErr:
		slog.Error("proxy server error", "error", err)
		os.Exit(1)
	case err := <-a2aErr:
		slog.Error("A2A server error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// authMiddlewareApp wraps a BasicApp and installs an HTTP middleware on the
// Gorilla mux router that extracts "Authorization: Bearer <token>" from every
// incoming request and injects the token into the request context via
// a2a.ContextWithAPIKey. This makes the token available to the agent's Run
// function regardless of how deep the framework buries the context.
type authMiddlewareApp struct {
	apps.BasicApp
}

// Run overrides the embedded Run so that apps.Run receives `w` as the app
// argument. Without this, the embedded Run calls apps.Run with the inner app,
// meaning apps.Run would invoke SetupRouters on the inner app and our
// middleware override would never be registered.
func (w *authMiddlewareApp) Run(ctx context.Context, config *apps.RunConfig) error {
	return apps.Run(ctx, config, w)
}

func (w *authMiddlewareApp) SetupRouters(router *mux.Router, config *apps.RunConfig) error {
	if err := w.BasicApp.SetupRouters(router, config); err != nil {
		return err
	}
	// Add the Bearer-token middleware after all routes are registered.
	router.Use(bearerTokenMiddleware)
	return nil
}

// bearerTokenMiddleware is a Gorilla mux middleware that reads
// "Authorization: Bearer <token>" and stores the token in the request context.
func bearerTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			if token, ok := strings.CutPrefix(auth, "Bearer "); ok && token != "" {
				r = r.WithContext(a2a.ContextWithAPIKey(r.Context(), token))
			}
		}
		next.ServeHTTP(w, r)
	})
}
