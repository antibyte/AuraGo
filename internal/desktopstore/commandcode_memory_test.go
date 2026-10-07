package desktopstore

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommandCodePreviewMemoryPreservesInstructions(t *testing.T) {
	dockerfile, files, err := commandCodeBuildContext()
	if err != nil {
		t.Fatal(err)
	}
	const guidePath = "/usr/local/share/aurago/commandcode-preview.md"
	if len(files["commandcode-preview.md"]) == 0 || !strings.Contains(string(dockerfile), "COPY commandcode-preview.md "+guidePath) {
		t.Fatal("published and embedded images must include the preview instructions")
	}
	bash, err := exec.LookPath("bash")
	if runtime.GOOS == "windows" {
		bash = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
		_, err = os.Stat(bash)
	}
	if err != nil {
		t.Skip("Bash is required to exercise the container's memory initialization")
	}
	// Execute the actual initialization without launching the preview gateway.
	init, _, ok := strings.Cut(string(files["commandcode-entrypoint.sh"]), "target_file=")
	if !ok {
		t.Fatal("entrypoint initialization boundary missing")
	}
	for _, existing := range []string{"", "# My instructions\nUse pnpm, never npm."} {
		t.Run(map[bool]string{true: "existing", false: "fresh"}[existing != ""], func(t *testing.T) {
			taskHome := t.TempDir()
			memoryDir := filepath.Join(taskHome, ".commandcode")
			memoryPath := filepath.Join(memoryDir, "AGENTS.md")
			if existing != "" {
				if err := os.MkdirAll(memoryDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(memoryPath, []byte(existing), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				cmd := exec.Command(bash, "--noprofile", "--norc", "-c", init)
				cmd.Env = append(os.Environ(), "HOME="+filepath.ToSlash(taskHome))
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("initialize memory: %v: %s", err, out)
				}
			}
			got, err := os.ReadFile(memoryPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != existing+"\n\n@"+guidePath+"\n" {
				t.Fatalf("memory changed user content or duplicated the import: %q", got)
			}
		})
	}
}
