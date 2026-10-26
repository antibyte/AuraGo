package server

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/detective"
)

func notifyFixture(t *testing.T, enabled bool) (*Server, *desktop.Hub, <-chan desktop.Event) {
	t.Helper()
	hub := desktop.NewHub(16)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = enabled
	cfg.Detective.Enabled = true
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DesktopHub: hub}
	return s, hub, events
}

// notifyTestService builds a Detective service whose notifications reach the server sink.
func notifyTestService(t *testing.T, s *Server) *detective.Service {
	t.Helper()
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "notify.db"), Notify: s.notifyDetective})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func publishOnce(t *testing.T, svc *detective.Service, key string) {
	t.Helper()
	svc.SetRunner(detectiveTestRunner(func(ctx context.Context, job *detective.Session) error {
		_, done, err := job.Activate(ctx)
		if err != nil {
			return err
		}
		defer done()
		quote := "The measured value is 42 units."
		source, err := job.RecordSource("https://example.org/report", quote, "web_scraper", quote, true)
		if err != nil {
			return err
		}
		finding, err := job.AddFinding(detective.Finding{Text: quote, Quote: quote, SourceID: source.ID})
		if err != nil {
			return err
		}
		return job.Submit(detective.Report{Title: "Report", Summary: "Summary", Blocks: []detective.Block{{Type: "paragraph", Text: quote, Evidence: []string{finding.ID}}}})
	}))
	c, err := svc.Create(detective.Request{Topic: "Notify", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Start(c.ID, "start", "quick", key); err != nil {
		t.Fatal(err)
	}
}

// A published revision reaches the notification center with a context that names the case.
func TestDetectiveNotificationOnPublish(t *testing.T) {
	s, _, events := notifyFixture(t, true)
	svc := notifyTestService(t, s)
	publishOnce(t, svc, "notify-1")
	select {
	case got := <-events:
		payload, _ := got.Payload.(map[string]any)
		launch, _ := payload["context"].(map[string]any)
		if got.Type != "notification" || payload["appId"] != "detective" || payload["type"] != "info" {
			t.Fatalf("event = %+v", got)
		}
		if !strings.Contains(payload["title"].(string), "revision 1 published") {
			t.Fatalf("title = %q", payload["title"])
		}
		if payload["message"] != "Notify" {
			t.Fatalf("message = %q", payload["message"])
		}
		caseID, _ := launch["case_id"].(string)
		if caseID == "" {
			t.Fatalf("context = %+v", launch)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no desktop notification")
	}
}

// Without Virtual Desktop nothing is broadcast.
func TestDetectiveNotificationSuppressedWhenDisabled(t *testing.T) {
	s, _, events := notifyFixture(t, false)
	svc := notifyTestService(t, s)
	publishOnce(t, svc, "notify-2")
	select {
	case got := <-events:
		t.Fatalf("unexpected notification: %+v", got)
	case <-time.After(300 * time.Millisecond):
	}
}

// At most one notification per case per minute; another case still notifies.
func TestDetectiveNotificationRateLimited(t *testing.T) {
	s, _, events := notifyFixture(t, true)
	s.notifyDetective("info", "case_a", "First", "one")
	s.notifyDetective("info", "case_a", "Second", "two")
	s.notifyDetective("info", "case_b", "Other case", "three")
	seen := []string{}
	for i := 0; i < 2; i++ {
		select {
		case got := <-events:
			payload, _ := got.Payload.(map[string]any)
			seen = append(seen, payload["title"].(string))
		case <-time.After(time.Second):
			t.Fatalf("only %d notifications arrived: %v", len(seen), seen)
		}
	}
	if len(seen) != 2 || seen[0] != "First" || seen[1] != "Other case" {
		t.Fatalf("notifications = %v", seen)
	}
	select {
	case got := <-events:
		t.Fatalf("a third notification arrived: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
}
