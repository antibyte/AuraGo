package detective

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// A v2 store runs the whole v2 -> v3 -> v4 path in one transaction after a backup, keeps
// IDs, revisions and summaries, and a second open does not migrate again (audit D18).
func TestMigrateV2ToV4KeepsCasesAndBacksUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case.db")
	old := legacyDetective(t, path, false)
	s, err := New(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestRevision != 1 || got.LatestCompleteRevision != 1 {
		t.Fatalf("revisions: latest=%d complete=%d", got.LatestRevision, got.LatestCompleteRevision)
	}
	var version int
	if err = s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion)
	}
	var stored string
	if err = s.db.QueryRow("SELECT body FROM detective_summaries WHERE case_id=?", old.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	projected, err := json.Marshal(caseSummary(&got))
	if err != nil {
		t.Fatal(err)
	}
	if stored != string(projected) {
		t.Fatalf("summary mismatch:\n stored: %s\n want:   %s", stored, projected)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.pre-v4-*.bak"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := New(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	backups, err = filepath.Glob(filepath.Join(filepath.Dir(path), "*.pre-v4-*.bak"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("a second open migrated again: %v", backups)
	}
	if got, err = again.Get(old.ID); err != nil || got.LatestRevision != 1 {
		t.Fatalf("reopened: %+v %v", got, err)
	}
}

// Enabling automatic updates schedules only a case with a complete report that is not
// running or waiting for an answer; disabling always clears the schedule.
func TestSetAutoUpdateMatrix(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		status  string
		sched   bool
		outcome string
	}{
		{"enable completed", true, "completed", true, "scheduled"},
		{"enable partial", true, "partial", true, "scheduled"},
		{"enable failed", true, "failed", true, "scheduled"},
		{"enable running", true, "running", false, ""},
		{"enable waiting", true, "waiting_for_user", false, "waiting_for_user"},
		{"disable completed", false, "completed", false, "paused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t)
			c, err := s.Create(Request{Topic: "Matrix", Effort: "quick"})
			if err != nil {
				t.Fatal(err)
			}
			baseline := provisionalSession(t, s, c.ID, 0)
			if err = baseline.Submit(updateReport(updateEvidence(t, baseline, 1))); err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			stored, err := s.getLocked(c.ID)
			if err != nil {
				s.mu.Unlock()
				t.Fatal(err)
			}
			stored.Run.Status = tc.status
			err = s.saveLocked(&stored)
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: tc.enabled, IntervalHours: 24, ReportStyle: "append", Effort: "quick"})
			if err != nil {
				t.Fatal(err)
			}
			if scheduled := !got.AutoUpdate.NextRunAt.IsZero(); scheduled != tc.sched {
				t.Fatalf("next_run_at scheduled=%v, want %v (%+v)", scheduled, tc.sched, got.AutoUpdate)
			}
			if got.AutoUpdate.Outcome != tc.outcome {
				t.Fatalf("outcome=%q, want %q", got.AutoUpdate.Outcome, tc.outcome)
			}
		})
	}
}

// A queued automatic update reports queued, running and its outcome through the live view,
// and no_changes leaves the revision and report untouched (audit D18).
func TestLiveUnderAutomaticUpdate(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "Live automation", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	baseline := provisionalSession(t, s, c.ID, 0)
	if err = baseline.Submit(updateReport(updateEvidence(t, baseline, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: true, IntervalHours: 1, ReportStyle: "append", Effort: "quick"}); err != nil {
		t.Fatal(err)
	}
	// Make the occurrence due and let the worker claim it.
	s.mu.Lock()
	stored, err := s.getLocked(c.ID)
	if err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	stored.AutoUpdate.NextRunAt = s.now().UTC().Add(-time.Minute)
	err = s.saveLocked(&stored)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error {
		_, done, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		<-release
		quote := "The measured value is 42 units."
		if _, err := job.RecordSource("https://example.org/check", quote, "web_scraper", quote, true); err != nil {
			return err
		}
		return job.NoChanges()
	}))
	queueUpdates(t, s)
	deadline := time.Now().Add(5 * time.Second)
	seen := map[string]bool{}
	for time.Now().Before(deadline) && !seen["running"] {
		live, err := s.Live(c.ID)
		if err != nil {
			t.Fatal(err)
		}
		seen[live.Status] = true
		time.Sleep(10 * time.Millisecond)
	}
	if !seen["running"] {
		t.Fatalf("statuses = %v", seen)
	}
	close(release)
	for time.Now().Before(deadline) {
		live, err := s.Live(c.ID)
		if err != nil {
			t.Fatal(err)
		}
		seen[live.Status] = true
		if live.Status == "completed" && live.AutoUpdate.Outcome == "no_changes" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !seen["running"] || !seen["completed"] {
		t.Fatalf("statuses = %v", seen)
	}
	got, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestRevision != 1 || len(got.Reports) != 1 || got.Reports[0].Revision != 1 {
		t.Fatalf("no_changes changed the report: %+v", got.Reports)
	}
	if got.AutoUpdate.Outcome != "no_changes" {
		t.Fatalf("outcome = %q", got.AutoUpdate.Outcome)
	}
}
