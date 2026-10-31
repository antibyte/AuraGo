package webhooks

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
	"aurago/internal/security"
)

type fakeDecisionService struct {
	yes   float64
	fail  bool
	calls int
}

func (f *fakeDecisionService) Snapshot(_ *config.Config, id consumers.ID) consumers.Snapshot {
	return consumers.Snapshot{Consumer: id, Available: true, Code: consumers.CodeAvailable,
		Current: func() bool { return true },
		Evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
			f.calls++
			if f.fail {
				return decisions.Response{}, context.DeadlineExceeded
			}
			p := f.yes
			return decisions.Response{Answers: map[string]decisions.Answer{"answer": {Type: "noul", Decision: "yes",
				Probabilities: map[string]float64{"yes": p, "no": 1 - p}, Noul: &p}}}, nil
		}}
}

func TestWebhookDecisionValidation(t *testing.T) {
	m, err := NewManager(filepath.Join(t.TempDir(), "w.json"), filepath.Join(t.TempDir(), "w.log"))
	if err != nil {
		t.Fatal(err)
	}
	mk := func(d *DeliveryDecision) Webhook {
		return Webhook{Name: "n" + strconv.Itoa(len(m.List())), Slug: "hook-" + strconv.Itoa(len(m.List())), Delivery: DeliveryConfig{Mode: DeliveryModeMessage, Decision: d}}
	}
	for _, bad := range []*DeliveryDecision{
		{Enabled: true, Question: "   "},
		{Enabled: true, Question: strings.Repeat("x", 501)},
		{Enabled: true, Question: "ok?", Mode: "block"},
	} {
		if _, err := m.Create(mk(bad)); err == nil {
			t.Fatalf("expected error for %+v", bad)
		}
	}
	created, err := m.Create(mk(&DeliveryDecision{Enabled: true, Question: "  Needs me?  "}))
	if err != nil {
		t.Fatal(err)
	}
	if d := created.Delivery.Decision; d.Mode != DecisionModeHint || d.Question != "Needs me?" {
		t.Fatalf("decision = %+v", d)
	}
	if _, err := m.Update(created.ID, Webhook{Delivery: DeliveryConfig{Decision: &DeliveryDecision{Enabled: true, Question: "q", Mode: "nope"}}}); err == nil {
		t.Fatal("update accepted bad mode")
	}
	updated, err := m.Update(created.ID, Webhook{Delivery: DeliveryConfig{Decision: &DeliveryDecision{Enabled: true, Question: "q", Mode: DecisionModeGate}}})
	if err != nil || updated.Delivery.Decision.Mode != DecisionModeGate {
		t.Fatalf("update = %+v, %v", updated.Delivery.Decision, err)
	}
}

type decisionRun struct {
	prompts   []string
	entries   []LogEntry
	callbacks int
}

func runDecisionWebhook(t *testing.T, d *DeliveryDecision, svc consumers.Service) decisionRun {
	t.Helper()
	var mu sync.Mutex
	var run decisionRun
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(b, &req)
		mu.Lock()
		for _, msg := range req.Messages {
			run.prompts = append(run.prompts, msg.Content)
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	tm, err := security.NewTokenManager(vault, filepath.Join(t.TempDir(), "tokens.bin"))
	if err != nil {
		t.Fatal(err)
	}
	raw, meta, err := tm.Create("t", []string{"webhook"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(filepath.Join(t.TempDir(), "w.json"), filepath.Join(t.TempDir(), "w.log"))
	if err != nil {
		t.Fatal(err)
	}
	wh, err := m.Create(Webhook{Name: "Test Hook", Slug: "test-hook", Enabled: true, TokenID: meta.ID,
		Format:   WebhookFormat{AcceptedContentTypes: []string{"application/json"}},
		Delivery: DeliveryConfig{Mode: DeliveryModeMessage, Decision: d}})
	if err != nil {
		t.Fatal(err)
	}
	m.RegisterMissionTrigger(wh.ID, func([]byte) { mu.Lock(); run.callbacks++; mu.Unlock() })
	cfg := &config.Config{}
	cfg.Server.Port = port
	h := NewHandler(m, tm, vault, nil, nil, cfg, slog.Default(), port, 4096, 0)
	h.SetInternalToken("tok")
	if svc != nil {
		h.SetDecisionService(svc)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook/test-hook", strings.NewReader(`{"event":"push"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e := m.GetLog().Recent(10); len(e) > 0 {
			run.entries = e
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	return run
}

func joined(r decisionRun) string { return strings.Join(r.prompts, "\n") }

func TestWebhookDecisionHintAppendsResult(t *testing.T) {
	run := runDecisionWebhook(t, &DeliveryDecision{Enabled: true, Question: "Needs my attention?"}, &fakeDecisionService{yes: 0.9})
	if !strings.Contains(joined(run), "Decisions: Yes (0.90, uncalibrated) - Needs my attention?") {
		t.Fatalf("hint missing: %q", joined(run))
	}
}

func TestWebhookDecisionGateSuppressesOnConfidentNo(t *testing.T) {
	run := runDecisionWebhook(t, &DeliveryDecision{Enabled: true, Question: "Needs me?", Mode: DecisionModeGate}, &fakeDecisionService{yes: 0.05})
	if len(run.prompts) != 0 {
		t.Fatalf("agent message delivered: %q", joined(run))
	}
	if len(run.entries) != 1 || run.entries[0].Error != DecisionSuppressedError || run.entries[0].Delivered {
		t.Fatalf("log = %+v", run.entries)
	}
	if run.callbacks != 1 {
		t.Fatalf("mission callbacks = %d, want 1", run.callbacks)
	}
}

func TestWebhookDecisionGateDeliversWhenUnsure(t *testing.T) {
	run := runDecisionWebhook(t, &DeliveryDecision{Enabled: true, Question: "Needs me?", Mode: DecisionModeGate}, &fakeDecisionService{yes: 0.3})
	if len(run.prompts) == 0 || strings.Contains(joined(run), "Decisions:") {
		t.Fatalf("expected plain delivery: %q", joined(run))
	}
}

func TestWebhookDecisionOffDelivers(t *testing.T) {
	svc := &fakeDecisionService{yes: 0.01}
	for _, d := range []*DeliveryDecision{nil, {Enabled: false, Question: "q", Mode: DecisionModeGate}} {
		run := runDecisionWebhook(t, d, svc)
		if len(run.prompts) == 0 || strings.Contains(joined(run), "Decisions:") {
			t.Fatalf("expected plain delivery: %q", joined(run))
		}
	}
	if svc.calls != 0 {
		t.Fatalf("classifier called %d times while off", svc.calls)
	}
	run := runDecisionWebhook(t, &DeliveryDecision{Enabled: true, Question: "q", Mode: DecisionModeGate}, &fakeDecisionService{fail: true})
	if len(run.prompts) == 0 {
		t.Fatal("classifier error must still deliver")
	}
}
