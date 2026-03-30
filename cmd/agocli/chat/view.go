package chat

import (
	"fmt"
	"strings"
	"time"
)

// View renders the chat TUI.
func (m *Model) View() string {
	var sb strings.Builder

	// Header
	sb.WriteString(headerStyle.Render("AuraGo Chat"))
	sb.WriteString("  ")
	if m.Connected {
		sb.WriteString(connectedStyle.Render("[connected]"))
	} else {
		sb.WriteString(disconnectedStyle.Render("[disconnected]"))
	}
	sb.WriteString("  ")
	sb.WriteString(statusStyle.Render(m.ServerURL))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", max(m.Width, 40)) + "\n")

	// Messages
	if len(m.Messages) == 0 && !m.Streaming {
		sb.WriteString(statusStyle.Render("No messages yet. Type your first message below.\n"))
	}

	// Scroll window for messages
	msgsToShow := m.Messages
	if len(msgsToShow) > 50 {
		msgsToShow = msgsToShow[len(msgsToShow)-50:]
	}

	for _, msg := range msgsToShow {
		sb.WriteString(m.renderMessage(msg))
	}

	// Current streaming response
	if m.Streaming && (m.CurrentResponse != "" || len(m.CurrentEvents) > 0) {
		sb.WriteString(m.renderStreaming())
	}

	// Input area
	sb.WriteString("\n")
	sb.WriteString(m.renderInput())

	// Footer
	sb.WriteString("\n")
	sb.WriteString(statusStyle.Render("Ctrl+C: quit | Enter: send"))

	return sb.String()
}

func (m *Model) renderMessage(msg Message) string {
	var sb strings.Builder

	roleStr := m.roleLabel(msg.Role)
	switch msg.Role {
	case "user":
		sb.WriteString(userStyle.Render(roleStr))
	case "assistant":
		sb.WriteString(assistantStyle.Render(roleStr))
	default:
		sb.WriteString(systemStyle.Render(roleStr))
	}

	content := msg.Content
	if content == "" && len(msg.Events) > 0 {
		// Show events as content when no text
		content = m.renderEvents(msg.Events)
	}
	if content != "" {
		sb.WriteString(" ")
		sb.WriteString(assistantStyle.Render(content))
	}
	sb.WriteString("\n")

	// Render tool events
	if len(msg.Events) > 0 && msg.Content != "" {
		sb.WriteString(m.renderEvents(msg.Events))
	}

	return sb.String()
}

func (m *Model) renderStreaming() string {
	var sb strings.Builder
	sb.WriteString(assistantStyle.Render("assistant: "))
	if m.CurrentResponse != "" {
		sb.WriteString(assistantStyle.Render(m.CurrentResponse))
	}
	sb.WriteString("\n")

	if len(m.CurrentEvents) > 0 {
		sb.WriteString(m.renderEvents(m.CurrentEvents))
	}

	sb.WriteString(spinnerStyle.Render("thinking..."))
	sb.WriteString("\n")
	return sb.String()
}

func (m *Model) renderEvents(events []Event) string {
	var sb strings.Builder
	for _, event := range events {
		switch event.Type {
		case "thinking":
			sb.WriteString("  ")
			sb.WriteString(thinkingStyle.Render("💭 " + event.Content))
			sb.WriteString("\n")
		case "tool_start":
			sb.WriteString("  ")
			sb.WriteString(toolStyle.Render("🔧 starting: " + event.Content))
			sb.WriteString("\n")
		case "tool_call":
			sb.WriteString("  ")
			sb.WriteString(toolStyle.Render("📞 " + event.Content))
			sb.WriteString("\n")
		case "tool_output":
			if len(event.Content) > 100 {
				sb.WriteString("  ")
				sb.WriteString(toolStyle.Render("📤 output: " + event.Content[:100] + "..."))
			} else {
				sb.WriteString("  ")
				sb.WriteString(toolStyle.Render("📤 " + event.Content))
			}
			sb.WriteString("\n")
		case "tool_end":
			sb.WriteString("  ")
			sb.WriteString(toolStyle.Render("✅ done: " + event.Content))
			sb.WriteString("\n")
		case "error":
			sb.WriteString("  ")
			sb.WriteString(errorStyle.Render("❌ error: " + event.Content))
			sb.WriteString("\n")
		default:
			if event.Content != "" {
				sb.WriteString("  ")
				sb.WriteString(toolStyle.Render(event.Type+": "+event.Content))
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func (m *Model) renderInput() string {
	// Simple single-line input display
	prompt := userStyle.Render("> ")
	inputText := m.Input
	if m.Streaming {
		prompt = statusStyle.Render("> ")
		inputText = statusStyle.Render("[waiting for response...]")
	}

	// Calculate available width
	availableWidth := m.Width - len(prompt) - 2
	if availableWidth < 20 {
		availableWidth = 80
	}

	if len(inputText) > availableWidth {
		inputText = inputText[len(inputText)-availableWidth:]
	}

	return fmt.Sprintf("%s%s", prompt, inputText)
}

func (m *Model) roleLabel(role string) string {
	switch role {
	case "user":
		return "user:"
	case "assistant":
		return "assistant:"
	case "system":
		return "system:"
	default:
		return role + ":"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Simple spinner for thinking state
func spinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧"}
	i := int(time.Now().UnixNano()) % len(frames)
	return frames[i]
}
