package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/i18n"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/ui"
)

func TestStorymakerAgentNotifiesOncePerJob(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	s, _, token := storymakerAgentRouteFixture(t, func(_ context.Context, _ render.Job, _ any, h *render.Handle) (render.Output, error) {
		h.SetAgent(storymaker.AgentJobInfo{Outcome: storymaker.OutcomeReadyUnverified})
		return render.Output{}, nil
	})
	_, hub, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	events, unsubscribe, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)

	want := map[string]bool{}
	for _, topic := range []string{"Erste Story", "Zweite Story"} {
		w := storymakerRequest(s, token, http.MethodPost, "/agent/jobs", strings.NewReader(`{"topic":"`+topic+`"}`), nil)
		if w.Code != http.StatusAccepted {
			t.Fatalf("start = %d %s", w.Code, w.Body.String())
		}
		var started struct {
			ProjectID string `json:"project_id"`
		}
		decodeBody(t, w, &started)
		want[started.ProjectID] = true
	}

	// File writes also reach the hub, so only notifications count.
	notificationProject := func(event desktop.Event) string {
		t.Helper()
		if event.Type != "notification" {
			return ""
		}
		payload, _ := event.Payload.(map[string]any)
		launch, _ := payload["context"].(map[string]any)
		id, _ := launch["project_id"].(string)
		if payload["appId"] != "storymaker" || payload["type"] != "info" || payload["title"] != "Your story is ready. Please watch it once." {
			t.Fatalf("notification = %+v", payload)
		}
		return id
	}
	seen := map[string]bool{}
	deadline := time.After(10 * time.Second)
	for len(seen) < len(want) {
		select {
		case event, open := <-events:
			if !open {
				t.Fatal("the hub closed")
			}
			if id := notificationProject(event); id != "" {
				if !want[id] || seen[id] {
					t.Fatalf("unexpected or repeated notification for project %s", id)
				}
				seen[id] = true
			}
		case <-deadline:
			t.Fatalf("notified %d of %d jobs", len(seen), len(want))
		}
	}
	quiet := time.After(300 * time.Millisecond)
	for {
		select {
		case event, open := <-events:
			if !open {
				return
			}
			if id := notificationProject(event); id != "" {
				t.Fatalf("a repeated notification for project %s", id)
			}
		case <-quiet:
			return
		}
	}
}
