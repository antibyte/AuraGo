package virtualcomputers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type SSHExecutor interface {
	Run(ctx context.Context, command string) (string, error)
}

type SetupManager struct {
	Executor SSHExecutor
	Token    string
}

type PreflightResult struct {
	Supported bool              `json:"supported"`
	Checks    map[string]string `json:"checks"`
	Issues    []string          `json:"issues,omitempty"`
}

func (m SetupManager) Preflight(ctx context.Context) (PreflightResult, error) {
	if m.Executor == nil {
		return PreflightResult{}, fmt.Errorf("ssh executor is not configured")
	}
	out, err := m.Executor.Run(ctx, "printf 'ARCH='; uname -m; printf 'HAS_KVM='; test -e /dev/kvm && echo 1 || echo 0; . /etc/os-release 2>/dev/null; printf 'OS_ID=%s\\n' \"$ID\"")
	if err != nil {
		return PreflightResult{}, err
	}
	return ParsePreflightOutput(out), nil
}

func ParsePreflightOutput(out string) PreflightResult {
	checks := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		checks[strings.ToUpper(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	var issues []string
	if arch := checks["ARCH"]; arch != "" && arch != "x86_64" && arch != "amd64" {
		issues = append(issues, "unsupported architecture: boring-computers setup currently expects x86_64")
	}
	if checks["HAS_KVM"] != "1" {
		issues = append(issues, "KVM is not available on the host")
	}
	if osID := strings.ToLower(checks["OS_ID"]); osID != "" && osID != "ubuntu" {
		issues = append(issues, "Ubuntu is required by the upstream setup script")
	}
	return PreflightResult{
		Supported: len(issues) == 0,
		Checks:    checks,
		Issues:    issues,
	}
}

func (m SetupManager) Install(ctx context.Context) (SetupStatus, error) {
	if m.Executor == nil {
		return SetupStatus{}, fmt.Errorf("ssh executor is not configured")
	}
	preflight, err := m.Preflight(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	if !preflight.Supported {
		return SetupStatus{Configured: false, Healthy: false, Message: strings.Join(preflight.Issues, "; ")}, nil
	}
	return SetupStatus{Configured: true, Healthy: true, Message: "host passed preflight; boringd install/update can run idempotently"}, nil
}

func (m SetupManager) RedactInstallLog(log string) string {
	return redactInstallLog(log, m.Token)
}

var installSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(BORING_TOKEN=)([^\s]+)`),
	regexp.MustCompile(`(?i)(ANTHROPIC_API_KEY=)([^\s]+)`),
	regexp.MustCompile(`(?i)(OPENROUTER_API_KEY=)([^\s]+)`),
	regexp.MustCompile(`(?i)(AWS_ACCESS_KEY_ID=)([^\s]+)`),
	regexp.MustCompile(`(?i)(AWS_SECRET_ACCESS_KEY=)([^\s]+)`),
}

func redactInstallLog(log string, secrets ...string) string {
	out := redactSecrets(log, secrets...)
	for _, pattern := range installSecretPatterns {
		out = pattern.ReplaceAllString(out, "${1}<redacted>")
	}
	return out
}
