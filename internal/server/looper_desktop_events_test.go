package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

const looperStoryPath = "Documents/Looper/short-story.md"

// looperDesktopClient plays a Looper run whose work round writes the story
// and (against its instructions) already opens it, and whose finish step opens
// it in Writer. It records every tool output it is shown.
type looperDesktopClient struct {
	mu          sync.Mutex
	finish      string
	calls       int
	toolOutputs []string
}

func (c *looperDesktopClient) CandidateRoutes(openai.ChatCompletionRequest) []llm.ModelRoute {
	return []llm.ModelRoute{{ProviderID: "primary", ProviderType: "custom", Model: "test-model", Primary: true, ContextWindowOverride: 32000, MaxOutputTokensOverride: 2048}}
}

func (c *looperDesktopClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	last := req.Messages[len(req.Messages)-1]
	if last.Role == openai.ChatMessageRoleTool {
		for i := len(req.Messages) - 1; i >= 0 && req.Messages[i].Role == openai.ChatMessageRoleTool; i-- {
			c.toolOutputs = append(c.toolOutputs, req.Messages[i].Content)
		}
		return looperDesktopText("Done."), nil
	}
	switch {
	case strings.Contains(last.Content, "independent reviewer"):
		return looperDesktopText(`{"score":95,"done":true,"feedback":"good","summary":"finished"}`), nil
	case last.Content == c.finish:
		return looperDesktopToolCalls(
			map[string]any{"operation": "open_in_app", "app_id": "writer", "path": looperStoryPath},
		), nil
	case strings.Contains(last.Content, "Work instructions"):
		return looperDesktopToolCalls(
			map[string]any{"operation": "write_file", "path": looperStoryPath, "content": "# Story\n\nOnce upon a time."},
			map[string]any{"operation": "open_in_app", "app_id": "writer", "path": looperStoryPath},
		), nil
	}
	return openai.ChatCompletionResponse{}, errors.New("unexpected request: " + last.Content)
}

func (c *looperDesktopClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("streaming not implemented")
}

func looperDesktopText(text string) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: text},
		FinishReason: openai.FinishReasonStop,
	}}}
}

func looperDesktopToolCalls(calls ...map[string]any) openai.ChatCompletionResponse {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
	for i, args := range calls {
		raw, _ := json.Marshal(args)
		msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
			ID:       "call_" + string(rune('a'+i)),
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: "virtual_desktop", Arguments: string(raw)},
		})
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: openai.FinishReasonToolCalls}}}
}

func looperDesktopConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 32000
	cfg.Directories.WorkspaceDir = filepath.Join(root, "workspace")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "desktop.db")
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.VirtualDesktop.MaxFileSizeMB = 1
	cfg.Tools.VirtualDesktop.Enabled = true
	return cfg
}

type looperDesktopMessage struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Payload struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	} `json:"payload"`
}

func drainSSE(ch chan string) []string {
	var out []string
	for {
		select {
		case msg := <-ch:
			out = append(out, msg)
		default:
			return out
		}
	}
}

// The desktop opens an app window only when a virtual_desktop_event reaches
// it over SSE, so the Looper's finish step must deliver its open_in_app event.
func TestLooperFinishOpensTheResultInTheDesktop(t *testing.T) {
	cfg := looperDesktopConfig(t)
	finish := "Open " + looperStoryPath + " in the Writer app with virtual_desktop open_in_app so the user can read the finished story."
	client := &looperDesktopClient{finish: finish}
	sse := NewSSEBroadcaster()
	events := sse.subscribe()
	defer sse.unsubscribe(events)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Cfg: cfg, SSE: sse, LLMClient: client, Logger: logger}

	auraCfg, llmClient, model, dispatchCtx, toolSchemas, err := buildLooperRuntime(s, "", "test-model")
	if err != nil {
		t.Fatalf("buildLooperRuntime: %v", err)
	}
	runCfg := desktop.LooperRunConfig{
		Goal:        "Write a short story and save it as " + looperStoryPath + ".",
		Work:        "Improve " + looperStoryPath + ".",
		Evaluate:    "Judge the story.",
		Finish:      finish,
		MaxRounds:   2,
		TargetScore: 90,
		Model:       model,
	}
	desktop.NormalizeLooperRunConfig(&runCfg)
	runner := NewLooperRunner(nil, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.TryStart(runCfg.MaxRounds, cancel); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	if err := runner.executeStarted(ctx, runCfg, auraCfg, llmClient, toolSchemas, dispatchCtx, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if state := runner.State(); state.Status != "completed" {
		t.Fatalf("status = %q, want completed (logs %+v)", state.Status, state.Logs)
	}

	client.mu.Lock()
	outputs := append([]string(nil), client.toolOutputs...)
	client.mu.Unlock()
	opened := 0
	for _, out := range outputs {
		if strings.Contains(out, "desktop app open event emitted") {
			opened++
		}
	}
	if opened != 2 {
		t.Fatalf("open_in_app succeeded %d times, want 2 (work and finish); tool outputs: %q", opened, outputs)
	}

	var opens []map[string]any
	for _, raw := range drainSSE(events) {
		var msg looperDesktopMessage
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			t.Fatalf("bad SSE message %q: %v", raw, err)
		}
		if msg.Type != string(EventVirtualDesktop) {
			t.Fatalf("Looper tool feedback leaked to the chat stream: %s", raw)
		}
		if msg.Payload.Type == "open_app" {
			opens = append(opens, msg.Payload.Payload)
		}
	}
	if len(opens) != 1 {
		t.Fatalf("open_app events = %v, want exactly the finish step's one (a work round must not open windows)", opens)
	}
	if opens[0]["app_id"] != "writer" || opens[0]["path"] != looperStoryPath {
		t.Fatalf("open_app payload = %v, want writer with %s", opens[0], looperStoryPath)
	}
}
