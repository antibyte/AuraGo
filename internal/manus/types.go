package manus

import "encoding/json"

// ContentPart is a text or uploaded-file part in a Manus message.
type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
}

// Message is the v2 task message envelope.
type Message struct {
	Content      any      `json:"content"`
	Connectors   []string `json:"connectors"`
	EnableSkills []string `json:"enable_skills,omitempty"`
	ForceSkills  []string `json:"force_skills,omitempty"`
}

// CreateTaskRequest contains AuraGo-controlled task creation options.
type CreateTaskRequest struct {
	Content                any
	ProjectID              string
	Locale                 string
	InteractiveMode        bool
	AgentProfile           string
	Title                  string
	Connectors             []string
	EnableSkills           []string
	ForceSkills            []string
	StructuredOutputSchema map[string]any
}

// CreateTaskResult identifies a newly created asynchronous task.
type CreateTaskResult struct {
	OK         bool   `json:"ok"`
	RequestID  string `json:"request_id"`
	TaskID     string `json:"task_id"`
	TaskTitle  string `json:"task_title"`
	TaskURL    string `json:"task_url"`
	Visibility string `json:"share_visibility"`
}

// Task is safe task metadata returned by Manus.
type Task struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	TaskType        string `json:"task_type"`
	ShareVisibility string `json:"share_visibility"`
	Title           string `json:"title"`
	CreditUsage     int    `json:"credit_usage"`
	TaskURL         string `json:"task_url"`
	AgentProfile    string `json:"agent_profile"`
}

// TaskAttachment describes a Manus-generated file without downloading it.
type TaskAttachment struct {
	Type        string `json:"type"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
}

// MessagePayload is shared by user and assistant events.
type MessagePayload struct {
	Content     string           `json:"content"`
	MessageType string           `json:"message_type,omitempty"`
	Attachments []TaskAttachment `json:"attachments,omitempty"`
}

// StatusDetail explains why a Manus task is waiting.
type StatusDetail struct {
	WaitingForEventID   string         `json:"waiting_for_event_id,omitempty"`
	WaitingForEventType string         `json:"waiting_for_event_type"`
	WaitingDescription  string         `json:"waiting_description"`
	ConfirmInputSchema  map[string]any `json:"confirm_input_schema,omitempty"`
}

// StatusUpdate is a task lifecycle transition.
type StatusUpdate struct {
	AgentStatus  string       `json:"agent_status"`
	StatusDetail StatusDetail `json:"status_detail"`
	Brief        string       `json:"brief,omitempty"`
	Description  string       `json:"description,omitempty"`
}

// StructuredOutputResult is valid only when Success is true.
type StructuredOutputResult struct {
	Success bool            `json:"success"`
	Value   json.RawMessage `json:"value"`
	Error   *string         `json:"error"`
}

// TaskEvent is the non-verbose event subset AuraGo exposes to its agent.
type TaskEvent struct {
	ID                     string                  `json:"id"`
	Type                   string                  `json:"type"`
	Timestamp              int64                   `json:"timestamp"`
	UserMessage            MessagePayload          `json:"user_message"`
	AssistantMessage       MessagePayload          `json:"assistant_message"`
	ErrorMessage           map[string]string       `json:"error_message"`
	StatusUpdate           StatusUpdate            `json:"status_update"`
	StructuredOutputResult *StructuredOutputResult `json:"structured_output_result,omitempty"`
}

// ListMessagesOptions controls safe, non-verbose pagination.
type ListMessagesOptions struct {
	TaskID string
	Cursor string
	Limit  int
	Order  string
}

// MessagePage is one cursor page of task events.
type MessagePage struct {
	OK         bool        `json:"ok"`
	RequestID  string      `json:"request_id"`
	TaskID     string      `json:"task_id"`
	Messages   []TaskEvent `json:"messages"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor"`
}

// SendMessageRequest continues a tracked task.
type SendMessageRequest struct {
	TaskID                 string
	Content                any
	Connectors             []string
	EnableSkills           []string
	ForceSkills            []string
	AgentProfile           string
	StructuredOutputSchema map[string]any
	ClearConnectors        bool
}

// SendMessageResult confirms that the task resumed.
type SendMessageResult struct {
	OK        bool   `json:"ok"`
	RequestID string `json:"request_id"`
	TaskID    string `json:"task_id"`
}
