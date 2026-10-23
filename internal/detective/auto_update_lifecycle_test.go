package detective

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutomaticUpdateCatchupQueueAndFreshConversation(t *testing.T) {
	var clock atomic.Int64
	clock.Store(time.Now().UTC().Unix())
	s, err := New(Options{Path: filepath.Join(t.TempDir(), "cases.db"), Now: func() time.Time { return time.Unix(clock.Load(), 0).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var cases []Case
	for i := 0; i < 2; i++ {
		c, err := s.Create(Request{Topic: "queue", Effort: "maximum", AutoUpdate: AutoUpdateSettings{Enabled: true, IntervalHours: 1, Effort: "normal"}})
		if err != nil {
			t.Fatal(err)
		}
		x := updateSession(t, s, c.ID, "append")
		if err = x.Submit(updateReport(updateEvidence(t, x, i))); err != nil {
			t.Fatal(err)
		}
		if err = s.SaveContinuation(c.ID, Continuation{Messages: json.RawMessage(`[{"role":"user","content":"old conversation"}]`)}); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, c)
	}
	clock.Add(30 * 24 * 60 * 60)
	entered, release := make(chan string, 2), make(chan struct{}, 2)
	var count atomic.Int32
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error {
		count.Add(1)
		current, _ := job.Snapshot()
		if !current.Run.Automatic || current.Run.Profile != Profiles()["normal"] || current.Request.Effort != "maximum" {
			return errors.New("update profile changed initial research")
		}
		checkpoint, err := job.Continuation()
		if err != nil || len(checkpoint.Messages) > 0 {
			return errors.New("old conversation reused")
		}
		_, stop, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		entered <- job.CaseID
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		if err = job.Checked(); err != nil {
			return err
		}
		return job.NoChanges()
	}))
	var first string
	select {
	case first = <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("catchup did not start")
	}
	queueUpdates(t, s)
	if count.Load() != 1 {
		t.Fatal("overlapping runs")
	}
	if _, err = s.Start(first, "continue", "", "overlap"); !errors.Is(err, ErrConflict) {
		t.Fatal("same case overlapped", err)
	}
	release <- struct{}{}
	select {
	case second := <-entered:
		if first == second {
			t.Fatal("missed intervals replayed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second case starved")
	}
	release <- struct{}{}
	for _, old := range cases {
		c := waitCase(t, s, old.ID, func(c Case) bool { return c.AutoUpdate.Outcome == "no_changes" })
		if !c.AutoUpdate.NextRunAt.Equal(s.now().Add(time.Hour)) {
			t.Fatal("next interval did not start at completion")
		}
	}
	queueUpdates(t, s)
	if count.Load() != 2 {
		t.Fatal("extra catchup runs")
	}
}

func TestAutomaticUpdateQuestionAndDuplicateURL(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "questions", AutoUpdate: AutoUpdateSettings{Enabled: true}})
	x := updateSession(t, s, c.ID, "append")
	f := updateEvidence(t, x, 1)
	if err := x.Submit(updateReport(f)); err != nil {
		t.Fatal(err)
	}
	x = updateSession(t, s, c.ID, "append")
	if err := x.Wait("Which station should be checked?"); err != nil {
		t.Fatal(err)
	}
	x.complete(context.Canceled)
	c, err := s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: true, IntervalHours: 1})
	if err != nil || !c.AutoUpdate.NextRunAt.IsZero() || c.AutoUpdate.Outcome != "waiting_for_user" {
		t.Fatal("question did not pause schedule", err)
	}
	oldRun := c.Run.ID
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error {
		current, _ := job.Snapshot()
		if current.Run.ID != oldRun || len(current.Answers) != 1 {
			return errors.New("question did not resume original run")
		}
		_, stop, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		source, err := job.RecordSource("https://example.org/second-publisher", "Same observation", "web_scraper", f.Quote, true)
		if err != nil {
			return err
		}
		repeated, err := job.AddFinding(Finding{Text: f.Text, Quote: f.Quote, SourceID: source.ID})
		if err != nil || repeated.ID != f.ID {
			return errors.New("additional URL duplicated the finding")
		}
		if err = job.Submit(updateReport(repeated)); err == nil {
			return errors.New("duplicate finding published a revision")
		}
		return job.NoChanges()
	}))
	queueUpdates(t, s)
	c, _ = s.Get(c.ID)
	if c.Run.Status != "waiting_for_user" {
		t.Fatal("scheduler answered the question")
	}
	if _, err = s.Start(c.ID, "continue", "", "no-answer"); err == nil {
		t.Fatal("question resumed without an answer")
	}
	if _, err = s.Start(c.ID, "continue", "maximum", "answer", "Station one"); err != nil {
		t.Fatal(err)
	}
	c = waitCase(t, s, c.ID, func(c Case) bool { return c.AutoUpdate.Outcome == "no_changes" })
	if c.LatestRevision != 1 || c.FindingCount != 1 || c.SourceCount != 2 || c.Run.Profile != Profiles()["quick"] {
		t.Fatal("duplicate URL or answer reset the report/budget")
	}
	x = updateSession(t, s, c.ID, "append")
	corrected, err := x.RecordSource("https://example.org/measurement/1", "Corrected station data", "web_scraper", "The corrected measurement is 2 units.", true)
	if err != nil {
		t.Fatal(err)
	}
	correction, err := x.AddFinding(Finding{SourceID: corrected.ID, Text: corrected.Excerpt, Quote: corrected.Excerpt})
	if err != nil {
		t.Fatal(err)
	}
	if err = x.Submit(updateReport(correction)); err != nil {
		t.Fatal(err)
	}
	old, err := s.ReportRevision(c.ID, 1)
	if err != nil || old.Findings[0].Text != f.Text || old.Sources[0].ID == corrected.ID {
		t.Fatal("correction rewrote original evidence", err)
	}
}

func TestDetectiveArchiveBackupFailureAndV1Rollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacyDetective(t, path, true)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA user_version=1; ALTER TABLE detective_cases DROP COLUMN active_ms"); err != nil {
		t.Fatal(err)
	}
	if err = backupBeforeMigration(db, filepath.Join(t.TempDir(), "absent", "legacy.db"), 1, SchemaVersion); err == nil {
		t.Fatal("backup failure ignored")
	}
	db.Close()
	if s, err := New(Options{Path: path}); err == nil {
		s.Close()
		t.Fatal("corrupt old database migrated")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, columns int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT count(*) FROM pragma_table_info('detective_cases') WHERE name='active_ms'").Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if version != 1 || columns != 0 {
		t.Fatal("failed migration altered old schema")
	}
}

func TestAutomaticUpdateReportPagesKeepFullContent(t *testing.T) {
	s := newTestService(t)
	c, _ := s.Create(Request{Topic: "full archive report"})
	x := updateSession(t, s, c.ID, "append")
	f := updateEvidence(t, x, 0)
	r := updateReport(f)
	for i := 0; i < 6; i++ {
		r.Blocks = append(r.Blocks, Block{Type: "table", Rows: [][]string{{"Full source table", "Result"}}, Evidence: []string{f.ID}})
		for j := 0; j < 25; j++ {
			r.Blocks[i+1].Rows = append(r.Blocks[i+1].Rows, []string{"Measurement", f.Text})
		}
	}
	if err := x.Submit(r); err != nil {
		t.Fatal(err)
	}
	first, err := x.History("report", 1, "", 0)
	if err != nil || first.NextBlock != 5 || len(first.Reports[0].Blocks[1].Rows) != 26 {
		t.Fatal("archive truncated report rows", err)
	}
	last, err := x.History("report", 1, "", first.NextBlock)
	if err != nil || last.NextBlock != 0 || len(last.Reports[0].Blocks) != 2 || len(last.Reports[0].Blocks[1].Rows) != 26 {
		t.Fatal("archive lost final report blocks", err)
	}
	if _, err = x.History("report", 1, "", -1); err == nil {
		t.Fatal("invalid block offset accepted")
	}
}
