package agent

// native_tools.go — Builds OpenAI-compatible tool schema definitions from the
// AuraGo built-in tool registry plus dynamically loaded skills and custom tools.
// Used when config.Agent.UseNativeFunctions = true.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/tools"
)

var nativeToolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var providerNativeToolNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]{0,127}$`)

var builtinToolSchemaCache sync.Map
var dynamicToolSchemaCache sync.Map

type dynamicToolSchemaCacheKey struct {
	Flags               ToolFeatureFlags
	SkillsFingerprint   string
	ManifestFingerprint string
}

var nativeMalformedArgFieldPatterns = map[string]*regexp.Regexp{
	"prompt":      regexp.MustCompile(`"prompt"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"content":     regexp.MustCompile(`"content"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"query":       regexp.MustCompile(`"query"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"operation":   regexp.MustCompile(`"operation"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"command":     regexp.MustCompile(`"command"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"code":        regexp.MustCompile(`"code"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"skill":       regexp.MustCompile(`"skill"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"path":        regexp.MustCompile(`"path"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"file_path":   regexp.MustCompile(`"file_path"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"project_dir": regexp.MustCompile(`"project_dir"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"name":        regexp.MustCompile(`"name"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"device_id":   regexp.MustCompile(`"device_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"task_id":     regexp.MustCompile(`"task_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"artifact_id": regexp.MustCompile(`"artifact_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"inbox_id":    regexp.MustCompile(`"inbox_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"message_id":  regexp.MustCompile(`"message_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"thread_id":   regexp.MustCompile(`"thread_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
	"draft_id":    regexp.MustCompile(`"draft_id"\s*:\s*"((?:[^"\\]|\\.)*)`),
}

var nativeJSONStringObjectArgNames = map[string]struct{}{
	"action_input":       {},
	"arguments":          {},
	"changes":            {},
	"config":             {},
	"entry_attributes":   {},
	"headers":            {},
	"files":              {},
	"manifest":           {},
	"metadata":           {},
	"mcp_args":           {},
	"output_schema":      {},
	"parameters":         {},
	"params":             {},
	"ports":              {},
	"properties":         {},
	"service_data":       {},
	"skill_args":         {},
	"tool_args":          {},
	"widget":             {},
	"webhook_parameters": {},
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
	FrigateEnabled          bool
	ThreeDPrinterEnabled    bool
	OllamaEnabled           bool
	TailscaleEnabled        bool
	AnsibleEnabled          bool
	InvasionControlEnabled  bool
	GitHubEnabled           bool
	HuggingFaceEnabled      bool
	MQTTEnabled             bool
	AdGuardEnabled          bool
	UptimeKumaEnabled       bool
	GrafanaEnabled          bool
	MCPEnabled              bool
	ComposioEnabled         bool
	ManusEnabled            bool
	EvomapEnabled           bool
	SandboxEnabled          bool
	MeshCentralEnabled      bool
	HomepageEnabled         bool
	NetlifyEnabled          bool
	VercelEnabled           bool
	FirewallEnabled         bool
	EmailEnabled            bool
	AgentMailEnabled        bool
	CloudflareTunnelEnabled bool
	GoogleWorkspaceEnabled  bool
	OneDriveEnabled         bool
	WebDAVEnabled           bool
	VirusTotalEnabled       bool
	GolangciLintEnabled     bool
	ImageGenerationEnabled  bool
	MusicGenerationEnabled  bool
	VideoGenerationEnabled  bool
	TTSEnabled              bool
	RemoteControlEnabled    bool
	PackageManagerEnabled   bool
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
	WorkspaceSearchEnabled       bool
	SendYouTubeVideoEnabled      bool
	WebCaptureEnabled            bool
	BrowserAutomationEnabled     bool
	SpaceAgentEnabled            bool
	VirtualDesktopEnabled        bool
	VirtualComputersEnabled      bool
	OpenSCADEnabled              bool
	OfficeDocumentEnabled        bool
	OfficeWorkbookEnabled        bool
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
	// AgoDesk chat push messaging
	AgoDeskChatEnabled bool
	// TrueNAS storage
	TrueNASEnabled bool
	// Koofr cloud storage
	KoofrEnabled bool
	// FritzBox sub-feature flags
	FritzBoxSystemEnabled  bool
	FritzBoxNetworkEnabled bool
	// YepAPI services
	YepAPIEnabled            bool
	YepAPISEOEnabled         bool
	YepAPISERPEnabled        bool
	YepAPIScrapingEnabled    bool
	YepAPIYouTubeEnabled     bool
	YepAPITikTokEnabled      bool
	YepAPIInstagramEnabled   bool
	YepAPIAmazonEnabled      bool
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
	if m == nil {
		return
	}
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

func normalizeStrictSchemaRequiredRec(m map[string]interface{}) {
	normalizeStrictSchemaRequiredRecWithVisited(m, make(map[uintptr]struct{}))
}

func normalizeStrictSchemaRequiredRecWithVisited(m map[string]interface{}, visited map[uintptr]struct{}) {
	if m == nil {
		return
	}
	if len(m) == 0 {
		m["type"] = "string"
		return
	}
	ptr := reflect.ValueOf(m).Pointer()
	if _, seen := visited[ptr]; seen {
		return
	}
	visited[ptr] = struct{}{}

	if _, hasType := m["type"]; !hasType && !hasSchemaCombinator(m) {
		if props, ok := m["properties"].(map[string]interface{}); ok && len(props) > 0 {
			m["type"] = "object"
		} else if _, ok := m["items"]; ok {
			m["type"] = "array"
		} else {
			m["type"] = "string"
		}
	}

	normalizeExplicitRequiredList(m)
	if m["type"] == "array" {
		if _, ok := m["items"]; !ok {
			m["items"] = map[string]interface{}{"type": "string"}
		}
	}
	if props, ok := m["properties"].(map[string]interface{}); ok {
		for name, raw := range props {
			if child, ok := raw.(map[string]interface{}); ok {
				if name == "structured_output_schema" && schemaObjectNeedsJSONString(child, false) {
					props[name] = jsonObjectStringSchema(name, child)
					continue
				}
				normalizeStrictSchemaRequiredRecWithVisited(child, visited)
			}
		}
	}
	if items, ok := m["items"].(map[string]interface{}); ok {
		normalizeStrictSchemaRequiredRecWithVisited(items, visited)
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if arr, ok := m[key].([]interface{}); ok {
			for _, raw := range arr {
				if child, ok := raw.(map[string]interface{}); ok {
					normalizeStrictSchemaRequiredRecWithVisited(child, visited)
				}
			}
		}
	}
}

func normalizeExplicitRequiredList(m map[string]interface{}) {
	if m["type"] != "object" {
		return
	}
	props, _ := m["properties"].(map[string]interface{})
	if len(props) == 0 {
		delete(m, "required")
		return
	}
	requiredRaw, exists := m["required"]
	if !exists {
		return
	}
	seen := make(map[string]struct{})
	required := make([]string, 0)
	appendRequired := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := props[name]; !ok {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		required = append(required, name)
	}
	switch typed := requiredRaw.(type) {
	case []string:
		for _, name := range typed {
			appendRequired(name)
		}
	case []interface{}:
		for _, raw := range typed {
			if name, ok := raw.(string); ok {
				appendRequired(name)
			}
		}
	default:
		delete(m, "required")
		return
	}
	if len(required) == 0 {
		delete(m, "required")
		return
	}
	sort.Strings(required)
	m["required"] = required
}

func hasSchemaCombinator(m map[string]interface{}) bool {
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func normalizeProviderFragileObjectSchemas(m map[string]interface{}) {
	normalizeProviderFragileObjectSchemasWithVisited(m, make(map[uintptr]struct{}), true, "")
}

func normalizeProviderFragileObjectSchemasWithVisited(m map[string]interface{}, visited map[uintptr]struct{}, isRoot bool, fieldName string) {
	if len(m) == 0 {
		return
	}
	ptr := reflect.ValueOf(m).Pointer()
	if _, seen := visited[ptr]; seen {
		return
	}
	visited[ptr] = struct{}{}

	if props, ok := m["properties"].(map[string]interface{}); ok {
		for name, raw := range props {
			child, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			// Manus accepts its structured-output schema as a JSON object. Keep this
			// one API-defined object field native while retaining the compatibility
			// string fallback for every other provider-fragile free object.
			if name != "structured_output_schema" && schemaObjectNeedsJSONString(child, false) {
				props[name] = jsonObjectStringSchema(name, child)
				continue
			}
			normalizeProviderFragileObjectSchemasWithVisited(child, visited, false, name)
		}
	}
	if items, ok := m["items"].(map[string]interface{}); ok {
		if schemaObjectNeedsJSONString(items, false) {
			m["items"] = jsonObjectStringSchema(fieldName+" item", items)
		} else {
			normalizeProviderFragileObjectSchemasWithVisited(items, visited, false, fieldName+" item")
		}
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if arr, ok := m[key].([]interface{}); ok {
			for i, raw := range arr {
				child, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if schemaObjectNeedsJSONString(child, isRoot) {
					arr[i] = jsonObjectStringSchema(fieldName, child)
					continue
				}
				normalizeProviderFragileObjectSchemasWithVisited(child, visited, false, fieldName)
			}
		}
	}
}

func schemaObjectNeedsJSONString(m map[string]interface{}, isRoot bool) bool {
	if isRoot || m["type"] != "object" {
		return false
	}
	props, hasProps := m["properties"].(map[string]interface{})
	if !hasProps || len(props) == 0 {
		return true
	}
	if ap, ok := m["additionalProperties"]; ok && ap != false {
		return true
	}
	return false
}

func jsonObjectStringSchema(name string, original map[string]interface{}) map[string]interface{} {
	description, _ := original["description"].(string)
	description = strings.TrimSpace(description)
	if description == "" {
		description = strings.TrimSpace(name)
	}
	if description == "" {
		description = "JSON object"
	}
	if !strings.Contains(strings.ToLower(description), "json") {
		description += ". Provide as a JSON object string."
	} else {
		description += ". Provide as a JSON string."
	}
	return map[string]interface{}{
		"type":        "string",
		"description": description,
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
	customFromShortcut := ""
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
	if strings.HasPrefix(name, "tool__") {
		customFromShortcut = strings.TrimSpace(strings.TrimPrefix(name, "tool__"))
		name = "run_tool"
	}

	tc := ToolCall{
		IsTool:       true,
		Action:       name,
		Skill:        skillFromShortcut,
		Name:         customFromShortcut,
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
		decodeNativeJSONStringObjectArgs(rawMap)
		if customFromShortcut != "" {
			rawMap = normalizeCustomToolShortcutArgs(customFromShortcut, rawMap)
		}
		if normalizedBytes, err := json.Marshal(rawMap); err == nil {
			normalizedArgs = string(normalizedBytes)
		}
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
		if tc.Path == "" {
			tc.Path = extractMalformedJSONStringField(native.Function.Arguments, "path")
		}
		if tc.FilePath == "" {
			tc.FilePath = extractMalformedJSONStringField(native.Function.Arguments, "file_path")
		}
		if tc.ProjectDir == "" {
			tc.ProjectDir = extractMalformedJSONStringField(native.Function.Arguments, "project_dir")
		}
		if tc.Name == "" {
			tc.Name = extractMalformedJSONStringField(native.Function.Arguments, "name")
		}
		if tc.DeviceID == "" {
			tc.DeviceID = extractMalformedJSONStringField(native.Function.Arguments, "device_id")
		}
		if tc.TaskID == "" {
			tc.TaskID = extractMalformedJSONStringField(native.Function.Arguments, "task_id")
		}
		if tc.ArtifactID == "" {
			tc.ArtifactID = extractMalformedJSONStringField(native.Function.Arguments, "artifact_id")
		}
		if tc.InboxID == "" {
			tc.InboxID = extractMalformedJSONStringField(native.Function.Arguments, "inbox_id")
		}
		if tc.MessageID == "" {
			tc.MessageID = extractMalformedJSONStringField(native.Function.Arguments, "message_id")
		}
		if tc.ThreadID == "" {
			tc.ThreadID = extractMalformedJSONStringField(native.Function.Arguments, "thread_id")
		}
		if tc.DraftID == "" {
			tc.DraftID = extractMalformedJSONStringField(native.Function.Arguments, "draft_id")
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
		normalizeNativeActionAlias(name, tc.Action, &tc)
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

var focusedHomepageToolOperations = map[string]map[string]bool{
	"homepage_project": {
		"init": true, "start": true, "stop": true, "status": true, "rebuild": true, "destroy": true,
		"exec": true, "init_project": true, "build": true, "install_deps": true, "dev": true,
		"webserver_start": true, "webserver_stop": true, "webserver_status": true, "publish_local": true,
		"tunnel": true, "test_connection": true,
	},
	"homepage_file": {
		"list_files": true, "read_file": true, "write_file": true, "edit_file": true,
		"json_edit": true, "yaml_edit": true, "xml_edit": true, "optimize_images": true,
	},
	"homepage_quality": {
		"lighthouse": true, "screenshot": true, "check_js": true, "lint": true, "optimize_images": true,
	},
	"homepage_deploy": {
		"build": true, "dev": true, "publish_local": true, "webserver_start": true, "webserver_stop": true,
		"webserver_status": true, "test_connection": true, "tunnel": true, "deploy": true,
		"deploy_netlify": true, "deploy_vercel": true,
	},
	"homepage_git": {
		"git_init": true, "git_commit": true, "git_status": true, "git_diff": true, "git_log": true,
		"git_rollback": true, "save_revision": true, "list_revisions": true, "get_revision": true,
		"diff_revision": true, "restore_revision": true, "revision_status": true,
	},
}

var homepageFileEditSubOperations = map[string]bool{
	"str_replace":   true,
	"insert_after":  true,
	"insert_before": true,
	"append":        true,
	"prepend":       true,
	"delete_lines":  true,
	"get":           true,
	"set":           true,
	"delete":        true,
	"remove":        true,
}

func normalizeNativeActionAlias(toolName, rawAction string, tc *ToolCall) {
	rawAction = strings.TrimSpace(rawAction)
	lowerAction := strings.ToLower(rawAction)
	if operations, ok := focusedHomepageToolOperations[toolName]; ok {
		if tc.Operation == "" && operations[lowerAction] {
			tc.Operation = lowerAction
			return
		}
		if toolName == "homepage_file" && homepageFileEditOperation(tc.Operation) && homepageFileEditSubOperations[lowerAction] {
			tc.SubOperation = lowerAction
		}
		return
	}
	tc.SubOperation = rawAction
}

func homepageFileEditOperation(operation string) bool {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "edit_file", "json_edit", "yaml_edit", "xml_edit":
		return true
	default:
		return false
	}
}

func normalizeCustomToolShortcutArgs(customName string, raw map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"name": customName}
	if len(raw) == 0 {
		return out
	}
	for _, key := range []string{"background", "vault_keys", "credential_ids", "_todo"} {
		if value, ok := raw[key]; ok {
			out[key] = value
		}
	}
	if args, ok := raw["args"]; ok {
		out["args"] = args
		return out
	}
	if params, ok := raw["params"]; ok {
		out["args"] = params
		return out
	}
	params := make(map[string]interface{})
	for key, value := range raw {
		switch key {
		case "action", "name", "tool", "tool_name", "args", "params", "background", "vault_keys", "credential_ids", "_todo":
			continue
		default:
			params[key] = value
		}
	}
	if len(params) > 0 {
		out["args"] = params
	}
	return out
}

func decodeNativeJSONStringObjectArgs(m map[string]interface{}) {
	for key, raw := range m {
		switch value := raw.(type) {
		case string:
			if !isNativeJSONStringObjectArg(key) {
				continue
			}
			if decoded := decodeJSONStringObject(value); decoded != nil {
				m[key] = decoded
			}
		case map[string]interface{}:
			decodeNativeJSONStringObjectArgs(value)
		case []interface{}:
			for _, item := range value {
				if child, ok := item.(map[string]interface{}); ok {
					decodeNativeJSONStringObjectArgs(child)
				}
			}
		}
	}
}

func isNativeJSONStringObjectArg(name string) bool {
	_, ok := nativeJSONStringObjectArgNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func decodeJSONStringObject(raw string) map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil || len(obj) == 0 {
		return nil
	}
	return obj
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

func buildNativeToolSchemasUncached(skillsDir string, manifest *tools.Manifest, ff ToolFeatureFlags, logger *slog.Logger) []openai.Tool {
	allTools := cloneToolSchemasForSnapshot(builtinToolSchemasCached(ff))
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
			if !isProviderSafeNativeToolName(schemaName) {
				if logger != nil {
					logger.Warn("[NativeTools] Skipping skill shortcut with provider-invalid function name",
						"skill", skill.Name,
						"schema_name", schemaName,
					)
				}
				continue
			}
			if _, exists := emittedNames[schemaName]; exists {
				continue
			}
			emittedNames[schemaName] = struct{}{}
			allTools = append(allTools, tool(
				schemaName,
				"(Skill) "+skill.Description+". Use execute_skill with skill='"+skill.Name+"'.",
				schema(map[string]interface{}{
					"skill_args": skillArgsSchemaFromManifest(skill.Parameters),
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
				if !isProviderSafeNativeToolName(schemaName) {
					if logger != nil {
						logger.Warn("[NativeTools] Skipping custom tool shortcut with provider-invalid function name",
							"tool", name,
							"schema_name", schemaName,
						)
					}
					continue
				}
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
	allTools = filterProviderSafeNativeToolSchemas(allTools, logger)

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
		}
		// CHANGE LOG 2026-04-11: OpenAI strict mode requires additionalProperties: false
		// on every object schema. The go-openai library does not auto-add this, so we
		// inject it here. Only affects strict-mode requests; non-strict calls ignore it.
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
	}

	if logger != nil {
		logger.Info("[NativeTools] Built tool schemas", "count", len(allTools))
	}

	return allTools
}

// BuildNativeToolSchemas returns the full tool list: built-ins + registered skills + custom tools.
func BuildNativeToolSchemas(skillsDir string, manifest *tools.Manifest, ff ToolFeatureFlags, logger *slog.Logger) []openai.Tool {
	return BuildNativeToolSchemaSnapshot(skillsDir, manifest, ff, logger).FullSchemas()
}

func nativeSkillsFingerprint(skillsDir string) string {
	skillsDir = strings.TrimSpace(skillsDir)
	if skillsDir == "" {
		return ""
	}
	parts := make([]string, 0)
	_ = filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(skillsDir, path)
		if err != nil {
			rel = path
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func nativeManifestFingerprint(manifest *tools.Manifest) string {
	if manifest == nil {
		return ""
	}
	entries, err := manifest.Load()
	if err != nil || len(entries) == 0 {
		return ""
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+entries[name])
	}
	return strings.Join(parts, "\n")
}

func isProviderSafeNativeToolName(name string) bool {
	return providerNativeToolNamePattern.MatchString(strings.TrimSpace(name))
}

func filterProviderSafeNativeToolSchemas(schemas []openai.Tool, logger *slog.Logger) []openai.Tool {
	if len(schemas) == 0 {
		return schemas
	}
	filtered := schemas[:0]
	for _, schema := range schemas {
		if schema.Function == nil || isProviderSafeNativeToolName(schema.Function.Name) {
			filtered = append(filtered, schema)
			continue
		}
		if logger != nil {
			logger.Warn("[NativeTools] Dropping provider-invalid function name",
				"name", schema.Function.Name,
			)
		}
	}
	return filtered
}

func nativeToolSortName(schema openai.Tool) string {
	if schema.Function == nil {
		return ""
	}
	return schema.Function.Name
}

func skillArgsSchemaFromManifest(params map[string]interface{}) map[string]interface{} {
	if len(params) == 0 {
		return map[string]interface{}{
			"type":        "object",
			"description": "Arguments for this skill",
		}
	}
	if schemaType, _ := params["type"].(string); schemaType == "object" {
		out := make(map[string]interface{}, len(params)+1)
		for k, v := range params {
			out[k] = v
		}
		if _, ok := out["description"]; !ok {
			out["description"] = "Arguments for this skill"
		}
		return out
	}
	props := make(map[string]interface{}, len(params))
	for name, raw := range params {
		if propSchema, ok := raw.(map[string]interface{}); ok {
			props[name] = propSchema
			continue
		}
		props[name] = map[string]interface{}{
			"type":        "string",
			"description": fmt.Sprint(raw),
		}
	}
	return map[string]interface{}{
		"type":        "object",
		"description": "Arguments for this skill",
		"properties":  props,
	}
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
