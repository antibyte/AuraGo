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

	cmd, exists := registry[cmdName]
	if !exists {
		return "❌ Unbekannter Befehl. Tippe /help für eine Liste der Befehle.", true, nil
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
	return "🧹 Chat-Verlauf und Kurzzeitgedächtnis wurden gelöscht.", nil
}

func (c *ResetCommand) Help() string {
	return "Löscht den aktuellen Chat-Verlauf (Short-Term Memory)."
}

// HelpCommand lists all available commands.
type HelpCommand struct{}

func (c *HelpCommand) Execute(args []string, ctx Context) (string, error) {
	var sb strings.Builder
	sb.WriteString("📜 **Verfügbare Befehle:**\n\n")
	for name, cmd := range registry {
		sb.WriteString("• /" + name + ": " + cmd.Help() + "\n")
	}
	return sb.String(), nil
}

func (c *HelpCommand) Help() string {
	return "Zeigt diese Hilfe an."
}

// StopCommand shuts down the agent.
type StopCommand struct{}

func (c *StopCommand) Execute(args []string, ctx Context) (string, error) {
	agent.InterruptSession("default")
	return "🛑 AuraGo wurde angewiesen, die aktuelle Aktion zu unterbrechen.", nil
}

func (c *StopCommand) Help() string {
	return "Unterbricht die aktuelle Aktion des Agenten."
}

// RestartCommand restarts the agent.
type RestartCommand struct{}

func (c *RestartCommand) Execute(args []string, ctx Context) (string, error) {
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(42)
	}()
	return "🔄 AuraGo wird neu gestartet...", nil
}

func (c *RestartCommand) Help() string {
	return "Startet den AuraGo-Server neu."
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
			return "❌ Ungültiges Argument. Benutze `/debug on` oder `/debug off`.", nil
		}
	} else {
		// No argument: toggle
		enabled = agent.ToggleDebugMode()
	}

	if enabled {
		return "🔍 **Agent Debug-Modus aktiviert.** Der Agent meldet Fehler jetzt mit detaillierten Informationen.", nil
	}
	return "🔇 **Agent Debug-Modus deaktiviert.** Der Agent verhält sich normal.", nil
}

func (c *DebugCommand) Help() string {
	return "Aktiviert/deaktiviert den Agent-Debug-Modus (detaillierte Fehlermeldungen im System-Prompt)."
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
		sb.WriteString("🎭 **Verfügbare Persönlichkeiten:**\n\n")
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
				name := strings.TrimSuffix(f.Name(), ".md")
				activeMarker := ""
				if name == ctx.Cfg.Personality.CorePersonality {
					activeMarker = " ✅ (aktiv)"
				}
				sb.WriteString("• " + name + activeMarker + "\n")
			}
		}
		sb.WriteString("\nNutze `/personality <name>` zum Umstellen.")
		return sb.String(), nil
	}

	// Switch personality
	target := strings.ToLower(args[0])
	profilePath := filepath.Join(personalitiesDir, target+".md")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return "❌ Persönlichkeit '" + target + "' nicht gefunden.", nil
	}

	ctx.Cfg.Personality.CorePersonality = target
	configPath := ctx.Cfg.ConfigPath
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := ctx.Cfg.Save(configPath); err != nil {
		return "", err
	}

	return "🎭 Persönlichkeit auf **" + target + "** umgestellt. Die Änderung ist permanent.", nil
}

func (c *PersonalityCommand) Help() string {
	return "Listet Persönlichkeiten auf oder wechselt sie (/personality <name>)."
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
			return "❌ Invalid argument. Use `/voice on` or `/voice off`.", nil
		}
	} else {
		enabled = agent.ToggleVoiceMode()
	}

	if enabled {
		return "🔊 **Voice mode enabled.** Short replies will be spoken via TTS.", nil
	}
	return "🔇 **Voice mode disabled.** Audio will not auto-play.", nil
}

func (c *VoiceCommand) Help() string {
	return "Toggles voice output mode — when on, the agent uses TTS for short replies (/voice on|off)."
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
