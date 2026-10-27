package flows

import "testing"

// urlFlow is an email trigger (untrusted) feeding an ai.step "ask" (untrusted output), a
// logic.set "cfg" that holds a fixed base address (trusted) and a node of typ with params.
func urlFlow(typ string, params map[string]any) (*Flow, string) {
	b := newFlow("Address from data")
	tr := b.node("mail", TypeTriggerEmail, nil)
	ask := b.node("ask", TypeAIStep, map[string]any{"prompt": "Summarise {{trigger.data.subject}}"})
	cfg := b.node("cfg", TypeSet, map[string]any{"fields": []any{
		map[string]any{"name": "base", "type": "text", "value": "https://api.example.org"}}})
	call := b.node("call", typ, params)
	b.edge(tr, PortOut, ask)
	b.edge(ask, PortOut, cfg)
	b.edge(cfg, PortOut, call)
	return b.build(), call
}

// Untrusted data that can reach the scheme, the user info, the host or the port of a web
// address is a publish error scoped to that parameter (a warning in a draft, like every
// publish rule), with or without a secret: the node connects where the data says. Data in
// the path, the query, the fragment or the body keeps the plain warning.
func TestLintUntrustedHostPositionIsError(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, c := range []struct {
		name, url string
		wantError bool
	}{
		{"the whole address", "{{ask.text}}", true},
		{"leading blanks", "  {{ask.text}}", true},
		{"the host", "https://{{ask.text}}/x", true},
		{"data glued to the host", "https://api.example.org{{ask.text}}", true},
		{"the port", "https://api.example.org:{{ask.text}}/x", true},
		{"the user info", "https://u{{ask.text}}@api.example.org/x", true},
		{"the scheme", "{{ask.text}}://api.example.org/x", true},
		{"data before a fixed path", "{{ask.text}}/items", true},
		{"a trusted base then data glued to it", "{{cfg.base}}{{ask.text}}", true},
		{"the path", "https://api.example.org/{{ask.text}}", false},
		{"the query", "https://api.example.org/x?q={{ask.text}}", false},
		{"the query of a bare host", "https://api.example.org?q={{ask.text}}", false},
		{"the fragment", "https://api.example.org/x#{{ask.text}}", false},
		{"a trusted base then data in the path", "{{cfg.base}}/items/{{ask.text}}", false},
	} {
		for _, typ := range []string{TypeHTTPRequest, TypeWebRead} {
			f, call := urlFlow(typ, map[string]any{"url": c.url})
			issues := Validate(f, reg, publishCtx())
			target := findIssue(issues, IssueUntrustedURLTarget, call)
			if c.wantError {
				if target == nil || target.Severity != SeverityError || target.Param != "url" {
					t.Errorf("%s/%s: url target issue = %+v, want an error on url", c.name, typ, target)
					continue
				}
				// One finding per parameter and source: the plain warning is not added on top.
				for _, other := range issues {
					if other.Code == IssueUntrustedData && other.NodeID == call && other.Param == "url" {
						t.Errorf("%s/%s: the same parameter is reported twice: %+v and %+v", c.name, typ, target, other)
					}
				}
				if is := findIssue(Validate(f, reg, draftCtx()), IssueUntrustedURLTarget, call); is == nil || is.Severity != SeverityWarning {
					t.Errorf("%s/%s: draft issue = %+v, want a warning", c.name, typ, is)
				}
				continue
			}
			if target != nil {
				t.Errorf("%s/%s: %+v reported, want only the plain warning", c.name, typ, target)
			}
			warn := findIssue(issues, IssueUntrustedData, call)
			if warn == nil || warn.Severity != SeverityWarning || warn.Param != "url" {
				t.Errorf("%s/%s: warning = %+v, want the plain warning on url", c.name, typ, warn)
			}
			if HasErrors(issues) {
				t.Errorf("%s/%s: errors = %+v", c.name, typ, issues)
			}
		}
	}
}

// Data in the body of a request, or in a header, stays a warning whatever it holds, and a
// secret target keeps its own code instead of getting a second finding.
func TestLintUntrustedBodyStaysAWarningAndSecretTargetKeepsItsCode(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())

	f, call := urlFlow(TypeHTTPRequest, map[string]any{"method": "POST", "url": "https://api.example.org/v1", "body": "{{ask.text}}"})
	issues := Validate(f, reg, publishCtx())
	if is := findIssue(issues, IssueUntrustedData, call); is == nil || is.Severity != SeverityWarning || is.Param != "body" {
		t.Errorf("body: issue = %+v, want the plain warning on body", is)
	}
	if HasErrors(issues) {
		t.Errorf("body: %+v", issues)
	}

	f, call = urlFlow(TypeHTTPRequest, map[string]any{"url": "{{ask.text}}", "auth_secret": "partner_token"})
	issues = Validate(f, reg, publishCtx())
	if is := findIssue(issues, IssueUntrustedSecretTarget, call); is == nil || is.Severity != SeverityError {
		t.Errorf("secret: issue = %+v, want the secret target error", is)
	}
	if is := findIssue(issues, IssueUntrustedURLTarget, call); is != nil {
		t.Errorf("secret: %+v reported next to the secret target error", is)
	}
}

// The other sinks keep the 2026-10-25 decision: untrusted data in a command, a path or
// any other non-address sink is a warning, never an error.
func TestLintOtherSinksStayWarnings(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	f, call := urlFlow(TypeFileWrite, map[string]any{"path": "{{ask.text}}", "content": "x"})
	issues := Validate(f, reg, publishCtx())
	if is := findIssue(issues, IssueUntrustedData, call); is == nil || is.Severity != SeverityWarning {
		t.Errorf("path: issue = %+v, want the plain warning", is)
	}
	if is := findIssue(issues, IssueUntrustedURLTarget, call); is != nil {
		t.Errorf("path: %+v is no web address", is)
	}
}

// A generic tool node treats a text parameter named url, uri, endpoint or *_url as a web
// address. Plural names hold lists, JSON parameters are not text, and a parameter that is
// not a sink is left alone.
func TestGenericURLParamsAreURLTargets(t *testing.T) {
	schema := genericSchemaOf(map[string]any{
		"url": genericProp("string", ""), "uri": genericProp("string", ""), "endpoint": genericProp("string", ""),
		"callback_url": genericProp("string", ""), "urls": genericProp("array", ""),
		"notes": genericProp("string", ""), "target": genericProp("string", ""),
	})
	params, _ := paramsFromSchema(schema)
	got := map[string]bool{}
	for _, p := range params {
		got[p.Name] = p.URLTarget
		if p.URLTarget && !p.SensitiveSink {
			t.Errorf("%s is a URLTarget without being a sink", p.Name)
		}
	}
	for name, want := range map[string]bool{"url": true, "uri": true, "endpoint": true, "callback_url": true,
		"urls": false, "notes": false, "target": false} {
		if got[name] != want {
			t.Errorf("%s: URLTarget = %v, want %v", name, got[name], want)
		}
	}
}

// An address that reads nothing untrusted, a trusted base or an escaped brace, draws no finding.
func TestLintURLWithoutUntrustedDataIsQuiet(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, url := range []string{"{{cfg.base}}/items", `https://api.example.org/\{{ask.text}}`, "https://api.example.org/v1"} {
		f, call := urlFlow(TypeHTTPRequest, map[string]any{"url": url})
		for _, is := range Validate(f, reg, publishCtx()) {
			if is.NodeID == call && (is.Code == IssueUntrustedURLTarget || is.Code == IssueUntrustedData) {
				t.Errorf("%s: %+v reported", url, is)
			}
		}
	}
}
