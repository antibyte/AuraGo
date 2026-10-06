package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerutil"
)

// browserAutomationRefusedBuildText is what docker prints when a socket proxy
// with BUILD=0 (HAProxy) denies the build API.
const browserAutomationRefusedBuildText = "Error response from daemon: <html><body><h1>403 Forbidden</h1>\nRequest forbidden by administrative rules.\n</body></html>"

func TestBrowserAutomationBuildCommandCarriesPreK19Fallback(t *testing.T) {
	dir := t.TempDir()
	base := []string{
		"PATH=/usr/bin",
		"DOCKER_HOST=tcp://elsewhere:2375",
		"DOCKER_TLS_VERIFY=1",
		"DOCKER_CONFIG=/home/aurago/.docker",
	}
	inv, err := browserAutomationBuildCommand(base, "aurago-browser:test", dir, "tcp://localhost:2375")
	if err != nil {
		t.Fatalf("browserAutomationBuildCommand() error = %v", err)
	}
	fb := inv.Fallback
	if fb == nil {
		t.Fatal("Fallback = nil, want the pre-K19 invocation for a different inherited engine")
	}
	if fb.DockerHost != "tcp://elsewhere:2375" {
		t.Fatalf("Fallback.DockerHost = %q, want the inherited endpoint", fb.DockerHost)
	}
	if got := browserAutomationEnvValues(fb.Env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{"tcp://elsewhere:2375"}) {
		t.Fatalf("Fallback DOCKER_HOST = %#v, want the inherited value untouched", got)
	}
	wantConfig := filepath.Join(dir, "data", ".docker")
	if got := browserAutomationEnvValues(fb.Env, "DOCKER_CONFIG"); !reflect.DeepEqual(got, []string{wantConfig}) {
		t.Fatalf("Fallback DOCKER_CONFIG = %#v, want only the override %q", got, wantConfig)
	}
	for _, kept := range []string{"PATH", "DOCKER_TLS_VERIFY"} {
		if got := browserAutomationEnvValues(fb.Env, kept); len(got) != 1 {
			t.Fatalf("Fallback %s = %#v, want the inherited value kept", kept, got)
		}
	}
	if !reflect.DeepEqual(fb.Args, inv.Args) {
		t.Fatalf("Fallback args = %#v, want the same build arguments %#v", fb.Args, inv.Args)
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{"tcp://localhost:2375"}) {
		t.Fatalf("primary DOCKER_HOST = %#v, want the configured endpoint", got)
	}
}

func TestBrowserAutomationBuildCommandFallbackTargets(t *testing.T) {
	for _, tc := range []struct {
		name         string
		base         []string
		configured   string
		wantFallback bool
		wantTarget   string
		wantEnvHost  []string
		wantContext  []string
	}{
		{
			name:         "no inherited engine reaches the default socket",
			configured:   "tcp://localhost:2375",
			wantFallback: true,
			wantTarget:   dockerutil.DefaultHost(),
		},
		{
			name:         "inherited engine equals the configured one",
			base:         []string{"DOCKER_HOST=tcp://localhost:2375"},
			configured:   "tcp://localhost:2375",
			wantFallback: false,
		},
		{
			name:         "default configured and nothing inherited",
			configured:   "",
			wantFallback: false,
		},
		{
			name:         "default context is the default engine",
			base:         []string{"DOCKER_CONTEXT=default"},
			configured:   "",
			wantFallback: false,
		},
		{
			name:         "the default context does not hide DOCKER_HOST",
			base:         []string{"DOCKER_CONTEXT=default", "DOCKER_HOST=tcp://elsewhere:2375"},
			configured:   "tcp://localhost:2375",
			wantFallback: true,
			wantTarget:   "tcp://elsewhere:2375",
			wantEnvHost:  []string{"tcp://elsewhere:2375"},
			wantContext:  []string{"default"},
		},
		{
			name:         "a context cannot be resolved with the DOCKER_CONFIG override",
			base:         []string{"DOCKER_CONTEXT=remote", "DOCKER_HOST=tcp://elsewhere:2375"},
			configured:   "tcp://localhost:2375",
			wantFallback: false,
		},
		{
			name:         "a context alone gets no fallback either",
			base:         []string{"docker_context=remote"},
			configured:   "tcp://localhost:2375",
			wantFallback: false,
		},
		{
			name:         "bare host and port equal the tcp URL",
			base:         []string{"DOCKER_HOST=192.168.1.10:2375"},
			configured:   "tcp://192.168.1.10:2375",
			wantFallback: false,
		},
		{
			name:         "tcp URL equals the bare host and port",
			base:         []string{"DOCKER_HOST=tcp://192.168.1.10:2375"},
			configured:   " 192.168.1.10:2375 ",
			wantFallback: false,
		},
		{
			name:         "surrounding space in the inherited value is ignored",
			base:         []string{"DOCKER_HOST= tcp://localhost:2375 "},
			configured:   "tcp://localhost:2375",
			wantFallback: false,
		},
		{
			name:         "an empty DOCKER_HOST is the default engine",
			base:         []string{"DOCKER_HOST="},
			configured:   "",
			wantFallback: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := browserAutomationBuildCommand(tc.base, "aurago-browser:test", t.TempDir(), tc.configured)
			if err != nil {
				t.Fatal(err)
			}
			if (inv.Fallback != nil) != tc.wantFallback {
				t.Fatalf("Fallback = %#v, want present=%v", inv.Fallback, tc.wantFallback)
			}
			if inv.Fallback == nil {
				return
			}
			if inv.Fallback.DockerHost != tc.wantTarget {
				t.Fatalf("Fallback.DockerHost = %q, want %q", inv.Fallback.DockerHost, tc.wantTarget)
			}
			if got := browserAutomationEnvValues(inv.Fallback.Env, "DOCKER_HOST"); len(got) != len(tc.wantEnvHost) || (len(got) > 0 && !reflect.DeepEqual(got, tc.wantEnvHost)) {
				t.Fatalf("Fallback DOCKER_HOST = %#v, want %#v (the old environment sets nothing of its own)", got, tc.wantEnvHost)
			}
			if got := browserAutomationEnvValues(inv.Fallback.Env, "DOCKER_CONTEXT"); len(got) != len(tc.wantContext) || (len(got) > 0 && !reflect.DeepEqual(got, tc.wantContext)) {
				t.Fatalf("Fallback DOCKER_CONTEXT = %#v, want %#v", got, tc.wantContext)
			}
		})
	}
}

func TestBrowserAutomationBuildCommandHandlesMixedCaseDockerEnv(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"Docker_Host=tcp://mixed:2375",
		"docker_config=/home/aurago/.docker",
		"Docker_TLS_Verify=1",
	}
	dir := t.TempDir()
	inv, err := browserAutomationBuildCommand(base, "aurago-browser:test", dir, "tcp://docker-proxy:2375")
	if err != nil {
		t.Fatal(err)
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{"tcp://docker-proxy:2375"}) {
		t.Fatalf("DOCKER_HOST = %#v, want the mixed-case inherited variable replaced", got)
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_CONFIG"); !reflect.DeepEqual(got, []string{filepath.Join(dir, "data", ".docker")}) {
		t.Fatalf("DOCKER_CONFIG = %#v, want only the override", got)
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_TLS_VERIFY"); len(got) != 1 {
		t.Fatalf("DOCKER_TLS_VERIFY = %#v, want the mixed-case TLS variable kept", got)
	}
	if inv.Fallback == nil || inv.Fallback.DockerHost != "tcp://mixed:2375" {
		t.Fatalf("Fallback = %#v, want the mixed-case inherited endpoint", inv.Fallback)
	}
	if got := browserAutomationEnvValues(inv.Fallback.Env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{"tcp://mixed:2375"}) {
		t.Fatalf("Fallback DOCKER_HOST = %#v, want the inherited variable kept as is", got)
	}
}

func TestBrowserAutomationBuildFallbackDecision(t *testing.T) {
	retry := &browserAutomationBuildInvocation{DockerHost: "unix:///var/run/docker.sock"}
	inv := browserAutomationBuildInvocation{DockerHost: "tcp://127.0.0.1:2375", Fallback: retry}
	failed := errors.New("exit status 1")

	for _, output := range []string{
		browserAutomationRefusedBuildText,
		"Request FORBIDDEN BY ADMINISTRATIVE RULES.",
		"error during connect: Post \"http://127.0.0.1:2375/v1.45/build\": 403 Forbidden",
	} {
		if got := browserAutomationBuildFallback(inv, failed, output); got != retry {
			t.Fatalf("output %q: fallback = %#v, want the retry invocation", output, got)
		}
	}

	buildKitDigests := "#1 [internal] load build definition from Dockerfile.browser_automation\n" +
		"#1 transferring dockerfile: 1.2kB done\n" +
		"#5 0.403 Temporary failure resolving 'deb.debian.org'\n" +
		"#6 sha256:4034f61a0b3a4c0e1c8f0a7c9a0c1e1f0a2a3b4c5d6e7f8091a2b3c4d5e6f708 10.2MB / 10.2MB 0.4s\n"
	npmStepFailure := "#7 [4/6] RUN npm ci\n" +
		"#7 2.114 npm ERR! 403 Forbidden - GET https://registry.npmjs.org/playwright\n" +
		"#7 ERROR: process \"/bin/sh -c npm ci\" did not complete successfully: exit code: 1\n"
	aptStepFailure := "Step 4/6 : RUN apt-get update\n" +
		"E: Failed to fetch http://deb.debian.org/debian/dists/bookworm/InRelease  403  Forbidden\n" +
		"W: Some index files failed to download\n" +
		"The command '/bin/sh -c apt-get update' returned a non-zero code: 100\n" +
		"403 Forbidden\n"
	for _, output := range []string{
		"",
		"no space left on device",
		"failed to solve: pull access denied for aurago-base",
		"dial tcp 127.0.0.1:2375: connect: connection refused",
		"HTTP 403",
		"error: forbidden",
		buildKitDigests,
		npmStepFailure,
		aptStepFailure,
		browserAutomationRefusedBuildText + "\n" + npmStepFailure,
	} {
		if got := browserAutomationBuildFallback(inv, failed, output); got != nil {
			t.Fatalf("output %q: fallback = %#v, want no retry", output, got)
		}
	}
	if got := browserAutomationBuildFallback(inv, nil, browserAutomationRefusedBuildText); got != nil {
		t.Fatalf("a successful build must not retry, got %#v", got)
	}
	if got := browserAutomationBuildFallback(browserAutomationBuildInvocation{}, failed, browserAutomationRefusedBuildText); got != nil {
		t.Fatalf("no alternate engine means no retry, got %#v", got)
	}
}

func TestBrowserAutomationCheckBuildContextNamesTheCause(t *testing.T) {
	dir := t.TempDir()
	inv, err := browserAutomationBuildCommand(nil, "aurago-browser:test", dir, "")
	if err != nil {
		t.Fatal(err)
	}

	err = browserAutomationCheckBuildContext(inv, "aurago-browser:test")
	if err == nil || !strings.Contains(err.Error(), "was not found") || !strings.Contains(err.Error(), "source checkout") || !strings.Contains(err.Error(), inv.Dockerfile) {
		t.Fatalf("missing Dockerfile error = %v, want the cause, the path and the source checkout hint", err)
	}

	if err := os.Mkdir(inv.Dockerfile, 0o755); err != nil {
		t.Fatal(err)
	}
	err = browserAutomationCheckBuildContext(inv, "aurago-browser:test")
	if err == nil || !strings.Contains(err.Error(), "not a regular file") || !strings.Contains(err.Error(), "source checkout") {
		t.Fatalf("directory error = %v, want the not-a-regular-file cause and the source checkout hint", err)
	}

	inv.Dockerfile = filepath.Join(dir, "bad\x00name")
	err = browserAutomationCheckBuildContext(inv, "aurago-browser:test")
	if err == nil || !strings.Contains(err.Error(), "cannot be read") || !strings.Contains(err.Error(), "source checkout") {
		t.Fatalf("stat failure error = %v, want the stat cause and the source checkout hint", err)
	}
}

func TestResolveBrowserAutomationSidecarConfigDetectsDockerEnvironment(t *testing.T) {
	cfg := browserAutomationTestConfig(t, "http://127.0.0.1:7331")
	cfg.Runtime.IsDocker = false
	sidecarCfg, err := ResolveBrowserAutomationSidecarConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveBrowserAutomationSidecarConfig() error = %v", err)
	}
	if want := browserAutomationRunsInDocker(); sidecarCfg.RuntimeIsDocker != want {
		t.Fatalf("RuntimeIsDocker = %v, want %v (the same /.dockerenv probe the sidecar URL and container name use)", sidecarCfg.RuntimeIsDocker, want)
	}
}

// fakeBuildDockerCLI puts a fake docker CLI first on PATH. Every call appends
// "host=<DOCKER_HOST> context=<DOCKER_CONTEXT>" to the returned file ("unset"
// when the variable is absent). A call whose DOCKER_HOST equals failHost
// ("*" matches every call) prints failText and exits 1.
func fakeBuildDockerCLI(t *testing.T, failHost, failText string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "docker.calls")
	pattern := "'__no_host__'"
	switch failHost {
	case "":
	case "*":
		pattern = "*"
	default:
		pattern = shellQuoteDockerSecurityTest(failHost)
	}
	script := "#!/bin/sh\n" +
		"printf 'host=%s context=%s\\n' \"${DOCKER_HOST-unset}\" \"${DOCKER_CONTEXT-unset}\" >> " + shellQuoteDockerSecurityTest(logFile) + "\n" +
		"case \"$DOCKER_HOST\" in\n" +
		"  " + pattern + ") echo " + shellQuoteDockerSecurityTest(failText) + " >&2; exit 1;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logFile
}

func fakeBuildDockerCalls(t *testing.T, logFile string) []string {
	t.Helper()
	raw, err := os.ReadFile(logFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read fake docker calls: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// setInheritedDockerEnv sets the process variable for the test; an empty value
// removes it. The original value is restored when the test ends.
func setInheritedDockerEnv(t *testing.T, name, value string) {
	t.Helper()
	t.Setenv(name, "")
	if value == "" {
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Setenv(name, value)
}

func browserAutomationBuildContextDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile.browser_automation"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildBrowserAutomationImageRetriesOnceWhenDockerHostRefusesBuilds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)

	const stepFailureText = "npm ERR! 403 Forbidden - GET https://registry.npmjs.org/playwright\n" +
		"process \"/bin/sh -c npm ci\" did not complete successfully: exit code: 1"

	// @HOST@ stands for the fake Docker API host that serves as docker.host.
	for _, tc := range []struct {
		name              string
		inheritedHost     string
		inheritedCtx      string
		defaultEngine     bool // docker.host is empty, i.e. the platform default
		failHost          string
		failText          string
		imageStatus       int // GET /images/<image>/json on docker.host after the build; 0 means 200
		buildTimeout      time.Duration
		wantCalls         []string
		wantErrs          []string
		wantRefusalCopies int
		wantWarnMsgs      []string // one substring per expected warning, in order
		wantBuiltOn       string   // the engine named by the last warning's arguments
		wantImageChecks   int
	}{
		{
			name:          "refusal retries once on the inherited engine",
			inheritedHost: "tcp://127.0.0.1:2376",
			failHost:      "@HOST@",
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=@HOST@ context=unset", "host=tcp://127.0.0.1:2376 context=unset"},
			wantWarnMsgs: []string{
				"docker.host refused the build with a 403-style response; built through tcp://127.0.0.1:2376 instead",
			},
			wantBuiltOn:     "tcp://127.0.0.1:2376",
			wantImageChecks: 1,
		},
		{
			name:      "refusal retries on the CLI default when nothing is inherited",
			failHost:  "@HOST@",
			failText:  browserAutomationRefusedBuildText,
			wantCalls: []string{"host=@HOST@ context=unset", "host=unset context=unset"},
			wantWarnMsgs: []string{
				"built through " + dockerutil.DefaultHost() + " instead",
			},
			wantBuiltOn:     dockerutil.DefaultHost(),
			wantImageChecks: 1,
		},
		{
			name:          "a remote retry endpoint warns about plain TCP",
			inheritedHost: "tcp://192.168.1.10:2375",
			failHost:      "@HOST@",
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=@HOST@ context=unset", "host=tcp://192.168.1.10:2375 context=unset"},
			wantWarnMsgs: []string{
				"Retrying on a remote Docker engine over plain TCP",
				"built through tcp://192.168.1.10:2375 instead",
			},
			wantBuiltOn:     "tcp://192.168.1.10:2375",
			wantImageChecks: 1,
		},
		{
			name:            "the image must be visible on docker.host after the fallback build",
			inheritedHost:   "tcp://127.0.0.1:2376",
			failHost:        "@HOST@",
			failText:        browserAutomationRefusedBuildText,
			imageStatus:     http.StatusNotFound,
			wantCalls:       []string{"host=@HOST@ context=unset", "host=tcp://127.0.0.1:2376 context=unset"},
			wantErrs:        []string{"the fallback built on tcp://127.0.0.1:2376", "still has no image aurago-browser:test"},
			wantImageChecks: 1,
		},
		{
			name:         "an inherited context disables the retry",
			inheritedCtx: "remote",
			failHost:     "@HOST@",
			failText:     browserAutomationRefusedBuildText,
			wantCalls:    []string{"host=@HOST@ context=unset"},
			wantErrs:     []string{"403 Forbidden"},
		},
		{
			name:          "other failures do not retry",
			inheritedHost: "tcp://127.0.0.1:2376",
			failHost:      "@HOST@",
			failText:      "no space left on device",
			wantCalls:     []string{"host=@HOST@ context=unset"},
			wantErrs:      []string{"no space left on device"},
		},
		{
			name:          "a failed step with a 403 does not retry",
			inheritedHost: "tcp://127.0.0.1:2376",
			failHost:      "@HOST@",
			failText:      stepFailureText,
			wantCalls:     []string{"host=@HOST@ context=unset"},
			wantErrs:      []string{"did not complete successfully"},
		},
		{
			name:          "success does not retry",
			inheritedHost: "tcp://127.0.0.1:2376",
			wantCalls:     []string{"host=@HOST@ context=unset"},
		},
		{
			name:          "refusal on the same engine does not retry",
			inheritedHost: "@HOST@",
			failHost:      "@HOST@",
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=@HOST@ context=unset"},
			wantErrs:      []string{"403 Forbidden"},
		},
		{
			name:          "refusal on the default engine does not retry",
			defaultEngine: true,
			failHost:      dockerutil.DefaultHost(),
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=" + dockerutil.DefaultHost() + " context=unset"},
			wantErrs:      []string{"403 Forbidden"},
		},
		{
			name:              "a refused retry fails after exactly two calls and keeps both outputs",
			inheritedHost:     "tcp://127.0.0.1:2376",
			failHost:          "*",
			failText:          browserAutomationRefusedBuildText,
			wantCalls:         []string{"host=@HOST@ context=unset", "host=tcp://127.0.0.1:2376 context=unset"},
			wantErrs:          []string{"the retry on tcp://127.0.0.1:2376 failed too", "first attempt on docker.host:"},
			wantRefusalCopies: 2,
		},
		{
			name:          "too little build time left skips the retry",
			inheritedHost: "tcp://127.0.0.1:2376",
			failHost:      "@HOST@",
			failText:      browserAutomationRefusedBuildText,
			buildTimeout:  time.Minute,
			wantCalls:     []string{"host=@HOST@ context=unset"},
			wantErrs:      []string{"too little to retry on tcp://127.0.0.1:2376", "403 Forbidden"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var imageChecks int
			dockerHost := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/"+dockerAPIVersion+"/images/") {
					imageChecks++
					if tc.imageStatus != 0 {
						w.WriteHeader(tc.imageStatus)
						return
					}
					_, _ = w.Write([]byte(`{}`))
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			})
			resolve := strings.NewReplacer("@HOST@", dockerHost).Replace
			configured := dockerHost
			if tc.defaultEngine {
				configured = ""
			}
			if tc.buildTimeout != 0 {
				previous := browserAutomationBuildTimeout
				browserAutomationBuildTimeout = tc.buildTimeout
				t.Cleanup(func() { browserAutomationBuildTimeout = previous })
			}
			logFile := fakeBuildDockerCLI(t, resolve(tc.failHost), tc.failText)
			setInheritedDockerEnv(t, "DOCKER_HOST", resolve(tc.inheritedHost))
			setInheritedDockerEnv(t, "DOCKER_CONTEXT", tc.inheritedCtx)

			logger := &recordingBuildLogger{}
			err := buildBrowserAutomationImage("aurago-browser:test", browserAutomationBuildContextDir(t), configured, false, logger)

			var wantCalls []string
			for _, call := range tc.wantCalls {
				wantCalls = append(wantCalls, resolve(call))
			}
			if got := fakeBuildDockerCalls(t, logFile); !reflect.DeepEqual(got, wantCalls) {
				t.Fatalf("docker calls = %#v, want %#v", got, wantCalls)
			}
			if len(tc.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("buildBrowserAutomationImage() error = %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("buildBrowserAutomationImage() succeeded, want an error containing %q", tc.wantErrs)
				}
				for _, want := range tc.wantErrs {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("buildBrowserAutomationImage() error = %v, want it to contain %q", err, want)
					}
				}
				if tc.wantRefusalCopies > 0 {
					if got := strings.Count(err.Error(), "Request forbidden by administrative rules"); got != tc.wantRefusalCopies {
						t.Fatalf("error holds %d copies of the refusal output, want %d: %v", got, tc.wantRefusalCopies, err)
					}
				}
			}
			if imageChecks != tc.wantImageChecks {
				t.Fatalf("image checks on docker.host = %d, want %d", imageChecks, tc.wantImageChecks)
			}
			if len(logger.warns) != len(tc.wantWarnMsgs) {
				t.Fatalf("warnings = %#v, want %d", logger.warns, len(tc.wantWarnMsgs))
			}
			for i, want := range tc.wantWarnMsgs {
				if !strings.Contains(logger.warns[i], resolve(want)) {
					t.Fatalf("warning %d = %q, want it to contain %q", i, logger.warns[i], resolve(want))
				}
			}
			if tc.wantBuiltOn != "" {
				args := fmt.Sprint(logger.warnArgs[len(logger.warnArgs)-1]...)
				for _, name := range []string{dockerHost, tc.wantBuiltOn} {
					if !strings.Contains(args, name) {
						t.Fatalf("warning arguments %q do not name %q", args, name)
					}
				}
			}
		})
	}
}

func TestBrowserAutomationBuildTimeLeft(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if left := browserAutomationBuildTimeLeft(ctx); left <= 0 || left > 30*time.Second {
		t.Fatalf("time left = %v, want within (0, 30s]", left)
	}
	if left := browserAutomationBuildTimeLeft(context.Background()); left < 24*time.Hour {
		t.Fatalf("time left without a deadline = %v, want effectively unlimited", left)
	}
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	if left := browserAutomationBuildTimeLeft(expired); left >= browserAutomationRetryMinRemaining {
		t.Fatalf("time left on an expired context = %v, want below the retry minimum", left)
	}
}

func TestBuildBrowserAutomationImageWarnsWhenDockerConfigDirIsUnavailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	logFile := fakeBuildDockerCLI(t, "", "")
	contextDir := browserAutomationBuildContextDir(t)
	if err := os.WriteFile(filepath.Join(contextDir, "data"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := &recordingBuildLogger{}
	if err := buildBrowserAutomationImage("aurago-browser:test", contextDir, "", false, logger); err != nil {
		t.Fatalf("buildBrowserAutomationImage() error = %v", err)
	}
	if len(fakeBuildDockerCalls(t, logFile)) != 1 {
		t.Fatal("the build must still run when the config directory cannot be created")
	}
	if len(logger.warns) != 1 || !strings.Contains(logger.warns[0], "config directory") {
		t.Fatalf("warnings = %#v, want one about the docker CLI config directory", logger.warns)
	}
}

func TestEnsureBrowserAutomationSidecarRunningBuildsOnConfiguredEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	logFile := fakeBuildDockerCLI(t, "", "")
	setInheritedDockerEnv(t, "DOCKER_HOST", "tcp://elsewhere:2375")
	setInheritedDockerEnv(t, "DOCKER_CONTEXT", "remote")

	var requests []string
	dockerHost := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		requests = append(requests, r.Method+" "+path)
		switch {
		case r.Method == http.MethodGet && path == "/networks/browser-egress":
			_, _ = w.Write([]byte(`{"Internal":true}`))
		case r.Method == http.MethodGet && (strings.HasPrefix(path, "/containers/") || strings.HasPrefix(path, "/images/")):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	sidecarCfg := BrowserAutomationSidecarConfig{
		URL:           "http://127.0.0.1:7331",
		Image:         "aurago-browser:test",
		ContainerName: "aurago-browser-test",
		AuthToken:     "test-token",
		AutoBuild:     true,
		DockerfileDir: browserAutomationBuildContextDir(t),
		CloakProxy:    "http://proxy:3128",
		EgressNetwork: "browser-egress",
		WorkspaceDir:  t.TempDir(),
		DownloadDir:   t.TempDir(),
	}
	EnsureBrowserAutomationSidecarRunning(dockerHost, sidecarCfg, &recordingBuildLogger{})

	want := []string{"host=" + dockerHost + " context=unset"}
	if got := fakeBuildDockerCalls(t, logFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("docker CLI calls = %#v, want %#v (the build must target the host the image check and container create use); Docker API requests: %#v", got, want, requests)
	}
}
