package virtualcomputers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type CommandRunner func(ctx context.Context, name string, args ...string) (string, error)

type LocalCommandExecutor struct {
	RuntimeGOOS    string
	RuntimeArch    string
	TempDir        string
	OSReleaseData  string
	OSReleasePath  string
	PathExists     func(path string) bool
	EffectiveUID   func() int
	DockerDetected func() bool
	CommandRunner  CommandRunner
}

func (e LocalCommandExecutor) Preflight(ctx context.Context) (string, error) {
	osID, osVersion := e.osRelease()
	checks := []string{
		"HOST_OS=" + e.goos(),
		"ARCH=" + e.arch(),
		"HAS_KVM=" + boolString(e.exists("/dev/kvm")),
		"OS_ID=" + osID,
		"OS_VERSION=" + osVersion,
		"RUNNING_IN_DOCKER=" + boolString(e.runningInDocker()),
		"HAS_SYSTEMD=" + boolString(e.hasSystemd()),
		"HAS_SUDO_OR_ROOT=" + boolString(e.hasSudoOrRoot(ctx)),
	}
	return strings.Join(checks, "\n") + "\n", nil
}

func (e LocalCommandExecutor) Run(ctx context.Context, command string) (string, error) {
	if e.goos() != "linux" {
		return "", fmt.Errorf("local boring-computers commands require Linux")
	}
	return e.runner()(ctx, "/bin/sh", "-c", command)
}

func (e LocalCommandExecutor) RunScript(ctx context.Context, script string) (string, error) {
	if e.goos() != "linux" {
		return "", fmt.Errorf("local boring-computers setup requires Linux")
	}
	tmp, err := os.CreateTemp(e.TempDir, "aurago-boring-setup-*.sh")
	if err != nil {
		return "", fmt.Errorf("create local setup script: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write local setup script: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close local setup script: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return "", fmt.Errorf("chmod local setup script: %w", err)
	}
	if e.euid() == 0 {
		return e.runner()(ctx, "bash", path)
	}
	return e.runner()(ctx, "sudo", "-n", "bash", path)
}

func (e LocalCommandExecutor) goos() string {
	if strings.TrimSpace(e.RuntimeGOOS) != "" {
		return strings.ToLower(strings.TrimSpace(e.RuntimeGOOS))
	}
	return runtime.GOOS
}

func (e LocalCommandExecutor) arch() string {
	if strings.TrimSpace(e.RuntimeArch) != "" {
		return strings.TrimSpace(e.RuntimeArch)
	}
	return runtime.GOARCH
}

func (e LocalCommandExecutor) exists(path string) bool {
	if e.PathExists != nil {
		return e.PathExists(path)
	}
	_, err := os.Stat(path)
	return err == nil
}

func (e LocalCommandExecutor) euid() int {
	if e.EffectiveUID != nil {
		return e.EffectiveUID()
	}
	return os.Geteuid()
}

func (e LocalCommandExecutor) runner() CommandRunner {
	if e.CommandRunner != nil {
		return e.CommandRunner
	}
	return func(ctx context.Context, name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return string(out), nil
	}
}

func (e LocalCommandExecutor) osRelease() (string, string) {
	data := e.OSReleaseData
	if strings.TrimSpace(data) == "" {
		path := strings.TrimSpace(e.OSReleasePath)
		if path == "" {
			path = "/etc/os-release"
		}
		raw, err := os.ReadFile(filepath.Clean(path))
		if err == nil {
			data = string(raw)
		}
	}
	values := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		values[strings.ToUpper(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values["ID"], values["VERSION_ID"]
}

func (e LocalCommandExecutor) runningInDocker() bool {
	if e.DockerDetected != nil {
		return e.DockerDetected()
	}
	if e.exists("/.dockerenv") {
		return true
	}
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false
	}
	cgroup := strings.ToLower(string(data))
	return strings.Contains(cgroup, "docker") ||
		strings.Contains(cgroup, "containerd") ||
		strings.Contains(cgroup, "kubepods")
}

func (e LocalCommandExecutor) hasSystemd() bool {
	return e.goos() == "linux" && e.exists("/run/systemd/system")
}

func (e LocalCommandExecutor) hasSudoOrRoot(ctx context.Context) bool {
	if e.goos() != "linux" {
		return false
	}
	if e.euid() == 0 {
		return true
	}
	_, err := e.runner()(ctx, "sudo", "-n", "true")
	return err == nil
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
