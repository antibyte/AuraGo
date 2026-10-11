package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplyShellSandboxDockerGrantRespectsPermission(t *testing.T) {
	SetShellSandboxDockerHost("unix:///var/run/docker.sock")
	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ClearRuntimePermissionsForTest()
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
	ClearRuntimePermissionsForTest()

	denied := exec.Command("true")
	denied.Env = []string{"PATH=/bin"}
	ApplyShellSandboxDockerGrant(denied)
	if strings.Join(denied.Env, "\n") != "PATH=/bin" {
		t.Fatalf("grant without permission changed env: %v", denied.Env)
	}

	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	allowed := exec.Command("true")
	allowed.Env = []string{"AURAGO_SBX_RW=/work"}
	ApplyShellSandboxDockerGrant(allowed)
	sock := grantedDockerSocketSegment("/var/run/docker.sock")
	if !containsEnv(allowed.Env, "DOCKER_HOST=unix:///var/run/docker.sock") {
		t.Fatalf("missing docker host: %v", allowed.Env)
	}
	if !containsEnvPrefix(allowed.Env, "AURAGO_SBX_RW=/work:"+sock) &&
		!containsEnvPrefix(allowed.Env, "AURAGO_SBX_RW="+sock+":/work") {
		t.Fatalf("socket not granted: %v", allowed.Env)
	}
	if !containsEnv(allowed.Env, "AURAGO_SBX_RW_REQUIRED="+sock) &&
		!containsEnvPrefix(allowed.Env, "AURAGO_SBX_RW_REQUIRED=/work:"+sock) &&
		!containsEnvPrefix(allowed.Env, "AURAGO_SBX_RW_REQUIRED="+sock+":/work") {
		t.Fatalf("socket not required: %v", allowed.Env)
	}
}

func TestApplyShellSandboxDockerGrantTCPHostAddsNoFilesystemPath(t *testing.T) {
	SetShellSandboxDockerHost("tcp://127.0.0.1:2375")
	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})

	cmd := exec.Command("true")
	cmd.Env = []string{"AURAGO_SBX_RW=/work", "AURAGO_SBX_RW_REQUIRED=/work"}
	ApplyShellSandboxDockerGrant(cmd)
	if !containsEnv(cmd.Env, "DOCKER_HOST=tcp://127.0.0.1:2375") {
		t.Fatalf("missing tcp docker host: %v", cmd.Env)
	}
	if !containsEnv(cmd.Env, "AURAGO_SBX_RW=/work") || !containsEnv(cmd.Env, "AURAGO_SBX_RW_REQUIRED=/work") {
		t.Fatalf("tcp host changed sandbox paths: %v", cmd.Env)
	}
}

func TestApplyShellSandboxDockerGrantSkipsSocketPathsThatSplitTheList(t *testing.T) {
	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})

	for _, host := range []string{
		"unix:///tmp/foo:bar",
		"unix:///tmp/foo\nbar",
		"unix:///tmp/foo\x00bar",
	} {
		SetShellSandboxDockerHost(host)
		cmd := exec.Command("true")
		cmd.Env = []string{"AURAGO_SBX_RW=/work", "AURAGO_SBX_RW_REQUIRED=/work"}
		ApplyShellSandboxDockerGrant(cmd)
		if !containsEnv(cmd.Env, "DOCKER_HOST="+host) {
			t.Fatalf("missing docker host %q: %v", host, cmd.Env)
		}
		if !containsEnv(cmd.Env, "AURAGO_SBX_RW=/work") || !containsEnv(cmd.Env, "AURAGO_SBX_RW_REQUIRED=/work") {
			t.Fatalf("unsafe socket path %q changed sandbox lists: %v", host, cmd.Env)
		}
	}
}

func TestApplyShellSandboxDockerGrantDoesNotDuplicateSocketPath(t *testing.T) {
	SetShellSandboxDockerHost("unix:///var/run/docker.sock")
	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	sock := grantedDockerSocketSegment("/var/run/docker.sock")

	cmd := exec.Command("true")
	cmd.Env = []string{
		"AURAGO_SBX_RW=/work:" + sock,
		"AURAGO_SBX_RW_REQUIRED=/work:" + sock,
		"DOCKER_HOST=unix:///var/run/docker.sock",
	}
	ApplyShellSandboxDockerGrant(cmd)
	if strings.Join(cmd.Env, "\n") != strings.Join([]string{
		"AURAGO_SBX_RW=/work:" + sock,
		"AURAGO_SBX_RW_REQUIRED=/work:" + sock,
		"DOCKER_HOST=unix:///var/run/docker.sock",
	}, "\n") {
		t.Fatalf("socket path was duplicated: %v", cmd.Env)
	}

	partial := exec.Command("true")
	partial.Env = []string{"AURAGO_SBX_RW=/work:" + sock}
	ApplyShellSandboxDockerGrant(partial)
	if !containsEnv(partial.Env, "AURAGO_SBX_RW=/work:"+sock) {
		t.Fatalf("existing socket path was rewritten: %v", partial.Env)
	}
	if !containsEnv(partial.Env, "AURAGO_SBX_RW_REQUIRED="+sock) {
		t.Fatalf("missing required socket: %v", partial.Env)
	}
	if !containsEnv(partial.Env, "DOCKER_HOST=unix:///var/run/docker.sock") {
		t.Fatalf("missing docker host: %v", partial.Env)
	}
}

func TestApplyShellSandboxDockerGrantLeavesInheritedEnvNil(t *testing.T) {
	SetShellSandboxDockerHost("unix:///var/run/docker.sock")
	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})

	cmd := exec.Command("true")
	ApplyShellSandboxDockerGrant(cmd)
	if cmd.Env != nil {
		t.Fatalf("grant replaced an inherited environment: %v", cmd.Env)
	}
}

func containsEnv(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

func containsEnvPrefix(env []string, prefix string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return true
		}
	}
	return false
}

// grantedDockerSocketSegment is the path AddUnixSocketGrant appends.
// Windows keeps the raw POSIX socket. Linux uses the Landlock absolute and
// symlink rule, including a cleaned absolute path when the socket is absent.
func grantedDockerSocketSegment(path string) string {
	if runtime.GOOS != "linux" {
		return path
	}
	if path == "" || strings.ContainsAny(path, ":\n\x00") {
		return path
	}
	if _, err := os.Lstat(path); err != nil {
		if !os.IsNotExist(err) {
			return path
		}
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return path
		}
		cleaned := filepath.Clean(abs)
		if strings.ContainsAny(cleaned, ":\n\x00") {
			return path
		}
		return cleaned
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil || strings.ContainsAny(resolved, ":\n\x00") {
		return path
	}
	return resolved
}
