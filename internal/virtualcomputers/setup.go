package virtualcomputers

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type CommandExecutor interface {
	Run(ctx context.Context, command string) (string, error)
}

type ScriptExecutor interface {
	RunScript(ctx context.Context, script string) (string, error)
}

type PreflightExecutor interface {
	Preflight(ctx context.Context) (string, error)
}

type SSHExecutor = CommandExecutor
type ScriptSSHExecutor = ScriptExecutor

const remotePreflightCommand = "printf 'HOST_OS='; uname -s | tr '[:upper:]' '[:lower:]'; printf 'ARCH='; uname -m; printf 'HAS_KVM='; test -e /dev/kvm && echo 1 || echo 0; . /etc/os-release 2>/dev/null; printf 'OS_ID=%s\\n' \"$ID\"; printf 'OS_VERSION=%s\\n' \"$VERSION_ID\"; printf 'RUNNING_IN_DOCKER='; if [ -f /.dockerenv ] || { [ -r /proc/self/cgroup ] && grep -qiE 'docker|containerd|kubepods' /proc/self/cgroup; }; then echo 1; else echo 0; fi; printf 'HAS_SYSTEMD='; if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then echo 1; else echo 0; fi; printf 'HAS_SUDO_OR_ROOT='; if [ \"$(id -u)\" -eq 0 ] || sudo -n true >/dev/null 2>&1; then echo 1; else echo 0; fi"
const defaultBoringdURL = "http://127.0.0.1:18080"

type SetupManager struct {
	Executor       CommandExecutor
	Token          string
	InstallOptions SetupInstallOptions
}

type PreflightResult struct {
	Supported bool              `json:"supported"`
	Checks    map[string]string `json:"checks"`
	Issues    []string          `json:"issues,omitempty"`
}

func (m SetupManager) Preflight(ctx context.Context) (PreflightResult, error) {
	if m.Executor == nil {
		return PreflightResult{}, fmt.Errorf("setup executor is not configured")
	}
	var out string
	var err error
	if preflight, ok := m.Executor.(PreflightExecutor); ok {
		out, err = preflight.Preflight(ctx)
	} else {
		out, err = m.Executor.Run(ctx, remotePreflightCommand)
	}
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
	if arch := checks["ARCH"]; arch != "" && arch != "x86_64" && arch != "amd64" && arch != "aarch64" && arch != "arm64" {
		issues = append(issues, "unsupported architecture: boring-computers setup currently expects x86_64/amd64 or aarch64/arm64")
	}
	if hostOS := strings.ToLower(checks["HOST_OS"]); hostOS != "" && hostOS != "linux" {
		issues = append(issues, "local boring-computers setup requires Linux; found "+hostOS)
	}
	if checks["HAS_KVM"] != "1" {
		issues = append(issues, "KVM is not available on the host")
	}
	if osID := strings.ToLower(checks["OS_ID"]); osID != "" && osID != "ubuntu" {
		issues = append(issues, "Ubuntu is required by the upstream setup script")
	}
	if checks["RUNNING_IN_DOCKER"] == "1" {
		issues = append(issues, "Docker container runtime is not supported for local boring-computers setup")
	}
	if hasSystemd := checks["HAS_SYSTEMD"]; hasSystemd != "" && hasSystemd != "1" {
		issues = append(issues, "systemd is required for local boring-computers setup")
	}
	if hasSudoOrRoot := checks["HAS_SUDO_OR_ROOT"]; hasSudoOrRoot != "" && hasSudoOrRoot != "1" {
		issues = append(issues, "root or passwordless sudo is required for local boring-computers setup")
	}
	return PreflightResult{
		Supported: len(issues) == 0,
		Checks:    checks,
		Issues:    issues,
	}
}

func (m SetupManager) Install(ctx context.Context) (SetupStatus, error) {
	if m.Executor == nil {
		return SetupStatus{}, fmt.Errorf("setup executor is not configured")
	}
	preflight, err := m.Preflight(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	if !preflight.Supported {
		return SetupStatus{Configured: false, Healthy: false, Message: strings.Join(preflight.Issues, "; "), Preflight: preflight}, nil
	}
	out, err := m.runInstall(ctx)
	if err != nil {
		msg := strings.TrimSpace(m.RedactInstallLog(out))
		if msg == "" {
			msg = m.RedactInstallLog(err.Error())
		}
		return SetupStatus{Configured: false, Healthy: false, Message: msg, Preflight: preflight}, fmt.Errorf("boring computers install failed: %w", err)
	}
	healthURL := boringdHealthURL(m.InstallOptions.BoringdURL)
	health, err := m.Executor.Run(ctx, "curl -fsS --max-time 8 "+shellQuote(healthURL))
	if err != nil {
		return SetupStatus{
			Configured:   true,
			Healthy:      false,
			Message:      strings.TrimSpace(m.RedactInstallLog(out + "\ncontrol-plane health check failed: " + err.Error())),
			Preflight:    preflight,
			ControlPlane: ComponentStatus{Configured: true, Healthy: false, Message: "boringd health check failed"},
			Management:   ComponentStatus{Configured: true, Healthy: false, Message: "management health was not checked"},
		}, nil
	}
	managementHealthURL := ManagementHealthURL(ManagementURL)
	managementHealth, err := m.Executor.Run(ctx, "curl -fsS --max-time 8 "+shellQuote(managementHealthURL))
	if err != nil {
		return SetupStatus{
			Configured:   true,
			Healthy:      false,
			Message:      strings.TrimSpace(m.RedactInstallLog(out + "\nmanagement health check failed: " + err.Error())),
			Preflight:    preflight,
			ControlPlane: ComponentStatus{Configured: true, Healthy: true},
			Management:   ComponentStatus{Configured: true, Healthy: false, Message: "management health check failed"},
		}, nil
	}
	return SetupStatus{
		Configured:   true,
		Healthy:      true,
		Message:      "boringd and management application installed and healthy: " + strings.TrimSpace(health) + " " + strings.TrimSpace(managementHealth),
		Preflight:    preflight,
		ControlPlane: ComponentStatus{Configured: true, Healthy: true},
		Management:   ComponentStatus{Configured: true, Healthy: true},
	}, nil
}

func (m SetupManager) RedactInstallLog(log string) string {
	return redactInstallLog(log, m.Token, m.InstallOptions.Token, m.InstallOptions.AnthropicKey, m.InstallOptions.OpenRouterKey, m.InstallOptions.S3AccessKeyID, m.InstallOptions.S3SecretKey)
}

func (m SetupManager) runInstall(ctx context.Context) (string, error) {
	script := m.installScript()
	if runner, ok := m.Executor.(ScriptExecutor); ok {
		return runner.RunScript(ctx, script)
	}
	return m.Executor.Run(ctx, heredocCommand("/tmp/aurago-boring-setup.sh", script))
}

func (m SetupManager) installScript() string {
	opts := m.InstallOptions
	installDir := strings.TrimSpace(opts.InstallDir)
	if installDir == "" {
		installDir = "/opt/boring"
	}
	token := strings.TrimSpace(opts.Token)
	if token == "" {
		token = strings.TrimSpace(m.Token)
	}
	boringdAddr := boringdListenAddr(opts.BoringdURL)
	healthURL := boringdHealthURL(opts.BoringdURL)
	maxMachines := opts.MaxRunningMachines
	if maxMachines <= 0 {
		maxMachines = 20
	}
	maxForks := opts.MaxForks
	if maxForks <= 0 {
		maxForks = 8
	}
	maxTemplates := 0
	if opts.AllowPublish {
		maxTemplates = 10
	}
	allowPersistent := "0"
	if opts.AllowPersistent {
		allowPersistent = "1"
	}
	guestNet := "0"
	if opts.AllowInternet {
		guestNet = "1"
	}
	skipDesktop := "0"
	if opts.SkipDesktop {
		skipDesktop = "1"
	}

	script := fmt.Sprintf(`set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
INSTALL_DIR=%s
REPO_URL="https://github.com/michaelshimeles/boring-computers.git"
BORING_REVISION=%s
GO_VERSION="1.25.0"
BORING_TOKEN_VALUE=%s
BORING_ANTHROPIC_KEY_VALUE=%s
BORING_OPENROUTER_KEY_VALUE=%s
BORING_S3_KEY_VALUE=%s
BORING_S3_SECRET_VALUE=%s
BORING_ADDR_VALUE=%s
BORING_HEALTH_URL_VALUE=%s
BORING_MAX_VALUE=%d
BORING_MAX_FORKS_VALUE=%d
BORING_MAX_TEMPLATES_VALUE=%d
BORING_ALLOW_PERSISTENT_VALUE=%s
BORING_NET_VALUE=%s
SKIP_DESKTOP_VALUE=%s

log() { printf '[aurago-boring-setup] %%s\n' "$*"; }

case "$(uname -m)" in
	x86_64|amd64) GOARCH="amd64" ;;
	aarch64|arm64) GOARCH="arm64" ;;
	*) echo "unsupported architecture: $(uname -m)" >&2; exit 2 ;;
esac

. /etc/os-release
if [ "${ID:-}" != "ubuntu" ]; then
	echo "Ubuntu is required by upstream boring-computers setup; found ${ID:-unknown}" >&2
	exit 2
fi
if [ ! -e /dev/kvm ]; then
	echo "/dev/kvm is missing; enable hardware or nested virtualization" >&2
	exit 2
fi

log "installing host dependencies"
apt-get update -y
apt-get install -y ca-certificates curl git rsync build-essential

REPO_DIR="${INSTALL_DIR}/boring-computers"
mkdir -p "${INSTALL_DIR}" /root/infra /opt/boring/src
if [ -d "${REPO_DIR}/.git" ]; then
	log "updating boring-computers source"
	git -C "${REPO_DIR}" fetch --depth=1 origin "${BORING_REVISION}"
else
	log "cloning boring-computers source"
	rm -rf "${REPO_DIR}"
	git clone --filter=blob:none --no-checkout "${REPO_URL}" "${REPO_DIR}"
	git -C "${REPO_DIR}" fetch --depth=1 origin "${BORING_REVISION}"
fi
git -C "${REPO_DIR}" checkout --detach "${BORING_REVISION}"

log "copying boring-computers infra"
cp "${REPO_DIR}"/infra/latitude/*.sh "${REPO_DIR}"/infra/latitude/*.service /root/infra/
cp "${REPO_DIR}"/infra/latitude/Caddyfile /root/infra/ 2>/dev/null || true
rsync -az --delete --exclude '*_test.go' "${REPO_DIR}/boringd/" /opt/boring/src/

log "ensuring Go ${GO_VERSION}"
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
	curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz" -o /tmp/go.tgz
	rm -rf /usr/local/go
	tar -C /usr/local -xzf /tmp/go.tgz
	rm -f /tmp/go.tgz
fi
/usr/local/go/bin/go version

log "bootstrapping Firecracker assets"
bash /root/infra/bootstrap.sh
bash /root/infra/build-rootfs.sh
bash /root/infra/build-template.sh python
if [ "${SKIP_DESKTOP_VALUE}" = "1" ]; then
	log "skipping desktop image"
else
	bash /root/infra/build-desktop-rootfs.sh
fi

log "configuring guest networking"
install -m0755 /root/infra/net-setup.sh /opt/boring/bin/net-setup.sh
bash /opt/boring/bin/net-setup.sh
cp /root/infra/boring-net.service /etc/systemd/system/ 2>/dev/null || true

log "building boringd"
cd /opt/boring/src
CGO_ENABLED=0 /usr/local/go/bin/go build -trimpath -ldflags="-s -w" -o /usr/local/bin/boringd .
cp /root/infra/boringd.service /etc/systemd/system/boringd.service

log "writing boringd environment"
install -d -m0755 /etc/boring
umask 077
cat > /etc/boring/boringd.env <<EOF
BORING_ADDR=${BORING_ADDR_VALUE}
BORING_ALLOW_PERSISTENT=${BORING_ALLOW_PERSISTENT_VALUE}
BORING_JAILER=1
BORING_NET=${BORING_NET_VALUE}
BORING_TOKEN=${BORING_TOKEN_VALUE}
BORING_ANTHROPIC_KEY=${BORING_ANTHROPIC_KEY_VALUE}
BORING_OPENROUTER_KEY=${BORING_OPENROUTER_KEY_VALUE}
BORING_MAX=${BORING_MAX_VALUE}
BORING_MAX_FORKS=${BORING_MAX_FORKS_VALUE}
BORING_MAX_TEMPLATES=${BORING_MAX_TEMPLATES_VALUE}
BORING_S3_KEY=${BORING_S3_KEY_VALUE}
BORING_S3_SECRET=${BORING_S3_SECRET_VALUE}
BORING_S3_BUCKET=boring-volumes
EOF

log "starting boringd"
systemctl daemon-reload
systemctl enable boring-net.service 2>/dev/null || true
systemctl enable boringd
systemctl restart boringd
sleep 2
systemctl is-active boringd
curl -fsS --max-time 8 "${BORING_HEALTH_URL_VALUE}"
`, shellQuote(installDir), shellQuote(PinnedUpstreamRevision), shellQuote(envLine(token)), shellQuote(envLine(opts.AnthropicKey)), shellQuote(envLine(opts.OpenRouterKey)), shellQuote(envLine(opts.S3AccessKeyID)), shellQuote(envLine(opts.S3SecretKey)), shellQuote(boringdAddr), shellQuote(healthURL), maxMachines, maxForks, maxTemplates, allowPersistent, guestNet, skipDesktop)
	return script + managementInstallScript(opts)
}

func boringdListenAddr(rawURL string) string {
	parsed, ok := parseBoringdURL(rawURL)
	if !ok {
		return "127.0.0.1:18080"
	}
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		default:
			port = "8080"
		}
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func boringdHealthURL(rawURL string) string {
	parsed, ok := parseBoringdURL(rawURL)
	if !ok {
		parsed, _ = url.Parse(defaultBoringdURL)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/healthz"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func parseBoringdURL(rawURL string) (*url.URL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		rawURL = defaultBoringdURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, false
	}
	return parsed, true
}

func heredocCommand(path, script string) string {
	return "cat > " + shellQuote(path) + " <<'AURAGO_BORING_SETUP'\n" + script + "\nAURAGO_BORING_SETUP\nif [ \"$(id -u)\" -eq 0 ]; then bash " + shellQuote(path) + "; else sudo -n bash " + shellQuote(path) + "; fi"
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'"
}

func envLine(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return strings.TrimSpace(v)
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
