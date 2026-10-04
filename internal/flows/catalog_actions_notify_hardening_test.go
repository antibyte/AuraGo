package flows

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// Control characters the tests need. They are named constants so a mangled escape
// sequence in an edit shows up in TestNotifyTestConstants instead of silently
// turning a test into one that proves nothing.
const (
	notifyCR   = "\r"
	notifyLF   = "\n"
	notifyCRLF = "\r\n"
	notifyNUL  = "\x00"
)

func TestNotifyTestConstants(t *testing.T) {
	if !reflect.DeepEqual([]byte(notifyCRLF), []byte{13, 10}) || []byte(notifyCR)[0] != 13 || []byte(notifyLF)[0] != 10 || !reflect.DeepEqual([]byte(notifyNUL), []byte{0}) {
		t.Fatal("an escape sequence in a test constant was mangled")
	}
}

// notifyReplies answer each notification tool with a success of its own shape.
var notifyReplies = map[string]string{
	"send_telegram":     sentOK,
	"send_email":        `Tool Output: {"status":"success","message":"Email sent"}`,
	"send_notification": `Tool Output: {"status":"success","results":[{"channel":"ntfy","status":"sent"}]}`,
	"send_discord":      `Tool Output: {"status":"success","message":"Message sent"}`,
}

func notifyAnswers(req ToolRequest) (ToolResponse, error) {
	return ToolResponse{Output: notifyReplies[req.Tool], Status: "success"}, nil
}

// notifyRun executes a notification node against tools that always succeed.
func notifyRun(t *testing.T, typ string, params map[string]any) (*fakeTools, ExecResult, error) {
	t.Helper()
	tools := &fakeTools{respond: notifyAnswers}
	res, err := execDef(lookupDef(t, notifyRegistry(t), typ), params, &Services{Tools: tools})
	return tools, res, err
}

// notifyWantInvalid fails unless err is a FLOW_PARAM_INVALID raised before any tool call,
// with a short, clean message that does not echo the value (echo, when it is long
// enough to be telling).
func notifyWantInvalid(t *testing.T, label string, tools *fakeTools, err error, echo string) {
	t.Helper()
	ne := asNodeError(err)
	switch {
	case ne == nil:
		t.Errorf("%s: accepted", label)
	case ne.Code != "FLOW_PARAM_INVALID":
		t.Errorf("%s: code %s: %.100s", label, ne.Code, ne.Message)
	case tools.count() != 0:
		t.Errorf("%s: the tool was called", label)
	case len(ne.Message) > maxEchoMessageBytes || !utf8.ValidString(ne.Message) || strings.ContainsAny(ne.Message, notifyLF+notifyCR+notifyNUL):
		t.Errorf("%s: unclean or long message %.100q", label, ne.Message)
	case len(echo) >= 4 && strings.Contains(ne.Message, echo):
		t.Errorf("%s: the message echoes the value: %q", label, ne.Message)
	}
}

func notifyEchoOf(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func notifyAddressList(n int) (list, normalized string) {
	var parts []string
	for i := 1; i <= n; i++ {
		parts = append(parts, fmt.Sprintf("u%d@x.de", i))
	}
	return strings.Join(parts, ","), strings.Join(parts, ", ")
}

// The recipients go into the To header as text and into RCPT commands one by one,
// so only plain addresses pass: no display name, no angle brackets, no line break,
// at most 20, and the text is normalised to "a, b".
func TestEmailRecipientRules(t *testing.T) {
	twenty, twentyNormal := notifyAddressList(20)
	twentyOne, _ := notifyAddressList(21)
	good := []struct {
		name string
		to   any
		want string
	}{
		{"single", "a@b.de", "a@b.de"},
		{"padded", " a@b.de" + notifyLF, "a@b.de"},
		{"two", "a@b.de,c@d.de", "a@b.de, c@d.de"},
		{"blanks around commas", "a@b.de ,\tc@d.de ,  e@f.de", "a@b.de, c@d.de, e@f.de"},
		{"plus and dots", "first.last+tag@sub.example.com", "first.last+tag@sub.example.com"},
		{"no top level domain", "root@localhost", "root@localhost"},
		{"twenty", twenty, twentyNormal},
	}
	for _, c := range good {
		tools, res, err := notifyRun(t, TypeEmail, map[string]any{"to": c.to, "body": "b"})
		if err != nil || res.Output["to"] != c.want || tools.last(t).Args["to"] != c.want {
			t.Errorf("%s: err %v, output %v", c.name, err, res.Output)
		}
	}
	bad := []struct {
		name string
		to   any
	}{
		{"empty", ""}, {"blank", "  "}, {"nil", nil}, {"only a comma", ","},
		{"empty entry", "a@b.de,,c@d.de"}, {"trailing comma", "a@b.de,"}, {"leading comma", ",a@b.de"},
		{"display name", "Anna Blume <a@b.de>"}, {"angle brackets", "<a@b.de>"}, {"comment", "a@b.de (Anna)"},
		{"quoted local part", `"a b"@c.de`}, {"quoted comma", `"a,b"@c.de`},
		{"no at sign", "abc"}, {"two at signs", "a@@b.de"}, {"space inside", "a b@c.de"}, {"semicolon", "a@b.de;c@d.de"},
		{"line feed and a header", "a@b.de" + notifyLF + "Bcc: evil@x.de"}, {"crlf and a header", "a@b.de" + notifyCRLF + "Bcc: evil@x.de"},
		{"carriage return", "a@b.de" + notifyCR + "Bcc: evil@x.de"}, {"line feed after a comma", "a@b.de," + notifyLF + "evil@x.de"},
		{"nul", "a@b.de" + notifyNUL}, {"bell", "a@b.de\a"}, {"delete", "a@b.de\x7f"}, {"invalid utf-8", "a@b.de\xff"},
		{"address too long", strings.Repeat("a", 250) + "@b.de"},
		{"twenty one", twentyOne}, {"huge list", strings.Repeat("a@b.de,", 1000)}, {"huge text", strings.Repeat("a", 1<<20)},
		{"number", 5.0}, {"flag", true}, {"list", []any{"a@b.de"}}, {"object", map[string]any{"a": "b@c.de"}},
	}
	for _, c := range bad {
		tools, _, err := notifyRun(t, TypeEmail, map[string]any{"to": c.to, "body": "b"})
		notifyWantInvalid(t, "to "+c.name, tools, err, notifyEchoOf(c.to))
	}
}

// A line break inside the recipients, the subject or the account is refused before
// anything is sent; a newline around the text (a template result usually ends with
// one) is trimmed away and harmless.
func TestEmailHeaderFieldsRejectLineBreaks(t *testing.T) {
	base := map[string]any{"to": "a@b.de", "subject": "S", "body": "b"}
	with := func(field string, v any) map[string]any {
		p := map[string]any{}
		for k, x := range base {
			p[k] = x
		}
		p[field] = v
		return p
	}
	attacks := []string{
		"x" + notifyCRLF + "Bcc: evil@x.de", "x" + notifyLF + "Bcc: evil@x.de", "x" + notifyCR + "Bcc: evil@x.de",
		"x" + notifyNUL + "evil", "x" + notifyCRLF + notifyCRLF + "evil body", notifyCRLF + "x" + notifyLF + "y",
	}
	for _, field := range []string{"to", "subject", "account"} {
		for i, attack := range attacks {
			value := attack
			if field == "to" {
				value = "a@b.de," + attack
			}
			tools, _, err := notifyRun(t, TypeEmail, with(field, value))
			notifyWantInvalid(t, fmt.Sprintf("%s attack %d", field, i), tools, err, "evil")
		}
	}
	tools, res, err := notifyRun(t, TypeEmail, map[string]any{"to": "a@b.de" + notifyCRLF, "subject": " S" + notifyCRLF, "account": notifyLF + "work" + notifyLF, "body": "b"})
	if err != nil || res.Output["to"] != "a@b.de" {
		t.Fatalf("surrounding newlines: %v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; args["subject"] != "S" || args["account"] != "work" {
		t.Errorf("not trimmed: %#v", args)
	}
}

// The account id is read back by the tool into its answer as JSON text without
// escaping; an id with a quote could write a "status":"success" of its own.
func TestEmailAccountRules(t *testing.T) {
	for _, id := range []string{"work", "  work ", "Büro 1", "a.b-c_d", "o'brien", strings.Repeat("a", maxAccountBytes)} {
		tools, _, err := notifyRun(t, TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "account": id})
		if err != nil || tools.last(t).Args["account"] != strings.TrimSpace(id) {
			t.Errorf("account %q: %v, %#v", id, err, tools.allCalls())
		}
	}
	bad := []any{
		`a"b`, `a\b`, "a\tb", `x", "status": "success", "z": "`, "a" + notifyNUL, "a\x1bb", "a\xff", strings.Repeat("a", maxAccountBytes+1),
		[]any{"work"}, map[string]any{"id": "work"},
	}
	for i, id := range bad {
		tools, _, err := notifyRun(t, TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "account": id})
		notifyWantInvalid(t, fmt.Sprintf("account %d", i), tools, err, notifyEchoOf(id))
	}
	// A blank account means the default one: nothing is passed.
	for _, v := range []any{nil, "", "  "} {
		tools, _, err := notifyRun(t, TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "account": v})
		if _, set := tools.last(t).Args["account"]; err != nil || set {
			t.Errorf("blank account %#v: %v %#v", v, err, tools.last(t).Args)
		}
	}
}

// Texts are limited before the tool is called and are never cut: what the tool gets
// is exactly what the flow said, or the node fails.
func TestEmailLimits(t *testing.T) {
	send := func(params map[string]any) (*fakeTools, error) {
		params["to"] = "a@b.de"
		tools, _, err := notifyRun(t, TypeEmail, params)
		return tools, err
	}
	subject700 := strings.Repeat("s", maxEmailSubjectBytes)
	umlauts350 := strings.Repeat("ä", maxEmailSubjectBytes/2)
	body := strings.Repeat("b", maxEmailBodyBytes)
	for _, s := range []string{subject700, umlauts350} {
		tools, err := send(map[string]any{"subject": s, "body": body})
		if err != nil || tools.last(t).Args["subject"] != s || tools.last(t).Args["body"] != body {
			t.Fatalf("limits are allowed: %v", err)
		}
	}
	for name, params := range map[string]map[string]any{
		"subject": {"subject": subject700 + "s", "body": "b"},
		"umlauts": {"subject": umlauts350 + "ä", "body": "b"},
		"body":    {"body": body + "b"},
	} {
		tools, err := send(params)
		notifyWantInvalid(t, name, tools, err, "")
	}
	// An empty or absent body is a mail with no text, as the tool allows. A list is JSON.
	for in, want := range map[any]string{nil: "", "": "", "   ": "   "} {
		tools, err := send(map[string]any{"body": in})
		if got := tools.last(t).Args["body"]; err != nil || got != want {
			t.Errorf("body %#v: %v %#v", in, err, got)
		}
	}
	tools, err := send(map[string]any{"body": []any{"a", 1.0}, "subject": 5.0})
	if err != nil || tools.last(t).Args["body"] != `["a",1]` || tools.last(t).Args["subject"] != "5" {
		t.Errorf("structured body: %v %#v", err, tools.last(t).Args)
	}
	for name, v := range map[string]any{"list": []any{"x"}, "object": map[string]any{}} {
		tools, err := send(map[string]any{"body": "b", "subject": v})
		notifyWantInvalid(t, "subject "+name, tools, err, "")
	}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	tools, err = send(map[string]any{"body": cyclic})
	notifyWantInvalid(t, "body that cannot be encoded", tools, err, "")
}

// Both attachment parameters take a path that the tool reads from the workspace. A
// path is bounded, free of NUL and valid UTF-8; a value that is no file is an error
// (the node must not report a mail as sent that silently lost its file).
func TestAttachmentsAreBoundedAndNeverDropped(t *testing.T) {
	cases := []struct {
		typ, param, arg string
		base            map[string]any
	}{
		{TypeTelegram, "file", "file_path", map[string]any{"message": "m"}},
		{TypeEmail, "attachment", "attachments", map[string]any{"to": "a@b.de", "body": "b"}},
	}
	path4096 := strings.Repeat("p", maxFilePathBytes)
	for _, c := range cases {
		run := func(v any) (*fakeTools, ExecResult, error) {
			params := map[string]any{c.param: v}
			for k, x := range c.base {
				params[k] = x
			}
			return notifyRun(t, c.typ, params)
		}
		want := func(p string) any {
			if c.typ == TypeEmail {
				return []any{p}
			}
			return p
		}
		for name, v := range map[string]any{"text": "docs/a.pdf", "padded": " docs/a.pdf" + notifyLF, "file object": FileRef("docs/a.pdf", "", "", "", 0), "object with extras": map[string]any{"path": "docs/a.pdf", "x": 1.0}} {
			if tools, _, err := run(v); err != nil || !reflect.DeepEqual(tools.last(t).Args[c.arg], want("docs/a.pdf")) {
				t.Errorf("%s %s: %v, %#v", c.typ, name, err, tools.allCalls())
			}
		}
		if tools, _, err := run(path4096); err != nil || !reflect.DeepEqual(tools.last(t).Args[c.arg], want(path4096)) {
			t.Errorf("%s: a path at the limit: %v", c.typ, err)
		}
		for name, v := range map[string]any{"nil": nil, "empty": "", "blank": " \t ", "empty object": map[string]any{}, "empty list": []any{}} {
			tools, _, err := run(v)
			if _, set := tools.last(t).Args[c.arg]; err != nil || set {
				t.Errorf("%s %s: no attachment expected: %v %#v", c.typ, name, err, tools.last(t).Args)
			}
		}
		bad := map[string]any{
			"nul": "a" + notifyNUL + "b", "invalid utf-8": "a\xff", "too long": path4096 + "p",
			"object without a path": map[string]any{"name": "a.pdf"}, "object with an empty path": map[string]any{"path": ""},
			"object with a number": map[string]any{"path": 5.0}, "object with a list": map[string]any{"path": []any{"a"}},
			"number": 5.0, "flag": true, "list": []any{"a.pdf"}, "huge": strings.Repeat("p", 1<<20),
		}
		for name, v := range bad {
			tools, _, err := run(v)
			notifyWantInvalid(t, c.typ+" "+name, tools, err, notifyEchoOf(v))
		}
	}
}

// Message, title, subject and body carry the content to a person: they are not
// sinks (like the content of doc.pdf_create). What decides where a message goes, from
// which account, or which file goes with it is.
func TestNotifySinkFlags(t *testing.T) {
	sinks := map[string][]string{
		TypeTelegram: {"file"},
		TypeEmail:    {"to", "attachment", "account"},
		TypePush:     {"channel"},
		TypeDiscord:  {"channel_id"},
	}
	reg := notifyRegistry(t)
	for typ, want := range sinks {
		var got []string
		for _, p := range lookupDef(t, reg, typ).Params {
			if p.SensitiveSink {
				got = append(got, p.Name)
			}
		}
		slices.Sort(got)
		slices.Sort(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: sink parameters %v, want %v", typ, got, want)
		}
	}
	for _, typ := range []string{TypeTelegram, TypeEmail, TypePush, TypeDiscord} {
		def := lookupDef(t, reg, typ)
		if def.UntrustedOutput || !reflect.DeepEqual(def.Effects, []Effect{EffectSendsMessage}) {
			t.Errorf("%s: untrusted %v, effects %v", typ, def.UntrustedOutput, def.Effects)
		}
	}

	// The lint: untrusted webhook data in a sink warns, in a message does not.
	full := triggerRegistry(t)
	if err := registerNotifyNodes(full, nil); err != nil {
		t.Fatal(err)
	}
	b := newFlow("Notify lint")
	tr := b.node("start", TypeTriggerWebhook, nil)
	ref := func(name string) string { return "{{trigger.data.payload." + name + "}}" }
	nodes := map[string]string{
		"tg": b.node("tg", TypeTelegram, map[string]any{"message": ref("m"), "title": ref("t"), "file": ref("f")}),
		"em": b.node("em", TypeEmail, map[string]any{"to": ref("to"), "subject": ref("s"), "body": ref("b"), "attachment": ref("a"), "account": ref("acc")}),
		"pu": b.node("pu", TypePush, map[string]any{"channel": ref("c"), "title": ref("t"), "message": ref("m")}),
		"dc": b.node("dc", TypeDiscord, map[string]any{"message": ref("m"), "channel_id": ref("c")}),
	}
	for _, id := range nodes {
		b.edge(tr, PortOut, id)
	}
	byNode := map[string][]string{}
	for _, is := range LintUntrustedData(b.build(), full) {
		if is.Code != IssueUntrustedData || is.Severity != SeverityWarning {
			t.Errorf("unexpected issue %+v", is)
		}
		byNode[is.NodeID] = append(byNode[is.NodeID], is.Param)
	}
	wantWarned := map[string][]string{
		nodes["tg"]: {"file"}, nodes["em"]: {"account", "attachment", "to"}, nodes["pu"]: {"channel"}, nodes["dc"]: {"channel_id"},
	}
	for id, want := range wantWarned {
		got := byNode[id]
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("node %s: warned on %v, want %v", id, got, want)
		}
	}
}

// Telegram counts UTF-16 code units of "<title>\n<message>" (the title is AuraGo when
// there is none) and the tool does not split, so what does not fit is refused.
func TestTelegramSizeLimit(t *testing.T) {
	send := func(msg, title any) (*fakeTools, error) {
		tools, _, err := notifyRun(t, TypeTelegram, map[string]any{"message": msg, "title": title})
		return tools, err
	}
	ascii := func(n int) string { return strings.Repeat("m", n) }
	fit := len(telegramDefaultName) + 1
	for _, c := range []struct {
		name       string
		msg, title string
		ok         bool
	}{
		{"default title fits", ascii(telegramTextLimit - fit), "", true},
		{"default title one over", ascii(telegramTextLimit - fit + 1), "", false},
		{"short title fits", ascii(telegramTextLimit - 5), "News", true},
		{"short title one over", ascii(telegramTextLimit - 4), "News", false},
		{"one character title fits", ascii(telegramTextLimit - 2), "N", true},
		{"emoji count twice", strings.Repeat("😀", (telegramTextLimit-fit)/2), "", true},
		{"emoji one over", strings.Repeat("😀", (telegramTextLimit-fit)/2+1), "", false},
		{"umlauts count once", strings.Repeat("ä", telegramTextLimit-fit), "", true},
		{"message alone too long", ascii(telegramTextLimit), "N", false},
	} {
		tools, err := send(c.msg, c.title)
		if c.ok {
			if err != nil || tools.last(t).Args["message"] != c.msg {
				t.Errorf("%s: %v (cut or refused)", c.name, err)
			}
			continue
		}
		notifyWantInvalid(t, c.name, tools, err, "")
	}
	title250 := strings.Repeat("ä", maxNotifyTitleRunes)
	if tools, err := send("m", title250); err != nil || tools.last(t).Args["title"] != title250 {
		t.Errorf("a title at the limit: %v", err)
	}
	for name, title := range map[string]any{
		"too long": title250 + "ä", "line feed": "a" + notifyLF + "b", "crlf": "a" + notifyCRLF + "b", "tab": "a\tb", "nul": "a" + notifyNUL, "invalid utf-8": "a\xff",
		"list": []any{"t"}, "object": map[string]any{},
	} {
		tools, err := send("m", title)
		notifyWantInvalid(t, "title "+name, tools, err, notifyEchoOf(title))
	}
	// An absent title is not passed, so the tool applies its own.
	if tools, err := send("m", nil); err != nil || tools.last(t).Args["title"] != nil {
		t.Errorf("no title: %v %#v", err, tools.last(t).Args)
	}
}

func TestPushRules(t *testing.T) {
	send := func(params map[string]any) (*fakeTools, error) {
		if _, ok := params["message"]; !ok {
			params["message"] = "m"
		}
		tools, _, err := notifyRun(t, TypePush, params)
		return tools, err
	}
	msg := strings.Repeat("m", maxPushMessageBytes)
	if tools, err := send(map[string]any{"message": msg}); err != nil || tools.last(t).Args["message"] != msg {
		t.Errorf("a message at the limit: %v", err)
	}
	umlauts := strings.Repeat("ä", maxPushMessageBytes/2)
	if tools, err := send(map[string]any{"message": umlauts}); err != nil || tools.last(t).Args["message"] != umlauts {
		t.Errorf("umlauts at the limit: %v", err)
	}
	for name, m := range map[string]any{"one byte over": msg + "m", "umlauts over": umlauts + "ä", "blank": " ", "nil": nil, "cyclic": func() any { c := map[string]any{}; c["x"] = c; return c }()} {
		tools, err := send(map[string]any{"message": m})
		notifyWantInvalid(t, "message "+name, tools, err, "")
	}

	for in, want := range map[any]string{nil: "all", "": "all", "  ": "all", " NTFY ": "ntfy", "pushover": "pushover", "Web-Push_2": "web-push_2"} {
		if tools, err := send(map[string]any{"channel": in}); err != nil || tools.last(t).Args["channel"] != want {
			t.Errorf("channel %#v: %v %#v", in, err, tools.last(t).Args)
		}
	}
	for i, ch := range []any{"a b", "nt" + notifyLF + "fy", `x"y`, "all;drop", "ü", strings.Repeat("a", maxChannelBytes+1), []any{"ntfy"}, 5.0, true} {
		tools, err := send(map[string]any{"channel": ch})
		notifyWantInvalid(t, fmt.Sprintf("channel %d", i), tools, err, notifyEchoOf(ch))
	}

	for in, want := range map[any]string{nil: "normal", "": "normal", "HIGH": "high", " critical ": "critical", "low": "low"} {
		if tools, err := send(map[string]any{"priority": in}); err != nil || tools.last(t).Args["priority"] != want {
			t.Errorf("priority %#v: %v %#v", in, err, tools.last(t).Args)
		}
	}
	for i, p := range []any{"urgent", "hi" + notifyLF + "gh", 2.0, true, []any{"low"}} {
		tools, err := send(map[string]any{"priority": p})
		notifyWantInvalid(t, fmt.Sprintf("priority %d", i), tools, err, notifyEchoOf(p))
	}

	// The title is an HTTP header (ntfy): one line only. Without one it is not passed.
	for i, title := range []any{"a" + notifyLF + "b", "a" + notifyCRLF + "X-Evil: 1", strings.Repeat("t", maxNotifyTitleRunes+1)} {
		tools, err := send(map[string]any{"title": title})
		notifyWantInvalid(t, fmt.Sprintf("title %d", i), tools, err, "evil")
	}
	if tools, err := send(map[string]any{}); err != nil || tools.last(t).Args["title"] != nil {
		t.Errorf("no title: %v %#v", err, tools.last(t).Args)
	}
}

func TestDiscordRules(t *testing.T) {
	send := func(params map[string]any) (*fakeTools, error) {
		if _, ok := params["message"]; !ok {
			params["message"] = "m"
		}
		tools, _, err := notifyRun(t, TypeDiscord, params)
		return tools, err
	}
	id20 := strings.Repeat("9", maxDiscordIDDigits)
	for in, want := range map[any]string{"123": "123", " 1234567890123456789 ": "1234567890123456789", id20: id20} {
		if tools, err := send(map[string]any{"channel_id": in}); err != nil || tools.last(t).Args["channel_id"] != want {
			t.Errorf("channel id %#v: %v %#v", in, err, tools.last(t).Args)
		}
	}
	for _, in := range []any{nil, "", "  "} {
		if tools, err := send(map[string]any{"channel_id": in}); err != nil || tools.last(t).Args["channel_id"] != nil {
			t.Errorf("no channel id %#v: %v %#v", in, err, tools.last(t).Args)
		}
	}
	// A number is floating point in a flow and loses the lower digits of an id.
	for i, id := range []any{id20 + "9", "12a", "-1", "1.5", "1e3", `1", "status": "success`, "1" + notifyLF + "2", "١٢٣", 123.0, 1.2345678901234568e17, 7, true, []any{"1"}} {
		tools, err := send(map[string]any{"channel_id": id})
		notifyWantInvalid(t, fmt.Sprintf("channel id %d", i), tools, err, notifyEchoOf(id))
	}
	// Discord takes 2000 characters and the tool splits longer text; the node stops at 8000.
	long := strings.Repeat("ä", maxDiscordMessageRunes)
	if tools, err := send(map[string]any{"message": long}); err != nil || tools.last(t).Args["message"] != long {
		t.Errorf("a message at the limit: %v", err)
	}
	for name, m := range map[string]any{"one over": long + "ä", "blank": " " + notifyLF, "nil": nil, "list that is empty": []any{}} {
		tools, err := send(map[string]any{"message": m})
		notifyWantInvalid(t, "message "+name, tools, err, "")
	}
	if tools, _, err := notifyRun(t, TypeDiscord, map[string]any{"message": []any{"a", map[string]any{"b": 1.0}}}); err != nil ||
		tools.last(t).Args["message"] != `["a",{"b":1}]` {
		t.Errorf("a structured message is sent as JSON: %v", err)
	}
}

// The output is JSON and the engine stores it: make sure what the nodes return can be encoded.
func TestNotifyOutputsAreJSON(t *testing.T) {
	for typ, params := range map[string]map[string]any{
		TypeTelegram: {"message": "m"}, TypeEmail: {"to": "a@b.de", "body": "b"}, TypePush: {"message": "m"}, TypeDiscord: {"message": "m"},
	} {
		_, res, err := notifyRun(t, typ, params)
		if _, jsonErr := json.Marshal(res.Output); err != nil || jsonErr != nil || res.Output["sent"] != true {
			t.Errorf("%s: %v %v %v", typ, err, jsonErr, res.Output)
		}
	}
}

// The limits are numbers the real tools and channels dictate. The other tests use the
// constants, so a constant that drifts would still pass them: pin each limit to its
// literal, one below or at the limit passes and one over fails.
func TestNotifyLimitsAreTheDocumentedNumbers(t *testing.T) {
	rep := func(s string, n int) string { return strings.Repeat(s, n) }
	to20, _ := notifyAddressList(20)
	to21, _ := notifyAddressList(21)
	cases := []struct {
		name   string
		typ    string
		params map[string]any
		ok     bool
	}{
		// Telegram: 4096 UTF-16 units for "AuraGo" + newline + message, so 4089 and not 4090.
		{"telegram 4089", TypeTelegram, map[string]any{"message": rep("m", 4089)}, true},
		{"telegram 4090", TypeTelegram, map[string]any{"message": rep("m", 4090)}, false},
		{"telegram title 250", TypeTelegram, map[string]any{"message": "m", "title": rep("t", 250)}, true},
		{"telegram title 251", TypeTelegram, map[string]any{"message": "m", "title": rep("t", 251)}, false},
		{"telegram file 4096", TypeTelegram, map[string]any{"message": "m", "file": rep("p", 4096)}, true},
		{"telegram file 4097", TypeTelegram, map[string]any{"message": "m", "file": rep("p", 4097)}, false},
		// Email: a subject of 700 bytes, 20 recipients, a body of 64 KiB.
		{"subject 700", TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "subject": rep("s", 700)}, true},
		{"subject 701", TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "subject": rep("s", 701)}, false},
		{"20 recipients", TypeEmail, map[string]any{"to": to20, "body": "b"}, true},
		{"21 recipients", TypeEmail, map[string]any{"to": to21, "body": "b"}, false},
		{"body 64 KiB", TypeEmail, map[string]any{"to": "a@b.de", "body": rep("b", 65536)}, true},
		{"body 64 KiB + 1", TypeEmail, map[string]any{"to": "a@b.de", "body": rep("b", 65537)}, false},
		{"attachment 4096", TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "attachment": rep("p", 4096)}, true},
		{"attachment 4097", TypeEmail, map[string]any{"to": "a@b.de", "body": "b", "attachment": rep("p", 4097)}, false},
		// Push: 4096 bytes, a title of 250 characters.
		{"push 4096", TypePush, map[string]any{"message": rep("m", 4096)}, true},
		{"push 4097", TypePush, map[string]any{"message": rep("m", 4097)}, false},
		{"push title 250", TypePush, map[string]any{"message": "m", "title": rep("t", 250)}, true},
		{"push title 251", TypePush, map[string]any{"message": "m", "title": rep("t", 251)}, false},
		// Discord: 8000 characters, a channel id of 20 digits.
		{"discord 8000", TypeDiscord, map[string]any{"message": rep("m", 8000)}, true},
		{"discord 8001", TypeDiscord, map[string]any{"message": rep("m", 8001)}, false},
		{"discord id 20", TypeDiscord, map[string]any{"message": "m", "channel_id": rep("7", 20)}, true},
		{"discord id 21", TypeDiscord, map[string]any{"message": "m", "channel_id": rep("7", 21)}, false},
	}
	for _, c := range cases {
		tools, _, err := notifyRun(t, c.typ, c.params)
		if c.ok {
			if err != nil || tools.count() != 1 {
				t.Errorf("%s: refused: %v", c.name, err)
			}
			continue
		}
		notifyWantInvalid(t, c.name, tools, err, "")
	}
}
