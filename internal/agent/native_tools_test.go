package agent

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	promptsembed "aurago/prompts"

	"github.com/sashabaranov/go-openai"
)

func TestNativeToolCallToToolCall_TruncatedJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name      string
		funcName  string
		args      string
		wantField string
		wantValue string
	}{
		{
			name:      "truncated generate_image recovers prompt",
			funcName:  "generate_image",
			args:      `{"prompt": "A dystopian cityscape with neon lights", "size": "1024x10`,
			wantField: "prompt",
			wantValue: "A dystopian cityscape with neon lights",
		},
		{
			name:      "truncated shell recovers command",
			funcName:  "shell",
			args:      `{"command": "ls -la /tmp", "background": tr`,
			wantField: "command",
			wantValue: "ls -la /tmp",
		},
		{
			name:      "truncated execute_skill recovers skill",
			funcName:  "execute_skill",
			args:      `{"skill": "weather_check", "skill_args": {"ci`,
			wantField: "skill",
			wantValue: "weather_check",
		},
		{
			name:      "truncated query_memory recovers query",
			funcName:  "query_memory",
			args:      `{"query": "user preferences for docker`,
			wantField: "query",
			wantValue: "user preferences for docker",
		},
		{
			name:      "truncated with escaped quotes in prompt",
			funcName:  "generate_image",
			args:      `{"prompt": "A sign saying \"hello world\"", "size": "10`,
			wantField: "prompt",
			wantValue: `A sign saying "hello world"`,
		},
		{
			name:      "valid JSON still works",
			funcName:  "generate_image",
			args:      `{"prompt": "A beautiful sunset", "size": "1024x1024"}`,
			wantField: "prompt",
			wantValue: "A beautiful sunset",
		},
		{
			name:      "empty arguments",
			funcName:  "generate_image",
			args:      "",
			wantField: "prompt",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native := openai.ToolCall{
				ID:   "call_test123",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tt.funcName,
					Arguments: tt.args,
				},
			}
			tc := NativeToolCallToToolCall(native, logger)

			var got string
			switch tt.wantField {
			case "prompt":
				got = tc.Prompt
			case "command":
				got = tc.Command
			case "skill":
				got = tc.Skill
			case "query":
				got = tc.Query
			case "content":
				got = tc.Content
			case "operation":
				got = tc.Operation
			}

			if got != tt.wantValue {
				t.Errorf("field %q = %q, want %q", tt.wantField, got, tt.wantValue)
			}

			if tc.Action != tt.funcName {
				t.Errorf("Action = %q, want %q", tc.Action, tt.funcName)
			}
		})
	}
}

// TestToolSchemaManualSync verifies that every built-in tool has a corresponding
// manual in the embedded prompts/tools_manuals/ directory.
// Tools listed in knownNoManual are exempt (simple tools that don't need guides).
func TestToolSchemaManualSync(t *testing.T) {
	// Get all tool names with all feature flags enabled.
	ff := ToolFeatureFlags{
		AllowShell: true, AllowPython: true, AllowFilesystemWrite: true,
		AllowNetworkRequests: true, AllowSelfUpdate: true, AllowRemoteShell: true,
		DockerEnabled: true, HomeAssistantEnabled: true, ProxmoxEnabled: true,
		CoAgentEnabled: true, SandboxEnabled: true,
		GitHubEnabled: true, SudoEnabled: true, AdGuardEnabled: true,
		HomepageEnabled: true, InventoryEnabled: true, MQTTEnabled: true,
		TailscaleEnabled: true, CloudflareTunnelEnabled: true, OllamaEnabled: true,
		WOLEnabled: true, WebhooksEnabled: true, FirewallEnabled: true,
		JournalEnabled: true, GoogleWorkspaceEnabled: true, MissionsEnabled: true,
		MediaRegistryEnabled: true, MeshCentralEnabled: true, NetlifyEnabled: true,
		EmailEnabled: true, NotesEnabled: true, InvasionControlEnabled: true,
		AnsibleEnabled: true, MCPEnabled: true, HomepageRegistryEnabled: true,
		RemoteControlEnabled: true, ImageGenerationEnabled: true,
		MemoryEnabled: true, KnowledgeGraphEnabled: true, SecretsVaultEnabled: true,
		SchedulerEnabled: true, StopProcessEnabled: true,
		MemoryMaintenanceEnabled: true, MemoryAnalysisEnabled: true,
		DocumentCreatorEnabled: true, HomepageAllowLocalServer: true,
	}
	schemas := builtinToolSchemas(ff)

	// Collect all embedded manual filenames (without .md extension).
	manuals := make(map[string]bool)
	_ = fs.WalkDir(promptsembed.FS, "tools_manuals", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		manuals[name] = true
		return nil
	})

	// Tools that are intentionally simple / don't need a manual,
	// or whose manual uses a different filename.
	knownNoManual := map[string]bool{
		"execute_sudo":             true, // single-param wrapper around shell
		"save_tool":                true, // simple tool registration
		"wake_on_lan":              true, // simple WOL packet
		"call_webhook":             true, // just triggers a named webhook
		"manage_webhooks":          true, // covered by webhook docs
		"manage_outgoing_webhooks": true, // covered by webhook docs
		"query_inventory":          true, // simple query tool
		"register_device":          true, // simple registration
		"firewall":                 true, // niche integration
		"invasion_control":         true, // internal egg/nest system
		"fetch_email":              true, // covered by email.md
		"send_email":               true, // covered by email.md
		"list_email_accounts":      true, // covered by email.md
		"manage_memory":            true, // covered by context_memory.md / core_memory.md
		"mqtt_get_messages":        true, // covered by mqtt.md
		"mqtt_subscribe":           true, // covered by mqtt.md
		"mqtt_unsubscribe":         true, // covered by mqtt.md
		"mqtt_publish":             true, // covered by mqtt.md
		"mcp_call":                 true, // manual is mcp.md
		"execute_sandbox":          true, // manual is sandbox.md
		"document_creator":         true, // simple single-purpose tool
	}

	var missing []string
	for _, s := range schemas {
		name := s.Function.Name
		if knownNoManual[name] {
			continue
		}
		if !manuals[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Tool schemas without a manual in prompts/tools_manuals/ (add a .md file or add to knownNoManual):\n  %s",
			strings.Join(missing, "\n  "))
	}
}
