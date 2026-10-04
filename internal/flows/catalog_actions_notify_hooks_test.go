package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func notifyEntry(channel, status, detail string) map[string]any {
	m := map[string]any{"channel": channel, "status": status}
	if detail != "" {
		m["detail"] = detail
	}
	return m
}

func notifyFailureOf(results ...any) error {
	_, err := notificationResults(map[string]any{"status": "success", "results": results})
	return err
}

func notifyResultsError(out map[string]any) error {
	_, err := notificationResults(out)
	return err
}

const (
	notifyBotToken = "bot123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw"
	notifyTopicURL = `https://ntfy.sh/secret-topic-1234`
)

// A send_notification answer lists every channel. One that sent is enough; if none
// did, the node fails with what the channels said, cleaned and bounded: text from a
// tool or a remote server must not forge lines, carry a credential or flood a record.
func TestNotificationFailureMessages(t *testing.T) {
	wantFailure := func(label string, err error, wantMessage string) {
		t.Helper()
		ne := asNodeError(err)
		if ne == nil || ne.Code != "FLOW_NOTIFY_FAILED" || (wantMessage != "" && ne.Message != wantMessage) {
			t.Errorf("%s: %v", label, err)
			return
		}
		if strings.ContainsAny(ne.Message, notifyLF+notifyCR+notifyNUL+"\x1b\t") || !utf8.ValidString(ne.Message) || utf8.RuneCountInString(ne.Message) > maxToolMessageRunes+1 {
			t.Errorf("%s: unclean message %.100q", label, ne.Message)
		}
	}
	wantFailure("one failure", notifyFailureOf(notifyEntry("telegram", "error", "chat not found")), "chat not found")
	wantFailure("two failures", notifyFailureOf(notifyEntry("ntfy", "error", "ntfy down"), notifyEntry("push", "error", "push off")), "ntfy down; push off")
	wantFailure("no detail", notifyFailureOf(notifyEntry("ntfy", "error", "")), "ntfy")
	wantFailure("only a status", notifyFailureOf(map[string]any{"status": "error"}), "error")
	wantFailure("an empty entry", notifyFailureOf(map[string]any{}), "a channel failed")
	wantFailure("a status that is not sent", notifyFailureOf(notifyEntry("ntfy", "queued", "")), "ntfy")
	wantFailure("entries that are no objects", notifyFailureOf("x", 5.0, nil), "a channel failed; a channel failed; a channel failed")
	wantFailure("a list that is not one", notifyResultsError(map[string]any{"results": "ok"}), "")
	wantFailure("an object instead of a list", notifyResultsError(map[string]any{"results": map[string]any{"status": "sent"}}), "")

	// Partial success and the lenient cases.
	for name, results := range map[string][]any{
		"one of two":      {notifyEntry("ntfy", "error", "down"), notifyEntry("push", "sent", "")},
		"upper case":      {notifyEntry("ntfy", "SENT", "")},
		"padded":          {notifyEntry("ntfy", " sent ", "")},
		"sent after fail": {notifyEntry("a", "error", "x"), notifyEntry("b", "error", "y"), notifyEntry("c", "sent", "")},
	} {
		if err := notifyFailureOf(results...); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// An answer that lists no channel confirms nothing: FLOW_TOOL_ERROR, not a send.
	for name, out := range map[string]map[string]any{
		"absent": {"status": "success"}, "null": {"results": nil}, "empty list": {"results": []any{}}, "nil list": {"results": []any(nil)},
	} {
		if ne := asNodeError(notifyResultsError(out)); ne == nil || ne.Code != "FLOW_TOOL_ERROR" || ne.Message != "the notification tool listed no channel" {
			t.Errorf("%s: %v", name, ne)
		}
	}

	// Cleaning.
	wantFailure("control characters", notifyFailureOf(notifyEntry("t", "error", "line1"+notifyLF+"line2\x1b[31m"+notifyCRLF+"\tx"+notifyNUL)), "line1 line2 [31m x")
	wantFailure("a hostile channel and status", notifyFailureOf(map[string]any{"channel": "a" + notifyLF + "b", "status": "ERROR" + notifyCR}), "a b")
	long := notifyFailureOf(notifyEntry("t", "error", strings.Repeat("x", 10000)))
	wantFailure("a huge detail", long, "")
	if n := utf8.RuneCountInString(asNodeError(long).Message); n != maxNotifyDetailText+1 {
		t.Errorf("a huge detail is %d runes", n)
	}
	wantFailure("invalid utf-8", notifyFailureOf(notifyEntry("t", "error", "a\xffb")), "a"+replacementRune+"b")
	var many []any
	for i := 0; i < 30; i++ {
		many = append(many, notifyEntry("c", "error", strings.Repeat("d", 100)))
	}
	wantFailure("many failures", notifyFailureOf(many...), "")

	// A failed HTTP call carries its URL, and the URL a Telegram bot token or a ntfy topic.
	wantFailure("an HTTP error", notifyFailureOf(notifyEntry("ntfy", "error", `ntfy request failed: Post "`+notifyTopicURL+`": dial tcp: lookup ntfy.sh: no such host`)),
		`ntfy request failed: Post "[url]": dial tcp: lookup ntfy.sh: no such host`)
	for name, detail := range map[string]string{
		"a URL that is cut off": `telegram request failed: Post "https://api.telegram.org/` + notifyBotToken + `/sendMes`,
		"a bare token":          `bad url https://api.telegram.org/` + notifyBotToken + `/sendMessage`,
		"a Get":                 `Get "https://api.telegram.org/` + notifyBotToken + `/getMe": EOF`,
	} {
		err := notifyFailureOf(notifyEntry("telegram", "error", detail))
		wantFailure(name, err, "")
		if msg := asNodeError(err).Message; strings.Contains(msg, "AAHdq") || strings.Contains(msg, "123456789") {
			t.Errorf("%s: the token is in %q", name, msg)
		}
	}
}

// The push node returns the channels' entries; what it returns is cleaned the same way.
func TestPushResultsAreCleaned(t *testing.T) {
	var entries []any
	entries = append(entries, notifyEntry("ntfy"+notifyLF, "SENT", ""), notifyEntry("push", "error", `Post "`+notifyTopicURL+`": `+strings.Repeat("e", 500)), "junk", notifyEntry("c", "error", "a"+notifyCRLF+"b"))
	for i := 0; i < 30; i++ {
		entries = append(entries, notifyEntry(fmt.Sprintf("c%d", i), "error", "x"))
	}
	raw, err := json.Marshal(map[string]any{"status": "success", "results": entries})
	if err != nil {
		t.Fatal(err)
	}
	tools := &fakeTools{respond: toolReply("Tool Output: " + string(raw))}
	res, err := execDef(lookupDef(t, notifyRegistry(t), TypePush), map[string]any{"message": "m"}, &Services{Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	results, _ := res.Output["results"].([]any)
	if len(results) != maxNotifyResults {
		t.Fatalf("%d results, want %d", len(results), maxNotifyResults)
	}
	first, _ := results[0].(map[string]any)
	if !reflect.DeepEqual(first, map[string]any{"channel": "ntfy", "status": "sent"}) {
		t.Errorf("first = %#v", first)
	}
	second, _ := results[1].(map[string]any)
	if detail, _ := second["detail"].(string); strings.Contains(detail, "secret-topic") || !strings.Contains(detail, `Post "[url]"`) ||
		utf8.RuneCountInString(detail) > maxNotifyDetailText+1 {
		t.Errorf("second detail = %q", detail)
	}
	if third, _ := results[2].(map[string]any); !reflect.DeepEqual(third, map[string]any{"channel": "", "status": ""}) {
		t.Errorf("an entry that is no object = %#v", third)
	}
	if fourth, _ := results[3].(map[string]any); fourth["detail"] != "a b" {
		t.Errorf("fourth = %#v", fourth)
	}
	if _, err := json.Marshal(res.Output); err != nil {
		t.Error(err)
	}
}

// What the tool does not call a success is not one: a plain-text refusal, an empty
// answer, another status. Denials and setup problems keep their final codes.
func TestNotifyFailsClosed(t *testing.T) {
	params := map[string]map[string]any{
		TypeTelegram: {"message": "m"}, TypeEmail: {"to": "a@b.de", "body": "b"}, TypePush: {"message": "m"}, TypeDiscord: {"message": "m"},
	}
	hugeRefusal := "[PERMISSION DENIED] " + strings.Repeat("no ", 5000)
	refusals := map[string]ToolResponse{
		"plain text refusal": {Output: "[PERMISSION DENIED] notifications are disabled", Status: "success"},
		"huge refusal":       {Output: hugeRefusal, Status: "success"},
		"empty output":       {Output: "", Status: "success"},
		"empty object":       {Output: "Tool Output: {}", Status: "success"},
		"another status":     {Output: `Tool Output: {"status":"queued"}`, Status: "success"},
		"status ok":          {Output: `{"status":"ok","message":"sent"}`, Status: "success"},
		"status as number":   {Output: `{"status":1}`, Status: "success"},
		"error status":       {Output: `{"status":"error","message":"boom"}`, Status: "success"},
		"flagged as error":   {Output: `{"status":"success"}`, Status: "success", IsError: true},
		"a json list":        {Output: `["sent"]`, Status: "success"},
	}
	for typ, p := range params {
		def := lookupDef(t, notifyRegistry(t), typ)
		for name, resp := range refusals {
			tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return resp, nil }}
			res, err := execDef(def, p, &Services{Tools: tools})
			ne := asNodeError(err)
			if ne == nil || ne.Code != "FLOW_TOOL_ERROR" || res.Output != nil || len(ne.Message) > maxEchoMessageBytes || strings.ContainsAny(ne.Message, notifyLF+notifyCR) {
				t.Errorf("%s %s: %v, output %v", typ, name, err, res.Output)
			}
		}
		for status, code := range map[string]string{"denied": "FLOW_TOOL_DENIED", "policy_denied": "FLOW_TOOL_DENIED", "needs_setup": "FLOW_NODE_UNAVAILABLE"} {
			tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
				return ToolResponse{Output: "[PERMISSION DENIED] off", Status: status}, nil
			}}
			if _, err := execDef(def, p, &Services{Tools: tools}); asNodeError(err).Code != code {
				t.Errorf("%s %s: %v, want %s", typ, status, err, code)
			}
		}
		// A tool that answers with a status in other letter case or with blanks is a success.
		for _, status := range []string{"SUCCESS", " success ", "Success"} {
			reply := notifyReplies[def.Tool]
			reply = strings.Replace(reply, `"status":"success"`, `"status":"`+status+`"`, 1)
			tools := &fakeTools{respond: toolReply(reply)}
			if res, err := execDef(def, p, &Services{Tools: tools}); err != nil || res.Output["sent"] != true {
				t.Errorf("%s status %q: %v", typ, status, err)
			}
		}
		// An invoker that fails, and a run that ended before the call.
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
			return ToolResponse{}, errors.New(strings.Repeat("down ", 500))
		}}
		if _, err := execDef(def, p, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_NODE_FAILED" || len(asNodeError(err).Message) > maxEchoMessageBytes {
			t.Errorf("%s: invoker error: %v", typ, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tools = &fakeTools{respond: notifyAnswers}
		if _, err := execDefCtx(ctx, def, withDefaults(def, p), &Services{Tools: tools}); err == nil || tools.count() != 0 {
			t.Errorf("%s: a cancelled run sent: %v after %d calls", typ, err, tools.count())
		}
		if _, err := execDef(def, p, &Services{}); asNodeError(err).Code != "FLOW_TOOLS_UNAVAILABLE" {
			t.Errorf("%s: no tools: %v", typ, err)
		}
	}
}

// Validate reports what Execute would reject, for literal parameters only: a template
// is judged at run time, an absent parameter is the required check's business.
func TestNotifyValidate(t *testing.T) {
	long := func(n int) string { return strings.Repeat("m", n) }
	cases := []struct {
		name   string
		typ    string
		params map[string]any
		want   []string
	}{
		{"email fine", TypeEmail, map[string]any{"to": "a@b.de, c@d.de", "subject": "S", "body": "b", "attachment": "a.pdf", "account": "work"}, nil},
		{"email bare", TypeEmail, map[string]any{}, nil},
		{"email templates", TypeEmail, map[string]any{"to": "{{trigger.data.to}}", "subject": "Re: {{trigger.data.s}}", "account": "{{trigger.data.a}}", "attachment": "{{trigger.data.f}}", "body": "{{trigger.data.b}}"}, nil},
		{"email bad address", TypeEmail, map[string]any{"to": "Anna <a@b.de>"}, []string{"to"}},
		{"email header injection", TypeEmail, map[string]any{"to": "a@b.de", "subject": "S" + notifyCRLF + "Bcc: evil@x.de", "account": "w" + notifyLF + "x"}, []string{"subject", "account"}},
		{"email recipient list too long", TypeEmail, map[string]any{"to": strings.Repeat("a@b.de,", 21) + "a@b.de"}, []string{"to"}},
		{"email account with a quote", TypeEmail, map[string]any{"account": `a"b`}, []string{"account"}},
		{"email attachment", TypeEmail, map[string]any{"attachment": "a" + notifyNUL}, []string{"attachment"}},
		{"email body too large", TypeEmail, map[string]any{"body": long(maxEmailBodyBytes + 1)}, []string{"body"}},
		{"email wrong types", TypeEmail, map[string]any{"to": []any{"a@b.de"}, "subject": map[string]any{"a": "b"}, "account": []any{"w"}}, []string{"to", "subject", "account"}},
		{"email empty wrong types", TypeEmail, map[string]any{"subject": map[string]any{}, "account": []any{}}, nil},
		{"telegram fine", TypeTelegram, map[string]any{"message": "m", "title": "t", "file": "a.pdf"}, nil},
		{"telegram too long", TypeTelegram, map[string]any{"message": long(telegramTextLimit)}, []string{"message"}},
		{"telegram title", TypeTelegram, map[string]any{"message": "m", "title": "a" + notifyLF + "b"}, []string{"title"}},
		{"telegram file", TypeTelegram, map[string]any{"file": map[string]any{"name": "x"}}, []string{"file"}},
		{"telegram templates", TypeTelegram, map[string]any{"message": "{{trigger.data.m}}", "title": "{{trigger.data.t}}", "file": "{{trigger.data.f}}"}, nil},
		{"push fine", TypePush, map[string]any{"message": "m", "channel": "ntfy", "priority": "high"}, nil},
		{"push bad", TypePush, map[string]any{"message": long(maxPushMessageBytes + 1), "channel": "a b", "priority": "urgent", "title": "a" + notifyLF}, []string{"channel", "message", "priority"}},
		{"push templates", TypePush, map[string]any{"message": "{{x.y}}", "channel": "{{x.c}}", "priority": "{{x.p}}"}, nil},
		{"discord fine", TypeDiscord, map[string]any{"message": "m", "channel_id": "123"}, nil},
		{"discord bad", TypeDiscord, map[string]any{"message": long(maxDiscordMessageRunes + 1), "channel_id": "12a"}, []string{"message", "channel_id"}},
		{"discord number id", TypeDiscord, map[string]any{"message": "m", "channel_id": 123.0}, []string{"channel_id"}},
	}
	reg := notifyRegistry(t)
	for _, c := range cases {
		def := lookupDef(t, reg, c.typ)
		node := &Node{ID: testNodeID(1), Key: "n", Type: c.typ, Params: c.params}
		var got []string
		declared := map[string]bool{}
		for _, p := range def.Params {
			declared[p.Name] = true
		}
		for _, is := range def.Validate(node, ValidateContext{Mode: ModePublish}) {
			got = append(got, is.Param)
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != node.ID || !declared[is.Param] ||
				len(is.Message) > maxEchoMessageBytes || strings.ContainsAny(is.Message, notifyLF+notifyCR) || strings.Contains(is.Message, "evil") {
				t.Errorf("%s: bad issue %+v", c.name, is)
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: issues on %v, want %v", c.name, got, c.want)
		}
		if len(c.want) > 0 {
			// Whatever Validate rejects, Execute rejects before any tool call.
			params := map[string]any{"message": "m", "to": "a@b.de", "body": "b"}
			for k, v := range c.params {
				params[k] = v
			}
			tools := &fakeTools{respond: notifyAnswers}
			if _, err := execDef(def, params, &Services{Tools: tools}); asNodeError(err).Code != "FLOW_PARAM_INVALID" || tools.count() != 0 {
				t.Errorf("%s: Execute: %v after %d calls", c.name, err, tools.count())
			}
		}
	}
	for _, typ := range []string{TypeTelegram, TypeEmail, TypePush, TypeDiscord} {
		if issues := lookupDef(t, reg, typ).Validate(nil, ValidateContext{}); issues != nil {
			t.Errorf("%s: nil node: %+v", typ, issues)
		}
	}
}

// notifyCheckArgs describes what is wrong with the arguments a notification tool got,
// or "". Whatever the parameters were, the tool only sees bounded, well formed values.
func notifyCheckArgs(c ToolRequest) string {
	text := func(key string) (string, bool) { s, ok := c.Args[key].(string); return s, ok || c.Args[key] == nil }
	str := func(key string) string { s, _ := text(key); return s }
	for key := range c.Args {
		if _, ok := text(key); !ok && key != "attachments" {
			return "argument " + key + " is not text"
		}
	}
	if p := str("file_path"); len(p) > maxFilePathBytes || strings.Contains(p, notifyNUL) || !utf8.ValidString(p) {
		return "bad file_path"
	}
	switch c.Tool {
	case "send_email":
		to := str("to")
		if !validHeaderValue(to) || !validHeaderValue(str("subject")) || !validHeaderValue(str("account")) ||
			len(str("subject")) > maxEmailSubjectBytes || len(str("body")) > maxEmailBodyBytes || len(str("account")) > maxAccountBytes ||
			strings.ContainsAny(str("account"), `"\`) {
			return "a malformed header field reached send_email"
		}
		parts := strings.Split(to, ", ")
		if len(parts) == 0 || len(parts) > maxEmailRecipients {
			return fmt.Sprintf("%d recipients", len(parts))
		}
		for _, part := range parts {
			if a, err := mail.ParseAddress(part); err != nil || a.Name != "" || a.Address != part {
				return "a recipient is not a plain address"
			}
		}
		if raw := c.Args["attachments"]; raw != nil {
			list, ok := raw.([]any)
			if !ok || len(list) != 1 {
				return "bad attachments"
			}
			if p, _ := list[0].(string); len(p) == 0 || len(p) > maxFilePathBytes || strings.Contains(p, notifyNUL) || !utf8.ValidString(p) {
				return "bad attachments"
			}
		}
	case "send_telegram":
		if title := str("title"); utf8.RuneCountInString(title) > maxNotifyTitleRunes || !validHeaderValue(title) || telegramUnits(title, str("message")) > telegramTextLimit {
			return "a telegram text that does not fit"
		}
	case "send_notification":
		ch := str("channel")
		if _, err := pushChannel(ch); err != nil || ch == "" || len(str("message")) > maxPushMessageBytes || !validHeaderValue(str("title")) {
			return "a malformed notification"
		}
		if _, ok := choiceParam(str("priority"), "", notifyPriorities...); !ok || str("priority") == "" {
			return "a bad priority"
		}
	case "send_discord":
		if id := str("channel_id"); len(id) > maxDiscordIDDigits || strings.Trim(id, "0123456789") != "" || utf8.RuneCountInString(str("message")) > maxDiscordMessageRunes {
			return "a malformed discord call"
		}
	}
	if strings.TrimSpace(str("message")) == "" && c.Tool != "send_email" {
		return "an empty message"
	}
	return ""
}

// Validate and Execute see raw parameters of any type. No hook may panic or produce
// unbounded text, everything Validate rejects Execute must reject too, a parameter
// problem is found before the tool is called, and what reaches a tool is bounded and
// well formed.
func TestNotifyHooksSurviveOddParams(t *testing.T) {
	reg := notifyRegistry(t)
	vc := ValidateContext{Mode: ModePublish}
	bases := map[string]map[string]any{
		TypeTelegram: {"message": "m"}, TypeEmail: {"to": "a@b.de", "body": "b"}, TypePush: {"message": "m"}, TypeDiscord: {"message": "m"},
	}
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	clean := func(msg string) bool {
		return len(msg) <= maxEchoMessageBytes && utf8.ValidString(msg) && !strings.ContainsAny(msg, notifyLF+notifyCR+notifyNUL)
	}
	runs := 0

	check := func(def *NodeDef, params map[string]any, label string) {
		runs++
		node := &Node{ID: testNodeID(1), Key: "notify", Type: def.Type, Params: params}
		tools := &fakeTools{respond: notifyAnswers}
		var (
			issues  []Issue
			res     ExecResult
			execErr error
		)
		for _, step := range []struct {
			name string
			fn   func()
		}{
			{"Validate", func() { issues = def.Validate(node, vc) }},
			{"Availability", func() { _ = def.Availability() }},
			{"FieldsOf", func() { _ = def.FieldsOf(node) }},
			{"OutputPorts", func() { _ = def.OutputPorts(node) }},
			{"EffectsOf", func() { _ = def.EffectsOf(node) }},
			{"Execute", func() { res, execErr = execDef(def, params, &Services{Tools: tools}) }},
		} {
			if p := catchPanic(step.fn); p != nil {
				fail("%s: %s panicked: %v", label, step.name, p)
				return
			}
		}
		declared := map[string]bool{}
		for _, p := range def.Params {
			declared[p.Name] = true
		}
		if len(issues) > len(def.Params) {
			fail("%s: %d issues", label, len(issues))
		}
		for _, is := range issues {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != node.ID || !declared[is.Param] || !clean(is.Message) {
				fail("%s: bad issue %.200q", label, fmt.Sprintf("%+v", is))
			}
			if execErr == nil {
				fail("%s: Validate rejected %s but Execute succeeded", label, is.Param)
			}
		}
		if execErr != nil {
			ne := asNodeError(execErr)
			if ne == nil || ne.Code == "" || !clean(ne.Message) {
				fail("%s: bad Execute error %T %.200q", label, execErr, execErr.Error())
				return
			}
			if ne.Code == "FLOW_PARAM_INVALID" && tools.count() != 0 {
				fail("%s: rejected the parameters after %d tool calls", label, tools.count())
			}
		} else if raw, err := json.Marshal(res.Output); err != nil || !utf8.Valid(raw) || res.Output["sent"] != true || tools.count() != 1 {
			fail("%s: bad success: %v %.100s after %d calls", label, err, raw, tools.count())
		}
		for _, c := range tools.allCalls() {
			if _, err := json.Marshal(c.Args); err != nil || len(c.AllowedTools) != 1 || c.AllowedTools[0] != c.Tool || c.Tool != def.Tool {
				fail("%s: malformed tool request %v: %+v", label, err, c.Tool)
			}
			if problem := notifyCheckArgs(c); problem != "" {
				fail("%s: %s", label, problem)
			}
		}
	}

	values := oddParamValues()
	for _, def := range reg.All() {
		base := bases[def.Type]
		names := []string{"unknown_param"}
		for _, p := range def.Params {
			names = append(names, p.Name)
		}
		for _, ov := range values {
			for _, name := range names {
				params := make(map[string]any, len(base)+1)
				for k, v := range base {
					params[k] = v
				}
				params[name] = ov.v
				check(def, params, fmt.Sprintf("%s %s=%s", def.Type, name, ov.name))
			}
			all := make(map[string]any, len(names))
			for _, name := range names {
				all[name] = ov.v
			}
			check(def, all, fmt.Sprintf("%s all=%s", def.Type, ov.name))
		}
		check(def, nil, def.Type+" nil params")
		check(def, map[string]any{}, def.Type+" empty params")
		check(def, base, def.Type+" plain params")

		if p := catchPanic(func() {
			_ = def.Validate(nil, vc)
			_ = def.FieldsOf(nil)
			_ = def.OutputPorts(nil)
			_ = def.EffectsOf(nil)
		}); p != nil {
			fail("%s: a hook panicked on a nil node: %v", def.Type, p)
		}
	}
	if runs < 600 {
		t.Fatalf("only %d combinations ran", runs)
	}
	if len(failures) > 0 {
		if len(failures) > 15 {
			failures = append(failures[:15], fmt.Sprintf("... and %d more", len(failures)-15))
		}
		t.Fatalf("%d of %d combinations failed:\n%s", len(failures), runs, strings.Join(failures, "\n"))
	}
}

// send_telegram and send_notification both run SendNotification, which always lists the
// channels it tried. A success without a list confirms no send: it fails closed, for
// the telegram and the push node alike (the push node would otherwise report
// "sent" and an empty result list).
func TestNotifyNeedsAListedChannel(t *testing.T) {
	for _, typ := range []string{TypeTelegram, TypePush} {
		def := lookupDef(t, notifyRegistry(t), typ)
		for name, reply := range map[string]string{
			"no results":    `Tool Output: {"status":"success"}`,
			"null results":  `Tool Output: {"status":"success","results":null}`,
			"empty results": `Tool Output: {"status":"success","results":[]}`,
		} {
			tools := &fakeTools{respond: toolReply(reply)}
			res, err := execDef(def, map[string]any{"message": "m"}, &Services{Tools: tools})
			ne := asNodeError(err)
			if ne == nil || ne.Code != "FLOW_TOOL_ERROR" || !strings.Contains(ne.Message, "listed no channel") || res.Output != nil || tools.count() != 1 {
				t.Errorf("%s %s: %v, output %v", typ, name, err, res.Output)
			}
		}
	}
	// Email and Discord answer with a message, not a channel list.
	for typ, params := range map[string]map[string]any{TypeEmail: {"to": "a@b.de", "body": "b"}, TypeDiscord: {"message": "m"}} {
		tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"success"}`)}
		if res, err := execDef(lookupDef(t, notifyRegistry(t), typ), params, &Services{Tools: tools}); err != nil || res.Output["sent"] != true {
			t.Errorf("%s: %v", typ, err)
		}
	}
}
