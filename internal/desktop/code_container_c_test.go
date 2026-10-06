package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeStudioCSampleWorkspace(t *testing.T) {
	for _, mode := range []string{"host", "container"} {
		for _, name := range []string{"empty", "legacy samples", "custom project", "existing C"} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				original := map[string]string{}
				if name == "legacy samples" || name == "existing C" {
					original["hello.go"] = "// user-edited Go sample\n"
					original["hello.py"] = "# user-edited Python sample\n"
				}
				if name == "existing C" {
					original["hello.c"] = "// user's C program\n"
				}
				if name == "custom project" {
					original["main.c"] = "int main(void) { return 0; }\n"
				}
				for file, content := range original {
					if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				for range 2 {
					if mode == "host" {
						if err := seedCodeStudioWorkspace(dir); err != nil {
							t.Fatal(err)
						}
					} else {
						shell, err := exec.LookPath("sh")
						if err != nil {
							t.Skip("POSIX shell required for container seed script")
						}
						quotedDir := "'" + strings.ReplaceAll(filepath.ToSlash(dir), "'", "'\"'\"'") + "'"
						script := strings.NewReplacer("mkdir -p /workspace", "mkdir -p "+quotedDir,
							"find /workspace ", "find "+quotedDir+" ", "/workspace/", quotedDir+"/").Replace(buildCodeStudioContainerSeedScript())
						if out, err := exec.Command(shell, "-c", script).CombinedOutput(); err != nil {
							t.Fatalf("container seed: %v\n%s", err, out)
						}
					}
				}
				want := original
				if name == "empty" {
					want = defaultCodeStudioWorkspaceFiles()
				} else if name == "legacy samples" {
					want["hello.c"] = defaultCodeStudioWorkspaceFiles()["hello.c"]
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != len(want) {
					t.Fatalf("workspace entries = %v, want %d: %v", entries, len(want), err)
				}
				for file, expected := range want {
					content, err := os.ReadFile(filepath.Join(dir, file))
					if err != nil || string(content) != expected {
						t.Errorf("%s content changed or missing: %q (%v)", file, content, err)
					}
				}
			})
		}
	}
}

func TestCodeStudioRuntimeRequiresCCompiler(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell required for runtime probe")
	}
	bin := t.TempDir()
	for _, tool := range []string{"node", "python3", "go", "gcc"} {
		if tool == "gcc" {
			cmd := exec.Command(shell, "-c", buildCodeStudioRuntimeProbeScript())
			cmd.Env = append(os.Environ(), "PATH="+bin)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "gcc not found") {
				t.Fatalf("missing compiler was not rejected: %v, %s", err, out)
			}
		}
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(shell, "-c", buildCodeStudioRuntimeProbeScript())
	cmd.Env = append(os.Environ(), "PATH="+bin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("complete runtime was rejected: %v, %s", err, out)
	}
}
