package setup

import "github.com/charmbracelet/lipgloss"

var (
	// Step styles
	stepActive     = lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")).Bold(true)
	stepDone       = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	stepPending    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	stepError      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true)
	stepTitle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	stepContent    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#39FF14")).Bold(true)
	outputLogStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333"))
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	outputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	keyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true)
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
			Title: "Welcome to AuraGo",
			Content: "This wizard will help you set up AuraGo for the first time.\n\n" +
				"AuraGo is an autonomous AI agent for your home lab.\n" +
				"It requires a configured LLM provider (OpenRouter, OpenAI, etc.).\n\n" +
				"Press Enter to start the setup...",
		}
	case StepDependencies:
		return StepInfo{
			Title:   "System Dependencies",
			Content: "Checking and installing system dependencies...",
		}
	case StepMasterKey:
		return StepInfo{
			Title:   "Master Key",
			Content: "Generating AES-256 encryption key for secure credential storage...",
		}
	case StepExtract:
		return StepInfo{
			Title:   "Extract Resources",
			Content: "Extracting embedded resources (prompts, UI, skills)...",
		}
	case StepNetwork:
		return StepInfo{
			Title:   "Network Configuration",
			Content: "Configure how AuraGo should be accessible.",
		}
	case StepConfig:
		return StepInfo{
			Title:   "Configuration",
			Content: "Running config merger and initializing configuration...",
		}
	case StepPassword:
		return StepInfo{
			Title:   "Initial Password",
			Content: "Generating initial access password...",
		}
	case StepService:
		return StepInfo{
			Title:   "System Service",
			Content: "Install AuraGo as a system service for automatic startup.",
		}
	case StepStart:
		return StepInfo{
			Title:   "Starting AuraGo",
			Content: "Starting the AuraGo server...",
		}
	case StepSummary:
		return StepInfo{
			Title:   "Setup Complete!",
			Content: "AuraGo has been successfully installed and configured.",
		}
	default:
		return StepInfo{Title: "Unknown", Content: ""}
	}
}

// StepLabels returns all step labels for progress display.
func StepLabels() []string {
	return []string{
		"Welcome",
		"Dependencies",
		"Master Key",
		"Extract",
		"Network",
		"Config",
		"Password",
		"Service",
		"Start",
		"Summary",
	}
}
