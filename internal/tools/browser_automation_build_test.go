package tools

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

type recordingBuildLogger struct {
	warns    []string
	warnArgs [][]any
}

func (l *recordingBuildLogger) Info(string, ...any) {}
func (l *recordingBuildLogger) Warn(msg string, args ...any) {
	l.warns = append(l.warns, msg)
	l.warnArgs = append(l.warnArgs, args)
}
func (l *recordingBuildLogger) Error(string, ...any) {}

func browserAutomationEnvValues(env []string, name string) []string {
	var values []string
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if ok && strings.EqualFold(key, name) {
			values = append(values, value)
		}
	}
	return values
}

func TestBrowserAutomationBuildCommandTargetsConfiguredEngine(t *testing.T) {
	dir := t.TempDir()
	base := []string{
		"PATH=/usr/bin",
		"DOCKER_HOST=tcp://elsewhere:2375",
		"DOCKER_CONTEXT=remote",
		"DOCKER_CONFIG=/home/aurago/.docker",
		"DOCKER_TLS_VERIFY=1",
	}
	inv, err := browserAutomationBuildCommand(base, "aurago-browser-automation:latest", dir, "tcp://docker-proxy:2375")
	if err != nil {
		t.Fatalf("browserAutomationBuildCommand() error = %v", err)
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{"tcp://docker-proxy:2375"}) {
		t.Fatalf("DOCKER_HOST = %#v, want only the configured endpoint", got)
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_CONTEXT"); len(got) != 0 {
		t.Fatalf("DOCKER_CONTEXT = %#v, want it dropped", got)
	}
	wantConfig := filepath.Join(dir, "data", ".docker")
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_CONFIG"); !reflect.DeepEqual(got, []string{wantConfig}) {
		t.Fatalf("DOCKER_CONFIG = %#v, want %q (location unchanged)", got, wantConfig)
	}
	for _, kept := range []string{"PATH", "DOCKER_TLS_VERIFY"} {
		if got := browserAutomationEnvValues(inv.Env, kept); len(got) != 1 {
			t.Fatalf("%s = %#v, want the inherited value kept", kept, got)
		}
	}
	wantArgs := []string{"build", "-f", filepath.Join(dir, "Dockerfile.browser_automation"), "-t", "aurago-browser-automation:latest", dir}
	if !reflect.DeepEqual(inv.Args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", inv.Args, wantArgs)
	}
	if inv.DockerHost != "tcp://docker-proxy:2375" {
		t.Fatalf("DockerHost = %q, want tcp://docker-proxy:2375", inv.DockerHost)
	}
}

func TestBrowserAutomationBuildCommandDefaultsToLocalEngine(t *testing.T) {
	inv, err := browserAutomationBuildCommand(nil, "aurago-browser:test", t.TempDir(), "")
	if err != nil {
		t.Fatalf("browserAutomationBuildCommand() error = %v", err)
	}
	if inv.DockerHost != dockerutil.DefaultHost() {
		t.Fatalf("DockerHost = %q, want %q", inv.DockerHost, dockerutil.DefaultHost())
	}
	if got := browserAutomationEnvValues(inv.Env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{dockerutil.DefaultHost()}) {
		t.Fatalf("DOCKER_HOST = %#v, want the platform default", got)
	}
}

func TestBrowserAutomationBuildCommandUsesAbsolutePaths(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, dir := range []string{"", ".", "-checkout", filepath.Join("sub", "dir")} {
		inv, err := browserAutomationBuildCommand(nil, "aurago-browser:test", dir, "")
		if err != nil {
			t.Fatalf("browserAutomationBuildCommand(%q) error = %v", dir, err)
		}
		if !filepath.IsAbs(inv.ContextDir) {
			t.Fatalf("context for %q = %q, want an absolute path", dir, inv.ContextDir)
		}
		last := inv.Args[len(inv.Args)-1]
		if last != inv.ContextDir || strings.HasPrefix(last, "-") {
			t.Fatalf("context argument for %q = %q", dir, last)
		}
		if inv.Args[2] != filepath.Join(inv.ContextDir, "Dockerfile.browser_automation") {
			t.Fatalf("-f argument for %q = %q", dir, inv.Args[2])
		}
	}
}

func TestBrowserAutomationBuildContextRequiresDockerfile(t *testing.T) {
	dir := t.TempDir()
	inv, err := browserAutomationBuildCommand(nil, "aurago-browser:test", dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := browserAutomationCheckBuildContext(inv, "aurago-browser:test"); err == nil || !strings.Contains(err.Error(), "source checkout") {
		t.Fatalf("missing Dockerfile error = %v, want a source checkout hint", err)
	}
	if err := os.Mkdir(inv.Dockerfile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := browserAutomationCheckBuildContext(inv, "aurago-browser:test"); err == nil {
		t.Fatal("a directory named like the Dockerfile must not pass")
	}
	if err := os.Remove(inv.Dockerfile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inv.Dockerfile, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := browserAutomationCheckBuildContext(inv, "aurago-browser:test"); err != nil {
		t.Fatalf("existing Dockerfile error = %v", err)
	}
}

func TestBrowserAutomationRemoteBuildEndpoint(t *testing.T) {
	for _, tc := range []struct {
		host     string
		inDocker bool
		want     bool
	}{
		{"", false, false},
		{"unix:///var/run/docker.sock", false, false},
		{"npipe:////./pipe/docker_engine", false, false},
		{"tcp://127.0.0.1:2375", false, false},
		{"tcp://localhost:2375", false, false},
		{"tcp://[::1]:2375", false, false},
		{"tcp://192.168.1.10:2375", false, true},
		{"192.168.1.10:2375", false, true},
		{"ssh://user@host", false, false},
		{"fd://", false, false},
		{"tcp://docker-proxy:2375", true, false},
		{"tcp://192.168.1.10:2375", true, false},
	} {
		if got := browserAutomationRemoteBuildEndpoint(tc.host, tc.inDocker); got != tc.want {
			t.Fatalf("browserAutomationRemoteBuildEndpoint(%q, %v) = %v, want %v", tc.host, tc.inDocker, got, tc.want)
		}
	}
}

func TestResolveBrowserAutomationSidecarConfigCarriesRuntimeDockerFlag(t *testing.T) {
	cfg := browserAutomationTestConfig(t, "http://127.0.0.1:7331")
	cfg.Runtime.IsDocker = true
	sidecarCfg, err := ResolveBrowserAutomationSidecarConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveBrowserAutomationSidecarConfig() error = %v", err)
	}
	if !sidecarCfg.RuntimeIsDocker {
		t.Fatal("RuntimeIsDocker = false, want the runtime probe result")
	}
}

func TestBuildBrowserAutomationImageRunsDockerAgainstConfiguredEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	binDir := t.TempDir()
	envFile := filepath.Join(t.TempDir(), "docker.env")
	script := "#!/bin/sh\nenv > " + shellQuoteDockerSecurityTest(envFile) + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_HOST", "tcp://elsewhere:2375")
	t.Setenv("DOCKER_CONTEXT", "remote")

	contextDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile.browser_automation"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := &recordingBuildLogger{}
	if err := buildBrowserAutomationImage("aurago-browser:test", contextDir, "tcp://127.0.0.1:2375", false, logger); err != nil {
		t.Fatalf("buildBrowserAutomationImage() error = %v", err)
	}
	raw, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("fake docker did not run: %v", err)
	}
	env := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if got := browserAutomationEnvValues(env, "DOCKER_HOST"); !reflect.DeepEqual(got, []string{"tcp://127.0.0.1:2375"}) {
		t.Fatalf("docker CLI DOCKER_HOST = %#v, want the configured loopback engine", got)
	}
	if got := browserAutomationEnvValues(env, "DOCKER_CONTEXT"); len(got) != 0 {
		t.Fatalf("docker CLI DOCKER_CONTEXT = %#v, want none", got)
	}
	if len(logger.warns) != 0 {
		t.Fatalf("loopback engine warned: %#v", logger.warns)
	}

	logger = &recordingBuildLogger{}
	if err := buildBrowserAutomationImage("aurago-browser:test", contextDir, "tcp://192.168.1.10:2375", false, logger); err != nil {
		t.Fatalf("remote engine build must still run: %v", err)
	}
	if len(logger.warns) != 1 {
		t.Fatalf("remote engine warnings = %#v, want exactly one", logger.warns)
	}

	if err := os.Remove(envFile); err != nil {
		t.Fatal(err)
	}
	err = buildBrowserAutomationImage("aurago-browser:test", t.TempDir(), "", false, &recordingBuildLogger{})
	if err == nil || !strings.Contains(err.Error(), "source checkout") {
		t.Fatalf("missing Dockerfile error = %v, want a source checkout hint", err)
	}
	if _, statErr := os.Stat(envFile); !os.IsNotExist(statErr) {
		t.Fatal("docker must not run when the Dockerfile is missing")
	}
}
