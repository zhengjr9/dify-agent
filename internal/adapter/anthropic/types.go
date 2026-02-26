package anthropic


// MessagesRequest mirrors the Anthropic Messages API request body.
type MessagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	System    string    `json:"system,omitempty"`
	Stream    bool      `json:"stream"`
}

// Message is a single Anthropic chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MessagesResponse is the blocking Anthropic response format.
type MessagesResponse struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Role         string    `json:"role"`
	Content      []Content `json:"content"`
	Model        string    `json:"model"`
	StopReason   string    `json:"stop_reason"`
	StopSequence *string   `json:"stop_sequence"`
	Usage        Usage     `json:"usage"`
}

// Content is a content block in a response.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Usage carries token counts.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent represents one Anthropic SSE event.
type StreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta *Delta `json:"delta,omitempty"`
}

// Delta carries incremental content in a stream event.
type Delta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

