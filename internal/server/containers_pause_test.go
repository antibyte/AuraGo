package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"aurago/internal/tools"
)

// TestContainerPauseAndUnpauseRoutesReachDocker pins the routes the API
// reference documents (they answered 404). Like start and stop they never
// consult container protection, and read-only mode refuses them.
func TestContainerPauseAndUnpauseRoutesReachDocker(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, "/containers/web/pause") || strings.HasSuffix(r.URL.Path, "/containers/web/unpause")) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
	})
	old := containerProtectionFor
	containerProtectionFor = func(context.Context, *Server, tools.DockerConfig, string) containerProtection {
		t.Fatal("pause and unpause must not consult container protection")
		return containerProtection{}
	}
	t.Cleanup(func() { containerProtectionFor = old })

	for _, action := range []string{"pause", "unpause"} {
		rec := httptest.NewRecorder()
		handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/web/"+action, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"action":"`+action+`"`) {
			t.Fatalf("%s = %d %s, want 200 ok", action, rec.Code, rec.Body.String())
		}
		rec = httptest.NewRecorder()
		handleContainerAction(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers/web/"+action, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s = %d, want 405", action, rec.Code)
		}
	}
	// A Docker refusal (already paused, not running) answers 502 like start and stop.
	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/other/pause", nil))
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "engine refused") {
		t.Fatalf("refused pause = %d %s, want 502 with Docker's message", rec.Code, rec.Body.String())
	}

	s.Cfg.Docker.ReadOnly = true
	for _, action := range []string{"pause", "unpause"} {
		rec := httptest.NewRecorder()
		handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/web/"+action, nil))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("read-only %s = %d, want 403", action, rec.Code)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"POST /v1.45/containers/web/pause", "POST /v1.45/containers/web/unpause", "POST /v1.45/containers/other/pause"}; !slices.Equal(seen, want) {
		t.Fatalf("Docker requests = %v, want %v", seen, want)
	}
}
