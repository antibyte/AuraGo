package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

func TestHandleDiscoverToolsMarksHiddenToolForSession(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "chromecast",
				Description: "Control Chromecast devices",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-1", allSchemas, nil, "")

	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "get_tool_info",
			"tool_name": "chromecast",
		},
	}, cfg, logger, "sess-1")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Tool == nil || payload.Tool.ToolStatus != string(ToolStatusHidden) || payload.Tool.CallMethod != "invoke_tool" {
		t.Fatalf("unexpected output: %+v raw=%s", payload, out)
	}
	requested := GetDiscoverRequestedTools("sess-1")
	if len(requested) != 1 || requested[0] != "chromecast" {
		t.Fatalf("requested tools = %v, want [chromecast]", requested)
	}
}

func TestHandleDiscoverToolsAllowsNilLogger(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "chromecast",
				Description: "Control Chromecast devices",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-nil-logger", allSchemas, allSchemas, "")

	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "search",
			"query":     "chromecast",
		},
	}, &config.Config{}, nil, "sess-nil-logger")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Status != "success" {
		t.Fatalf("status = %q, want success: %s", payload.Status, out)
	}
}

func TestHandleDiscoverToolsFamilyNameSurfacesEnabledYepAPITools(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "yepapi_instagram",
				Description: "Instagram data via YepAPI",
				Parameters:  map[string]any{"type": "object"},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "yepapi_youtube",
				Description: "YouTube data via YepAPI",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-yepapi", allSchemas, nil, "")

	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "search",
			"query":     "yepapi",
		},
	}, cfg, logger, "sess-yepapi")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Status != "success" || len(payload.Results) < 2 {
		t.Fatalf("expected YepAPI family tools in output, got: %+v raw=%s", payload, out)
	}
	requested := GetDiscoverRequestedTools("sess-yepapi")
	if len(requested) != 0 {
		t.Fatalf("family discovery should not permanently request every family tool, got %v", requested)
	}
}

func TestHandleDiscoverToolsSearchMarksHiddenToolForSession(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "yepapi_instagram",
				Description: "Instagram data via YepAPI",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-search", allSchemas, nil, "")

	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "search",
			"query":     "yepapi_instagram",
		},
	}, cfg, logger, "sess-search")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if len(payload.Results) != 1 {
		t.Fatalf("expected one result, got: %+v raw=%s", payload, out)
	}
	got := payload.Results[0]
	if got.Name != "yepapi_instagram" || got.ToolStatus != string(ToolStatusHidden) || got.CallMethod != "invoke_tool" {
		t.Fatalf("expected hidden enabled result, got: %+v raw=%s", got, out)
	}
	requested := GetDiscoverRequestedTools("sess-search")
	if len(requested) != 0 {
		t.Fatalf("search should not request hidden tools until exact get_tool_info or invoke_tool, got %v", requested)
	}
}

func TestGetDiscoverRequestedToolsIsSessionScoped(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	MarkDiscoverRequestedTool("sess-a", "chromecast")
	MarkDiscoverRequestedTool("sess-b", "proxmox")

	gotA := GetDiscoverRequestedTools("sess-a")
	gotB := GetDiscoverRequestedTools("sess-b")
	if len(gotA) != 1 || gotA[0] != "chromecast" {
		t.Fatalf("sess-a requested tools = %v, want [chromecast]", gotA)
	}
	if len(gotB) != 1 || gotB[0] != "proxmox" {
		t.Fatalf("sess-b requested tools = %v, want [proxmox]", gotB)
	}
}

func TestMarkDiscoverRequestedToolUsesDefaultSessionForEmptyID(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	MarkDiscoverRequestedTool("", "chromecast")

	got := ConsumeDiscoverRequestedTools("")
	if len(got) != 1 || got[0] != "chromecast" {
		t.Fatalf("default-session requested tools = %v, want [chromecast]", got)
	}
}

func TestDiscoverToolsCatalogIsSessionScoped(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	chromecastSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "chromecast",
				Description: "Control Chromecast devices",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	proxmoxSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "proxmox",
				Description: "Manage Proxmox hosts",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-a", chromecastSchemas, chromecastSchemas, "")
	SetDiscoverToolsState("sess-b", proxmoxSchemas, proxmoxSchemas, "")

	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	out := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"operation": "get_tool_info",
			"tool_name": "chromecast",
		},
	}, cfg, logger, "sess-a")

	var payload DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Tool == nil || payload.Tool.Name != "chromecast" || payload.Tool.ToolStatus != string(ToolStatusActive) {
		t.Fatalf("session A discovery used the wrong catalog: %+v raw=%s", payload, out)
	}
}

func TestSetDiscoverToolsStateExpiresOldSnapshots(t *testing.T) {
	t.Cleanup(func() {
		SetDiscoverToolsSnapshotTTL(5 * time.Minute)
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	oldSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "chromecast",
				Description: "Control Chromecast devices",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("old-session", oldSchemas, oldSchemas, "")
	MarkDiscoverRequestedTool("old-session", "chromecast")

	discoverToolsState.mu.Lock()
	snapshot := discoverToolsState.snapshots["old-session"]
	snapshot.updatedAt = time.Now().Add(-discoverToolsSnapshotTTL - time.Minute)
	discoverToolsState.snapshots["old-session"] = snapshot
	discoverToolsState.mu.Unlock()

	freshSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "proxmox",
				Description: "Manage Proxmox hosts",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("fresh-session", freshSchemas, freshSchemas, "")

	if catalog := GetToolCatalogState("old-session"); catalog != nil {
		t.Fatalf("old-session catalog should have expired, got %+v", catalog)
	}
	if requested := GetDiscoverRequestedTools("old-session"); len(requested) != 0 {
		t.Fatalf("old-session requested tools should have expired, got %v", requested)
	}
	if catalog := GetToolCatalogState("fresh-session"); catalog == nil {
		t.Fatal("fresh-session catalog missing after pruning")
	}
}

func TestSetDiscoverToolsSnapshotTTLFallsBackToDefault(t *testing.T) {
	t.Cleanup(func() {
		SetDiscoverToolsSnapshotTTL(5 * time.Minute)
	})

	SetDiscoverToolsSnapshotTTL(12 * time.Minute)
	if got := DiscoverToolsSnapshotTTL(); got != 12*time.Minute {
		t.Fatalf("DiscoverToolsSnapshotTTL() = %v, want 12m", got)
	}

	SetDiscoverToolsSnapshotTTL(0)
	if got := DiscoverToolsSnapshotTTL(); got != 5*time.Minute {
		t.Fatalf("DiscoverToolsSnapshotTTL() after zero = %v, want default 5m", got)
	}
}

func TestSetDiscoverToolsStateExpiresUninitializedSnapshots(t *testing.T) {
	t.Cleanup(func() {
		SetDiscoverToolsSnapshotTTL(5 * time.Minute)
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	discoverToolsState.mu.Lock()
	discoverToolsState.snapshots = map[string]discoverToolsSnapshot{
		"uninitialized-session": {
			catalog: BuildToolCatalog([]openai.Tool{
				{
					Type: openai.ToolTypeFunction,
					Function: &openai.FunctionDefinition{
						Name:        "chromecast",
						Description: "Control Chromecast devices",
						Parameters:  map[string]any{"type": "object"},
					},
				},
			}, nil, ""),
		},
	}
	discoverToolsState.requested = map[string]map[string]int{
		"uninitialized-session": {"chromecast": 1},
	}
	discoverToolsState.mu.Unlock()

	freshSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "proxmox",
				Description: "Manage Proxmox hosts",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("fresh-after-zero", freshSchemas, freshSchemas, "")

	if catalog := GetToolCatalogState("uninitialized-session"); catalog != nil {
		t.Fatalf("uninitialized-session catalog should have expired, got %+v", catalog)
	}
	if requested := GetDiscoverRequestedTools("uninitialized-session"); len(requested) != 0 {
		t.Fatalf("uninitialized-session requested tools should have expired, got %v", requested)
	}
}

func TestConsumeDiscoverRequestedToolsIsOneShot(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	MarkDiscoverRequestedTool("sess-once", "chromecast")

	first := ConsumeDiscoverRequestedTools("sess-once")
	if len(first) != 1 || first[0] != "chromecast" {
		t.Fatalf("first consume = %v, want [chromecast]", first)
	}
	second := ConsumeDiscoverRequestedTools("sess-once")
	if len(second) != 0 {
		t.Fatalf("second consume = %v, want empty", second)
	}
}

func TestHandleLegacyActivateToolsReportsRequiredCallMethodsWithoutMutation(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "docker",
				Description: "Manage Docker resources",
				Parameters:  map[string]any{"type": "object"},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "chromecast",
				Description: "Control Chromecast devices",
				Parameters:  map[string]any{"type": "object"},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "skill__weather_check",
				Description: "Check weather",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-activate", allSchemas, allSchemas[:1], "")

	out := handleActivateTools(ToolCall{
		Params: map[string]interface{}{
			"names": []interface{}{"docker", "chromecast", "weather_check", "uptime_kuma", "no_such_tool"},
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), "sess-activate")

	var payload ActivateToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Status != "success" {
		t.Fatalf("unexpected activate_tools response: %+v raw=%s", payload, out)
	}
	if payload.RequiredCallMethods["docker"] != "direct" {
		t.Fatalf("docker call method = %q, want direct", payload.RequiredCallMethods["docker"])
	}
	if payload.RequiredCallMethods["chromecast"] != "invoke_tool" {
		t.Fatalf("chromecast call method = %q, want invoke_tool", payload.RequiredCallMethods["chromecast"])
	}
	if payload.RequiredCallMethods["skill__weather_check"] != "execute_skill" {
		t.Fatalf("weather call method = %q, want execute_skill", payload.RequiredCallMethods["skill__weather_check"])
	}
	if !containsName(payload.Disabled, "uptime_kuma") {
		t.Fatalf("disabled = %v, want uptime_kuma", payload.Disabled)
	}
	if !containsName(payload.Unknown, "no_such_tool") {
		t.Fatalf("unknown = %v, want no_such_tool", payload.Unknown)
	}

	if entry, ok := GetToolCatalogState("sess-activate").Get("chromecast"); !ok || entry.Active {
		t.Fatalf("legacy handler mutated hidden tool state: %+v", entry)
	}
}

func TestLegacyActivateToolsThroughNativeDispatcherDoesNotMutateState(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name: "docker", Parameters: map[string]any{"type": "object"},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name: "chromecast", Parameters: map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-dispatch-activate", allSchemas, allSchemas[:1], "")
	call := ToolCall{
		Action: "activate_tools", IsTool: true,
		Params: map[string]interface{}{"names": []interface{}{"chromecast"}},
	}
	out := DispatchToolCall(context.Background(), &call, &DispatchContext{
		Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionID: "sess-dispatch-activate",
	}, "activate the discovered tool")
	if !strings.Contains(out, `"required_call_methods":{"chromecast":"invoke_tool"}`) {
		t.Fatalf("dispatcher output = %q", out)
	}
	if entry, ok := GetToolCatalogState("sess-dispatch-activate").Get("chromecast"); !ok || entry.Active {
		t.Fatalf("legacy dispatcher mutated hidden tool state: %+v", entry)
	}
}

func TestHandleActivateToolsRejectsMoreThanEightNames(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	SetDiscoverToolsState("sess-too-many", []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:       "chromecast",
				Parameters: map[string]any{"type": "object"},
			},
		},
	}, nil, "")

	out := handleActivateTools(ToolCall{
		Params: map[string]interface{}{
			"names": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		},
	}, nil, "sess-too-many")
	var payload ActivateToolsResponse
	decodeToolOutputJSON(t, out, &payload)
	if payload.Status != "error" {
		t.Fatalf("status = %q, want error: %s", payload.Status, out)
	}
}

func TestHandleDiscoverToolsAliasFallbacks(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	allSchemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "chromecast",
				Description: "Control Chromecast devices",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	SetDiscoverToolsState("sess-fallback", allSchemas, nil, "")

	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Test 1: Fallback for 'operation' and 'category'
	out1 := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"op":  "list_categories",
			"cat": "system",
		},
	}, cfg, logger, "sess-fallback")
	var payload1 DiscoverToolsResponse
	decodeToolOutputJSON(t, out1, &payload1)
	if payload1.Status != "success" || payload1.Category != "system" {
		t.Fatalf("list_categories fallback failed: status=%s, category=%s, raw=%s", payload1.Status, payload1.Category, out1)
	}

	// Test 2: Fallback for 'query'
	out2 := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"op": "search",
			"q":  "chromecast",
		},
	}, cfg, logger, "sess-fallback")
	var payload2 DiscoverToolsResponse
	decodeToolOutputJSON(t, out2, &payload2)
	if payload2.Status != "success" || len(payload2.Results) != 1 {
		t.Fatalf("search fallback failed: status=%s, results=%d, raw=%s", payload2.Status, len(payload2.Results), out2)
	}

	// Test 3: Fallback for 'tool_name' using 'name'
	out3 := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"op":   "get_tool_info",
			"name": "chromecast",
		},
	}, cfg, logger, "sess-fallback")
	var payload3 DiscoverToolsResponse
	decodeToolOutputJSON(t, out3, &payload3)
	if payload3.Status != "success" || payload3.Tool == nil || payload3.Tool.Name != "chromecast" {
		t.Fatalf("get_tool_info fallback name failed: status=%s, raw=%s", payload3.Status, out3)
	}

	// Test 4: Fallback for 'tool_name' using 'tool'
	out4 := handleDiscoverTools(ToolCall{
		Params: map[string]interface{}{
			"op":   "get_tool_info",
			"tool": "chromecast",
		},
	}, cfg, logger, "sess-fallback")
	var payload4 DiscoverToolsResponse
	decodeToolOutputJSON(t, out4, &payload4)
	if payload4.Status != "success" || payload4.Tool == nil || payload4.Tool.Name != "chromecast" {
		t.Fatalf("get_tool_info fallback tool failed: status=%s, raw=%s", payload4.Status, out4)
	}
}

// flakyGuideSearcher answers the manual search with the given manuals, or fails
// the way an unreachable embedding provider does.
type flakyGuideSearcher struct {
	memory.VectorDB
	fail    bool
	manuals []string
}

func (s *flakyGuideSearcher) SearchToolGuideMatchesContext(context.Context, string, int) ([]memory.ToolGuideMatch, error) {
	if s.fail {
		return nil, errors.New("embedding provider unavailable")
	}
	return manualGuideSearcher{manuals: s.manuals}.SearchToolGuideMatchesContext(context.Background(), "", 0)
}

// TestDiscoverToolsSearchPagesKeepTheOrderOfTheFirstPage guards the cursor of a
// search: whether the manual search answers can differ from call to call, and
// a page of another order must not reset the cursor of the page before.
func TestDiscoverToolsSearchPagesKeepTheOrderOfTheFirstPage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		failFirst, then bool
	}{
		{"manual search fails on the first page", true, false},
		{"manual search fails on the second page", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Eight tools match the query; the manual search adds three more.
			var schemas []openai.Tool
			for i := 0; i < 8; i++ {
				schemas = append(schemas, testToolSchema(fmt.Sprintf("lexical_%d", i), "Frobnicate widgets"))
			}
			manuals := []string{"manual_a", "manual_b", "manual_c"}
			for _, manual := range manuals {
				schemas = append(schemas, testToolSchema(manual, "Unrelated text "+manual))
			}
			sid := t.Name()
			SetDiscoverToolsState(sid, schemas, nil, "")
			t.Cleanup(func() { ClearDiscoverToolsState(sid) })
			source := &flakyGuideSearcher{fail: tc.failFirst, manuals: manuals}
			dc := &DispatchContext{SessionID: sid, LongTermMem: source}
			page := func(cursor string) DiscoverToolsResponse {
				params := map[string]interface{}{"operation": "search", "query": "frobnicate widgets"}
				if cursor != "" {
					params["cursor"] = cursor
				}
				var response DiscoverToolsResponse
				decodeToolOutputJSON(t, handleDiscoverToolsContext(context.Background(), ToolCall{Params: params}, &config.Config{}, nil, sid, dc), &response)
				return response
			}
			first := page("")
			if first.NextCursor == "" {
				t.Fatalf("fixture: the search has no second page: %d results of %d", len(first.Results), first.Total)
			}
			source.fail = tc.then
			second := page(first.NextCursor)
			if second.CursorReset || len(second.Results) == 0 {
				t.Fatalf("the second page did not continue the first: reset=%v results=%d", second.CursorReset, len(second.Results))
			}
			for _, a := range first.Results {
				for _, b := range second.Results {
					if a.Name == b.Name {
						t.Errorf("%s is on both pages", a.Name)
					}
				}
			}
		})
	}
}

// TestDiscoverSearchOrderMemoIsBounded guards the memory of the paging memo: a
// session cannot grow it without limit, and an order lives for the snapshot TTL.
func TestDiscoverSearchOrderMemoIsBounded(t *testing.T) {
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	for i := 0; i < 3*discoverSearchOrderLimit; i++ {
		rememberDiscoverSearchOrder(discoverSearchKey{sid, fmt.Sprint(i), "r"}, []string{"a"})
	}
	discoverToolsState.mu.RLock()
	kept := len(discoverToolsState.orders)
	discoverToolsState.mu.RUnlock()
	if kept > discoverSearchOrderLimit {
		t.Errorf("%d orders kept, limit %d", kept, discoverSearchOrderLimit)
	}
	long := discoverSearchKey{sid, "long", "r"}
	rememberDiscoverSearchOrder(long, make([]string, discoverSearchOrderNames+1))
	if _, ok := recallDiscoverSearchOrder(long); ok {
		t.Errorf("an order of %d names was kept", discoverSearchOrderNames+1)
	}

	SetDiscoverToolsSnapshotTTL(20 * time.Millisecond)
	t.Cleanup(func() { SetDiscoverToolsSnapshotTTL(0) })
	expiring := discoverSearchKey{sid, "expiring", "r"}
	rememberDiscoverSearchOrder(expiring, []string{"a"})
	if _, ok := recallDiscoverSearchOrder(expiring); !ok {
		t.Fatalf("a fresh order was not recalled")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := recallDiscoverSearchOrder(expiring); ok {
		t.Errorf("an order outlived the snapshot TTL")
	}
	rememberDiscoverSearchOrder(expiring, []string{"a"})
	ClearDiscoverToolsState(sid)
	if _, ok := recallDiscoverSearchOrder(expiring); ok {
		t.Errorf("clearing the session kept its orders")
	}
}
