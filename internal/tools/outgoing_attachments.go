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

// Settings an operator narrows when a folder is refused as attachment root.
const (
	outgoingWorkspaceSetting = "directories.workspace_dir"
	outgoingDocumentsSetting = "tools.document_creator.output_dir"
)

// ResolveOutgoingAttachmentPath resolves a file that a tool sends to an external recipient
// (Telegram document, email attachment). Only existing regular files inside the agent
// workspace or the document output folder qualify, so configuration, vault and databases
// can never leave AuraGo. A folder that holds AuraGo's data directory or its configuration
// file is not accepted as root, and files that are protected system state (config, vault,
// databases, .env, the master key, also under another name through a hard link) are refused.
// A copy is not caught: a copy of the config or of a database under another name passes, and
// only vault.bin, *.env and aurago_master.key are refused by name. Making a copy needs read
// access to the original, which the file tools and Landlock deny.
// Relative paths are tried against the working directory and against both roots; the
// documents folder is resolved against the configuration directory when the config is
// loaded, and document_creator reports absolute paths.
//
// The returned path is only a snapshot, not a security boundary on its own: a path component
// can be swapped for a link before the caller opens it, and a case-sensitive directory can
// make the name differ from the content. Callers that read the file must use
// OpenOutgoingAttachment.
//
// On Windows a path through any junction is refused (filepath.EvalSymlinks fails on one), so
// a workspace configured as a junction makes every attachment fail. That is safe, not a bug.
func ResolveOutgoingAttachmentPath(filePath string, cfg *config.Config) (string, error) {
	resolved, _, err := resolveOutgoingAttachment(filePath, cfg)
	return resolved, err
}

// OpenOutgoingAttachment opens a file that a tool sends to an external recipient and is
// the entry point for Telegram documents and email attachments. The file qualifies under
// the rules of ResolveOutgoingAttachmentPath; on top of that it is opened through an
// os.Root bound to the workspace or documents folder that contains it, so a symlink that is
// swapped in after the check and leads out of that folder is refused instead of followed.
// The folder itself must still be a plain directory, the open handle must be the file the
// path names, a regular file, and not a hard link to protected system state.
//
// It returns the file, opened read-only, and its resolved absolute path. The caller owns
// the file and must Close it. The path is meant for naming the attachment (filepath.Base);
// reading goes through the returned file, never through a second open of the path. A
// failure returns no file, and its message does not reveal whether a file outside the
// folders exists. The message repeats at most maxEchoedAttachmentPathRunes runes of filePath.
//
// As for ResolveOutgoingAttachmentPath, a path through a junction is refused on Windows.
func OpenOutgoingAttachment(filePath string, cfg *config.Config) (*os.File, string, error) {
	resolved, root, err := resolveOutgoingAttachment(filePath, cfg)
	if err != nil {
		return nil, "", err
	}
	file, err := openWithinOutgoingRoot(cfg, root, resolved)
	if err != nil {
		return nil, "", fmt.Errorf("attachment %q cannot be opened: %w", truncateStr(filePath, maxEchoedAttachmentPathRunes), err)
	}
	return file, resolved, nil
}

// outgoingRoots holds the folders an outgoing attachment may live in.
type outgoingRoots struct {
	// canonical are the symlink-resolved folders; containment is decided against them.
	canonical []string
	// lexical are the canonical and the as-configured absolute forms of the same folders.
	// They only decide whether a path may be looked at before any filesystem access.
	lexical []string
	// dropped names the settings of configured folders that were left out because they hold
	// AuraGo's data directory or configuration file.
	dropped []string
}

// outgoingAttachmentRoots returns the folders an outgoing attachment may live in: the
// workspace and the documents folder, minus any that contains the data directory or the
// directory of the config file. Such a folder would send the vault, the databases and the
// config, whatever per-file checks follow.
func outgoingAttachmentRoots(cfg *config.Config) outgoingRoots {
	docsDir := strings.TrimSpace(cfg.Tools.DocumentCreator.OutputDir)
	if docsDir == "" {
		docsDir = "data/documents"
	}
	guards := outgoingProtectedFolders(cfg)
	var roots outgoingRoots
	for _, folder := range []struct{ configured, setting string }{
		{cfg.Directories.WorkspaceDir, outgoingWorkspaceSetting},
		{docsDir, outgoingDocumentsSetting},
	} {
		canonical := canonicalExistingRoot(folder.configured)
		if canonical == "" {
			continue
		}
		if outgoingRootHoldsAny(canonical, guards) {
			roots.dropped = append(roots.dropped, folder.setting)
			continue
		}
		roots.canonical = append(roots.canonical, canonical)
		roots.lexical = append(roots.lexical, canonical)
		if abs, err := filepath.Abs(strings.TrimSpace(folder.configured)); err == nil {
			roots.lexical = append(roots.lexical, abs)
		}
	}
	roots.canonical = uniqueCanonicalPaths(roots.canonical...)
	roots.lexical = uniqueCanonicalPaths(roots.lexical...)
	return roots
}

// outgoingProtectedFolders returns the canonical folders no attachment root may contain:
// the data directory and the directory of the config file. An unset value is skipped,
// because filepath.Dir("") is ".".
func outgoingProtectedFolders(cfg *config.Config) []string {
	var folders []string
	if dir := strings.TrimSpace(cfg.Directories.DataDir); dir != "" {
		folders = append(folders, canonicalExistingRoot(dir))
	}
	if dir := outgoingConfigDir(cfg); dir != "" {
		folders = append(folders, canonicalExistingRoot(dir))
	}
	return uniqueCanonicalPaths(folders...)
}

func outgoingConfigDir(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.ConfigPath) == "" {
		return ""
	}
	return filepath.Dir(cfg.ConfigPath)
}

func outgoingRootHoldsAny(root string, folders []string) bool {
	for _, folder := range folders {
		if pathWithinCanonicalRoot(root, folder) {
			return true
		}
	}
	return false
}

// outgoingProtectedFiles lists the files that never leave AuraGo as attachment: the exact
// files agent file tools protect (config, vault, databases), plus the credential files that
// sit next to the config and in the data directory. Only existing ones matter to the
// hard-link comparison.
func outgoingProtectedFiles(cfg *config.Config) []string {
	files := protectedSystemFilesFromConfig(cfg)
	if cfg == nil {
		return files
	}
	for _, dir := range []string{cfg.Directories.DataDir, outgoingConfigDir(cfg)} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		files = append(files, filepath.Join(dir, ".env"), filepath.Join(dir, "aurago_master.key"))
	}
	return files
}

// outgoingPathIsProtected reports whether the resolved path names protected system state. It
// passes no data directory on purpose: that prefix rule would block the documents folder,
// which lives below data/ in the default layout. Roots that hold the data directory are
// dropped instead.
func outgoingPathIsProtected(resolved string, protected []string) bool {
	return isProtectedSystemPathAbs(resolved, "", protected)
}

// outgoingSameFileAsProtected reports whether info is one of the protected files, which also
// catches a hard link to it under an unremarkable name.
func outgoingSameFileAsProtected(info os.FileInfo, protected []string) bool {
	for _, path := range protected {
		if strings.TrimSpace(path) == "" {
			continue
		}
		other, err := os.Stat(path)
		if err == nil && os.SameFile(info, other) {
			return true
		}
	}
	return false
}

// outgoingPathLexicallyInside reports, without touching the filesystem, whether path lies
// inside one of the roots. The resolver asks it before EvalSymlinks and os.Stat, because on
// Windows those open UNC and device paths: \\host\share\x starts an SMB session and sends the
// host's NTLM hash to host before any containment check could refuse the path.
func outgoingPathLexicallyInside(path string, roots []string) bool {
	path = filepath.Clean(path)
	for _, root := range roots {
		if pathWithinCanonicalRoot(root, path) {
			return true
		}
	}
	return false
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
	if len(roots.canonical) == 0 {
		if len(roots.dropped) > 0 {
			return "", "", outgoingNotFoundError(filePath, roots.dropped)
		}
		return "", "", fmt.Errorf("no attachment folders are configured")
	}
	protected := outgoingProtectedFiles(cfg)
	candidates := []string{filePath}
	if !filepath.IsAbs(filePath) {
		for _, root := range roots.canonical {
			candidates = append(candidates, filepath.Join(root, filePath))
		}
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if !outgoingPathLexicallyInside(abs, roots.lexical) {
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
		for _, root := range roots.canonical {
			if !pathWithinCanonicalRoot(root, resolved) {
				continue
			}
			if outgoingPathIsProtected(resolved, protected) || outgoingSameFileAsProtected(info, protected) {
				return "", "", outgoingProtectedError(filePath)
			}
			return resolved, root, nil
		}
	}
	return "", "", outgoingNotFoundError(filePath, roots.dropped)
}

// outgoingNotFoundError is the one answer for a path that is missing or outside the roots,
// so it does not tell the two apart. When a configured folder was left out, it tells the
// operator how to fix that.
func outgoingNotFoundError(filePath string, dropped []string) error {
	message := fmt.Sprintf("attachment %q is missing or outside the workspace and the documents folder",
		truncateStr(filePath, maxEchoedAttachmentPathRunes))
	for _, setting := range dropped {
		message += fmt.Sprintf(" (the folder set by %s holds AuraGo configuration or data and is not used for attachments; narrow it)", setting)
	}
	return errors.New(message)
}

func outgoingProtectedError(filePath string) error {
	return fmt.Errorf("attachment %q is protected AuraGo configuration, credential or database state",
		truncateStr(filePath, maxEchoedAttachmentPathRunes))
}

// openWithinOutgoingRoot opens resolved, which must lie inside root, through an os.Root
// so that no path component can lead out of root, and checks the open handle.
func openWithinOutgoingRoot(cfg *config.Config, root, resolved string) (*os.File, error) {
	if !pathWithinCanonicalRoot(root, resolved) {
		return nil, fmt.Errorf("the file is outside its folder")
	}
	protected := outgoingProtectedFiles(cfg)
	if outgoingPathIsProtected(resolved, protected) {
		return nil, fmt.Errorf("the file is protected AuraGo state")
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
	if err := requirePlainOutgoingRoot(root, dir); err != nil {
		return nil, err
	}
	// O_NONBLOCK keeps the open of a FIFO that was swapped in from waiting for a writer. It
	// does nothing for a regular file.
	file, err := dir.OpenFile(rel, os.O_RDONLY|outgoingOpenNonblock, 0)
	if err != nil {
		return nil, fmt.Errorf("open the file inside its folder: %w", withoutPathError(err))
	}
	if err := checkOpenedOutgoingFile(file, resolved, protected); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

// isPlainOutgoingDirMode reports whether an Lstat mode names a real directory and not a
// link. Symlinks and junctions (name-surrogate reparse points) never get ModeDir. A
// directory with a reparse tag that is no link, such as a OneDrive or other cloud
// placeholder folder, is ModeDir|ModeIrregular on Windows and stays acceptable.
func isPlainOutgoingDirMode(mode fs.FileMode) bool {
	return mode.IsDir() && mode&fs.ModeSymlink == 0
}

// requirePlainOutgoingRoot refuses a root that was swapped for a symlink or junction after
// it was resolved: os.Root only keeps paths inside the directory it was opened on, and that
// would be the link's target.
func requirePlainOutgoingRoot(root string, dir *os.Root) error {
	linkInfo, err := os.Lstat(root)
	if err != nil || !isPlainOutgoingDirMode(linkInfo.Mode()) {
		return fmt.Errorf("the attachment folder is not a plain directory")
	}
	openInfo, err := dir.Stat(".")
	if err != nil || !os.SameFile(linkInfo, openInfo) {
		return fmt.Errorf("the attachment folder changed while it was opened")
	}
	return nil
}

// checkOpenedOutgoingFile checks the open handle itself: a regular file, the file the path
// names (a case-sensitive Windows directory can make the opened name and the path differ),
// and not a hard link to protected system state.
func checkOpenedOutgoingFile(file *os.File, resolved string, protected []string) error {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("the file is not a regular file")
	}
	current, err := os.Stat(resolved)
	if err != nil || !os.SameFile(info, current) {
		return fmt.Errorf("the file is not the one its path names")
	}
	if outgoingSameFileAsProtected(info, protected) {
		return fmt.Errorf("the file is protected AuraGo state")
	}
	return nil
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
