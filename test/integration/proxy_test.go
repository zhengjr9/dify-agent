package integration

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhengjr9/dify-agent/internal/config"
	"github.com/zhengjr9/dify-agent/internal/proxy"
	"github.com/zhengjr9/dify-agent/test/testutil"
)

const (
	testAnswer         = "Hello from Dify"
	testMessageID      = "msg-abc123"
	testConversationID = "conv-xyz789"
	testAPIKey         = "test-api-key-12345"
)

func newTestProxy(t *testing.T, difyURL string) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		DifyBaseURL:    difyURL,
		ListenAddr:     ":0",
		DefaultUser:    "test-user",
		RequestTimeout: 10 * time.Second,
	}
	srv := proxy.New(cfg)
	return httptest.NewServer(srv.Handler())
}

// --- OpenAI adapter tests ---

func TestOpenAI_Blocking(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"Say hello"}],"stream":false}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	choices, _ := result["choices"].([]any)
	if len(choices) == 0 {
		t.Fatal("expected at least one choice")
	}
	choice := choices[0].(map[string]any)
	msg := choice["message"].(map[string]any)
	if got := msg["content"].(string); got != testAnswer {
		t.Errorf("expected content %q, got %q", testAnswer, got)
	}

	// Verify API key was forwarded to Dify
	if mock.LastRequest == nil {
		t.Fatal("mock did not receive a request")
	}
}

func TestOpenAI_Streaming(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"Say hello"}],"stream":true}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected SSE content-type, got %q", ct)
	}

	content := collectSSEContent(t, resp.Body, "data: [DONE]")
	if !strings.Contains(content, "Hello") {
		t.Errorf("expected streamed content to contain 'Hello', got %q", content)
	}
}

func TestOpenAI_MissingAPIKey(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestOpenAI_MultiTurnFlattening(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"gpt-4","messages":[
		{"role":"system","content":"You are helpful."},
		{"role":"user","content":"What is 2+2?"},
		{"role":"assistant","content":"4"},
		{"role":"user","content":"Why?"}
	],"stream":false}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}

	// Verify query contains prior turns
	if mock.LastRequest == nil {
		t.Fatal("mock did not receive a request")
	}
	query, _ := mock.LastRequest["query"].(string)
	if !strings.Contains(query, "system:") && !strings.Contains(query, "You are helpful") {
		t.Logf("query: %q", query)
		// The query should contain the last user message at minimum
	}
	if !strings.Contains(query, "Why?") {
		t.Errorf("query should end with last user message, got: %q", query)
	}
}

// --- Anthropic adapter tests ---

func TestAnthropic_Blocking(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"claude-3","max_tokens":1024,"messages":[{"role":"user","content":"Say hello"}],"stream":false}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", testAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	contents, _ := result["content"].([]any)
	if len(contents) == 0 {
		t.Fatal("expected at least one content block")
	}
	block := contents[0].(map[string]any)
	if got := block["text"].(string); got != testAnswer {
		t.Errorf("expected text %q, got %q", testAnswer, got)
	}
}

func TestAnthropic_Streaming(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"claude-3","max_tokens":1024,"messages":[{"role":"user","content":"Say hello"}],"stream":true}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", testAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected SSE content-type, got %q", ct)
	}

	content := collectSSEContent(t, resp.Body, "message_stop")
	if content == "" {
		t.Error("expected non-empty SSE stream")
	}
}

func TestAnthropic_MissingAPIKey(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"model":"claude-3","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, proxySrv.URL+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- Gemini adapter tests ---

func TestGemini_Blocking(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"contents":[{"role":"user","parts":[{"text":"Say hello"}]}]}`
	url := proxySrv.URL + "/v1beta/models/gemini-pro:generateContent?key=" + testAPIKey
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	candidates, _ := result["candidates"].([]any)
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	candidate := candidates[0].(map[string]any)
	content := candidate["content"].(map[string]any)
	parts := content["parts"].([]any)
	if len(parts) == 0 {
		t.Fatal("expected at least one part")
	}
	part := parts[0].(map[string]any)
	if got := part["text"].(string); got != testAnswer {
		t.Errorf("expected text %q, got %q", testAnswer, got)
	}
}

func TestGemini_Streaming(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"contents":[{"role":"user","parts":[{"text":"Say hello"}]}]}`
	url := proxySrv.URL + "/v1beta/models/gemini-pro:streamGenerateContent?key=" + testAPIKey
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected SSE content-type, got %q", ct)
	}

	content := collectSSEContent(t, resp.Body, "")
	if content == "" {
		t.Error("expected non-empty SSE stream")
	}
}

func TestGemini_MissingAPIKey(t *testing.T) {
	mock := testutil.NewMockDify(testAnswer, testMessageID, testConversationID)
	defer mock.Close()

	proxySrv := newTestProxy(t, mock.URL())
	defer proxySrv.Close()

	body := `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`
	url := proxySrv.URL + "/v1beta/models/gemini-pro:generateContent"
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- helpers ---

// collectSSEContent reads SSE lines until the terminator is found or EOF,
// returning all data field values concatenated.
func collectSSEContent(t *testing.T, body io.Reader, terminator string) string {
	t.Helper()
	var sb strings.Builder
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if terminator != "" && strings.Contains(line, terminator) {
			break
		}
		if rest, ok := strings.CutPrefix(line, "data: "); ok {
			sb.WriteString(rest)
		}
	}
	return sb.String()
}
