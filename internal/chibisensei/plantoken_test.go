package chibisensei

import (
	"testing"
	"time"
)

func TestPlanTokens(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	p := NewPlanTokens(func() time.Time { return now })
	tok, ttl := p.Issue(PlanTicket{ProjectID: "p1", RunID: "r1", Preset: "quick", Hash: "h"})
	if len(tok) != 32 || ttl != PlanTokenTTL {
		t.Fatalf("issue: %q %v", tok, ttl)
	}
	if _, ok := p.Consume(tok, "p2"); ok {
		t.Fatal("wrong project accepted")
	}
	if _, ok := p.Consume(tok, "p1"); ok {
		t.Fatal("token survived a failed consume")
	}
	tok, _ = p.Issue(PlanTicket{ProjectID: "p1", RunID: "r1", Hash: "h"})
	got, ok := p.Consume(tok, "p1")
	if !ok || got.RunID != "r1" || got.Hash != "h" || !got.Expires.Equal(now.Add(PlanTokenTTL)) {
		t.Fatalf("consume: %+v %v", got, ok)
	}
	if _, ok := p.Consume(tok, "p1"); ok {
		t.Fatal("token consumed twice")
	}
	tok, _ = p.Issue(PlanTicket{ProjectID: "p1"})
	now = now.Add(PlanTokenTTL + time.Second)
	if _, ok := p.Consume(tok, "p1"); ok {
		t.Fatal("expired token accepted")
	}
	first, _ := p.Issue(PlanTicket{ProjectID: "p1"})
	for i := 0; i < maxPlanTokens; i++ {
		now = now.Add(time.Millisecond)
		p.Issue(PlanTicket{ProjectID: "p1"})
	}
	if _, ok := p.Consume(first, "p1"); ok {
		t.Fatal("the earliest-expiring token was not evicted")
	}
}

func TestPlanHash(t *testing.T) {
	plan := Plan{Preset: "quick", ProfileID: "p", Hyper: Hyper{Epochs: 2}, DataRev: 3}
	base := PlanHash(plan, "ps", "ss", "tester", "NvidiaTeslaT4", true)
	if len(base) != 64 || base != PlanHash(plan, "ps", "ss", "tester", "NvidiaTeslaT4", true) {
		t.Fatalf("hash %q", base)
	}
	rev, epochs := plan, plan
	rev.DataRev++
	epochs.Hyper.Epochs++
	for name, h := range map[string]string{
		"data_rev":    PlanHash(rev, "ps", "ss", "tester", "NvidiaTeslaT4", true),
		"epochs":      PlanHash(epochs, "ps", "ss", "tester", "NvidiaTeslaT4", true),
		"profile sha": PlanHash(plan, "ps2", "ss", "tester", "NvidiaTeslaT4", true),
		"script sha":  PlanHash(plan, "ps", "ss2", "tester", "NvidiaTeslaT4", true),
		"owner":       PlanHash(plan, "ps", "ss", "other", "NvidiaTeslaT4", true),
		"shape":       PlanHash(plan, "ps", "ss", "tester", "NvidiaL4", true),
		"delete flag": PlanHash(plan, "ps", "ss", "tester", "NvidiaTeslaT4", false),
	} {
		if h == base {
			t.Errorf("%s did not change the hash", name)
		}
	}
}
