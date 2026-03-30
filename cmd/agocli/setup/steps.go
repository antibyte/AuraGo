package setup

import "github.com/charmbracelet/lipgloss"

var (
	// Step styles
	stepActive    = lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")).Bold(true)
	stepDone      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	stepPending   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	stepError     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true)
	stepTitle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	stepContent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	headerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")).Bold(true)
	outputLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	outputStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
)

// StepInfo contains information about a step.
type StepInfo struct {
	Title   string
	Content string
}

// GetStepInfo returns information about a step.
func GetStepInfo(step Step) StepInfo {
	switch step {
	case StepWelcome:
		return StepInfo{
			Title:   "Welcome to AuraGo",
			Content: "This wizard will help you set up AuraGo for the first time.\n\nAuraGo is an autonomous AI agent for your home lab. It requires:\n• A configured LLM provider (OpenRouter, OpenAI, etc.)\n• Network access for API calls\n\nPress Enter to continue...",
		}
	case StepPrerequisites:
		return StepInfo{
			Title:   "Checking Prerequisites",
			Content: "Looking for resources.dat and verifying permissions...",
		}
	case StepExtract:
		return StepInfo{
			Title:   "Extracting Resources",
			Content: "Extracting embedded resources...",
		}
	case StepMasterKey:
		return StepInfo{
			Title:   "Generating Master Key",
			Content: "Creating encryption keys for secure credential storage...",
		}
	case StepConfig:
		return StepInfo{
			Title:   "Configuration",
			Content: "Setting up your configuration...",
		}
	case StepService:
		return StepInfo{
			Title:   "Installing Service",
			Content: "Installing the AuraGo system service...",
		}
	case StepStart:
		return StepInfo{
			Title:   "Starting AuraGo",
			Content: "Starting the AuraGo server...",
		}
	case StepSummary:
		return StepInfo{
			Title:   "Setup Complete!",
			Content: "AuraGo has been successfully installed and started.",
		}
	default:
		return StepInfo{Title: "Unknown", Content: ""}
	}
}

// StepLabels returns all step labels for progress display.
func StepLabels() []string {
	return []string{
		"Welcome",
		"Prerequisites",
		"Extract",
		"Master Key",
		"Config",
		"Service",
		"Start",
		"Summary",
	}
}
