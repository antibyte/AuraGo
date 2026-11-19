package audit

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerHTTPSChoices(t *testing.T) {
	installer := strings.ReplaceAll(readRepoFile(t, "install.sh"), "\r\n", "\n")
	choose := shellFunctionBody(t, installer, "choose_install_network")
	for _, tc := range []struct{ name, answers, want string }{
		{"unattended", "", "127.0.0.1 true selfsigned 0 0"},
		{"LAN HTTPS", "NET_REPLY:y", "0.0.0.0 true selfsigned 0 0"},
		{"explicit HTTP", "NET_REPLY:y HTTPS_REPLY:n", "0.0.0.0 false selfsigned 0 0"},
		{"Lets Encrypt", "NET_REPLY:y ACME_REPLY:y HTTPS_DOMAIN:aurago.example.com HTTPS_EMAIL:admin@example.com", "0.0.0.0 true auto 0 80"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := `set -eu
info() { :; }
warn() { printf 'warning: %s\n' "$*"; }
die() { printf 'error: %s\n' "$*"; exit 1; }
prompt_value() {
  local answer="$4" item
  for item in $ANSWERS; do case "$item" in "$1":*) answer="${item#*:}" ;; esac; done
  printf -v "$1" '%s' "$answer"
}
ANSWERS='` + tc.answers + `'
` + choose + `
choose_install_network
printf '%s %s %s %s %s\n' "$SERVER_HOST" "$HTTPS_ENABLED" "$HTTPS_CERT_MODE" "$HTTPS_INIT_PORT" "$HTTP_REDIRECT_PORT"
`
			out, err := runBashScript(t, bluetoothKitShell(t), script)
			if err != nil || !strings.Contains(out, tc.want) {
				t.Fatalf("%v: %s", err, out)
			}
			if tc.name == "explicit HTTP" && !strings.Contains(out, "microphone") {
				t.Fatal("HTTP opt-out lacks feature warning")
			}
		})
	}
}

func TestDockerFreshHTTPSAndHealthcheck(t *testing.T) {
	entry := readRepoFile(t, "deploy/docker/docker-entrypoint.sh")
	for _, want := range []string{"https_port: 8443", "http_port: 0", "cert_mode: selfsigned", `if [ "$FRESH_CONFIG" = true ]`, "--https-port=8443"} {
		if !strings.Contains(entry, want) {
			t.Fatalf("fresh Docker path missing %s", want)
		}
	}
	if !strings.Contains(readRepoFile(t, "docker-compose.yml"), `${AURAGO_HTTPS_PORT:-8443}:8443`) {
		t.Fatal("Compose HTTPS mapping missing")
	}
	dockerfile := readRepoFile(t, "Dockerfile")
	if !strings.Contains(dockerfile, `"--healthcheck"`) || strings.Contains(dockerfile, "urllib.request.urlopen") {
		t.Fatal("Docker must use the CLI healthcheck")
	}
}

func shellSection(t *testing.T, script, start, end string) string {
	t.Helper()
	_, afterStart, foundStart := strings.Cut(script, start)
	body, _, foundEnd := strings.Cut(afterStart, end)
	if !foundStart || !foundEnd {
		t.Fatalf("could not find shell section %q through %q", start, end)
	}
	return start + body
}

func TestDockerFirstBootStagesConfigAndKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Linux shell utilities")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required")
	}
	entry := readRepoFile(t, "deploy/docker/docker-entrypoint.sh")
	key := shellSection(t, entry, "persist_master_key_fallback() (", "\nread_master_key_file()")
	config := shellSection(t, entry, "publish_initial_config() (", "\n# Recover the exact template")
	config = strings.Replace(config, "/app/aurago ", "aurago ", 1)
	script := "set -eu\n" + key + "\n" + config + `
CONFIG_FILE=config.yaml
TEMPLATE_FILE=template.yaml
ENV_FILE=.env
FRESH_CONFIG=true
normalize_file() { :; }
aurago() {
    [ "$1" = "--config" ] && [ "$2" != "$CONFIG_FILE" ] && [ ! -s "$CONFIG_FILE" ] || return 1
    [[ " $* " == *" --https-port=8443 "* ]] || return 1
    printf 'https_port: 8443\n' > "$2"
    [ "$fail_init" = false ]
}
printf 'https_port: 443\n' > "$TEMPLATE_FILE"
: > "$CONFIG_FILE"
fail_init=true
if publish_initial_config; then exit 10; fi
[ -f "$CONFIG_FILE" ] && [ ! -s "$CONFIG_FILE" ]
fail_init=false
publish_initial_config
grep -qx 'https_port: 8443' "$CONFIG_FILE"
persist_master_key_fallback "fixture-key"
[ "$(stat -c %a "$ENV_FILE")" = 600 ]
mv() { return 1; }
if persist_master_key_fallback "replacement-key"; then exit 11; fi
grep -qx 'AURAGO_MASTER_KEY="fixture-key"' "$ENV_FILE"
`
	cmd := exec.Command(bash, "-c", script)
	cmd.Dir = t.TempDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Docker bootstrap fixture: %v\n%s", err, out)
	}
}

func TestInstallerSummaryFormatsIPv6(t *testing.T) {
	installer := strings.ReplaceAll(readRepoFile(t, "install.sh"), "\r\n", "\n")
	start := strings.Index(installer, `case "$WEB_HOST" in`)
	if start < 0 {
		t.Fatal("missing summary host formatting")
	}
	end := strings.Index(installer[start:], "esac")
	if end < 0 {
		t.Fatal("missing summary case end")
	}
	format := installer[start : start+end+len("esac")]
	for _, host := range []string{"::1", "[::1]", "localhost"} {
		out, err := runBashScript(t, bluetoothKitShell(t), "WEB_HOST='"+host+"'\n"+format+"\nprintf '%s' \"$WEB_HOST\"\n")
		want := "[::1]"
		if host == "localhost" {
			want = host
		}
		if err != nil || strings.TrimSpace(out) != want {
			t.Fatalf("host %q: %q, %v", host, out, err)
		}
	}
}
