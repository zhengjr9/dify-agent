package gemini

// GenerateContentRequest mirrors the Gemini generateContent request body.
type GenerateContentRequest struct {
	Contents         []Content         `json:"contents"`
	SystemInstruction *SystemInstruction `json:"system_instruction,omitempty"`
}

// Content is a single turn in a Gemini conversation.
type Content struct {
	Role  string `json:"role"` // "user" | "model"
	Parts []Part `json:"parts"`
}

// Part carries text content.
type Part struct {
	Text string `json:"text"`
}

// SystemInstruction carries the system prompt.
type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

// GenerateContentResponse is the Gemini blocking response format.
type GenerateContentResponse struct {
	Candidates    []Candidate   `json:"candidates"`
	UsageMetadata UsageMetadata `json:"usageMetadata"`
}

// Candidate is one response candidate.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason"`
	Index        int     `json:"index"`
}

// UsageMetadata carries token counts.
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// StreamResponse wraps a single SSE payload for Gemini streaming.
type StreamResponse struct {
	Candidates    []Candidate   `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}
