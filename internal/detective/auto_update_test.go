package detective

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func updateSession(t *testing.T, s *Service, key, style string) *Session {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.getLocked(key)
	if err != nil {
		t.Fatal(err)
	}
	c.Run = Run{ID: id("run_"), Status: "running", Phase: "research", Profile: Profiles()["quick"], Automatic: c.LatestRevision > 0, BaseRevision: c.LatestRevision, ReportStyle: style}
	c.Sources, c.Findings = []Source{}, []Finding{}
	if err = s.saveLocked(&c); err != nil {
		t.Fatal(err)
	}
	return &Session{service: s, CaseID: key, cancel: func() {}}
}

func updateEvidence(t *testing.T, x *Session, n int) Finding {
	t.Helper()
	quote := fmt.Sprintf("The observed measurement is %d units.", n)
	source, err := x.RecordSource(fmt.Sprintf("https://example.org/measurement/%d", n), quote, "web_scraper", quote, true)
	if err != nil {
		t.Fatal(err)
	}
	f, err := x.AddFinding(Finding{Text: quote, Quote: quote, SourceID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func updateReport(f ...Finding) Report {
	r := Report{Title: "Measurements", Summary: "Evidence from the station"}
	for _, v := range f {
		r.Blocks = append(r.Blocks, Block{Type: "paragraph", Text: v.Text, Evidence: []string{v.ID}})
	}
	return r
}

func queueUpdates(t *testing.T, s *Service) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.queueDueLocked(); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticUpdateScheduleAndNoChanges(t *testing.T) {
	var clock atomic.Int64
	start := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	clock.Store(start.Unix())
	s, err := New(Options{Path: filepath.Join(t.TempDir(), "case.db"), Now: func() time.Time { return time.Unix(clock.Load(), 0).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var calls atomic.Int32
	s.SetRunner(testRunner(func(ctx context.Context, x *Session) error {
		calls.Add(1)
		_, stop, err := x.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		c, _ := x.Snapshot()
		f := updateEvidence(t, x, 1)
		if c.Run.Automatic {
			return x.NoChanges()
		}
		return x.Submit(updateReport(f))
	}))
	c, err := s.Create(Request{Topic: "monitor", AutoUpdate: AutoUpdateSettings{Enabled: true, IntervalHours: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !c.AutoUpdate.NextRunAt.IsZero() {
		t.Fatal("draft was scheduled")
	}
	if _, err = s.Start(c.ID, "start", "quick", "initial"); err != nil {
		t.Fatal(err)
	}
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.Run.Status == "completed" })
	if !c.AutoUpdate.NextRunAt.Equal(start.Add(time.Hour)) {
		t.Fatal(c.AutoUpdate)
	}
	original, err := s.ExportRevision(context.Background(), c.ID, 1, "md")
	if err != nil {
		t.Fatal(err)
	}
	clock.Store(start.Add(59 * time.Minute).Unix())
	queueUpdates(t, s)
	if calls.Load() != 1 {
		t.Fatal("early update")
	}
	clock.Store(start.Add(time.Hour).Unix())
	queueUpdates(t, s)
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.AutoUpdate.Outcome == "no_changes" })
	if calls.Load() != 2 || c.LatestRevision != 1 || c.SourceCount != 1 || c.FindingCount != 1 {
		t.Fatalf("duplicate update: %+v calls=%d", c, calls.Load())
	}
	if !c.AutoUpdate.LastCheckedAt.Equal(start.Add(time.Hour)) || !c.AutoUpdate.NextRunAt.Equal(start.Add(2*time.Hour)) {
		t.Fatal(c.AutoUpdate)
	}
	if err = json.Unmarshal([]byte(`{"enabled":true,"interval_hours":3,"report_style":"rewrite","effort":"maximum"}`), &c.Request.AutoUpdate); err != nil {
		t.Fatal(err)
	}
	c, err = s.SetAutoUpdate(c.ID, c.Request.AutoUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if !c.AutoUpdate.NextRunAt.Equal(start.Add(4 * time.Hour)) {
		t.Fatal("interval was not rescheduled")
	}
	queueUpdates(t, s)
	if calls.Load() != 2 {
		t.Fatal("same occurrence replayed")
	}
	again, err := s.ExportRevision(context.Background(), c.ID, 1, "md")
	if err != nil || !bytes.Equal(original.Data, again.Data) {
		t.Fatal("no_changes modified the report", err)
	}
}

func TestAutomaticUpdateArchiveBeyondOriginalLimits(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "archive", AutoUpdate: AutoUpdateSettings{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	var original []byte
	for round := 0; round < 41; round++ {
		x := updateSession(t, s, c.ID, "append")
		var findings []Finding
		for i := 0; i < 10; i++ {
			findings = append(findings, updateEvidence(t, x, round*10+i))
		}
		if err = x.Submit(updateReport(findings...)); err != nil {
			t.Fatal(err)
		}
		x.complete(nil)
		if round == 0 {
			a, err := s.ExportRevision(context.Background(), c.ID, 1, "md")
			if err != nil {
				t.Fatal(err)
			}
			original = a.Data
		}
	}
	c, err = s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.LatestRevision != 41 || c.SourceCount != 410 || c.FindingCount != 410 || len(c.Reports) != 1 || len(c.Sources) != 10 {
		t.Fatalf("unbounded or incomplete working set: reports=%d sources=%d counts=%d/%d/%d", len(c.Reports), len(c.Sources), c.LatestRevision, c.SourceCount, c.FindingCount)
	}
	for _, kind := range []string{"reports", "sources", "findings"} {
		before, count := int64(0), 0
		for {
			p, err := s.Archive(c.ID, kind, before, 7, "")
			if err != nil {
				t.Fatal(err)
			}
			count += len(p.Reports) + len(p.Sources) + len(p.Findings)
			if p.Next == 0 {
				break
			}
			if before > 0 && p.Next >= before {
				t.Fatal("cursor did not advance")
			}
			before = p.Next
		}
		want := 410
		if kind == "reports" {
			want = 41
		}
		if count != want {
			t.Fatalf("%s: %d", kind, count)
		}
	}
	resolved, err := s.resolvedReport(context.Background(), c.ID, 41)
	if err != nil || len(resolved.Blocks) != 490 {
		t.Fatalf("append chain: %d %v", len(resolved.Blocks), err)
	}
	latest, _ := s.ReportRevision(c.ID, 41)
	if latest.BaseRevision != 40 || len(latest.Blocks) != 10 {
		t.Fatal("append copied history")
	}
	x := updateSession(t, s, c.ID, "rewrite")
	if err = x.Submit(updateReport(updateEvidence(t, x, 999))); err != nil {
		t.Fatal(err)
	}
	rewritten, err := s.resolvedReport(context.Background(), c.ID, 42)
	if err != nil || len(rewritten.Blocks) != 1 || rewritten.BaseRevision != 0 {
		t.Fatal("rewrite retained append chain", err)
	}
	a, err := s.ExportRevision(context.Background(), c.ID, 1, "md")
	if err != nil || !bytes.Equal(a.Data, original) {
		t.Fatal("old export changed", err)
	}
	page, err := s.Archive(c.ID, "findings", 0, 20, "409 units")
	if err != nil || len(page.Findings) != 1 {
		t.Fatal("archive search", err)
	}
	if _, err = s.Archive(c.ID, "sources", 0, 51, ""); err == nil {
		t.Fatal("unbounded page accepted")
	}
}

func TestAutomaticUpdateFailureRetainsReportAndPendingEvidence(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "failure", AutoUpdate: AutoUpdateSettings{Enabled: true}})
	x := updateSession(t, s, c.ID, "append")
	if err := x.Submit(updateReport(updateEvidence(t, x, 0))); err != nil {
		t.Fatal(err)
	}
	x = updateSession(t, s, c.ID, "append")
	if err := x.NoChanges(); err == nil {
		t.Fatal("failed retrieval counted as successful")
	}
	f := updateEvidence(t, x, 1)
	if err := x.NoChanges(); err == nil {
		t.Fatal("new finding suppressed")
	}
	x.complete(errors.New("provider failed"))
	failed, _ := s.Get(c.ID)
	if failed.LatestRevision != 1 || failed.AutoUpdate.Outcome != "error" || !failed.AutoUpdate.LastCheckedAt.IsZero() {
		t.Fatal("failed update published", failed.AutoUpdate)
	}
	x = updateSession(t, s, c.ID, "append")
	recovered := updateEvidence(t, x, 1)
	if recovered.ID != f.ID {
		t.Fatal("pending finding duplicated")
	}
	if err := x.Submit(updateReport(recovered)); err != nil {
		t.Fatal("pending evidence could not be published", err)
	}
	x = updateSession(t, s, c.ID, "append")
	f = updateEvidence(t, x, 2)
	if _, err := s.db.Exec(`CREATE TRIGGER reject_report BEFORE INSERT ON detective_reports BEGIN SELECT RAISE(ABORT,'fixture publication failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := x.Submit(updateReport(f)); err == nil || x.Done() {
		t.Fatal("failed transaction completed")
	}
	failed, _ = s.Get(c.ID)
	if failed.LatestRevision != 2 || failed.Run.Status != "running" {
		t.Fatal("partial publication escaped transaction")
	}
	if err := s.Stop(c.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := x.Submit(updateReport(f)); err == nil {
		t.Fatal("late publication after stop")
	}
	x.complete(context.Canceled)
	failed, _ = s.Get(c.ID)
	if failed.AutoUpdate.Enabled || failed.LatestRevision != 2 {
		t.Fatal("stop did not pause automation")
	}
}

func TestAutomaticUpdateRestartRetainsBudgetAndPermissionGate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case.db")
	s, err := New(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.Create(Request{Topic: "restart", AutoUpdate: AutoUpdateSettings{Enabled: true}})
	x := updateSession(t, s, c.ID, "append")
	if err = x.Submit(updateReport(updateEvidence(t, x, 0))); err != nil {
		t.Fatal(err)
	}
	x = updateSession(t, s, c.ID, "append")
	c, _ = s.Get(c.ID)
	c.Run.Usage.ActiveMS, c.Run.Usage.Tools = 123000, 7
	if err = s.saveLocked(&c); err != nil {
		t.Fatal(err)
	}
	oldID := c.Run.ID
	if err = s.SaveContinuation(c.ID, Continuation{Messages: json.RawMessage(`[{"role":"assistant","content":"saved update"}]`)}); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	var allowed atomic.Bool
	s, err = New(Options{Path: path, Allowed: allowed.Load})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var entered atomic.Int32
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error {
		entered.Add(1)
		before, _ := job.Snapshot()
		checkpoint, err := job.Continuation()
		if err != nil || !strings.Contains(string(checkpoint.Messages), "saved update") {
			return errors.New("restart lost conversation")
		}
		if before.Run.ID != oldID || before.Run.Usage.ActiveMS != 123000 || before.Run.Usage.Tools != 7 {
			return errors.New("restart reset the budget")
		}
		_, stop, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		updateEvidence(t, job, 0)
		return job.NoChanges()
	}))
	live, err := s.Live(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.AutoUpdate.Outcome != "permission_blocked" || entered.Load() != 0 {
		t.Fatal("permission gate ignored")
	}
	allowed.Store(true)
	s.wake()
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.AutoUpdate.Outcome == "no_changes" })
	if entered.Load() != 1 || c.Run.Usage.ActiveMS < 123000 || c.Run.ID != oldID {
		t.Fatal("restart changed the run")
	}
}

func TestAutomaticUpdateDisableCancelsActiveRun(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "stop", AutoUpdate: AutoUpdateSettings{Enabled: true}})
	x := updateSession(t, s, c.ID, "append")
	if err := x.Submit(updateReport(updateEvidence(t, x, 0))); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error {
		ctx, stop, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		updateEvidence(t, job, 1)
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}))
	s.mu.Lock()
	c, _ = s.getLocked(c.ID)
	c.AutoUpdate.NextRunAt = time.Now().UTC().Add(-time.Hour)
	err := s.saveLocked(&c)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	s.wake()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("update did not start")
	}
	c, err = s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.Run.Status == "cancelled" })
	if c.LatestRevision != 1 || c.AutoUpdate.Enabled || !c.AutoUpdate.NextRunAt.IsZero() || c.FindingCount != 2 {
		t.Fatal("disable lost evidence or published a partial report")
	}
}

func legacyDetective(t *testing.T, path string, invalid bool) Case {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE detective_cases(id TEXT PRIMARY KEY,body BLOB NOT NULL,status TEXT NOT NULL,updated TEXT NOT NULL,active_ms INTEGER NOT NULL DEFAULT 0);
CREATE TABLE detective_artifacts(case_id TEXT,revision INTEGER,format TEXT,body BLOB,mime TEXT,filename TEXT,sha256 TEXT,PRIMARY KEY(case_id,revision,format)); PRAGMA user_version=2;`)
	if err != nil {
		t.Fatal(err)
	}
	src := Source{ID: "legacy_source", URL: "https://example.org/legacy", Excerpt: "The original recorded evidence.", Hash: "original", Status: "read"}
	f := Finding{ID: "legacy_finding", SourceID: src.ID, Text: src.Excerpt, Quote: src.Excerpt}
	r := updateReport(f)
	r.Revision = 1
	r.CreatedAt = time.Now().UTC()
	r.Sources = []Source{src}
	r.Findings = []Finding{f}
	c := Case{ID: "legacy_case", Request: Request{Topic: "legacy", Effort: "normal"}, Run: Run{Status: "completed", Usage: Usage{ActiveMS: 10}}, Sources: []Source{src}, Findings: []Finding{f}, Reports: []Report{r}, UpdatedAt: r.CreatedAt}
	b, _ := json.Marshal(c)
	if invalid {
		b = []byte("invalid-json")
	}
	if _, err = db.Exec("INSERT INTO detective_cases VALUES(?,?,?,?,?)", c.ID, b, "completed", c.UpdatedAt.Format(time.RFC3339Nano), 2000); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO detective_artifacts VALUES(?,?,?,?,?,?,?)", c.ID, 1, "md", []byte("original cached bytes"), "text/markdown", "legacy.md", "original-sha"); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDetectiveArchiveMigrationAndRollback(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "case.db")
			old := legacyDetective(t, path, invalid)
			s, err := New(Options{Path: path})
			if invalid {
				if err == nil {
					s.Close()
					t.Fatal("corrupt migration succeeded")
				}
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				var version, tables int
				if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
					t.Fatal(err)
				}
				if err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='detective_reports'").Scan(&tables); err != nil {
					t.Fatal(err)
				}
				if version != 2 || tables != 0 {
					t.Fatal("migration was not atomic")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				got, err := s.Get(old.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.AutoUpdate.Enabled || got.Run.Usage.ActiveMS != 2000 || got.LatestRevision != 1 {
					t.Fatal("migration altered defaults or time")
				}
				r, err := s.ReportRevision(old.ID, 1)
				if err != nil {
					t.Fatal(err)
				}
				before, _ := json.Marshal(old.Reports[0])
				after, _ := json.Marshal(r)
				if !bytes.Equal(before, after) {
					t.Fatal("migration altered report")
				}
				a, err := s.ExportRevision(context.Background(), old.ID, 1, "md")
				if err != nil || string(a.Data) != "original cached bytes" {
					t.Fatal("cached export changed", err)
				}
				if err = s.Close(); err != nil {
					t.Fatal(err)
				}
				s, err = New(Options{Path: path})
				if err != nil {
					t.Fatal(err)
				}
				s.Close()
			}
			backups, err := filepath.Glob(path + ".pre-v4-*.bak")
			if err != nil || len(backups) != 1 {
				t.Fatal("missing or repeated backup", err)
			}
		})
	}
}

func TestAutomaticUpdateSettingsAndExportCancellation(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "limits"})
	for _, v := range []AutoUpdateSettings{{IntervalHours: -1}, {IntervalHours: 8761}, {ReportStyle: "unknown"}, {Effort: "unknown"}} {
		if _, err := s.SetAutoUpdate(c.ID, v); err == nil {
			t.Fatal("invalid settings accepted", v)
		}
	}
	x := updateSession(t, s, c.ID, "append")
	if err := x.Submit(updateReport(updateEvidence(t, x, 0))); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.resolvedReport(ctx, c.ID, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	page, err := x.History("sources", 0, "", 0)
	if err != nil || len(page.Sources) != 1 || strings.Contains(fmt.Sprint(page), "reasoning_content") {
		t.Fatal("invalid history", err)
	}
}
