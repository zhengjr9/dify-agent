# dify-agent

A Go gateway that exposes [Dify](https://dify.ai) applications through standard AI API protocols.

- **Proxy Server** — accepts OpenAI / Anthropic / Gemini API requests and forwards them to Dify
- **A2A Server** — wraps Dify as a [Google ADK](https://google.github.io/adk-docs/) agent over the [A2A protocol](https://google.github.io/A2A/)

## Architecture

```
Client (OpenAI / Anthropic / Gemini / A2A)
        │
        ▼
┌───────────────────┐     ┌────────────────┐
│  Proxy Server     │     │  A2A Server    │
│  :8080            │     │  :8000         │
│                   │     │  JSON-RPC 2.0  │
└────────┬──────────┘     └───────┬────────┘
         │                        │
         └──────────┬─────────────┘
                    ▼
           ┌────────────────┐
           │  Dify Client   │
           │  (SSE / block) │
           └────────────────┘
                    │
                    ▼
           Dify /v1/chat-messages
```

## Quick Start

```bash
# Build
go build ./...

# Proxy only (OpenAI / Anthropic / Gemini compatible)
go run ./cmd/server \
  --dify-base-url https://your-dify-host/v1 \
  --listen-addr :8080

# Proxy + A2A
go run ./cmd/server \
  --dify-base-url https://your-dify-host/v1 \
  --a2a \
  --a2a-port 8000 \
  --agent-name "my-agent"
```

## Configuration

All flags can also be set via environment variables.

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--dify-base-url` | `DIFY_BASE_URL` | `http://localhost` | Dify base URL or full chat-messages endpoint |
| `--dify-api-key` | `DIFY_API_KEY` | *(empty)* | Fallback Dify API key for A2A (optional) |
| `--dify-proxy-url` | `DIFY_PROXY_URL` | *(empty)* | HTTP/HTTPS proxy for Dify requests (e.g. `http://proxy:8080`) |
| `--listen-addr` | `LISTEN_ADDR` | `:8080` | Proxy listen address |
| `--default-user` | `DEFAULT_USER` | `dify-agent` | `user` field sent to Dify |
| `--request-timeout` | `REQUEST_TIMEOUT` | `120s` | Dify request timeout |
| `--a2a` | `A2A_ENABLED` | `false` | Enable A2A server |
| `--a2a-port` | `A2A_PORT` | `8000` | A2A server port |
| `--agent-name` | `AGENT_NAME` | `dify-agent` | A2A AgentCard name |
| `--agent-desc` | `AGENT_DESC` | `Dify-backed agent...` | A2A AgentCard description |

> When `--dify-proxy-url` is not set, the standard `HTTP_PROXY` / `HTTPS_PROXY` environment variables are respected automatically.

## Proxy Server

The proxy translates standard AI API requests into Dify `chat-messages` calls. The caller's `Authorization: Bearer <key>` header is forwarded as the Dify API key.

### OpenAI — `POST /v1/chat/completions`

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"dify","messages":[{"role":"user","content":"Hello"}],"stream":false}'
```

### Anthropic — `POST /v1/messages`

```bash
curl http://localhost:8080/v1/messages \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'
```

### Gemini — `POST /v1beta/models/{model}:generateContent`

```bash
curl "http://localhost:8080/v1beta/models/gemini-1.5-pro:generateContent" \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}'
```

All three endpoints support both blocking and streaming (`stream: true` / `:streamGenerateContent`).

## A2A Server

Implements the [A2A protocol](https://google.github.io/A2A/) (JSON-RPC 2.0 over SSE) on `:8000`.

The caller's `Authorization: Bearer <key>` header is used as the Dify API key per request. If omitted, `--dify-api-key` is used as a fallback.

### Agent Card

```bash
curl http://localhost:8000/.well-known/agent-card.json
```

### Send Message

```bash
curl -X POST http://localhost:8000/ \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -d '{
    "jsonrpc": "2.0", "id": "1", "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"kind": "text", "text": "Hello"}],
        "messageId": "msg-001"
      }
    }
  }'
```

### Streaming

```bash
curl -X POST http://localhost:8000/ \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -d '{
    "jsonrpc": "2.0", "id": "2", "method": "message/stream",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"kind": "text", "text": "Hello"}],
        "messageId": "msg-002"
      }
    }
  }'
```

## Development

```bash
# Run all tests
go test ./...

# Run E2E tests (requires a live Dify instance)
DIFY_BASE_URL=https://your-dify-host/v1 \
DIFY_API_KEY=app-xxxxxxxxxxxxxxxxxxxx \
DEFAULT_USER=your-username \
go test ./test/e2e/... -v -timeout 300s

# Lint
golangci-lint run ./...

# Vet
go vet ./...
```

## Project Layout

```
cmd/server/          # Binary entrypoint
internal/
  a2a/               # A2A agent (Dify → ADK session.Event)
  adapter/           # Protocol adapters (OpenAI / Anthropic / Gemini)
  config/            # Flag + env config
  dify/              # Dify HTTP client (blocking + streaming)
  proxy/             # Proxy HTTP server
test/
  e2e/               # End-to-end tests
  integration/       # Integration tests
docs/
  API.md             # Detailed API reference
```

## Requirements

- Go 1.25+
- A running [Dify](https://dify.ai) instance with at least one published app
