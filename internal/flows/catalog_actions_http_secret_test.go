package flows

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// secretCallFlow is an email trigger (untrusted) feeding an http.request with params.
func secretCallFlow(params map[string]any) (*Flow, string) {
	b := newFlow("Secret call")
	tr := b.node("mail", TypeTriggerEmail, nil)
	call := b.node("call", TypeHTTPRequest, params)
	b.edge(tr, PortOut, call)
	return b.build(), call
}

// A secret_ref parameter names a vault entry, so it is written out: a template in it is a
// publish error (a warning in a draft) for every node type, and http.request refuses one at
// run time as well, before it reads the vault, even though the run resolved the template
// to a plain name. Data from a mail must not choose which secret is sent.
func TestSecretRefParamRefusesATemplate(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	f, call := secretCallFlow(map[string]any{"url": "https://api.example.org/v1", "auth_secret": "{{trigger.data.subject}}"})
	for mode, want := range map[string]struct {
		vc  ValidateContext
		sev Severity
	}{"publish": {publishCtx(), SeverityError}, "draft": {draftCtx(), SeverityWarning}} {
		is := findIssue(Validate(f, reg, want.vc), IssueParamInvalid, call)
		if is == nil || is.Param != "auth_secret" || is.Severity != want.sev {
			t.Errorf("%s: templated auth_secret issue = %+v, want %s on auth_secret", mode, is, want.sev)
		}
	}
	f, call = secretCallFlow(map[string]any{"url": "https://api.example.org/v1", "auth_secret": "partner_token"})
	for _, is := range Validate(f, reg, publishCtx()) {
		if is.NodeID == call && is.Severity == SeverityError {
			t.Errorf("a plain secret name was refused: %+v", is)
		}
	}

	// Run time: the raw parameter is the template, the resolved one a valid name.
	def := httpNodeDef(t)
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "{}"))}
	node := &Node{ID: testNodeID(1), Key: "call", Type: TypeHTTPRequest,
		Params: map[string]any{"url": "https://api.example.org/v1", "auth_secret": "{{trigger.data.subject}}"}}
	_, err := def.Execute(context.Background(), ExecInput{Node: node,
		Params:   withDefaults(def, map[string]any{"url": "https://api.example.org/v1", "auth_secret": "partner_token"}),
		Services: &Services{Tools: tools, Secrets: fakeSecrets{"partner_token": "tok_" + secretMarker}}})
	wantParamInvalid(t, "templated auth_secret", tools, err, "partner_token", secretMarker)
	if ne := asNodeError(err); ne == nil || !hopeless(err) || !strings.Contains(ne.Message, "auth_secret") {
		t.Errorf("error = %v; want a non-retryable refusal that names auth_secret", err)
	}
}

// Where a vault secret is sent must not depend on outside data: with an auth_secret set,
// untrusted data in the url or the headers is an error when publishing (a warning in a
// draft, like every publish rule), with its own code, and Publish refuses the flow. The
// same flow without a secret keeps the plain warning. Test runs validate with draft
// rules, so they only see the warning.
func TestUntrustedDataCannotChooseWhereASecretGoes(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, c := range []struct {
		name, param string
		params      map[string]any
	}{
		{"url", "url", map[string]any{"url": "{{trigger.data.link}}", "auth_secret": "partner_token"}},
		{"part of the url", "url", map[string]any{"url": "https://{{trigger.data.host}}/v1", "auth_secret": "partner_token"}},
		{"headers", "headers", map[string]any{"url": "https://api.example.org/v1", "headers": map[string]any{"X-Next": "{{trigger.data.link}}"}, "auth_secret": "partner_token"}},
	} {
		f, call := secretCallFlow(c.params)
		issues := Validate(f, reg, publishCtx())
		is := findIssue(issues, IssueUntrustedSecretTarget, call)
		if is == nil || is.Severity != SeverityError || is.Param != c.param {
			t.Errorf("%s: publish issue = %+v, want an error on %s", c.name, is, c.param)
			continue
		}
		if !strings.Contains(is.Message, "fixed") {
			t.Errorf("%s: message = %q; want it to say what to do", c.name, is.Message)
		}
		for _, other := range issues {
			if other.Code == IssueUntrustedData && other.NodeID == call && other.Param == c.param {
				t.Errorf("%s: the same parameter is reported twice: %+v and %+v", c.name, is, other)
			}
		}
		if is := findIssue(Validate(f, reg, draftCtx()), IssueUntrustedSecretTarget, call); is == nil || is.Severity != SeverityWarning {
			t.Errorf("%s: draft issue = %+v, want a warning", c.name, is)
		}

		// Publish refuses it, with the issue in the error.
		s, _ := newServiceFixture(t, nil)
		rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: f})
		if err != nil {
			t.Fatalf("%s: CreateFlow: %v", c.name, err)
		}
		_, found, err := s.Publish(context.Background(), rec.ID, rec.DraftRevision)
		var ve *ValidationError
		if !errors.As(err, &ve) || findIssue(found, IssueUntrustedSecretTarget, "") == nil {
			t.Errorf("%s: Publish = %v, %+v; want a validation error with %s", c.name, err, found, IssueUntrustedSecretTarget)
		}
	}

	// Without a secret the data still only draws the plain warning, and outside data that
	// reaches neither the url nor the headers does not matter for the secret.
	for name, params := range map[string]map[string]any{
		"no secret":    {"url": "{{trigger.data.link}}"},
		"blank secret": {"url": "{{trigger.data.link}}", "auth_secret": "  "},
	} {
		f, call := secretCallFlow(params)
		issues := Validate(f, reg, publishCtx())
		if is := findIssue(issues, IssueUntrustedSecretTarget, call); is != nil {
			t.Errorf("%s: %+v reported", name, is)
		}
		if is := findIssue(issues, IssueUntrustedData, call); is == nil || is.Severity != SeverityWarning || HasErrors(issues) {
			t.Errorf("%s: issue = %+v, errors = %v; want only the plain untrusted-data warning", name, is, HasErrors(issues))
		}
	}
	f, call := secretCallFlow(map[string]any{"method": "POST", "url": "https://api.example.org/v1", "body": "{{trigger.data.text}}", "auth_secret": "partner_token"})
	issues := Validate(f, reg, publishCtx())
	if findIssue(issues, IssueUntrustedSecretTarget, call) != nil || HasErrors(issues) {
		t.Errorf("outside data in the body only: %+v", issues)
	}
}
