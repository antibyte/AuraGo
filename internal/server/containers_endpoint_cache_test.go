package server

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/tools"
)

// reset forgets the cached lookup and the warning state between tests.
func (c *containerEndpointLookupCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.host, c.addrs, c.err, c.expires = "", nil, nil, time.Time{}
	c.warnHost, c.warnedAt = "", time.Time{}
}

// TestAdminContainerListCachesTheEndpointLookup: GET /api/containers reuses one
// docker.host lookup for containerEndpointCacheTTL and warns about a failed
// lookup once per failure streak, then at most every containerEndpointWarnInterval (M5).
func TestAdminContainerListCachesTheEndpointLookup(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/json") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		http.NotFound(w, r)
	})
	var lookups atomic.Int32
	var fail atomic.Bool
	old := containerDockerEndpointAddresses
	containerDockerEndpointAddresses = func(context.Context, string) ([]string, error) {
		lookups.Add(1)
		if fail.Load() {
			return nil, fmt.Errorf("lookup docker-proxy: i/o timeout")
		}
		return []string{"172.18.0.5"}, nil
	}
	listEndpointLookup.reset()
	t.Cleanup(func() { containerDockerEndpointAddresses = old; listEndpointLookup.reset() })
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	oldNow := containerEndpointNow
	containerEndpointNow = func() time.Time { return now }
	t.Cleanup(func() { containerEndpointNow = oldNow })

	var logs bytes.Buffer
	s := testContainerServer(true, false)
	s.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	s.Cfg.Docker.Host = host
	list := func() {
		t.Helper()
		rec := httptest.NewRecorder()
		handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
		}
	}
	warnings := func() int { return strings.Count(logs.String(), "Docker endpoint lookup failed") }
	step := containerEndpointCacheTTL + time.Second

	list()
	list()
	if got := lookups.Load(); got != 1 {
		t.Fatalf("lookups = %d, want 1 within the TTL", got)
	}
	now = now.Add(step)
	list()
	if got := lookups.Load(); got != 2 {
		t.Fatalf("lookups = %d, want 2 after the TTL", got)
	}

	fail.Store(true)
	now = now.Add(step)
	list()
	list() // cached failure: no second 2 s lookup
	now = now.Add(step)
	list() // fails again; the warning stays suppressed
	if got := lookups.Load(); got != 4 {
		t.Fatalf("lookups = %d, want 4 (failures are cached for the TTL too)", got)
	}
	if got := warnings(); got != 1 {
		t.Fatalf("warnings = %d, want 1 per interval", got)
	}
	now = now.Add(containerEndpointWarnInterval)
	list()
	if got := warnings(); got != 2 {
		t.Fatalf("warnings = %d, want 2 after the interval", got)
	}

	// After a successful lookup the next failure warns at once.
	fail.Store(false)
	now = now.Add(step)
	list()
	fail.Store(true)
	now = now.Add(step)
	list()
	if got := warnings(); got != 3 {
		t.Fatalf("warnings = %d, want 3: a new failure streak warns again", got)
	}
}

// TestContainerActionsResolveTheEndpointEveryTime pins that terminal, update
// and remove never use the list cache.
func TestContainerActionsResolveTheEndpointEveryTime(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/web/json") {
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/web","Config":{"Labels":{}}}`))
			return
		}
		http.NotFound(w, r)
	})
	var lookups atomic.Int32
	old := containerDockerEndpointAddresses
	containerDockerEndpointAddresses = func(context.Context, string) ([]string, error) {
		lookups.Add(1)
		return []string{"172.18.0.5"}, nil
	}
	t.Cleanup(func() { containerDockerEndpointAddresses = old })
	t.Cleanup(replaceContainerSelfHostname("aurago-host"))
	t.Cleanup(replaceContainerSelfProcFiles(nil))
	s := testContainerServer(true, false)
	for i := 0; i < 3; i++ {
		_ = classifyContainerForAction(context.Background(), s, tools.DockerConfig{Host: host}, "web")
	}
	if got := lookups.Load(); got != 3 {
		t.Fatalf("lookups = %d, want 3: actions resolve every time", got)
	}
}
