package server

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

func TestFlowPDFCreateThenTelegramUsesGeneratedAttachment(t *testing.T) {
	cfg, _, _ := c13DocsConfig(t)
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Telegram.BotToken = "test-placeholder-not-a-real-token"
	cfg.Telegram.UserID = 123
	var generated string
	sends := 0
	inv, _, _ := c13Invoker(cfg, map[string]bool{"document_creator": true, "send_telegram": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
		dc.SessionID = "flow-" + t.Name()
		dc.ExecutionHooks = &agent.ExecutionHooks{HandleTool: func(_ context.Context, call agent.ToolCall) (string, bool) {
			if call.Action != "send_telegram" {
				return "", false // Generate the PDF with the real native tool.
			}
			path, _ := call.Params["file_path"].(string)
			if generated == "" || path != generated {
				t.Fatalf("attachment = %q, want generated path %q", path, generated)
			}
			data, err := os.ReadFile(path)
			if err != nil || !strings.HasPrefix(string(data), "%PDF-") {
				t.Fatalf("attachment is not the created PDF: %v", err)
			}
			sends++
			// Intercept delivery: this test must never contact Telegram.
			return `{"status":"success","results":[{"channel":"telegram","status":"sent"}]}`, true
		}}
		return agent.DispatchToolCallResult(ctx, tc, dc, "")
	})
	env := flows.StaticEnv{"document_creator": {}, "send_telegram": {}}
	created, err := c13Exec(t, flows.TypePDFCreate, env, inv, map[string]any{"title": "News", "content": "A short public news report."}, "run_pdf_telegram")
	if err != nil {
		t.Fatal(err)
	}
	file, _ := created.Output["file"].(map[string]any)
	generated, _ = file["path"].(string)
	if len([]rune(generated)) < 48 {
		t.Fatalf("fixture path is too short to exercise the copy gate: %q", generated)
	}
	sent, err := c13Exec(t, flows.TypeTelegram, env, inv, map[string]any{"message": "Your news PDF is attached.", "file": file}, "run_pdf_telegram")
	if err != nil || sent.Output["sent"] != true || sends != 1 {
		t.Fatalf("PDF to Telegram = %+v, error=%v, deliveries=%d", sent, err, sends)
	}
}

func TestFlowCopyRefusalDoesNotRecommendAnAllowRule(t *testing.T) {
	for _, copied := range []bool{false, true} {
		name := "missing_rule"
		if copied {
			name = "copied_text"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Telegram.BotToken = "test-placeholder-not-a-real-token"
			cfg.Telegram.UserID = 123
			cfg.Agent.ActionPolicy.Enabled = true
			if copied {
				cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "send_telegram", Operation: "execute", Target: "tool:send_telegram", RunType: "flow", ScopeID: t.Name()}}
			}
			message := "The source paragraph contains a detailed release announcement and several verified public facts."
			if copied {
				agent.RecordExternalIngress("flow-"+t.Name(), security.ExternalEmail, message)
			}
			inv, _, _ := c13Invoker(cfg, map[string]bool{"send_telegram": true}, func(ctx context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
				dc.ExecutionHooks = &agent.ExecutionHooks{HandleTool: func(context.Context, agent.ToolCall) (string, bool) {
					t.Fatal("denied send reached the delivery handler")
					return "", true
				}}
				return agent.DispatchToolCallResult(ctx, tc, dc, "")
			})
			req := c13Request("send_telegram", map[string]any{"message": message})
			req.FlowID = t.Name()
			_, err := inv.InvokeTool(context.Background(), req)
			var ne *flows.NodeError
			if !errors.As(err, &ne) {
				t.Fatalf("expected a nonretryable node refusal, got %v", err)
			}
			want := "FLOW_ACTION_RULE_REQUIRED"
			if copied {
				want = "FLOW_TOOL_DENIED"
				if !strings.Contains(ne.Message, "exact call") || !strings.Contains(ne.Message, "does not exempt") || strings.Contains(ne.Message, message) {
					t.Fatalf("copy refusal lost its safe explanation: %s", ne.Message)
				}
			}
			if ne.Code != want {
				t.Fatalf("refusal code = %s, want %s", ne.Code, want)
			}
		})
	}
}
