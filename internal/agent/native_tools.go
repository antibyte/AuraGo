package agent

// native_tools.go — Builds OpenAI-compatible tool schema definitions from the
// AuraGo built-in tool registry plus dynamically loaded skills and custom tools.
// Used when config.Agent.UseNativeFunctions = true.

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/tools"
)

var nativeToolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var builtinToolSchemaCache sync.Map

var nativeMalformedArgFieldPatterns = map[string]*regexp.Regexp{
	"prompt":    regexp.MustCompile(`"prompt"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"content":   regexp.MustCompile(`"content"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"query":     regexp.MustCompile(`"query"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"operation": regexp.MustCompile(`"operation"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"command":   regexp.MustCompile(`"command"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"code":      regexp.MustCompile(`"code"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"skill":     regexp.MustCompile(`"skill"\s*:\s*"((?:[^"\\]|\\.)*)`),
}

// prop creates a JSON Schema property entry.
func prop(typ, description string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": description}
}

// schema builds a standard object schema with required fields.
func schema(properties map[string]interface{}, required ...string) map[string]interface{} {
	s := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// tool creates an openai.Tool from a name, description, and parameters schema.
func tool(name, description string, params map[string]interface{}) openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  params,
		},
	}
}

// ToolFeatureFlags controls which optional tool schemas are included.
type ToolFeatureFlags struct {
	HomeAssistantEnabled    bool
	DockerEnabled           bool
	CoAgentEnabled          bool
	SudoEnabled             bool
	WebhooksEnabled         bool
	ProxmoxEnabled          bool
	OllamaEnabled           bool
	TailscaleEnabled        bool
	AnsibleEnabled          bool
	InvasionControlEnabled  bool
	GitHubEnabled           bool
	MQTTEnabled             bool
	AdGuardEnabled          bool
	UptimeKumaEnabled       bool
	MCPEnabled              bool
	SandboxEnabled          bool
	MeshCentralEnabled      bool
	HomepageEnabled         bool
	NetlifyEnabled          bool
	VercelEnabled           bool
	FirewallEnabled         bool
	EmailEnabled            bool
	CloudflareTunnelEnabled bool
	GoogleWorkspaceEnabled  bool
	OneDriveEnabled         bool
	VirusTotalEnabled       bool
	GolangciLintEnabled     bool
	ImageGenerationEnabled  bool
	MusicGenerationEnabled  bool
	VideoGenerationEnabled  bool
	TTSEnabled              bool
	RemoteControlEnabled    bool
	// Danger Zone toggles
	AllowShell               bool
	AllowPython              bool
	AllowFilesystemWrite     bool
	AllowNetworkRequests     bool
	AllowRemoteShell         bool
	AllowSelfUpdate          bool
	HomepageAllowLocalServer bool // Allow Python HTTP server fallback when Docker unavailable
	// Built-in tool toggles
	MemoryEnabled                bool
	KnowledgeGraphEnabled        bool
	SecretsVaultEnabled          bool
	SchedulerEnabled             bool
	NotesEnabled                 bool
	MissionsEnabled              bool
	StopProcessEnabled           bool
	InventoryEnabled             bool
	MemoryMaintenanceEnabled     bool
	WOLEnabled                   bool
	MediaRegistryEnabled         bool
	HomepageRegistryEnabled      bool
	ContactsEnabled              bool
	PlannerEnabled               bool
	JournalEnabled               bool
	MemoryAnalysisEnabled        bool
	DocumentCreatorEnabled       bool
	MediaConversionEnabled       bool
	VideoDownloadEnabled         bool
	VideoDownloadAllowDownload   bool
	VideoDownloadAllowTranscribe bool
	SendYouTubeVideoEnabled      bool
	WebCaptureEnabled            bool
	BrowserAutomationEnabled     bool
	NetworkPingEnabled           bool
	WebScraperEnabled            bool
	S3Enabled                    bool
	NetworkScanEnabled           bool
	FormAutomationEnabled        bool
	UPnPScanEnabled              bool
	// Jellyfin media server
	JellyfinEnabled bool
	// Obsidian knowledge management
	ObsidianEnabled bool
	// Chromecast / Google Cast
	ChromecastEnabled bool
	// Discord messaging
	DiscordEnabled bool
	// Telegram messaging
	TelegramEnabled bool
	// TrueNAS storage
	TrueNASEnabled bool
	// Koofr cloud storage
	KoofrEnabled bool
	// FritzBox sub-feature flags
	FritzBoxSystemEnabled    bool
	FritzBoxNetworkEnabled   bool
	FritzBoxTelephonyEnabled bool
	FritzBoxSmartHomeEnabled bool
	FritzBoxStorageEnabled   bool
	FritzBoxTVEnabled        bool
	// Telnyx integration flags
	TelnyxSMSEnabled  bool
	TelnyxCallEnabled bool
	// SQL Connections flag
	SQLConnectionsEnabled bool
	// Python secret injection
	PythonSecretInjectionEnabled bool
	// Daemon skills
	DaemonSkillsEnabled bool
	// LDAP integration
	LDAPEnabled bool
	// Paperless-ngx document management
	PaperlessNGXEnabled bool
}

// injectAdditionalPropertiesRec recursively sets additionalProperties: false
// on all object-typed nodes in a JSON Schema, but only if the node does NOT
// already have an explicit additionalProperties value. This preserves intentional
// open-schema fields like call_webhook.parameters and ldap.attributes.
func injectAdditionalPropertiesRec(m map[string]interface{}) {
	injectAdditionalPropertiesRecWithVisited(m, make(map[uintptr]struct{}))
}

func injectAdditionalPropertiesRecWithVisited(m map[string]interface{}, visited map[uintptr]struct{}) {
	if len(m) == 0 {
		return
	}
	ptr := reflect.ValueOf(m).Pointer()
	if _, seen := visited[ptr]; seen {
		return
	}
	visited[ptr] = struct{}{}

	if typ, ok := m["type"]; ok && typ == "object" {
		// Only set additionalProperties: false if it is not already explicitly set.
		// An explicit value can be true, false, or a JSON Schema object.
		if _, alreadySet := m["additionalProperties"]; !alreadySet {
			m["additionalProperties"] = false
		}
	}
	// Walk properties
	if props, ok := m["properties"].(map[string]interface{}); ok {
		for _, v := range props {
			if child, ok := v.(map[string]interface{}); ok {
				injectAdditionalPropertiesRecWithVisited(child, visited)
			}
		}
	}
	// Walk items (for arrays of objects)
	if items, ok := m["items"].(map[string]interface{}); ok {
		injectAdditionalPropertiesRecWithVisited(items, visited)
	}
	// Walk anyOf, allOf, oneOf
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if arr, ok := m[key].([]interface{}); ok {
			for _, v := range arr {
				if child, ok := v.(map[string]interface{}); ok {
					injectAdditionalPropertiesRecWithVisited(child, visited)
				}
			}
		}
	}
}

func extractMalformedJSONStringField(rawArgs, key string) string {
	re, ok := nativeMalformedArgFieldPatterns[key]
	if !ok {
		return ""
	}
	matches := re.FindStringSubmatch(rawArgs)
	if len(matches) <= 1 {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(matches[1], `\"`, `"`), `\\`, `\`)
}

// NativeToolCallToToolCall converts an OpenAI native ToolCall response to AuraGo's ToolCall struct.
// Arguments JSON is unmarshalled directly into the struct fields.
func NativeToolCallToToolCall(native openai.ToolCall, logger *slog.Logger) ToolCall {
	// Convert skill__name shortcut to execute_skill so the skill dispatcher handles it correctly.
	name := strings.TrimSpace(native.Function.Name)
	skillFromShortcut := ""
	if strings.HasPrefix(name, "skill__") {
		skillFromShortcut = strings.TrimPrefix(name, "skill__")
		if _, err := tools.ValidateSkillShortcutName(skillFromShortcut); err != nil {
			return ToolCall{
				IsTool:              true,
				Action:              "execute_skill",
				Skill:               skillFromShortcut,
				NativeCallID:        native.ID,
				NativeArgsMalformed: true,
				NativeArgsError:     err.Error(),
				NativeArgsRaw:       native.Function.Arguments,
			}
		}
		name = "execute_skill"
	}

	tc := ToolCall{
		IsTool:       true,
		Action:       name,
		Skill:        skillFromShortcut,
		NativeCallID: native.ID,
	}

	if !nativeToolNamePattern.MatchString(name) {
		tc.NativeArgsMalformed = true
		tc.NativeArgsError = "invalid native function name"
		tc.NativeArgsRaw = native.Function.Arguments
		return tc
	}

	if native.Function.Arguments == "" {
		return tc
	}

	// Unmarshal the arguments JSON into the ToolCall struct.
	// Pre-normalize arrays in string fields (e.g. "tags") to avoid type-mismatch errors.
	normalizedArgs := normalizeTagsInJSON(native.Function.Arguments)
	var rawMap map[string]interface{}
	rawMapOK := json.Unmarshal([]byte(normalizedArgs), &rawMap) == nil
	if rawMapOK {
		tc.Params = rawMap
		if decoded, ok := decodeExecuteSkillNativeToolCall(tc, normalizedArgs); ok {
			return decoded
		}
	}
	if err := json.Unmarshal([]byte(normalizedArgs), &tc); err != nil {
		tc.NativeArgsMalformed = true
		tc.NativeArgsError = err.Error()
		tc.NativeArgsRaw = native.Function.Arguments
		if logger != nil {
			logger.Warn("[NativeTools] Failed to unmarshal native tool arguments, using raw",
				"name", native.Function.Name, "error", err)
		}
		// Fallback 1: try to put the raw args into Params (works for valid-but-unexpected JSON)
		if !rawMapOK && json.Unmarshal([]byte(native.Function.Arguments), &rawMap) == nil {
			tc.Params = rawMap
		}
		// Fallback 2: for truncated/malformed JSON, extract known string fields via regex.
		// LLMs occasionally return truncated JSON (e.g. connection reset, token limit).
		// The beginning of the JSON is usually intact, so we can rescue key fields.
		if tc.Prompt == "" {
			tc.Prompt = extractMalformedJSONStringField(native.Function.Arguments, "prompt")
		}
		if tc.Content == "" {
			tc.Content = extractMalformedJSONStringField(native.Function.Arguments, "content")
		}
		if tc.Query == "" {
			tc.Query = extractMalformedJSONStringField(native.Function.Arguments, "query")
		}
		if tc.Operation == "" {
			tc.Operation = extractMalformedJSONStringField(native.Function.Arguments, "operation")
		}
		if tc.Command == "" {
			tc.Command = extractMalformedJSONStringField(native.Function.Arguments, "command")
		}
		if tc.Code == "" {
			tc.Code = extractMalformedJSONStringField(native.Function.Arguments, "code")
		}
		if name == "execute_skill" && tc.Skill == "" {
			tc.Skill = extractMalformedJSONStringField(native.Function.Arguments, "skill")
		}
		return tc
	}

	// Native function name is the canonical tool action. Some tools historically
	// used an "action" argument for a sub-operation, which can overwrite tc.Action
	// during unmarshal. Preserve that value separately and restore the tool name.
	if tc.Action != "" && tc.Action != name && tc.SubOperation == "" {
		tc.SubOperation = tc.Action
	}
	tc.Action = name

	// Handle execute_skill: LLM may use "skill_name" key
	if tc.Action == "execute_skill" && tc.Skill == "" {
		for _, key := range []string{"skill_name", "name"} {
			if tc.Params != nil {
				if v, ok := tc.Params[key].(string); ok && v != "" {
					tc.Skill = v
					break
				}
			}
		}
	}

	return tc
}

func decodeExecuteSkillNativeToolCall(base ToolCall, normalizedArgs string) (ToolCall, bool) {
	if base.Action != "execute_skill" {
		return base, false
	}

	var envelope executeSkillNativeEnvelope
	if err := json.Unmarshal([]byte(normalizedArgs), &envelope); err != nil {
		return base, false
	}

	skillName := strings.TrimSpace(envelope.Skill)
	if skillName == "" {
		skillName = strings.TrimSpace(envelope.SkillName)
	}
	if skillName == "" {
		return base, false
	}

	base.Skill = skillName
	params := decodeJSONObject(envelope.Params)
	skillArgs := decodeJSONObject(envelope.SkillArgs)
	if len(params) > 0 {
		base.Params = params
	}
	if len(skillArgs) > 0 {
		base.SkillArgs = skillArgs
		if len(params) == 0 {
			base.Params = skillArgs
		}
	}
	if base.SkillArgs == nil && len(base.Params) > 0 {
		base.SkillArgs = base.Params
	}
	if base.Params == nil && len(base.SkillArgs) > 0 {
		base.Params = base.SkillArgs
	}

	return base, true
}

// BuildNativeToolSchemas returns the full tool list: built-ins + registered skills + custom tools.
func BuildNativeToolSchemas(skillsDir string, manifest *tools.Manifest, ff ToolFeatureFlags, logger *slog.Logger) []openai.Tool {
	allTools := builtinToolSchemasCached(ff)
	builtinNames := allBuiltinToolNameSet()
	emittedNames := make(map[string]struct{}, len(allTools))
	for _, toolSchema := range allTools {
		if toolSchema.Function == nil || toolSchema.Function.Name == "" {
			continue
		}
		emittedNames[toolSchema.Function.Name] = struct{}{}
	}

	// Add skills as sub-variants of execute_skill (informational context; already handled by execute_skill schema)
	if skills, err := tools.ListSkills(skillsDir); err == nil {
		sort.SliceStable(skills, func(i, j int) bool {
			if skills[i].Name != skills[j].Name {
				return skills[i].Name < skills[j].Name
			}
			return skills[i].Executable < skills[j].Executable
		})
		for _, skill := range skills {
			if skill.Executable == "__builtin__" && skill.Name == "virustotal_scan" && !ff.VirusTotalEnabled {
				continue
			}
			if skill.Executable != "__builtin__" {
				if collisionName, ok := customToolBuiltinCollisionName(skill.Name, builtinNames); ok {
					if logger != nil {
						logger.Warn("[NativeTools] Skipping custom skill that collides with built-in tool",
							"skill", skill.Name,
							"builtin", collisionName,
						)
					}
					continue
				}
			}
			schemaName := "skill__" + skill.Name
			if _, exists := emittedNames[schemaName]; exists {
				continue
			}
			emittedNames[schemaName] = struct{}{}
			allTools = append(allTools, tool(
				schemaName,
				"(Skill) "+skill.Description+". Use execute_skill with skill='"+skill.Name+"'.",
				schema(map[string]interface{}{
					"skill_args": map[string]interface{}{
						"type":        "object",
						"description": "Arguments for this skill",
					},
				}),
			))
		}
	}

	// Add custom tools from manifest
	if manifest != nil {
		if entries, err := manifest.Load(); err == nil {
			names := make([]string, 0, len(entries))
			for name := range entries {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				description := entries[name]
				if collisionName, ok := customToolBuiltinCollisionName(name, builtinNames); ok {
					if logger != nil {
						logger.Warn("[NativeTools] Skipping custom tool that collides with built-in tool",
							"tool", name,
							"builtin", collisionName,
						)
					}
					continue
				}
				schemaName := "tool__" + name
				if _, exists := emittedNames[schemaName]; exists {
					continue
				}
				emittedNames[schemaName] = struct{}{}
				customToolProps := map[string]interface{}{
					"params": map[string]interface{}{
						"type":        "object",
						"description": "Parameters to pass to the tool",
					},
				}
				if ff.PythonSecretInjectionEnabled {
					customToolProps["vault_keys"] = map[string]interface{}{
						"type":        "array",
						"description": "List of vault secret key names to inject as AURAGO_SECRET_<KEY> environment variables.",
						"items":       map[string]interface{}{"type": "string"},
					}
					customToolProps["credential_ids"] = map[string]interface{}{
						"type":        "array",
						"description": "List of credential UUIDs to inject as AURAGO_CRED_<NAME>_USERNAME / _PASSWORD / _TOKEN environment variables.",
						"items":       map[string]interface{}{"type": "string"},
					}
				}
				allTools = append(allTools, tool(
					schemaName,
					"(Custom tool) "+description,
					schema(customToolProps),
				))
			}
		}
	}

	sort.SliceStable(allTools, func(i, j int) bool {
		return nativeToolSortName(allTools[i]) < nativeToolSortName(allTools[j])
	})

	// Inject _todo property into every tool schema so the agent can piggyback
	// a session-scoped task list on each tool call.
	//
	// Strict-mode compatibility (OpenAI Structured Outputs):
	//   - type must be a single string, not an array (no union types allowed)
	//   - every property in the schema must appear in "required"
	todoProperty := map[string]interface{}{
		"type":        "string",
		"description": "Session task list. '- [x] done' / '- [ ] pending', one per line. Update each call. Empty string if unused.",
	}
	builtinTodoNames := builtinToolNameSet(ff)
	for i := range allTools {
		if allTools[i].Function == nil || allTools[i].Function.Parameters == nil {
			continue
		}
		params, ok := allTools[i].Function.Parameters.(map[string]interface{})
		if !ok {
			continue
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, injectTodo := builtinTodoNames[allTools[i].Function.Name]; injectTodo {
			props["_todo"] = todoProperty
			// Add _todo to "required" so strict-mode schemas remain valid.
			// Tools that already declare a required array get _todo appended;
			// tools without one get a new required array containing ["_todo"].
			switch req := params["required"].(type) {
			case []string:
				if !containsRequiredString(req, "_todo") {
					params["required"] = append(req, "_todo")
				}
			case []interface{}:
				if !containsRequiredInterfaceString(req, "_todo") {
					params["required"] = append(req, "_todo")
				}
			default:
				params["required"] = []string{"_todo"}
			}
		}
		// CHANGE LOG 2026-04-11: OpenAI strict mode requires additionalProperties: false
		// on every object schema. The go-openai library does not auto-add this, so we
		// inject it here. Only affects strict-mode requests; non-strict calls ignore it.
		injectAdditionalPropertiesRec(params)
	}

	if logger != nil {
		logger.Info("[NativeTools] Built tool schemas", "count", len(allTools))
	}

	return allTools
}

func nativeToolSortName(schema openai.Tool) string {
	if schema.Function == nil {
		return ""
	}
	return schema.Function.Name
}

func containsRequiredString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsRequiredInterfaceString(items []interface{}, target string) bool {
	for _, item := range items {
		if value, ok := item.(string); ok && value == target {
			return true
		}
	}
	return false
}
