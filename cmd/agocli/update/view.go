package update

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the update wizard.
func (m *Model) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(headerStyle.Render("AuraGo Update Wizard"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", clamp(m.Width, 40, 120)) + "\n\n")

	// Progress steps
	sb.WriteString(m.renderProgress())
	sb.WriteString("\n\n")

	// Step content
	info := GetStepInfo(m.CurrentStep)
	sb.WriteString(stepTitle.Render(info.Title))
	sb.WriteString("\n\n")

	// Render active huh form if present
	if form := m.activeForm(); form != nil {
		sb.WriteString(form.View())
		sb.WriteString("\n")
	} else if m.CurrentStep == StepChangelog && m.Changelog != "" {
		sb.WriteString(changelogStyle.Render(m.Changelog))
		sb.WriteString("\n\n")
		sb.WriteString(stepContent.Render("Press Enter to continue..."))
		sb.WriteString("\n")
	} else if m.Done && m.Err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.Err)))
		sb.WriteString("\n")
	} else {
		sb.WriteString(stepContent.Render(info.Content))
		sb.WriteString("\n")
	}

	// Version info
	if m.LatestVersion != "" && m.CurrentStep <= StepConfirm {
		sb.WriteString("\n")
		sb.WriteString(versionStyle.Render("Latest: " + m.LatestVersion))
		sb.WriteString("\n")
	}

	// Output log
	if len(m.Output) > 0 {
		maxLines := 15
		start := 0
		if len(m.Output) > maxLines {
			start = len(m.Output) - maxLines
		}

		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("─", clamp(m.Width, 40, 120)) + "\n")
		sb.WriteString(outputLogStyle.Render("Log:"))
		sb.WriteString("\n")
		for _, line := range m.Output[start:] {
			if strings.Contains(line, "ERROR") || strings.Contains(line, "failed") {
				sb.WriteString(errorStyle.Render("  " + line))
			} else if strings.Contains(line, "WARNING") {
				sb.WriteString(warningStyle.Render("  " + line))
			} else if strings.Contains(line, "successfully") || strings.Contains(line, "complete") {
				sb.WriteString(successStyle.Render("  " + line))
			} else {
				sb.WriteString(outputStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

	// Prompt
	sb.WriteString("\n")
	if m.Done {
		sb.WriteString(stepContent.Render("Press Enter to exit..."))
	} else if m.Running {
		sb.WriteString(outputStyle.Render("Working..."))
	}

	return sb.String()
}

func (m *Model) renderProgress() string {
	labels := StepLabels()
	var sb strings.Builder

	for i, label := range labels {
		step := Step(i)
		if step < m.CurrentStep {
			sb.WriteString(stepDone.Render("[✓] " + label))
		} else if step == m.CurrentStep {
			if m.Done && m.Err != nil {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true).Render("[✗] " + label))
			} else {
				sb.WriteString(stepActive.Render("[→] " + label))
			}
		} else {
			sb.WriteString(stepPending.Render("[ ] " + label))
		}
		sb.WriteString("  ")
	}

	return sb.String()
}

// Additional styles
var (
	changelogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).MarginLeft(2)
	versionStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	outputLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	outputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
)

func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}
