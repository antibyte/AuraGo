package tools

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDockerCLIMinimalEnvironmentKeepsOnlyWhatComposeNeeds(t *testing.T) {
	masterKey := strings.Repeat("ab", 32)
	in := []string{
		"PATH=/usr/bin", "HOME=/home/aurago", "LANG=de_DE.UTF-8", "LC_ALL=C.UTF-8", "http_proxy=http://proxy:3128", "NO_PROXY=localhost",
		"DOCKER_HOST=unix:///run/user/1000/docker.sock", "DOCKER_PASSWORD=hunter2", "COMPOSE_PROJECT_NAME=lab", "BUILDKIT_PROGRESS=plain",
		"XDG_RUNTIME_DIR=/run/user/1000", `SystemRoot=C:\Windows`,
		"AURAGO_MASTER_KEY=" + masterKey, "AURAGO_BROWSER_AUTOMATION_TOKEN=t", "LLM_API_KEY=k", "OPENAI_API_KEY=k",
		"TAILSCALE_API_KEY=k", "ANSIBLE_API_TOKEN=k", "GOMEMLIMIT=1600MiB", "MY_SECRET=s",
	}
	want := []string{
		"PATH=/usr/bin", "HOME=/home/aurago", "LANG=de_DE.UTF-8", "LC_ALL=C.UTF-8", "http_proxy=http://proxy:3128", "NO_PROXY=localhost",
		"DOCKER_HOST=unix:///run/user/1000/docker.sock", "COMPOSE_PROJECT_NAME=lab", "BUILDKIT_PROGRESS=plain",
		"XDG_RUNTIME_DIR=/run/user/1000", `SystemRoot=C:\Windows`,
	}
	if got := dockerCLIMinimalEnvironment(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("dockerCLIMinimalEnvironment() = %q, want %q", got, want)
	}
}

func TestDockerCLICommandForKeepsTheFullEnvironmentWithoutTheFlag(t *testing.T) {
	t.Setenv("FCC4_PROBE", "kept")
	full := dockerCLICommandFor(context.Background(), DockerConfig{}, "compose", "version")
	if want := dockerCLICommand(context.Background(), "compose", "version"); !reflect.DeepEqual(full.Env, want.Env) || !reflect.DeepEqual(full.Args, want.Args) {
		t.Fatal("without MinimalCLIEnvironment the command must be exactly dockerCLICommand's (grandfathered installs)")
	}
	minimal := dockerCLICommandFor(context.Background(), DockerConfig{MinimalCLIEnvironment: true}, "compose", "version")
	for _, entry := range minimal.Env {
		if strings.HasPrefix(entry, "FCC4_PROBE=") {
			t.Fatal("the minimal environment kept an unlisted variable")
		}
	}
	if len(minimal.Env) == 0 {
		t.Fatal("the minimal environment is empty; PATH must survive")
	}
}

func TestDockerComposeResolvedConfigUsesTheMinimalRunnerOnlyWhenAsked(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	var full, minimal int
	originalFull, originalMinimal := runDockerComposeConfig, runDockerComposeConfigMinimal
	t.Cleanup(func() { runDockerComposeConfig, runDockerComposeConfigMinimal = originalFull, originalMinimal })
	runDockerComposeConfig = func(context.Context, []string) ([]byte, []byte, error) {
		full++
		return []byte(`{"services":{}}`), nil, nil
	}
	runDockerComposeConfigMinimal = func(context.Context, []string) ([]byte, []byte, error) {
		minimal++
		return []byte(`{"services":{}}`), nil, nil
	}
	workspace := t.TempDir()
	for _, cfg := range []DockerConfig{{WorkspaceDir: workspace}, {WorkspaceDir: workspace, MinimalCLIEnvironment: true}} {
		if _, err := DockerComposeResolvedConfigContext(context.Background(), cfg, "compose.yml", DockerComposeConfigOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if full != 1 || minimal != 1 {
		t.Fatalf("full runner %d, minimal runner %d; want one each", full, minimal)
	}
}

func TestDockerComposePassesTheEnvironmentChoiceToTheCLI(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	var seen []bool
	previous := dockerComposeCLIRunner
	dockerComposeCLIRunner = func(cfg DockerConfig, args ...string) string {
		seen = append(seen, cfg.MinimalCLIEnvironment)
		return composeOKResult
	}
	t.Cleanup(func() { dockerComposeCLIRunner = previous })
	workspace := t.TempDir()
	DockerCompose(DockerConfig{WorkspaceDir: workspace}, "compose.yml", "ps")
	DockerCompose(DockerConfig{WorkspaceDir: workspace, MinimalCLIEnvironment: true}, "compose.yml", "ps")
	if !reflect.DeepEqual(seen, []bool{false, true}) {
		t.Fatalf("runner saw MinimalCLIEnvironment %v, want [false true]", seen)
	}
}

// Certificate and credential-helper pointers name files or profiles, not
// secrets, and registry logins or TLS to a private registry need them; the
// secret values themselves stay out.
func TestDockerCLIMinimalEnvironmentKeepsCertificateAndCredentialHelperPointers(t *testing.T) {
	pointers := []string{
		"SSL_CERT_FILE=/etc/ssl/custom.pem", "SSL_CERT_DIR=/etc/ssl/certs", "GNUPGHOME=/home/aurago/.gnupg",
		"PASSWORD_STORE_DIR=/home/aurago/.password-store", "AWS_PROFILE=registry", "AWS_REGION=eu-central-1",
		"AWS_DEFAULT_REGION=eu-central-1", "AWS_CONFIG_FILE=/home/aurago/.aws/config",
		"AWS_SHARED_CREDENTIALS_FILE=/home/aurago/.aws/credentials", "CLOUDSDK_CONFIG=/home/aurago/.config/gcloud",
	}
	secrets := []string{"AWS_ACCESS_KEY_ID=AKIA", "AWS_SECRET_ACCESS_KEY=s", "AWS_SESSION_TOKEN=t", "DOCKER_AUTH_CONFIG={}"}
	got := dockerCLIMinimalEnvironment(append(append([]string(nil), pointers...), secrets...))
	if !reflect.DeepEqual(got, pointers) {
		t.Fatalf("dockerCLIMinimalEnvironment() = %q, want exactly the pointers %q", got, pointers)
	}
}

// The resolver's own limit kills `docker compose config` ("signal: killed");
// the error must say it was the deadline, so the agent's recheck does not
// read the repeat as changed input.
func TestDockerComposeResolvedConfigReportsItsOwnTimeoutAsADeadline(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	previousTimeout, previousRunner := dockerComposeConfigTimeout, runDockerComposeConfig
	t.Cleanup(func() { dockerComposeConfigTimeout, runDockerComposeConfig = previousTimeout, previousRunner })
	dockerComposeConfigTimeout = 20 * time.Millisecond
	runDockerComposeConfig = func(ctx context.Context, _ []string) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, errors.New("signal: killed")
	}
	_, err := DockerComposeResolvedConfigContext(context.Background(), DockerConfig{WorkspaceDir: t.TempDir()}, "compose.yml", DockerComposeConfigOptions{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want one that wraps context.DeadlineExceeded", err)
	}
}
