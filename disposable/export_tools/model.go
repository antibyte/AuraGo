package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"aurago/internal/agent"

	openai "github.com/sashabaranov/go-openai"
	"github.com/xeipuuv/gojsonschema"
)

const (
	datasetSchemaVersion = "2.0"
	tierManifestVersion  = 1
	contractVersion      = 2
	targetScenarioCount  = 5000
	maxToolsPerScenario  = 20
	maxSchemaTokens      = 6500
)

var coreToolNames = map[string]bool{
	"discover_tools": true,
	"invoke_tool":    true,
	"execute_skill":  true,
	"run_tool":       true,
	"filesystem":     true,
	"query_memory":   true,
	"manage_memory":  true,
	"execute_shell":  true,
}

var compiledToolSchemas sync.Map

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Strict      bool                   `json:"strict"`
}

type ToolExport struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Parameters    map[string]interface{} `json:"parameters"`
	Required      []string               `json:"required,omitempty"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
	Tier          string                 `json:"tier"`
	Operations    []OperationRef         `json:"operations,omitempty"`
	ManualPath    string                 `json:"manual_path,omitempty"`
	ManualSnippet string                 `json:"manual_snippet,omitempty"`
}

func (t ToolExport) Definition() ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: FunctionDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  cloneMap(t.Parameters),
			Strict:      true,
		},
	}
}

type TierManifest struct {
	Version  int      `json:"version"`
	Core     []string `json:"core"`
	Extended []string `json:"extended"`
	Rare     []string `json:"rare"`
}

type OperationRef struct {
	Selector string      `json:"selector"`
	Value    interface{} `json:"value"`
}

type OperationFixture struct {
	Selector       string                 `json:"selector"`
	Value          interface{}            `json:"value"`
	Arguments      map[string]interface{} `json:"arguments"`
	RequiredFields []string               `json:"required_fields"`
	ExcludedFields []string               `json:"excluded_fields"`
}

type ToolContract struct {
	DefaultArguments map[string]interface{} `json:"default_arguments"`
	Operations       []OperationFixture     `json:"operations,omitempty"`
}

type OperationContractManifest struct {
	Version      int                     `json:"version"`
	SchemaSHA256 string                  `json:"schema_sha256"`
	Tools        map[string]ToolContract `json:"tools"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ExpectedCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type Expectations struct {
	ShouldCall bool           `json:"should_call"`
	Calls      []ExpectedCall `json:"calls,omitempty"`
	Outcome    string         `json:"outcome"`
	TargetTool string         `json:"target_tool,omitempty"`
	Selector   string         `json:"selector,omitempty"`
	Value      interface{}    `json:"value,omitempty"`
}

type Scenario struct {
	ID            string           `json:"id"`
	SchemaVersion string           `json:"schema_version"`
	Family        string           `json:"family"`
	Language      string           `json:"language"`
	Source        string           `json:"source"`
	Tier          string           `json:"tier"`
	Kind          string           `json:"kind"`
	Split         string           `json:"split"`
	Tools         []ToolDefinition `json:"tools"`
	Messages      []Message        `json:"messages"`
	Expectations  Expectations     `json:"expectations"`
}

type TaggedScenario struct {
	ID            string           `json:"id"`
	SchemaVersion string           `json:"schema_version"`
	Family        string           `json:"family"`
	Language      string           `json:"language"`
	Source        string           `json:"source"`
	Tier          string           `json:"tier"`
	Kind          string           `json:"kind"`
	Split         string           `json:"split"`
	Tools         []ToolDefinition `json:"tools"`
	Messages      []Message        `json:"messages"`
	Expectations  Expectations     `json:"expectations"`
}

type BuildResult struct {
	Tools          []ToolExport
	Tiers          TierManifest
	Contracts      OperationContractManifest
	Scenarios      []Scenario
	Tagged         []TaggedScenario
	Challenge      []Scenario
	OperationCount int
}

func allFlags() agent.ToolFeatureFlags {
	ff := agent.ToolFeatureFlags{}
	v := reflect.ValueOf(&ff).Elem()
	for i := 0; i < v.NumField(); i++ {
		if field := v.Field(i); field.CanSet() && field.Kind() == reflect.Bool {
			field.SetBool(true)
		}
	}
	return ff
}

func loadStrictTools(root string) ([]ToolExport, error) {
	tmpSkills, err := os.MkdirTemp("", "aurago-export-skills-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary skills directory: %w", err)
	}
	defer os.RemoveAll(tmpSkills)

	snapshot := agent.BuildNativeToolSchemaSnapshot(tmpSkills, nil, allFlags(), nil)
	schemas := snapshot.StrictSchemas()
	byName := make(map[string]openai.Tool, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil || strings.TrimSpace(schema.Function.Name) == "" {
			continue
		}
		if _, exists := byName[schema.Function.Name]; !exists {
			byName[schema.Function.Name] = schema
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]ToolExport, 0, len(names))
	for _, name := range names {
		schema := byName[name]
		params := normalizeParams(schema.Function.Parameters)
		if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(params)); err != nil {
			return nil, fmt.Errorf("tool %s has invalid native JSON Schema: %w", name, err)
		}
		props, required := propsAndRequired(params)
		manualPath, snippet := manualMetadata(root, name)
		tools = append(tools, ToolExport{
			Name:          name,
			Description:   schema.Function.Description,
			Parameters:    params,
			Required:      required,
			Properties:    props,
			ManualPath:    manualPath,
			ManualSnippet: snippet,
		})
	}
	return tools, nil
}

func normalizeParams(params interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "additionalProperties": false}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "additionalProperties": false}
	}
	var normalized map[string]interface{}
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "additionalProperties": false}
	}
	if _, ok := normalized["additionalProperties"]; !ok {
		normalized["additionalProperties"] = false
	}
	return normalized
}

func propsAndRequired(params map[string]interface{}) (map[string]interface{}, []string) {
	props, _ := params["properties"].(map[string]interface{})
	required := stringSlice(params["required"])
	sort.Strings(required)
	return props, required
}

func stringSlice(raw interface{}) []string {
	switch values := raw.(type) {
	case []string:
		return append([]string(nil), values...)
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func manualMetadata(root, toolName string) (string, string) {
	path := filepath.Join(root, "prompts", "tools_manuals", toolName+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	body := string(data)
	if strings.HasPrefix(body, "---") {
		if end := strings.Index(body[3:], "---"); end >= 0 {
			body = strings.TrimSpace(body[3+end+3:])
		}
	}
	if len(body) > 800 {
		body = body[:800] + "…"
	}
	relative, _ := filepath.Rel(root, path)
	return filepath.ToSlash(relative), body
}

func buildDefaultTiers(tools []ToolExport) TierManifest {
	tiers := TierManifest{Version: tierManifestVersion}
	for _, tool := range tools {
		switch {
		case coreToolNames[tool.Name]:
			tiers.Core = append(tiers.Core, tool.Name)
		case tool.ManualPath != "" || len(extractOperations(tool.Parameters)) > 0:
			tiers.Extended = append(tiers.Extended, tool.Name)
		default:
			tiers.Rare = append(tiers.Rare, tool.Name)
		}
	}
	sort.Strings(tiers.Core)
	sort.Strings(tiers.Extended)
	sort.Strings(tiers.Rare)
	return tiers
}

func applyTiers(tools []ToolExport, tiers TierManifest) error {
	if tiers.Version != tierManifestVersion {
		return fmt.Errorf("unsupported tool tier manifest version %d", tiers.Version)
	}
	assignments := make(map[string]string, len(tools))
	add := func(tier string, names []string) error {
		for _, name := range names {
			if previous := assignments[name]; previous != "" {
				return fmt.Errorf("tool %s appears in both %s and %s tiers", name, previous, tier)
			}
			assignments[name] = tier
		}
		return nil
	}
	if err := add("core", tiers.Core); err != nil {
		return err
	}
	if err := add("extended", tiers.Extended); err != nil {
		return err
	}
	if err := add("rare", tiers.Rare); err != nil {
		return err
	}
	known := make(map[string]bool, len(tools))
	for i := range tools {
		known[tools[i].Name] = true
		tier := assignments[tools[i].Name]
		if tier == "" {
			return fmt.Errorf("tool %s is missing from tool_tiers.json", tools[i].Name)
		}
		tools[i].Tier = tier
		tools[i].Operations = extractOperations(tools[i].Parameters)
	}
	for name := range assignments {
		if !known[name] {
			return fmt.Errorf("tool_tiers.json references unknown tool %s", name)
		}
	}
	return nil
}

func extractOperations(schema map[string]interface{}) []OperationRef {
	props, _ := schema["properties"].(map[string]interface{})
	var operations []OperationRef
	for _, selector := range []string{"operation", "action", "op"} {
		property, _ := props[selector].(map[string]interface{})
		for _, value := range enumValues(property["enum"]) {
			operations = append(operations, OperationRef{Selector: selector, Value: value})
		}
	}
	sort.SliceStable(operations, func(i, j int) bool {
		if operations[i].Selector != operations[j].Selector {
			return operations[i].Selector < operations[j].Selector
		}
		return fmt.Sprint(operations[i].Value) < fmt.Sprint(operations[j].Value)
	})
	return operations
}

func enumValues(raw interface{}) []interface{} {
	switch values := raw.(type) {
	case []interface{}:
		return append([]interface{}(nil), values...)
	case []string:
		out := make([]interface{}, len(values))
		for i, value := range values {
			out[i] = value
		}
		return out
	default:
		return nil
	}
}

func schemaDigest(tools []ToolExport) (string, error) {
	payload := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		payload = append(payload, tool.Definition())
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func validateArguments(tool ToolExport, arguments map[string]interface{}) error {
	raw, _ := json.Marshal(tool.Parameters)
	sum := sha256.Sum256(raw)
	cacheKey := tool.Name + ":" + hex.EncodeToString(sum[:])
	var compiled *gojsonschema.Schema
	if cached, ok := compiledToolSchemas.Load(cacheKey); ok {
		compiled = cached.(*gojsonschema.Schema)
	} else {
		var err error
		compiled, err = gojsonschema.NewSchema(gojsonschema.NewGoLoader(tool.Parameters))
		if err != nil {
			return fmt.Errorf("compile JSON Schema: %w", err)
		}
		compiledToolSchemas.Store(cacheKey, compiled)
	}
	result, err := compiled.Validate(gojsonschema.NewGoLoader(arguments))
	if err != nil {
		return fmt.Errorf("run JSON Schema validation: %w", err)
	}
	if result.Valid() {
		return nil
	}
	parts := make([]string, 0, len(result.Errors()))
	for _, item := range result.Errors() {
		parts = append(parts, item.String())
		if len(parts) == 5 {
			break
		}
	}
	return fmt.Errorf("%s", strings.Join(parts, "; "))
}

func cloneMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	raw, _ := json.Marshal(input)
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func sanitizeID(raw string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
