package agent

import (
	"aurago/internal/config"
	"aurago/internal/meshcore"
	"context"
	"database/sql"
	"encoding/json"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeshCoreNoticeOnlyAtDirectContact(t *testing.T) {
	dir := t.TempDir()
	m, err := meshcore.NewManager(context.Background(), dir, meshcore.Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	previous := meshcore.DefaultManager()
	meshcore.SetDefaultManager(m)
	defer meshcore.SetDefaultManager(previous)
	db, err := sql.Open("sqlite", filepath.Join(dir, "meshcore.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	msg := meshcore.Message{ID: strings.Repeat("44", 32), Direction: "incoming", Kind: "direct", Sender: strings.Repeat("22", 6), Text: "PRIVATE_EXTERNAL_BODY", State: "quarantine"}
	data, _ := json.Marshal(msg)
	if _, err := db.Exec("INSERT INTO meshcore_messages(id,received,state,data) VALUES(?,0,?,?)", msg.ID, msg.State, data); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{MeshCore: meshcore.Config{Enabled: true}}
	for _, rc := range []RunConfig{{Config: cfg, MessageSource: "meshcore"}, {Config: cfg, MessageSource: "web_chat", IsMission: true}, {Config: cfg, MessageSource: "web_chat", IsCoAgent: true}, {Config: cfg, MessageSource: "web_chat", IsMaintenance: true}} {
		appendMeshCoreNotice(&rc)()
		if len(rc.TrustedPromptAddenda) != 0 {
			t.Fatal("background consumed inbox notice")
		}
	}
	rc := RunConfig{Config: cfg, MessageSource: "web_chat"}
	ack := appendMeshCoreNotice(&rc)
	if len(rc.TrustedPromptAddenda) != 1 || !strings.Contains(rc.TrustedPromptAddenda[0].Text, msg.ID) || strings.Contains(rc.TrustedPromptAddenda[0].Text, msg.Text) {
		t.Fatal("unsafe or missing notice")
	}
	if notice, _, _ := m.PendingNotice(); notice == "" {
		t.Fatal("notice acknowledged before completion")
	}
	ack()
	if notice, _, _ := m.PendingNotice(); notice != "" {
		t.Fatal("notice not acknowledged")
	}
}

type meshSearchTransport func(*http.Request) (*http.Response, error)

func (f meshSearchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMeshCoreMinimalLoopEnforcesSchemasDispatcherAndCallCount(t *testing.T) {
	old := http.DefaultTransport
	calls := 0
	http.DefaultTransport = meshSearchTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.search.brave.com" || r.URL.Query().Get("q") != "LoRa" {
			t.Fatalf("unexpected search %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"web":{"results":[{"title":"LoRa","url":"https://example.org","description":"Public answer"}]}}`))}, nil
	})
	defer func() { http.DefaultTransport = old }()
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	cfg.BraveSearch.Enabled = true
	cfg.BraveSearch.APIKey = "test"
	cfg.MCP.PreferredCapabilities.WebSearch = config.MCPPreferredToolSelection{Server: "private-mcp", Tool: "private_search"}
	dc := &DispatchContext{Cfg: cfg, MessageSource: "meshcore_reply", ToolScopeRestricted: true, AllowedTools: map[string]struct{}{"brave_search": {}}, SkillScopeRestricted: true, AllowedAgentSkills: map[string]struct{}{}}
	for _, name := range []string{"execute_shell", "read_file", "execute_python", "execute_skill", "mcp_call", "co_agent", "missions", "meshcore", "invoke_tool", "activate_tools", "discover_tools"} {
		tc := ToolCall{Action: name, Params: map[string]interface{}{"tool": "brave_search", "query": "LoRa"}}
		out := DispatchToolCall(context.Background(), &tc, dc, "untrusted")
		if !strings.Contains(out, "tool_scope_denied") {
			t.Fatalf("%s escaped: %s", name, out)
		}
	}
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	client.respond = func(req openai.ChatCompletionRequest, n int) (openai.ChatCompletionResponse, error) {
		msg := openai.ChatCompletionMessage{Role: "assistant", Content: "Public answer"}
		reason := openai.FinishReasonStop
		if n == 1 {
			if len(req.Tools) != 1 || req.Tools[0].Function.Name != "brave_search" {
				t.Fatalf("schemas escaped scope: %+v", req.Tools)
			}
			msg.Content = ""
			for _, id := range []string{"a", "b", "c"} {
				msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "brave_search", Arguments: `{"query":"LoRa"}`}})
			}
			reason = openai.FinishReasonToolCalls
		} else if len(req.Tools) != 0 {
			t.Fatal("tools remained after cap")
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: msg, FinishReason: reason}}}, nil
	}
	res, hist, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", "Public only", "What is LoRa?", []openai.Tool{MeshCoreSearchSchema(), meshCoreToolSchema()}, dc, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), &MinimalLoopOptions{MaxToolRounds: 2, MaxToolCalls: 2})
	if err != nil || res.Response != "Public answer" || res.ToolCalls != 2 || calls != 2 {
		t.Fatalf("%+v calls=%d %v", res, calls, err)
	}
	results := 0
	for _, m := range hist {
		if m.Role == "tool" {
			results++
		}
	}
	if results != 3 {
		t.Fatalf("missing contiguous tool results: %d", results)
	}
}

func TestMeshCoreToolFreeRejectsUnexpectedCalls(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes(), respond: func(openai.ChatCompletionRequest, int) (openai.ChatCompletionResponse, error) {
			m := openai.ChatCompletionMessage{Role: "assistant", Content: "safe 0 fine"}
			if legacy {
				m.FunctionCall = &openai.FunctionCall{Name: "execute_shell"}
			} else {
				m.ToolCalls = []openai.ToolCall{{ID: "bad", Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{}`}}}
			}
			return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: m, FinishReason: openai.FinishReasonStop}}}, nil
		}}
		cfg := &config.Config{}
		cfg.Agent.ContextWindow = 6000
		dc := &DispatchContext{Cfg: cfg, MessageSource: "meshcore_reply", ToolScopeRestricted: true, AllowedTools: map[string]struct{}{}}
		res, _, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", "scan", "untrusted", nil, dc, nil, nil, &MinimalLoopOptions{MaxToolRounds: 0})
		if err == nil || res.ToolCalls != 0 {
			t.Fatalf("unexpected call processed: %+v %v", res, err)
		}
	}
}
