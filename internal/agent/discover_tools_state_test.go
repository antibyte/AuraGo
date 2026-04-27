package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func TestHandleDiscoverToolsMarksHiddenToolForSession(t *testing.T) {
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

func TestHandleDiscoverToolsFamilyNameSurfacesEnabledYepAPITools(t *testing.T) {
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
	requestedSet := make(map[string]bool, len(requested))
	for _, name := range requested {
		requestedSet[name] = true
	}
	for _, want := range []string{"yepapi_instagram", "yepapi_youtube"} {
		if !requestedSet[want] {
			t.Fatalf("requested tools = %v, missing %s", requested, want)
		}
	}
}

func TestHandleDiscoverToolsSearchMarksHiddenToolForSession(t *testing.T) {
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
	if len(requested) != 1 || requested[0] != "yepapi_instagram" {
		t.Fatalf("requested tools = %v, want [yepapi_instagram]", requested)
	}
}

func TestGetDiscoverRequestedToolsIsSessionScoped(t *testing.T) {
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
