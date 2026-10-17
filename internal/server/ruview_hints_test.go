package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
)

func TestRuViewOperationalHintRecordsAndResolvesOnFreshProbe(t *testing.T) {
	var phase atomic.Value
	phase.Store("fresh")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if phase.Load().(string) == "stale" {
			_, _ = io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"stale","last_frame_age_ms":30000}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ready","source":"esp32","source_state":"live_unverified","last_frame_age_ms":20}`)
	}))
	t.Cleanup(upstream.Close)
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled = true
	c.RuView.AllowPrivate = true
	c.RuView.Mode = "external"
	c.RuView.URL = upstream.URL
	m := ruview.New(func() *config.Config { return c }, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(m.Close)
	s := testOperationalIssueServer(t)
	bindRuViewOperationalHints(m, s.PlannerDB, slog.New(slog.NewTextHandler(io.Discard, nil)))

	probe := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, err := m.Test(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
	}
	probe(1)
	var rows int
	if err := s.PlannerDB.QueryRow(`SELECT count(*) FROM operational_issues`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("fresh probe recorded %d issues", rows)
	}
	phase.Store("stale")
	probe(3)
	var status, kind, source, title, detail string
	var occurrences int
	err := s.PlannerDB.QueryRow(`
		SELECT status, kind, source, title, detail, occurrences
		FROM operational_issues WHERE fingerprint=?`, "ruview:sensing_stale").
		Scan(&status, &kind, &source, &title, &detail, &occurrences)
	if err != nil {
		t.Fatal(err)
	}
	if status != "open" || kind != "runtime_failure" || source != "ruview" || occurrences != 1 {
		t.Fatalf("issue = %s %s %s occurrences %d", status, kind, source, occurrences)
	}
	if !strings.Contains(title, "RuView sensing is stale") || strings.Contains(detail, "http") || strings.Contains(detail, "127.0.0.1") || strings.Contains(strings.ToLower(detail), "token") {
		t.Fatalf("hint text = %q / %q", title, detail)
	}
	if err = s.PlannerDB.QueryRow(`SELECT count(*) FROM operational_issues`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rows = %d, err = %v", rows, err)
	}
	phase.Store("fresh")
	probe(2)
	var resolution string
	err = s.PlannerDB.QueryRow(`
		SELECT status, occurrences, resolution FROM operational_issues WHERE fingerprint=?`, "ruview:sensing_stale").
		Scan(&status, &occurrences, &resolution)
	if err != nil {
		t.Fatal(err)
	}
	if status != "done" || occurrences != 1 || resolution != "RuView sensing is fresh again." {
		t.Fatalf("resolved = %s occurrences %d resolution %q", status, occurrences, resolution)
	}
}
