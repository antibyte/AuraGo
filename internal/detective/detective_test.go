package detective

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type testRunner func(context.Context, *Session) error

func (f testRunner) Run(ctx context.Context, s *Session) error { return f(ctx, s) }
func newTestService(t *testing.T) *Service {
	t.Helper()
	s, err := New(Options{Path: filepath.Join(t.TempDir(), "detective.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func waitCase(t *testing.T, s *Service, key string, predicate func(Case) bool) Case {
	t.Helper()
	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		c, err := s.Get(key)
		if err != nil {
			t.Fatal(err)
		}
		s.mu.Lock()
		idle := s.active == nil
		s.mu.Unlock()
		if predicate(c) && idle {
			return c
		}
		time.Sleep(10 * time.Millisecond)
	}
	c, _ := s.Get(key)
	t.Fatalf("case did not settle: %+v", c.Run)
	return c
}

func sampleEvidence(t *testing.T, s *Session) Report {
	t.Helper()
	source, err := s.RecordSource("https://example.org/paper?utm_source=test", "Primary research", "web_scraper", "Measured result: 42 units. Überprüfung.", true)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.AddFinding(Finding{SourceID: source.ID, Text: "The measurement was 42 units.", Quote: "42 units"})
	if err != nil {
		t.Fatal(err)
	}
	return Report{Title: "Überprüfung – Research", Summary: "A measured result", Blocks: []Block{{Type: "paragraph", Text: f.Text, Evidence: []string{f.ID}}, {Type: "table", Rows: [][]string{{"Measurement", "Value"}, {"Output", "42 units"}}, Evidence: []string{f.ID}}}}
}

func TestResearchEndToEndAndImmutableExports(t *testing.T) {
	s := newTestService(t)
	s.SetRunner(testRunner(func(ctx context.Context, x *Session) error {
		ctx, done, err := x.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		if err = x.Iterate(); err != nil {
			return err
		}
		r := sampleEvidence(t, x)
		return x.Submit(r)
	}))
	c, err := s.Create(Request{Topic: "Measured result", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Start(c.ID, "start", "", "first"); err != nil {
		t.Fatal(err)
	}
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.Run.Status == "completed" })
	if len(c.Reports) != 1 || c.Run.Usage.Iterations != 1 {
		t.Fatalf("unexpected case %+v", c)
	}
	if _, err = s.Start(c.ID, "start", "", "first"); err != nil {
		t.Fatalf("idempotent request failed: %v", err)
	}
	for _, format := range []string{"md", "pdf", "docx"} {
		t.Run(format, func(t *testing.T) {
			a, err := s.ExportRevision(context.Background(), c.ID, 1, format)
			if err != nil {
				t.Fatal(err)
			}
			b, err := s.ExportRevision(context.Background(), c.ID, 1, format)
			if err != nil || !bytes.Equal(a.Data, b.Data) || len(a.SHA256) != 64 {
				t.Fatal("export was not immutable", err)
			}
			switch format {
			case "md":
				if !strings.Contains(string(a.Data), "42 units") || !strings.Contains(string(a.Data), "https://example.org/paper") {
					t.Fatal("missing content/source")
				}
			case "pdf":
				if !bytes.HasPrefix(a.Data, []byte("%PDF-")) || !bytes.Contains(a.Data, []byte("/FontFile2")) {
					t.Fatal("invalid PDF or missing embedded fonts")
				}
			case "docx":
				z, err := zip.NewReader(bytes.NewReader(a.Data), int64(len(a.Data)))
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, file := range z.File {
					r, _ := file.Open()
					data, _ := io.ReadAll(r)
					r.Close()
					d := xml.NewDecoder(bytes.NewReader(data))
					for {
						_, err := d.Token()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatalf("invalid OOXML %s: %v", file.Name, err)
						}
					}
					if file.Name == "word/document.xml" {
						found = bytes.Contains(data, []byte("w:tbl")) && bytes.Contains(data, []byte("42 units"))
					}
				}
				if !found {
					t.Fatal("missing editable Word table")
				}
			}
		})
	}
	if err = s.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExportRevision(context.Background(), c.ID, 1, "md"); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted export still accessible")
	}
}

func TestEvidenceRejectsUnseenAndSearchOnlySources(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "facts"})
	x := &Session{service: s, CaseID: c.ID}
	source, err := x.RecordSource("https://example.org/", "Search hit", "ddg_search", "plausible fact", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{source.ID, "fabricated"} {
		if _, err = x.AddFinding(Finding{Text: "fact", SourceID: id, Quote: "plausible"}); err == nil {
			t.Fatal("accepted uninspected evidence")
		}
	}
	read, _ := x.RecordSource("https://example.org/?utm_campaign=a", "Source", "web_scraper", "verified quotation", true)
	again, _ := x.RecordSource("https://example.org/", "Source", "web_scraper", "verified quotation", true)
	if read.ID != again.ID {
		t.Fatal("source dedup failed")
	}
	if _, err = x.AddFinding(Finding{Text: "fact", SourceID: read.ID, Quote: "invented"}); err == nil {
		t.Fatal("accepted invented quotation")
	}
	if err = ValidateReport(c, Report{Title: "fake", Blocks: []Block{{Type: "paragraph", Text: "claim", Evidence: []string{"fake"}}}}); err == nil {
		t.Fatal("accepted unknown reference")
	}
}

func TestResearchBudgetResumeAndParallelReservations(t *testing.T) {
	s := newTestService(t)
	var clockMu sync.Mutex
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
	c, _ := s.Create(Request{Topic: "budget", Effort: "quick"})
	s.mu.Lock()
	c.Run = Run{Status: "running", Profile: Profiles()["quick"]}
	_ = s.saveLocked(&c)
	s.mu.Unlock()
	x := &Session{service: s, CaseID: c.ID, last: now}
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = x.BeforeTool("web_scraper") }()
	}
	wg.Wait()
	c, _ = s.Get(c.ID)
	if c.Run.Usage.Tools != 40 || c.Run.Phase != "writing" {
		t.Fatalf("unbounded parallel calls: %+v", c.Run)
	}
	clockMu.Lock()
	now = now.Add(241 * time.Second)
	clockMu.Unlock()
	if err := x.Iterate(); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Get(c.ID)
	if c.Run.Usage.ActiveMS != 241000 {
		t.Fatal("active time not accumulated")
	}
	x.complete(context.Canceled)
	s.SetRunner(testRunner(func(ctx context.Context, x *Session) error {
		_, done, err := x.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		if err = x.BeforeTool("web_scraper"); err == nil {
			return errors.New("reset research budget")
		}
		return ErrBudget
	}))
	if _, err := s.Start(c.ID, "continue", "maximum", "resume"); err != nil {
		t.Fatal(err)
	}
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.Run.Status == "partial" })
	if c.Run.Profile.Tools != 40 || c.Run.Usage.Tools != 40 || c.Run.Usage.ActiveMS < 241000 {
		t.Fatal("continuation reset cumulative allowance")
	}
}

func TestResearchCancellationCheckpointAndRestart(t *testing.T) {
	s := newTestService(t)
	started := make(chan struct{})
	s.SetRunner(testRunner(func(ctx context.Context, x *Session) error {
		ctx, done, err := x.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		_ = x.Checkpoint(Continuation{Provider: "p", Model: "m", Messages: []byte(`[{"role":"assistant","content":"checkpoint"}]`)})
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}))
	c, _ := s.Create(Request{Topic: "cancel"})
	_, _ = s.Start(c.ID, "start", "", "once")
	<-started
	if err := s.Stop(c.ID, false); err != nil {
		t.Fatal(err)
	}
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.Run.Status == "cancelled" })
	saved, err := s.Continuation(c.ID)
	if err != nil || !bytes.Contains(saved.Messages, []byte("checkpoint")) {
		t.Fatal("lost checkpoint")
	}
	if strings.Contains(string(func() []byte { b, _ := json.Marshal(c); return b }()), "checkpoint") {
		t.Fatal("private continuation leaked through case")
	}
}
