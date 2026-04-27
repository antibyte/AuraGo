package agent

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
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

func TestNativeToolCallToToolCall_RejectsTraversingSkillShortcut(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	native := openai.ToolCall{
		ID:   "call_shortcut_bad",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "skill__../escape",
			Arguments: `{"input":"ignored"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "execute_skill" {
		t.Fatalf("Action = %q, want execute_skill", tc.Action)
	}
	if !tc.NativeArgsMalformed {
		t.Fatal("expected malformed native args for invalid skill shortcut")
	}
	if !strings.Contains(tc.NativeArgsError, "must not contain path separators") {
		t.Fatalf("unexpected NativeArgsError: %q", tc.NativeArgsError)
	}
}

func TestNativeToolCallToToolCall_AllowsValidSkillShortcut(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	native := openai.ToolCall{
		ID:   "call_shortcut_ok",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "skill__weather_check",
			Arguments: `{"city":"Berlin"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "execute_skill" {
		t.Fatalf("Action = %q, want execute_skill", tc.Action)
	}
	if tc.Skill != "weather_check" {
		t.Fatalf("Skill = %q, want weather_check", tc.Skill)
	}
	if tc.NativeArgsMalformed {
		t.Fatalf("unexpected malformed flag: %q", tc.NativeArgsError)
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
		GitHubEnabled: true, SudoEnabled: true, AdGuardEnabled: true, UptimeKumaEnabled: true,
		HomepageEnabled: true, InventoryEnabled: true, MQTTEnabled: true,
		TailscaleEnabled: true, CloudflareTunnelEnabled: true, OllamaEnabled: true,
		WOLEnabled: true, WebhooksEnabled: true, FirewallEnabled: true,
		JournalEnabled: true, GoogleWorkspaceEnabled: true, MissionsEnabled: true,
		MediaRegistryEnabled: true, MeshCentralEnabled: true, NetlifyEnabled: true,
		VercelEnabled: true,
		EmailEnabled:  true, NotesEnabled: true, InvasionControlEnabled: true,
		AnsibleEnabled: true, MCPEnabled: true, HomepageRegistryEnabled: true,
		RemoteControlEnabled: true, ImageGenerationEnabled: true, VideoGenerationEnabled: true,
		MemoryEnabled: true, KnowledgeGraphEnabled: true, SecretsVaultEnabled: true,
		SchedulerEnabled: true, StopProcessEnabled: true,
		MemoryMaintenanceEnabled: true, MemoryAnalysisEnabled: true,
		DocumentCreatorEnabled: true, HomepageAllowLocalServer: true,
		WebCaptureEnabled: true, NetworkPingEnabled: true, WebScraperEnabled: true,
		S3Enabled: true,
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
		"list_skills":                true, // covered by skills_engine.md
		"execute_sudo":               true, // single-param wrapper around shell
		"save_tool":                  true, // simple tool registration
		"list_skill_templates":       true, // covered by skill_templates.md
		"create_skill_from_template": true, // covered by skill_templates.md
		"get_skill_documentation":    true, // covered by skills_engine.md
		"set_skill_documentation":    true, // covered by skills_engine.md
		"wake_on_lan":                true, // simple WOL packet
		"call_webhook":               true, // just triggers a named webhook
		"manage_webhooks":            true, // covered by webhook docs
		"manage_outgoing_webhooks":   true, // covered by webhook docs
		"query_inventory":            true, // simple query tool
		"register_device":            true, // simple registration
		"firewall":                   true, // niche integration
		"invasion_control":           true, // internal egg/nest system
		"fetch_email":                true, // covered by email.md
		"send_email":                 true, // covered by email.md
		"list_email_accounts":        true, // covered by email.md
		"manage_memory":              true, // covered by context_memory.md / core_memory.md
		"mqtt_get_messages":          true, // covered by mqtt.md
		"mqtt_subscribe":             true, // covered by mqtt.md
		"mqtt_unsubscribe":           true, // covered by mqtt.md
		"mqtt_publish":               true, // covered by mqtt.md
		"mcp_call":                   true, // manual is mcp.md
		"execute_sandbox":            true, // manual is sandbox.md
		"document_creator":           true, // simple single-purpose tool
		"transfer_remote_file":       true, // covered by remote_execution manual
		"send_discord":               true, // covered by discord.md
		"fetch_discord":              true, // covered by discord.md
		"list_discord_channels":      true, // covered by discord.md
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

func TestBuiltinToolSchemasIncludeUptimeKumaWhenEnabled(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{UptimeKumaEnabled: true})
	for _, schema := range schemas {
		if schema.Function != nil && schema.Function.Name == "uptime_kuma" {
			return
		}
	}
	t.Fatal("expected uptime_kuma tool schema when UptimeKumaEnabled is true")
}

func TestBuildNativeToolSchemasIncludesVirusTotalAndListSkills(t *testing.T) {
	skillsDir := t.TempDir()
	skillManifest := `{
  "name": "virustotal_scan",
  "description": "Scan a URL, domain, IP address, or file hash against VirusTotal.",
  "executable": "__builtin__",
  "parameters": {"resource": "Resource to scan"}
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "virustotal_scan.json"), []byte(skillManifest), 0o644); err != nil {
		t.Fatalf("write skill manifest: %v", err)
	}

	schemas := BuildNativeToolSchemas(skillsDir, nil, ToolFeatureFlags{VirusTotalEnabled: true}, nil)
	names := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		if s.Function != nil {
			names[s.Function.Name] = true
		}
	}

	for _, name := range []string{"list_skills", "execute_skill", "virustotal_scan", "skill__virustotal_scan"} {
		if !names[name] {
			t.Fatalf("expected schema %q to be present", name)
		}
	}
}

func TestBuiltinToolSchemasRegistersMeshCentralOnlyOnce(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{MeshCentralEnabled: true})
	count := 0
	for _, schema := range schemas {
		if schema.Function != nil && schema.Function.Name == "meshcentral" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("meshcentral schema count = %d, want 1", count)
	}
}

func TestBuiltinToolSchemasIncludeVercelWhenEnabled(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{VercelEnabled: true})
	for _, schema := range schemas {
		if schema.Function != nil && schema.Function.Name == "vercel" {
			return
		}
	}
	t.Fatal("expected vercel tool schema when VercelEnabled is true")
}

func TestBuiltinToolSchemasInvasionControlSupportsEggName(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{InvasionControlEnabled: true})

	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "invasion_control" {
			continue
		}
		if !strings.Contains(strings.ToLower(s.Function.Description), "egg names are not tool names") {
			t.Fatalf("invasion_control description should warn about egg-name/tool-name collisions, got %q", s.Function.Description)
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("invasion_control parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("invasion_control properties type = %T", params["properties"])
		}
		if _, ok := props["egg_name"]; !ok {
			t.Fatal("invasion_control schema should expose egg_name")
		}
		if _, ok := props["task_id"]; !ok {
			t.Fatal("invasion_control schema should expose task_id")
		}
		if _, ok := props["artifact_id"]; !ok {
			t.Fatal("invasion_control schema should expose artifact_id")
		}
		if _, ok := props["file_path"]; !ok {
			t.Fatal("invasion_control schema should expose file_path")
		}
		return
	}

	t.Fatal("invasion_control schema not found")
}

func TestNativeToolCallToToolCallInvasionControlEggName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_invasion_egg_name",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "invasion_control",
			Arguments: `{"operation":"send_task","egg_name":"web scraper","task":"sage mir einen witz"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "invasion_control" {
		t.Fatalf("Action = %q, want invasion_control", tc.Action)
	}
	if tc.Operation != "send_task" {
		t.Fatalf("Operation = %q, want send_task", tc.Operation)
	}
	if tc.EggName != "web scraper" {
		t.Fatalf("EggName = %q, want web scraper", tc.EggName)
	}
	if tc.Task != "sage mir einen witz" {
		t.Fatalf("Task = %q, want sage mir einen witz", tc.Task)
	}
}

func TestBuildNativeToolSchemasOmitsVirusTotalWhenDisabled(t *testing.T) {
	skillsDir := t.TempDir()
	skillManifest := `{
  "name": "virustotal_scan",
  "description": "Scan a URL, domain, IP address, or file hash against VirusTotal.",
  "executable": "__builtin__",
  "parameters": {"resource": "Resource to scan"}
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "virustotal_scan.json"), []byte(skillManifest), 0o644); err != nil {
		t.Fatalf("write skill manifest: %v", err)
	}

	schemas := BuildNativeToolSchemas(skillsDir, nil, ToolFeatureFlags{VirusTotalEnabled: false}, nil)
	names := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		if s.Function != nil {
			names[s.Function.Name] = true
		}
	}

	if names["virustotal_scan"] {
		t.Fatal("did not expect virustotal_scan schema when integration is disabled")
	}
	if names["skill__virustotal_scan"] {
		t.Fatal("did not expect skill__virustotal_scan schema when integration is disabled")
	}
	if !names["list_skills"] {
		t.Fatal("expected list_skills schema to remain available")
	}
}

func TestBuildNativeToolSchemasReturnsIsolatedCopiesWhenCached(t *testing.T) {
	ff := ToolFeatureFlags{VirusTotalEnabled: true}
	first := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)
	second := BuildNativeToolSchemas(t.TempDir(), nil, ff, nil)

	if len(first) == 0 || len(second) == 0 {
		t.Fatal("expected built-in schemas to be present")
	}

	params, ok := first[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("expected first tool params map")
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected first tool properties map")
	}
	props["cache_probe"] = map[string]interface{}{"type": "string"}

	params2, ok := second[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("expected second tool params map")
	}
	props2, ok := params2["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected second tool properties map")
	}
	if _, exists := props2["cache_probe"]; exists {
		t.Fatal("expected cached schema copies to be isolated")
	}
}

func TestNativeToolCallToToolCallVirusTotalFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_vt",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "virustotal_scan",
			Arguments: `{"resource":"example.com","file_path":"sample.txt","mode":"auto"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "virustotal_scan" {
		t.Fatalf("Action = %q, want virustotal_scan", tc.Action)
	}
	if tc.Resource != "example.com" {
		t.Fatalf("Resource = %q, want example.com", tc.Resource)
	}
	if tc.FilePath != "sample.txt" {
		t.Fatalf("FilePath = %q, want sample.txt", tc.FilePath)
	}
	if tc.Mode != "auto" {
		t.Fatalf("Mode = %q, want auto", tc.Mode)
	}
	if got, _ := tc.Params["resource"].(string); got != "example.com" {
		t.Fatalf("Params[resource] = %q, want example.com", got)
	}
}

func TestNativeToolCallToToolCallPreservesUnknownBuiltinArgsInParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_web_scraper",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "web_scraper",
			Arguments: `{"url":"https://example.com","search_query":"find the pricing details"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "web_scraper" {
		t.Fatalf("Action = %q, want web_scraper", tc.Action)
	}
	if got, _ := tc.Params["url"].(string); got != "https://example.com" {
		t.Fatalf("Params[url] = %q, want https://example.com", got)
	}
	if got, _ := tc.Params["search_query"].(string); got != "find the pricing details" {
		t.Fatalf("Params[search_query] = %q, want find the pricing details", got)
	}
}

func TestNativeToolCallToToolCallExecuteSkillUsesRawEnvelope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_execute_skill",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "execute_skill",
			Arguments: `{"skill":"paperless","skill_args":{"operation":"search","name":"Invoices","limit":3}}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "execute_skill" {
		t.Fatalf("Action = %q, want execute_skill", tc.Action)
	}
	if tc.Skill != "paperless" {
		t.Fatalf("Skill = %q, want paperless", tc.Skill)
	}
	if got, _ := tc.SkillArgs["name"].(string); got != "Invoices" {
		t.Fatalf("SkillArgs[name] = %q, want Invoices", got)
	}
	if got, _ := tc.Params["operation"].(string); got != "search" {
		t.Fatalf("Params[operation] = %q, want search", got)
	}
	if got, _ := tc.Params["limit"].(float64); got != 3 {
		t.Fatalf("Params[limit] = %v, want 3", got)
	}
}

func TestNativeToolCallToToolCallHomepageSubOperationPreservesToolAction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_homepage_edit",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "homepage",
			Arguments: `{"operation":"edit_file","path":"demo/index.html","action":"append","content":"<footer>ok</footer>"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "homepage" {
		t.Fatalf("Action = %q, want homepage", tc.Action)
	}
	if tc.Operation != "edit_file" {
		t.Fatalf("Operation = %q, want edit_file", tc.Operation)
	}
	if tc.SubOperation != "append" {
		t.Fatalf("SubOperation = %q, want append", tc.SubOperation)
	}
}

func TestNativeToolCallToToolCallMarksMalformedArguments(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_bad_args",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "filesystem",
			Arguments: `{"operation":"write_file","path":"src/App.tsx","content":"unterminated`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if !tc.NativeArgsMalformed {
		t.Fatal("expected malformed native args to be flagged")
	}
	if tc.Action != "filesystem" {
		t.Fatalf("Action = %q, want filesystem", tc.Action)
	}
	if tc.NativeArgsRaw == "" {
		t.Fatal("expected raw malformed args to be preserved")
	}
}

func TestNativeToolCallToToolCallMarksMalformedFunctionName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_bad_name",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "filesystem\", \"content\">import './style.css'",
			Arguments: `{"operation":"write_file"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if !tc.NativeArgsMalformed {
		t.Fatal("expected malformed native function name to be flagged")
	}
	if tc.NativeArgsError == "" {
		t.Fatal("expected malformed native function name to include an error")
	}
}

func TestBuiltinToolSchemasFilesystemIncludesBatchOperations(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{AllowFilesystemWrite: true})

	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "filesystem" {
			continue
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("filesystem parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("filesystem properties type = %T, want map[string]interface{}", params["properties"])
		}
		op := props["operation"].(map[string]interface{})
		enum := make([]string, 0)
		switch rawEnum := op["enum"].(type) {
		case []string:
			enum = append(enum, rawEnum...)
		case []interface{}:
			for _, v := range rawEnum {
				if s, ok := v.(string); ok {
					enum = append(enum, s)
				}
			}
		default:
			t.Fatalf("filesystem operation enum type = %T", op["enum"])
		}
		joined := strings.Join(enum, ",")
		for _, want := range []string{"copy", "copy_batch", "move_batch", "delete_batch", "create_dir_batch"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("filesystem schema missing %s in enum %v", want, enum)
			}
		}
		if _, ok := props["items"]; !ok {
			t.Fatal("filesystem schema should expose items for batch operations")
		}
		return
	}

	t.Fatal("filesystem schema not found")
}

func TestBuiltinToolSchemasHomepageUsesSubOperationField(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{HomepageEnabled: true, NetlifyEnabled: true})

	var homepageProps map[string]interface{}
	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "homepage" {
			continue
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("homepage parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("homepage properties missing")
		}
		homepageProps = props
		break
	}

	if homepageProps == nil {
		t.Fatal("homepage schema not found")
	}
	if _, ok := homepageProps["sub_operation"]; !ok {
		t.Fatal("homepage schema missing sub_operation property")
	}
	if _, ok := homepageProps["file_path"]; !ok {
		t.Fatal("homepage schema missing file_path alias for path")
	}
	if _, ok := homepageProps["action"]; ok {
		t.Fatal("homepage schema should not expose action as edit sub-operation field")
	}
}

func TestSendYouTubeVideoSchemaHonorsFeatureFlag(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "send_youtube_video") {
		t.Fatal("send_youtube_video should be hidden when disabled")
	}
	if !containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{SendYouTubeVideoEnabled: true})), "send_youtube_video") {
		t.Fatal("send_youtube_video should be visible when enabled")
	}
}

func TestVideoDownloadSchemaReflectsOptionalWritePermissions(t *testing.T) {
	readOnlySchemas := builtinToolSchemas(ToolFeatureFlags{VideoDownloadEnabled: true})
	readOnlyOps := videoDownloadOperationEnum(t, readOnlySchemas)
	for _, want := range []string{"search", "info"} {
		if !containsName(readOnlyOps, want) {
			t.Fatalf("read-only video_download schema missing %q in %v", want, readOnlyOps)
		}
	}
	for _, blocked := range []string{"download", "transcribe"} {
		if containsName(readOnlyOps, blocked) {
			t.Fatalf("read-only video_download schema unexpectedly exposes %q in %v", blocked, readOnlyOps)
		}
	}

	fullSchemas := builtinToolSchemas(ToolFeatureFlags{
		VideoDownloadEnabled:         true,
		VideoDownloadAllowDownload:   true,
		VideoDownloadAllowTranscribe: true,
	})
	fullOps := videoDownloadOperationEnum(t, fullSchemas)
	for _, want := range []string{"search", "info", "download", "transcribe"} {
		if !containsName(fullOps, want) {
			t.Fatalf("full video_download schema missing %q in %v", want, fullOps)
		}
	}
}

func videoDownloadOperationEnum(t *testing.T, schemas []openai.Tool) []string {
	t.Helper()
	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "video_download" {
			continue
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("video_download parameters type = %T", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("video_download properties missing")
		}
		opProp, ok := props["operation"].(map[string]interface{})
		if !ok {
			t.Fatal("video_download operation property missing")
		}
		enum, ok := opProp["enum"].([]string)
		if !ok {
			t.Fatalf("video_download operation enum type = %T", opProp["enum"])
		}
		return enum
	}
	t.Fatal("video_download schema not found")
	return nil
}

func TestBuiltinToolSchemasNetlifyOmitsZipDeployOperations(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{NetlifyEnabled: true})

	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "netlify" {
			continue
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("netlify parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("netlify properties missing")
		}
		opProp, ok := props["operation"].(map[string]interface{})
		if !ok {
			t.Fatal("netlify operation property missing")
		}
		enumVals, ok := opProp["enum"].([]string)
		if !ok {
			t.Fatalf("netlify operation enum type = %T, want []string", opProp["enum"])
		}
		for _, op := range enumVals {
			if op == "deploy_zip" || op == "deploy_draft" {
				t.Fatalf("unexpected ZIP deploy operation still exposed: %s", op)
			}
		}
		if _, ok := props["content"]; ok {
			t.Fatal("netlify schema should not expose content for ZIP deploys in agent flow")
		}
		return
	}

	t.Fatal("netlify schema not found")
}

func TestBuiltinToolSchemasDoNotContainDuplicateNames(t *testing.T) {
	schemas := builtinToolSchemas(allBuiltinToolFeatureFlags())

	seen := make(map[string]struct{}, len(schemas))
	for _, s := range schemas {
		if s.Function == nil || s.Function.Name == "" {
			continue
		}
		if _, ok := seen[s.Function.Name]; ok {
			t.Fatalf("duplicate builtin tool schema name: %s", s.Function.Name)
		}
		seen[s.Function.Name] = struct{}{}
	}
}

func TestAllBuiltinToolFeatureFlagsIncludeYepAPITools(t *testing.T) {
	schemas := builtinToolSchemas(allBuiltinToolFeatureFlags())
	names := toolNames(schemas)
	for _, want := range []string{
		"yepapi_seo",
		"yepapi_serp",
		"yepapi_scrape",
		"yepapi_youtube",
		"yepapi_tiktok",
		"yepapi_instagram",
		"yepapi_amazon",
	} {
		if !containsName(names, want) {
			t.Fatalf("allBuiltinToolFeatureFlags missing %s; discover_tools/tests cannot see enabled YepAPI tools", want)
		}
	}
}

func TestBuiltinToolSchemasIncludeWikipediaAndDDGSearch(t *testing.T) {
	schemas := builtinToolSchemas(allBuiltinToolFeatureFlags())
	foundWikipedia := false
	foundDDG := false
	for _, s := range schemas {
		if s.Function == nil {
			continue
		}
		switch s.Function.Name {
		case "wikipedia_search":
			foundWikipedia = true
		case "ddg_search":
			foundDDG = true
		}
	}
	if !foundWikipedia {
		t.Fatal("expected wikipedia_search builtin schema to be present")
	}
	if !foundDDG {
		t.Fatal("expected ddg_search builtin schema to be present")
	}
}

func TestBuildNativeToolSchemasSkipsCustomSkillCollidingWithBuiltinTool(t *testing.T) {
	skillsDir := t.TempDir()
	skillManifest := `{
  "name": "web_scraper",
  "description": "Custom colliding skill",
  "executable": "custom_web_scraper.py",
  "parameters": {
    "url": "URL to scrape"
  }
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "web_scraper.json"), []byte(skillManifest), 0o644); err != nil {
		t.Fatalf("write skill manifest: %v", err)
	}

	names := make(map[string]bool)
	for _, toolSchema := range BuildNativeToolSchemas(skillsDir, nil, allBuiltinToolFeatureFlags(), nil) {
		if toolSchema.Function != nil {
			names[toolSchema.Function.Name] = true
		}
	}

	if names["skill__web_scraper"] {
		t.Fatal("did not expect custom skill shortcut for built-in tool name collision")
	}
}

func TestBuildNativeToolSchemasSkipsCustomToolCollidingWithBuiltinTool(t *testing.T) {
	toolsDir := t.TempDir()
	manifest := tools.NewManifest(toolsDir)
	if err := manifest.Register("virustotal_scan", "Custom colliding tool"); err != nil {
		t.Fatalf("register custom tool: %v", err)
	}

	names := make(map[string]bool)
	for _, toolSchema := range BuildNativeToolSchemas(t.TempDir(), manifest, allBuiltinToolFeatureFlags(), nil) {
		if toolSchema.Function != nil {
			names[toolSchema.Function.Name] = true
		}
	}

	if names["tool__virustotal_scan"] {
		t.Fatal("did not expect custom tool shortcut for built-in tool name collision")
	}
}

func TestBuildNativeToolSchemasSortsCustomToolsByName(t *testing.T) {
	toolsDir := t.TempDir()
	manifest := tools.NewManifest(toolsDir)
	manifestJSON := `{
  "version": 2,
  "tools": {
    "zeta_helper": "Zeta helper",
    "alpha_helper": "Alpha helper",
    "middle_helper": "Middle helper"
  }
}`
	if err := os.WriteFile(filepath.Join(toolsDir, "manifest.json"), []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var got []string
	for _, toolSchema := range BuildNativeToolSchemas(t.TempDir(), manifest, ToolFeatureFlags{}, nil) {
		if toolSchema.Function != nil && strings.HasPrefix(toolSchema.Function.Name, "tool__") {
			got = append(got, toolSchema.Function.Name)
		}
	}

	want := []string{"tool__alpha_helper", "tool__middle_helper", "tool__zeta_helper"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("custom tool order = %v, want %v", got, want)
	}
}

func TestBuildNativeToolSchemasReturnsGloballySortedTools(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, allBuiltinToolFeatureFlags(), nil)
	last := ""
	for _, toolSchema := range schemas {
		if toolSchema.Function == nil {
			continue
		}
		name := toolSchema.Function.Name
		if last != "" && name < last {
			t.Fatalf("tool schemas are not sorted: %q came after %q", name, last)
		}
		last = name
	}
}

func TestToolFeatureFlagsKeyChangesWhenFlagsChange(t *testing.T) {
	base := ToolFeatureFlags{AllowShell: true, DockerEnabled: true}
	same := ToolFeatureFlags{AllowShell: true, DockerEnabled: true}
	changed := ToolFeatureFlags{AllowShell: true, DockerEnabled: false}

	if base.Key() != same.Key() {
		t.Fatal("expected identical feature flags to produce identical cache keys")
	}
	if base.Key() == changed.Key() {
		t.Fatal("expected different feature flags to produce different cache keys")
	}
}

// TestInjectAdditionalPropertiesRecPreservesExplicitTrue verifies that
// injectAdditionalPropertiesRec does NOT overwrite explicitly set
// additionalProperties:true values (e.g. call_webhook.parameters).
func TestInjectAdditionalPropertiesRecPreservesExplicitTrue(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"webhook_name": map[string]interface{}{"type": "string"},
			"parameters": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true, // explicit: must be preserved
			},
		},
	}

	injectAdditionalPropertiesRec(schema)

	params := schema["properties"].(map[string]interface{})["parameters"].(map[string]interface{})
	if params["additionalProperties"] != true {
		t.Fatalf("expected additionalProperties=true to be preserved, got %v", params["additionalProperties"])
	}

	// Top-level should also get additionalProperties:false
	if schema["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false on top-level, got %v", schema["additionalProperties"])
	}
}

// TestInjectAdditionalPropertiesRecPreservesSchemaObject verifies that
// a schema-level additionalProperties (i.e. {"type": "string"}) is preserved.
func TestInjectAdditionalPropertiesRecPreservesSchemaObject(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"headers": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": map[string]interface{}{"type": "string"},
			},
		},
	}

	injectAdditionalPropertiesRec(schema)

	headers := schema["properties"].(map[string]interface{})["headers"].(map[string]interface{})
	if _, ok := headers["additionalProperties"].(map[string]interface{}); !ok {
		t.Fatalf("expected additionalProperties schema object to be preserved, got %v", headers["additionalProperties"])
	}
}

func TestInjectAdditionalPropertiesRecHandlesCycles(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
	}
	schema["properties"] = map[string]interface{}{
		"self": schema,
	}

	injectAdditionalPropertiesRec(schema)

	if schema["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false on cyclic schema root, got %v", schema["additionalProperties"])
	}
	self := schema["properties"].(map[string]interface{})["self"].(map[string]interface{})
	if self["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false on cyclic child, got %v", self["additionalProperties"])
	}
}

func TestBuildNativeToolSchemasInjectsTodoOnlyForBuiltinTools(t *testing.T) {
	toolsDir := t.TempDir()
	manifest := tools.NewManifest(toolsDir)
	if err := manifest.Register("hello_custom", "Custom helper"); err != nil {
		t.Fatalf("register custom tool: %v", err)
	}

	schemas := BuildNativeToolSchemas(t.TempDir(), manifest, ToolFeatureFlags{AllowShell: true}, nil)

	var executeShellParams map[string]interface{}
	var customParams map[string]interface{}
	for _, toolSchema := range schemas {
		if toolSchema.Function == nil {
			continue
		}
		switch toolSchema.Function.Name {
		case "execute_shell":
			executeShellParams = toolSchema.Function.Parameters.(map[string]interface{})
		case "tool__hello_custom":
			customParams = toolSchema.Function.Parameters.(map[string]interface{})
		}
	}

	if executeShellParams == nil {
		t.Fatal("expected execute_shell schema")
	}
	if customParams == nil {
		t.Fatal("expected custom tool schema")
	}

	builtinProps := executeShellParams["properties"].(map[string]interface{})
	if _, ok := builtinProps["_todo"]; !ok {
		t.Fatal("expected builtin tool schema to include _todo")
	}
	customProps := customParams["properties"].(map[string]interface{})
	if _, ok := customProps["_todo"]; ok {
		t.Fatal("did not expect custom tool schema to include _todo")
	}
}

// TestBuildToolFlagsFromConfigProducesConsistentResults verifies that
// buildToolFlagsFromConfig returns consistent values for all config-only flags.
func TestBuildToolFlagsFromConfigProducesConsistentResults(t *testing.T) {
	cfg := &config.Config{}
	cfg.HomeAssistant.Enabled = true
	cfg.Docker.Enabled = true
	cfg.CoAgents.Enabled = true
	cfg.Agent.SudoEnabled = true
	cfg.Agent.AllowShell = true
	cfg.Agent.AllowPython = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Agent.AllowNetworkRequests = true
	cfg.Agent.AllowRemoteShell = true
	cfg.Agent.AllowSelfUpdate = true
	cfg.Webhooks.Enabled = true
	cfg.Proxmox.Enabled = true
	cfg.Ollama.Enabled = true
	cfg.Tailscale.Enabled = true
	cfg.Ansible.Enabled = true
	cfg.InvasionControl.Enabled = true
	cfg.GitHub.Enabled = true
	cfg.MQTT.Enabled = true
	cfg.AdGuard.Enabled = true
	cfg.MCP.Enabled = true
	cfg.Agent.AllowMCP = true
	cfg.Sandbox.Enabled = true
	cfg.MeshCentral.Enabled = true
	cfg.Homepage.Enabled = true
	cfg.Netlify.Enabled = true
	cfg.Firewall.Enabled = true
	cfg.Runtime.IsDocker = false
	cfg.Runtime.DockerSocketOK = true
	cfg.Runtime.NoNewPrivileges = false
	cfg.Runtime.FirewallAccessOK = false
	cfg.Email.Enabled = true
	cfg.EmailAccounts = nil
	cfg.CloudflareTunnel.Enabled = true
	cfg.GoogleWorkspace.Enabled = true
	cfg.OneDrive.Enabled = true
	cfg.VirusTotal.Enabled = true
	cfg.GolangciLint.Enabled = true
	cfg.ImageGeneration.Enabled = true
	cfg.MusicGeneration.Enabled = true
	cfg.VideoGeneration.Enabled = true
	cfg.RemoteControl.Enabled = true
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.SecretsVault.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.StopProcess.Enabled = true
	cfg.Tools.Inventory.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.WOL.Enabled = true
	cfg.MediaRegistry.Enabled = true
	cfg.Tools.Contacts.Enabled = true
	cfg.Tools.Planner.Enabled = true
	cfg.MemoryAnalysis.Enabled = true
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.WebCapture.Enabled = true
	cfg.Tools.NetworkPing.Enabled = true
	cfg.Tools.WebScraper.Enabled = true
	cfg.S3.Enabled = true
	cfg.Tools.NetworkScan.Enabled = true
	cfg.Tools.FormAutomation.Enabled = true
	cfg.Tools.UPnPScan.Enabled = true
	cfg.Jellyfin.Enabled = true
	cfg.Chromecast.Enabled = true
	cfg.Discord.Enabled = true
	cfg.Telegram.BotToken = "test"
	cfg.Telegram.UserID = 12345
	cfg.TrueNAS.Enabled = true
	cfg.Koofr.Enabled = true
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.System.Enabled = true
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Telephony.Enabled = true
	cfg.FritzBox.SmartHome.Enabled = true
	cfg.FritzBox.Storage.Enabled = true
	cfg.FritzBox.TV.Enabled = true
	cfg.Telnyx.Enabled = true
	cfg.Telnyx.ReadOnly = false
	cfg.SQLConnections.Enabled = true
	cfg.Tools.PythonSecretInjection.Enabled = true
	cfg.Tools.DaemonSkills.Enabled = true
	cfg.LDAP.Enabled = true

	ff := buildToolFlagsFromConfig(cfg)

	// Verify key flags that previously had drift issues
	if !ff.DockerEnabled {
		t.Error("expected DockerEnabled=true")
	}
	if !ff.SudoEnabled {
		t.Error("expected SudoEnabled=true")
	}
	if !ff.SandboxEnabled {
		t.Error("expected SandboxEnabled=true")
	}
	if !ff.HomepageEnabled {
		t.Error("expected HomepageEnabled=true")
	}
	if !ff.WOLEnabled {
		t.Error("expected WOLEnabled=true")
	}
	if !ff.LDAPEnabled {
		t.Error("expected LDAPEnabled=true")
	}
	if !ff.RemoteControlEnabled {
		t.Error("expected RemoteControlEnabled=true")
	}
}
