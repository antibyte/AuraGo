package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestFlowTypedPDFOutputDoesNotBecomeModelCopiedText(t *testing.T) {
	for _, source := range []string{MessageSourceFlow, "web_chat", "mission"} {
		t.Run(source, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.AllowFilesystemWrite = true
			cfg.Tools.DocumentCreator.Enabled = true
			cfg.Tools.DocumentCreator.Backend = "maroto"
			cfg.Agent.AllowNetworkRequests = true
			cfg.Telegram.BotToken = "test-placeholder-not-a-real-token"
			cfg.Telegram.UserID = 123
			path := "/workspace/generated/reports/" + strings.Repeat("weekly-", 6) + "news.pdf"
			body, err := json.Marshal(map[string]any{"status": "success", "file_path": path})
			if err != nil {
				t.Fatal(err)
			}
			dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), MessageSource: source, FlowID: "flow-fixture",
				Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
				ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { return string(body), true }},
			}
			if source == "mission" {
				dc.MissionID = "mission-fixture"
			}
			t.Cleanup(func() { clearIsolatedModelBodies(dc.SessionID) })
			create := ToolCall{Action: "document_creator", Operation: "create_pdf", Params: map[string]interface{}{"operation": "create_pdf", "content": "Public news"}}
			created := DispatchToolCallResult(context.Background(), &create, dc, "")
			if created.Status != ToolResultSuccess {
				t.Fatalf("create: %s %s", created.Status, created.Output)
			}
			send := ToolCall{Action: "send_telegram", Params: map[string]interface{}{"message": "Your report is attached.", "file_path": path}}
			_, state := PreviewActionPolicy(send, dc)
			want := "rule_required"
			if source == MessageSourceFlow {
				want = "allowed"
			}
			if state != want {
				t.Fatalf("PDF reference preview=%s, want %s", state, want)
			}
			if source != MessageSourceFlow {
				return
			}
			// Real action rules still apply to the author-selected send.
			cfg.Agent.ActionPolicy.Enabled = true
			if _, state := PreviewActionPolicy(send, dc); state != "rule_required" {
				t.Fatalf("missing rule was bypassed: %s", state)
			}
			cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "send_telegram", Operation: "execute", Target: "tool:send_telegram", RunType: "flow", ScopeID: dc.FlowID}}
			if _, state := PreviewActionPolicy(send, dc); state != "allowed" {
				t.Fatalf("scoped author rule did not match: %s", state)
			}
			cfg.Agent.ActionPolicy.Allow[0].ScopeID = "another-flow"
			if _, state := PreviewActionPolicy(send, dc); state != "rule_required" {
				t.Fatalf("another flow's rule was accepted: %s", state)
			}
			cfg.Agent.ActionPolicy.Allow[0].ScopeID = dc.FlowID
			// External reads still enter the copy gate, even for authored flow nodes.
			foreign := "This private source paragraph is long enough to require exact interactive approval before forwarding."
			readBody, err := json.Marshal(map[string]string{"status": "success", "body": foreign})
			if err != nil {
				t.Fatal(err)
			}
			dc.ExecutionHooks.HandleTool = func(context.Context, ToolCall) (string, bool) { return string(readBody), true }
			read := ToolCall{Action: "api_request", Params: map[string]interface{}{"url": "https://example.com/news", "method": "GET"}}
			if result := DispatchToolCallResult(context.Background(), &read, dc, ""); result.Status != ToolResultSuccess {
				t.Fatalf("external read: %s %s", result.Status, result.Output)
			}
			send.Params["message"] = foreign
			if _, state := PreviewActionPolicy(send, dc); state != "rule_required" {
				t.Fatalf("real copied text was allowed by a rule: %s", state)
			}
		})
	}
}
