# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`dify-agent` is a Go module (`github.com/zhengjr9/dify-agent`) targeting Go 1.25.

It exposes a single binary (`cmd/server`) that starts:
- **Proxy Server** (default `:8080`) — translates OpenAI / Anthropic / Gemini API requests into Dify `chat-messages` calls and streams responses back.
- **A2A Server** (optional, default `:8000`) — wraps the same Dify backend as a Google ADK / A2A-protocol agent (JSON-RPC 2.0 over SSE).

## Common Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run a single test
go test ./path/to/package -run TestFunctionName

# Lint (requires golangci-lint)
golangci-lint run ./...

# Vet
go vet ./...
```

## Architecture

This repository is intended to implement a Dify agent in Go. As the codebase grows, update this section with:
- Package layout and responsibilities
- Key interfaces and abstractions
- External dependencies and integrations (e.g., Dify API)
