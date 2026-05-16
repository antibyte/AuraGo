package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDockerUpdateContainerImageRecreatesRunningContainerWithFreshImage(t *testing.T) {
	var calls []string
	var createPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		calls = append(calls, r.Method+" "+path)

		switch {
		case r.Method == http.MethodGet && path == "/containers/demo/json":
			writeDockerJSON(w, http.StatusOK, map[string]interface{}{
				"Id":   "old-container-id",
				"Name": "/manifest",
				"State": map[string]interface{}{
					"Running": true,
				},
				"Config": map[string]interface{}{
					"Image":        "manifestdotbuild/manifest:5",
					"Env":          []string{"MANIFEST_PORT=3000"},
					"Cmd":          []string{"start"},
					"Labels":       map[string]string{"app": "manifest"},
					"ExposedPorts": map[string]interface{}{"3000/tcp": map[string]interface{}{}},
				},
				"HostConfig": map[string]interface{}{
					"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
					"Binds":         []string{"manifest-data:/app/data"},
					"PortBindings":  map[string]interface{}{"3000/tcp": []map[string]string{{"HostPort": "3000"}}},
					"NetworkMode":   "bridge",
				},
				"NetworkSettings": map[string]interface{}{
					"Networks": map[string]interface{}{
						"bridge": map[string]interface{}{
							"Aliases": []string{"manifest"},
						},
					},
				},
			})
		case r.Method == http.MethodPost && path == "/images/create":
			if got := r.URL.Query().Get("fromImage"); got != "manifestdotbuild/manifest:5" {
				t.Fatalf("fromImage = %q", got)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"pulled"}` + "\n"))
		case r.Method == http.MethodPost && path == "/containers/demo/stop":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && path == "/containers/demo/rename":
			if !strings.HasPrefix(r.URL.Query().Get("name"), "manifest-aurago-old-") {
				t.Fatalf("backup name = %q", r.URL.Query().Get("name"))
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && path == "/containers/create":
			if got := r.URL.Query().Get("name"); got != "manifest" {
				t.Fatalf("create name = %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			writeDockerJSON(w, http.StatusCreated, map[string]string{"Id": "new-container-id"})
		case r.Method == http.MethodPost && path == "/containers/new-container-id/start":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "/containers/manifest-aurago-old-"):
			if r.URL.Query().Get("force") != "true" {
				t.Fatalf("old container removal should be forced")
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected Docker request: %s %s raw=%s", r.Method, path, r.URL.String())
		}
	}))
	defer server.Close()

	host := "tcp://" + strings.TrimPrefix(server.URL, "http://")
	raw := DockerUpdateContainerImage(context.Background(), DockerConfig{Host: host}, "demo", nil)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("parse result: %v\n%s", err, raw)
	}
	if result["status"] != "ok" {
		t.Fatalf("status = %v, result=%s", result["status"], raw)
	}
	if result["image"] != "manifestdotbuild/manifest:5" {
		t.Fatalf("image = %v", result["image"])
	}
	if createPayload["Image"] != "manifestdotbuild/manifest:5" {
		t.Fatalf("create payload Image = %v", createPayload["Image"])
	}
	if _, ok := createPayload["HostConfig"].(map[string]interface{}); !ok {
		t.Fatalf("create payload missing HostConfig: %#v", createPayload)
	}
	if _, ok := createPayload["NetworkingConfig"].(map[string]interface{}); !ok {
		t.Fatalf("create payload missing NetworkingConfig: %#v", createPayload)
	}

	wantOrder := []string{
		"GET /containers/demo/json",
		"POST /images/create",
		"POST /containers/demo/stop",
		"POST /containers/demo/rename",
		"POST /containers/create",
		"POST /containers/new-container-id/start",
	}
	for i, want := range wantOrder {
		if i >= len(calls) || calls[i] != want {
			t.Fatalf("call[%d] = %q, want %q; all calls=%v", i, calls[i], want, calls)
		}
	}
}

func TestDockerUpdateContainerImageRejectsImageIDOnlyContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeDockerJSON(w, http.StatusOK, map[string]interface{}{
			"Id":     "old-container-id",
			"Name":   "/anonymous",
			"State":  map[string]interface{}{"Running": false},
			"Config": map[string]interface{}{"Image": "sha256:1234567890abcdef"},
		})
	}))
	defer server.Close()

	host := "tcp://" + strings.TrimPrefix(server.URL, "http://")
	raw := DockerUpdateContainerImage(context.Background(), DockerConfig{Host: host}, "demo", nil)
	if !strings.Contains(raw, "reusable image reference") {
		t.Fatalf("result = %s, want reusable image reference error", raw)
	}
}

func writeDockerJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
