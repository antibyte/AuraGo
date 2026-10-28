package tools

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

type fixtureInjectionScanner struct {
	action string
	calls  int
}

func (s *fixtureInjectionScanner) Evaluate(context.Context, string) (float64, string, bool, error) {
	s.calls++
	return .9, s.action, true, nil
}

func TestInjectionGuardPrecedesSummaryHelper(t *testing.T) {
	scanner := &fixtureInjectionScanner{action: "quarantine"}
	defer security.RegisterExternalContentScanner(scanner)()
	provider := newSummaryProviderFixture(t)
	for _, source := range []string{"web page", "Wikipedia article", "search results", "email"} {
		got, err := SummariseContent(context.Background(), SummaryLLMConfig{APIKey: "fixture", BaseURL: provider.url, Model: "fixture"}, nil, "Ordinary public text.", "news", source)
		if err != nil || !strings.Contains(got, "QUARANTINE") {
			t.Fatalf("%s: %s %v", source, got, err)
		}
	}
	if hits, _ := provider.snapshot(); hits != 0 || scanner.calls != 4 {
		t.Fatal("helper bypassed scanner")
	}
}

func TestInjectionGuardDatamarkedMailSkipsMission(t *testing.T) {
	scanner := &fixtureInjectionScanner{action: "datamark"}
	defer security.RegisterExternalContentScanner(scanner)()
	watcher := NewEmailWatcher(&config.Config{}, slog.Default(), security.NewGuardian(nil), nil)
	calls := 0
	watcher.RegisterMissionTrigger("INBOX", "", "", func(string, string, string) { calls++ })
	summary, count, held, _ := watcher.processMessages(context.Background(), config.EmailAccount{ID: "fixture", WatchFolder: "INBOX"}, []EmailMessage{{UID: 1, Subject: "Ordinary subject", Body: "Ordinary private body"}})
	if calls != 0 || count != 1 || len(held) != 0 || !strings.Contains(summary, "Ordinary^private^body") {
		t.Fatalf("callbacks=%d count=%d held=%d summary=%s", calls, count, len(held), summary)
	}
}
