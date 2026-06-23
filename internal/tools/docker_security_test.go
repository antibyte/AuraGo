package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type dockerSecurityTestLogger struct{}

func (dockerSecurityTestLogger) Info(string, ...any)  {}
func (dockerSecurityTestLogger) Warn(string, ...any)  {}
func (dockerSecurityTestLogger) Error(string, ...any) {}

func configureDockerSecurityTestPermissions(t *testing.T, readOnly bool) {
	t.Helper()
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true, DockerReadOnly: readOnly})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
}

func fakeDockerHost(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return "tcp://" + strings.TrimPrefix(server.URL, "http://")
}

func TestPullImageWaitAllowsLocalImageButRejectsReadOnlyPull(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)

	t.Run("local image does not require mutation permission", func(t *testing.T) {
		var postSeen bool
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				postSeen = true
				t.Fatalf("unexpected Docker pull request for local image: %s", r.URL.String())
			}
			if r.Method != http.MethodGet || r.URL.Path != "/"+dockerAPIVersion+"/images/json" {
				t.Fatalf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"Id":"sha256:local"}]`))
		})

		err := PullImageWait(context.Background(), DockerConfig{Host: host}, "ghcr.io/example/local:latest", nil)
		if err != nil {
			t.Fatalf("PullImageWait() error = %v, want nil for local image", err)
		}
		if postSeen {
			t.Fatal("local image path should not try to pull")
		}
	})

	t.Run("missing image requires mutation permission before pull", func(t *testing.T) {
		var postSeen bool
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/"+dockerAPIVersion+"/images/json" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
				return
			}
			if r.Method == http.MethodPost && r.URL.Path == "/"+dockerAPIVersion+"/images/create" {
				postSeen = true
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"pulled"}` + "\n"))
				return
			}
			t.Fatalf("unexpected Docker request: %s %s", r.Method, r.URL.String())
		})

		err := PullImageWait(context.Background(), DockerConfig{Host: host}, "ghcr.io/example/missing:latest", nil)
		if err == nil || !strings.Contains(err.Error(), "docker mutation is disabled") {
			t.Fatalf("PullImageWait() error = %v, want docker read-only denial", err)
		}
		if postSeen {
			t.Fatal("read-only pull should be denied before POST /images/create")
		}
	})
}

func TestVideoDownloadDockerRequestContextRejectsMutationWhenReadOnly(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	var called bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":"container-id"}`))
	})

	_, _, err := dockerRequestContext(context.Background(), DockerConfig{Host: host}, http.MethodPost, "/containers/create?name=yt", `{}`)
	if err == nil || !strings.Contains(err.Error(), "docker mutation is disabled") {
		t.Fatalf("dockerRequestContext() error = %v, want docker read-only denial", err)
	}
	if called {
		t.Fatal("read-only video download mutation should be denied before Docker request")
	}
}

func TestHomepageBuildImageRejectsReadOnlyBeforeDockerAPIRequest(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	var called bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"stream":"ok"}` + "\n"))
	})

	result := homepageBuildImage(DockerConfig{Host: host})
	if !strings.Contains(result, "docker mutation is disabled") {
		t.Fatalf("homepageBuildImage() = %s, want docker read-only denial", result)
	}
	if called {
		t.Fatal("homepage build should be denied before Docker API request")
	}
}

func TestDockerCreatePayloadBindsRejectSensitiveHostPathBeforeRequest(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var called bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":"container-id"}`))
	})
	payload, err := json.Marshal(map[string]any{
		"Image": "alpine:latest",
		"HostConfig": map[string]any{
			"Binds": []string{"/var/run/docker.sock:/sock"},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, _, err = dockerRequest(DockerConfig{Host: host}, http.MethodPost, "/containers/create?name=unsafe", string(payload))
	if err == nil || !strings.Contains(err.Error(), "mounting sensitive host path") {
		t.Fatalf("dockerRequest() error = %v, want bind validation denial", err)
	}
	if called {
		t.Fatal("unsafe create payload should be denied before Docker request")
	}
}

func TestDockerRequestContextPayloadBindsRejectWorkspaceEscape(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	var called bool
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":"container-id"}`))
	})
	payload, err := json.Marshal(map[string]any{
		"Image": "alpine:latest",
		"HostConfig": map[string]any{
			"Binds": []string{outside + ":/data"},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, _, err = DockerRequestContext(context.Background(), DockerConfig{Host: host, WorkspaceDir: workspace}, http.MethodPost, "/containers/create", string(payload))
	if err == nil || !strings.Contains(err.Error(), "must stay within the configured workspace") {
		t.Fatalf("DockerRequestContext() error = %v, want workspace bind denial", err)
	}
	if called {
		t.Fatal("workspace-escaping create payload should be denied before Docker request")
	}
}

func TestDockerComposeRejectsComposeFileOutsideWorkspace(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	composeFile := filepath.Join(outside, "compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	result := DockerCompose(DockerConfig{WorkspaceDir: workspace}, composeFile, "config")
	if !strings.Contains(result, "must stay within the configured workspace") {
		t.Fatalf("DockerCompose() = %s, want workspace denial", result)
	}
}

func TestDockerComposeResolvesRelativeFileAgainstWorkspace(t *testing.T) {
	workspace := t.TempDir()
	composeFile := filepath.Join(workspace, "compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	got, err := validateDockerComposeFilePath(DockerConfig{WorkspaceDir: workspace}, "compose.yml")
	if err != nil {
		t.Fatalf("validateDockerComposeFilePath() returned error: %v", err)
	}
	if got != composeFile {
		t.Fatalf("compose path = %q, want %q", got, composeFile)
	}
}

func TestCLIBuildsRejectReadOnlyBeforeRunningDocker(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	markerDir := prependFakeCommandsToPath(t, "docker", "git")
	logger := dockerSecurityTestLogger{}

	if err := buildAnsibleImage("aurago-ansible:test", t.TempDir(), logger); err == nil || !strings.Contains(err.Error(), "docker mutation is disabled") {
		t.Fatalf("buildAnsibleImage() error = %v, want docker read-only denial", err)
	}
	if err := buildBrowserAutomationImage("aurago-browser:test", t.TempDir(), logger); err == nil || !strings.Contains(err.Error(), "docker mutation is disabled") {
		t.Fatalf("buildBrowserAutomationImage() error = %v, want docker read-only denial", err)
	}
	sourcePath := filepath.Join(t.TempDir(), "space-agent")
	if err := os.MkdirAll(filepath.Join(sourcePath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir fake git repo: %v", err)
	}
	cfg := SpaceAgentSidecarConfig{
		Image:          "aurago-space-agent:test",
		SourcePath:     sourcePath,
		DataPath:       filepath.Join(t.TempDir(), "space-data"),
		CustomwarePath: filepath.Join(t.TempDir(), "customware"),
		AdminUser:      "admin",
		GitRef:         "main",
		RepoURL:        "https://example.invalid/space-agent.git",
	}
	if err := ensureSpaceAgentSourceAndImage(cfg, logger); err == nil || !strings.Contains(err.Error(), "docker mutation is disabled") {
		t.Fatalf("ensureSpaceAgentSourceAndImage() error = %v, want docker read-only denial", err)
	}
	if entries, err := os.ReadDir(markerDir); err != nil {
		t.Fatalf("read marker dir: %v", err)
	} else if len(entries) != 0 {
		t.Fatalf("external commands should not run under read-only Docker permissions, marker files = %d", len(entries))
	}
}

func prependFakeCommandsToPath(t *testing.T, names ...string) string {
	t.Helper()
	binDir := t.TempDir()
	markerDir := t.TempDir()
	for _, name := range names {
		var path string
		var content []byte
		if runtime.GOOS == "windows" {
			path = filepath.Join(binDir, name+".bat")
			content = []byte("@echo off\r\necho ran > \"" + filepath.Join(markerDir, name+".marker") + "\"\r\nexit /b 0\r\n")
		} else {
			path = filepath.Join(binDir, name)
			content = []byte("#!/bin/sh\nprintf ran > " + shellQuoteDockerSecurityTest(filepath.Join(markerDir, name+".marker")) + "\nexit 0\n")
		}
		if err := os.WriteFile(path, content, 0o755); err != nil {
			t.Fatalf("write fake %s command: %v", name, err)
		}
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return markerDir
}

func shellQuoteDockerSecurityTest(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
