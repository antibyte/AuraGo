package chat

import "github.com/charmbracelet/lipgloss"

var (
	// Header styles
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#39FF14")).
			Bold(true)

	// Message styles
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	// Tool/event styles
	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Italic(true)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Bold(true)

	// Status bar
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	connectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	disconnectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333"))

	// Input area
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#39FF14")).
			Padding(1).
			Width(80)

	// Spinner
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#39FF14"))
)
