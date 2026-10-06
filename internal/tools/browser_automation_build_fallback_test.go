package tools

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

const browserAutomationRefusedBuildText = "request returned 403 Forbidden for API route and version http://127.0.0.1:2375/v1.45/build"

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
			name:         "context selects another engine",
			base:         []string{"DOCKER_CONTEXT=remote", "DOCKER_HOST=tcp://elsewhere:2375"},
			configured:   "tcp://localhost:2375",
			wantFallback: true,
			wantTarget:   "docker context remote",
			wantEnvHost:  []string{"tcp://elsewhere:2375"},
			wantContext:  []string{"remote"},
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
		"error: forbidden",
		"Request FORBIDDEN by administrative rules.",
		"HTTP 403",
	} {
		if got := browserAutomationBuildFallback(inv, failed, output); got != retry {
			t.Fatalf("output %q: fallback = %#v, want the retry invocation", output, got)
		}
	}
	for _, output := range []string{
		"",
		"no space left on device",
		"failed to solve: pull access denied for aurago-base",
		"dial tcp 127.0.0.1:2375: connect: connection refused",
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
	const configured = "tcp://127.0.0.1:2375"

	for _, tc := range []struct {
		name          string
		inheritedHost string
		inheritedCtx  string
		configured    string
		failHost      string
		failText      string
		wantCalls     []string
		wantErr       string
		wantWarns     int
		wantWarnNames []string
	}{
		{
			name:          "refusal retries once on the inherited engine",
			inheritedHost: "tcp://elsewhere:2375",
			configured:    configured,
			failHost:      configured,
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=" + configured + " context=unset", "host=tcp://elsewhere:2375 context=unset"},
			wantWarns:     1,
			wantWarnNames: []string{configured, "tcp://elsewhere:2375"},
		},
		{
			name:          "refusal retries on the CLI default when nothing is inherited",
			inheritedHost: "",
			configured:    configured,
			failHost:      configured,
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=" + configured + " context=unset", "host=unset context=unset"},
			wantWarns:     1,
			wantWarnNames: []string{configured, dockerutil.DefaultHost()},
		},
		{
			name:          "the retry keeps an inherited context",
			inheritedHost: "",
			inheritedCtx:  "remote",
			configured:    configured,
			failHost:      configured,
			failText:      "Forbidden",
			wantCalls:     []string{"host=" + configured + " context=unset", "host=unset context=remote"},
			wantWarns:     1,
			wantWarnNames: []string{configured, "docker context remote"},
		},
		{
			name:          "other failures do not retry",
			inheritedHost: "tcp://elsewhere:2375",
			configured:    configured,
			failHost:      configured,
			failText:      "no space left on device",
			wantCalls:     []string{"host=" + configured + " context=unset"},
			wantErr:       "no space left on device",
		},
		{
			name:          "success does not retry",
			inheritedHost: "tcp://elsewhere:2375",
			configured:    configured,
			wantCalls:     []string{"host=" + configured + " context=unset"},
		},
		{
			name:          "refusal on the same engine does not retry",
			inheritedHost: configured,
			configured:    configured,
			failHost:      configured,
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=" + configured + " context=unset"},
			wantErr:       "403 Forbidden",
		},
		{
			name:       "refusal on the default engine does not retry",
			configured: "",
			failHost:   dockerutil.DefaultHost(),
			failText:   browserAutomationRefusedBuildText,
			wantCalls:  []string{"host=" + dockerutil.DefaultHost() + " context=unset"},
			wantErr:    "403 Forbidden",
		},
		{
			name:          "a refused retry fails after exactly two calls",
			inheritedHost: "tcp://elsewhere:2375",
			configured:    configured,
			failHost:      "*",
			failText:      browserAutomationRefusedBuildText,
			wantCalls:     []string{"host=" + configured + " context=unset", "host=tcp://elsewhere:2375 context=unset"},
			wantErr:       "retry on tcp://elsewhere:2375 failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logFile := fakeBuildDockerCLI(t, tc.failHost, tc.failText)
			setInheritedDockerEnv(t, "DOCKER_HOST", tc.inheritedHost)
			setInheritedDockerEnv(t, "DOCKER_CONTEXT", tc.inheritedCtx)

			logger := &recordingBuildLogger{}
			err := buildBrowserAutomationImage("aurago-browser:test", browserAutomationBuildContextDir(t), tc.configured, false, logger)

			if got := fakeBuildDockerCalls(t, logFile); !reflect.DeepEqual(got, tc.wantCalls) {
				t.Fatalf("docker calls = %#v, want %#v", got, tc.wantCalls)
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("buildBrowserAutomationImage() error = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("buildBrowserAutomationImage() error = %v, want it to contain %q", err, tc.wantErr)
			}
			if len(logger.warns) != tc.wantWarns {
				t.Fatalf("warnings = %#v, want %d", logger.warns, tc.wantWarns)
			}
			if tc.wantWarns == 1 {
				if !strings.Contains(logger.warns[0], "docker.host refused the build") {
					t.Fatalf("warning = %q, want it to explain the refused build", logger.warns[0])
				}
				args := fmt.Sprint(logger.warnArgs[0]...)
				for _, name := range tc.wantWarnNames {
					if !strings.Contains(args, name) {
						t.Fatalf("warning arguments %q do not name %q", args, name)
					}
				}
			}
		})
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
