package audit

import (
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
	entry := readRepoFile(t, "docker-entrypoint.sh")
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
