package update

import "github.com/charmbracelet/lipgloss"

// StepInfo contains information about a step.
type StepInfo struct {
	Title   string
	Content string
}

// GetStepInfo returns information about a step.
func GetStepInfo(step Step) StepInfo {
	switch step {
	case StepCheck:
		return StepInfo{
			Title:   "Checking for Updates",
			Content: "Checking if a newer version of AuraGo is available...",
		}
	case StepChangelog:
		return StepInfo{
			Title:   "Changelog",
			Content: "Changes in the new version:",
		}
	case StepConfirm:
		return StepInfo{
			Title:   "Confirm Update",
			Content: "Are you sure you want to update AuraGo?\n\nThis will:\n• Download the latest version\n• Backup your data\n• Restart the service\n\nPress Enter to confirm or Ctrl+C to cancel...",
		}
	case StepApply:
		return StepInfo{
			Title:   "Applying Update",
			Content: "Running update script...",
		}
	case StepKeyMigrate:
		return StepInfo{
			Title:   "Security Recommendation",
			Content: "Your master key is currently stored in .env.\n\nFor better security, it is recommended to move it to /etc/aurago/master.key\n\nWould you like to migrate the key now?",
		}
	case StepRestart:
		return StepInfo{
			Title:   "Restart",
			Content: "Update complete! Would you like to restart AuraGo now?",
		}
	case StepSummary:
		return StepInfo{
			Title:   "Update Complete!",
			Content: "AuraGo has been successfully updated.",
		}
	default:
		return StepInfo{Title: "Unknown", Content: ""}
	}
}

// StepLabels returns all step labels for progress display.
func StepLabels() []string {
	return []string{
		"Check",
		"Changelog",
		"Confirm",
		"Apply",
		"Key Migrate",
		"Restart",
		"Summary",
	}
}

// Styles
var (
	headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")).Bold(true)
	stepActive  = lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")).Bold(true)
	stepDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	stepPending = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	stepTitle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	stepContent = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true)
)
