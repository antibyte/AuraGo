package chat

import (
	"time"

	"github.com/charmbracelet/bubbletea"
)

// Model is the chat TUI state.
type Model struct {
	ServerURL string
	Input     string

	// Messages
	Messages []Message

	// Streaming state
	Streaming       bool
	CurrentResponse string
	CurrentEvents   []Event

	// Connection state
	Connected bool
	Err       error

	// SSE client
	sseClient *SSEClient

	// Dimensions
	Width  int
	Height int
}

// Message represents a single message in the chat.
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Events    []Event   `json:"events,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Event represents a tool or agent event.
type Event struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// NewModel creates a new chat model.
func NewModel(serverURL string) *Model {
	return &Model{
		ServerURL:  serverURL,
		Messages:   []Message{},
		Connected:  false,
		Input:      "",
		Streaming:  false,
		sseClient: NewSSEClient(serverURL),
	}
}

// Init initializes the chat model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		connectSSE(m.sseClient),
	)
}

// HandleSSEEvent processes an incoming SSE event.
func (m *Model) HandleSSEEvent(event Event) {
	switch event.Type {
	case "thinking":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "thinking", Content: "thinking..."})
	case "tool_start":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "tool_start", Content: event.Content})
	case "tool_call":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "tool_call", Content: event.Content})
	case "tool_output":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "tool_output", Content: event.Content})
	case "tool_end":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "tool_end", Content: event.Content})
	case "error_recovery":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "error", Content: event.Content})
	case "done":
		// Response complete - finalize the message
		if m.CurrentResponse != "" || len(m.CurrentEvents) > 0 {
			m.Messages = append(m.Messages, Message{
				Role:      "assistant",
				Content:   m.CurrentResponse,
				Events:    m.CurrentEvents,
				Timestamp: time.Now(),
			})
			m.CurrentResponse = ""
			m.CurrentEvents = []Event{}
		}
		m.Streaming = false
	case "error":
		m.CurrentEvents = append(m.CurrentEvents, Event{Type: "error", Content: event.Content})
	}
}

// AddToken appends a token to the current response.
func (m *Model) AddToken(token string) {
	m.CurrentResponse += token
}

// FinalizeMessage adds the current streaming response as a final message.
func (m *Model) FinalizeMessage() {
	if m.CurrentResponse != "" || len(m.CurrentEvents) > 0 {
		m.Messages = append(m.Messages, Message{
			Role:      "assistant",
			Content:   m.CurrentResponse,
			Events:    m.CurrentEvents,
			Timestamp: time.Now(),
		})
		m.CurrentResponse = ""
		m.CurrentEvents = []Event{}
	}
}
