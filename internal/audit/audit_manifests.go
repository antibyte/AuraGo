package audit

type Capability string

const (
	CapabilityRead    Capability = "read"
	CapabilityWrite   Capability = "write"
	CapabilityChange  Capability = "change"
	CapabilityDelete  Capability = "delete"
	CapabilityExecute Capability = "execute"
	CapabilityNetwork Capability = "network"
	CapabilityHost    Capability = "host"
)

type ToolPermission struct {
	Name         string
	ConfigGate   string
	Capabilities []Capability
	ReadOnlyGate string
}

func ToolPermissionMatrix() []ToolPermission {
	return []ToolPermission{
		{Name: "api_request", ConfigGate: "agent.allow_network_requests", Capabilities: []Capability{CapabilityNetwork}},
		{Name: "call_webhook", ConfigGate: "webhooks.enabled", Capabilities: []Capability{CapabilityNetwork}},
		{Name: "chromecast", ConfigGate: "tools.chromecast.enabled", Capabilities: []Capability{CapabilityRead, CapabilityChange, CapabilityNetwork}},
		{Name: "docker", ConfigGate: "docker.enabled", ReadOnlyGate: "docker.read_only", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityExecute, CapabilityHost}},
		{Name: "execute_python", ConfigGate: "agent.allow_python", Capabilities: []Capability{CapabilityExecute, CapabilityHost}},
		{Name: "execute_shell", ConfigGate: "agent.allow_shell", Capabilities: []Capability{CapabilityExecute, CapabilityHost}},
		{Name: "execute_sudo", ConfigGate: "agent.sudo_enabled", Capabilities: []Capability{CapabilityExecute, CapabilityHost}},
		{Name: "file_editor", ConfigGate: "agent.allow_filesystem_write", Capabilities: []Capability{CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityHost}},
		{Name: "filesystem", ConfigGate: "agent.allow_filesystem_write", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityHost}},
		{Name: "home_assistant", ConfigGate: "home_assistant.enabled", Capabilities: []Capability{CapabilityRead, CapabilityChange, CapabilityNetwork}},
		{Name: "homepage", ConfigGate: "homepage.enabled", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityExecute, CapabilityNetwork, CapabilityHost}},
		{Name: "invasion_control", ConfigGate: "invasion.enabled", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityExecute, CapabilityNetwork, CapabilityHost}},
		{Name: "json_editor", ConfigGate: "agent.allow_filesystem_write", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityHost}},
		{Name: "manage_outgoing_webhooks", ConfigGate: "webhooks.enabled", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityNetwork}},
		{Name: "meshcentral", ConfigGate: "meshcentral.enabled", ReadOnlyGate: "meshcentral.readonly", Capabilities: []Capability{CapabilityRead, CapabilityChange, CapabilityExecute, CapabilityNetwork}},
		{Name: "netlify", ConfigGate: "netlify.enabled", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityNetwork}},
		{Name: "remote_execution", ConfigGate: "agent.allow_remote_shell", Capabilities: []Capability{CapabilityExecute, CapabilityNetwork, CapabilityHost}},
		{Name: "secrets_vault", ConfigGate: "tools.secrets_vault.enabled", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityDelete}},
		{Name: "transfer_remote_file", ConfigGate: "agent.allow_remote_shell", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityNetwork, CapabilityHost}},
		{Name: "truenas", ConfigGate: "truenas.enabled", ReadOnlyGate: "truenas.read_only", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityNetwork}},
		{Name: "vercel", ConfigGate: "vercel.enabled", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityNetwork}},
		{Name: "video_download", ConfigGate: "tools.video_download.enabled", ReadOnlyGate: "tools.video_download.read_only", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityNetwork, CapabilityHost}},
		{Name: "xml_editor", ConfigGate: "agent.allow_filesystem_write", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityHost}},
		{Name: "yaml_editor", ConfigGate: "agent.allow_filesystem_write", Capabilities: []Capability{CapabilityRead, CapabilityWrite, CapabilityChange, CapabilityDelete, CapabilityHost}},
	}
}

type HomeLabIntegration struct {
	Name                   string
	ConfigGate             string
	CredentialSource       string
	AllowsLocalNetwork     bool
	ReadOnlyGate           string
	HasWriteOrDelete       bool
	HasTestEndpoint        bool
	EnabledByDefault       bool
	DestructiveGuardConfig string
}

func HomeLabIntegrationMatrix() []HomeLabIntegration {
	return []HomeLabIntegration{
		{Name: "adguard", ConfigGate: "adguard.enabled", CredentialSource: "config/vault", AllowsLocalNetwork: true, HasTestEndpoint: true},
		{Name: "chromecast", ConfigGate: "tools.chromecast.enabled", CredentialSource: "none", AllowsLocalNetwork: true},
		{Name: "docker", ConfigGate: "docker.enabled", CredentialSource: "docker socket", AllowsLocalNetwork: true, ReadOnlyGate: "docker.read_only", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "fritzbox", ConfigGate: "fritzbox.*.enabled", CredentialSource: "config/vault", AllowsLocalNetwork: true, HasTestEndpoint: true},
		{Name: "home_assistant", ConfigGate: "home_assistant.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, ReadOnlyGate: "home_assistant.readonly", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "jellyfin", ConfigGate: "jellyfin.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, HasTestEndpoint: true},
		{Name: "meshcentral", ConfigGate: "meshcentral.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, ReadOnlyGate: "meshcentral.readonly", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "mqtt", ConfigGate: "mqtt.enabled", CredentialSource: "config/vault", AllowsLocalNetwork: true, ReadOnlyGate: "mqtt.readonly", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "obsidian", ConfigGate: "obsidian.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, ReadOnlyGate: "obsidian.readonly", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "proxmox", ConfigGate: "proxmox.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, ReadOnlyGate: "proxmox.readonly", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "tailscale", ConfigGate: "tailscale.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, ReadOnlyGate: "tailscale.readonly", HasWriteOrDelete: true, HasTestEndpoint: true},
		{Name: "truenas", ConfigGate: "truenas.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, ReadOnlyGate: "truenas.read_only", HasWriteOrDelete: true, HasTestEndpoint: true, DestructiveGuardConfig: "truenas.allow_destructive"},
		{Name: "uptime_kuma", ConfigGate: "uptime_kuma.enabled", CredentialSource: "vault", AllowsLocalNetwork: true, HasTestEndpoint: true},
	}
}

type RouteContract struct {
	Pattern      string
	Methods      []string
	Auth         string
	Category     string
	ContentTypes []string
}

func RouteContractManifest() []RouteContract {
	return []RouteContract{
		{Pattern: "/api/auth/", Methods: []string{"GET", "POST", "DELETE"}, Auth: "public-bootstrap", Category: "auth"},
		{Pattern: "/api/setup", Methods: []string{"GET", "POST"}, Auth: "public-bootstrap", Category: "setup", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/i18n", Methods: []string{"GET"}, Auth: "public", Category: "i18n"},
		{Pattern: "/api/openrouter/models", Methods: []string{"GET"}, Auth: "public", Category: "setup"},
		{Pattern: "/api/internal/", Methods: []string{"POST", "DELETE"}, Auth: "internal-loopback-token", Category: "internal"},
		{Pattern: "/api/n8n/", Methods: []string{"GET", "POST", "DELETE"}, Auth: "bearer-token", Category: "automation"},
		{Pattern: "/api/remote/ws", Methods: []string{"GET"}, Auth: "remote-key-handshake", Category: "remote"},
		{Pattern: "/api/remote/download/", Methods: []string{"GET"}, Auth: "enrollment-token", Category: "remote"},
		{Pattern: "/api/invasion/ws", Methods: []string{"GET"}, Auth: "hmac-handshake", Category: "invasion"},
		{Pattern: "/api/config", Methods: []string{"GET", "PUT"}, Auth: "session", Category: "config", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/config/", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "config"},
		{Pattern: "/api/providers", Methods: []string{"GET", "PUT"}, Auth: "session", Category: "config", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/vault", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session", Category: "secrets", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/credentials", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "secrets", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/tokens", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "secrets", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/webhooks", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "webhooks", ContentTypes: []string{"application/json"}},
		{Pattern: "/webhook/", Methods: []string{"POST"}, Auth: "webhook-token", Category: "webhooks"},
		{Pattern: "/api/missions", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "missions", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/missions/v2", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "missions", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/skills", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "skills", ContentTypes: []string{"application/json", "multipart/form-data"}},
		{Pattern: "/api/media", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session", Category: "media", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/image-gallery", Methods: []string{"GET", "DELETE"}, Auth: "session", Category: "media", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/knowledge", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "knowledge"},
		{Pattern: "/api/knowledge-graph", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "knowledge", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/debug/", Methods: []string{"GET", "POST"}, Auth: "session", Category: "debug"},
		{Pattern: "/api/dashboard/", Methods: []string{"GET", "POST"}, Auth: "session", Category: "dashboard"},
		{Pattern: "/api/indexing/", Methods: []string{"GET", "POST"}, Auth: "session", Category: "knowledge"},
		{Pattern: "/api/plans", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "planner", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/todos", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "planner", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/devices", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "inventory", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/remote/", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "remote", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/invasion/", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session-or-internal-token", Category: "invasion", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/truenas/", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session", Category: "integration", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/containers", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session", Category: "integration", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/daemons", Methods: []string{"GET", "POST"}, Auth: "session", Category: "skills"},
		{Pattern: "/api/sql-connections", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "integration", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/mcp", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "mcp", ContentTypes: []string{"application/json"}},
		{Pattern: "/mcp", Methods: []string{"POST"}, Auth: "bearer-token", Category: "mcp"},
		{Pattern: "/api/", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session", Category: "misc"},
		{Pattern: "/", Methods: []string{"GET"}, Auth: "public-static", Category: "ui"},
		{Pattern: "/events", Methods: []string{"GET"}, Auth: "session", Category: "sse"},
		{Pattern: "/history", Methods: []string{"GET"}, Auth: "session", Category: "chat"},
		{Pattern: "/clear", Methods: []string{"DELETE"}, Auth: "session", Category: "chat"},
		{Pattern: "/notifications", Methods: []string{"GET"}, Auth: "session", Category: "notifications"},
		{Pattern: "/notifications/read", Methods: []string{"POST"}, Auth: "session", Category: "notifications"},
		{Pattern: "/v1/chat/completions", Methods: []string{"POST"}, Auth: "session-or-api-key", Category: "llm", ContentTypes: []string{"application/json"}},
	}
}

type NetworkClientUse struct {
	Path           string
	Classification string
	RequiresSSRF   bool
	AllowsLocalNet bool
	Credentialed   bool
}

func NetworkClientInventory() []NetworkClientUse {
	return []NetworkClientUse{
		{Path: "cmd/aurago/", Classification: "internal-loopback-and-configured-cron", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/a2a/", Classification: "configured-agent-endpoint", Credentialed: true},
		{Path: "internal/agent/", Classification: "agent-managed-http-tools", RequiresSSRF: true, AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/fritzbox/", Classification: "local-home-lab", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/invasion/", Classification: "managed-remote-nest", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/jellyfin/", Classification: "configured-media-server", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/llm/", Classification: "provider-api", Credentialed: true},
		{Path: "internal/media/", Classification: "user-requested-media-fetch", RequiresSSRF: true},
		{Path: "internal/memory/", Classification: "provider-api", Credentialed: true},
		{Path: "internal/meshcentral/", Classification: "configured-home-lab", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/obsidian/", Classification: "configured-local-api", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/prompts/", Classification: "tokenizer-download", RequiresSSRF: true},
		{Path: "internal/rocketchat/", Classification: "configured-messaging", Credentialed: true},
		{Path: "internal/security/", Classification: "ssrf-policy-implementation", RequiresSSRF: true},
		{Path: "internal/server/", Classification: "server-handler-or-loopback", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/telegram/", Classification: "telegram-api", Credentialed: true},
		{Path: "internal/telnyx/", Classification: "telnyx-api", Credentialed: true},
		{Path: "internal/tools/", Classification: "tool-specific-network-policy", RequiresSSRF: true, AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/truenas/", Classification: "configured-home-lab", AllowsLocalNet: true, Credentialed: true},
		{Path: "internal/webhooks/", Classification: "user-configured-outgoing-webhook", RequiresSSRF: true, Credentialed: true},
		{Path: "scripts/", Classification: "developer-helper-download", RequiresSSRF: true},
	}
}

type DBMigrationDomain struct {
	Domain               string
	PackagePath          string
	SchemaVersioned      bool
	HasLegacyFixtureTest bool
	OwnsRuntimeData      bool
}

func DBMigrationManifest() []DBMigrationDomain {
	return []DBMigrationDomain{
		{Domain: "contacts", PackagePath: "internal/contacts", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "credentials", PackagePath: "internal/credentials", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "inventory", PackagePath: "internal/inventory", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "planner", PackagePath: "internal/planner", SchemaVersioned: true, HasLegacyFixtureTest: true, OwnsRuntimeData: true},
		{Domain: "remote", PackagePath: "internal/remote", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "memory-short-term", PackagePath: "internal/memory", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "cheatsheets", PackagePath: "internal/tools", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "mission-preparation", PackagePath: "internal/tools", SchemaVersioned: true, OwnsRuntimeData: true},
		{Domain: "media-registry", PackagePath: "internal/tools", SchemaVersioned: false, OwnsRuntimeData: true},
		{Domain: "skills-registry", PackagePath: "internal/tools", SchemaVersioned: false, OwnsRuntimeData: true},
		{Domain: "truenas-registry", PackagePath: "internal/truenas", SchemaVersioned: false, OwnsRuntimeData: true},
	}
}

type SecurityBoundary struct {
	Name         string
	Boundary     string
	Enforcement  string
	TestCoverage string
}

func SecurityBoundaryManifest() []SecurityBoundary {
	return []SecurityBoundary{
		{
			Name:         "registered-secret-scrub",
			Boundary:     "security.RegisterSensitive -> security.Scrub -> chat/tool-visible output",
			Enforcement:  "registered literals plus fragmented, hex, and base64 variants are replaced with a visible placeholder",
			TestCoverage: "internal/security/scrubber_test.go and internal/audit cross-package sentinel",
		},
		{
			Name:         "sandbox-env-filter",
			Boundary:     "parent process environment -> sandbox helper and child process environment",
			Enforcement:  "sandbox.FilterEnv removes AuraGo, provider, token, password, secret, and credential env vars",
			TestCoverage: "internal/sandbox/sandbox_test.go and internal/audit cross-package sentinel",
		},
		{
			Name:         "python-vault-key-denylist",
			Boundary:     "vault secret key request -> Python execution secret injection",
			Enforcement:  "tools.IsPythonAccessibleSecret blocks system/integration-managed exact keys and prefixes",
			TestCoverage: "internal/tools/python_secrets_test.go and internal/audit cross-package sentinel",
		},
		{
			Name:         "skill-test-secret-denylist",
			Boundary:     "skill registry vault_keys -> /api/skills/{id}/test execution secrets",
			Enforcement:  "server.loadPlainSkillSecrets delegates to tools.IsPythonAccessibleSecret before reading vault values",
			TestCoverage: "internal/server/skills_handlers_execution_test.go",
		},
		{
			Name:         "vault-api-values-hidden",
			Boundary:     "vault storage -> REST/UI configuration and secret listing responses",
			Enforcement:  "vault list endpoints return keys/metadata or masked indicators, never raw secret values",
			TestCoverage: "internal/server/vault_handlers_test.go, provider_handlers_test.go, webhook_handlers_test.go",
		},
		{
			Name:         "external-data-wrapper",
			Boundary:     "untrusted external messages/content -> LLM prompt input",
			Enforcement:  "security.IsolateExternalData wraps content and escapes nested external_data tags",
			TestCoverage: "internal/security/guardian_test.go and internal/audit messaging ingress manifest",
		},
		{
			Name:         "tool-output-scrub",
			Boundary:     "tool stdout/stderr/errors -> agent response stream",
			Enforcement:  "dispatch paths call security.Scrub before composing user-visible tool output",
			TestCoverage: "internal/agent dispatch tests and internal/audit cross-package sentinel",
		},
	}
}

type HostIsolationBoundary struct {
	Name         string
	Enforcement  string
	ConfigGate   string
	PlatformGate string
	TestCoverage string
	HostEffect   bool
}

func HostIsolationBoundaryManifest() []HostIsolationBoundary {
	return []HostIsolationBoundary{
		{
			Name:         "filesystem-workspace-resolver",
			Enforcement:  "tools.secureResolve resolves user paths under the workspace and rejects traversal outside the project root",
			ConfigGate:   "agent.allow_filesystem_write for mutating operations",
			TestCoverage: "internal/tools/filesystem_test.go",
			HostEffect:   true,
		},
		{
			Name:         "file-editor-workspace-resolver",
			Enforcement:  "file/json/yaml/xml/toml editors use tools.secureResolve before reading or writing files",
			ConfigGate:   "agent.allow_filesystem_write",
			TestCoverage: "internal/tools/*_editor_test.go and internal/agent/dispatch_filesystem.go permission tests",
			HostEffect:   true,
		},
		{
			Name:         "python-working-directory",
			Enforcement:  "Python execution writes temporary files and runs inside cfg.Directories.WorkspaceDir",
			ConfigGate:   "agent.allow_python",
			TestCoverage: "internal/tools/python_secrets_test.go and internal/agent/dispatch_python tests",
			HostEffect:   true,
		},
		{
			Name:         "shell-privilege-wrapper-block",
			Enforcement:  "tools.ValidateShellCommandPolicy rejects privilege wrappers and high-risk host operations before shell start",
			ConfigGate:   "agent.allow_shell",
			TestCoverage: "internal/tools/shell_test.go",
			HostEffect:   true,
		},
		{
			Name:         "sudo-feature-gate",
			Enforcement:  "agent dispatch requires sudo_enabled, no-new-privileges availability, and ProtectSystem compatibility before ExecuteSudo",
			ConfigGate:   "agent.sudo_enabled and agent.sudo_unrestricted",
			PlatformGate: "not Windows, not Docker/no_new_privileges, ProtectSystem-aware",
			TestCoverage: "internal/agent/dispatch_shell.go static guard and internal/tools/shell_test.go",
			HostEffect:   true,
		},
		{
			Name:         "remote-file-allowed-dirs",
			Enforcement:  "cmd/remote executor validates resolved artifact paths against allowed directories",
			ConfigGate:   "agent.allow_remote_shell",
			TestCoverage: "cmd/remote/executor_test.go",
			HostEffect:   true,
		},
		{
			Name:         "sandbox-helper-env-filter",
			Enforcement:  "sandbox helper receives a minimal filtered environment and Landlock path rules on Linux",
			PlatformGate: "Linux Landlock when available; fallback is explicitly marked unavailable",
			TestCoverage: "internal/sandbox/sandbox_test.go",
			HostEffect:   true,
		},
		{
			Name:         "system-service-operations",
			Enforcement:  "service_manager is routed through shell access and remains disabled when allow_shell is false",
			ConfigGate:   "agent.allow_shell",
			TestCoverage: "internal/agent/dispatch_shell.go static guard",
			HostEffect:   true,
		},
	}
}

type MessagingIngressBoundary struct {
	Channel                     string
	SourcePath                  string
	WrapsExternalData           bool
	RequiresPromptInjectionScan bool
}

func MessagingIngressManifest() []MessagingIngressBoundary {
	return []MessagingIngressBoundary{
		{
			Channel:                     "telegram",
			SourcePath:                  "internal/telegram/bot.go",
			WrapsExternalData:           true,
			RequiresPromptInjectionScan: true,
		},
		{
			Channel:                     "discord",
			SourcePath:                  "internal/discord/bot.go",
			WrapsExternalData:           true,
			RequiresPromptInjectionScan: true,
		},
		{
			Channel:                     "rocketchat",
			SourcePath:                  "internal/rocketchat/bot.go",
			WrapsExternalData:           true,
			RequiresPromptInjectionScan: true,
		},
		{
			Channel:           "telnyx",
			SourcePath:        "internal/telnyx/broker.go",
			WrapsExternalData: true,
		},
	}
}
