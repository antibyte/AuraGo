package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (c *Config) FindProvider(id string) *ProviderEntry {
	for i := range c.Providers {
		if c.Providers[i].ID == id {
			return &c.Providers[i]
		}
	}
	// Synthetic provider for Google Workspace OAuth
	if id == "google_workspace" && c.GoogleWorkspace.ClientID != "" {
		secret := c.GoogleWorkspace.ClientSecret
		scopes := c.googleWorkspaceOAuthScopes()
		c.gwProvider = ProviderEntry{
			ID:                "google_workspace",
			Name:              "Google Workspace",
			AuthType:          "oauth2",
			OAuthAuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			OAuthTokenURL:     "https://oauth2.googleapis.com/token",
			OAuthClientID:     c.GoogleWorkspace.ClientID,
			OAuthClientSecret: secret,
			OAuthScopes:       scopes,
		}
		return &c.gwProvider
	}
	return nil
}

func (c *Config) googleWorkspaceOAuthScopes() string {
	scopes := []string{}
	gw := c.GoogleWorkspace
	if gw.Gmail || gw.GmailSend {
		if gw.GmailSend {
			scopes = append(scopes, "https://www.googleapis.com/auth/gmail.modify", "https://www.googleapis.com/auth/gmail.send")
		} else {
			scopes = append(scopes, "https://www.googleapis.com/auth/gmail.readonly")
		}
	}
	if gw.Calendar || gw.CalendarWrite {
		if gw.CalendarWrite {
			scopes = append(scopes, "https://www.googleapis.com/auth/calendar")
		} else {
			scopes = append(scopes, "https://www.googleapis.com/auth/calendar.readonly")
		}
	}
	if gw.Drive {
		scopes = append(scopes, "https://www.googleapis.com/auth/drive.readonly")
	}
	if gw.Docs || gw.DocsWrite {
		if gw.DocsWrite {
			scopes = append(scopes, "https://www.googleapis.com/auth/documents")
		} else {
			scopes = append(scopes, "https://www.googleapis.com/auth/documents.readonly")
		}
	}
	if gw.Sheets || gw.SheetsWrite {
		if gw.SheetsWrite {
			scopes = append(scopes, "https://www.googleapis.com/auth/spreadsheets")
		} else {
			scopes = append(scopes, "https://www.googleapis.com/auth/spreadsheets.readonly")
		}
	}
	if len(scopes) == 0 {
		// Default minimal scope set
		scopes = append(scopes, "https://www.googleapis.com/auth/gmail.readonly",
			"https://www.googleapis.com/auth/calendar.readonly",
			"https://www.googleapis.com/auth/drive.readonly")
	}
	return strings.Join(scopes, " ")
}

// migrateBudgetModelsToProviders copies legacy budget.models entries into
// matching providers (by model name). Already-present entries are skipped.
// This is a one-way read-only migration: budget.models is never modified.
func (c *Config) migrateBudgetModelsToProviders() {
	if len(c.Budget.Models) == 0 || len(c.Providers) == 0 {
		return
	}

	for _, bm := range c.Budget.Models {
		lowerName := strings.ToLower(bm.Name)
		// Find the provider whose default model matches
		var target *ProviderEntry
		for i := range c.Providers {
			if strings.ToLower(c.Providers[i].Model) == lowerName {
				target = &c.Providers[i]
				break
			}
		}
		if target == nil {
			continue
		}

		// Skip if this model already exists in the provider's Models list
		alreadyExists := false
		for _, pm := range target.Models {
			if strings.ToLower(pm.Name) == lowerName {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			target.Models = append(target.Models, bm)
		}
	}
}

// FindEmailAccount returns the EmailAccount with the given ID, or nil.
func (c *Config) FindEmailAccount(id string) *EmailAccount {
	for i := range c.EmailAccounts {
		if c.EmailAccounts[i].ID == id {
			return &c.EmailAccounts[i]
		}
	}
	return nil
}

// DefaultEmailAccount returns the first non-disabled EmailAccount, or nil if none are active.
func (c *Config) DefaultEmailAccount() *EmailAccount {
	for i := range c.EmailAccounts {
		if !c.EmailAccounts[i].Disabled {
			return &c.EmailAccounts[i]
		}
	}
	return nil
}

// MigrateEmailAccounts migrates the legacy single Email config into the
// EmailAccounts slice. Called once at startup during ApplyDefaults.
func (c *Config) MigrateEmailAccounts() {
	// If email_accounts already populated, nothing to migrate
	if len(c.EmailAccounts) > 0 {
		return
	}
	// Check if legacy email section has data worth migrating
	if c.Email.Username == "" && c.Email.IMAPHost == "" && c.Email.SMTPHost == "" {
		return
	}
	// Build an account from the legacy fields
	acct := EmailAccount{
		ID:            "default",
		Name:          "Default",
		IMAPHost:      c.Email.IMAPHost,
		IMAPPort:      c.Email.IMAPPort,
		SMTPHost:      c.Email.SMTPHost,
		SMTPPort:      c.Email.SMTPPort,
		Username:      c.Email.Username,
		Password:      c.Email.Password,
		FromAddress:   c.Email.FromAddress,
		WatchEnabled:  c.Email.WatchEnabled,
		WatchInterval: c.Email.WatchInterval,
		WatchFolder:   c.Email.WatchFolder,
	}
	c.EmailAccounts = append(c.EmailAccounts, acct)
}

// ResolveProviders populates the resolved (yaml:"-") fields on every LLM slot
// from the corresponding ProviderEntry.  It also handles legacy migration: if
// the Providers list is empty but inline fields exist (old-format config), it
// auto-creates provider entries and sets the references.
func (c *Config) ResolveProviders() {
	c.migrateInlineProviders()
	c.migrateBudgetModelsToProviders()

	// ── LLM ──
	if p := c.FindProvider(c.LLM.Provider); p != nil {
		c.LLM.ProviderType = p.Type
		c.LLM.BaseURL = p.BaseURL
		c.LLM.APIKey = p.APIKey
		c.LLM.Model = p.Model
		c.LLM.AccountID = p.AccountID
	} else if c.LLM.LegacyAPIKey != "" {
		// Legacy fallback: use inline fields from old config format
		c.LLM.BaseURL = c.LLM.LegacyURL
		c.LLM.APIKey = c.LLM.LegacyAPIKey
		c.LLM.Model = c.LLM.LegacyModel
		c.LLM.ProviderType = c.LLM.Provider // old value is the type string
	}

	// ── FallbackLLM ──
	if p := c.FindProvider(c.FallbackLLM.Provider); p != nil {
		c.FallbackLLM.ProviderType = p.Type
		c.FallbackLLM.BaseURL = p.BaseURL
		c.FallbackLLM.APIKey = p.APIKey
		c.FallbackLLM.Model = p.Model
		c.FallbackLLM.AccountID = p.AccountID
	} else if c.FallbackLLM.LegacyAPIKey != "" {
		c.FallbackLLM.BaseURL = c.FallbackLLM.LegacyURL
		c.FallbackLLM.APIKey = c.FallbackLLM.LegacyAPIKey
		c.FallbackLLM.Model = c.FallbackLLM.LegacyModel
	}

	// ── Vision ── (falls back to main LLM if provider empty)
	if c.Vision.Provider != "" {
		if p := c.FindProvider(c.Vision.Provider); p != nil {
			c.Vision.ProviderType = p.Type
			c.Vision.BaseURL = p.BaseURL
			c.Vision.APIKey = p.APIKey
			c.Vision.Model = p.Model
		} else if c.Vision.LegacyAPIKey != "" || c.Vision.LegacyURL != "" {
			c.Vision.BaseURL = c.Vision.LegacyURL
			c.Vision.APIKey = c.Vision.LegacyAPIKey
			c.Vision.Model = c.Vision.LegacyModel
		}
	}
	if c.Vision.APIKey == "" {
		c.Vision.APIKey = c.LLM.APIKey
	}
	if c.Vision.BaseURL == "" {
		c.Vision.BaseURL = c.LLM.BaseURL
	}

	// ── Whisper ── (falls back to main LLM if provider empty)
	if c.Whisper.Provider != "" {
		if p := c.FindProvider(c.Whisper.Provider); p != nil {
			c.Whisper.ProviderType = p.Type
			c.Whisper.BaseURL = p.BaseURL
			c.Whisper.APIKey = p.APIKey
			c.Whisper.Model = p.Model
		} else if c.Whisper.LegacyAPIKey != "" || c.Whisper.LegacyURL != "" {
			c.Whisper.BaseURL = c.Whisper.LegacyURL
			c.Whisper.APIKey = c.Whisper.LegacyAPIKey
			c.Whisper.Model = c.Whisper.LegacyModel
		}
	}
	if c.Whisper.APIKey == "" {
		c.Whisper.APIKey = c.LLM.APIKey
	}
	if c.Whisper.BaseURL == "" {
		c.Whisper.BaseURL = c.LLM.BaseURL
	}

	// ── Embeddings ── ("disabled" is a special value, not a provider ID)
	if c.Embeddings.Provider != "" && c.Embeddings.Provider != "disabled" {
		if p := c.FindProvider(c.Embeddings.Provider); p != nil {
			c.Embeddings.ProviderType = p.Type
			c.Embeddings.BaseURL = p.BaseURL
			c.Embeddings.APIKey = p.APIKey
			c.Embeddings.Model = p.Model
		} else if c.Embeddings.LegacyAPIKey != "" {
			c.Embeddings.APIKey = c.Embeddings.LegacyAPIKey
		}
	}
	// Auto-wire local Ollama embeddings when enabled and no explicit provider is set
	if c.Embeddings.LocalOllama.Enabled && (c.Embeddings.Provider == "" || c.Embeddings.Provider == "disabled") {
		port := c.Embeddings.LocalOllama.ContainerPort
		if port <= 0 {
			port = 11435
		}
		model := c.Embeddings.LocalOllama.Model
		if model == "" {
			model = "nomic-embed-text"
		}
		c.Embeddings.Provider = "local-ollama-embeddings"
		c.Embeddings.ProviderType = "ollama"
		c.Embeddings.BaseURL = fmt.Sprintf("http://127.0.0.1:%d/v1", port)
		c.Embeddings.APIKey = "ollama" // Ollama ignores auth but field must be non-empty
		c.Embeddings.Model = model
	}
	if c.Embeddings.APIKey == "" {
		c.Embeddings.APIKey = c.LLM.APIKey
	}

	// ── CoAgents.LLM ── (falls back to main LLM if provider empty)
	if c.CoAgents.LLM.Provider != "" {
		if p := c.FindProvider(c.CoAgents.LLM.Provider); p != nil {
			c.CoAgents.LLM.ProviderType = p.Type
			c.CoAgents.LLM.BaseURL = p.BaseURL
			c.CoAgents.LLM.APIKey = p.APIKey
			c.CoAgents.LLM.Model = p.Model
		} else if c.CoAgents.LLM.LegacyAPIKey != "" || c.CoAgents.LLM.LegacyURL != "" {
			c.CoAgents.LLM.BaseURL = c.CoAgents.LLM.LegacyURL
			c.CoAgents.LLM.APIKey = c.CoAgents.LLM.LegacyAPIKey
			c.CoAgents.LLM.Model = c.CoAgents.LLM.LegacyModel
		}
	}
	if c.CoAgents.LLM.APIKey == "" {
		c.CoAgents.LLM.APIKey = c.LLM.APIKey
	}
	if c.CoAgents.LLM.BaseURL == "" {
		c.CoAgents.LLM.BaseURL = c.LLM.BaseURL
	}
	if c.CoAgents.LLM.Model == "" {
		c.CoAgents.LLM.Model = c.LLM.Model
	}

	// ── Personality V2 ── (falls back to main LLM if provider empty)
	if c.Agent.PersonalityV2Provider != "" {
		if p := c.FindProvider(c.Agent.PersonalityV2Provider); p != nil {
			c.Agent.PersonalityV2ProviderType = p.Type
			c.Agent.PersonalityV2ResolvedURL = p.BaseURL
			c.Agent.PersonalityV2ResolvedKey = p.APIKey
			c.Agent.PersonalityV2ResolvedModel = p.Model
		}
	}
	// Legacy fallback: use inline fields if provider ref resolved nothing
	if c.Agent.PersonalityV2ResolvedModel == "" && c.Agent.PersonalityV2Model != "" {
		c.Agent.PersonalityV2ResolvedModel = c.Agent.PersonalityV2Model
	}
	if c.Agent.PersonalityV2ResolvedURL == "" && c.Agent.PersonalityV2URL != "" {
		c.Agent.PersonalityV2ResolvedURL = c.Agent.PersonalityV2URL
	}
	if c.Agent.PersonalityV2ResolvedKey == "" && c.Agent.PersonalityV2APIKey != "" {
		c.Agent.PersonalityV2ResolvedKey = c.Agent.PersonalityV2APIKey
	}

	// ── WebScraper summary ── (falls back to main LLM if provider empty)
	if c.Tools.WebScraper.SummaryProvider != "" {
		if p := c.FindProvider(c.Tools.WebScraper.SummaryProvider); p != nil {
			c.Tools.WebScraper.SummaryBaseURL = p.BaseURL
			c.Tools.WebScraper.SummaryAPIKey = p.APIKey
			c.Tools.WebScraper.SummaryModel = p.Model
		}
	}
	if c.Tools.WebScraper.SummaryAPIKey == "" {
		c.Tools.WebScraper.SummaryAPIKey = c.LLM.APIKey
	}
	if c.Tools.WebScraper.SummaryBaseURL == "" {
		c.Tools.WebScraper.SummaryBaseURL = c.LLM.BaseURL
	}
	if c.Tools.WebScraper.SummaryModel == "" {
		c.Tools.WebScraper.SummaryModel = c.LLM.Model
	}

	// ── Wikipedia summary ── (falls back to main LLM if provider empty)
	if c.Tools.Wikipedia.SummaryProvider != "" {
		if p := c.FindProvider(c.Tools.Wikipedia.SummaryProvider); p != nil {
			c.Tools.Wikipedia.SummaryBaseURL = p.BaseURL
			c.Tools.Wikipedia.SummaryAPIKey = p.APIKey
			c.Tools.Wikipedia.SummaryModel = p.Model
		}
	}
	if c.Tools.Wikipedia.SummaryAPIKey == "" {
		c.Tools.Wikipedia.SummaryAPIKey = c.LLM.APIKey
	}
	if c.Tools.Wikipedia.SummaryBaseURL == "" {
		c.Tools.Wikipedia.SummaryBaseURL = c.LLM.BaseURL
	}
	if c.Tools.Wikipedia.SummaryModel == "" {
		c.Tools.Wikipedia.SummaryModel = c.LLM.Model
	}

	// ── DDG Search summary ── (falls back to main LLM if provider empty)
	if c.Tools.DDGSearch.SummaryProvider != "" {
		if p := c.FindProvider(c.Tools.DDGSearch.SummaryProvider); p != nil {
			c.Tools.DDGSearch.SummaryBaseURL = p.BaseURL
			c.Tools.DDGSearch.SummaryAPIKey = p.APIKey
			c.Tools.DDGSearch.SummaryModel = p.Model
		}
	}
	if c.Tools.DDGSearch.SummaryAPIKey == "" {
		c.Tools.DDGSearch.SummaryAPIKey = c.LLM.APIKey
	}
	if c.Tools.DDGSearch.SummaryBaseURL == "" {
		c.Tools.DDGSearch.SummaryBaseURL = c.LLM.BaseURL
	}
	if c.Tools.DDGSearch.SummaryModel == "" {
		c.Tools.DDGSearch.SummaryModel = c.LLM.Model
	}

	// ── PDF Extractor summary ── (falls back to main LLM if provider empty)
	if c.Tools.PDFExtractor.SummaryProvider != "" {
		if p := c.FindProvider(c.Tools.PDFExtractor.SummaryProvider); p != nil {
			c.Tools.PDFExtractor.SummaryBaseURL = p.BaseURL
			c.Tools.PDFExtractor.SummaryAPIKey = p.APIKey
			c.Tools.PDFExtractor.SummaryModel = p.Model
		}
	}
	if c.Tools.PDFExtractor.SummaryAPIKey == "" {
		c.Tools.PDFExtractor.SummaryAPIKey = c.LLM.APIKey
	}
	if c.Tools.PDFExtractor.SummaryBaseURL == "" {
		c.Tools.PDFExtractor.SummaryBaseURL = c.LLM.BaseURL
	}
	if c.Tools.PDFExtractor.SummaryModel == "" {
		c.Tools.PDFExtractor.SummaryModel = c.LLM.Model
	}

	// ── Memory Analysis ── (falls back to main LLM if provider empty)
	if c.MemoryAnalysis.Provider != "" {
		if p := c.FindProvider(c.MemoryAnalysis.Provider); p != nil {
			c.MemoryAnalysis.ProviderType = p.Type
			c.MemoryAnalysis.BaseURL = p.BaseURL
			c.MemoryAnalysis.APIKey = p.APIKey
			if c.MemoryAnalysis.Model == "" {
				c.MemoryAnalysis.ResolvedModel = p.Model
			} else {
				c.MemoryAnalysis.ResolvedModel = c.MemoryAnalysis.Model
			}
		}
	}
	if c.MemoryAnalysis.APIKey == "" {
		c.MemoryAnalysis.APIKey = c.LLM.APIKey
	}
	if c.MemoryAnalysis.BaseURL == "" {
		c.MemoryAnalysis.BaseURL = c.LLM.BaseURL
	}
	if c.MemoryAnalysis.ResolvedModel == "" {
		c.MemoryAnalysis.ResolvedModel = c.LLM.Model
	}
	if c.MemoryAnalysis.ProviderType == "" {
		c.MemoryAnalysis.ProviderType = c.LLM.ProviderType
	}

	// ── LLM Guardian ── (falls back to main LLM if provider empty)
	if c.LLMGuardian.Provider != "" {
		if p := c.FindProvider(c.LLMGuardian.Provider); p != nil {
			c.LLMGuardian.ProviderType = p.Type
			c.LLMGuardian.BaseURL = p.BaseURL
			c.LLMGuardian.APIKey = p.APIKey
			if c.LLMGuardian.Model == "" {
				c.LLMGuardian.ResolvedModel = p.Model
			} else {
				c.LLMGuardian.ResolvedModel = c.LLMGuardian.Model
			}
		}
	}
	if c.LLMGuardian.APIKey == "" {
		c.LLMGuardian.APIKey = c.LLM.APIKey
	}
	if c.LLMGuardian.BaseURL == "" {
		c.LLMGuardian.BaseURL = c.LLM.BaseURL
	}
	if c.LLMGuardian.ResolvedModel == "" {
		c.LLMGuardian.ResolvedModel = c.LLM.Model
	}
	if c.LLMGuardian.ProviderType == "" {
		c.LLMGuardian.ProviderType = c.LLM.ProviderType
	}

	// ── Image Generation ── (no fallback — must be explicitly configured)
	if c.ImageGeneration.Provider != "" {
		if p := c.FindProvider(c.ImageGeneration.Provider); p != nil {
			c.ImageGeneration.ProviderType = p.Type
			c.ImageGeneration.BaseURL = p.BaseURL
			c.ImageGeneration.APIKey = p.APIKey
			if c.ImageGeneration.Model == "" {
				c.ImageGeneration.ResolvedModel = p.Model
			} else {
				c.ImageGeneration.ResolvedModel = c.ImageGeneration.Model
			}
		}
	}
}

// ApplyOAuthTokens reads stored OAuth2 access tokens from the vault and injects
// them into the resolved APIKey fields of providers that use auth_type "oauth2".
// Call this after ResolveProviders() whenever a vault is available.
func (c *Config) ApplyOAuthTokens(vault SecretReader) {
	if vault == nil {
		return
	}
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.AuthType != "oauth2" {
			continue
		}
		raw, err := vault.ReadSecret("oauth_" + p.ID)
		if err != nil || raw == "" {
			continue
		}
		var tok OAuthToken
		if err := json.Unmarshal([]byte(raw), &tok); err != nil {
			continue
		}
		if tok.AccessToken == "" {
			continue
		}
		// Inject the access token as the API key for this provider
		p.APIKey = tok.AccessToken
	}

	// Re-resolve: copy updated APIKey from providers into the resolved slot fields.
	// We only overwrite slots that reference an oauth2 provider.
	applyIfOAuth := func(providerID string, target *string) {
		p := c.FindProvider(providerID)
		if p != nil && p.AuthType == "oauth2" && p.APIKey != "" {
			*target = p.APIKey
		}
	}
	applyIfOAuth(c.LLM.Provider, &c.LLM.APIKey)
	applyIfOAuth(c.FallbackLLM.Provider, &c.FallbackLLM.APIKey)
	applyIfOAuth(c.Vision.Provider, &c.Vision.APIKey)
	applyIfOAuth(c.Whisper.Provider, &c.Whisper.APIKey)
	applyIfOAuth(c.Embeddings.Provider, &c.Embeddings.APIKey)
	applyIfOAuth(c.CoAgents.LLM.Provider, &c.CoAgents.LLM.APIKey)
	applyIfOAuth(c.Agent.PersonalityV2Provider, &c.Agent.PersonalityV2ResolvedKey)

	// ── Google Workspace OAuth token ──
	if raw, err := vault.ReadSecret("oauth_google_workspace"); err == nil && raw != "" {
		var tok OAuthToken
		if err := json.Unmarshal([]byte(raw), &tok); err == nil {
			c.GoogleWorkspace.AccessToken = tok.AccessToken
			c.GoogleWorkspace.RefreshToken = tok.RefreshToken
			c.GoogleWorkspace.TokenExpiry = tok.Expiry
		}
	}
}

// ApplyVaultSecrets populates all vault-only secret fields from the vault.
// Must be called after Load() and before the config is used for the first time.
// After calling this, ResolveProviders() should be called again to propagate
// provider API keys into the resolved LLM/Vision/etc. slots.
func (c *Config) ApplyVaultSecrets(vault SecretReader) {
	if vault == nil {
		return
	}
	apply := func(vaultKey string, target *string) {
		if v, err := vault.ReadSecret(vaultKey); err == nil && v != "" {
			*target = v
		}
	}

	// ── Provider secrets ──
	for i := range c.Providers {
		p := &c.Providers[i]
		apply("provider_"+p.ID+"_api_key", &p.APIKey)
		apply("provider_"+p.ID+"_oauth_client_secret", &p.OAuthClientSecret)
	}

	// ── Telegram / Discord ──
	apply("telegram_bot_token", &c.Telegram.BotToken)
	apply("discord_bot_token", &c.Discord.BotToken)

	// ── MeshCentral ──
	apply("meshcentral_password", &c.MeshCentral.Password)
	apply("meshcentral_token", &c.MeshCentral.LoginToken)

	// ── Tailscale / Ansible ──
	apply("tailscale_api_key", &c.Tailscale.APIKey)
	apply("ansible_token", &c.Ansible.Token)

	// ── API keys ──
	apply("virustotal_api_key", &c.VirusTotal.APIKey)
	apply("brave_search_api_key", &c.BraveSearch.APIKey)
	apply("tts_elevenlabs_api_key", &c.TTS.ElevenLabs.APIKey)

	// ── Notifications ──
	apply("ntfy_token", &c.Notifications.Ntfy.Token)
	apply("pushover_user_key", &c.Notifications.Pushover.UserKey)
	apply("pushover_app_token", &c.Notifications.Pushover.AppToken)

	// ── Auth ──
	apply("auth_password_hash", &c.Auth.PasswordHash)
	apply("auth_session_secret", &c.Auth.SessionSecret)
	apply("auth_totp_secret", &c.Auth.TOTPSecret)

	// ── Existing vault-only fields ──
	apply("home_assistant_access_token", &c.HomeAssistant.AccessToken)
	apply("webdav_password", &c.WebDAV.Password)
	apply("koofr_password", &c.Koofr.AppPassword)
	apply("proxmox_secret", &c.Proxmox.Secret)
	apply("github_token", &c.GitHub.Token)
	apply("rocketchat_auth_token", &c.RocketChat.AuthToken)
	apply("mqtt_password", &c.MQTT.Password)
	apply("adguard_password", &c.AdGuard.Password)

	// ── Paperless-ngx ──
	apply("paperless_ngx_api_token", &c.PaperlessNGX.APIToken)

	// ── Homepage deploy secrets ──
	apply("homepage_deploy_password", &c.Homepage.DeployPassword)
	apply("homepage_deploy_key", &c.Homepage.DeployKey)

	// ── Netlify ──
	apply("netlify_token", &c.Netlify.Token)

	// ── Google Workspace ──
	apply("google_workspace_client_secret", &c.GoogleWorkspace.ClientSecret)

	// ── Email account passwords ──
	apply("email_password", &c.Email.Password)
	for i := range c.EmailAccounts {
		a := &c.EmailAccounts[i]
		apply("email_"+a.ID+"_password", &a.Password)
	}
}

// migrateInlineProviders auto-creates provider entries from old-format config
// files that use inline base_url/api_key/model fields.  This is called once
// during Load() and ensures all resolved fields are populated.
func (c *Config) migrateInlineProviders() {
	if len(c.Providers) > 0 {
		return // new-format config — no migration needed
	}

	seen := map[string]bool{}

	addProvider := func(id, name, typ, baseURL, apiKey, model string) string {
		if id == "" || seen[id] {
			return id
		}
		seen[id] = true
		c.Providers = append(c.Providers, ProviderEntry{
			ID: id, Name: name, Type: typ,
			BaseURL: baseURL, APIKey: apiKey, Model: model,
		})
		return id
	}

	inferType := func(baseURL, providerHint string) string {
		if providerHint != "" {
			return strings.ToLower(providerHint)
		}
		lower := strings.ToLower(baseURL)
		switch {
		case strings.Contains(lower, "openrouter"):
			return "openrouter"
		case strings.Contains(lower, "anthropic"):
			return "anthropic"
		case strings.Contains(lower, "googleapis") || strings.Contains(lower, "generativelanguage"):
			return "google"
		case strings.Contains(lower, "11434"):
			return "ollama"
		default:
			return "openai"
		}
	}

	// Migrate main LLM (always present in old configs)
	// The old LLM.Provider was a type string like "openrouter", not an ID
	oldLLMType := c.LLM.Provider // save before overwriting
	if oldLLMType == "" {
		oldLLMType = inferType(c.LLM.LegacyURL, "")
	}
	c.LLM.Provider = addProvider("main", "Haupt-LLM", oldLLMType, c.LLM.LegacyURL, c.LLM.LegacyAPIKey, c.LLM.LegacyModel)

	// Migrate FallbackLLM
	if c.FallbackLLM.Enabled && c.FallbackLLM.LegacyURL != "" {
		fbType := inferType(c.FallbackLLM.LegacyURL, "")
		c.FallbackLLM.Provider = addProvider("fallback", "Fallback-LLM", fbType,
			c.FallbackLLM.LegacyURL, c.FallbackLLM.LegacyAPIKey, c.FallbackLLM.LegacyModel)
	}

	// Migrate Vision
	if c.Vision.LegacyURL != "" || c.Vision.LegacyModel != "" {
		vURL := c.Vision.LegacyURL
		if vURL == "" {
			vURL = c.LLM.LegacyURL
		}
		vKey := c.Vision.LegacyAPIKey
		if vKey == "" {
			vKey = c.LLM.LegacyAPIKey
		}
		vType := inferType(vURL, c.Vision.Provider)
		// Only create separate entry if different from main
		if vURL != c.LLM.LegacyURL || vKey != c.LLM.LegacyAPIKey || c.Vision.LegacyModel != c.LLM.LegacyModel {
			c.Vision.Provider = addProvider("vision", "Vision", vType, vURL, vKey, c.Vision.LegacyModel)
		} else {
			c.Vision.Provider = "main"
		}
	}

	// Migrate Whisper
	if c.Whisper.LegacyURL != "" || c.Whisper.LegacyModel != "" {
		wURL := c.Whisper.LegacyURL
		if wURL == "" {
			wURL = c.LLM.LegacyURL
		}
		wKey := c.Whisper.LegacyAPIKey
		if wKey == "" {
			wKey = c.LLM.LegacyAPIKey
		}
		wType := inferType(wURL, "")
		// Migrate old provider field as mode if it's a mode-like value
		oldProv := strings.ToLower(c.Whisper.Provider)
		if oldProv == "multimodal" || oldProv == "local" {
			c.Whisper.Mode = oldProv
		} else if oldProv == "openai" || oldProv == "openrouter" || oldProv == "ollama" {
			// Old provider type — will be migrated as provider ref
			if c.Whisper.Mode == "" {
				c.Whisper.Mode = "whisper"
			}
		}
		if wURL != c.LLM.LegacyURL || wKey != c.LLM.LegacyAPIKey || c.Whisper.LegacyModel != c.LLM.LegacyModel {
			c.Whisper.Provider = addProvider("whisper", "Whisper / STT", wType, wURL, wKey, c.Whisper.LegacyModel)
		} else {
			c.Whisper.Provider = "main"
		}
	}

	// Migrate Embeddings
	oldEmbProv := c.Embeddings.Provider
	switch oldEmbProv {
	case "internal":
		// internal means: use main LLM provider + InternalModel
		embModel := c.Embeddings.InternalModel
		if embModel == "" {
			embModel = "text-embedding-3-small"
		}
		// Create a dedicated embedding provider (same URL/key as main but different model)
		c.Embeddings.Provider = addProvider("embeddings", "Embeddings", oldLLMType, c.LLM.LegacyURL, c.LLM.LegacyAPIKey, embModel)
	case "external":
		embKey := c.Embeddings.LegacyAPIKey
		if embKey == "" || embKey == "dummy_key" {
			embKey = c.LLM.LegacyAPIKey
		}
		embModel := c.Embeddings.ExternalModel
		embURL := c.Embeddings.ExternalURL
		eType := inferType(embURL, "")
		c.Embeddings.Provider = addProvider("embeddings", "Embeddings", eType, embURL, embKey, embModel)
	case "disabled":
		// keep as "disabled" — not a provider ref
	default:
		c.Embeddings.Provider = "disabled"
	}

	// Migrate CoAgents.LLM
	if c.CoAgents.LLM.LegacyURL != "" || c.CoAgents.LLM.LegacyModel != "" {
		caURL := c.CoAgents.LLM.LegacyURL
		if caURL == "" {
			caURL = c.LLM.LegacyURL
		}
		caKey := c.CoAgents.LLM.LegacyAPIKey
		if caKey == "" {
			caKey = c.LLM.LegacyAPIKey
		}
		caModel := c.CoAgents.LLM.LegacyModel
		// Old provider field was a type string
		caOldType := c.CoAgents.LLM.Provider
		caType := inferType(caURL, caOldType)
		if caURL != c.LLM.LegacyURL || caKey != c.LLM.LegacyAPIKey || caModel != c.LLM.LegacyModel {
			c.CoAgents.LLM.Provider = addProvider("coagent", "Co-Agent LLM", caType, caURL, caKey, caModel)
		} else {
			c.CoAgents.LLM.Provider = "main"
		}
	} else if c.CoAgents.LLM.Provider == "" {
		c.CoAgents.LLM.Provider = "main"
	}

	// Migrate Personality V2
	if c.Agent.PersonalityV2URL != "" || c.Agent.PersonalityV2Model != "" {
		v2URL := c.Agent.PersonalityV2URL
		v2Key := c.Agent.PersonalityV2APIKey
		v2Model := c.Agent.PersonalityV2Model
		if v2URL != "" && (v2URL != c.LLM.LegacyURL || v2Key != c.LLM.LegacyAPIKey) {
			v2Type := inferType(v2URL, "")
			c.Agent.PersonalityV2Provider = addProvider("personality-v2", "Personality V2", v2Type, v2URL, v2Key, v2Model)
		}
		// If no separate URL, V2 uses main LLM provider (resolved in ResolveProviders)
	}
}
