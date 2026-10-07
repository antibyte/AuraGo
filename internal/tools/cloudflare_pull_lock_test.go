package tools

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

// The cloudflared pull runs while tunnelMu is held. Before the shared pull
// helper it ran on the 60-second request client, so a config save, a status
// request or shutdown waited at most about a minute for the lock.
func TestCloudflaredPullKeepsTodaysLockBound(t *testing.T) {
	if cloudflaredPullTimeout != 60*time.Second {
		t.Fatalf("cloudflaredPullTimeout = %s, want the 60 s bound of the request client", cloudflaredPullTimeout)
	}
}

func TestCloudflareStartNeverHoldsTunnelLockForASlowPull(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	previous := cloudflaredPullTimeout
	cloudflaredPullTimeout = 300 * time.Millisecond
	t.Cleanup(func() { cloudflaredPullTimeout = previous })

	pullStarted := make(chan struct{})
	release := make(chan struct{})
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/images/json":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodPost && path == "/images/create":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"status":"Pulling fs layer","id":"4f4fb700ef54"}`+"\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(pullStarted)
			select { // a registry that stalls: the pull never ends on its own
			case <-r.Context().Done():
			case <-release:
			}
		case r.Method == http.MethodPost && path == "/containers/create":
			writeDockerJSON(w, http.StatusInternalServerError, map[string]string{"message": "end of test"})
		default: // removal of an old container
			w.WriteHeader(http.StatusNotFound)
		}
	})
	t.Cleanup(func() { close(release) })

	vault, err := security.NewVault(strings.Repeat("5a", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("cloudflared_token", "fp5-test-token"); err != nil {
		t.Fatal(err)
	}
	cfg := CloudflareTunnelConfig{Enabled: true, Mode: "docker", AuthMethod: "token", DockerHost: host, DataDir: t.TempDir()}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	started := make(chan string, 1)
	go func() { started <- CloudflareTunnelStart(cfg, vault, nil, logger) }()

	select {
	case <-pullStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("the cloudflared pull never started")
	}
	locked := make(chan time.Duration, 1)
	waitStart := time.Now()
	go func() {
		tunnelMu.Lock() // what CloudflareTunnelShutdown and the status endpoints do
		locked <- time.Since(waitStart)
		tunnelMu.Unlock()
	}()
	select {
	case waited := <-locked:
		if waited > 5*time.Second {
			t.Fatalf("tunnelMu was held for %s during the pull", waited)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tunnelMu is still held 5 s into a stalled pull; the pull is not bounded like the request client")
	}
	select {
	case result := <-started:
		if !strings.Contains(result, "Failed to create cloudflared container") {
			t.Fatalf("start result = %s, want the create to run after the cut pull (create anyway)", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("CloudflareTunnelStart did not return")
	}
}
