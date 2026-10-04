package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// independentParams lists every parameter flagged OutputIndependent: it reaches only
// the node's effect and never the node's output. Adding to this list needs a look at
// Execute, which is what TestOutputIndependentParamsAreNotInTheOutput automates.
var independentParams = []string{
	"doc.pdf_create.content", "doc.pdf_create.title",
	"file.write.content",
	"notify.discord.message",
	"notify.email.body", "notify.email.subject",
	"notify.push.message", "notify.push.title",
	"notify.telegram.message", "notify.telegram.title",
}

func independentRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	for _, err := range []error{
		RegisterLogicNodes(reg), RegisterTriggerNodes(reg), registerWebNodes(reg, nil),
		RegisterAINodes(reg), registerDocNodes(reg, nil), registerNotifyNodes(reg, nil),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return reg
}

// The flag is on exactly these parameters and no sink that picks a name, path or
// address carries it.
func TestOutputIndependentFlagIsOnTheListedParams(t *testing.T) {
	var got []string
	for _, def := range independentRegistry(t).All() {
		for _, p := range def.Params {
			if p.OutputIndependent {
				got = append(got, def.Type+"."+p.Name)
			}
		}
	}
	slices.Sort(got)
	if !reflect.DeepEqual(got, independentParams) {
		t.Fatalf("flagged %v, want %v", got, independentParams)
	}
	// The flag is a lint hint, not part of the editor's API.
	raw, err := json.Marshal(ParamSpec{Name: "x", OutputIndependent: true})
	if err != nil || strings.Contains(string(raw), "independent") {
		t.Errorf("the flag is serialised: %s %v", raw, err)
	}
}

// Whatever a flagged parameter holds, the node's output does not: run each node with a
// marker in the parameter and look for the marker in the output.
func TestOutputIndependentParamsAreNotInTheOutput(t *testing.T) {
	const marker = "MARKER-7f3a91"
	bases := map[string]map[string]any{
		TypePDFCreate: {"content": "c"}, TypeFileWrite: {"path": "out.txt"},
		TypeTelegram: {"message": "m"}, TypeEmail: {"to": "a@b.de", "body": "b"}, TypePush: {"message": "m"}, TypeDiscord: {"message": "m"},
	}
	answers := func(req ToolRequest) (ToolResponse, error) {
		if req.Tool == "document_creator" {
			return ToolResponse{Output: `{"status":"success","file_path":"/data/documents/x.pdf","web_path":"/files/documents/x.pdf","filename":"x.pdf"}`, Status: "success"}, nil
		}
		if reply, ok := notifyReplies[req.Tool]; ok {
			return ToolResponse{Output: reply, Status: "success"}, nil
		}
		return ToolResponse{Output: `{"status":"success"}`, Status: "success"}, nil
	}
	reg := independentRegistry(t)
	for _, name := range independentParams {
		i := strings.LastIndex(name, ".")
		typ, param := name[:i], name[i+1:]
		params := map[string]any{}
		for k, v := range bases[typ] {
			params[k] = v
		}
		params[param] = marker
		tools := &fakeTools{respond: answers}
		res, err := execDef(lookupDef(t, reg, typ), params, &Services{Tools: tools})
		if err != nil || tools.count() == 0 {
			t.Errorf("%s: %v after %d calls", name, err, tools.count())
			continue
		}
		if raw, _ := json.Marshal(res.Output); strings.Contains(string(raw), marker) {
			t.Errorf("%s is copied into the output %s", name, raw)
		}
	}
}

// flagshipFlow is the shape of the flagship template: untrusted search results go
// through an AI step into a PDF, the PDF goes to Telegram, and a mail is sent and
// its result written to a file.
func flagshipFlow(pdfParams, mailParams map[string]any, writeContent string) *Flow {
	b := newFlow("flagship")
	sch := b.node("schedule", TypeTriggerSchedule, map[string]any{"mode": "weekdays", "time": "07:00"})
	search := b.node("search", TypeWebSearch, map[string]any{"query": "ai news", "count": 5.0})
	summary := b.node("summary", TypeAIStep, map[string]any{"prompt": `x {{search.results | pluck("snippet") | join(" ")}}`})
	pdf := b.node("pdf", TypePDFCreate, pdfParams)
	tg := b.node("telegram", TypeTelegram, map[string]any{"message": "Hi", "file": "{{pdf.file}}"})
	mail := b.node("mail", TypeEmail, mailParams)
	wr := b.node("wr", TypeFileWrite, map[string]any{"path": "out.txt", "content": writeContent})
	b.edge(sch, PortOut, search)
	b.edge(search, PortOut, summary)
	b.edge(summary, PortOut, pdf)
	b.edge(pdf, PortOut, tg)
	b.edge(tg, PortOut, mail)
	b.edge(mail, PortOut, wr)
	return b.build()
}

func lintSummary(issues []Issue) []string {
	var got []string
	for _, is := range issues {
		got = append(got, fmt.Sprintf("%s.%s", is.NodeID, is.Param))
	}
	return got
}

// Untrusted text that only becomes the content of a document or the text of a message
// does not make the document's path or the mail's result untrusted. What picks the
// document's name, or the mail's recipient, still does.
func TestLintOutputIndependentParams(t *testing.T) {
	reg := independentRegistry(t)
	node := func(f *Flow, key string) string {
		t.Helper()
		for _, n := range f.Nodes {
			if n.Key == key {
				return n.ID
			}
		}
		t.Fatalf("no node %s", key)
		return ""
	}
	untrusted := "{{summary.text}}"
	mailBody := map[string]any{"to": "a@b.de", "subject": "News", "body": untrusted}

	// Content only: the PDF path, the Telegram file and the mail result are clean.
	f := flagshipFlow(map[string]any{"title": "News", "content": untrusted}, mailBody, "{{mail.sent}}")
	if issues := LintUntrustedData(f, reg); len(issues) != 0 {
		t.Errorf("content only: %v", lintSummary(issues))
	}

	// The same data picking the file name taints the path: pdf.filename and telegram.file warn.
	f = flagshipFlow(map[string]any{"title": "News", "content": untrusted, "filename": untrusted}, mailBody, "{{mail.sent}}")
	pdf, tg := node(f, "pdf"), node(f, "telegram")
	want := []string{pdf + ".filename", tg + ".file"}
	if got := lintSummary(LintUntrustedData(f, reg)); !reflect.DeepEqual(got, want) {
		t.Errorf("tainted filename: %v, want %v", got, want)
	}

	// Untrusted data in the recipient: mail.to warns, and the result carries the taint
	// (the mail returns its recipients).
	f = flagshipFlow(map[string]any{"title": "News", "content": "x"}, map[string]any{"to": untrusted, "body": "b"}, "{{mail.to}}")
	mail, wr := node(f, "mail"), node(f, "wr")
	want = []string{mail + ".to", wr + ".content"}
	if got := lintSummary(LintUntrustedData(f, reg)); !reflect.DeepEqual(got, want) {
		t.Errorf("tainted recipient: %v, want %v", got, want)
	}

	// Untrusted data in a message of every notification node: no taint on what they return.
	for _, c := range []struct {
		typ    string
		params map[string]any
		out    string
	}{
		{TypeTelegram, map[string]any{"message": untrusted, "title": untrusted}, "{{n.sent}}"},
		{TypePush, map[string]any{"message": untrusted, "title": untrusted}, "{{n.results}}"},
		{TypeDiscord, map[string]any{"message": untrusted}, "{{n.sent}}"},
		{TypeEmail, map[string]any{"to": "a@b.de", "subject": untrusted, "body": untrusted}, "{{n.to}}"},
	} {
		b := newFlow(c.typ)
		search := b.node("search", TypeWebSearch, map[string]any{"query": "q"})
		summary := b.node("summary", TypeAIStep, map[string]any{"prompt": "{{search.results}}"})
		n := b.node("n", c.typ, c.params)
		w := b.node("w", TypeFileWrite, map[string]any{"path": c.out, "content": "x"})
		b.edge(search, PortOut, summary)
		b.edge(summary, PortOut, n)
		b.edge(n, PortOut, w)
		// The path of the write is a sink and reads the notification's output.
		if issues := LintUntrustedData(b.build(), reg); len(issues) != 0 {
			t.Errorf("%s: %v", c.typ, lintSummary(issues))
		}
	}
}

// A node with more than the cap of template expressions cannot be judged: it counts as
// tainted whatever its parameters are, so a flagged parameter does not help.
func TestLintOutputIndependentKeepsTheCapConservative(t *testing.T) {
	reg := independentRegistry(t)
	b := newFlow("cap")
	web := b.node("web", TypeWebSearch, map[string]any{"query": "q"})
	// body comes after the harmless refs in key order, so the cap drops it.
	params := map[string]any{"to": "a@b.de", "body": "{{web.results}}"}
	for k, v := range harmlessRefParams("a", maxTemplateRefs+5) {
		params[k] = v
	}
	// Setup: the cap cuts off the only reference to untrusted data.
	refs, problems := CollectTemplateRefs(params)
	for _, r := range refs {
		if r.Expr.Root == "web" {
			t.Fatal("setup: the reference to the untrusted node was not cut off")
		}
	}
	if !hasTooManyRefs(problems) {
		t.Fatal("setup: the cap was not reached")
	}
	mail := b.node("mail", TypeEmail, params)
	wr := b.node("wr", TypeFileWrite, map[string]any{"path": "{{mail.sent}}", "content": "x"})
	b.edge(web, PortOut, mail)
	b.edge(mail, PortOut, wr)
	got := lintSummary(LintUntrustedData(b.build(), reg))
	// The capped node warns on every sink parameter, in definition order, and what it feeds is flagged.
	want := []string{mail + ".to", mail + ".attachment", mail + ".account", wr + ".path"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cap: %v, want %v", got, want)
	}

	// Without the cap the same untrusted body reference is judged and taints nothing.
	b = newFlow("nocap")
	web = b.node("web", TypeWebSearch, map[string]any{"query": "q"})
	mail = b.node("mail", TypeEmail, map[string]any{"to": "a.de", "body": "{{web.results}}"})
	wr = b.node("wr", TypeFileWrite, map[string]any{"path": "{{mail.sent}}", "content": "x"})
	b.edge(web, PortOut, mail)
	b.edge(mail, PortOut, wr)
	if got := lintSummary(LintUntrustedData(b.build(), reg)); len(got) != 0 {
		t.Errorf("without the cap: %v", got)
	}
}

// The engine rule on its own, with synthetic definitions: a flagged parameter does not
// taint the output, an unflagged one does, and a flagged sink still warns.
func TestLintOutputIndependentSemantics(t *testing.T) {
	reg := newTestRegistry(t)
	echo := func(typ string, params []ParamSpec) {
		reg.MustRegister(&NodeDef{Type: typ, Category: "test", Params: params, Execute: func(_ context.Context, in ExecInput) (ExecResult, error) {
			return ExecResult{Output: map[string]any{"value": "v"}}, nil
		}})
	}
	echo("test.indep", []ParamSpec{
		{Name: "text", Kind: ParamText, Templatable: true, OutputIndependent: true},
		{Name: "target", Kind: ParamText, Templatable: true, SensitiveSink: true, OutputIndependent: true},
		{Name: "other", Kind: ParamText, Templatable: true},
	})
	chain := func(params map[string]any) []Issue {
		b := newFlow("semantics")
		tr := b.node("start", "test.trigger", nil)
		web := b.node("web", "test.web", nil)
		mid := b.node("mid", "test.indep", params)
		sink := b.node("shell", "test.sink", map[string]any{"command": "{{mid.value}}"})
		b.edge(tr, PortOut, web)
		b.edge(web, PortOut, mid)
		b.edge(mid, PortOut, sink)
		return LintUntrustedData(b.build(), reg)
	}
	params := func(param string) map[string]any {
		return map[string]any{param: "{{web.text}}"}
	}
	if issues := chain(params("text")); len(issues) != 0 {
		t.Errorf("independent param: %+v", issues)
	}
	// Nested references count for their top parameter.
	if issues := chain(map[string]any{"text": "a {{web.text}} b"}); len(issues) != 0 {
		t.Errorf("independent param, text around the reference: %+v", issues)
	}
	issues := chain(params("other"))
	if len(issues) != 1 || issues[0].Param != "command" {
		t.Errorf("an ordinary param taints the output: %+v", issues)
	}
	// A flagged sink warns on itself, and its output stays clean.
	issues = chain(params("target"))
	if len(issues) != 1 || issues[0].Param != "target" || !strings.Contains(issues[0].Message, "mid") {
		t.Errorf("a flagged sink: %+v", issues)
	}
	// One tainted ordinary param among flagged ones is enough.
	if issues := chain(map[string]any{"text": "{{web.text}}", "other": "{{web.text}}"}); len(issues) != 1 || issues[0].Param != "command" {
		t.Errorf("mixed params: %+v", issues)
	}
	// An untrusted trigger works the same way.
	b := newFlow("trigger")
	hook := b.node("hook", "test.untrusted_trigger", nil)
	mid := b.node("mid", "test.indep", map[string]any{"text": "{{trigger.data.x}}"})
	sink := b.node("shell", "test.sink", map[string]any{"command": "{{mid.value}}"})
	b.edge(hook, PortOut, mid)
	b.edge(mid, PortOut, sink)
	if issues := LintUntrustedData(b.build(), reg); len(issues) != 0 {
		t.Errorf("untrusted trigger into an independent param: %+v", issues)
	}
}
