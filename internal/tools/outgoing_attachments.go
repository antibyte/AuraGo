package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"aurago/internal/config"
)

// maxEchoedAttachmentPathRunes bounds how much of a caller-supplied attachment path an
// error message repeats (the cut is marked with an ellipsis).
const maxEchoedAttachmentPathRunes = 200

// ResolveOutgoingAttachmentPath resolves a file that a tool sends to an external recipient
// (Telegram document, email attachment). Only existing regular files inside the agent
// workspace or the document output folder qualify, so configuration, vault and databases
// can never leave AuraGo. Relative paths are tried against the working directory (where
// document_creator writes "data/documents/…") and against both roots.
//
// The returned path is only a snapshot: a path component can be swapped for a symlink
// before the caller opens it. Callers that read the file must use OpenOutgoingAttachment.
func ResolveOutgoingAttachmentPath(filePath string, cfg *config.Config) (string, error) {
	resolved, _, err := resolveOutgoingAttachment(filePath, cfg)
	return resolved, err
}

// OpenOutgoingAttachment opens a file that a tool sends to an external recipient and is
// the entry point for Telegram documents and email attachments. The file qualifies under
// the rules of ResolveOutgoingAttachmentPath; on top of that it is opened through an
// os.Root bound to the workspace or documents folder that contains it, so a symlink that is
// swapped in after the check and leads out of that folder is refused instead of followed.
//
// It returns the file, opened read-only, and its resolved absolute path. The caller owns
// the file and must Close it. The path is meant for naming the attachment (filepath.Base);
// reading goes through the returned file, never through a second open of the path. A
// failure returns no file, and its message does not reveal whether a file outside the
// folders exists. The message repeats at most maxEchoedAttachmentPathRunes runes of filePath.
func OpenOutgoingAttachment(filePath string, cfg *config.Config) (*os.File, string, error) {
	resolved, root, err := resolveOutgoingAttachment(filePath, cfg)
	if err != nil {
		return nil, "", err
	}
	file, err := openWithinOutgoingRoot(root, resolved)
	if err != nil {
		return nil, "", fmt.Errorf("attachment %q cannot be opened: %w", truncateStr(filePath, maxEchoedAttachmentPathRunes), err)
	}
	return file, resolved, nil
}

// outgoingAttachmentRoots returns the canonical folders an outgoing attachment may live in.
func outgoingAttachmentRoots(cfg *config.Config) []string {
	docsDir := strings.TrimSpace(cfg.Tools.DocumentCreator.OutputDir)
	if docsDir == "" {
		docsDir = "data/documents"
	}
	return uniqueCanonicalPaths(canonicalExistingRoot(cfg.Directories.WorkspaceDir), canonicalExistingRoot(docsDir))
}

// resolveOutgoingAttachment returns the symlink-free path of the attachment and the
// canonical root that contains it.
func resolveOutgoingAttachment(filePath string, cfg *config.Config) (string, string, error) {
	if cfg == nil {
		return "", "", fmt.Errorf("config is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", "", fmt.Errorf("attachment path is empty")
	}
	if strings.ContainsRune(filePath, 0) || !utf8.ValidString(filePath) {
		return "", "", fmt.Errorf("attachment path contains characters that are not allowed")
	}
	roots := outgoingAttachmentRoots(cfg)
	if len(roots) == 0 {
		return "", "", fmt.Errorf("no attachment folders are configured")
	}
	candidates := []string{filePath}
	if !filepath.IsAbs(filePath) {
		for _, root := range roots {
			candidates = append(candidates, filepath.Join(root, filePath))
		}
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			continue
		}
		resolved = filepath.Clean(resolved)
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		for _, root := range roots {
			if pathWithinCanonicalRoot(root, resolved) {
				return resolved, root, nil
			}
		}
	}
	return "", "", fmt.Errorf("attachment %q is missing or outside the workspace and the documents folder", truncateStr(filePath, maxEchoedAttachmentPathRunes))
}

// openWithinOutgoingRoot opens resolved, which must lie inside root, through an os.Root
// so that no path component can lead out of root, and requires a regular file.
func openWithinOutgoingRoot(root, resolved string) (*os.File, error) {
	if !pathWithinCanonicalRoot(root, resolved) {
		return nil, fmt.Errorf("the file is outside its folder")
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return nil, fmt.Errorf("the file is outside its folder")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open the attachment folder: %w", withoutPathError(err))
	}
	defer dir.Close()
	file, err := dir.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("open the file inside its folder: %w", withoutPathError(err))
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("the file is not a regular file")
	}
	return file, nil
}

// withoutPathError drops the operation and path that os errors carry, so that a message
// never repeats a file name taken from the disk or from the caller.
func withoutPathError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}
