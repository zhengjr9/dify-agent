package dify

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client sends requests to a Dify instance.
type Client struct {
	// chatURL is the full URL of the Dify chat-messages endpoint,
	// e.g. "https://aigc.example.com/dify/server/v1/chat-messages".
	// If it does not already end with "/v1/chat-messages" the suffix is appended
	// automatically so that callers can pass either a base host or the full URL.
	chatURL    string
	httpClient *http.Client
	// streamTransport is used by streaming requests (no timeout, but same proxy).
	streamTransport http.RoundTripper
}

// NewClient constructs a Client with the given base URL (or full endpoint URL), timeout,
// and optional proxy URL. proxyURL may be empty to use the default environment proxy.
func NewClient(baseURL string, timeout time.Duration, proxyURL string) *Client {
	chatURL := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(chatURL, "/v1/chat-messages") {
		chatURL += "/v1/chat-messages"
	}

	transport := &http.Transport{}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &Client{
		chatURL: chatURL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		streamTransport: transport,
	}
}

// SendBlocking sends a blocking chat-messages request and returns the parsed response.
func (c *Client) SendBlocking(ctx context.Context, apiKey string, req *ChatRequest) (*BlockingResponse, error) {
	req.ResponseMode = "blocking"
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	if req.User != "" {
		httpReq.Header.Set("AIGC-USER", req.User)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("dify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dify %d: %s", resp.StatusCode, string(raw))
	}

	var result BlockingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// SendStreaming sends a streaming chat-messages request and returns a channel of StreamEvents.
// The HTTP response body is closed when the channel is drained.
func (c *Client) SendStreaming(ctx context.Context, apiKey string, req *ChatRequest) (<-chan StreamEvent, error) {
	req.ResponseMode = "streaming"
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	if req.User != "" {
		httpReq.Header.Set("AIGC-USER", req.User)
	}

	// Use a client without timeout for streaming (context carries deadline),
	// but reuse the same transport so the proxy setting is preserved.
	streamClient := &http.Client{Transport: c.streamTransport}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("dify request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("dify %d: %s", resp.StatusCode, string(raw))
	}

	scanner := bufio.NewScanner(resp.Body)
	ch := make(chan StreamEvent, 16)
	go func() {
		defer resp.Body.Close()
		inner := ReadStream(scanner)
		for ev := range inner {
			ch <- ev
		}
		close(ch)
	}()
	return ch, nil
}
