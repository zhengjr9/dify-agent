package a2a

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/zhengjr9/dify-agent/internal/dify"
)

// apiKeyContextKey is the context key used to propagate the caller's API key
// from the HTTP layer into the agent's Run function.
type apiKeyContextKey struct{}

// ContextWithAPIKey returns a new context carrying the given Dify API key.
// Call this in an HTTP middleware before the request reaches the A2A handler.
func ContextWithAPIKey(ctx context.Context, apiKey string) context.Context {
	return context.WithValue(ctx, apiKeyContextKey{}, apiKey)
}

// apiKeyFromContext retrieves the API key injected by the HTTP middleware.
// Returns ("", false) when no key was injected.
func apiKeyFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(apiKeyContextKey{}).(string)
	return v, ok && v != ""
}

// AgentConfig holds the configuration for the Dify-backed A2A agent.
type AgentConfig struct {
	// Name is the agent name exposed via A2A AgentCard.
	Name string
	// Description is exposed via A2A AgentCard.
	Description string
	// DifyClient is the pre-constructed Dify HTTP client.
	DifyClient *dify.Client
	// APIKey is the optional server-side Dify API key.
	// When empty the per-request key extracted from the caller's
	// Authorization header is used instead.
	APIKey string
	// DefaultUser is the fallback user field for Dify requests.
	DefaultUser string
}

// New returns an agent.Agent whose Run logic calls the Dify chat-messages API
// (streaming) and converts the SSE events into session.Events that the ADK
// runner understands.
func New(cfg AgentConfig) (agent.Agent, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("a2a agent: Name must not be empty")
	}
	if cfg.DifyClient == nil {
		return nil, fmt.Errorf("a2a agent: DifyClient must not be nil")
	}
	if cfg.DefaultUser == "" {
		cfg.DefaultUser = "dify-agent-a2a"
	}

	return agent.New(agent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Run:         runFunc(cfg),
	})
}

// runFunc returns the Run closure that drives one agent invocation.
func runFunc(cfg AgentConfig) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			// Resolve API key: prefer per-request key injected by the HTTP
			// middleware; fall back to the server-side configured key.
			apiKey, ok := apiKeyFromContext(ctx)
			if !ok {
				apiKey = cfg.APIKey
			}
			if apiKey == "" {
				yield(nil, fmt.Errorf("no Dify API key: set --dify-api-key or pass Authorization: Bearer <key>"))
				return
			}

			query := extractQuery(ctx.UserContent())
			if query == "" {
				ev := session.NewEvent(ctx.InvocationID())
				ev.Author = cfg.Name
				ev.LLMResponse = model.LLMResponse{
					Content: textContent("(empty input)"),
				}
				yield(ev, nil)
				return
			}

			difyReq := &dify.ChatRequest{
				Inputs:      map[string]any{},
				Query:       query,
				User:        cfg.DefaultUser,
			}

			streamCh, err := cfg.DifyClient.SendStreaming(ctx, apiKey, difyReq)
			if err != nil {
				yield(nil, fmt.Errorf("dify streaming request failed: %w", err))
				return
			}

			var fullText strings.Builder
			for ev := range streamCh {
				if ev.Err != nil {
					yield(nil, fmt.Errorf("dify stream error: %w", ev.Err))
					return
				}
				if ev.Event != "message" && ev.Event != "agent_message" {
					continue
				}
				fullText.WriteString(ev.Answer)

				// Emit a partial event so streaming A2A clients see tokens as they arrive.
				partialEv := session.NewEvent(ctx.InvocationID())
				partialEv.Author = cfg.Name
				partialEv.Branch = ctx.Branch()
				partialEv.LLMResponse = model.LLMResponse{
					Content: textContent(ev.Answer),
					Partial: true,
				}
				if !yield(partialEv, nil) {
					return
				}
			}

			// Emit the final (non-partial) event with the complete answer so that
			// IsFinalResponse() returns true and the runner closes the invocation.
			finalEv := session.NewEvent(ctx.InvocationID())
			finalEv.Author = cfg.Name
			finalEv.Branch = ctx.Branch()
			finalEv.LLMResponse = model.LLMResponse{
				Content: textContent(fullText.String()),
				Partial: false,
			}
			yield(finalEv, nil)
		}
	}
}

// extractQuery pulls the plain-text content from the genai.Content that ADK
// puts in the InvocationContext when the caller sends a message.
func extractQuery(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(sb.String())
}

// textContent is a small helper that wraps a string into a *genai.Content.
func textContent(text string) *genai.Content {
	return &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{{Text: text}},
	}
}

