package config

const (
	adaptivePromptBudgetPerTool        = 48
	adaptivePromptBudgetPerIntegration = 160
	adaptivePromptBudgetHardCap        = 4096
)

// CalculateAdaptiveSystemPromptTokenBudget returns the base system prompt token budget
// plus a small fixed surcharge for enabled tools and integrations when adaptive prompt
// budgeting is enabled. The result is capped to keep growth bounded.
func CalculateAdaptiveSystemPromptTokenBudget(cfg *Config) int {
	if cfg == nil {
		return 0
	}

	baseBudget := cfg.Agent.SystemPromptTokenBudget
	if baseBudget <= 0 {
		return 0
	}
	if !cfg.Agent.AdaptiveSystemPromptTokenBudget {
		return baseBudget
	}

	toolCount := countAdaptivePromptBudgetTools(cfg)
	integrationCount := countAdaptivePromptBudgetIntegrations(cfg)
	extraBudget := (toolCount * adaptivePromptBudgetPerTool) + (integrationCount * adaptivePromptBudgetPerIntegration)
	if extraBudget <= 0 {
		return baseBudget
	}

	maxExtra := adaptivePromptBudgetHardCap
	if halfBase := baseBudget / 2; halfBase > 0 && halfBase < maxExtra {
		maxExtra = halfBase
	}
	if maxExtra <= 0 {
		return baseBudget
	}
	if extraBudget > maxExtra {
		extraBudget = maxExtra
	}

	return baseBudget + extraBudget
}

func countAdaptivePromptBudgetTools(cfg *Config) int {
	if cfg == nil {
		return 0
	}

	count := 0
	flags := []bool{
		cfg.Tools.Memory.Enabled,
		cfg.Tools.KnowledgeGraph.Enabled,
		cfg.Tools.SecretsVault.Enabled,
		cfg.Tools.Scheduler.Enabled,
		cfg.Tools.Notes.Enabled,
		cfg.Tools.Missions.Enabled,
		cfg.Tools.StopProcess.Enabled,
		cfg.Tools.Inventory.Enabled,
		cfg.Tools.MemoryMaintenance.Enabled,
		cfg.Tools.Journal.Enabled,
		cfg.Tools.WOL.Enabled,
		cfg.Tools.WebScraper.Enabled,
		cfg.Tools.PDFExtractor.Enabled,
		cfg.Tools.DocumentCreator.Enabled,
		cfg.Tools.WebCapture.Enabled,
		cfg.Tools.NetworkPing.Enabled,
		cfg.Tools.NetworkScan.Enabled,
		cfg.Tools.FormAutomation.Enabled,
		cfg.Tools.UPnPScan.Enabled,
		cfg.Tools.Contacts.Enabled,
	}
	for _, enabled := range flags {
		if enabled {
			count++
		}
	}
	return count
}

func countAdaptivePromptBudgetIntegrations(cfg *Config) int {
	if cfg == nil {
		return 0
	}

	count := 0
	flags := []bool{
		cfg.Discord.Enabled,
		cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		cfg.HomeAssistant.Enabled,
		cfg.FritzBox.Enabled,
		cfg.Telnyx.Enabled,
		cfg.MeshCentral.Enabled,
		cfg.Docker.Enabled,
		cfg.WebDAV.Enabled,
		cfg.Koofr.Enabled,
		cfg.S3.Enabled,
		cfg.PaperlessNGX.Enabled,
		cfg.Proxmox.Enabled,
		cfg.Tailscale.Enabled,
		cfg.CloudflareTunnel.Enabled,
		cfg.Ansible.Enabled,
		cfg.GitHub.Enabled,
		cfg.Netlify.Enabled,
		cfg.AdGuard.Enabled,
		cfg.MQTT.Enabled,
		cfg.GoogleWorkspace.Enabled,
		cfg.OneDrive.Enabled,
		cfg.Jellyfin.Enabled,
		cfg.RemoteControl.Enabled,
		cfg.InvasionControl.Enabled,
		cfg.SQLConnections.Enabled,
		cfg.Webhooks.Enabled,
		cfg.N8n.Enabled,
		cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		cfg.Homepage.Enabled,
		cfg.CoAgents.Enabled,
		cfg.ImageGeneration.Enabled,
		(cfg.Embeddings.Provider != "" && cfg.Embeddings.Provider != "disabled"),
		cfg.Vision.Provider != "",
		cfg.Whisper.Provider != "" || cfg.Whisper.Mode != "",
		cfg.TTS.Provider != "" || cfg.TTS.Piper.Enabled,
	}
	for _, enabled := range flags {
		if enabled {
			count++
		}
	}
	return count
}
