package agent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
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
			name:      "truncated filesystem recovers path",
			funcName:  "filesystem",
			args:      `{"operation": "read_file", "path": "agent_workspace/workdir/report.md`,
			wantField: "path",
			wantValue: "agent_workspace/workdir/report.md",
		},
		{
			name:      "truncated homepage recovers project_dir",
			funcName:  "homepage_file",
			args:      `{"operation": "read_file", "project_dir": "site-one`,
			wantField: "project_dir",
			wantValue: "site-one",
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
			case "path":
				got = tc.Path
			case "project_dir":
				got = tc.ProjectDir
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

func TestNativeToolCallToToolCall_ConvertsCustomToolShortcut(t *testing.T) {
	native := openai.ToolCall{
		ID:   "call_custom_shortcut",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "tool__hello_custom.py",
			Arguments: `{"params":{"city":"Berlin","units":"metric"}}`,
		},
	}

	tc := NativeToolCallToToolCall(native, nil)
	if tc.NativeArgsMalformed {
		t.Fatalf("unexpected malformed flag: %q", tc.NativeArgsError)
	}
	if tc.Action != "run_tool" {
		t.Fatalf("Action = %q, want run_tool", tc.Action)
	}
	if tc.Name != "hello_custom.py" {
		t.Fatalf("Name = %q, want hello_custom.py", tc.Name)
	}
	args := decodeRunToolArgs(tc).Args
	if len(args) != 1 || !strings.Contains(args[0], `"city":"Berlin"`) || !strings.Contains(args[0], `"units":"metric"`) {
		t.Fatalf("Args = %#v, want one JSON argument with params", args)
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
		EmailEnabled:  true, AgentMailEnabled: true, NotesEnabled: true, InvasionControlEnabled: true,
		AnsibleEnabled: true, MCPEnabled: true, HomepageRegistryEnabled: true,
		RemoteControlEnabled: true, ImageGenerationEnabled: true, VideoGenerationEnabled: true,
		MemoryEnabled: true, KnowledgeGraphEnabled: true, SecretsVaultEnabled: true,
		SchedulerEnabled: true, StopProcessEnabled: true,
		MemoryMaintenanceEnabled: true, MemoryAnalysisEnabled: true,
		DocumentCreatorEnabled: true, HomepageAllowLocalServer: true,
		WebCaptureEnabled: true, NetworkPingEnabled: true, WebScraperEnabled: true,
		OpenSCADEnabled: true, S3Enabled: true,
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
		"retrieve_original_output":   true, // simple meta-tool for compressed output retrieval
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

func TestBuiltinToolSchemasExposeFocusedRemoteControlTools(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{RemoteControlEnabled: true})
	names := toolNames(schemas)
	for _, legacy := range []string{"remote_control"} {
		if containsName(names, legacy) {
			t.Fatalf("legacy %s mega-tool should no longer be emitted as a native schema", legacy)
		}
	}
	for _, want := range []string{"remote_control_devices", "remote_control_shell", "remote_control_files", "remote_control_desktop"} {
		if !containsName(names, want) {
			t.Fatalf("%s schema missing from focused remote control set: %v", want, names)
		}
	}

	for _, schema := range schemas {
		if schema.Function == nil || schema.Function.Name != "remote_control_desktop" {
			continue
		}
		if !strings.Contains(strings.ToLower(schema.Function.Description), "screenshot") {
			t.Fatalf("remote_control_desktop description should mention screenshots: %s", schema.Function.Description)
		}
		params, ok := schema.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("remote_control_desktop parameters type = %T, want map[string]interface{}", schema.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("remote_control_desktop properties missing")
		}
		opProp, ok := props["operation"].(map[string]interface{})
		if !ok {
			t.Fatal("remote_control_desktop operation property missing")
		}
		for _, want := range []string{
			"desktop_screenshot", "desktop_permission_request", "desktop_input",
			"desktop_list_displays", "desktop_list_windows", "desktop_active_window", "desktop_host_info",
			"desktop_ui_tree", "desktop_ui_action",
			"desktop_browser_connect", "desktop_browser_snapshot", "desktop_browser_action", "desktop_browser_disconnect",
		} {
			if !containsInterfaceString(opProp["enum"], want) {
				t.Fatalf("remote_control_desktop operation enum missing %s: %#v", want, opProp["enum"])
			}
		}
		for _, want := range []string{
			"display_id", "window_id", "format", "quality", "include_data_base64",
			"kind", "x", "y", "absolute", "button", "input_action", "key", "code", "text",
			"element_id", "selector", "endpoint", "include_html", "value",
		} {
			if _, ok := props[want]; !ok {
				t.Fatalf("remote_control_desktop properties missing desktop field %s", want)
			}
		}
		return
	}
	t.Fatal("remote_control_desktop schema not found")
}

func TestBuiltinToolSchemasExposeVercelDeleteProject(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{VercelEnabled: true})
	for _, schema := range schemas {
		if schema.Function == nil || schema.Function.Name != "vercel" {
			continue
		}
		params, ok := schema.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("vercel parameters type = %T, want map[string]interface{}", schema.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("vercel properties missing")
		}
		opProp, ok := props["operation"].(map[string]interface{})
		if !ok {
			t.Fatal("vercel operation property missing")
		}
		enumVals, ok := opProp["enum"].([]string)
		if !ok {
			t.Fatalf("vercel operation enum type = %T, want []string", opProp["enum"])
		}
		for _, op := range enumVals {
			if op == "delete_project" {
				return
			}
		}
		t.Fatal("vercel schema must expose delete_project behind config permissions")
	}
	t.Fatal("vercel schema not found")
}

func TestBuiltinToolSchemasExposeFocusedInvasionControlTools(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{InvasionControlEnabled: true})
	names := toolNames(schemas)
	if containsName(names, "invasion_control") {
		t.Fatal("legacy invasion_control mega-tool should no longer be emitted as a native schema")
	}
	for _, want := range []string{"invasion_nests", "invasion_tasks", "invasion_artifacts"} {
		if !containsName(names, want) {
			t.Fatalf("%s schema missing from focused invasion set: %v", want, names)
		}
	}

	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "invasion_tasks" {
			continue
		}
		if !strings.Contains(strings.ToLower(s.Function.Description), "egg names are not tool names") {
			t.Fatalf("invasion_tasks description should warn about egg-name/tool-name collisions, got %q", s.Function.Description)
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("invasion_tasks parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("invasion_tasks properties type = %T", params["properties"])
		}
		if _, ok := props["egg_name"]; !ok {
			t.Fatal("invasion_tasks schema should expose egg_name")
		}
		if _, ok := props["task_id"]; !ok {
			t.Fatal("invasion_tasks schema should expose task_id")
		}
		return
	}

	t.Fatal("invasion_tasks schema not found")
}

func TestFocusedInvasionSchemasExposeAllDispatchOperations(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{InvasionControlEnabled: true})
	wantByTool := map[string][]string{
		"invasion_nests": {
			"list_nests", "list_eggs", "nest_status", "assign_egg", "hatch_egg", "stop_egg", "egg_status",
		},
		"invasion_tasks": {
			"send_task", "task_status", "get_result", "list_egg_messages", "ack_egg_message", "send_host_message", "send_secret",
		},
		"invasion_artifacts": {
			"list_artifacts", "get_artifact", "read_artifact", "upload_artifact",
		},
	}

	for toolName, wantOps := range wantByTool {
		enum := focusedToolOperationEnum(t, schemas, toolName)
		for _, wantOp := range wantOps {
			if !containsName(enum, wantOp) {
				t.Fatalf("%s operation enum missing %s: %#v", toolName, wantOp, enum)
			}
		}
	}
}

func TestFocusedHomepageSchemaPromptsMatchOperationEnums(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{HomepageEnabled: true, NetlifyEnabled: true})
	deployOps := focusedToolOperationEnum(t, schemas, "homepage_deploy")
	for _, wantOp := range []string{"build", "dev", "publish_local", "webserver_start", "webserver_stop", "webserver_status", "test_connection", "tunnel", "deploy", "deploy_netlify", "deploy_vercel"} {
		if !containsName(deployOps, wantOp) {
			t.Fatalf("homepage_deploy operation enum missing prompted operation %s: %#v", wantOp, deployOps)
		}
	}

	qualityOps := focusedToolOperationEnum(t, schemas, "homepage_quality")
	if !containsName(qualityOps, "optimize_images") {
		t.Fatalf("homepage_quality operation enum missing prompted operation optimize_images: %#v", qualityOps)
	}
}

func focusedToolOperationEnum(t *testing.T, schemas []openai.Tool, toolName string) []string {
	t.Helper()
	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != toolName {
			continue
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("%s parameters type = %T, want map[string]interface{}", toolName, s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s properties type = %T, want map[string]interface{}", toolName, params["properties"])
		}
		op, ok := props["operation"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s operation property type = %T, want map[string]interface{}", toolName, props["operation"])
		}
		switch enum := op["enum"].(type) {
		case []string:
			return enum
		case []interface{}:
			out := make([]string, 0, len(enum))
			for _, raw := range enum {
				if value, ok := raw.(string); ok {
					out = append(out, value)
				}
			}
			return out
		default:
			t.Fatalf("%s operation enum type = %T, want []string", toolName, op["enum"])
		}
	}
	t.Fatalf("%s schema not found", toolName)
	return nil
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

func TestBuildNativeToolSchemasCoAgentOutputSchemaUsesJSONStringField(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{CoAgentEnabled: true}, nil)
	for _, toolSchema := range schemas {
		if toolSchema.Function == nil || toolSchema.Function.Name != "co_agent" {
			continue
		}
		params, ok := toolSchema.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("co_agent parameters type = %T", toolSchema.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("co_agent properties missing")
		}
		outputSchema, ok := props["output_schema"].(map[string]interface{})
		if !ok {
			t.Fatalf("output_schema property type = %T", props["output_schema"])
		}
		if outputSchema["type"] != "string" {
			t.Fatalf("output_schema type = %v, want string after native schema normalization", outputSchema["type"])
		}
		desc, _ := outputSchema["description"].(string)
		if !strings.Contains(strings.ToLower(desc), "json") {
			t.Fatalf("output_schema description = %q, want JSON guidance", desc)
		}
		return
	}
	t.Fatal("co_agent schema not found")
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

func TestNativeToolSchemaSnapshotPrecomputesStrictVariant(t *testing.T) {
	ff := ToolFeatureFlags{VirusTotalEnabled: true}
	snapshot := BuildNativeToolSchemaSnapshot(t.TempDir(), nil, ff, nil)
	full := snapshot.FullSchemas()
	strict := snapshot.StrictSchemas()

	if len(full) == 0 || len(strict) == 0 {
		t.Fatal("expected built-in schemas to be present")
	}
	if len(full) != len(strict) {
		t.Fatalf("strict schema count = %d, want %d", len(strict), len(full))
	}
	for i, schema := range strict {
		if schema.Function == nil {
			continue
		}
		if !schema.Function.Strict {
			t.Fatalf("strict schema %d (%s) did not set Function.Strict", i, schema.Function.Name)
		}
		params, ok := schema.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("strict schema %s parameters type = %T, want map[string]interface{}", schema.Function.Name, schema.Function.Parameters)
		}
		var violations []string
		collectStrictOpenAISchemaViolations(schema.Function.Name+".parameters", params, &violations)
		if len(violations) > 0 {
			t.Fatalf("strict schema %s has compatibility violations:\n%s", schema.Function.Name, strings.Join(violations, "\n"))
		}
	}
}

func TestNativeToolSchemaSnapshotStrictVariantDoesNotMutateFullVariant(t *testing.T) {
	ff := ToolFeatureFlags{AllowShell: true}
	snapshot := BuildNativeToolSchemaSnapshot(t.TempDir(), nil, ff, nil)
	full := snapshot.FullSchemas()
	strict := snapshot.StrictSchemas()

	if len(full) == 0 || len(strict) == 0 {
		t.Fatal("expected built-in schemas to be present")
	}

	fullByName := make(map[string]openai.Tool, len(full))
	for _, schema := range full {
		if schema.Function != nil {
			fullByName[schema.Function.Name] = schema
		}
	}
	var strictShell openai.Tool
	for _, schema := range strict {
		if schema.Function != nil && schema.Function.Name == "execute_shell" {
			strictShell = schema
			break
		}
	}
	fullShell, ok := fullByName["execute_shell"]
	if !ok || strictShell.Function == nil {
		t.Fatal("expected execute_shell in both full and strict variants")
	}
	if fullShell.Function.Strict {
		t.Fatal("full schema variant should not set Function.Strict")
	}

	strictParams, ok := strictShell.Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("expected strict tool params map")
	}
	strictProps, ok := strictParams["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected strict tool properties map")
	}
	strictProps["strict_probe"] = map[string]interface{}{"type": "string"}

	fullParams, ok := fullShell.Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("expected full tool params map")
	}
	fullProps, ok := fullParams["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected full tool properties map")
	}
	if _, exists := fullProps["strict_probe"]; exists {
		t.Fatal("strict schema mutation leaked into full schema snapshot")
	}
}

func TestToolFeatureFlagsKeyIncludesTTSEnabled(t *testing.T) {
	withoutTTS := ToolFeatureFlags{LDAPEnabled: true}
	withTTS := withoutTTS
	withTTS.TTSEnabled = true

	if withoutTTS.Key() == withTTS.Key() {
		t.Fatal("expected TTSEnabled to change the native tool schema cache key")
	}
}

func TestBuildNativeToolSchemasCacheSeparatesTTSFlag(t *testing.T) {
	withoutTTS := ToolFeatureFlags{LDAPEnabled: true}
	withTTS := withoutTTS
	withTTS.TTSEnabled = true

	withoutNames := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, withoutTTS, nil))
	if containsName(withoutNames, "tts") {
		t.Fatalf("did not expect tts schema when TTSEnabled=false, got %v", withoutNames)
	}

	withNames := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, withTTS, nil))
	if !containsName(withNames, "tts") {
		t.Fatalf("expected tts schema when TTSEnabled=true, got %v", withNames)
	}
}

func TestBuildNativeToolSchemasGatesComposioCall(t *testing.T) {
	disabled := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{}, nil))
	if containsName(disabled, "composio_call") {
		t.Fatalf("did not expect composio_call schema when ComposioEnabled=false, got %v", disabled)
	}

	enabled := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{ComposioEnabled: true}, nil))
	if !containsName(enabled, "composio_call") {
		t.Fatalf("expected composio_call schema when ComposioEnabled=true, got %v", enabled)
	}
}

func TestToolFeatureFlagsKeyIncludesComposioEnabled(t *testing.T) {
	withoutComposio := ToolFeatureFlags{LDAPEnabled: true}
	withComposio := withoutComposio
	withComposio.ComposioEnabled = true

	if withoutComposio.Key() == withComposio.Key() {
		t.Fatal("expected ComposioEnabled to change the native tool schema cache key")
	}
}

func TestAllBuiltinToolFeatureFlagsIncludesTTS(t *testing.T) {
	names := builtinToolNames(allBuiltinToolFeatureFlags())
	if !containsName(names, "tts") {
		t.Fatalf("expected all-builtin feature flags to include tts, got %v", names)
	}
}

func TestAllBuiltinToolFeatureFlagsEnablesEveryFlag(t *testing.T) {
	flags := reflect.ValueOf(allBuiltinToolFeatureFlags())
	fields := reflect.TypeOf(ToolFeatureFlags{})

	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		if !flags.Field(i).Bool() {
			t.Fatalf("allBuiltinToolFeatureFlags leaves %s disabled", field.Name)
		}
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

func TestNativeToolCallToToolCallDecodesJSONStringObjectArgs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_execute_skill",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "execute_skill",
			Arguments: `{"skill":"paperless","skill_args":"{\"operation\":\"search\",\"name\":\"Invoices\",\"limit\":3}"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.NativeArgsMalformed {
		t.Fatalf("did not expect malformed args: %s", tc.NativeArgsError)
	}
	if tc.Action != "execute_skill" {
		t.Fatalf("Action = %q, want execute_skill", tc.Action)
	}
	if got, _ := tc.SkillArgs["name"].(string); got != "Invoices" {
		t.Fatalf("SkillArgs[name] = %q, want Invoices", got)
	}
	if got, _ := tc.Params["operation"].(string); got != "search" {
		t.Fatalf("Params[operation] = %q, want search", got)
	}
}

func TestNativeToolCallToToolCallIgnoresNonObjectPropertiesString(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_kg_search_with_bad_properties",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "knowledge_graph",
			Arguments: `{"operation":"search","query":"Rosemarie","properties":"unused"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.NativeArgsMalformed {
		t.Fatalf("did not expect malformed args: %s", tc.NativeArgsError)
	}
	if tc.Action != "knowledge_graph" {
		t.Fatalf("Action = %q, want knowledge_graph", tc.Action)
	}
	if tc.Operation != "search" {
		t.Fatalf("Operation = %q, want search", tc.Operation)
	}
	if tc.Query != "Rosemarie" {
		t.Fatalf("Query = %q, want Rosemarie", tc.Query)
	}
	if len(tc.Properties) != 0 {
		t.Fatalf("Properties = %#v, want empty map for non-object properties string", tc.Properties)
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

func TestNativeToolCallToToolCallFocusedHomepageFileLegacyActionOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_homepage_file_write",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "homepage_file",
			Arguments: `{"action":"write_file","path":"demo/index.html","content":"ok"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "homepage_file" {
		t.Fatalf("Action = %q, want homepage_file", tc.Action)
	}
	if tc.Operation != "write_file" {
		t.Fatalf("Operation = %q, want write_file", tc.Operation)
	}
	if tc.SubOperation != "" {
		t.Fatalf("SubOperation = %q, want empty", tc.SubOperation)
	}
}

func TestNativeToolCallToToolCallFocusedHomepageFileDoesNotTreatFileOperationAsEditSubOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_homepage_file_edit_bad_alias",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "homepage_file",
			Arguments: `{"operation":"edit_file","path":"demo/index.html","action":"write_file","content":"ok"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.Action != "homepage_file" {
		t.Fatalf("Action = %q, want homepage_file", tc.Action)
	}
	if tc.Operation != "edit_file" {
		t.Fatalf("Operation = %q, want edit_file", tc.Operation)
	}
	if tc.SubOperation == "write_file" {
		t.Fatalf("SubOperation = %q, want a valid edit operation or empty", tc.SubOperation)
	}
}

func TestNativeToolCallToToolCallHomepageStringBoolDraft(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	native := openai.ToolCall{
		ID:   "call_homepage_deploy",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "homepage",
			Arguments: `{"operation":"deploy_netlify","project_dir":"ki-news","build_dir":"dist","draft":"false"}`,
		},
	}

	tc := NativeToolCallToToolCall(native, logger)
	if tc.NativeArgsMalformed {
		t.Fatalf("did not expect malformed args: %s", tc.NativeArgsError)
	}
	if tc.Action != "homepage" {
		t.Fatalf("Action = %q, want homepage", tc.Action)
	}
	if tc.Operation != "deploy_netlify" {
		t.Fatalf("Operation = %q, want deploy_netlify", tc.Operation)
	}
	if tc.Draft {
		t.Fatal("Draft = true, want false")
	}
	if got, ok := tc.Params["draft"].(bool); !ok || got {
		t.Fatalf("Params[draft] = %#v, want boolean false", tc.Params["draft"])
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

func TestBuiltinToolSchemasIncludeTomlEditor(t *testing.T) {
	fullSchemas := builtinToolSchemas(ToolFeatureFlags{AllowFilesystemWrite: true})
	if !containsName(toolNames(fullSchemas), "toml_editor") {
		t.Fatalf("writable schemas missing toml_editor: %v", toolNames(fullSchemas))
	}

	readOnlySchemas := builtinToolSchemas(ToolFeatureFlags{AllowFilesystemWrite: false})
	if !containsName(toolNames(readOnlySchemas), "toml_editor") {
		t.Fatalf("read-only schemas missing toml_editor: %v", toolNames(readOnlySchemas))
	}
}

func TestBuiltinToolSchemasIncludeCertificateManager(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{})
	if !containsName(toolNames(schemas), "certificate_manager") {
		t.Fatalf("schemas missing certificate_manager: %v", toolNames(schemas))
	}
}

func TestBuiltinToolSchemasHomepageUsesFocusedTools(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{HomepageEnabled: true, NetlifyEnabled: true})
	names := toolNames(schemas)
	if containsName(names, "homepage") {
		t.Fatal("legacy homepage mega-tool should no longer be emitted as a native schema")
	}
	for _, want := range []string{"homepage_project", "homepage_file", "homepage_quality", "homepage_deploy", "homepage_git"} {
		if !containsName(names, want) {
			t.Fatalf("%s schema missing from focused homepage set: %v", want, names)
		}
	}

	var homepageFileProps map[string]interface{}
	homepageFileDescription := ""
	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "homepage_file" {
			continue
		}
		homepageFileDescription = s.Function.Description
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("homepage_file parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("homepage_file properties missing")
		}
		homepageFileProps = props
		break
	}

	if homepageFileProps == nil {
		t.Fatal("homepage_file schema not found")
	}
	if _, ok := homepageFileProps["sub_operation"]; !ok {
		t.Fatal("homepage_file schema missing sub_operation property")
	}
	if _, ok := homepageFileProps["file_path"]; !ok {
		t.Fatal("homepage_file schema missing file_path alias for path")
	}
	if _, ok := homepageFileProps["action"]; ok {
		t.Fatal("homepage_file schema should not expose action as edit sub-operation field")
	}
	if !strings.Contains(strings.ToLower(homepageFileDescription), "homepage workspace") {
		t.Fatalf("homepage_file description should contain concise workspace guidance: %s", homepageFileDescription)
	}
}

func TestToolCallUnmarshalToleratesStringItemsField(t *testing.T) {
	raw := []byte(`{"operation":"init_project","name":"ki-news","items":"not a valid batch array"}`)

	var tc ToolCall
	if err := json.Unmarshal(raw, &tc); err != nil {
		t.Fatalf("ToolCall unmarshal should tolerate malformed items fields: %v", err)
	}
	if tc.Operation != "init_project" || tc.Name != "ki-news" {
		t.Fatalf("decoded tool call lost fields: %#v", tc)
	}
	if len(tc.Items) != 0 {
		t.Fatalf("malformed string items should not populate typed Items, got: %#v", tc.Items)
	}
	if got, _ := tc.Params["_items_raw"].(string); got != "not a valid batch array" {
		t.Fatalf("expected raw malformed items to be preserved in Params, got: %#v", tc.Params)
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

func TestBuiltinToolSchemasExposeNetlifyDeleteSite(t *testing.T) {
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
			if op == "delete_site" {
				return
			}
		}
		t.Fatal("netlify schema must expose delete_site behind config permissions")
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

func TestBuildNativeToolSchemasSkipsProviderInvalidDynamicShortcutNames(t *testing.T) {
	skillsDir := t.TempDir()
	badSkillManifest := `{
  "name": "bad skill",
  "description": "Invalid native shortcut name",
  "executable": "bad_skill.py",
  "parameters": {}
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "bad_skill.json"), []byte(badSkillManifest), 0o644); err != nil {
		t.Fatalf("write bad skill manifest: %v", err)
	}
	goodSkillManifest := `{
  "name": "good_skill",
  "description": "Valid native shortcut name",
  "executable": "good_skill.py",
  "parameters": {}
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "good_skill.json"), []byte(goodSkillManifest), 0o644); err != nil {
		t.Fatalf("write good skill manifest: %v", err)
	}

	toolsDir := t.TempDir()
	manifestJSON := `{
  "version": 2,
  "tools": {
    "bad tool.py": "Invalid native shortcut name",
    "good_tool.py": "Valid native shortcut name"
  }
}`
	if err := os.WriteFile(filepath.Join(toolsDir, "manifest.json"), []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("write custom tool manifest: %v", err)
	}

	schemas := BuildNativeToolSchemas(skillsDir, tools.NewManifest(toolsDir), ToolFeatureFlags{}, nil)
	names := make(map[string]bool, len(schemas))
	for _, toolSchema := range schemas {
		if toolSchema.Function == nil {
			continue
		}
		if !providerNativeToolNamePattern.MatchString(toolSchema.Function.Name) {
			t.Fatalf("provider-invalid function name emitted: %q", toolSchema.Function.Name)
		}
		names[toolSchema.Function.Name] = true
	}

	if names["skill__bad skill"] {
		t.Fatal("did not expect provider-invalid skill shortcut")
	}
	if names["tool__bad tool.py"] {
		t.Fatal("did not expect provider-invalid custom tool shortcut")
	}
	if !names["skill__good_skill"] {
		t.Fatal("expected valid skill shortcut to remain")
	}
	if !names["tool__good_tool.py"] {
		t.Fatal("expected valid custom tool shortcut to remain")
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

func TestSelectedNativeToolDescriptionsStayCompact(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, allBuiltinToolFeatureFlags(), nil)
	descriptions := make(map[string]string, len(schemas))
	for _, toolSchema := range schemas {
		if toolSchema.Function == nil {
			continue
		}
		descriptions[toolSchema.Function.Name] = toolSchema.Function.Description
	}

	tests := []struct {
		name         string
		requiredText []string
	}{
		{name: "discover_tools", requiredText: []string{"tool catalog", "get_tool_info"}},
		{name: "invoke_tool", requiredText: []string{"discover_tools", "call_method=invoke_tool"}},
		{name: "retrieve_original_output", requiredText: []string{"archived original output", "compressed"}},
		{name: "document_creator", requiredText: []string{"PDF", "document backend"}},
		{name: "media_registry", requiredText: []string{"Search", "media registry"}},
		{name: "homepage_registry", requiredText: []string{"homepage/web", "deploy history", "project history", "add_history", "list_history"}},
		{name: "web_capture", requiredText: []string{"PNG", "PDF", "Chromium"}},
		{name: "web_performance_audit", requiredText: []string{"page load", "Chromium"}},
		{name: "browser_automation", requiredText: []string{"browser sidecar", "screenshots"}},
		{name: "manage_updates", requiredText: []string{"AuraGo updates", "user approval"}},
		{name: "execute_sudo", requiredText: []string{"sudo", "elevated privileges"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			description, ok := descriptions[tt.name]
			if !ok {
				t.Fatalf("%s schema description not found", tt.name)
			}
			if description == "" {
				t.Fatalf("%s description is empty", tt.name)
			}
			if len(description) > 180 {
				t.Fatalf("%s description has %d chars, want <= 180: %q", tt.name, len(description), description)
			}
			for _, want := range tt.requiredText {
				if !strings.Contains(description, want) {
					t.Fatalf("%s description missing %q: %q", tt.name, want, description)
				}
			}
		})
	}
}

func TestBuildNativeToolSchemasDoesNotExposeFreeObjectArguments(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, allBuiltinToolFeatureFlags(), nil)
	var violations []string
	for _, toolSchema := range schemas {
		if toolSchema.Function == nil {
			continue
		}
		params, ok := toolSchema.Function.Parameters.(map[string]interface{})
		if !ok {
			continue
		}
		collectFreeObjectSchemaViolations(toolSchema.Function.Name+".parameters", params, true, &violations)
	}
	if len(violations) > 0 {
		t.Fatalf("native tool schemas expose provider-fragile free object arguments:\n%s", strings.Join(violations, "\n"))
	}
}

func collectFreeObjectSchemaViolations(path string, node map[string]interface{}, isRoot bool, violations *[]string) {
	if node["type"] == "object" {
		props, _ := node["properties"].(map[string]interface{})
		if !isRoot {
			if len(props) == 0 {
				*violations = append(*violations, path+" has no concrete properties")
			}
			if ap, ok := node["additionalProperties"]; ok && ap != false {
				*violations = append(*violations, path+" allows additionalProperties")
			}
		}
		for name, raw := range props {
			if child, ok := raw.(map[string]interface{}); ok {
				collectFreeObjectSchemaViolations(path+"."+name, child, false, violations)
			}
		}
	}
	if items, ok := node["items"].(map[string]interface{}); ok {
		collectFreeObjectSchemaViolations(path+"[]", items, false, violations)
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if arr, ok := node[key].([]interface{}); ok {
			for i, raw := range arr {
				if child, ok := raw.(map[string]interface{}); ok {
					collectFreeObjectSchemaViolations(fmt.Sprintf("%s.%s[%d]", path, key, i), child, false, violations)
				}
			}
		}
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

func TestNormalizeProviderFragileObjectSchemasConvertsFreeObjectFields(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"parameters": map[string]interface{}{
				"type":                 "object",
				"description":          "Webhook parameters",
				"additionalProperties": true,
			},
			"known": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	normalizeProviderFragileObjectSchemas(schema)

	props := schema["properties"].(map[string]interface{})
	parameters := props["parameters"].(map[string]interface{})
	if parameters["type"] != "string" {
		t.Fatalf("free object field type = %v, want string", parameters["type"])
	}
	known := props["known"].(map[string]interface{})
	if known["type"] != "object" {
		t.Fatalf("concrete object field type = %v, want object", known["type"])
	}
}

func TestNormalizeStrictSchemaPreservesExplicitRequiredProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": prop("string", "Operation"),
			"category":  prop("string", "Optional category"),
			"nested": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":  prop("string", "Name"),
					"limit": prop("integer", "Limit"),
				},
				"required": []string{"name"},
			},
		},
		"required": []string{"operation"},
	}

	normalizeStrictSchemaRequiredRec(schema)

	required := schema["required"]
	if !containsRequiredValue(required, "operation") {
		t.Fatalf("top-level required %#v missing operation", required)
	}
	for _, optional := range []string{"category", "nested"} {
		if containsRequiredValue(required, optional) {
			t.Fatalf("top-level required %#v should not add optional property %q", required, optional)
		}
	}
	nested := schema["properties"].(map[string]interface{})["nested"].(map[string]interface{})
	nestedRequired := nested["required"]
	if !containsRequiredValue(nestedRequired, "name") {
		t.Fatalf("nested required %#v missing name", nestedRequired)
	}
	if containsRequiredValue(nestedRequired, "limit") {
		t.Fatalf("nested required %#v should not add optional limit", nestedRequired)
	}
}

func TestNormalizeStrictSchemaHandlesNilSchemaMap(t *testing.T) {
	normalizeStrictSchemaRequiredRec(nil)
	injectAdditionalPropertiesRec(nil)
}

func TestNormalizeStrictSchemaAddsMissingArrayItems(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"values": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "array",
				},
			},
		},
	}

	normalizeStrictSchemaRequiredRec(schema)

	values := schema["properties"].(map[string]interface{})["values"].(map[string]interface{})
	row := values["items"].(map[string]interface{})
	if _, ok := row["items"].(map[string]interface{}); !ok {
		t.Fatalf("nested array schema missing items after normalization: %#v", row)
	}
}

func TestNormalizeStrictSchemaAddsTypeToEmptyArrayItems(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"depends_on": map[string]interface{}{
				"type":        "array",
				"description": "Dependencies as IDs or indices",
				"items":       map[string]interface{}{},
			},
		},
	}

	normalizeStrictSchemaRequiredRec(schema)

	dependsOn := schema["properties"].(map[string]interface{})["depends_on"].(map[string]interface{})
	items := dependsOn["items"].(map[string]interface{})
	if items["type"] != "string" {
		t.Fatalf("empty array items type = %v, want string", items["type"])
	}
}

func TestBuildNativeToolSchemasAreStrictOpenAICompatibleAfterNormalization(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, allBuiltinToolFeatureFlags(), nil)
	var violations []string

	for _, toolSchema := range schemas {
		if toolSchema.Function == nil {
			continue
		}
		params, ok := toolSchema.Function.Parameters.(map[string]interface{})
		if !ok {
			violations = append(violations, toolSchema.Function.Name+".parameters is not an object schema")
			continue
		}
		normalizeStrictSchemaRequiredRec(params)
		collectStrictOpenAISchemaViolations(toolSchema.Function.Name+".parameters", params, &violations)
	}

	if len(violations) > 0 {
		t.Fatalf("native tool schemas are not OpenAI strict-compatible:\n%s", strings.Join(violations, "\n"))
	}
}

func collectStrictOpenAISchemaViolations(path string, node map[string]interface{}, violations *[]string) {
	if _, hasType := node["type"]; !hasType && !hasSchemaCombinator(node) {
		*violations = append(*violations, path+" schema is missing type")
	}
	switch node["type"] {
	case "object":
		props, _ := node["properties"].(map[string]interface{})
		if requiredRaw, exists := node["required"]; exists {
			switch typed := requiredRaw.(type) {
			case []string:
				for _, name := range typed {
					if _, ok := props[name]; !ok {
						*violations = append(*violations, path+" required references unknown property "+name)
					}
				}
			case []interface{}:
				for _, raw := range typed {
					name, ok := raw.(string)
					if !ok {
						*violations = append(*violations, path+" required contains non-string value")
						continue
					}
					if _, ok := props[name]; !ok {
						*violations = append(*violations, path+" required references unknown property "+name)
					}
				}
			default:
				*violations = append(*violations, path+" required must be a string array when present")
			}
		}
		if node["additionalProperties"] != false {
			*violations = append(*violations, path+" object must set additionalProperties=false")
		}
		for name, raw := range props {
			if child, ok := raw.(map[string]interface{}); ok {
				collectStrictOpenAISchemaViolations(path+"."+name, child, violations)
			}
		}
	case "array":
		items, ok := node["items"].(map[string]interface{})
		if !ok {
			*violations = append(*violations, path+" array is missing object items schema")
			return
		}
		collectStrictOpenAISchemaViolations(path+"[]", items, violations)
	default:
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if arr, ok := node[key].([]interface{}); ok {
			for i, raw := range arr {
				if child, ok := raw.(map[string]interface{}); ok {
					collectStrictOpenAISchemaViolations(fmt.Sprintf("%s.%s[%d]", path, key, i), child, violations)
				}
			}
		}
	}
}

func containsRequiredValue(items interface{}, target string) bool {
	switch typed := items.(type) {
	case []string:
		return containsRequiredString(typed, target)
	case []interface{}:
		return containsRequiredInterfaceString(typed, target)
	default:
		return false
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
	if containsRequiredValue(executeShellParams["required"], "_todo") {
		t.Fatalf("_todo should remain optional, required=%#v", executeShellParams["required"])
	}
	customProps := customParams["properties"].(map[string]interface{})
	if _, ok := customProps["_todo"]; ok {
		t.Fatal("did not expect custom tool schema to include _todo")
	}
}

func TestFileEditorSchemaWarnsAgainstVirtualDesktopPaths(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{AllowFilesystemWrite: true}, nil)
	var description string
	for _, toolSchema := range schemas {
		if toolSchema.Function != nil && toolSchema.Function.Name == "file_editor" {
			description = toolSchema.Function.Description
			break
		}
	}
	if description == "" {
		t.Fatal("file_editor schema not found")
	}
	for _, want := range []string{"Apps/", "Widgets/", "virtual_desktop"} {
		if !strings.Contains(description, want) {
			t.Fatalf("file_editor description missing %q: %s", want, description)
		}
	}
}

func TestBuiltinToolSchemasVirtualDesktopUsesFocusedTools(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{
		VirtualDesktopEnabled: true,
		OfficeDocumentEnabled: true,
		OfficeWorkbookEnabled: true,
	})
	names := toolNames(schemas)
	if containsName(names, "virtual_desktop") {
		t.Fatal("legacy virtual_desktop mega-tool should no longer be emitted as a native schema")
	}
	for _, want := range []string{"virtual_desktop_files", "virtual_desktop_apps", "virtual_desktop_widgets", "office_document", "office_workbook"} {
		if !containsName(names, want) {
			t.Fatalf("%s schema missing from focused virtual desktop set: %v", want, names)
		}
	}

	for _, s := range schemas {
		if s.Function == nil || s.Function.Name != "virtual_desktop_files" {
			continue
		}
		params, ok := s.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("virtual_desktop_files parameters type = %T, want map[string]interface{}", s.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("virtual_desktop_files properties missing")
		}
		opProp, ok := props["operation"].(map[string]interface{})
		if !ok {
			t.Fatal("virtual_desktop_files operation property missing")
		}
		enum := fmt.Sprint(opProp["enum"])
		for _, forbidden := range []string{"read_document", "write_document", "read_workbook", "write_workbook"} {
			if strings.Contains(enum, forbidden) {
				t.Fatalf("virtual_desktop_files should route Office work to office tools, found %s in %v", forbidden, opProp["enum"])
			}
		}
		return
	}
	t.Fatal("virtual_desktop_files schema not found")
}

func TestBuiltinToolSchemasIncludesOpenSCADRenderWhenEnabled(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{OpenSCADEnabled: true})
	var params map[string]interface{}
	for _, schema := range schemas {
		if schema.Function != nil && schema.Function.Name == "openscad_render" {
			params = schema.Function.Parameters.(map[string]interface{})
			break
		}
	}
	if params == nil {
		t.Fatal("openscad_render schema not found")
	}
	props := params["properties"].(map[string]interface{})
	if _, ok := props["source_scad"]; !ok {
		t.Fatalf("openscad_render source_scad property missing: %#v", props)
	}
	exports := props["exports"].(map[string]interface{})
	items := exports["items"].(map[string]interface{})
	for _, want := range []string{"png", "stl", "3mf", "off", "amf", "dxf", "svg", "pdf", "csg", "echo"} {
		if !containsInterfaceString(items["enum"], want) {
			t.Fatalf("openscad_render exports enum missing %s: %#v", want, items["enum"])
		}
	}
	defines := props["defines"].(map[string]interface{})
	defineItems := defines["items"].(map[string]interface{})
	if defineItems["additionalProperties"] != false {
		t.Fatalf("openscad_render defines item must be strict, got additionalProperties=%#v", defineItems["additionalProperties"])
	}
	if !containsRequiredValue(defineItems["required"], "name") || !containsRequiredValue(defineItems["required"], "value") {
		t.Fatalf("openscad_render defines item required = %#v, want name and value", defineItems["required"])
	}
}

func TestFilesystemSchemaIncludesHashlineReadOption(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags ToolFeatureFlags
	}{
		{name: "write-enabled", flags: ToolFeatureFlags{AllowFilesystemWrite: true}},
		{name: "read-only", flags: ToolFeatureFlags{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schemas := BuildNativeToolSchemas(t.TempDir(), nil, tc.flags, nil)
			var params map[string]interface{}
			for _, toolSchema := range schemas {
				if toolSchema.Function != nil && toolSchema.Function.Name == "filesystem" {
					params = toolSchema.Function.Parameters.(map[string]interface{})
					break
				}
			}
			if params == nil {
				t.Fatal("filesystem schema not found")
			}
			props := params["properties"].(map[string]interface{})
			if _, ok := props["include_hashes"]; !ok {
				t.Fatalf("filesystem schema missing include_hashes: %#v", props)
			}
		})
	}
}

func TestFileEditorSchemaIncludesHashlineOperationsAndAnchors(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{AllowFilesystemWrite: true}, nil)
	var params map[string]interface{}
	for _, toolSchema := range schemas {
		if toolSchema.Function != nil && toolSchema.Function.Name == "file_editor" {
			params = toolSchema.Function.Parameters.(map[string]interface{})
			break
		}
	}
	if params == nil {
		t.Fatal("file_editor schema not found")
	}

	props := params["properties"].(map[string]interface{})
	for _, want := range []string{"anchor_line", "anchor_hash"} {
		if _, ok := props[want]; !ok {
			t.Fatalf("file_editor schema missing %s: %#v", want, props)
		}
	}

	operation := props["operation"].(map[string]interface{})
	enum := operation["enum"].([]string)
	for _, want := range []string{"hashline_replace", "hashline_insert_after", "hashline_insert_before", "hashline_delete"} {
		found := false
		for _, got := range enum {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("file_editor operation enum missing %s: %#v", want, enum)
		}
	}
}

func TestExecuteShellSchemaWarnsAgainstVirtualDesktopPaths(t *testing.T) {
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{AllowShell: true}, nil)
	var description string
	for _, toolSchema := range schemas {
		if toolSchema.Function != nil && toolSchema.Function.Name == "execute_shell" {
			description = toolSchema.Function.Description
			break
		}
	}
	if description == "" {
		t.Fatal("execute_shell schema not found")
	}
	for _, want := range []string{"Virtual Desktop", "Apps/", "Widgets/", "agent_workspace/virtual_desktop", "homepage"} {
		if !strings.Contains(description, want) {
			t.Fatalf("execute_shell description missing %q: %s", want, description)
		}
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
