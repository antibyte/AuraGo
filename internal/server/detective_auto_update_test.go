package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/detective"
)

func TestDetectiveAutomaticUpdateHTTPAndScopedCompletion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Detective.Enabled, cfg.VirtualDesktop.Enabled, cfg.VirtualDesktop.AllowAgentControl = true, true, true
	s := &Server{Cfg: cfg}
	var clock atomic.Int64
	clock.Store(time.Now().UTC().Unix())
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "detective.db"), Now: func() time.Time { return time.Unix(clock.Load(), 0).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s.Detective = svc
	call := func(method, path, body string, status int) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		s.handleDetective(w, httptest.NewRequest(method, "/api/desktop/detective/"+path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var c detective.Case
	if err = json.Unmarshal(call("POST", "cases", `{"topic":"water","auto_update":{"enabled":true,"interval_hours":1}}`, 201), &c); err != nil {
		t.Fatal(err)
	}
	path := "cases/" + c.ID
	if !c.AutoUpdate.Enabled || !c.AutoUpdate.NextRunAt.IsZero() {
		t.Fatal("draft schedule")
	}
	runner := detectiveTestRunner(func(ctx context.Context, job *detective.Session) error {
		ctx, release, err := s.beginDesktopRun(ctx)
		if err != nil {
			return err
		}
		defer release()
		_, stop, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		current, _ := job.Snapshot()
		if current.Run.Automatic {
			if out := detectiveReportCall(job, agent.ToolCall{Operation: "no_changes"}); !strings.Contains(out, `"error"`) {
				return fmt.Errorf("failed retrieval accepted: %s", out)
			}
			detectiveCapture(job, agent.ToolCall{Action: "ddg_search"}, `{"status":"success","results":[]}`)
			out := detectiveReportCall(job, agent.ToolCall{Operation: "history", Content: `{"kind":"sources"}`})
			if !strings.Contains(out, "https://example.org/water") {
				return fmt.Errorf("archive unavailable: %s", out)
			}
			return publishDesktopResult(ctx, func() error {
				out := detectiveReportCall(job, agent.ToolCall{Operation: "no_changes"})
				if strings.Contains(out, `"error"`) {
					return fmt.Errorf("completion: %s", out)
				}
				return nil
			})
		}
		source, err := job.RecordSource("https://example.org/water", "Water", "web_scraper", "Water is stored in the tank.", true)
		if err != nil {
			return err
		}
		f, err := job.AddFinding(detective.Finding{SourceID: source.ID, Text: source.Excerpt, Quote: source.Excerpt})
		if err != nil {
			return err
		}
		return job.Submit(detective.Report{Title: "Water", Summary: source.Excerpt, Blocks: []detective.Block{{Type: "paragraph", Text: f.Text, Evidence: []string{f.ID}}}})
	})
	svc.SetRunner(runner)
	call("POST", path+"/run", `{"action":"start","idempotency_key":"first"}`, 202)
	wait := func(outcome string) detective.Case {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			c, _ = svc.Get(c.ID)
			if c.Run.Status == "completed" && (outcome == "" || c.AutoUpdate.Outcome == outcome) {
				return c
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("incomplete case: %+v", c.Run)
		return c
	}
	c = wait("")
	clock.Add(60 * 60)
	svc.SetRunner(runner)
	c = wait("no_changes")
	if c.LatestRevision != 1 || c.AutoUpdate.LastCheckedAt.IsZero() {
		t.Fatal("no-changes published a revision")
	}
	for _, kind := range []string{"reports", "sources", "findings"} {
		call("GET", path+"/"+kind+"?limit=1", "", 200)
		call("GET", path+"/"+kind+"?limit=51", "", 400)
		call("GET", path+"/"+kind+"?before=invalid", "", 400)
	}
	call("GET", path+"/report?revision=1", "", 200)
	call("PUT", path+"/auto-update", `{"enabled":false,"interval_hours":48,"effort":"normal","report_style":"rewrite"}`, 200)
	c, _ = svc.Get(c.ID)
	if c.AutoUpdate.IntervalHours != 48 || c.AutoUpdate.Effort != "normal" || c.AutoUpdate.ReportStyle != "rewrite" {
		t.Fatal("disabled settings not saved")
	}
	call("PUT", path+"/auto-update", `{"enabled":true,"interval_hours":48}`, 200)
	cfg.Detective.ReadOnly = true
	call("PUT", path+"/auto-update", `{"enabled":true}`, 403)
	call("PUT", path+"/auto-update", `{"enabled":false,"interval_hours":1}`, 200)
	c, _ = svc.Get(c.ID)
	if c.AutoUpdate.Enabled || c.AutoUpdate.IntervalHours != 48 || !c.AutoUpdate.NextRunAt.IsZero() {
		t.Fatal("read-only stop mutated settings or failed to pause")
	}
	cfg.Auth.Enabled = true
	call("GET", path+"/reports", "", 401)
}

func TestDetectiveAutomaticUpdateRevokesPublication(t *testing.T) {
	cfg := &config.Config{}
	cfg.Detective.Enabled, cfg.VirtualDesktop.Enabled, cfg.VirtualDesktop.AllowAgentControl = true, true, true
	s := &Server{Cfg: cfg}
	s.cfgSnapshot.Store(cfg)
	var clock atomic.Int64
	clock.Store(time.Now().UTC().Unix())
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "case.db"), Now: func() time.Time { return time.Unix(clock.Load(), 0).UTC() }, Allowed: func() bool { return detectiveResearchAllowed(s.ConfigSnapshot()) }})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s.Detective = svc
	entered, attempted := make(chan struct{}), make(chan error, 1)
	runner := detectiveTestRunner(func(ctx context.Context, job *detective.Session) error {
		ctx, release, err := s.beginDesktopRun(ctx)
		if err != nil {
			return err
		}
		defer release()
		ctx, stop, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer stop()
		c, _ := job.Snapshot()
		quote := "The original water measurement."
		if c.Run.Automatic {
			quote = "The corrected water measurement."
		}
		source, err := job.RecordSource("https://example.org/station", "Station", "web_scraper", quote, true)
		if err != nil {
			return err
		}
		f, err := job.AddFinding(detective.Finding{Text: quote, Quote: quote, SourceID: source.ID})
		if err != nil {
			return err
		}
		r := detective.Report{Title: "Station", Blocks: []detective.Block{{Type: "paragraph", Text: quote, Evidence: []string{f.ID}}}}
		if c.Run.Automatic {
			close(entered)
			<-ctx.Done()
		}
		err = publishDesktopResult(ctx, func() error { return job.Submit(r) })
		if c.Run.Automatic {
			attempted <- err
		}
		return err
	})
	svc.SetRunner(runner)
	c, err := svc.Create(detective.Request{Topic: "revoke update", AutoUpdate: detective.AutoUpdateSettings{Enabled: true, IntervalHours: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Start(c.ID, "start", "quick", "initial"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, _ = svc.Get(c.ID)
		if c.LatestRevision == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if c.LatestRevision != 1 {
		t.Fatal("baseline did not publish")
	}
	clock.Add(60 * 60)
	svc.SetRunner(runner)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("update did not start")
	}
	next := *cfg
	next.Detective.ReadOnly = true
	s.replaceConfigSnapshot(&next)
	select {
	case err = <-attempted:
		if err == nil {
			t.Fatal("late publication passed revoked permission")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("permission did not cancel update")
	}
	c, err = svc.Get(c.ID)
	if err != nil || c.LatestRevision != 1 || c.FindingCount != 2 {
		t.Fatal("revocation lost pending evidence or replaced report", err)
	}
	live, err := svc.Live(c.ID)
	if err != nil || live.AutoUpdate.Outcome != "permission_blocked" {
		t.Fatal("permission block not visible", err)
	}
}
