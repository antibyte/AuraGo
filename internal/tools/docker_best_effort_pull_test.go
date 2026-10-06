package tools

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

// bestEffortSidecarDockerHost answers like an engine without the sidecar
// container, streams pullBody for the image pull and refuses the create so the
// start ends before any readiness wait.
func bestEffortSidecarDockerHost(t *testing.T, pullBody string) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/containers/json":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
			writeDockerJSON(w, http.StatusNotFound, map[string]string{"message": "No such container"})
		case r.Method == http.MethodPost && path == "/images/create":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, pullBody)
		case r.Method == http.MethodPost && path == "/containers/create":
			writeDockerJSON(w, http.StatusInternalServerError, map[string]string{"message": "end of test"})
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestEnsurePiperRunningLogsPullStreamErrorAndStillCreates(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	cfg := &config.Config{}
	cfg.Docker.Host = bestEffortSidecarDockerHost(t, pullStreamFailure)
	cfg.TTS.Piper.Enabled = true
	cfg.TTS.Piper.Voice = "de_DE-thorsten-high"
	cfg.TTS.Piper.Image = "rhasspy/wyoming-piper:fp5-test"
	cfg.TTS.Piper.DataPath = t.TempDir()
	var logs bytes.Buffer
	EnsurePiperRunning(cfg, slog.New(slog.NewTextHandler(&logs, nil)))
	text := logs.String()
	if !strings.Contains(text, "Image pull failed, trying to create container anyway") || !strings.Contains(text, "no space left on device") {
		t.Fatalf("logs = %s, want the stream's pull error", text)
	}
	if !strings.Contains(text, "Failed to create container") {
		t.Fatalf("logs = %s, want the create to run after a failed pull (create anyway)", text)
	}
}

func TestEnsureSupertonicRunningLogsPullStreamErrorAndStillCreates(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	t.Cleanup(func() { setSupertonicLifecycle("idle") })
	cfg := &config.Config{}
	cfg.Docker.Host = bestEffortSidecarDockerHost(t, pullStreamFailure)
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.AutoStart = true
	cfg.TTS.Supertonic.Image = "ghcr.io/antibyte/aurago-supertonic:fp5-test"
	cfg.TTS.Supertonic.DataPath = t.TempDir()
	var logs bytes.Buffer
	EnsureSupertonicRunning(cfg, slog.New(slog.NewTextHandler(&logs, nil)))
	text := logs.String()
	if !strings.Contains(text, "Image pull failed, trying to create container anyway") || !strings.Contains(text, "no space left on device") {
		t.Fatalf("logs = %s, want the stream's pull error", text)
	}
	if !strings.Contains(text, "Failed to create container") {
		t.Fatalf("logs = %s, want the create to run after a failed pull (create anyway)", text)
	}
}

// bestEffortSidecarStarts runs each managed TTS sidecar start against host and
// returns its log.
var bestEffortSidecarStarts = map[string]func(t *testing.T, host string) string{
	"piper": func(t *testing.T, host string) string {
		cfg := &config.Config{}
		cfg.Docker.Host = host
		cfg.TTS.Piper.Enabled = true
		cfg.TTS.Piper.Voice = "de_DE-thorsten-high"
		cfg.TTS.Piper.Image = "rhasspy/wyoming-piper:fp5-test"
		cfg.TTS.Piper.DataPath = t.TempDir()
		var logs bytes.Buffer
		EnsurePiperRunning(cfg, slog.New(slog.NewTextHandler(&logs, nil)))
		return logs.String()
	},
	"supertonic": func(t *testing.T, host string) string {
		t.Cleanup(func() { setSupertonicLifecycle("idle") })
		cfg := &config.Config{}
		cfg.Docker.Host = host
		cfg.TTS.Provider = "supertonic"
		cfg.TTS.Supertonic.AutoStart = true
		cfg.TTS.Supertonic.Image = "ghcr.io/antibyte/aurago-supertonic:fp5-test"
		cfg.TTS.Supertonic.DataPath = t.TempDir()
		var logs bytes.Buffer
		EnsureSupertonicRunning(cfg, slog.New(slog.NewTextHandler(&logs, nil)))
		return logs.String()
	},
}

// countingSidecarDockerHost is bestEffortSidecarDockerHost with a chosen pull
// status and counters for the pull and create requests that reach the Engine.
func countingSidecarDockerHost(t *testing.T, pullStatus int, pullBody string, pulls, creates *atomic.Int32) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/containers/json":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
			writeDockerJSON(w, http.StatusNotFound, map[string]string{"message": "No such container"})
		case r.Method == http.MethodPost && path == "/images/create":
			pulls.Add(1)
			w.WriteHeader(pullStatus)
			_, _ = io.WriteString(w, pullBody)
		case r.Method == http.MethodPost && path == "/containers/create":
			creates.Add(1)
			writeDockerJSON(w, http.StatusInternalServerError, map[string]string{"message": "end of test"})
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// Pins: in read-only mode no pull reaches the Engine and the start still goes
// on to the create, which the same gate then refuses, as with the request client.
func TestBestEffortSidecarPullsKeepTheReadOnlyGate(t *testing.T) {
	for name, start := range bestEffortSidecarStarts {
		t.Run(name, func(t *testing.T) {
			configureDockerSecurityTestPermissions(t, true)
			var pulls, creates atomic.Int32
			text := start(t, countingSidecarDockerHost(t, http.StatusOK, pullStreamSuccess, &pulls, &creates))
			if pulls.Load() != 0 || creates.Load() != 0 {
				t.Fatalf("pulls = %d, creates = %d reached the Engine in read-only mode, want 0 and 0", pulls.Load(), creates.Load())
			}
			if !strings.Contains(text, "Image pull failed, trying to create container anyway") || !strings.Contains(text, "docker mutation is disabled") {
				t.Fatalf("logs = %s, want the read-only denial of the pull", text)
			}
			if !strings.Contains(text, "Failed to create container") {
				t.Fatalf("logs = %s, want the create to be attempted after the denied pull", text)
			}
		})
	}
}

// Pins: a non-2xx pull answer logs "returned unexpected status" with its code,
// and the create still runs.
func TestBestEffortSidecarPullsLogUnexpectedStatusAndStillCreate(t *testing.T) {
	for name, start := range bestEffortSidecarStarts {
		t.Run(name, func(t *testing.T) {
			configureDockerSecurityTestPermissions(t, false)
			var pulls, creates atomic.Int32
			text := start(t, countingSidecarDockerHost(t, http.StatusNotFound, `{"message":"manifest unknown"}`, &pulls, &creates))
			if !strings.Contains(text, "Image pull returned unexpected status") || !strings.Contains(text, "code=404") || !strings.Contains(text, "manifest unknown") {
				t.Fatalf("logs = %s, want the status branch with Docker's reason", text)
			}
			if pulls.Load() != 1 || creates.Load() != 1 {
				t.Fatalf("pulls = %d, creates = %d, want one pull and the create after it", pulls.Load(), creates.Load())
			}
		})
	}
}

func TestCloudflarePullImageReportsStreamErrorAndSkipsPresentImage(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var pulls atomic.Int32
	host := pullOnlyDockerHost(t, http.StatusOK, pullStreamFailure, &pulls)
	if err := pullImage(DockerConfig{Host: host}, cfdImageName, logger); err == nil || !strings.Contains(err.Error(), "no space left on device") || pulls.Load() != 1 {
		t.Fatalf("pullImage() = %v after %d pulls, want the stream error after one pull", err, pulls.Load())
	}
	present := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/"+dockerAPIVersion+"/images/json" {
			t.Errorf("unexpected Docker request for a present image: %s %s", r.Method, r.URL.String())
		}
		_, _ = io.WriteString(w, `[{"Id":"sha256:present"}]`)
	})
	if err := pullImage(DockerConfig{Host: present}, cfdImageName, logger); err != nil {
		t.Fatalf("pullImage() = %v for a present image, want nil", err)
	}
}
