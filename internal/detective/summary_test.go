package detective

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

// A version-3 file gains the summary column, one backup and a backfill through the
// same projection every later save uses; a second open must not migrate again.
func TestMigrateV3ToV4BackfillsSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.db")
	s, err := New(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Create(Request{Topic: "migration", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	c.Run.Status = "completed"
	c.Sources = []Source{{ID: "src_a", URL: "https://example.org/a", Title: "A", Status: "read", Excerpt: "evidence"}}
	c.SourceCount = 1
	c.LatestRevision = 2
	c.LatestSourceID = "src_a"
	if err = s.saveLocked(&c); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()
	// Turn the file back into a version-3 database: no summary table, version 3.
	if _, err = s.db.Exec("DROP TABLE detective_summaries"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("PRAGMA user_version=3"); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := New(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var version int
	if err = reopened.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != SchemaVersion {
		t.Fatalf("version=%d err=%v", version, err)
	}
	backups, err := filepath.Glob(path + ".pre-v4-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	var stored string
	if err = reopened.db.QueryRow("SELECT body FROM detective_summaries WHERE case_id=?", c.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(caseSummary(&got))
	if err != nil {
		t.Fatal(err)
	}
	var want, have CaseSummary
	if err = json.Unmarshal(wantJSON, &want); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(stored), &have); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, have) {
		t.Fatalf("summary mismatch\nwant %+v\nhave %+v", want, have)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := New(Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = third.Close() })
	if again, _ := filepath.Glob(path + ".pre-v4-*.bak"); len(again) != 1 {
		t.Fatalf("repeated backup: %v", again)
	}
}

// The live view answers from the summary row; a damaged evidence body cannot break it.
func TestLiveReadsSummaryOnly(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "summary topic", Effort: "normal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("UPDATE detective_cases SET body='{broken' WHERE id=?", c.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Live(c.ID)
	if err != nil {
		t.Fatalf("live parsed the body: %v", err)
	}
	if got.Topic != "summary topic" || got.Effort != "normal" || got.Status != "draft" {
		t.Fatalf("%+v", got)
	}
}

// Every write path keeps the stored summary byte-identical to the projection.
func TestCaseSummaryMatchesProjectionOnEverySave(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "projection", Effort: "quick", AutoUpdate: AutoUpdateSettings{Enabled: true, IntervalHours: 1}})
	if err != nil {
		t.Fatal(err)
	}
	check := func(stage string) {
		t.Helper()
		var stored string
		if err := s.db.QueryRow("SELECT body FROM detective_summaries WHERE case_id=?", c.ID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(c.ID)
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(caseSummary(&got))
		if err != nil {
			t.Fatal(err)
		}
		if stored != string(want) {
			t.Fatalf("%s: stored %s want %s", stage, stored, want)
		}
	}
	check("create")
	if _, err = s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: true, IntervalHours: 2, ReportStyle: "rewrite", Effort: "quick"}); err != nil {
		t.Fatal(err)
	}
	check("auto update")
	s.mu.Lock()
	live, err := s.getLocked(c.ID)
	if err == nil {
		live.Run.Status = "running"
		live.Run.Usage.Tools = 3
		err = s.saveLocked(&live)
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	check("save")
}
