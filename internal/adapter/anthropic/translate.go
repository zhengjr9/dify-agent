package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/zhengjr9/dify-agent/internal/dify"
)

// ToDifyRequest converts an Anthropic Messages request to a Dify ChatRequest.
func ToDifyRequest(ctx context.Context, r *http.Request, defaultUser string) (*dify.ChatRequest, bool, error) {
	var req MessagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, false, fmt.Errorf("decode body: %w", err)
	}
	if len(req.Messages) == 0 {
		return nil, false, fmt.Errorf("messages must not be empty")
	}

	query := flattenMessages(req.Messages, req.System)

	difyReq := &dify.ChatRequest{
		Inputs: map[string]any{},
		Query:  query,
		User:   defaultUser,
	}

	return difyReq, req.Stream, nil
}

// flattenMessages converts Anthropic messages into a single query string.
func flattenMessages(msgs []Message, system string) string {
	var sb strings.Builder
	if system != "" {
		sb.WriteString("system: ")
		sb.WriteString(system)
		sb.WriteString("\n")
	}

	if len(msgs) == 1 {
		if sb.Len() > 0 {
			sb.WriteString(msgs[0].Content)
			return sb.String()
		}
		return msgs[0].Content
	}

	last := msgs[len(msgs)-1]
	prior := msgs[:len(msgs)-1]
	for _, m := range prior {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	sb.WriteString(last.Content)
	return sb.String()
}

// WriteBlockingResponse encodes a Dify blocking response as an Anthropic MessagesResponse.
func WriteBlockingResponse(w http.ResponseWriter, resp *dify.BlockingResponse, model string) error {
	out := MessagesResponse{
		ID:         resp.MessageID,
		Type:       "message",
		Role:       "assistant",
		Content:    []Content{{Type: "text", Text: resp.Answer}},
		Model:      model,
		StopReason: "end_turn",
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(out)
}

// WriteStreamingResponse encodes Dify stream events as Anthropic SSE events.
func WriteStreamingResponse(w http.ResponseWriter, stream <-chan dify.StreamEvent, model string) error {
	// Send message_start
	startEvt := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":    "",
			"type":  "message",
			"role":  "assistant",
			"model": model,
		},
	}
	if err := writeSSEEvent(w, "message_start", startEvt); err != nil {
		return err
	}

	// Send content_block_start
	blockStart := StreamEvent{Type: "content_block_start", Index: 0}
	if err := writeSSEEvent(w, "content_block_start", blockStart); err != nil {
		return err
	}

	for ev := range stream {
		if ev.Err != nil {
			return ev.Err
		}
		if ev.Event != "message" && ev.Event != "agent_message" {
			continue
		}

		delta := StreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &Delta{Type: "text_delta", Text: ev.Answer},
		}
		if err := writeSSEEvent(w, "content_block_delta", delta); err != nil {
			return err
		}
	}

	// Send content_block_stop and message_stop
	if err := writeSSEEvent(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0}); err != nil {
		return err
	}
	if err := writeSSEEvent(w, "message_stop", map[string]any{"type": "message_stop"}); err != nil {
		return err
	}
	return nil
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event %s: %w", event, err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}
