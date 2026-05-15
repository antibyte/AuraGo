package tools

import (
	"strings"
	"testing"
	"time"
)

func TestApplyHomepageTemplateDockerUsesLinuxPathsAndReportsMessage(t *testing.T) {
	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()

	dockerHost := "unit-test-template"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	defer invalidateDockerAvailabilityCache(dockerHost)

	var calls []string
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		calls = append(calls, command)
		if strings.Contains(command, `src\styles`) {
			return `{"status":"error","message":"bad windows path in container command","exit_code":1}`
		}
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	got := applyHomepageTemplate(HomepageConfig{DockerHost: dockerHost}, "ki-news", "blog", slogDiscard())
	if got != "" {
		t.Fatalf("expected template write to succeed with slash paths, got: %s", got)
	}
	if len(calls) == 0 {
		t.Fatal("expected template writer to execute commands through homepageDockerExecFunc")
	}
	for _, call := range calls {
		if strings.Contains(call, `src\styles`) {
			t.Fatalf("container command used Windows path separators: %s", call)
		}
	}
}

func TestApplyHomepageTemplateDockerIncludesMessageOnWriteFailure(t *testing.T) {
	oldExec := homepageDockerExecFunc
	defer func() { homepageDockerExecFunc = oldExec }()

	dockerHost := "unit-test-template-error"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	defer invalidateDockerAvailabilityCache(dockerHost)

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"error","message":"permission denied writing template","exit_code":1}`
	}

	got := applyHomepageTemplate(HomepageConfig{DockerHost: dockerHost}, "ki-news", "blog", slogDiscard())
	if !strings.Contains(got, "permission denied writing template") {
		t.Fatalf("expected DockerExec message to appear in template error, got: %s", got)
	}
}
