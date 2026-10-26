package detective

import (
	"context"
	"testing"
	"time"
)

// An answer appears in the activity feed, so a window that was closed while the question
// was open still shows what the user replied.
func TestAnswerIsAnActivityEvent(t *testing.T) {
	s := newTestService(t)
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error {
		_, done, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		return job.Wait("Which year should the report cover?")
	}))
	c, err := s.Create(Request{Topic: "Answers", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Start(c.ID, "start", "quick", "answers-1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, getErr := s.Get(c.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		// The worker clears its active slot after the terminal write; a new run needs both.
		s.mu.Lock()
		idle := s.active == nil
		s.mu.Unlock()
		if idle && got.Run.Status == "waiting_for_user" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the run never asked its question")
		}
		time.Sleep(10 * time.Millisecond)
	}
	const answer = "Use the 2024 report and ignore the draft"
	if _, err = s.Start(c.ID, "continue", "", "answers-2", answer); err != nil {
		t.Fatal(err)
	}
	events, err := s.Events(c.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Kind == "answer" {
			found = true
			if e.Text != answer {
				t.Fatalf("answer event text=%q", e.Text)
			}
		}
	}
	if !found {
		t.Fatalf("no answer event: %+v", events)
	}
	got, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Answers) != 1 || got.Answers[0] != answer {
		t.Fatalf("answers=%v", got.Answers)
	}
}
