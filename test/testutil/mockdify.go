package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

// MockDify is an httptest.Server that simulates a Dify /v1/chat-messages endpoint.
type MockDify struct {
	Server *httptest.Server

	// Configurable response fields
	Answer         string
	MessageID      string
	ConversationID string

	// LastRequest captures the most recent request body parsed.
	LastRequest map[string]any
}

// NewMockDify creates and starts a mock Dify server.
func NewMockDify(answer, messageID, conversationID string) *MockDify {
	m := &MockDify{
		Answer:         answer,
		MessageID:      messageID,
		ConversationID: conversationID,
	}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

// Close shuts down the mock server.
func (m *MockDify) Close() {
	m.Server.Close()
}

// URL returns the base URL of the mock server.
func (m *MockDify) URL() string {
	return m.Server.URL
}

func (m *MockDify) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/chat-messages" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	// Parse request body
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	m.LastRequest = body

	mode, _ := body["response_mode"].(string)

	if mode == "streaming" {
		m.writeStreaming(w)
		return
	}
	m.writeBlocking(w)
}

func (m *MockDify) writeBlocking(w http.ResponseWriter) {
	resp := map[string]any{
		"message_id":      m.MessageID,
		"conversation_id": m.ConversationID,
		"mode":            "blocking",
		"answer":          m.Answer,
		"metadata":        map[string]any{},
		"created_at":      time.Now().Unix(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (m *MockDify) writeStreaming(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, hasFlusher := w.(http.Flusher)

	// Split the answer into words for a realistic stream
	words := splitWords(m.Answer)
	for i, word := range words {
		chunk := map[string]any{
			"event":           "message",
			"task_id":         "task-1",
			"message_id":      m.MessageID,
			"conversation_id": m.ConversationID,
			"answer":          word,
			"created_at":      time.Now().Unix(),
		}
		if i > 0 {
			chunk["answer"] = " " + word
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if hasFlusher {
			flusher.Flush()
		}
	}

	// Send message_end event
	endChunk := map[string]any{
		"event":           "message_end",
		"task_id":         "task-1",
		"message_id":      m.MessageID,
		"conversation_id": m.ConversationID,
	}
	data, _ := json.Marshal(endChunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if hasFlusher {
		flusher.Flush()
	}
}

func splitWords(s string) []string {
	var words []string
	start := -1
	for i, c := range s {
		if c != ' ' {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				words = append(words, s[start:i])
				start = -1
			}
		}
	}
	if start != -1 {
		words = append(words, s[start:])
	}
	if len(words) == 0 {
		words = []string{s}
	}
	return words
}
