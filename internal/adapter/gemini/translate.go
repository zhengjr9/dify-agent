package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/zhengjr9/dify-agent/internal/dify"
)

// ToDifyRequest converts a Gemini generateContent request to a Dify ChatRequest.
// streaming indicates whether the request was to the streamGenerateContent endpoint.
func ToDifyRequest(ctx context.Context, r *http.Request, defaultUser string, streaming bool) (*dify.ChatRequest, bool, error) {
	var req GenerateContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, false, fmt.Errorf("decode body: %w", err)
	}
	if len(req.Contents) == 0 {
		return nil, false, fmt.Errorf("contents must not be empty")
	}

	query := flattenContents(req.Contents, req.SystemInstruction)

	difyReq := &dify.ChatRequest{
		Inputs: map[string]any{},
		Query:  query,
		User:   defaultUser,
	}

	return difyReq, streaming, nil
}

// flattenContents converts Gemini contents into a single query string.
func flattenContents(contents []Content, sys *SystemInstruction) string {
	var sb strings.Builder

	if sys != nil && len(sys.Parts) > 0 {
		sb.WriteString("system: ")
		sb.WriteString(joinParts(sys.Parts))
		sb.WriteString("\n")
	}

	if len(contents) == 1 {
		text := joinParts(contents[0].Parts)
		if sb.Len() > 0 {
			sb.WriteString(text)
			return sb.String()
		}
		return text
	}

	last := contents[len(contents)-1]
	prior := contents[:len(contents)-1]
	for _, c := range prior {
		role := c.Role
		if role == "model" {
			role = "assistant"
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(joinParts(c.Parts))
		sb.WriteString("\n")
	}
	sb.WriteString(joinParts(last.Parts))
	return sb.String()
}

func joinParts(parts []Part) string {
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		texts = append(texts, p.Text)
	}
	return strings.Join(texts, "")
}

// WriteBlockingResponse encodes a Dify blocking response as a Gemini GenerateContentResponse.
func WriteBlockingResponse(w http.ResponseWriter, resp *dify.BlockingResponse) error {
	out := GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Role:  "model",
					Parts: []Part{{Text: resp.Answer}},
				},
				FinishReason: "STOP",
				Index:        0,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(out)
}

// WriteStreamingResponse encodes Dify stream events as Gemini SSE JSON payloads.
func WriteStreamingResponse(w http.ResponseWriter, stream <-chan dify.StreamEvent) error {
	for ev := range stream {
		if ev.Err != nil {
			return ev.Err
		}
		if ev.Event != "message" && ev.Event != "agent_message" {
			continue
		}

		chunk := StreamResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Role:  "model",
						Parts: []Part{{Text: ev.Answer}},
					},
					FinishReason: "",
					Index:        0,
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
	return nil
}
