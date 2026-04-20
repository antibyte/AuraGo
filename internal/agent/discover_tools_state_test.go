package agent

import (
	"io"
	"log/slog"
	"strings"
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

	if !strings.Contains(out, "hidden by adaptive filtering but enabled") {
		t.Fatalf("unexpected output: %s", out)
	}
	requested := GetDiscoverRequestedTools("sess-1")
	if len(requested) != 1 || requested[0] != "chromecast" {
		t.Fatalf("requested tools = %v, want [chromecast]", requested)
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
