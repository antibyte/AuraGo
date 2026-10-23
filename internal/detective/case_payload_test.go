package detective

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// The working set carries source metadata only; the excerpt stays in the archive and is
// returned to the tool that recorded it (audit 4.1/4.3).
func TestRecordSourceKeepsExcerptInArchiveOnly(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "sources", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	c.Run.Status = "running"
	c.Run.Profile = Profiles()["quick"]
	if err = s.saveLocked(&c); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()
	x := &Session{service: s, CaseID: c.ID}
	excerpt := strings.Repeat("quoted measurement 42 units. ", 2000)
	src, err := x.RecordSource("https://example.org/p", "P", "web_scraper", excerpt, true)
	if err != nil {
		t.Fatal(err)
	}
	if src.Excerpt == "" {
		t.Fatal("the tool receipt lost its excerpt")
	}
	archived, err := s.Source(c.ID, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Excerpt != bounded(excerpt, 32000) {
		t.Fatal("the archive lost the excerpt")
	}
	got, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || got.Sources[0].Excerpt != "" {
		t.Fatalf("working set kept the excerpt: %+v", got.Sources)
	}
	var body []byte
	if err = s.db.QueryRow("SELECT body FROM detective_cases WHERE id=?", c.ID).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("quoted measurement")) {
		t.Fatal("the case body kept evidence text")
	}
}

// A maximum case with full-size excerpts keeps a small working-set body, so state
// writes and the case payload stay bounded (audit 4.1).
func TestCaseBodyStaysBounded(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "bounded", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	c.Run.Status = "running"
	c.Run.Profile = Profiles()["maximum"]
	if err = s.saveLocked(&c); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()
	x := &Session{service: s, CaseID: c.ID}
	for i := 0; i < 200; i++ {
		if _, err = x.RecordSource(fmt.Sprintf("https://example.org/%d", i), "Title", "web_scraper", strings.Repeat("x", 20000), true); err != nil {
			t.Fatal(err)
		}
	}
	var size int
	if err = s.db.QueryRow("SELECT length(body) FROM detective_cases WHERE id=?", c.ID).Scan(&size); err != nil {
		t.Fatal(err)
	}
	if size > 256<<10 {
		t.Fatalf("case body grew to %d bytes", size)
	}
	got, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 200 || got.SourceCount != 200 {
		t.Fatalf("sources=%d count=%d", len(got.Sources), got.SourceCount)
	}
	for _, source := range got.Sources {
		if source.Excerpt != "" {
			t.Fatal("a source kept its excerpt in the working set")
		}
	}
}
