package commands

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/warnings"
)

// Context provides dependencies to commands.
type Context struct {
	STM              *memory.SQLiteMemory
	HM               *memory.HistoryManager
	Vault            *security.Vault
	InventoryDB      *sql.DB
	BudgetTracker    *budget.Tracker
	Cfg              *config.Config
	PromptsDir       string
	WarningsRegistry *warnings.Registry
	Lang             string // UI language for i18n
}

// Command defines the interface for a slash command.
type Command interface {
	Execute(args []string, ctx Context) (string, error)
	Help() string
}

var registry = make(map[string]Command)

// Register adds a command to the registry.
func Register(name string, cmd Command) {
	registry[name] = cmd
}

// Handle processes the input if it's a command.
func Handle(input string, ctx Context) (string, bool, error) {
	if !strings.HasPrefix(input, "/") {
		return "", false, nil
	}

	parts := strings.Fields(input)
	cmdName := parts[0][1:] // Remove leading slash
	args := parts[1:]

	// Default to German if no language set
	lang := ctx.Lang
	if lang == "" {
		lang = "de"
	}

	cmd, exists := registry[cmdName]
	if !exists {
		return i18n.T(lang, "backend.cmd_unknown", cmdName), true, nil
	}

	result, err := cmd.Execute(args, ctx)
	return result, true, err
}

// ResetCommand clears the chat history.
type ResetCommand struct{}

func (c *ResetCommand) Execute(args []string, ctx Context) (string, error) {
	sessionID := "default"
	if err := ctx.STM.Clear(sessionID); err != nil {
		return "", err
	}
	if err := ctx.HM.Clear(); err != nil {
		return "", err
	}
	agent.ResetInnerVoiceState()
	return i18n.T(ctx.Lang, "backend.cmd_reset_success"), nil
}

func (c *ResetCommand) Help() string {
	return i18n.T("de", "backend.cmd_reset_help")
}

// HelpCommand lists all available commands.
type HelpCommand struct{}

func (c *HelpCommand) Execute(args []string, ctx Context) (string, error) {
	var sb strings.Builder
	sb.WriteString(i18n.T(ctx.Lang, "backend.cmd_help_header") + "\n\n")
	for name, cmd := range registry {
		sb.WriteString("• /" + name + ": " + cmd.Help() + "\n")
	}
	return sb.String(), nil
}

func (c *HelpCommand) Help() string {
	return i18n.T("de", "backend.cmd_help_help")
}

// StopCommand shuts down the agent.
type StopCommand struct{}

func (c *StopCommand) Execute(args []string, ctx Context) (string, error) {
	agent.InterruptSession("default")
	return i18n.T(ctx.Lang, "backend.cmd_stop_success"), nil
}

func (c *StopCommand) Help() string {
	return i18n.T("de", "backend.cmd_stop_help")
}

// RestartCommand restarts the agent.
type RestartCommand struct{}

func (c *RestartCommand) Execute(args []string, ctx Context) (string, error) {
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(42)
	}()
	return i18n.T(ctx.Lang, "backend.cmd_restart_success"), nil
}

func (c *RestartCommand) Help() string {
	return i18n.T("de", "backend.cmd_restart_help")
}

// DebugCommand toggles the agent's debug mode (extra debug instructions in the system prompt).
type DebugCommand struct{}

func (c *DebugCommand) Execute(args []string, ctx Context) (string, error) {
	var enabled bool
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "on", "1", "true":
			enabled = true
			agent.SetDebugMode(true)
		case "off", "0", "false":
			enabled = false
			agent.SetDebugMode(false)
		default:
			return i18n.T(ctx.Lang, "backend.cmd_debug_invalid"), nil
		}
	} else {
		// No argument: toggle
		enabled = agent.ToggleDebugMode()
	}

	if enabled {
		return i18n.T(ctx.Lang, "backend.cmd_debug_enabled"), nil
	}
	return i18n.T(ctx.Lang, "backend.cmd_debug_disabled"), nil
}

func (c *DebugCommand) Help() string {
	return i18n.T("de", "backend.cmd_debug_help")
}

// PersonalityCommand manages the agent's core personality.
type PersonalityCommand struct{}

func (c *PersonalityCommand) Execute(args []string, ctx Context) (string, error) {
	personalitiesDir := filepath.Join(ctx.PromptsDir, "personalities")

	if len(args) == 0 {
		// List personalities
		files, err := os.ReadDir(personalitiesDir)
		if err != nil {
			return "", err
		}

		var sb strings.Builder
		sb.WriteString(i18n.T(ctx.Lang, "backend.cmd_personality_header") + "\n\n")
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
				name := strings.TrimSuffix(f.Name(), ".md")
				activeMarker := ""
				if name == ctx.Cfg.Personality.CorePersonality {
					activeMarker = i18n.T(ctx.Lang, "backend.cmd_personality_active")
				}
				sb.WriteString("• " + name + activeMarker + "\n")
			}
		}
		sb.WriteString("\n" + i18n.T(ctx.Lang, "backend.cmd_personality_usage"))
		return sb.String(), nil
	}

	// Switch personality
	target := strings.ToLower(args[0])
	profilePath := filepath.Join(personalitiesDir, target+".md")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return i18n.T(ctx.Lang, "backend.cmd_personality_not_found", target), nil
	}

	ctx.Cfg.Personality.CorePersonality = target
	configPath := ctx.Cfg.ConfigPath
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := ctx.Cfg.Save(configPath); err != nil {
		return "", err
	}

	return i18n.T(ctx.Lang, "backend.cmd_personality_changed", target), nil
}

func (c *PersonalityCommand) Help() string {
	return i18n.T("de", "backend.cmd_personality_help")
}

// VoiceCommand toggles voice output mode (TTS auto-play / voice notes).
type VoiceCommand struct{}

func (c *VoiceCommand) Execute(args []string, ctx Context) (string, error) {
	var enabled bool
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "on", "1", "true":
			enabled = true
			agent.SetVoiceMode(true)
		case "off", "0", "false":
			enabled = false
			agent.SetVoiceMode(false)
		default:
			return i18n.T(ctx.Lang, "backend.cmd_voice_invalid"), nil
		}
	} else {
		enabled = agent.ToggleVoiceMode()
	}

	if enabled {
		return i18n.T(ctx.Lang, "backend.cmd_voice_enabled"), nil
	}
	return i18n.T(ctx.Lang, "backend.cmd_voice_disabled"), nil
}

func (c *VoiceCommand) Help() string {
	return i18n.T("de", "backend.cmd_voice_help")
}

func init() {
	Register("reset", &ResetCommand{})
	Register("stop", &StopCommand{})
	Register("restart", &RestartCommand{})
	Register("help", &HelpCommand{})
	Register("debug", &DebugCommand{})
	Register("personality", &PersonalityCommand{})
	Register("voice", &VoiceCommand{})
}
