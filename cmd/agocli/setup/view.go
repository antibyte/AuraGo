package setup

import (
	"strings"
)

// View renders the setup wizard.
func (m *Model) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(headerStyle.Render("AuraGo Setup Wizard"))
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
			} else {
				sb.WriteString(outputStyle.Render("  " + line + "\n"))
			}
		}
	}

	// Prompt
	sb.WriteString("\n")
	if m.Done {
		sb.WriteString(stepContent.Render("Press Enter to exit..."))
	} else if m.CurrentStep == StepWelcome {
		sb.WriteString(stepContent.Render("Press Enter to continue..."))
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
