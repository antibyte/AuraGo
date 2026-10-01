package fileutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameContextPreservesFilesOnCancelAndFailure(t *testing.T) {
	for _, mode := range []string{"cancel", "directory"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "replacement")
			destination := filepath.Join(root, "original")
			original := destination
			if mode == "directory" {
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatal(err)
				}
				original = filepath.Join(destination, "retained")
			}
			for path, content := range map[string]string{source: "new", original: "old"} {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			err := RenameContext(ctx, source, destination)
			if err == nil || (mode == "cancel" && !errors.Is(err, context.Canceled)) {
				t.Fatalf("replacement error: %v", err)
			}
			for path, want := range map[string]string{source: "new", original: "old"} {
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Fatalf("file changed after failed replacement: %q %v", got, err)
				}
			}
		})
	}
}
