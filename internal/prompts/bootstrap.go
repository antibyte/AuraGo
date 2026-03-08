package prompts

import (
	promptsembed "aurago/prompts"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// EnsurePromptsDir ensures that the on-disk prompts directory exists and
// contains the user-editable files.
//
// System prompts (rules.md, tools_*.md, maintenance.md, …) are embedded
// directly in the binary and loaded at runtime from the embed.FS — they are
// intentionally NOT written to disk so users cannot accidentally or
// deliberately tamper with them.
//
// Only two things are placed on disk:
//   - identity.md   — the agent's base identity, fully customisable by the user.
//   - personalities/ — empty directory where users can add custom personality
//     profiles.  Built-in profiles (friend.md, professional.md, …) live in the
//     binary embed and are read from there at runtime.
func EnsurePromptsDir(dir string, logger *slog.Logger) {
	// Always ensure the personalities sub-directory exists for user additions.
	if err := os.MkdirAll(filepath.Join(dir, "personalities"), 0750); err != nil {
		logger.Error("Cannot create personalities directory", "error", err)
		return
	}

	// Extract identity.md only if it does not yet exist (preserve user edits).
	identityPath := filepath.Join(dir, "identity.md")
	if _, err := os.Stat(identityPath); err == nil {
		return // already present
	}

	data, err := io.ReadAll(func() io.Reader {
		f, e := promptsembed.FS.Open("identity.md")
		if e != nil {
			return strings.NewReader("")
		}
		return f
	}())
	if err != nil || len(data) == 0 {
		logger.Error("Could not read embedded identity.md", "error", err)
		return
	}

	if err := os.WriteFile(identityPath, data, 0640); err != nil {
		logger.Error("Failed to write identity.md", "path", identityPath, "error", err)
		return
	}
	logger.Info("identity.md extracted for customisation", "path", identityPath)
}
