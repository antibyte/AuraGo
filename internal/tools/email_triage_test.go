package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
	"aurago/internal/security"
)

type fakeTriage struct {
	calls   int
	pick    func(state string, call int) (string, error)
	offline bool
}

func (f *fakeTriage) Snapshot(_ *config.Config, id consumers.ID) consumers.Snapshot {
	if f.offline {
		return consumers.Snapshot{Consumer: id, Code: consumers.CodeConsumerDisabled}
	}
	return consumers.Snapshot{Consumer: id, Available: true, Code: consumers.CodeAvailable,
		Current: func() bool { return true },
		Evaluate: func(_ context.Context, req decisions.Request) (decisions.Response, error) {
			f.calls++
			choice, err := f.pick(fmt.Sprint(req.State), f.calls)
			if err != nil {
				return decisions.Response{}, err
			}
			probs := map[string]float64{}
			for _, o := range emailTriageOptions {
				probs[o.ID] = 0.05
			}
			probs[choice] = 0.85
			return decisions.Response{Answers: map[string]decisions.Answer{"triage": {Type: "choice", Decision: choice, Probabilities: probs}}}, nil
		}}
}

func triageFixture(t *testing.T, n int, svc EmailTriageService) (*EmailWatcher, config.EmailAccount, []emailSummaryItem) {
	t.Helper()
	w := NewEmailWatcher(&config.Config{}, slog.Default(), security.NewGuardian(nil), nil)
	if svc != nil {
		w.SetTriageService(svc, nil)
	}
	acct := config.EmailAccount{ID: "a", Name: "Main", FromAddress: "me@example.com", WatchFolder: "INBOX"}
	var msgs []EmailMessage
	for i := 1; i <= n; i++ {
		msgs = append(msgs, EmailMessage{UID: uint32(i), From: "s@example.com", Subject: "subject-" + string(rune(65+i-1)), Body: "plain body"})
	}
	items, _, _ := w.collectMessages(context.Background(), acct, msgs)
	return w, acct, items
}

func TestEmailTriageOffKeepsFlatSummary(t *testing.T) {
	for name, svc := range map[string]EmailTriageService{"none": nil, "disabled": &fakeTriage{offline: true}} {
		w, acct, items := triageFixture(t, 3, svc)
		groups := w.triageItems(context.Background(), items)
		got := buildEmailNotificationPromptGrouped(acct, 3, items, groups)
		want := buildEmailNotificationPrompt(acct, 3, flatEmailSummary(items))
		if got != want || groups != nil {
			t.Fatalf("%s: prompt differs from flat summary", name)
		}
	}
}

func TestEmailTriageGroupsUrgentFirst(t *testing.T) {
	svc := &fakeTriage{pick: func(state string, _ int) (string, error) {
		switch {
		case strings.Contains(state, "subject-B"):
			return "urgent_reply", nil
		case strings.Contains(state, "subject-A"):
			return "automated", nil
		}
		return "fyi", nil
	}}
	w, acct, items := triageFixture(t, 3, svc)
	prompt := buildEmailNotificationPromptGrouped(acct, 3, items, w.triageItems(context.Background(), items))
	iB, iC, iA := strings.Index(prompt, "subject-B"), strings.Index(prompt, "subject-C"), strings.Index(prompt, "subject-A")
	if !(iB > 0 && iB < iC && iC < iA) {
		t.Fatalf("order wrong: %d %d %d\n%s", iB, iC, iA, prompt)
	}
	for _, s := range []string{"subject-A", "subject-B", "subject-C"} {
		if strings.Count(prompt, s) != 1 {
			t.Fatalf("%s appears %d times", s, strings.Count(prompt, s))
		}
	}
	if strings.Count(prompt, "<external_data>") != 3 {
		t.Fatalf("each group should be isolated: %s", prompt)
	}
}

func TestEmailTriageNeverSeesQuarantined(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLMGuardian.ScanEmails = true
	svc := &fakeTriage{pick: func(string, int) (string, error) { return "fyi", nil }}
	w := NewEmailWatcher(cfg, slog.Default(), security.NewGuardian(nil), nil)
	w.llmGuardian = &emailWatcherTestEvaluator{result: security.GuardianResult{Decision: security.DecisionQuarantine, QuarantineReason: security.QuarantineIncomplete}}
	w.SetTriageService(svc, nil)
	items, quarantined, _ := w.collectMessages(context.Background(), config.EmailAccount{ID: "a", WatchFolder: "INBOX"}, []EmailMessage{{UID: 1, Subject: "q"}})
	if groups := w.triageItems(context.Background(), items); len(items) != 0 || len(quarantined) != 1 || svc.calls != 0 || groups != nil {
		t.Fatalf("quarantined mail reached triage: items=%d calls=%d", len(items), svc.calls)
	}
}

func TestEmailTriageDeferredKeepsOrder(t *testing.T) {
	svc := &fakeTriage{pick: func(_ string, call int) (string, error) {
		if call >= 2 {
			return "", errors.New(consumers.CodeAdmissionDeferred)
		}
		return "automated", nil
	}}
	w, acct, items := triageFixture(t, 3, svc)
	prompt := buildEmailNotificationPromptGrouped(acct, 3, items, w.triageItems(context.Background(), items))
	if svc.calls != 2 {
		t.Fatalf("calls = %d, want 2", svc.calls)
	}
	iA, iB, iC := strings.Index(prompt, "subject-A"), strings.Index(prompt, "subject-B"), strings.Index(prompt, "subject-C")
	un := strings.Index(prompt, "Not classified:")
	if !(iA > 0 && iA < un && un < iB && iB < iC) {
		t.Fatalf("unexpected layout: %s", prompt)
	}
}

func TestEmailTriageCapsPerPoll(t *testing.T) {
	svc := &fakeTriage{pick: func(string, int) (string, error) { return "fyi", nil }}
	w, acct, items := triageFixture(t, 20, svc)
	prompt := buildEmailNotificationPromptGrouped(acct, 20, items, w.triageItems(context.Background(), items))
	if svc.calls > emailTriageMaxPerPoll || svc.calls == 0 {
		t.Fatalf("calls = %d", svc.calls)
	}
	if !strings.Contains(prompt, "subject-T") {
		t.Fatalf("mail 20 was dropped")
	}
}

type cfgGatedTriage struct {
	fakeTriage
	enabled *config.Config
}

func (g *cfgGatedTriage) Snapshot(cfg *config.Config, id consumers.ID) consumers.Snapshot {
	g.offline = cfg != g.enabled
	return g.fakeTriage.Snapshot(cfg, id)
}

func TestEmailTriageReadsLiveConfig(t *testing.T) {
	startCfg, liveCfg := &config.Config{}, &config.Config{}
	svc := &cfgGatedTriage{enabled: liveCfg}
	svc.pick = func(string, int) (string, error) { return "fyi", nil }
	w, _, items := triageFixture(t, 1, nil)
	w.cfg = startCfg
	current := startCfg
	w.SetTriageService(svc, func() *config.Config { return current })
	if w.triageItems(context.Background(), items) != nil || svc.calls != 0 {
		t.Fatal("triage ran while off")
	}
	current = liveCfg
	if w.triageItems(context.Background(), items) == nil || svc.calls != 1 {
		t.Fatalf("triage did not pick up the live config, calls=%d", svc.calls)
	}
}
