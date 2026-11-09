package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeStudioNodeSampleWorkspace(t *testing.T) {
	for _, mode := range []string{"host", "container"} {
		for _, name := range []string{"empty", "legacy samples", "custom project", "existing samples"} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				original := map[string]string{}
				if name == "legacy samples" || name == "existing samples" {
					original["hello.go"] = "// user-edited Go sample\n"
					original["hello.py"] = "# user-edited Python sample\n"
				}
				if name == "existing samples" {
					original["hello.c"] = "// user's C program\n"
					original["hello.js"] = "// user's JavaScript sample\n"
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
				switch name {
				case "empty":
					want = defaultCodeStudioWorkspaceFiles()
				case "legacy samples":
					for _, sample := range codeStudioExtraSamples {
						want[sample] = defaultCodeStudioWorkspaceFiles()[sample]
					}
				case "existing samples":
					want["hello.ts"] = defaultCodeStudioWorkspaceFiles()["hello.ts"]
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

func TestCodeStudioNodeSamplesAreValid(t *testing.T) {
	files := defaultCodeStudioWorkspaceFiles()
	for _, name := range []string{"hello.js", "hello.ts"} {
		content, ok := files[name]
		if !ok || strings.TrimSpace(content) == "" {
			t.Fatalf("%s sample is missing from the default workspace", name)
		}
		if !strings.Contains(content, "Hello from Code Studio") {
			t.Fatalf("%s sample does not greet the user: %q", name, content)
		}
	}
	readme := files["README.md"]
	for _, want := range []string{"node hello.js", "tsx hello.ts"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("workspace README %q missing %q", readme, want)
		}
	}
}
