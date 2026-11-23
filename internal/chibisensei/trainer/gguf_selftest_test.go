package trainer_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/chibisensei"
)

// assertDryRunGGUF checks the GGUF files of a trainer --dry-run in dir with the production reader: the tuned model
// must exist; the base model is checked when present.
func assertDryRunGGUF(t *testing.T, dir, wantArch string) {
	t.Helper()
	for _, name := range []string{chibisensei.OutModelGGUF, chibisensei.OutBaseGGUF} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			if name == chibisensei.OutModelGGUF {
				t.Fatalf("dry run wrote no %s", name)
			}
			continue
		}
		info, err := chibisensei.ReadGGUFInfo(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if info.Version != 3 || info.Tensors == 0 || info.Architecture != wantArch {
			t.Fatalf("%s: %+v, want version 3, tensors > 0, architecture %q", name, info, wantArch)
		}
	}
}
