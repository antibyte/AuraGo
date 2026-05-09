package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreMemoryWritesRemainAgentToolOnly(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	allowed := map[string]bool{
		filepath.Clean("internal/memory/short_term_profile.go"): true, // storage primitive definitions
		filepath.Clean("internal/tools/core_memory.go"):         true, // agent-facing manage_memory/core_memory tool
	}

	var offenders []string
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			if !strings.Contains(text, "AddCoreMemoryFact(") && !strings.Contains(text, "UpdateCoreMemoryFact(") {
				return nil
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.Clean(rel)
			if !allowed[rel] {
				offenders = append(offenders, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("core memory writes must stay agent-tool-only; unexpected writers: %s", strings.Join(offenders, ", "))
	}
}
