package proxy

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/tools"
)

// allowDockerForTest sets the process-wide Docker gates that the real tools
// helpers check and restores the previous gates afterwards.
func allowDockerForTest(t *testing.T, readOnly bool) {
	t.Helper()
	previous, configured := tools.CurrentRuntimePermissionsForTest()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true, DockerReadOnly: readOnly})
	t.Cleanup(func() {
		if configured {
			tools.ConfigureRuntimePermissions(previous)
			return
		}
		tools.ClearRuntimePermissionsForTest()
	})
}

// fakeDockerDaemon answers the Engine's version negotiation and hands every
// other request to handler. It returns the tcp:// host.
func fakeDockerDaemon(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return "tcp://" + strings.TrimPrefix(server.URL, "http://")
}

// missingImagesEngine answers like runningEngine, except that no proxy image
// exists yet and the tag request succeeds.
func missingImagesEngine(image string) func(method, endpoint, body string) ([]byte, int, error) {
	running := runningEngine(image)
	return func(method, endpoint, body string) ([]byte, int, error) {
		switch {
		case method == "GET" && strings.HasPrefix(endpoint, "/images/"):
			return nil, 404, nil
		case method == "POST" && strings.HasPrefix(endpoint, "/images/") && strings.Contains(endpoint, "/tag?"):
			return nil, 201, nil
		}
		return running(method, endpoint, body)
	}
}

func TestManagerStartPullsAndTagsOfficialImageWhenMissing(t *testing.T) {
	cfg := proxyConfig()
	fake := &fakeEngine{handle: missingImagesEngine(imageName)}
	m := testManager(t, cfg, fake)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if want := []string{"caddy:latest"}; !reflect.DeepEqual(fake.pulls, want) {
		t.Fatalf("pulls = %#v, want %#v", fake.pulls, want)
	}
	if !fake.called("POST /images/caddy:latest/tag?repo=aurago-proxy&tag=latest") {
		t.Fatal("Start did not tag caddy:latest as " + imageName)
	}
	if len(fake.builds) != 0 {
		t.Fatalf("builds = %#v, want none without rate limiting", fake.builds)
	}
	payload := decodeCreatePayload(t, fake.body("POST /containers/create?name="+containerName))
	if payload["Image"] != imageName {
		t.Fatalf("Image = %#v, want %s", payload["Image"], imageName)
	}
}

// pullStreamFailure is a pull the Engine answers with 200 and then fails
// inside the progress stream.
const pullStreamFailure = `{"status":"Pulling fs layer","progressDetail":{},"id":"4f4fb700ef54"}` + "\n" +
	`{"errorDetail":{"message":"failed to register layer: no space left on device"},"error":"failed to register layer: no space left on device"}` + "\n"

func TestManagerStartFailsOnCaddyPullStreamError(t *testing.T) {
	allowDockerForTest(t, false)
	var pulls atomic.Int32
	host := fakeDockerDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images/create") {
			pulls.Add(1)
			if got := r.URL.Query().Get("fromImage"); got != "caddy:latest" {
				t.Errorf("fromImage = %q, want caddy:latest", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(pullStreamFailure))
			return
		}
		t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
	})
	cfg := proxyConfig()
	cfg.SecurityProxy.DockerHost = host
	fake := &fakeEngine{handle: missingImagesEngine(imageName)}
	m := testManager(t, cfg, fake)
	// The real pull helper reads the Engine's progress stream.
	m.engine.pull = dockerEngine.pull
	path, previous := writeRunningCaddyfile(t, cfg)

	err := m.Start()
	if err == nil || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("Start() error = %v, want the error event from the pull stream", err)
	}
	if pulls.Load() != 1 {
		t.Fatalf("pull requests = %d, want 1", pulls.Load())
	}
	if fake.called("POST /images/caddy:latest/tag") {
		t.Fatal("Start tagged caddy:latest although its pull failed")
	}
	assertRunningProxyUntouched(t, fake, path, previous)
}

func TestManagerStartExplainsRateLimitBuildRefusedByReadOnly(t *testing.T) {
	allowDockerForTest(t, true)
	cfg := proxyConfig()
	cfg.Docker.ReadOnly = true
	cfg.SecurityProxy.RateLimiting.Enabled = true
	fake := &fakeEngine{handle: missingImagesEngine(imageName)}
	m := testManager(t, cfg, fake)
	// The real build helper applies the docker.read_only gate.
	m.engine.build = dockerEngine.build
	path, previous := writeRunningCaddyfile(t, cfg)

	err := m.Start()
	if !errors.Is(err, ErrRateLimitImageReadOnly) {
		t.Fatalf("Start() error = %v, want ErrRateLimitImageReadOnly", err)
	}
	if !errors.Is(err, ErrRateLimitImageUnavailable) {
		t.Fatalf("Start() error = %v, want it to stay an ErrRateLimitImageUnavailable", err)
	}
	assertRunningProxyUntouched(t, fake, path, previous)
}

// The manuals give a host-side build for setups without build access. It must
// produce exactly the image the proxy looks for, so it follows the pins.
func TestManualRateLimitImageRecipeMatchesPins(t *testing.T) {
	for _, lang := range []string{"en", "de"} {
		path := filepath.Join("..", "..", "documentation", "manual", lang, "08-integrations.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lines := map[string]bool{}
		for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
			lines[strings.TrimSpace(line)] = true
		}
		if want := "docker build -t " + rateLimitImageName + " - <<'EOF'"; !lines[want] {
			t.Errorf("%s lacks the line %q", path, want)
		}
		for _, line := range strings.Split(string(rateLimitDockerfile()), "\n") {
			if line = strings.TrimSpace(line); line != "" && !lines[line] {
				t.Errorf("%s lacks the Dockerfile line %q", path, line)
			}
		}
	}
}

func TestManagerStartKeepsBuildHintWhenBuildFailsWithoutReadOnly(t *testing.T) {
	cfg := proxyConfig()
	cfg.SecurityProxy.RateLimiting.Enabled = true
	fake := &fakeEngine{handle: missingImagesEngine(imageName)}
	fake.build = func(string, []byte) error { return errors.New("build image: HTTP 403: build is disabled") }
	m := testManager(t, cfg, fake)

	err := m.Start()
	if !errors.Is(err, ErrRateLimitImageUnavailable) {
		t.Fatalf("Start() error = %v, want ErrRateLimitImageUnavailable", err)
	}
	if errors.Is(err, ErrRateLimitImageReadOnly) {
		t.Fatalf("Start() error = %v, must not blame docker.read_only for a refused build endpoint", err)
	}
}
