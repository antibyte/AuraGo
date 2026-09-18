package detective

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestResearchProfilesTokenReserveAndDurableRestart(t *testing.T) {
	for _, effort := range []string{"quick", "normal", "maximum"} {
		t.Run(effort, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "case.db")
			s, err := New(Options{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			c, _ := s.Create(Request{Topic: "Token boundary", Effort: effort})
			c.Run = Run{ID: "r", Status: "running", Profile: Profiles()[effort]}
			c.Run.Profile.Tokens = 10000
			c.Run.Usage.PromptTokens = 7900
			s.mu.Lock()
			_ = s.saveLocked(&c)
			s.mu.Unlock()
			x := &Session{service: s, CaseID: c.ID}
			if limit, err := x.BeforeRequest(1000, 4000); err != nil || limit != 744 {
				t.Fatalf("output reserve=%d %v", limit, err)
			}
			if err = x.RecordUsage(1000, 500, 100); err != nil {
				t.Fatal(err)
			}
			if err = x.Iterate(); err != nil {
				t.Fatal(err)
			}
			if err = x.BeforeTool("web_scraper"); err == nil {
				t.Fatal("research entered synthesis reserve")
			}
			if _, err = x.BeforeRequest(1000, 4000); !errors.Is(err, ErrBudget) {
				t.Fatal("next prompt overran tokens", err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = New(Options{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			c, _ = s.Get(c.ID)
			if c.Run.Status != "interrupted" || c.Run.Usage.PromptTokens != 8900 || c.Run.Usage.CachedTokens != 100 || c.Run.Usage.Requests != 1 {
				t.Fatalf("lost restart ledger: %+v", c.Run)
			}
		})
	}
}

func TestResearchWaitingTimeNotChargedAndAnswerIsIdempotent(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "Question", Effort: "quick"})
	s.mu.Lock()
	c.Run = Run{ID: "run", Status: "waiting_for_user", Profile: Profiles()["quick"], Usage: Usage{ActiveMS: 5000}}
	_ = s.saveLocked(&c)
	s.mu.Unlock()
	entered := make(chan struct{})
	release := make(chan struct{})
	s.SetRunner(testRunner(func(ctx context.Context, x *Session) error {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		return context.Canceled
	}))
	if _, err := s.Start(c.ID, "continue", "", "reply", "A clarification"); err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err := s.Start(c.ID, "continue", "", "reply", "A clarification"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	c, _ = s.Get(c.ID)
	if len(c.Answers) != 1 || c.Run.Usage.ActiveMS != 5000 {
		t.Fatalf("duplicate answer or queue charge: %+v", c)
	}
	close(release)
	waitCase(t, s, c.ID, func(c Case) bool { return c.Run.Status == "failed" })
}
