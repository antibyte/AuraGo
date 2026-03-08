package prompts

import (
	"log/slog"
	"os"
	"path/filepath"
)

// EnsurePromptsDir ensures that the on-disk prompts directory and its
// sub-directories exist so the server can write user-created files into them.
//
// System prompts (identity.md, rules.md, tools_*.md, …) and built-in
// personality profiles are embedded directly in the binary and loaded at
// runtime from the embed.FS.  They are intentionally NOT extracted to disk.
//
// Only user-created files land on disk:
//   - prompts/*.md      — additional prompts created via the Config UI
//   - prompts/personalities/*.md — user-defined personality profiles
func EnsurePromptsDir(dir string, logger *slog.Logger) {
	for _, sub := range []string{dir, filepath.Join(dir, "personalities")} {
		if err := os.MkdirAll(sub, 0750); err != nil {
			logger.Error("Cannot create prompts directory", "path", sub, "error", err)
			return
		}
	}
	logger.Debug("Prompts directory ready", "path", dir)
}
