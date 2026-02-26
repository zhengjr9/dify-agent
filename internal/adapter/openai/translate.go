package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zhengjr9/dify-agent/internal/dify"
)

// ToDifyRequest converts an OpenAI chat completions request to a Dify ChatRequest.
func ToDifyRequest(ctx context.Context, r *http.Request, defaultUser string) (*dify.ChatRequest, bool, error) {
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, false, fmt.Errorf("decode body: %w", err)
	}
	if len(req.Messages) == 0 {
		return nil, false, fmt.Errorf("messages must not be empty")
	}

	query, history := flattenMessages(req.Messages)

	difyReq := &dify.ChatRequest{
		Inputs:  map[string]any{},
		Query:   query,
		User:    defaultUser,
	}
	_ = history // prepended into query already

	return difyReq, req.Stream, nil
}

// flattenMessages converts OpenAI messages into a single query string.
// The last user message becomes the query; prior messages are prepended as context.
func flattenMessages(msgs []Message) (query string, _ []Message) {
	if len(msgs) == 1 {
		return msgs[0].Content, nil
	}

	last := msgs[len(msgs)-1]
	prior := msgs[:len(msgs)-1]

	var sb strings.Builder
	for _, m := range prior {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	sb.WriteString(last.Content)
	return sb.String(), prior
}

// WriteBlockingResponse encodes a Dify blocking response as an OpenAI ChatCompletionResponse.
func WriteBlockingResponse(w http.ResponseWriter, resp *dify.BlockingResponse, model string) error {
	finishReason := "stop"
	out := ChatCompletionResponse{
		ID:      resp.MessageID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: resp.Answer},
				FinishReason: finishReason,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(out)
}

// WriteStreamingResponse encodes Dify stream events as OpenAI SSE chunks.
func WriteStreamingResponse(w http.ResponseWriter, stream <-chan dify.StreamEvent, model string) error {
	for ev := range stream {
		if ev.Err != nil {
			return ev.Err
		}
		if ev.Event != "message" && ev.Event != "agent_message" {
			continue
		}

		chunk := StreamChunk{
			ID:      ev.MessageID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []StreamChoice{
				{
					Index: 0,
					Delta: Delta{Content: ev.Answer},
				},
			},
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("marshal chunk: %w", err)
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	_, err := fmt.Fprintf(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return err
}
