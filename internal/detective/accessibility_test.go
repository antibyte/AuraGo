package detective

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Recent retries keep their idempotency keys; older rows are pruned after a successful
// start so the table cannot grow without bound (audit 4.23).
func TestDetectiveKeyRetention(t *testing.T) {
	s := newTestService(t)
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error { return nil }))
	c, err := s.Create(Request{Topic: "Retention", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	for i := 0; i < 60; i++ {
		if _, err = s.db.Exec("INSERT INTO detective_keys(case_id, key, run_id) VALUES(?,?,?)", c.ID, fmt.Sprintf("old-%02d", i), "run_old"); err != nil {
			s.mu.Unlock()
			t.Fatal(err)
		}
	}
	s.mu.Unlock()
	if _, err = s.Start(c.ID, "start", "quick", "fresh-key"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM detective_keys WHERE case_id=?", c.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 50 {
		t.Fatalf("kept %d keys, want 50", count)
	}
	var one int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM detective_keys WHERE case_id=? AND key='fresh-key'", c.ID).Scan(&one); err != nil {
		t.Fatal(err)
	}
	if one != 1 {
		t.Fatal("the newest key was pruned")
	}
	var oldest int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM detective_keys WHERE case_id=? AND key='old-00'", c.ID).Scan(&oldest); err != nil {
		t.Fatal(err)
	}
	if oldest != 0 {
		t.Fatal("the oldest key survived")
	}
}

// The store names what it accepts and leaves reachability to the tool layer (audit 4.23).
func TestDetectiveSourceURLErrorMessage(t *testing.T) {
	for _, raw := range []string{"ftp://example.org/report", "https://user:pass@example.org/report", "not a url"} {
		_, err := canonicalURL(raw)
		if err == nil {
			t.Fatalf("%q was accepted", raw)
		}
		if !strings.Contains(err.Error(), "HTTP(S) URL without credentials") {
			t.Fatalf("%q: %v", raw, err)
		}
		if strings.Contains(err.Error(), "public") {
			t.Fatalf("%q still claims public reachability: %v", raw, err)
		}
	}
	if got, err := canonicalURL("https://example.org/report#frag"); err != nil || got != "https://example.org/report" {
		t.Fatalf("canonicalURL = %q, %v", got, err)
	}
}
