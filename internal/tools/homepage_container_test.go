package tools

import (
	"strings"
	"testing"
)

func TestHomepageContainerVersionsPinned(t *testing.T) {
	if homepageWebImage != "caddy:2.11.2-alpine" {
		t.Fatalf("expected pinned caddy image, got %q", homepageWebImage)
	}

	requiredSnippets := []string{
		"FROM mcr.microsoft.com/playwright:v1.59.1-noble",
		"ARG CLOUDFLARED_VERSION=2026.3.0",
		`apt-get install -y --no-install-recommends`,
		`arch="$(dpkg --print-architecture)"`,
		`amd64) cloudflared_arch="amd64" ;;`,
		`arm64) cloudflared_arch="arm64" ;;`,
		`npm cache clean --force`,
		`ENV NODE_ENV=development`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(homepageDockerfile, snippet) {
			t.Fatalf("expected homepageDockerfile to contain %q", snippet)
		}
	}
}

func TestHomepageEnsureWorkspaceWritableRepairsBindMountAsContainerUser(t *testing.T) {
	oldExec := homepageDockerExecInternalFunc
	defer func() { homepageDockerExecInternalFunc = oldExec }()

	type execCall struct {
		cmd  string
		user string
	}
	var calls []execCall
	writeChecks := 0

	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		calls = append(calls, execCall{cmd: cmd, user: user})
		switch {
		case strings.Contains(cmd, ".aurago-write-test"):
			writeChecks++
			if writeChecks == 1 {
				return `{"status":"error","exit_code":1,"output":"touch: Permission denied"}`
			}
			return `{"status":"ok","exit_code":0,"output":""}`
		case strings.Contains(cmd, "id -u"):
			return `{"status":"ok","exit_code":0,"output":"1001:1001\n"}`
		case user == "0:0" && strings.Contains(cmd, "chown -R 1001:1001 /workspace"):
			return `{"status":"ok","exit_code":0,"output":""}`
		default:
			t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
			return `{"status":"error","exit_code":1}`
		}
	}

	if err := homepageEnsureWorkspaceWritable(DockerConfig{}, slogDiscard()); err != nil {
		t.Fatalf("homepageEnsureWorkspaceWritable returned error: %v", err)
	}

	foundRootRepair := false
	for _, call := range calls {
		if call.user == "0:0" && strings.Contains(call.cmd, "chown -R 1001:1001 /workspace") {
			foundRootRepair = true
			break
		}
	}
	if !foundRootRepair {
		t.Fatalf("expected root repair chown call, got %#v", calls)
	}
	if writeChecks < 2 {
		t.Fatalf("expected workspace writability to be rechecked after repair, got %d checks", writeChecks)
	}
}

func TestHomepageExecRepairsWorkspaceBeforeRunningCommand(t *testing.T) {
	oldExec := homepageDockerExecInternalFunc
	defer func() { homepageDockerExecInternalFunc = oldExec }()

	var ranUserCommand bool
	repaired := false
	writeChecks := 0

	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		switch {
		case strings.Contains(cmd, ".aurago-write-test"):
			writeChecks++
			if writeChecks == 1 {
				return `{"status":"error","exit_code":1,"output":"touch: Permission denied"}`
			}
			return `{"status":"ok","exit_code":0,"output":""}`
		case strings.Contains(cmd, "id -u"):
			return `{"status":"ok","exit_code":0,"output":"1001:1001\n"}`
		case user == "0:0" && strings.Contains(cmd, "chown -R 1001:1001 /workspace"):
			repaired = true
			return `{"status":"ok","exit_code":0,"output":""}`
		case cmd == "npm install":
			if !repaired {
				return `{"status":"error","exit_code":1,"output":"npm ERR! EACCES: permission denied"}`
			}
			ranUserCommand = true
			return `{"status":"ok","exit_code":0,"output":"installed"}`
		default:
			t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
			return `{"status":"error","exit_code":1}`
		}
	}

	result := HomepageExec(HomepageConfig{}, "npm install", nil, slogDiscard())
	if !strings.Contains(result, `"output":"installed"`) {
		t.Fatalf("expected user command output, got: %s", result)
	}
	if !ranUserCommand {
		t.Fatal("expected HomepageExec to run the requested command after repair")
	}
	if writeChecks < 2 {
		t.Fatalf("expected HomepageExec to repair and recheck workspace, got %d write checks", writeChecks)
	}
}

func TestHomepageExecRejectsDirectWriteToGeneratedOutput(t *testing.T) {
	oldExec := homepageDockerExecInternalFunc
	defer func() { homepageDockerExecInternalFunc = oldExec }()

	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		t.Fatalf("HomepageExec should reject generated output writes before docker exec, got cmd=%q", cmd)
		return `{"status":"error"}`
	}

	cmd := "cat > /workspace/ki-news-new/dist/index.html << 'EOF'\n<html></html>\nEOF"
	result := HomepageExec(HomepageConfig{}, cmd, nil, slogDiscard())
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected error result, got: %s", result)
	}
	if !strings.Contains(result, "generated output") || !strings.Contains(result, "write_file") {
		t.Fatalf("expected generated-output guidance, got: %s", result)
	}
}

func TestHomepageExecAllowsReadOnlyGeneratedOutputInspection(t *testing.T) {
	oldExec := homepageDockerExecInternalFunc
	defer func() { homepageDockerExecInternalFunc = oldExec }()

	writeChecks := 0
	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		switch {
		case strings.Contains(cmd, ".aurago-write-test"):
			writeChecks++
			return `{"status":"ok","exit_code":0,"output":""}`
		case cmd == "ls -la /workspace/ki-news-new/dist/":
			return `{"status":"ok","exit_code":0,"output":"index.html"}`
		default:
			t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
			return `{"status":"error","exit_code":1}`
		}
	}

	result := HomepageExec(HomepageConfig{}, "ls -la /workspace/ki-news-new/dist/", nil, slogDiscard())
	if !strings.Contains(result, `"output":"index.html"`) {
		t.Fatalf("expected read-only inspection to run, got: %s", result)
	}
	if writeChecks == 0 {
		t.Fatal("expected workspace writability check before read-only command")
	}
}
