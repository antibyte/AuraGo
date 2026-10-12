package server

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/persondetect"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// persondetect isolates camera facts with its own copy of security.IsolateExternalData (it
// must not import internal/security); TestIsolateAlertFactsFormat pins the same literal.
func TestPersonAlertIsolationMatchesSecurity(t *testing.T) {
	if got, want := security.IsolateExternalData(`a<b & "c"`), "<external_data>\na&lt;b &amp; &#34;c&#34;\n</external_data>"; got != want {
		t.Fatalf("security.IsolateExternalData = %q; persondetect.isolateAlertFacts produces %q — update it", got, want)
	}
}

func TestPersonAlertToastNeedsTheDesktop(t *testing.T) {
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DesktopHub: hub}
	ev := persondetect.AlertEvent{EventID: 42, CameraID: "garden"}

	var skip *persondetect.SkipError
	if err := s.showPersonAlertToast(ev, "T", "M"); !errors.As(err, &skip) || skip.Reason != "desktop unavailable" {
		t.Fatalf("desktop off: %v", err)
	}
	cfg.VirtualDesktop.Enabled = true
	if err := s.showPersonAlertToast(ev, "Person detected – Garten", "Garten (Einfahrt), 21:14, 91%"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-events:
		payload, _ := got.Payload.(map[string]any)
		launch, _ := payload["context"].(map[string]any)
		if got.Type != "notification" || payload["appId"] != "network-cameras" || payload["type"] != "info" ||
			payload["title"] != "Person detected – Garten" || payload["message"] != "Garten (Einfahrt), 21:14, 91%" ||
			launch["event_id"] != int64(42) || launch["camera_id"] != "garden" {
			t.Fatalf("event = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("no desktop notification")
	}
}

func TestPersonAlertDispatcherReachesDesktopMissionAndAgent(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	mm := tools.NewMissionManagerV2(t.TempDir(), nil)
	t.Cleanup(mm.Stop)
	if err := mm.Create(&tools.MissionV2{ID: "person_garden", Name: "Garten", Prompt: "Check the garden camera",
		ExecutionType: tools.ExecutionTriggered, TriggerType: tools.TriggerPersonDetected,
		TriggerConfig: &tools.TriggerConfig{PersonCameraID: "garden"}}); err != nil {
		t.Fatal(err)
	}
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MissionManagerV2: mm, DesktopHub: hub}

	d := s.newPersonAlertDispatcher()
	var mu sync.Mutex
	reports := map[string]persondetect.AlertResult{}
	d.Dispatch(persondetect.AlertEvent{
		EventID: 42, CameraID: "garden", CameraName: "Garten", Zones: []string{"Einfahrt"}, Score: 0.91, Persons: 1, Armed: true,
		StartedAt: time.Date(2026, 10, 11, 21, 14, 0, 0, time.Local),
		Actions:   persondetect.Actions{Agent: true, Desktop: true, Channel: "all"},
		Fire:      persondetect.FireSet{Agent: true, Desktop: true, Mission: true},
		Language:  "en",
	}, func(action string, result persondetect.AlertResult) {
		mu.Lock()
		defer mu.Unlock()
		reports[action] = result
	})
	d.Wait()
	for _, action := range []string{persondetect.ActionAgent, persondetect.ActionDesktop, persondetect.ActionMission} {
		if reports[action].Status != "sent" {
			t.Fatalf("%s = %+v (all reports %+v)", action, reports[action], reports)
		}
	}
	select {
	case got := <-events:
		payload, _ := got.Payload.(map[string]any)
		if title, _ := payload["title"].(string); !strings.Contains(title, "Garten") {
			t.Fatalf("toast = %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("no toast")
	}
	queue, _ := mm.GetQueue()
	if items := queue.List(); len(items) != 1 || items[0].MissionID != "person_garden" || !strings.Contains(items[0].TriggerData, `"event_id":42`) {
		t.Fatalf("mission queue = %+v", items)
	}
	select {
	case prompt := <-s.personAlertAgents.queue:
		if !strings.Contains(prompt, "<external_data>") || !strings.Contains(prompt, "Event: 42") {
			t.Fatalf("agent prompt = %q", prompt)
		}
	default:
		t.Fatal("no agent wake-up queued")
	}
}

// Adaptation: the mission sink reads MissionManagerV2 when it dispatches (the dispatcher is
// built before Start) and reports skipped while there is none.
func TestPersonAlertMissionSkipsWithoutMissionControl(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	d := s.newPersonAlertDispatcher()
	var mu sync.Mutex
	var got persondetect.AlertResult
	d.Dispatch(persondetect.AlertEvent{EventID: 7, CameraID: "garden", Fire: persondetect.FireSet{Mission: true}},
		func(_ string, result persondetect.AlertResult) {
			mu.Lock()
			defer mu.Unlock()
			got = result
		})
	d.Wait()
	if got != (persondetect.AlertResult{Status: persondetect.AlertSkipped, Detail: "missions unavailable"}) {
		t.Fatalf("mission = %+v", got)
	}
}

// The person detection service built by the server must report to this slice's dispatcher.
func TestPersonDetectServiceGetsTheAlertDispatcher(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	literals, callers := 0, 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		if idx := strings.Index(src, "persondetect.Deps{"); idx >= 0 {
			literals++
			if window := src[idx:min(len(src), idx+2000)]; !strings.Contains(window, "Alerts:") {
				t.Errorf("%s: persondetect.Deps has no Alerts field", file)
			}
		}
		callers += strings.Count(src, "newPersonAlertDispatcher()") - strings.Count(src, "func (s *Server) newPersonAlertDispatcher()")
	}
	if literals != 1 || callers < 1 {
		t.Fatalf("persondetect.Deps literals = %d, newPersonAlertDispatcher() calls = %d; want 1 and at least 1", literals, callers)
	}
}
