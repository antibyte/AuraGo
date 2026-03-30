package update

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the update wizard.
func (m *Model) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(headerStyle.Render("AuraGo Update Wizard"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", max(m.Width, 40)) + "\n\n")

	// Progress steps
	sb.WriteString(m.renderProgress())
	sb.WriteString("\n")

	// Step content
	info := GetStepInfo(m.CurrentStep)
	sb.WriteString(stepTitle.Render(info.Title))
	sb.WriteString("\n\n")
	sb.WriteString(stepContent.Render(info.Content))
	sb.WriteString("\n")

	// Show changelog if available
	if m.CurrentStep == StepChangelog && m.Changelog != "" {
		sb.WriteString("\n")
		sb.WriteString(changelogStyle.Render(m.Changelog))
		sb.WriteString("\n")
	}

	// Version info
	if m.CurrentStep == StepCheck {
		sb.WriteString("\n")
		sb.WriteString(versionStyle.Render("Current: " + m.CurrentVersion + " → Latest: " + m.LatestVersion))
		sb.WriteString("\n")
	}

	// Output log
	if len(m.Output) > 0 {
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("─", max(m.Width, 40)) + "\n")
		sb.WriteString(outputLogStyle.Render("Output:\n"))
		for _, line := range m.Output {
			if strings.Contains(line, "ERROR") || strings.Contains(line, "failed") {
				sb.WriteString(errorStyle.Render("  " + line + "\n"))
			} else if strings.Contains(line, "WARNING") {
				sb.WriteString(warningStyle.Render("  " + line + "\n"))
			} else if strings.Contains(line, "Update complete") || strings.Contains(line, "successfully") {
				sb.WriteString(successStyle.Render("  " + line + "\n"))
			} else {
				sb.WriteString(outputStyle.Render("  " + line + "\n"))
			}
		}
	}

	// Prompt
	sb.WriteString("\n")
	if m.Done {
		sb.WriteString(stepContent.Render("Press Enter to exit..."))
	} else if m.CurrentStep == StepConfirm || m.CurrentStep == StepRestart {
		sb.WriteString(stepContent.Render("Press Enter to continue..."))
	} else if m.CurrentStep == StepCheck || m.CurrentStep == StepApply {
		sb.WriteString(stepContent.Render("Press Enter to check for updates..."))
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
			sb.WriteString(stepActive.Render("[→] " + label))
		} else {
			sb.WriteString(stepPending.Render("[ ] " + label))
		}
		sb.WriteString("  ")
	}

	return sb.String()
}

// Additional styles
var (
	changelogStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).MarginLeft(2)
	versionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	outputLogStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	outputStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	warningStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
