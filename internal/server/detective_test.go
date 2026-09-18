package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/detective"
	"aurago/internal/tools"
	"github.com/sashabaranov/go-openai"
)

func TestDetectiveRealAgentLoopPublishesViaScopedTool(t *testing.T) {
	var calls atomic.Int32
	var prefix, toolPrefix string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n > 3 {
			t.Error("agent did not stop after publication")
			http.Error(w, "too many calls", 500)
			return
		}
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		toolBytes, _ := json.Marshal(req.Tools)
		if n == 1 {
			prefix = req.Messages[0].Content
			toolPrefix = string(toolBytes)
		} else if prefix != req.Messages[0].Content || toolPrefix != string(toolBytes) {
			t.Error("stable prompt/tool prefix changed between rounds")
		}
		found := false
		for _, tool := range req.Tools {
			if tool.Function.Name == "detective_report" {
				found = true
			}
			if tool.Function.Name == "execute_shell" {
				t.Error("shell exposed")
			}
		}
		if !found {
			t.Error("report tool missing")
		}
		content, _ := json.Marshal(detective.Report{Title: "Fixture", Summary: "Source unavailable", Partial: true, Limitations: "fixture only", Blocks: []detective.Block{{Type: "paragraph", Text: "No evidence was retrieved."}}})
		args, _ := json.Marshal(map[string]string{"operation": "finish", "content": string(content)})
		if n == 1 {
			args, _ = json.Marshal(map[string]string{"operation": "plan", "content": `["Read primary sources"]`})
		}
		delta := map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "report-1", "type": "function", "function": map[string]string{"name": "detective_report", "arguments": string(args)}}}}, "finish_reason": "tool_calls"}}, "usage": map[string]int{"prompt_tokens": 100, "completion_tokens": 80, "total_tokens": 180}}
		b, _ := json.Marshal(delta)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.LLM.Model = "test-research"
	cfg.LLM.ProviderType = "openai"
	cfg.Agent.ContextWindow = 65536
	cfg.CircuitBreaker.LLMTimeoutSeconds = 10
	cfg.Detective.Enabled = true
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.ToolsDir = filepath.Join(cfg.Directories.WorkspaceDir, "tools")
	cc := openai.DefaultConfig("fixture")
	cc.BaseURL = provider.URL
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "case.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s := &Server{Cfg: cfg, Detective: svc, LLMClient: openai.NewClientWithConfig(cc), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Registry = tools.NewProcessRegistry(s.Logger)
	svc.SetRunner(detectiveTestRunner(func(ctx context.Context, job *detective.Session) (err error) {
		defer func() {
			if v := recover(); v != nil {
				t.Errorf("panic: %v\n%s", v, debug.Stack())
				err = fmt.Errorf("panic")
			}
		}()
		err = (&detectiveRunner{server: s}).Run(ctx, job)
		if err != nil {
			t.Logf("runner: %v", err)
		}
		return err
	}))
	c, _ := svc.Create(detective.Request{Topic: "A test topic", Effort: "quick"})
	_, err = svc.Start(c.ID, "start", "", "test")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		c, _ = svc.Get(c.ID)
		if len(c.Reports) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(c.Reports) != 1 || c.Reports[0].Title != "Fixture" {
		t.Fatalf("report not published: %+v", c)
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected LLM requests: %d", calls.Load())
	}
	saved, err := svc.Continuation(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = saved
}

func TestDetectiveOperationScope(t *testing.T) {
	cfg := &config.Config{}
	req := detective.Request{}
	for _, tc := range []agent.ToolCall{{Action: "execute_shell"}, {Action: "execute_skill", Skill: "evil"}, {Action: "api_request", Method: "POST"}, {Action: "browser_automation", Operation: "evaluate"}, {Action: "virtual_browser", Operation: "click"}} {
		if detectiveAuthorize(cfg, req, tc) == nil {
			t.Errorf("accepted forbidden call: %+v", tc)
		}
	}
	if err := detectiveAuthorize(cfg, req, agent.ToolCall{Action: "api_request", Method: "GET"}); err != nil {
		t.Fatal(err)
	}
}

func TestDetectiveHTTPGates(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.Detective.Enabled = true
	cfg.Auth.Enabled = true
	s := &Server{Cfg: cfg}
	r := httptest.NewRequest("GET", "/api/desktop/detective/cases", nil)
	w := httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 401 {
		t.Fatalf("unauthenticated case access: %d", w.Code)
	}
	cfg.Auth.Enabled = false
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "case.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s.Detective = svc
	r = httptest.NewRequest("POST", "/api/desktop/detective/cases", strings.NewReader(`{"topic":"test"}`))
	w = httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 403 {
		t.Fatalf("agent control disabled: %d", w.Code)
	}
	cfg.VirtualDesktop.AllowAgentControl = true
	r = httptest.NewRequest("POST", "/api/desktop/detective/cases", strings.NewReader(`{"topic":"test","effort":"quick"}`))
	w = httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var c detective.Case
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	_ = svc.SaveContinuation(c.ID, detective.Continuation{Messages: json.RawMessage(`[{"reasoning_content":"private-fixture"}]`)})
	r = httptest.NewRequest("GET", "/api/desktop/detective/cases/"+c.ID, nil)
	w = httptest.NewRecorder()
	s.handleDetective(w, r)
	if strings.Contains(w.Body.String(), "private-fixture") {
		t.Fatal("private context leaked")
	}
	if _, err := svc.ExportRevision(context.Background(), c.ID, 0, "md"); err == nil {
		t.Fatal("invalid revision exported")
	}
}

type detectiveTestRunner func(context.Context, *detective.Session) error

func (f detectiveTestRunner) Run(ctx context.Context, s *detective.Session) error { return f(ctx, s) }
