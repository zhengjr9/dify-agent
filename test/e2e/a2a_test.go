// Package e2e contains end-to-end tests for the A2A server.
//
// Required environment variables (test skips if absent):
//
//	DIFY_BASE_URL  – Dify chat-messages endpoint
//	DIFY_API_KEY   – Dify application API key
//	DEFAULT_USER   – user field forwarded to Dify
//
// Optional:
//
//	AGENT_NAME     – A2A AgentCard name (default: "e2e-test-agent")
package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// requireEnv returns the value of an env var or skips the test if it is unset.
func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("env %s not set – skipping E2E test", key)
	}
	return v
}

// freePort returns an available TCP port on loopback.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// waitReady polls url until it returns a non-5xx response or timeout expires.
func waitReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("server not ready at %s within %s", url, timeout)
}

// startA2AServer launches cmd/server with --a2a on randomly chosen ports,
// registers a cleanup to kill it, and returns the A2A base URL.
func startA2AServer(t *testing.T) string {
	t.Helper()

	difyBaseURL := requireEnv(t, "DIFY_BASE_URL")
	difyAPIKey := requireEnv(t, "DIFY_API_KEY")
	defaultUser := requireEnv(t, "DEFAULT_USER")
	agentName := os.Getenv("AGENT_NAME")
	if agentName == "" {
		agentName = "e2e-test-agent"
	}

	proxyPort := freePort(t)
	a2aPort := freePort(t)

	cmd := exec.Command(
		"go", "run", "github.com/zhengjr9/dify-agent/cmd/server",
		"--dify-base-url", difyBaseURL,
		"--dify-api-key", difyAPIKey,
		"--listen-addr", fmt.Sprintf(":%d", proxyPort),
		"--default-user", defaultUser,
		"--a2a",
		"--a2a-port", fmt.Sprintf("%d", a2aPort),
		"--agent-name", agentName,
	)
	// Print server output to test log so failures are diagnosable.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// WaitDelay ensures the process is cleaned up even if its I/O is slow.
	cmd.WaitDelay = 5 * time.Second

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	a2aBase := fmt.Sprintf("http://127.0.0.1:%d", a2aPort)

	// go run compiles first – give it up to 90s.
	if err := waitReady(a2aBase+"/.well-known/agent-card.json", 90*time.Second); err != nil {
		t.Fatalf("A2A server not ready: %v", err)
	}
	t.Logf("A2A server ready at %s (proxy :%d)", a2aBase, proxyPort)
	return a2aBase
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestE2E_A2A_AgentCard(t *testing.T) {
	agentName := os.Getenv("AGENT_NAME")
	if agentName == "" {
		agentName = "e2e-test-agent"
	}
	a2aBase := startA2AServer(t)

	resp, err := http.Get(a2aBase + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatalf("GET agent-card: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var card map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("decode agent-card: %v", err)
	}

	t.Logf("agent-card: %+v", card)

	if got, _ := card["name"].(string); got != agentName {
		t.Errorf("name: want %q, got %q", agentName, got)
	}
	if card["capabilities"] == nil {
		t.Error("missing capabilities field")
	}
	caps, _ := card["capabilities"].(map[string]any)
	if streaming, _ := caps["streaming"].(bool); !streaming {
		t.Errorf("expected capabilities.streaming=true, got %v", caps["streaming"])
	}
}

func TestE2E_A2A_MessageSend(t *testing.T) {
	a2aBase := startA2AServer(t)

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "e2e-send-1",
		"method":  "message/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":      "user",
				"parts":     []any{map[string]any{"kind": "text", "text": "你好"}},
				"messageId": "e2e-msg-001",
			},
		},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(a2aBase+"/", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /: %v", err)
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

	t.Logf("message/send response: %+v", result)

	if errField := result["error"]; errField != nil {
		t.Fatalf("JSON-RPC error: %v", errField)
	}

	res, _ := result["result"].(map[string]any)
	if res == nil {
		t.Fatalf("no result field in response: %v", result)
	}
	if res["kind"] != "task" {
		t.Errorf("expected kind=task, got %v", res["kind"])
	}

	status, _ := res["status"].(map[string]any)
	if state, _ := status["state"].(string); state != "completed" {
		t.Errorf("expected state=completed, got %q", state)
	}

	artifacts, _ := res["artifacts"].([]any)
	if len(artifacts) == 0 {
		t.Fatal("expected at least one artifact")
	}
	art, _ := artifacts[0].(map[string]any)
	parts, _ := art["parts"].([]any)
	if len(parts) == 0 {
		t.Fatal("expected at least one part in artifact")
	}
	part, _ := parts[0].(map[string]any)
	text, _ := part["text"].(string)
	if text == "" {
		t.Error("expected non-empty text in artifact")
	}
	t.Logf("answer (%d chars): %s…", len(text), truncate(text, 120))
}

func TestE2E_A2A_MessageStream(t *testing.T) {
	a2aBase := startA2AServer(t)

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "e2e-stream-1",
		"method":  "message/stream",
		"params": map[string]any{
			"message": map[string]any{
				"role":      "user",
				"parts":     []any{map[string]any{"kind": "text", "text": "你好"}},
				"messageId": "e2e-msg-002",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, a2aBase+"/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST / (stream): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}

	var (
		eventCount   int
		gotCompleted bool
		allText      strings.Builder
	)

	// extractParts collects non-empty text from an artifact's parts slice.
	extractParts := func(parts []any) {
		for _, p := range parts {
			part, _ := p.(map[string]any)
			if part["kind"] == "text" {
				if txt, _ := part["text"].(string); txt != "" {
					allText.WriteString(txt)
				}
			}
		}
	}

	// extractText handles both A2A event shapes:
	//   - task format:            result.artifacts[].parts[]
	//   - artifact-update format: result.artifact.parts[]
	extractText := func(res map[string]any) {
		// artifact-update event (streaming chunks)
		if art, ok := res["artifact"].(map[string]any); ok {
			parts, _ := art["parts"].([]any)
			extractParts(parts)
		}
		// task / final event
		for _, a := range func() []any { v, _ := res["artifacts"].([]any); return v }() {
			art, _ := a.(map[string]any)
			parts, _ := art["parts"].([]any)
			extractParts(parts)
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	// Default scanner buffer may be too small for large SSE lines.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		rest, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		eventCount++

		var ev map[string]any
		if err := json.Unmarshal([]byte(rest), &ev); err != nil {
			t.Logf("non-JSON SSE data (skipping): %s", rest[:min(len(rest), 80)])
			continue
		}

		if errField := ev["error"]; errField != nil {
			t.Fatalf("stream JSON-RPC error: %v", errField)
		}

		res, _ := ev["result"].(map[string]any)
		if res == nil {
			continue
		}

		// Collect text from every event (partial tokens arrive in working events).
		extractText(res)

		status, _ := res["status"].(map[string]any)
		if state, _ := status["state"].(string); state == "completed" {
			gotCompleted = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("SSE scanner error: %v", err)
	}

	t.Logf("received %d SSE events", eventCount)

	if eventCount == 0 {
		t.Error("no SSE events received")
	}
	if !gotCompleted {
		t.Error("stream never reached state=completed")
	}
	if allText.Len() == 0 {
		t.Error("expected non-empty streamed text")
	}
	t.Logf("streamed answer (%d chars): %s…", allText.Len(), truncate(allText.String(), 120))
}

// TestE2E_A2A_MultiTurn verifies that contextId is accepted in the second turn.
func TestE2E_A2A_MultiTurn(t *testing.T) {
	a2aBase := startA2AServer(t)

	send := func(text, messageID, contextID string) map[string]any {
		t.Helper()
		msg := map[string]any{
			"role":      "user",
			"parts":     []any{map[string]any{"kind": "text", "text": text}},
			"messageId": messageID,
		}
		if contextID != "" {
			msg["contextId"] = contextID
		}
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"method":  "message/send",
			"params":  map[string]any{"message": msg},
		}
		body, _ := json.Marshal(payload)
		resp, err := http.Post(a2aBase+"/", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /: %v", err)
		}
		defer resp.Body.Close()
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return result
	}

	// First turn
	r1 := send("你好", "mt-001", "")
	if r1["error"] != nil {
		t.Fatalf("turn 1 error: %v", r1["error"])
	}
	res1, _ := r1["result"].(map[string]any)
	contextID, _ := res1["contextId"].(string)
	t.Logf("turn 1 contextId: %s", contextID)

	// Second turn – pass contextId
	r2 := send("请继续", "mt-002", contextID)
	if r2["error"] != nil {
		t.Fatalf("turn 2 error: %v", r2["error"])
	}
	res2, _ := r2["result"].(map[string]any)
	status2, _ := res2["status"].(map[string]any)
	if state := status2["state"]; state != "completed" {
		t.Errorf("turn 2 expected state=completed, got %v", state)
	}
	t.Logf("turn 2 completed ok")
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
