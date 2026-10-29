package flows

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestCallToolQuarantineNotice(t *testing.T) {
	for _, tc := range []struct {
		reason security.QuarantineReason
		code   string
		want   string
	}{
		{security.QuarantineSuspicious, "FLOW_TOOL_DENIED", "suspicious content"},
		{security.QuarantineIncomplete, "FLOW_TOOL_DENIED", "incomplete or size limit exceeded"},
		{security.QuarantineUnavailable, "FLOW_TOOL_ERROR", "scanner unavailable"},
	} {
		notice := security.QuarantineNotice("source-must-not-be-echoed", "", security.ContentScanQuarantine(tc.reason))
		for name, raw := range map[string]string{
			"plain":        notice,
			"labelled":     "Tool Output: " + notice,
			"isolated":     "Tool Output: " + security.IsolateExternalData(notice),
			"JSON string":  tplJSON(t, notice),
			"error field":  tplJSON(t, map[string]any{"status": "error", "error": notice}),
			"message":      tplJSON(t, map[string]any{"status": "error", "message": notice}),
			"success text": tplJSON(t, map[string]any{"status": "success", "text": security.IsolateExternalData(notice)}),
		} {
			t.Run(string(tc.reason)+"/"+name, func(t *testing.T) {
				tools := &fakeTools{respond: toolReply(raw)}
				out, err := callTool(context.Background(), toolInput(tools), DDGSearchTool, nil)
				ne := asNodeError(err)
				if ne == nil || ne.Code != tc.code || !strings.Contains(ne.Message, tc.want) {
					t.Fatalf("got %#v, %v; want %s with %q", out, err, tc.code, tc.want)
				}
				if out["message"] != ne.Message || strings.Contains(ne.Message, "source-must-not-be-echoed") || len(ne.Message) > maxToolMessageRunes {
					t.Fatalf("quarantine diagnostic was not concise and local: %#v, %v", out, err)
				}
			})
		}
	}
}

func TestCallToolQuarantineDoesNotEchoUnknownReasons(t *testing.T) {
	tools := &fakeTools{respond: toolReply("[QUARANTINE NOTICE]\nReason: attacker-controlled reason " + strings.Repeat("x", 1000))}
	out, err := callTool(context.Background(), toolInput(tools), DDGSearchTool, nil)
	ne := asNodeError(err)
	if ne == nil || ne.Code != "FLOW_TOOL_DENIED" || strings.Contains(ne.Message, "attacker-controlled") || out["message"] != ne.Message {
		t.Fatalf("unknown quarantine reason was not safely refused: %#v, %v", out, err)
	}
	// A document mentioning quarantine, or a withheld result field inside a valid
	// search envelope, is data rather than a whole-answer security refusal.
	for _, raw := range []string{
		"The documentation describes [QUARANTINE NOTICE] messages.",
		tplJSON(t, map[string]any{"status": "success", "results": []any{map[string]any{"snippet": "[QUARANTINE NOTICE]"}}}),
	} {
		tools.setRespond(toolReply(raw))
		if _, err := callTool(context.Background(), toolInput(tools), DDGSearchTool, nil); err != nil {
			t.Fatalf("quarantine mentioned inside ordinary data became a refusal: %v", err)
		}
	}
}

func TestNewsPDFTemplateStopsAtQuarantinedSearch(t *testing.T) {
	for _, tc := range []struct {
		reason   security.QuarantineReason
		code     string
		attempts int
	}{
		{security.QuarantineSuspicious, "FLOW_TOOL_DENIED", 1},
		{security.QuarantineIncomplete, "FLOW_TOOL_DENIED", 1},
		{security.QuarantineUnavailable, "FLOW_TOOL_ERROR", 4},
	} {
		t.Run(string(tc.reason), func(t *testing.T) {
			reg := catalogRegistry(t, fullEnv())
			f := tplBuild(t, "ai_news_pdf_telegram", tplIdentity)
			search := f.NodeByKey("search")
			search.Settings.Retry = RetryPolicy{Count: 3}
			tools := &fakeTools{respond: toolReply(security.QuarantineNotice("web", "", security.ContentScanQuarantine(tc.reason)))}
			llm := &fakeLLM{}
			clock := newFakeClock(time.Date(2026, 10, 30, 7, 0, 0, 0, time.UTC))
			trigger := tplTrigger(t, reg, f)
			res, _ := runWith(context.Background(), newTestEngine(reg, &Services{Tools: tools, LLM: llm, Clock: clock}, 2), f,
				RunRequest{TriggerNode: trigger.ID, TriggerData: TriggerSample(trigger)})
			if res.Status != RunError || res.ErrorCode != tc.code || !strings.Contains(res.ErrorMessage, "quarantined") {
				t.Fatalf("quarantined search lost its reason: %+v", res)
			}
			if tools.count() != tc.attempts || tools.last(t).Tool != BraveSearchTool || llm.callCount() != 0 {
				t.Fatalf("want %d search attempts and no summary, PDF or Telegram: tools=%v, model calls=%d", tc.attempts, tools.allCalls(), llm.callCount())
			}
		})
	}
}
