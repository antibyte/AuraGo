package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func testToolSchema(name, desc string) openai.Tool {
	return tool(name, desc, schema(map[string]interface{}{
		"query": prop("string", "Query"),
	}))
}

func TestToolCatalogClassifiesNativeSkillAndCustomTools(t *testing.T) {
	schemas := []openai.Tool{
		testToolSchema("yepapi_instagram", "Instagram data via YepAPI"),
		testToolSchema("skill__weather_lookup", "(Skill) Weather lookup"),
		testToolSchema("tool__my_tool.py", "(Custom tool) My tool"),
	}
	active := []openai.Tool{schemas[0]}

	catalog := BuildToolCatalog(schemas, active, "")

	nativeEntry, ok := catalog.Get("yepapi_instagram")
	if !ok {
		t.Fatal("missing yepapi_instagram catalog entry")
	}
	if nativeEntry.Kind != ToolKindNative {
		t.Fatalf("yepapi_instagram kind = %q, want native", nativeEntry.Kind)
	}
	if nativeEntry.Status != ToolStatusActive {
		t.Fatalf("yepapi_instagram status = %q, want active", nativeEntry.Status)
	}

	skillEntry, ok := catalog.Get("weather_lookup")
	if !ok {
		t.Fatal("missing weather_lookup catalog entry")
	}
	if skillEntry.Kind != ToolKindSkill || skillEntry.Routing.SkillName != "weather_lookup" {
		t.Fatalf("skill entry = %+v, want skill routing", skillEntry)
	}
	if _, ok := catalog.Get("skill__weather_lookup"); !ok {
		t.Fatal("expected skill__ alias to resolve")
	}

	customEntry, ok := catalog.Get("my_tool.py")
	if !ok {
		t.Fatal("missing custom tool catalog entry")
	}
	if customEntry.Kind != ToolKindCustom || customEntry.Routing.CustomName != "my_tool.py" {
		t.Fatalf("custom entry = %+v, want custom routing", customEntry)
	}
	if _, ok := catalog.Get("tool__my_tool.py"); !ok {
		t.Fatal("expected tool__ alias to resolve")
	}
}

func TestToolCatalogMarksHiddenAndDisabledTools(t *testing.T) {
	schemas := []openai.Tool{
		testToolSchema("yepapi_instagram", "Instagram data via YepAPI"),
	}

	catalog := BuildToolCatalog(schemas, nil, "")

	hidden, ok := catalog.Get("yepapi_instagram")
	if !ok {
		t.Fatal("missing yepapi_instagram")
	}
	if hidden.Status != ToolStatusHidden || hidden.HiddenReason != "adaptive_filter" {
		t.Fatalf("hidden status = %+v, want hidden adaptive_filter", hidden)
	}

	disabled, ok := catalog.Get("yepapi_youtube")
	if !ok {
		t.Fatal("expected disabled categorized yepapi_youtube entry")
	}
	if disabled.Status != ToolStatusDisabled || disabled.Enabled {
		t.Fatalf("disabled entry = %+v, want disabled", disabled)
	}
}

func TestDiscoverToolsReturnsStructuredHiddenNativeResult(t *testing.T) {
	resetToolCatalogForTest(t)
	schemas := []openai.Tool{
		testToolSchema("yepapi_instagram", "Instagram data via YepAPI"),
	}
	SetDiscoverToolsState("sess-catalog", schemas, nil, "")

	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "search",
			"query":     "yepapi_instagram",
		},
	}, &config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "sess-catalog")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Status != "success" {
		t.Fatalf("status = %q, want success: %s", payload.Status, out)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results = %d, want 1: %s", len(payload.Results), out)
	}
	got := payload.Results[0]
	if got.Name != "yepapi_instagram" || got.Kind != string(ToolKindNative) || got.ToolStatus != string(ToolStatusHidden) {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.CallMethod != "invoke_tool" || !got.CallableNow {
		t.Fatalf("call fields = %+v, want invoke_tool callable", got)
	}
	requested := GetDiscoverRequestedTools("sess-catalog")
	if len(requested) != 1 || requested[0] != "yepapi_instagram" {
		t.Fatalf("requested = %v, want [yepapi_instagram]", requested)
	}
}

func TestDiscoverToolsReturnsSkillCallMethod(t *testing.T) {
	resetToolCatalogForTest(t)
	schemas := []openai.Tool{
		testToolSchema("skill__weather_lookup", "(Skill) Weather lookup"),
	}
	SetDiscoverToolsState("sess-skill", schemas, nil, "")

	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "search",
			"query":     "weather_lookup",
		},
	}, &config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "sess-skill")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if len(payload.Results) != 1 {
		t.Fatalf("results = %d, want 1: %s", len(payload.Results), out)
	}
	got := payload.Results[0]
	if got.Kind != string(ToolKindSkill) || got.CallMethod != "execute_skill" {
		t.Fatalf("skill result = %+v, want execute_skill method", got)
	}
}

func TestInvokeToolRoutesHiddenNativeTool(t *testing.T) {
	resetToolCatalogForTest(t)
	cfg := &config.Config{}
	cfg.YepAPI.Enabled = true
	cfg.YepAPI.Instagram.Enabled = false
	schemas := []openai.Tool{testToolSchema("yepapi_instagram", "Instagram data via YepAPI")}
	SetDiscoverToolsState("sess-invoke", schemas, nil, "")

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action: "invoke_tool",
		Params: map[string]interface{}{
			"tool_name": "yepapi_instagram",
			"arguments": map[string]interface{}{
				"operation": "user",
				"username":  "jopliness",
			},
		},
	}, &DispatchContext{
		Cfg:       cfg,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionID: "sess-invoke",
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle invoke_tool")
	}
	if !strings.Contains(out, "YepAPI Instagram is disabled") {
		t.Fatalf("expected native handler disabled response, got %s", out)
	}
	requested := GetDiscoverRequestedTools("sess-invoke")
	if len(requested) != 1 || requested[0] != "yepapi_instagram" {
		t.Fatalf("requested = %v, want [yepapi_instagram]", requested)
	}
}

func TestInvokeToolAcceptsFlattenedArguments(t *testing.T) {
	args := flattenedInvokeArgs(map[string]interface{}{
		"tool_name":       "yepapi_instagram",
		"operation":       "user",
		"username_or_url": "jopliness",
	})
	routed := toolCallFromInvokeArgs("yepapi_instagram", args)
	if routed.Action != "yepapi_instagram" || routed.Operation != "user" {
		t.Fatalf("routed = %+v, want native action with operation", routed)
	}
	if routed.Params["username_or_url"] != "jopliness" {
		t.Fatalf("params = %+v, want username_or_url preserved", routed.Params)
	}
	if _, ok := routed.Params["tool_name"]; ok {
		t.Fatalf("params = %+v, tool_name should not leak into invoked tool args", routed.Params)
	}
}

func TestInvokeToolLogsFlattenedArgumentsAndMissingOperation(t *testing.T) {
	resetToolCatalogForTest(t)
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := &config.Config{}
	cfg.YepAPI.Enabled = true
	cfg.YepAPI.Instagram.Enabled = false
	schemas := []openai.Tool{testToolSchema("yepapi_instagram", "Instagram data via YepAPI")}
	SetDiscoverToolsState("sess-invoke-log", schemas, nil, "")

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action: "invoke_tool",
		Params: map[string]interface{}{
			"tool_name":       "yepapi_instagram",
			"username_or_url": "jopliness",
		},
	}, &DispatchContext{
		Cfg:       cfg,
		Logger:    logger,
		SessionID: "sess-invoke-log",
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle invoke_tool")
	}
	if !strings.Contains(out, "YepAPI Instagram is disabled") {
		t.Fatalf("expected disabled response, got %s", out)
	}
	got := logs.String()
	if !strings.Contains(got, "invoke_tool received flattened arguments") {
		t.Fatalf("logs = %q, want flattened argument diagnostic", got)
	}
	if !strings.Contains(got, "missing_operation") {
		t.Fatalf("logs = %q, want missing operation diagnostic", got)
	}
	if strings.Contains(got, "jopliness") {
		t.Fatalf("logs = %q, should not include raw argument values", got)
	}
}

func TestToolCommandErrorMessageDetectsErrorEnvelope(t *testing.T) {
	msg, ok := toolCommandErrorMessage(`{"status":"error","message":"username_or_url is required"}`)
	if !ok {
		t.Fatal("expected error envelope to be detected")
	}
	if msg != "username_or_url is required" {
		t.Fatalf("message = %q, want username_or_url is required", msg)
	}
}

func TestInvokeToolRejectsDisabledTool(t *testing.T) {
	resetToolCatalogForTest(t)
	SetDiscoverToolsState("sess-disabled", nil, nil, "")

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action: "invoke_tool",
		Params: map[string]interface{}{
			"tool_name": "yepapi_youtube",
			"arguments": map[string]interface{}{
				"operation": "search",
				"query":     "golang",
			},
		},
	}, &DispatchContext{
		Cfg:    &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle invoke_tool")
	}
	if !strings.Contains(out, "disabled") {
		t.Fatalf("expected disabled response, got %s", out)
	}
}

func TestBuildNativeToolSchemasUsesSkillManifestParameters(t *testing.T) {
	skillsDir := t.TempDir()
	manifest := `{
  "name": "weather_lookup",
  "description": "Weather lookup",
  "executable": "weather.py",
  "parameters": {
    "type": "object",
    "properties": {
      "city": {"type": "string", "description": "City name"}
    },
    "required": ["city"]
  }
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "weather_lookup.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write skill manifest: %v", err)
	}

	schemas := BuildNativeToolSchemas(skillsDir, nil, ToolFeatureFlags{}, nil)
	var skillSchema *openai.FunctionDefinition
	for _, s := range schemas {
		if s.Function != nil && s.Function.Name == "skill__weather_lookup" {
			skillSchema = s.Function
			break
		}
	}
	if skillSchema == nil {
		t.Fatal("missing skill__weather_lookup schema")
	}
	params, _ := skillSchema.Parameters.(map[string]interface{})
	props, _ := params["properties"].(map[string]interface{})
	skillArgs, _ := props["skill_args"].(map[string]interface{})
	argProps, _ := skillArgs["properties"].(map[string]interface{})
	if _, ok := argProps["city"]; !ok {
		t.Fatalf("expected skill_args.city schema, got %#v", skillArgs)
	}
}

func resetToolCatalogForTest(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.allSchemas = nil
		discoverToolsState.activeNames = nil
		discoverToolsState.enabledNames = nil
		discoverToolsState.requested = nil
		discoverToolsState.promptsDir = ""
		discoverToolsState.catalog = nil
		discoverToolsState.mu.Unlock()
	})
}

func decodeToolOutputJSON(t *testing.T, out string, dest interface{}) {
	t.Helper()
	raw := strings.TrimPrefix(out, "Tool Output:")
	raw = strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
}
