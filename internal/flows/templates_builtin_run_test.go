package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"reflect"
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

// tplIsolated is how the tools wrap text of the outside (security.IsolateExternalData):
// HTML special characters escaped, between <external_data> tags.
func tplIsolated(text string) string {
	if text == "" {
		return ""
	}
	return "<external_data>\n" + html.EscapeString(text) + "\n</external_data>"
}

// tplJSON encodes a fake tool answer.
func tplJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The texts the fake tools return, wrapped and escaped by tplIsolated; the characters &, < and >
// come back as entities in the tool's answer. callTool unwraps the answer and undoes the
// escaping (ParseToolOutput), so a node, and the prompt built from it, sees the plain text.
var (
	tplSnippet1 = "Chips & fish"
	tplSnippet2 = "Use <b> sparingly"
	tplTitleA   = "Title A & more"
	tplTitleC   = "Title C"
)

// tplRunTools answers the tools the templates call, the way the real ones answer.
//
// brave_search (ExecuteBraveSearch in internal/tools/brave.go) answers with a title, a url and
// a description, title and description wrapped in <external_data>, and a "published" date
// when there is one; an empty description stays empty. The web search node maps description
// to snippet. web_scraper in rss mode (internal/tools/scraper.go) answers with items that
// have title, link, description, published and guid, the keys left out when empty, title and
// description wrapped; an item may have no title.
func tplRunTools(t *testing.T) *fakeTools {
	reply := func(out string) (ToolResponse, error) { return ToolResponse{Output: out, Status: "success"}, nil }
	return &fakeTools{respond: func(req ToolRequest) (ToolResponse, error) {
		switch req.Tool {
		case BraveSearchTool, DDGSearchTool:
			return reply(tplJSON(t, map[string]any{"status": "success", "query": "q", "result_count": 3, "results": []any{
				map[string]any{"title": tplIsolated("Go & AI"), "url": "https://a.example", "description": tplIsolated(tplSnippet1), "published": "2026-10-02"},
				map[string]any{"title": tplIsolated("Second"), "url": "https://b.example", "description": tplIsolated(tplSnippet2)},
				map[string]any{"title": tplIsolated("No description"), "url": "https://c.example", "description": ""},
			}}))
		case "document_creator":
			return reply(`{"status":"success","file_path":"/data/documents/news.pdf","filename":"news.pdf","web_path":"/files/documents/news.pdf"}`)
		case "send_telegram", "send_notification":
			return reply(`{"status":"success","results":[{"channel":"telegram","status":"sent"}]}`)
		case "send_email":
			return reply(`{"status":"success"}`)
		case "web_scraper":
			return reply(tplJSON(t, map[string]any{"status": "success", "mode": "rss", "title": tplIsolated("Feed"), "content": "md", "items": []any{
				map[string]any{"title": tplIsolated(tplTitleA), "link": "https://example.org/a", "description": tplIsolated("Desc A"),
					"published": "Sat, 03 Oct 2026 06:00:00 +0000", "guid": "a-1"},
				map[string]any{"link": "https://example.org/b", "description": tplIsolated("Desc B"),
					"published": "Sat, 03 Oct 2026 05:00:00 +0000", "guid": "b-1"},
				map[string]any{"title": tplIsolated(tplTitleC), "link": "https://example.org/c", "guid": "c-1"},
			}}))
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

// tplRun fills the blanks of a template built with tr, checks that it is publishable then,
// and runs it from its trigger with data (the trigger sample when data is nil).
func tplRun(t *testing.T, id string, tr func(string) string, fill map[string]map[string]any, data map[string]any) tplRunOutcome {
	t.Helper()
	reg := catalogRegistry(t, fullEnv())
	f := tplBuild(t, id, tr)
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
	out := tplRunOutcome{tools: tplRunTools(t), llm: &fakeLLM{responses: []LLMResponse{{Text: "SUMMARY", Model: "m", InputTokens: 10, OutputTokens: 5}}}}
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
// entries of the list end up where the template sends them. The texts are the keys, so
// they are known; the last two subtests use the texts that hold expressions.
func TestStarterTemplatesRun(t *testing.T) {
	const key = "easydrag.template."

	t.Run("ai_news_pdf_telegram", func(t *testing.T) {
		o := tplRun(t, "ai_news_pdf_telegram", tplIdentity, nil, nil)
		search := o.call(t, BraveSearchTool)
		tplWant(t, "query", search["query"], key+"ai_news_pdf_telegram.text_1")
		// The type is pinned too: the tool gets a whole number, not a float.
		if !reflect.DeepEqual(search["count"], 5) {
			t.Errorf("count = %#v, want the int 5", search["count"])
		}
		// The snippets, unwrapped and unescaped. The third result has none, which leaves an empty
		// last line that the ai.step trims with the rest of the prompt.
		tplWant(t, "prompt", o.llm.callAt(0).Prompt, key+"ai_news_pdf_telegram.text_2\n\n"+tplSnippet1+"\n"+tplSnippet2)
		pdf := o.call(t, "document_creator")
		tplWant(t, "pdf content", pdf["content"], "SUMMARY")
		tplWant(t, "pdf title", pdf["title"], key+"ai_news_pdf_telegram.text_3 03.10.2026")
		tg := o.call(t, "send_telegram")
		tplWant(t, "telegram message", tg["message"], key+"ai_news_pdf_telegram.text_4")
		tplWant(t, "telegram file", tg["file_path"], "/data/documents/news.pdf")
	})

	t.Run("webhook_summary_email", func(t *testing.T) {
		data := NormalizeTriggerData("webhook", `{"text":"Hallo"}`)
		o := tplRun(t, "webhook_summary_email", tplIdentity, map[string]map[string]any{
			"hook": {"webhook": "wh_1"}, "mail": {"to": "me@example.com"}}, data)
		tplWant(t, "prompt", o.llm.callAt(0).Prompt, key+"webhook_summary_email.text_1\n\n"+`{"text":"Hallo"}`)
		mail := o.call(t, "send_email")
		tplWant(t, "to", mail["to"], "me@example.com")
		tplWant(t, "subject", mail["subject"], key+"webhook_summary_email.text_2")
		tplWant(t, "body", mail["body"], "SUMMARY")
	})

	t.Run("appointment_reminder", func(t *testing.T) {
		o := tplRun(t, "appointment_reminder", tplIdentity, nil, nil)
		tplWant(t, "message", o.call(t, "send_telegram")["message"], key+"appointment_reminder.text_1")
	})

	t.Run("leaving_home", func(t *testing.T) {
		fill := map[string]map[string]any{"presence": {"entity": "device_tracker.phone"}, "lights": {"entity": "light.hall"}}
		o := tplRun(t, "leaving_home", tplIdentity, fill, map[string]any{"entity_id": "device_tracker.phone", "new_state": "not_home", "old_state": "home"})
		ha := o.call(t, "home_assistant")
		tplWant(t, "domain", ha["domain"], "light")
		tplWant(t, "service", ha["service"], "turn_off")
		tplWant(t, "entity", ha["entity_id"], "light.hall")

		// Somebody arrives: the condition is false and the lights stay as they are.
		o = tplRun(t, "leaving_home", tplIdentity, fill, map[string]any{"entity_id": "device_tracker.phone", "new_state": "home", "old_state": "not_home"})
		if n := o.tools.count(); n != 0 {
			t.Errorf("a home state called %d tools, want none", n)
		}
	})

	t.Run("budget_guard", func(t *testing.T) {
		o := tplRun(t, "budget_guard", tplIdentity, nil, nil)
		push := o.call(t, "send_notification")
		tplWant(t, "channel", push["channel"], "all")
		tplWant(t, "title", push["title"], key+"budget_guard.text_1")
		tplWant(t, "message", push["message"], key+"budget_guard.text_2")
	})

	// The texts of plan 1c read the trigger data. The data is what Mission Control sends
	// (NotifyPlannerAppointmentDue and NotifyBudgetEvent in internal/tools/missions_v2.go).
	// The date filter shows a date in the zone of the flow (UTC here), whatever zone the
	// string carries.
	t.Run("appointment_reminder with the shipped text", func(t *testing.T) {
		for _, tc := range []struct{ dateTime, want string }{
			{"2026-10-05T09:00:00Z", "Reminder: Dentist (05.10.2026 09:00)"},
			{"2026-10-05T09:00:00+02:00", "Reminder: Dentist (05.10.2026 07:00)"},
		} {
			data := NormalizeTriggerData("planner_appointment_due",
				`{"appointment_id":"apt-7","title":"Dentist","date_time":"`+tc.dateTime+`","time":"2026-10-05T08:00:00Z"}`)
			o := tplRun(t, "appointment_reminder", tplShipped, nil, data)
			tplWant(t, "message for "+tc.dateTime, o.call(t, "send_telegram")["message"], tc.want)
		}
	})

	// round(2) rounds half away from zero and prints no trailing zeros: 4.2367 is 4.24 and 5 is 5.
	t.Run("budget_guard with the shipped text", func(t *testing.T) {
		for _, tc := range []struct{ spent, limit, want string }{
			{"8.4", "10", "8.4 of 10 USD of the daily budget are used."},
			{"4.2367", "5.0", "4.24 of 5 USD of the daily budget are used."},
		} {
			data := NormalizeTriggerData("budget_warning",
				`{"event":"budget_warning","spent_usd":`+tc.spent+`,"limit_usd":`+tc.limit+`,"percentage":0.84,"time":"2026-10-03T07:00:00Z"}`)
			o := tplRun(t, "budget_guard", tplShipped, nil, data)
			push := o.call(t, "send_notification")
			tplWant(t, "title", push["title"], "AI budget warning")
			tplWant(t, "message for "+tc.spent+"/"+tc.limit, push["message"], tc.want)
		}
	})

	t.Run("rss_digest", func(t *testing.T) {
		o := tplRun(t, "rss_digest", tplIdentity, map[string]map[string]any{
			"feed": {"url": "https://example.org/feed"}, "mail": {"to": "me@example.com"}}, nil)
		feed := o.call(t, "web_scraper")
		tplWant(t, "url", feed["url"], "https://example.org/feed")
		tplWant(t, "mode", feed["mode"], "rss")
		// The titles, unwrapped and unescaped; the item without a title is an empty line.
		tplWant(t, "prompt", o.llm.callAt(0).Prompt, key+"rss_digest.text_1\n\n"+tplTitleA+"\n\n"+tplTitleC)
		mail := o.call(t, "send_email")
		tplWant(t, "body", mail["body"], "SUMMARY")
		tplWant(t, "subject", mail["subject"], key+"rss_digest.text_2")
	})
}
