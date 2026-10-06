package tools

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func homepageBuildHost(t *testing.T, status int, contentType, body string) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/"+dockerAPIVersion+"/build" {
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

func decodeHomepageResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return result
}

func TestHomepageBuildImageExplainsProxyRefusal(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	t.Cleanup(func() { homepageBuildRefused.Store(false) })
	host := homepageBuildHost(t, http.StatusForbidden, "text/html", "<html><body><h1>403 Forbidden</h1>\nRequest forbidden by administrative rules.\n</body></html>\n")
	result := decodeHomepageResult(t, homepageBuildImage(DockerConfig{Host: host}))
	message, _ := result["message"].(string)
	if result["status"] != "error" || result["code"] != "homepage_image_build_forbidden" || result["build_command"] != homepageHostBuildCommand {
		t.Fatalf("result = %#v, want the coded refusal with the host build command", result)
	}
	for _, want := range []string{"HTTP 403", "BUILD=0", "BUILD=1", homepageHostBuildCommand, "--print-homepage-dockerfile"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message = %q, want it to name %q", message, want)
		}
	}
	if !homepageBuildRefused.Load() {
		t.Fatal("the refusal must be remembered for the status API")
	}
}

func TestHomepageBuildImageKeepsOtherResults(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	t.Cleanup(func() { homepageBuildRefused.Store(false) })
	if got := decodeHomepageResult(t, homepageBuildImage(DockerConfig{Host: homepageBuildHost(t, http.StatusInternalServerError, "application/json", "")}))["message"]; got != "Image build failed with status 500" {
		t.Fatalf("500 message = %v", got)
	}
	if got := decodeHomepageResult(t, homepageBuildImage(DockerConfig{Host: homepageBuildHost(t, http.StatusOK, "application/json", `{"stream":"Step 1/1"}`+"\n"+`{"error":"boom"}`+"\n")}))["message"]; got != "Image build error: boom" {
		t.Fatalf("stream error message = %v", got)
	}
	homepageBuildRefused.Store(true)
	result := decodeHomepageResult(t, homepageBuildImage(DockerConfig{Host: homepageBuildHost(t, http.StatusOK, "application/json", `{"stream":"Successfully built"}`+"\n")}))
	if result["status"] != "ok" || homepageBuildRefused.Load() {
		t.Fatalf("result = %#v, refused = %v; a successful build must clear the refusal", result, homepageBuildRefused.Load())
	}
}

func TestHomepageStatusReportsRefusedBuildUntilTheImageExists(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	t.Cleanup(func() { homepageBuildRefused.Store(false) })
	var imagePresent atomic.Bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case path == "/_ping":
			_, _ = io.WriteString(w, "OK")
		case strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
			writeDockerJSON(w, http.StatusNotFound, map[string]string{"message": "No such container"})
		case path == "/images/json":
			if imagePresent.Load() {
				_, _ = io.WriteString(w, `[{"Id":"sha256:homepage"}]`)
				return
			}
			_, _ = io.WriteString(w, `[]`)
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := HomepageConfig{DockerHost: host}
	homepageBuildRefused.Store(true)
	status := decodeHomepageResult(t, HomepageStatus(cfg, logger))
	if status["image_build_refused"] != true || status["image_build_command"] != homepageHostBuildCommand {
		t.Fatalf("status = %#v, want the refused build with the command", status)
	}
	imagePresent.Store(true)
	status = decodeHomepageResult(t, HomepageStatus(cfg, logger))
	if _, ok := status["image_build_refused"]; ok || homepageBuildRefused.Load() {
		t.Fatalf("status = %#v, want no refusal once the image exists", status)
	}
}

func TestHomepageDockerfileIsTheBuildDockerfile(t *testing.T) {
	if HomepageDockerfile() != homepageDockerfile || !strings.HasPrefix(HomepageDockerfile(), "FROM ") {
		t.Fatal("HomepageDockerfile must return the embedded dev-image Dockerfile")
	}
}
