package prompts

import (
	promptsembed "aurago/prompts"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// EnsurePromptsDir extracts the embedded default prompt files to dir if the
// directory does not yet exist (or contains no .md files at its root).
// Existing files on disk are never overwritten so user customisations survive.
func EnsurePromptsDir(dir string, logger *slog.Logger) {
	// Check if identity.md exists – the primary marker of a valid prompts dir.
	if _, err := os.Stat(filepath.Join(dir, "identity.md")); err == nil {
		return // already populated
	}

	logger.Info("Extracting embedded prompt defaults", "dest", dir)

	if err := os.MkdirAll(dir, 0750); err != nil {
		logger.Error("Cannot create prompts directory", "error", err)
		return
	}

	err := fs.WalkDir(promptsembed.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the embed.go file itself
		if path == "embed.go" {
			return nil
		}

		dest := filepath.Join(dir, filepath.FromSlash(path))

		if d.IsDir() {
			return os.MkdirAll(dest, 0750)
		}

		// Never overwrite existing files
		if _, err := os.Stat(dest); err == nil {
			return nil
		}

		src, err := promptsembed.FS.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
			return err
		}

		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, src)
		return err
	})

	if err != nil {
		logger.Error("Failed to extract prompt defaults", "error", err)
	} else {
		logger.Info("Prompt defaults extracted", "dest", dir)
	}
}
