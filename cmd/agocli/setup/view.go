package setup

import (
	"fmt"
	"strings"
)

// View renders the setup wizard.
func (m *Model) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(headerStyle.Render("AuraGo Setup Wizard"))
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
	} else if m.Running {
		sb.WriteString(stepContent.Render(info.Content))
		sb.WriteString("\n")
	} else if m.CurrentStep == StepSummary && m.Done && m.Err == nil {
		// Summary with highlighted password
		sb.WriteString(m.renderSummaryBox())
		sb.WriteString("\n")
	} else if m.Done && m.Err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.Err)))
		sb.WriteString("\n")
	} else {
		sb.WriteString(stepContent.Render(info.Content))
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
			} else if strings.Contains(line, "✓") || strings.Contains(line, "successfully") {
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
	} else if m.CurrentStep == StepWelcome {
		sb.WriteString(stepContent.Render("Press Enter to start..."))
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
				sb.WriteString(stepError.Render("[✗] " + label))
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

func (m *Model) renderSummaryBox() string {
	var sb strings.Builder
	w := clamp(m.Width, 40, 80)
	border := strings.Repeat("═", w)

	sb.WriteString(successStyle.Render(border))
	sb.WriteString("\n")
	sb.WriteString(successStyle.Render("  Setup Complete!"))
	sb.WriteString("\n")
	sb.WriteString(successStyle.Render(border))
	sb.WriteString("\n\n")

	sb.WriteString(stepContent.Render("  Access URL:  "))
	sb.WriteString(keyStyle.Render(m.AccessURL))
	sb.WriteString("\n")

	if m.Password != "" {
		sb.WriteString(stepContent.Render("  Password:    "))
		sb.WriteString(keyStyle.Render(m.Password))
		sb.WriteString("\n")
		sb.WriteString(outputStyle.Render("  (also saved in firstpassword.txt)"))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(stepContent.Render("  Next steps:"))
	sb.WriteString("\n")
	sb.WriteString(stepContent.Render("  1. Open the Web UI at the URL above"))
	sb.WriteString("\n")
	sb.WriteString(stepContent.Render("  2. Configure your LLM provider in Settings"))
	sb.WriteString("\n")
	sb.WriteString(stepContent.Render("  3. Or use 'agocli' for terminal chat"))
	sb.WriteString("\n")

	return sb.String()
}

func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}
