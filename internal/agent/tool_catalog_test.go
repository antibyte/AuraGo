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

func TestBuildNativeToolSchemasDocumentsVirtualDesktopPapirusIconCatalog(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{VirtualDesktopEnabled: true}, nil)
	var virtualDesktop *openai.FunctionDefinition
	for _, item := range schemas {
		if item.Function != nil && item.Function.Name == "virtual_desktop" {
			virtualDesktop = item.Function
			break
		}
	}
	if virtualDesktop == nil {
		t.Fatal("missing virtual_desktop schema")
	}
	for _, want := range []string{"icon_catalog", "semantic icons", "Emoji", "sprite:<name>"} {
		if !strings.Contains(virtualDesktop.Description, want) {
			t.Fatalf("virtual_desktop description missing %q: %s", want, virtualDesktop.Description)
		}
	}
	params, _ := virtualDesktop.Parameters.(map[string]interface{})
	props, _ := params["properties"].(map[string]interface{})
	operation, _ := props["operation"].(map[string]interface{})
	if !containsInterfaceString(operation["enum"], "open_in_app") {
		t.Fatalf("virtual_desktop operation enum missing open_in_app: %#v", operation["enum"])
	}
	appID, _ := props["app_id"].(map[string]interface{})
	appIDDescription, _ := appID["description"].(string)
	if !strings.Contains(appIDDescription, "open_in_app") {
		t.Fatalf("app_id description missing open_in_app guidance: %s", appIDDescription)
	}
	manifest, _ := props["manifest"].(map[string]interface{})
	manifestDescription, _ := manifest["description"].(string)
	for _, want := range []string{"icon_catalog.preferred", "icon_catalog.aliases", "icon is optional", "runtime defaults to aura-desktop-sdk@1"} {
		if !strings.Contains(manifestDescription, want) {
			t.Fatalf("manifest description missing %q: %s", want, manifestDescription)
		}
	}
	widget, _ := props["widget"].(map[string]interface{})
	widgetDescription, _ := widget["description"].(string)
	if !strings.Contains(widgetDescription, "icon_catalog") {
		t.Fatalf("widget description missing icon_catalog guidance: %s", widgetDescription)
	}
	if !strings.Contains(widgetDescription, "inferred") {
		t.Fatalf("widget description missing inferred icon guidance: %s", widgetDescription)
	}
}

func TestVirtualDesktopManualDocumentsGeneratedAppAndWidgetAPIs(t *testing.T) {
	t.Parallel()

	manualPath := filepath.Join("..", "..", "prompts", "tools_manuals", "virtual_desktop.md")
	data, err := os.ReadFile(manualPath)
	if err != nil {
		t.Fatalf("read virtual desktop manual: %v", err)
	}
	manual := string(data)
	for _, want := range []string{
		"`open_app` / `open_in_app`",
		"`AuraDesktop.menu.set(menus)`",
		"`AuraDesktop.menu.clear()`",
		"`AuraDesktop.menu.onAction(handler)`",
		"`select`",
		"`toast`",
		"`analytics`",
		"`backup`",
		"`camera`",
		"`workflow`",
		"Fruity",
		"WhiteSur",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("virtual desktop manual missing %q", want)
		}
	}
}

func TestBuildNativeToolSchemasIncludesOfficeTools(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{
		OfficeDocumentEnabled: true,
		OfficeWorkbookEnabled: true,
	}, nil)
	names := toolNames(schemas)
	for _, want := range []string{"office_document", "office_workbook"} {
		if !containsName(names, want) {
			t.Fatalf("missing %s in schemas: %v", want, names)
		}
	}

	var documentSchema, workbookSchema *openai.FunctionDefinition
	for _, item := range schemas {
		if item.Function == nil {
			continue
		}
		switch item.Function.Name {
		case "office_document":
			documentSchema = item.Function
		case "office_workbook":
			workbookSchema = item.Function
		}
	}
	if documentSchema == nil || workbookSchema == nil {
		t.Fatalf("missing office schemas document=%v workbook=%v", documentSchema != nil, workbookSchema != nil)
	}
	documentParams, _ := documentSchema.Parameters.(map[string]interface{})
	assertRequiredContains(t, documentParams, "operation")
	assertRequiredContains(t, documentParams, "path")
	documentProps, _ := documentParams["properties"].(map[string]interface{})
	documentOperation, _ := documentProps["operation"].(map[string]interface{})
	if !containsInterfaceString(documentOperation["enum"], "patch") {
		t.Fatalf("office_document operation enum missing patch: %#v", documentOperation["enum"])
	}
	workbookParams, _ := workbookSchema.Parameters.(map[string]interface{})
	assertRequiredContains(t, workbookParams, "operation")
	assertRequiredContains(t, workbookParams, "path")
	workbookProps, _ := workbookParams["properties"].(map[string]interface{})
	workbookOperation, _ := workbookProps["operation"].(map[string]interface{})
	for _, want := range []string{"set_range", "evaluate_formula"} {
		if !containsInterfaceString(workbookOperation["enum"], want) {
			t.Fatalf("office_workbook operation enum missing %s: %#v", want, workbookOperation["enum"])
		}
	}
}

func TestDiscoverToolsReturnsOfficeToolManuals(t *testing.T) {
	resetToolCatalogForTest(t)
	promptsDir := filepath.Clean(filepath.Join("..", "..", "prompts"))
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{
		OfficeDocumentEnabled: true,
		OfficeWorkbookEnabled: true,
	}, nil)
	SetDiscoverToolsState("sess-office-manuals", schemas, nil, promptsDir)
	cfg := &config.Config{}
	cfg.Directories.PromptsDir = promptsDir
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tc := range []struct {
		toolName string
		want     []string
	}{
		{
			toolName: "office_document",
			want:     []string{"# office_document", "tools.office_document.readonly", "patch"},
		},
		{
			toolName: "office_workbook",
			want:     []string{"# office_workbook", "tools.office_workbook.readonly", "evaluate_formula"},
		},
	} {
		out := handleDiscoverTools(ToolCall{
			Params: map[string]interface{}{
				"operation": "get_tool_info",
				"tool_name": tc.toolName,
			},
		}, cfg, logger, "sess-office-manuals")
		var payload DiscoverToolsResponse
		decodeToolOutputJSON(t, out, &payload)
		if payload.Status != "success" || payload.Tool == nil || payload.Tool.Name != tc.toolName {
			t.Fatalf("discover_tools get_tool_info for %s = %+v output=%s", tc.toolName, payload, out)
		}
		for _, want := range tc.want {
			if !strings.Contains(payload.Manual, want) {
				t.Fatalf("discover_tools manual for %s missing %q: %s", tc.toolName, want, payload.Manual)
			}
		}

		manualOut, ok := dispatchComm(context.Background(), ToolCall{
			Action: "get_tool_manual",
			Params: map[string]interface{}{
				"tool_name": tc.toolName,
			},
		}, &DispatchContext{
			Cfg:       cfg,
			Logger:    logger,
			SessionID: "sess-office-manuals",
		})
		if !ok {
			t.Fatalf("dispatchComm did not handle get_tool_manual for %s", tc.toolName)
		}
		for _, want := range tc.want {
			if !strings.Contains(manualOut, want) {
				t.Fatalf("get_tool_manual for %s missing %q: %s", tc.toolName, want, manualOut)
			}
		}
	}
}

func TestVirtualDesktopSchemaDocumentsPathRequirementsWithoutGlobalPathRequirement(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{VirtualDesktopEnabled: true}, nil)
	var virtualDesktop *openai.FunctionDefinition
	for _, item := range schemas {
		if item.Function != nil && item.Function.Name == "virtual_desktop" {
			virtualDesktop = item.Function
			break
		}
	}
	if virtualDesktop == nil {
		t.Fatal("missing virtual_desktop schema")
	}
	params, _ := virtualDesktop.Parameters.(map[string]interface{})
	if containsInterfaceString(params["required"], "path") {
		t.Fatalf("virtual_desktop must not require path globally: %#v", params["required"])
	}
	props, _ := params["properties"].(map[string]interface{})
	pathProp, _ := props["path"].(map[string]interface{})
	description, _ := pathProp["description"].(string)
	for _, want := range []string{"Required for file operations", "Office operations", "export_file"} {
		if !strings.Contains(description, want) {
			t.Fatalf("virtual_desktop path description missing %q: %s", want, description)
		}
	}
}

func assertRequiredContains(t *testing.T, params map[string]interface{}, want string) {
	t.Helper()
	if containsInterfaceString(params["required"], want) {
		return
	}
	t.Fatalf("required fields %#v missing %q", params["required"], want)
}

func containsInterfaceString(raw interface{}, want string) bool {
	items, ok := raw.([]string)
	if ok {
		for _, item := range items {
			if item == want {
				return true
			}
		}
		return false
	}
	values, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range values {
		if item == want {
			return true
		}
	}
	return false
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
