package flows

import (
	"context"
	"strings"
	"testing"
)

// R3a: auth_header and auth_prefix shape the credential header. While auth_secret is set they
// must be written out: a template is a publish error (a draft warning) and is refused at run
// time before the vault is read, even though the run resolved it to a valid value.
func TestB3AuthHeaderAndPrefixAreLiteralWithASecret(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, param := range []string{"auth_header", "auth_prefix"} {
		f, call := secretCallFlow(map[string]any{"url": "https://api.example.org/v1", "auth_secret": "partner_token", param: "{{trigger.data.subject}}"})
		for mode, want := range map[string]struct {
			vc  ValidateContext
			sev Severity
		}{"publish": {publishCtx(), SeverityError}, "draft": {draftCtx(), SeverityWarning}} {
			issues := Validate(f, reg, want.vc)
			var found *Issue
			for i := range issues {
				if issues[i].Code == IssueParamInvalid && issues[i].NodeID == call && issues[i].Param == param {
					found = &issues[i]
				}
			}
			if found == nil || found.Severity != want.sev || !strings.Contains(found.Message, "written out") {
				t.Errorf("%s %s: issue = %+v, want a %s that says to write it out", param, mode, found, want.sev)
			}
		}
		f, call = secretCallFlow(map[string]any{"url": "https://api.example.org/v1", param: "{{trigger.data.subject}}"})
		for _, is := range Validate(f, reg, publishCtx()) {
			if is.NodeID == call && is.Param == param {
				t.Errorf("%s without a secret: %+v, want no issue", param, is)
			}
		}

		def := httpNodeDef(t)
		tools := &fakeTools{respond: answerWith(apiAnswer(200, "{}"))}
		secrets := &countingSecrets{fakeSecrets: fakeSecrets{"partner_token": "tok_" + secretMarker}}
		resolved := map[string]any{"url": "https://api.example.org/v1", "auth_secret": "partner_token", param: "X-Key"}
		if param == "auth_prefix" {
			resolved[param] = "Token "
		}
		raw := map[string]any{"url": "https://api.example.org/v1", "auth_secret": "partner_token", param: "{{trigger.data.subject}}"}
		_, err := def.Execute(context.Background(), ExecInput{Node: &Node{ID: testNodeID(1), Key: "call", Type: TypeHTTPRequest, Params: raw},
			Params: withDefaults(def, resolved), Services: &Services{Tools: tools, Secrets: secrets}})
		wantParamInvalid(t, param+" from a template", tools, err, secretMarker)
		if secrets.count() != 0 || !hopeless(err) {
			t.Errorf("%s: %d vault reads, non-retryable %v; want the refusal before the vault and no retry", param, secrets.count(), hopeless(err))
		}
	}
}

// R3b: the lint walks in topological order, so a reference to a node on a parallel branch was
// checked before that node was tainted. In a secret target a reference to a node that is not an
// ancestor (TEMPLATE_NOT_UPSTREAM when publishing) now counts as untrusted whatever that node is,
// so a test run of the draft refuses the request instead of sending the secret.
func TestB3NonUpstreamRefInASecretTargetIsUntrusted(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for _, c := range []struct {
		name, branchType string
		branchParams     map[string]any
		ref              string
	}{
		{"an untrusted page on a parallel branch", TypeWebRead, map[string]any{"url": "https://news.example"}, "{{page.content}}"},
		{"a trusted value on a parallel branch", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "next", "value": "https://partner.example"}}}, "{{page.next}}"},
	} {
		b := newFlow("Parallel")
		start := b.node("start", TypeTriggerManual, nil)
		call := b.node("call", TypeHTTPRequest, map[string]any{"url": c.ref, "auth_secret": "partner_token"})
		page := b.node("page", c.branchType, c.branchParams)
		b.edge(start, PortOut, call)
		b.edge(start, PortOut, page)
		f := b.build()
		if is := findIssue(LintUntrustedData(f, reg), IssueUntrustedSecretTarget, call); is == nil || is.Param != "url" {
			t.Errorf("%s: lint issue = %+v, want UNTRUSTED_SECRET_TARGET on url", c.name, is)
		}
		if secretTargetRefusals(f, reg)[call] == nil {
			t.Errorf("%s: a test run would send the secret", c.name)
		}
	}
	b := newFlow("Upstream")
	start := b.node("start", TypeTriggerManual, nil)
	prep := b.node("prep", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "next", "value": "https://partner.example/v1"}}})
	call := b.node("call", TypeHTTPRequest, map[string]any{"url": "{{prep.next}}", "auth_secret": "partner_token"})
	b.edge(start, PortOut, prep)
	b.edge(prep, PortOut, call)
	if is := findIssue(LintUntrustedData(b.build(), reg), IssueUntrustedSecretTarget, call); is != nil {
		t.Errorf("a trusted upstream reference was refused: %+v", is)
	}
}

// R3c: a Home Assistant state or attribute can be set by integrations, MQTT or automations, like
// the home.assistant action's output, so trigger.ha_state is untrusted: with a secret its data
// cannot pick where the secret goes.
func TestB3HAStateTriggerIsUntrusted(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	b := newFlow("HA")
	tr := b.node("door", TypeTriggerHAState, map[string]any{"entity": "binary_sensor.door"})
	call := b.node("call", TypeHTTPRequest, map[string]any{"url": "https://{{trigger.data.new_state}}/hook", "auth_secret": "partner_token"})
	b.edge(tr, PortOut, call)
	if is := findIssue(LintUntrustedData(b.build(), reg), IssueUntrustedSecretTarget, call); is == nil {
		t.Fatal("data of trigger.ha_state chose where the secret goes without an error")
	}
}
