package flows

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Runs of the starter templates against fake tools and a fake model, and the size of the
// prompts they build. The helpers start with tplRun.

// A template feeds untrusted text into an ai.step prompt, which the node caps at 256 KiB
// (bytes). The truncate filter counts runes, so the bound holds for four-byte characters
// too, with room for a long translated instruction, and the node accepts the prompt.
func TestTemplatePromptsAreBounded(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	aiDef := lookupDef(t, reg, TypeAIStep)
	// 3000 characters of three bytes: a long instruction in a language that needs them.
	instruction := strings.Repeat("世", 3000)
	tr := func(key string) string {
		if strings.Contains(key, ".text_") {
			return instruction
		}
		return key
	}
	huge := strings.Repeat("😀", 300000) // 1.2 MB, four bytes per character
	feedItems := make([]any, 5000)
	for i := range feedItems {
		feedItems[i] = map[string]any{"title": strings.Repeat("😀", 100)}
	}
	for _, tc := range []struct {
		template string
		roots    map[string]any
		short    map[string]any
		wantTail string
	}{
		{"webhook_summary_email",
			map[string]any{"trigger": map[string]any{"data": map[string]any{"raw": huge}}},
			map[string]any{"trigger": map[string]any{"data": map[string]any{"raw": `{"text":"Hallo"}`}}},
			`{"text":"Hallo"}`},
		{"rss_digest",
			map[string]any{"feed": map[string]any{"items": feedItems}},
			map[string]any{"feed": map[string]any{"items": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}}},
			"A\nB"},
	} {
		f := tplBuild(t, tc.template, tr)
		prompt := f.NodeByKey("summary").Params["prompt"]
		resolve := func(roots map[string]any) string {
			params, err := ResolveParams(map[string]any{"prompt": prompt}, &Env{Roots: roots, Location: time.UTC})
			if err != nil {
				t.Fatalf("%s: %v", tc.template, err)
			}
			return params["prompt"].(string)
		}

		// Without the bound the prompt would not fit: this is what the template avoids.
		var unbounded string
		switch tc.template {
		case "webhook_summary_email":
			unbounded = instruction + "\n\n" + huge
		default:
			unbounded = instruction + "\n\n" + strings.Repeat(strings.Repeat("😀", 100)+"\n", len(feedItems))
		}
		if len(unbounded) <= maxAIPromptBytes {
			t.Fatalf("%s: setup: the unbounded prompt is only %d bytes", tc.template, len(unbounded))
		}

		got := resolve(tc.roots)
		if limit := maxAIPromptBytes * 3 / 4; len(got) > limit {
			t.Errorf("%s: the prompt is %d bytes; it should stay within %d of the %d the node allows", tc.template, len(got), limit, maxAIPromptBytes)
		}
		if !strings.HasPrefix(got, instruction+"\n\n") || !strings.HasSuffix(got, "…") {
			t.Errorf("%s: the prompt must keep the instruction and mark the cut, got %.20q … %.12q", tc.template, got, got[len(got)-12:])
		}
		llm := &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
		if _, err := execDef(aiDef, map[string]any{"prompt": got}, &Services{LLM: llm}); err != nil {
			t.Errorf("%s: the ai.step refuses the prompt: %v", tc.template, err)
		}

		// What fits is passed on whole.
		if got := resolve(tc.short); got != instruction+"\n\n"+tc.wantTail {
			t.Errorf("%s: a short value changed: %.60q", tc.template, got[len(instruction):])
		}
		// A missing value (an empty webhook body has no raw) leaves the instruction alone.
		if got := resolve(map[string]any{}); got != instruction+"\n\n" {
			t.Errorf("%s: with nothing to read the prompt is %.60q", tc.template, got[len(instruction):])
		}
	}
}

// tplRunTools answers the tools the templates call, the way the real ones answer.
func tplRunTools() *fakeTools {
	reply := func(out string) (ToolResponse, error) { return ToolResponse{Output: out, Status: "success"}, nil }
	return &fakeTools{respond: func(req ToolRequest) (ToolResponse, error) {
		switch req.Tool {
		case BraveSearchTool, DDGSearchTool:
			return reply(`{"status":"success","results":[{"title":"T1","url":"https://a.example","snippet":"S1 snippet"},` +
				`{"title":"T2","url":"https://b.example","snippet":"S2 snippet"}]}`)
		case "document_creator":
			return reply(`{"status":"success","file_path":"/data/documents/news.pdf","filename":"news.pdf","web_path":"/files/documents/news.pdf"}`)
		case "send_telegram", "send_notification":
			return reply(`{"status":"success","results":[{"channel":"telegram","status":"sent"}]}`)
		case "send_email":
			return reply(`{"status":"success"}`)
		case "web_scraper":
			return reply(`{"status":"success","mode":"rss","title":"Feed","content":"md","items":[{"title":"Title A"},{"title":"Title B"}]}`)
		case "home_assistant":
			return reply(`{"status":"success","affected_entities":["light.hall"]}`)
		}
		return ToolResponse{}, fmt.Errorf("a template called the unexpected tool %q", req.Tool)
	}}
}

type tplRunOutcome struct {
	res   RunResult
	tools *fakeTools
	llm   *fakeLLM
}

// call returns the arguments of the only call of tool, and fails when there is not exactly one.
func (o tplRunOutcome) call(t *testing.T, tool string) map[string]any {
	t.Helper()
	var found []ToolRequest
	for _, c := range o.tools.allCalls() {
		if c.Tool == tool {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s was called %d times, want once (calls: %v)", tool, len(found), o.tools.allCalls())
	}
	return found[0].Args
}

// tplRun fills the blanks of a template, checks that it is publishable then, and runs it
// from its trigger with data. The translator returns the keys, so the texts are known.
func tplRun(t *testing.T, id string, fill map[string]map[string]any, data map[string]any) tplRunOutcome {
	t.Helper()
	reg := catalogRegistry(t, fullEnv())
	f := tplBuild(t, id, tplIdentity)
	for key, params := range fill {
		n := f.NodeByKey(key)
		if n == nil {
			t.Fatalf("%s: no node %q", id, key)
		}
		for name, value := range params {
			n.Params[name] = value
		}
	}
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	vc := ValidateContext{Mode: ModePublish, Now: clock.Now(), Location: time.UTC}
	if issues := Validate(f, reg, vc); len(issues) != 0 {
		t.Fatalf("%s: not publishable once filled in: %+v", id, issues)
	}
	if issues := LintUntrustedData(f, reg); len(issues) != 0 {
		t.Fatalf("%s: lint warnings once filled in: %+v", id, issues)
	}
	out := tplRunOutcome{tools: tplRunTools(), llm: &fakeLLM{responses: []LLMResponse{{Text: "SUMMARY", Model: "m", InputTokens: 10, OutputTokens: 5}}}}
	trigger := tplTrigger(t, reg, f)
	if data == nil {
		data = TriggerSample(trigger)
	}
	eng := newTestEngine(reg, &Services{Tools: out.tools, LLM: out.llm, Clock: clock, Location: time.UTC}, 2)
	out.res, _ = runWith(context.Background(), eng, f, RunRequest{
		TriggerNode: trigger.ID, TriggerType: strings.TrimPrefix(trigger.Type, "trigger."), TriggerData: data})
	if out.res.Status != RunSuccess {
		t.Fatalf("%s: run %s: %s %s", id, out.res.Status, out.res.ErrorCode, out.res.ErrorMessage)
	}
	return out
}

func tplWant(t *testing.T, what string, got, want any) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s = %#v, want %#v", what, got, want)
	}
}

// Every starter template runs from its trigger once its blanks are filled in: the values
// of the plan pass the strict readers of the nodes, every reference reads data the node
// before it really produced, and the text of the model, the file of the PDF and the
// entries of the list end up where the template sends them.
func TestStarterTemplatesRun(t *testing.T) {
	const key = "easydrag.template."

	t.Run("ai_news_pdf_telegram", func(t *testing.T) {
		o := tplRun(t, "ai_news_pdf_telegram", nil, nil)
		search := o.call(t, BraveSearchTool)
		tplWant(t, "query", search["query"], key+"ai_news_pdf_telegram.text_1")
		tplWant(t, "count", search["count"], 5)
		tplWant(t, "prompt", o.llm.callAt(0).Prompt, key+"ai_news_pdf_telegram.text_2\n\nS1 snippet\nS2 snippet")
		pdf := o.call(t, "document_creator")
		tplWant(t, "pdf content", pdf["content"], "SUMMARY")
		tplWant(t, "pdf title", pdf["title"], key+"ai_news_pdf_telegram.text_3 03.10.2026")
		tg := o.call(t, "send_telegram")
		tplWant(t, "telegram message", tg["message"], key+"ai_news_pdf_telegram.text_4")
		tplWant(t, "telegram file", tg["file_path"], "/data/documents/news.pdf")
	})

	t.Run("webhook_summary_email", func(t *testing.T) {
		data := NormalizeTriggerData("webhook", `{"text":"Hallo"}`)
		o := tplRun(t, "webhook_summary_email", map[string]map[string]any{
			"hook": {"webhook": "wh_1"}, "mail": {"to": "me@example.com"}}, data)
		tplWant(t, "prompt", o.llm.callAt(0).Prompt, key+"webhook_summary_email.text_1\n\n"+`{"text":"Hallo"}`)
		mail := o.call(t, "send_email")
		tplWant(t, "to", mail["to"], "me@example.com")
		tplWant(t, "subject", mail["subject"], key+"webhook_summary_email.text_2")
		tplWant(t, "body", mail["body"], "SUMMARY")
	})

	t.Run("appointment_reminder", func(t *testing.T) {
		o := tplRun(t, "appointment_reminder", nil, nil)
		tplWant(t, "message", o.call(t, "send_telegram")["message"], key+"appointment_reminder.text_1")
	})

	t.Run("leaving_home", func(t *testing.T) {
		fill := map[string]map[string]any{"presence": {"entity": "device_tracker.phone"}, "lights": {"entity": "light.hall"}}
		o := tplRun(t, "leaving_home", fill, map[string]any{"entity_id": "device_tracker.phone", "new_state": "not_home", "old_state": "home"})
		ha := o.call(t, "home_assistant")
		tplWant(t, "domain", ha["domain"], "light")
		tplWant(t, "service", ha["service"], "turn_off")
		tplWant(t, "entity", ha["entity_id"], "light.hall")

		// Somebody arrives: the condition is false and the lights stay as they are.
		o = tplRun(t, "leaving_home", fill, map[string]any{"entity_id": "device_tracker.phone", "new_state": "home", "old_state": "not_home"})
		if n := o.tools.count(); n != 0 {
			t.Errorf("a home state called %d tools, want none", n)
		}
	})

	t.Run("budget_guard", func(t *testing.T) {
		o := tplRun(t, "budget_guard", nil, nil)
		push := o.call(t, "send_notification")
		tplWant(t, "channel", push["channel"], "all")
		tplWant(t, "title", push["title"], key+"budget_guard.text_1")
		tplWant(t, "message", push["message"], key+"budget_guard.text_2")
	})

	t.Run("rss_digest", func(t *testing.T) {
		o := tplRun(t, "rss_digest", map[string]map[string]any{
			"feed": {"url": "https://example.org/feed"}, "mail": {"to": "me@example.com"}}, nil)
		feed := o.call(t, "web_scraper")
		tplWant(t, "url", feed["url"], "https://example.org/feed")
		tplWant(t, "mode", feed["mode"], "rss")
		tplWant(t, "prompt", o.llm.callAt(0).Prompt, key+"rss_digest.text_1\n\nTitle A\nTitle B")
		mail := o.call(t, "send_email")
		tplWant(t, "body", mail["body"], "SUMMARY")
		tplWant(t, "subject", mail["subject"], key+"rss_digest.text_2")
	})
}
