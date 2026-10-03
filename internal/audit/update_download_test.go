package audit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func updateScriptSpan(t *testing.T, start, end string) string {
	t.Helper()
	script := readRepoFile(t, "update.sh")
	a := strings.Index(script, start)
	if a < 0 {
		t.Fatalf("missing update stage %q", start)
	}
	b := strings.Index(script[a:], end)
	if b < 0 {
		t.Fatalf("missing update stage end %q", end)
	}
	return script[a : a+b]
}

func runUpdateScript(t *testing.T, script string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, releaseBash(t), "-s")
	cmd.Stdin = strings.NewReader("set -euo pipefail\n" + script)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestUpdaterRetriesTransientDownloads(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		failures int32
		requests int32
		success  bool
	}{
		{"recovers from 503", 503, 2, 3, true},
		{"exhausts three retries", 503, 99, 4, false},
		{"does not retry missing asset", 404, 99, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) <= tc.failures {
					w.WriteHeader(tc.status)
					_, _ = fmt.Fprint(w, "must-not-leak-partial-response")
					return
				}
				_, _ = fmt.Fprint(w, `{"tag_name":"v-fixture"}`)
			}))
			defer server.Close()
			helpers := updateScriptSpan(t, "fetch_url_to_file() {", "\nread_master_key_from_env() {")
			script := helpers + fmt.Sprintf(`
if ! command -v curl >/dev/null; then echo NO_CURL; exit 0; fi
if body="$(fetch_url_stdout '%s')"; then
    printf 'SUCCESS:%%s\n' "$body"
else
    echo FAILED
fi
`, server.URL)
			output, err := runUpdateScript(t, script)
			if strings.TrimSpace(output) == "NO_CURL" {
				t.Skip("curl is unavailable")
			}
			if err != nil || calls.Load() != tc.requests || strings.Contains(output, "must-not-leak") {
				t.Fatalf("requests=%d want=%d err=%v output=%s", calls.Load(), tc.requests, err, output)
			}
			if strings.Contains(output, `SUCCESS:{"tag_name":"v-fixture"}`) != tc.success {
				t.Fatalf("unexpected download outcome: %s", output)
			}
		})
	}
}

func TestUpdaterOptionalMissingSignatureIsQuiet(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	helpers := updateScriptSpan(t, "fetch_url_to_file() {", "\nsha256_file() {")
	output, err := runUpdateScript(t, helpers+fmt.Sprintf(`
out="$(mktemp)"
trap 'rm -f -- "$out"' EXIT
if fetch_optional_url_to_file '%s' "$out"; then exit 1; else echo MISSING; fi
`, server.URL))
	if err != nil || strings.TrimSpace(output) != "MISSING" || calls.Load() != 1 {
		t.Fatalf("optional signature: requests=%d err=%v output=%s", calls.Load(), err, output)
	}
}

func TestUpdaterReleasePreflightBeforeShutdown(t *testing.T) {
	preflight := updateScriptSpan(t, "# Release preflight for source checkouts without Go.", "# Capture the pre-update readiness contract")
	for _, tc := range []struct {
		name, goFound, binaryOnly, tag  string
		checksumOK, wantsRelease, fails bool
	}{
		{"source build ignores release outage", "true", "false", "", false, false, false},
		{"binary mode keeps its pinned release", "false", "true", "v-old", false, false, false},
		{"release preflight succeeds", "false", "false", "v-fixture", true, true, false},
		{"metadata failure stops before shutdown", "false", "false", "", false, true, true},
		{"checksum failure stops before shutdown", "false", "false", "v-fixture", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := fmt.Sprintf(`
GO_FOUND=%s
BINARY_ONLY=%s
GITHUB_REPO=example/project
RELEASE_TAG=v-old
latest_release_tag() { echo REQUEST_RELEASE >&2; printf '%%s' '%s'; }
fetch_release_checksums() { echo REQUEST_CHECKSUMS; %t; }
info() { :; }
warn() { :; }
die() { echo "FAILED_PREFLIGHT: $*"; exit 77; }
`, tc.goFound, tc.binaryOnly, tc.tag, tc.checksumOK) + preflight + "\necho SHUTDOWN\n"
			output, err := runUpdateScript(t, script)
			if (err != nil) != tc.fails || strings.Contains(output, "SHUTDOWN") == tc.fails || strings.Contains(output, "REQUEST_RELEASE") != tc.wantsRelease {
				t.Fatalf("preflight outcome: err=%v output=%s", err, output)
			}
		})
	}
	script := readRepoFile(t, "update.sh")
	stop := strings.Index(script, `section "Stopping running instances"`)
	if stop < 0 || strings.Index(script, "GO_FOUND=false") > stop {
		t.Fatal("build path must be selected before shutdown")
	}
	for _, forbidden := range []string{"latest_release_tag", "fetch_release_checksums", `RELEASE_TAG="latest"`} {
		if strings.Contains(script[stop:], forbidden) {
			t.Fatalf("release pair must be resolved only before shutdown: %s", forbidden)
		}
	}
}

func TestUpdaterRequiredResourceFailureUsesRollback(t *testing.T) {
	branch := updateScriptSpan(t, `    if ! download_release_asset "resources.dat"`, "\n    TMPEXT=")
	output, err := runUpdateScript(t, `
TMPRES=unused-fixture-path
download_release_asset() { return 1; }
abort_update() { echo ROLLBACK; exit 79; }
die() { echo UNRECOVERED_EXIT; exit 78; }
`+branch+"\necho INSTALL\n")
	if err == nil || strings.TrimSpace(output) != "ROLLBACK" {
		t.Fatalf("required artifact failure bypassed rollback: err=%v output=%s", err, output)
	}
}

func TestUpdaterReleaseChecksumStillRejectsCorruptArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			_, _ = fmt.Fprintf(w, "%s  fixture.bin\n", strings.Repeat("0", 64))
		case "/fixture.bin":
			_, _ = fmt.Fprint(w, "corrupt artifact")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	helpers := updateScriptSpan(t, "fetch_url_to_file() {", "\nread_master_key_from_env() {")
	output, err := runUpdateScript(t, helpers+fmt.Sprintf(`
RELEASE_BASE='%s'
GITHUB_REPO=example/project
AURAGO_STRICT_RELEASE_VERIFY=0
RELEASE_CHECKSUMS_FILE=''
out="$(mktemp)"
trap 'rm -f -- "$out" "${RELEASE_CHECKSUMS_FILE:-}"' EXIT
warn() { :; }
ok() { :; }
die() { echo "UNEXPECTED: $*"; exit 1; }
tui_run() { shift; "$@"; }
fetch_release_checksums
if download_release_asset fixture.bin "$out"; then echo ACCEPTED; else echo REJECTED; fi
`, server.URL))
	if err != nil || strings.TrimSpace(output) != "REJECTED" {
		t.Fatalf("corrupt release artifact accepted: err=%v output=%s", err, output)
	}
}
