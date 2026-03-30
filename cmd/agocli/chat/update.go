package chat

import (
	"context"
	"time"

	"aurago/cmd/agocli/shared"

	"github.com/charmbracelet/bubbletea"
)

// Update handles all message types for the chat TUI.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case SSEEvent:
		m.HandleSSEEvent(msg.Event)
		// Continue listening for more events
		return m, connectSSE(m.sseClient)

	case SSEError:
		m.Err = msg.Err
		m.Connected = false
		return m, nil

	case streamingChunk:
		if msg.IsDone {
			m.FinalizeMessage()
			m.Streaming = false
		} else {
			m.AddToken(msg.Text)
			m.Streaming = true
		}
		return m, nil

	case connectedMsg:
		m.Connected = true
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.Input == "" || m.Streaming {
			return m, nil
		}

		userMsg := m.Input
		m.Input = ""
		m.Streaming = true

		// Add user message to history
		m.Messages = append(m.Messages, Message{
			Role:      "user",
			Content:   userMsg,
			Timestamp: time.Now(),
		})

		// Build message list for API
		apiMessages := make([]shared.ChatMessage, 0, len(m.Messages)+1)
		for _, msg := range m.Messages {
			apiMessages = append(apiMessages, shared.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		// Start streaming
		client := shared.NewChatClient(m.ServerURL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		return m, streamingResponse(ctx, client, apiMessages, cancel)

	case tea.KeyCtrlC:
		if m.sseClient != nil {
			m.sseClient.Stop()
		}
		return m, tea.Quit

	case tea.KeyBackspace:
		if len(m.Input) > 0 {
			m.Input = m.Input[:len(m.Input)-1]
		}
	}

	return m, nil
}

// streamingChunk is a tea.Msg containing a chunk of streaming response.
type streamingChunk struct {
	Text   string
	IsDone bool
	Err    error
}

// streamingResponse is a tea.Cmd that streams a chat response.
func streamingResponse(ctx context.Context, client *shared.ChatClient, messages []shared.ChatMessage, cancel context.CancelFunc) tea.Cmd {
	return func() tea.Msg {
		var lastText string
		err := client.StreamChat(ctx, messages, func(text string, isDone bool) {
			lastText = text
		})
		cancel()

		if err != nil {
			return streamingChunk{Err: err}
		}

		return streamingChunk{Text: lastText, IsDone: true}
	}
}

// connectedMsg indicates SSE connection is established.
type connectedMsg struct{}
