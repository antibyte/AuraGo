package config

import "testing"

func TestCalculateAdaptiveSystemPromptTokenBudget_DisabledKeepsBase(t *testing.T) {
	cfg := &Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.AdaptiveSystemPromptTokenBudget = false
	cfg.Tools.Memory.Enabled = true
	cfg.Docker.Enabled = true

	got := CalculateAdaptiveSystemPromptTokenBudget(cfg)
	if got != 12000 {
		t.Fatalf("CalculateAdaptiveSystemPromptTokenBudget() = %d, want 12000", got)
	}
}

func TestCalculateAdaptiveSystemPromptTokenBudget_AddsToolAndIntegrationSurcharge(t *testing.T) {
	cfg := &Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.AdaptiveSystemPromptTokenBudget = true
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Docker.Enabled = true
	cfg.GitHub.Enabled = true

	got := CalculateAdaptiveSystemPromptTokenBudget(cfg)
	want := 12000 + (2 * adaptivePromptBudgetPerTool) + (2 * adaptivePromptBudgetPerIntegration)
	if got != want {
		t.Fatalf("CalculateAdaptiveSystemPromptTokenBudget() = %d, want %d", got, want)
	}
}

func TestCalculateAdaptiveSystemPromptTokenBudget_CapsExtraBudget(t *testing.T) {
	cfg := &Config{}
	cfg.Agent.SystemPromptTokenBudget = 4000
	cfg.Agent.AdaptiveSystemPromptTokenBudget = true

	cfg.Tools.Memory.Enabled = true
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.SecretsVault.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.StopProcess.Enabled = true
	cfg.Tools.Inventory.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Tools.WOL.Enabled = true
	cfg.Tools.WebScraper.Enabled = true
	cfg.Tools.PDFExtractor.Enabled = true
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.WebCapture.Enabled = true
	cfg.Tools.NetworkPing.Enabled = true
	cfg.Tools.NetworkScan.Enabled = true
	cfg.Tools.FormAutomation.Enabled = true
	cfg.Tools.UPnPScan.Enabled = true
	cfg.Tools.Contacts.Enabled = true

	cfg.Discord.Enabled = true
	cfg.Email.Enabled = true
	cfg.HomeAssistant.Enabled = true
	cfg.FritzBox.Enabled = true
	cfg.Telnyx.Enabled = true
	cfg.MeshCentral.Enabled = true
	cfg.Docker.Enabled = true
	cfg.WebDAV.Enabled = true
	cfg.Koofr.Enabled = true
	cfg.S3.Enabled = true
	cfg.PaperlessNGX.Enabled = true
	cfg.Proxmox.Enabled = true
	cfg.Tailscale.Enabled = true
	cfg.CloudflareTunnel.Enabled = true
	cfg.Ansible.Enabled = true
	cfg.GitHub.Enabled = true
	cfg.Netlify.Enabled = true
	cfg.AdGuard.Enabled = true
	cfg.MQTT.Enabled = true
	cfg.GoogleWorkspace.Enabled = true
	cfg.OneDrive.Enabled = true
	cfg.Jellyfin.Enabled = true
	cfg.RemoteControl.Enabled = true
	cfg.InvasionControl.Enabled = true
	cfg.SQLConnections.Enabled = true
	cfg.Webhooks.Enabled = true
	cfg.N8n.Enabled = true
	cfg.MCP.Enabled = true
	cfg.Agent.AllowMCP = true
	cfg.Homepage.Enabled = true
	cfg.CoAgents.Enabled = true
	cfg.ImageGeneration.Enabled = true
	cfg.Embeddings.Provider = "emb"
	cfg.Vision.Provider = "vision"
	cfg.Whisper.Mode = "whisper"
	cfg.TTS.Provider = "google"

	got := CalculateAdaptiveSystemPromptTokenBudget(cfg)
	want := 6000 // capped at base + base/2
	if got != want {
		t.Fatalf("CalculateAdaptiveSystemPromptTokenBudget() = %d, want %d", got, want)
	}
}
