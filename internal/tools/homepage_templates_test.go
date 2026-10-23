package tools

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// templateArchiveEngine serves the archive upload used by the Homepage
// template writer and records the container paths it was asked to write to.
func templateArchiveEngine(t *testing.T, handler http.HandlerFunc) DockerConfig {
	t.Helper()
	dockerCfg := newArchiveTestEngine(t, handler)
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerCfg.Host] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	t.Cleanup(func() { invalidateDockerAvailabilityCache(dockerCfg.Host) })
	return dockerCfg
}

func TestApplyHomepageTemplateDockerUsesLinuxPathsAndReportsMessage(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()

	var uploadPaths []string
	dockerCfg := templateArchiveEngine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
		}
		uploadPaths = append(uploadPaths, r.URL.Query().Get("path"))
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})
	homepageDockerExecFunc = func(DockerConfig, string, string, string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	got := applyHomepageTemplate(HomepageConfig{DockerHost: dockerCfg.Host}, "ki-news", "blog", slogDiscard())
	if got != "" {
		t.Fatalf("expected template write to succeed with slash paths, got: %s", got)
	}
	if len(uploadPaths) == 0 {
		t.Fatal("expected the template writer to upload the template files as archives")
	}
	for _, uploadPath := range uploadPaths {
		if strings.Contains(uploadPath, `\`) {
			t.Fatalf("container path used Windows path separators: %s", uploadPath)
		}
		if uploadPath != "/workspace/ki-news" && !strings.HasPrefix(uploadPath, "/workspace/ki-news/") {
			t.Fatalf("container path is not rooted in the project workspace: %s", uploadPath)
		}
	}
}

func TestApplyHomepageTemplateDockerIncludesMessageOnWriteFailure(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()

	dockerCfg := templateArchiveEngine(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"permission denied writing template"}`)
	})
	homepageDockerExecFunc = func(DockerConfig, string, string, string) string {
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	got := applyHomepageTemplate(HomepageConfig{DockerHost: dockerCfg.Host}, "ki-news", "blog", slogDiscard())
	if !strings.Contains(got, "permission denied writing template") {
		t.Fatalf("expected the upload failure message to appear in the template error, got: %s", got)
	}
}

// TestApplyHomepageTemplateDockerNeverPutsFileContentInAShellArgument proves the
// template writer uses the archive transport: no template content may reach a
// DockerExec command.
func TestApplyHomepageTemplateDockerNeverPutsFileContentInAShellArgument(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()

	var commands []string
	homepageDockerExecFunc = func(_ DockerConfig, _, command, _ string) string {
		commands = append(commands, command)
		return `{"status":"ok","exit_code":0,"output":""}`
	}
	dockerCfg := templateArchiveEngine(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})

	if got := applyHomepageTemplate(HomepageConfig{DockerHost: dockerCfg.Host}, "ki-news", "blog", slogDiscard()); got != "" {
		t.Fatalf("expected the template write to succeed, got: %s", got)
	}
	for _, command := range commands {
		if strings.Contains(command, "base64") || strings.Contains(command, "printf") {
			t.Fatalf("template content must not be written through a shell argument: %s", command)
		}
	}
}
