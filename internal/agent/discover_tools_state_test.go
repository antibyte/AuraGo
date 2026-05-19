package agent

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"

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
			"operation": "get_tool_info",
			"tool_name": "yepapi",
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
