package dify

// FileInput represents a file attachment in a Dify chat request.
type FileInput struct {
	Type           string `json:"type"`            // e.g. "image"
	TransferMethod string `json:"transfer_method"` // "remote_url" | "local_file"
	URL            string `json:"url,omitempty"`
	UploadFileID   string `json:"upload_file_id,omitempty"`
}

// ChatRequest is sent to POST /v1/chat-messages.
type ChatRequest struct {
	Inputs         map[string]any `json:"inputs"`
	Query          string         `json:"query"`
	ResponseMode   string         `json:"response_mode"` // "blocking" | "streaming"
	ConversationID string         `json:"conversation_id,omitempty"`
	User           string         `json:"user"`
	Files          []FileInput    `json:"files,omitempty"`
}

// BlockingResponse is the full Dify response for response_mode=blocking.
type BlockingResponse struct {
	MessageID          string         `json:"message_id"`
	ConversationID     string         `json:"conversation_id"`
	Mode               string         `json:"mode"`
	Answer             string         `json:"answer"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          int64          `json:"created_at"`
}

// StreamEvent is one SSE event from Dify for response_mode=streaming.
type StreamEvent struct {
	Event          string `json:"event"`
	TaskID         string `json:"task_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Answer         string `json:"answer,omitempty"`
	CreatedAt      int64  `json:"created_at,omitempty"`
	// Error fields
	Status  int    `json:"status,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	// Err is set when the Go stream reader itself encounters an error.
	Err error `json:"-"`
}
