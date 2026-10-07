package tools

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

func writeAnsibleDockerfile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile.ansible"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildAnsibleImageBuildsOnTheConfiguredEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	dockerHost := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	calls := fakeBuildDockerCLI(t, "", "")
	setInheritedDockerEnv(t, "DOCKER_HOST", "tcp://127.0.0.1:2376")
	setInheritedDockerEnv(t, "DOCKER_CONTEXT", "")
	if err := buildAnsibleImageOnEngine("aurago-ansible:test", writeAnsibleDockerfile(t), dockerHost, &recordingBuildLogger{}); err != nil {
		t.Fatal(err)
	}
	if got, want := fakeBuildDockerCalls(t, calls), []string{"host=" + dockerutil.NormalizeHost(dockerHost) + " context=unset"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("docker calls = %q, want one build on docker.host", got)
	}
}

func TestBuildAnsibleImageRetriesOnceWhenDockerHostRefusesBuilds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	var imageChecks int
	dockerHost := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/"+dockerAPIVersion+"/images/") {
			imageChecks++
		}
		w.WriteHeader(http.StatusOK)
	})
	calls := fakeBuildDockerCLI(t, dockerutil.NormalizeHost(dockerHost), browserAutomationRefusedBuildText)
	setInheritedDockerEnv(t, "DOCKER_HOST", "tcp://127.0.0.1:2376")
	setInheritedDockerEnv(t, "DOCKER_CONTEXT", "")
	logger := &recordingBuildLogger{}
	if err := buildAnsibleImageOnEngine("aurago-ansible:test", writeAnsibleDockerfile(t), dockerHost, logger); err != nil {
		t.Fatal(err)
	}
	want := []string{"host=" + dockerutil.NormalizeHost(dockerHost) + " context=unset", "host=tcp://127.0.0.1:2376 context=unset"}
	if got := fakeBuildDockerCalls(t, calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("docker calls = %q, want %q", got, want)
	}
	if imageChecks != 1 || len(logger.warns) != 1 || !strings.Contains(logger.warns[0], "built through tcp://127.0.0.1:2376 instead") {
		t.Fatalf("image checks %d, warnings %q", imageChecks, logger.warns)
	}
}

func TestBuildAnsibleImageFailsWhenTheFallbackImageIsNotOnDockerHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	dockerHost := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/"+dockerAPIVersion+"/images/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	fakeBuildDockerCLI(t, dockerutil.NormalizeHost(dockerHost), browserAutomationRefusedBuildText)
	setInheritedDockerEnv(t, "DOCKER_HOST", "tcp://127.0.0.1:2376")
	setInheritedDockerEnv(t, "DOCKER_CONTEXT", "")
	err := buildAnsibleImageOnEngine("aurago-ansible:test", writeAnsibleDockerfile(t), dockerHost, &recordingBuildLogger{})
	if err == nil || !strings.Contains(err.Error(), "still has no image") {
		t.Fatalf("error = %v, want the missing-image error", err)
	}
}

func TestBuildSpaceAgentImageBuildsOnTheConfiguredEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker CLI is a POSIX shell script")
	}
	configureDockerSecurityTestPermissions(t, false)
	dockerHost := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	calls := fakeBuildDockerCLI(t, "", "")
	setInheritedDockerEnv(t, "DOCKER_HOST", "tcp://127.0.0.1:2376")
	setInheritedDockerEnv(t, "DOCKER_CONTEXT", "remote")
	source := t.TempDir()
	dockerfile := filepath.Join(source, "Dockerfile.aurago")
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := SpaceAgentSidecarConfig{Image: "aurago-space-agent:test", SourcePath: source, DockerHost: dockerHost}
	if err := buildSpaceAgentImage(cfg, dockerfile, &recordingBuildLogger{}); err != nil {
		t.Fatal(err)
	}
	if got, want := fakeBuildDockerCalls(t, calls), []string{"host=" + dockerutil.NormalizeHost(dockerHost) + " context=unset"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("docker calls = %q, want one build on docker.host without the inherited context", got)
	}
}
