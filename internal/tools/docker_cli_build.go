package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/sandbox"
)

// dockerCLIBuildRequest is one `docker build` AuraGo runs through the CLI for a
// managed sidecar whose container it then creates on docker.host.
type dockerCLIBuildRequest struct {
	Image      string   // the tag being built; checked on docker.host after a fallback build
	Args       []string // docker CLI arguments, starting with "build"
	Dir        string   // working directory of the CLI ("" = AuraGo's)
	ConfigDir  string   // DOCKER_CONFIG, writable under systemd ProtectHome
	DockerHost string   // docker.host
	Timeout    time.Duration
	LogPrefix  string // "[Ansible]", "[SpaceAgent]"
}

type dockerCLIBuildLogger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

// runDockerCLIBuildOnConfiguredEngine builds on docker.host, the engine that
// creates the sidecar, like the browser-automation build (K19): an inherited
// DOCKER_HOST or DOCKER_CONTEXT no longer redirects it. When docker.host
// refuses builds with a 403-style answer (a socket proxy with BUILD=0) and the
// CLI's inherited engine is another one, the build is retried once there, as
// it ran before, and the image must then be visible on docker.host (only a 404
// fails; any other answer warns and lets the container create decide).
func runDockerCLIBuildOnConfiguredEngine(req dockerCLIBuildRequest, logger dockerCLIBuildLogger) ([]byte, error) {
	baseEnv := sandbox.FilterEnv(os.Environ())
	host := dockerutil.NormalizeHost(req.DockerHost)
	env := make([]string, 0, len(baseEnv)+2)
	legacy := make([]string, 0, len(baseEnv)+1)
	for _, kv := range baseEnv {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "DOCKER_HOST", "DOCKER_CONTEXT":
			legacy = append(legacy, kv)
			continue
		case "DOCKER_CONFIG":
			continue
		}
		env = append(env, kv)
		legacy = append(legacy, kv)
	}
	env = append(env, "DOCKER_HOST="+host, "DOCKER_CONFIG="+req.ConfigDir)
	legacy = append(legacy, "DOCKER_CONFIG="+req.ConfigDir)

	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()
	run := func(env []string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "docker", req.Args...)
		cmd.Dir = req.Dir
		cmd.Env = env
		return cmd.CombinedOutput()
	}
	out, err := run(env)
	if err == nil {
		return out, nil
	}
	first := strings.TrimSpace(string(out))
	inherited, ok := browserAutomationInheritedEngine(baseEnv)
	if !ok || browserAutomationEngineURL(inherited) == browserAutomationEngineURL(host) || !browserAutomationBuildRefused(first) {
		return out, fmt.Errorf("docker build: %w\n%s", err, first)
	}
	if left := browserAutomationBuildTimeLeft(ctx); left < browserAutomationRetryMinRemaining {
		return out, fmt.Errorf("docker build: docker.host %s refused the build, but only %s of the build time is left, too little to retry on %s: %w\n%s", host, left.Round(time.Second), inherited, err, first)
	}
	retryOut, retryErr := run(legacy)
	if retryErr != nil {
		return retryOut, fmt.Errorf("docker build: docker.host %s refused the build and the retry on %s failed too: %w\n%s\nfirst attempt on docker.host:\n%s", host, inherited, retryErr, strings.TrimSpace(string(retryOut)), first)
	}
	_, code, reqErr := dockerRequest(DockerConfig{Host: req.DockerHost}, http.MethodGet, "/images/"+req.Image+"/json", "")
	switch {
	case reqErr == nil && code == http.StatusNotFound:
		return retryOut, fmt.Errorf("the fallback built on %s, but docker.host %s still has no image %s: it is a different engine, so build the image there or point docker.host at the engine that docker uses by default", inherited, host, req.Image)
	case reqErr != nil || code != http.StatusOK:
		logger.Warn(req.LogPrefix+" Fallback build done, but could not verify the image on docker.host, continuing", "docker_host", host, "image", req.Image, "status", code, "error", reqErr)
	}
	logger.Warn(fmt.Sprintf("%s docker.host refused the build with a 403-style response; built through %s instead", req.LogPrefix, inherited), "docker_host", host, "built_on", inherited)
	return retryOut, nil
}
