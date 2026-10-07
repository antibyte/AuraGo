package agent

import (
	"aurago/internal/llm"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
	"github.com/xeipuuv/gojsonschema"
)

func TestStrictSchemaPreservesConstraintsAndOriginal(t *testing.T) {
	original := schema(map[string]interface{}{
		"mode":          map[string]interface{}{"type": "string", "enum": []string{"read", "write"}},
		"nullable_enum": map[string]interface{}{"type": []interface{}{"string", "null"}, "enum": []interface{}{"kept", nil}},
		"rows": map[string]interface{}{"type": "array", "minItems": 1, "items": schema(map[string]interface{}{
			"id": prop("integer", "ID"), "label": map[string]interface{}{"type": "string", "minLength": 2},
		}, "id")},
		"constant": map[string]interface{}{"type": "integer", "const": 7},
		"choice":   map[string]interface{}{"anyOf": []interface{}{schema(map[string]interface{}{"id": prop("integer", "ID")}, "id"), prop("string", "Text")}},
	}, "mode")
	before, _ := json.Marshal(original)
	snapshot := newNativeToolSchemaSnapshot([]openai.Tool{tool("fixture", "fixture", original)})
	strict := snapshot.StrictSchemas()[0].Function.Parameters.(map[string]interface{})
	full, _ := json.Marshal(snapshot.FullSchemas()[0].Function.Parameters)
	after, _ := json.Marshal(original)
	if string(before) != string(after) || string(before) != string(full) {
		t.Fatal("Strict normalization mutated original/full schema")
	}
	first, _ := json.Marshal(strict)
	normalizeStrictSchemaRequiredRec(strict)
	second, _ := json.Marshal(strict)
	if string(first) != string(second) {
		t.Fatal("normalization is not idempotent")
	}
	var violations []string
	collectStrictOpenAISchemaViolations("fixture", strict, &violations)
	if len(violations) != 0 {
		t.Fatal(violations)
	}
	for _, sample := range []struct {
		args  string
		valid bool
	}{
		{`{"mode":"read","nullable_enum":null,"rows":null,"constant":null,"choice":null}`, true},
		{`{"mode":"read","nullable_enum":"kept","rows":[{"id":0,"label":null}],"constant":7,"choice":{"id":0}}`, true},
		{`{"mode":null,"nullable_enum":null,"rows":null,"constant":null,"choice":null}`, false},
		{`{"mode":"read","nullable_enum":"wrong","rows":null,"constant":null,"choice":null}`, false},
		{`{"mode":"read","nullable_enum":null,"rows":[{"id":0,"label":"x"}],"constant":null,"choice":null}`, false},
		{`{"mode":"read","nullable_enum":null,"rows":[],"constant":8,"choice":null}`, false},
	} {
		result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(strict), gojsonschema.NewStringLoader(sample.args))
		if err != nil || result.Valid() != sample.valid {
			t.Fatalf("%s: result=%v err=%v", sample.args, result, err)
		}
	}
}

func TestStrictNullPreparationAcrossTransports(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	params := schema(map[string]interface{}{
		"background": prop("boolean", "Background"), "priority": prop("integer", "Priority"),
		"title": prop("string", "Title"), "tags": map[string]interface{}{"type": "array", "items": prop("string", "Tag")},
		"metadata":  schema(map[string]interface{}{"keep": prop("string", "Value"), "omit": prop("string", "Optional")}, "keep"),
		"free_data": prop("object", "Free payload"), "json": prop("string", "JSON"),
		"required_nullable": map[string]interface{}{"type": []string{"string", "null"}},
	}, "required_nullable")
	definitions := []openai.Tool{tool("fixture", "fixture", params), tool("skill__fixture", "fixture", params), tool("tool__fixture", "fixture", params)}
	dc := &DispatchContext{Cfg: &config.Config{}, Logger: logger, SessionID: t.Name()}
	SetDiscoverToolsState(dc.SessionID, definitions, definitions, "")
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	args := `{"background":false,"priority":0,"title":"","tags":[],"metadata":{"keep":"yes","omit":null},"free_data":{"omit":null},"json":"{\"omit\":null}","required_nullable":null}`
	for _, name := range []string{"fixture", "skill__fixture", "tool__fixture"} {
		for _, wrapped := range []bool{false, true} {
			nativeName, nativeArgs := name, args
			if wrapped {
				raw, _ := json.Marshal(map[string]interface{}{"tool_name": name, "arguments": args})
				nativeName, nativeArgs = "invoke_tool", string(raw)
			}
			tc := NativeToolCallToToolCall(openai.ToolCall{ID: "call-original", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: nativeName, Arguments: nativeArgs}}, logger)
			tc = prepareToolCall(tc, dc)
			if tc.PreparationError != "" || tc.NativeArgsMalformed || tc.NativeCallID != "call-original" {
				t.Fatalf("%s wrapped=%v: %+v", name, wrapped, tc)
			}
			actual := tc.Params
			background := actual["background"]
			if name == "tool__fixture" {
				actual = tc.Params["args"].(map[string]interface{})
			}
			if _, exists := actual["metadata"].(map[string]interface{})["omit"]; exists {
				t.Fatalf("%s kept synthetic null: %+v", name, actual)
			}
			if value, exists := actual["free_data"].(map[string]interface{})["omit"]; !exists || value != nil {
				t.Fatal("free payload changed")
			}
			if actual["json"] != `{"omit":null}` || background != false || actual["priority"] != float64(0) || actual["title"] != "" || len(actual["tags"].([]interface{})) != 0 {
				t.Fatalf("explicit values changed: %+v", actual)
			}
			if _, exists := actual["required_nullable"]; !exists {
				t.Fatal("required nullable argument removed")
			}
			if name == "fixture" && (tc.Background || tc.Priority != 0 || len(tc.Metadata) != 1) {
				t.Fatal("typed fields disagree with Params")
			}
			if name == "skill__fixture" && tc.Skill != "fixture" || name == "tool__fixture" && tc.Name != "fixture" {
				t.Fatal("shortcut target changed")
			}
			if wrapped && tc.TransportAction != "invoke_tool" {
				t.Fatal("lost transport identity")
			}
			if twice := prepareToolCall(tc, dc); !reflect.DeepEqual(tc, twice) {
				t.Fatal("argument preparation is not idempotent")
			}
		}
	}
}

func TestStrictCheatsheetNullTagsKeepAndEmptyTagsClear(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := tools.InitCheatsheetDB(filepath.Join(t.TempDir(), "cheatsheets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dc := &DispatchContext{Cfg: &config.Config{}, Logger: logger, CheatsheetDB: db, LongTermMem: &fakeVectorDB{}, SessionID: t.Name()}
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, allBuiltinToolFeatureFlags(), logger)
	SetDiscoverToolsState(dc.SessionID, schemas, schemas, "")
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	for _, wrapped := range []bool{false, true} {
		for _, tags := range []string{"null", "[]"} {
			sheet, err := tools.CheatsheetCreateWithTags(db, "Keep tags", "old", "agent", []string{"kept"})
			if err != nil {
				t.Fatal(err)
			}
			args := fmt.Sprintf(`{"operation":"update","id":%q,"content":"new","tags":%s}`, sheet.ID, tags)
			name := "cheatsheet"
			if wrapped {
				raw, _ := json.Marshal(map[string]interface{}{"tool_name": name, "arguments": args})
				name, args = "invoke_tool", string(raw)
			}
			tc := NativeToolCallToToolCall(openai.ToolCall{ID: "tags-call", Function: openai.FunctionCall{Name: name, Arguments: args}}, logger)
			result := DispatchToolCallResult(context.Background(), &tc, dc, "Update content")
			if result.Status != ToolResultSuccess {
				t.Fatalf("wrapped=%v tags=%s: %+v", wrapped, tags, result)
			}
			updated, err := tools.CheatsheetGet(db, sheet.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if tags == "null" {
				want = 1
			}
			if updated.Content != "new" || len(updated.Tags) != want {
				t.Fatalf("wrapped=%v tags=%s: %+v", wrapped, tags, updated)
			}
		}
	}
}

func TestStrictGameMakerSettingsNullRetained(t *testing.T) {
	definitions := GameMakerPhaseToolSchemas("planning", "3d")
	dc := &DispatchContext{Cfg: &config.Config{}, SessionID: t.Name()}
	SetDiscoverToolsState(dc.SessionID, definitions, definitions, "")
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	for _, wrapped := range []bool{false, true} {
		args := `{"operation":"set_design","design":{"settings":null,"presentation":null}}`
		name := "game_maker_project"
		if wrapped {
			raw, _ := json.Marshal(map[string]interface{}{"tool_name": name, "arguments": args})
			name, args = "invoke_tool", string(raw)
		}
		tc := prepareToolCall(NativeToolCallToToolCall(openai.ToolCall{Function: openai.FunctionCall{Name: name, Arguments: args}}, nil), dc)
		design, ok := tc.Params["design"].(map[string]interface{})
		if !ok || tc.PreparationError != "" {
			t.Fatalf("%+v", tc)
		}
		if settings, exists := design["settings"]; !exists || settings != nil {
			t.Fatal("settings:null lost its clearing semantics")
		}
		if _, exists := design["presentation"]; exists {
			t.Fatal("synthetic presentation null survived")
		}
	}
}

func TestStrictProviderChecksActualAgentRequests(t *testing.T) {
	requests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		requests++
		var violations []string
		for _, tool := range request.Tools {
			if tool.Function.Strict {
				collectStrictOpenAISchemaViolations(tool.Function.Name, tool.Function.Parameters.(map[string]interface{}), &violations)
			}
		}
		if len(violations) != 0 {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]interface{}{"message": strings.Join(violations, ";"), "type": "invalid_request_error"}})
			return
		}
		if len(request.Tools) == 0 {
			t.Error("agent sent no tools")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"Done. <done/>"},"finish_reason":"stop"}]}`)
	}))
	defer provider.Close()
	clientCfg := openai.DefaultConfig("local-fixture")
	clientCfg.BaseURL = provider.URL
	client := llm.WrapOpenAIClient(openai.NewClientWithConfig(clientCfg))
	bad := tool("bad", "negative control", schema(map[string]interface{}{"optional": prop("string", "Optional")}))
	bad.Function.Strict = true
	if _, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "gpt-4o", Tools: []openai.Tool{bad}}); err == nil {
		t.Fatal("provider accepted invalid Strict schema")
	}
	run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
	defer cleanup()
	run.SuppressTurnSideEffects = true
	run.Config.LLM.ProviderType, run.Config.LLM.Model, run.Config.LLM.UseNativeFunctions = "openai", "gpt-4o", true
	run.Config.CircuitBreaker.LLMTimeoutSeconds = 30
	run.LLMClient = client
	if _, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: "gpt-4o", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Hello"}}}, run, false, NoopBroker{}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want rejected control plus actual loop", requests)
	}
}
