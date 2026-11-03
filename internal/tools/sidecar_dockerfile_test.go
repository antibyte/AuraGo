package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSidecarDockerfilePath(t *testing.T) {
	for _, name := range []string{"Dockerfile.ansible", browserAutomationDockerfileName, browserAutomationEgressDockerfile} {
		for _, tc := range []struct {
			name    string
			legacy  bool
			bundled bool
		}{
			{name: "missing"},
			{name: "custom", legacy: true},
			{name: "repository", bundled: true},
			{name: "custom takes precedence", legacy: true, bundled: true},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				contextDir := t.TempDir()
				legacy := filepath.Join(contextDir, name)
				bundled := filepath.Join(contextDir, "deploy", "docker", name)
				for path, create := range map[string]bool{legacy: tc.legacy, bundled: tc.bundled} {
					if create {
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte("FROM scratch\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				want := legacy
				if tc.bundled && !tc.legacy {
					want = bundled
				}
				if got := sidecarDockerfilePath(contextDir, name); got != want {
					t.Fatalf("Dockerfile = %q; want %q", got, want)
				}
			})
		}
	}
}

func TestBrowserAutomationBuildFindsRepositoryDockerfiles(t *testing.T) {
	contextDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(contextDir, "deploy", "docker"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{browserAutomationDockerfileName, browserAutomationEgressDockerfile} {
		if err := os.WriteFile(filepath.Join(contextDir, "deploy", "docker", name), []byte("FROM scratch\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := browserAutomationBuildCommand([]string{"DOCKER_HOST=tcp://legacy:2375"}, "fixture", contextDir, "tcp://configured:2375")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{browserAutomationDockerfileName, browserAutomationEgressDockerfile} {
		if err := browserAutomationSelectDockerfile(&inv, name); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(contextDir, "deploy", "docker", name)
		if inv.Dockerfile != want || inv.Args[2] != want || inv.Fallback == nil || inv.Fallback.Dockerfile != want || inv.Fallback.Args[2] != want {
			t.Fatalf("build and fallback must use %s: %+v", want, inv)
		}
		if err := browserAutomationCheckBuildContext(inv, "fixture"); err != nil {
			t.Fatal(err)
		}
	}
}
