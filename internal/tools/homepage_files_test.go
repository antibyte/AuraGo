package tools

import (
	"strings"
	"testing"
	"time"
)

func TestHomepageWriteFileRepairsProjectBeforeWriting(t *testing.T) {
	oldInternalExec := homepageDockerExecInternalFunc
	oldExec := homepageDockerExecFunc
	defer func() {
		homepageDockerExecInternalFunc = oldInternalExec
		homepageDockerExecFunc = oldExec
	}()

	dockerHost := "unit-test-write-project-repair"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	defer invalidateDockerAvailabilityCache(dockerHost)

	projectChecks := 0
	repairedProject := false
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		switch {
		case strings.Contains(cmd, "/workspace/ki-news/.aurago-project-write-test"):
			projectChecks++
			if projectChecks == 1 {
				return `{"status":"error","exit_code":1,"output":"touch: Permission denied"}`
			}
			return `{"status":"ok","exit_code":0,"output":""}`
		case strings.Contains(cmd, "id -u"):
			return `{"status":"ok","exit_code":0,"output":"1001:1001\n"}`
		case user == "0:0" && strings.Contains(cmd, "chown -R 1001:1001 /workspace/ki-news"):
			repairedProject = true
			return `{"status":"ok","exit_code":0,"output":""}`
		default:
			t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
			return `{"status":"error","exit_code":1}`
		}
	}

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		if !repairedProject {
			return `{"status":"error","exit_code":1,"output":"/bin/sh: 1: cannot create /workspace/ki-news/src/main.ts: Permission denied"}`
		}
		if strings.Contains(command, `src\main.ts`) {
			t.Fatalf("write command used Windows separators in container path: %s", command)
		}
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	got := HomepageWriteFile(HomepageConfig{DockerHost: dockerHost}, "ki-news/src/main.ts", "console.log('ok')", slogDiscard())
	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected write_file to succeed after repair, got: %s", got)
	}
	if projectChecks < 2 || !repairedProject {
		t.Fatalf("expected project writability repair, checks=%d repaired=%v", projectChecks, repairedProject)
	}
}

func TestHomepageWriteFileReportsDockerOutputOnFailure(t *testing.T) {
	oldInternalExec := homepageDockerExecInternalFunc
	oldExec := homepageDockerExecFunc
	defer func() {
		homepageDockerExecInternalFunc = oldInternalExec
		homepageDockerExecFunc = oldExec
	}()

	dockerHost := "unit-test-write-failure-message"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	defer invalidateDockerAvailabilityCache(dockerHost)

	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		if strings.Contains(cmd, "/workspace/ki-news/.aurago-project-write-test") {
			return `{"status":"ok","exit_code":0,"output":""}`
		}
		t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
		return `{"status":"error","exit_code":1}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"error","exit_code":1,"output":"/bin/sh: 1: cannot create /workspace/ki-news/src/main.ts: Permission denied"}`
	}

	got := HomepageWriteFile(HomepageConfig{DockerHost: dockerHost}, "ki-news/src/main.ts", "console.log('ok')", slogDiscard())
	if !strings.Contains(got, "Permission denied") {
		t.Fatalf("expected write_file error to include Docker output, got: %s", got)
	}
}
