package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/detective"
)

// The state endpoint answers status and new events in one round trip and advances the
// cursor exactly like /live plus /events (audit 4.1/4.18).
func TestDetectiveStateEndpointMatchesLiveAndEvents(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = false
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Detective.Enabled = true
	s := &Server{Cfg: cfg}
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "state.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s.Detective = svc
	svc.SetRunner(detectiveTestRunner(func(ctx context.Context, job *detective.Session) error {
		_, done, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		return nil
	}))
	c, err := svc.Create(detective.Request{Topic: "state", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Start(c.ID, "start", "quick", "state-1"); err != nil {
		t.Fatal(err)
	}
	read := func(after int64) (detective.LiveView, []detective.Event, int64) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/desktop/detective/cases/"+c.ID+"/state?after="+strconv.FormatInt(after, 10), nil)
		w := httptest.NewRecorder()
		s.handleDetective(w, r)
		if w.Code != 200 {
			t.Fatalf("state: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Live   detective.LiveView `json:"live"`
			Events []detective.Event  `json:"events"`
			Next   int64              `json:"next"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Live, body.Events, body.Next
	}
	// Wait for the run to settle, then read the complete event page once.
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, getErr := svc.Get(c.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if got.Run.Status != "queued" && got.Run.Status != "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("run did not settle")
		}
		time.Sleep(10 * time.Millisecond)
	}
	live, events, next := read(0)
	if len(events) == 0 {
		t.Fatal("no events were recorded")
	}
	if live.ID != c.ID || live.Topic != "state" {
		t.Fatalf("live=%+v", live)
	}
	if next != events[len(events)-1].ID {
		t.Fatalf("next=%d, last event=%d", next, events[len(events)-1].ID)
	}
	incrementalLive, incremental, cursor := read(next)
	if len(incremental) != 0 || cursor != next {
		t.Fatalf("incremental read returned %d events and cursor %d", len(incremental), cursor)
	}
	if incrementalLive.ID != c.ID {
		t.Fatalf("incremental live=%+v", incrementalLive)
	}

	r := httptest.NewRequest("GET", "/api/desktop/detective/cases/case_unknown/state", nil)
	w := httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 404 {
		t.Fatalf("unknown case: %d %s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/api/desktop/detective/cases/"+c.ID+"/state?after=-5", nil)
	w = httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 400 {
		t.Fatalf("invalid cursor: %d %s", w.Code, w.Body.String())
	}
}
