package virtualcomputers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sudoticket"
)

type CommandRunner func(ctx context.Context, name string, args ...string) (string, error)
type InputCommandRunner func(ctx context.Context, name, input string, args ...string) (string, error)

type LocalCommandExecutor struct {
	RuntimeGOOS        string
	RuntimeArch        string
	TempDir            string
	OSReleaseData      string
	OSReleasePath      string
	PathExists         func(path string) bool
	EffectiveUID       func() int
	DockerDetected     func() bool
	CommandRunner      CommandRunner
	SudoPassword       string
	InputCommandRunner InputCommandRunner
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
		"HAS_DOCKER=" + boolString(e.hasDocker(ctx)),
	}
	return strings.Join(checks, "\n") + "\n", nil
}

func (e LocalCommandExecutor) hasDocker(ctx context.Context) bool {
	if e.goos() != "linux" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := e.runner()(ctx, "docker", "info")
	return err == nil
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
	if e.hasSystemd() {
		return e.runScriptInTransientSystemdService(ctx, script)
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
	lease, authOut, err := e.sudoTicket(ctx)
	if err != nil {
		return authOut, err
	}
	defer lease.Release()
	out, err := e.runner()(ctx, "sudo", "-n", "bash", path)
	return out, lease.Explain(err)
}

// runScriptInTransientSystemdService pipes script into `systemd-run --pipe
// ... /bin/bash -s`. Its stdin is the script alone: a password line in front
// of it would run as the first root command.
func (e LocalCommandExecutor) runScriptInTransientSystemdService(ctx context.Context, script string) (string, error) {
	args := transientSystemdScriptArgs()
	if e.euid() == 0 {
		return e.inputRunner()(ctx, "systemd-run", script, args...)
	}
	lease, authOut, err := e.sudoTicket(ctx)
	if err != nil {
		return authOut, err
	}
	defer lease.Release()
	sudoArgs := append([]string{"-n", "systemd-run"}, args...)
	out, err := e.inputRunner()(ctx, "sudo", script, sudoArgs...)
	return out, lease.Explain(err)
}

// sudoTicket prepares one `sudo -n` run. Without a Vault password, or when
// `sudo -n true` already succeeds, it returns a nil lease and the run relies
// on root rules or NOPASSWD. Otherwise the password goes only to the shared
// sudo ticket (package sudoticket), never to the command's stdin.
func (e LocalCommandExecutor) sudoTicket(ctx context.Context) (*sudoticket.Lease, string, error) {
	if e.SudoPassword == "" {
		return nil, "", nil
	}
	if _, err := e.runner()(ctx, "sudo", "-n", "true"); err == nil {
		return nil, "", nil
	}
	return sudoticket.Acquire(ctx, "", e.SudoPassword)
}

func transientSystemdScriptArgs() []string {
	return []string{
		"--quiet",
		"--pipe",
		"--wait",
		"--collect",
		"--service-type=exec",
		"--property=ProtectSystem=no",
		"--property=PrivateTmp=no",
		"--property=NoNewPrivileges=no",
		"--setenv=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv=HOME=/root",
		"/bin/bash",
		"-s",
	}
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

func (e LocalCommandExecutor) inputRunner() InputCommandRunner {
	if e.InputCommandRunner != nil {
		return e.InputCommandRunner
	}
	return func(ctx context.Context, name, input string, args ...string) (string, error) {
		out, err := newInputCommand(ctx, name, input, args...).CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return string(out), nil
	}
}

// newInputCommand builds the process the default input runner starts, with
// input as its whole stdin.
func newInputCommand(ctx context.Context, name, input string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd
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
	if err == nil {
		return true
	}
	if e.SudoPassword == "" {
		return false
	}
	lease, _, err := sudoticket.Acquire(ctx, "", e.SudoPassword)
	if err != nil {
		return false
	}
	defer lease.Release()
	_, err = e.runner()(ctx, "sudo", "-n", "true")
	return err == nil
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
