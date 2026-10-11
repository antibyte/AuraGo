package ruview

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestRuViewDockerOwnershipAndBoundedResources(t *testing.T) {
	previous := artifactsJSON
	t.Cleanup(func() { artifactsJSON = previous })
	artifactsJSON = []byte(`{"docker_image":"test@sha256:` + strings.Repeat("a", 64) + `"}`)
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Enabled = true
	c.RuView.ReadOnly = false
	c.Docker.Enabled = true
	c.Directories.DataDir = t.TempDir()
	c.RuView.UDPEnabled = true
	c.RuView.UDPBind = "192.168.1.2"
	c.RuView.UDPAllow = []string{"192.168.1.4"}
	var payload map[string]any
	owned, started, stopped := false, false, false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := regexp.MustCompile(`^/v[0-9.]+/`).ReplaceAllString(r.URL.Path, "/")
		switch {
		case path == "/_ping":
			w.Header().Set("API-Version", "1.51")
			io.WriteString(w, "OK")
		case path == "/version":
			io.WriteString(w, `{"ApiVersion":"1.51","MinAPIVersion":"1.24"}`)
		case path == "/containers/aurago-ruview/json":
			if !owned {
				w.WriteHeader(404)
				io.WriteString(w, `{}`)
			} else {
				json.NewEncoder(w).Encode(map[string]any{"Id": "owned-id", "Config": map[string]any{"Labels": map[string]string{"aurago.managed": "ruview", "aurago.ruview.owner": owner(c)}}})
			}
		case strings.HasPrefix(path, "/images/"):
			io.WriteString(w, `{}`)
		case path == "/containers/create":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			owned = true
			w.WriteHeader(201)
			io.WriteString(w, `{"Id":"owned-id"}`)
		case path == "/containers/owned-id/start":
			started = true
			w.WriteHeader(204)
		case path == "/containers/owned-id/stop":
			stopped = true
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected Docker request %s %s", r.Method, path)
			w.WriteHeader(500)
		}
	}))
	defer api.Close()
	c.Docker.Host = "tcp://" + strings.TrimPrefix(api.URL, "http://")
	m := New(func() *config.Config { return c }, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := m.startDocker(context.Background(), c, filepath.Join(c.Directories.DataDir, "ruview/data"), "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("not started")
	}
	host := payload["HostConfig"].(map[string]any)
	if host["Memory"].(float64) != 2<<30 || host["ReadonlyRootfs"] != true || host["PidsLimit"].(float64) != 128 {
		t.Fatalf("unbounded resources: %+v", host)
	}
	ports := host["PortBindings"].(map[string]any)
	if len(ports) != 2 || ports["3001/tcp"] != nil {
		t.Fatal("unexpected exposure")
	}
	if ports["3000/tcp"].([]any)[0].(map[string]any)["HostIp"] != "127.0.0.1" {
		t.Fatal("HTTP exposed to LAN")
	}
	m.runningConfig = c
	m.managed = true
	if err := m.Stop(context.Background()); err != nil || !stopped {
		t.Fatalf("stop failed %v", err)
	}
	stopped = false
	if err := m.Stop(context.Background()); err != nil || !stopped {
		t.Fatalf("owned container could not be stopped after manager restart: %v", err)
	}
}

func TestRuViewDockerPullReportsBoundedScrubbedReason(t *testing.T) {
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.Docker.Enabled = true
	c.Directories.DataDir = t.TempDir()
	const secret = "RuView-fixture-registry-secret"
	security.RegisterSensitive(secret)
	pulls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := regexp.MustCompile(`^/v[0-9.]+/`).ReplaceAllString(r.URL.Path, "/")
		switch {
		case path == "/version":
			io.WriteString(w, `{"ApiVersion":"1.51","MinAPIVersion":"1.24"}`)
		case path == "/containers/aurago-ruview/json", strings.HasPrefix(path, "/images/") && path != "/images/create":
			w.WriteHeader(http.StatusNotFound)
		case path == "/images/create":
			pulls++
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"message": "registry denied\n\u202e " + secret + strings.Repeat("x", 1024)})
		default:
			t.Errorf("unexpected Docker request %s %s", r.Method, path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer api.Close()
	c.Docker.Host = "tcp://" + strings.TrimPrefix(api.URL, "http://")
	m := New(func() *config.Config { return c }, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := m.startDocker(context.Background(), c, c.Directories.DataDir, "fixture-secret")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403: registry denied") || pulls != 1 {
		t.Fatalf("missing pull failure or unexpected retry: %v (pulls=%d)", err, pulls)
	}
	if text := err.Error(); len(text) > 560 || strings.ContainsAny(text, "\n\r\u202e") || strings.Contains(text, secret) {
		t.Fatal("pull failure was not bounded, sanitized and scrubbed")
	}
}

func TestRuViewProcessChild(t *testing.T) {
	if os.Getenv("AURAGO_RUVIEW_TEST_CHILD") != "1" {
		return
	}
	time.Sleep(time.Minute)
}
func TestRuViewProcessStopsAndWaits(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestRuViewProcessChild$")
	cmd.Env = append(os.Environ(), "AURAGO_RUVIEW_TEST_CHILD=1")
	p, err := launchNative(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer p.kill()
	if err = p.kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("native process did not stop")
	}
}

func TestRuViewRuntimeArgsPreserveUDPAndAvoidAutomaticSimulation(t *testing.T) {
	c := &config.Config{RuView: config.DefaultRuViewConfig()}
	c.RuView.Mode = "native"
	c.RuView.UDPPort = 5010
	c.RuView.UDPEnabled = true
	c.RuView.UDPBind = "10.0.0.2"
	c.RuView.UDPAllow = []string{"10.0.0.3"}
	if args := strings.Join(runtimeArgs(c, "/data", "127.0.0.1", 3022), " "); !strings.Contains(args, "--source esp32") || !strings.Contains(args, "--udp-port 5010") || strings.Contains(args, "auto") {
		t.Fatal(args)
	}
	if env := strings.Join(runtimeEnv(c, "fixture", "127.0.0.1"), " "); !strings.Contains(env, "RUVIEW_UDP_BIND=10.0.0.2") {
		t.Fatal("UDP LAN bind was discarded")
	}
}
