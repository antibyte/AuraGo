package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// devNullPath is the special marker used in unified diffs for non-existent files.
const devNullPath = "/dev/null"

// patchPathRegex matches the path in --- and +++ diff header lines.
// Unified diff format: --- a/path/to/file or +++ b/path/to/file
var patchPathRegex = regexp.MustCompile(`^[+-]{3}\s+([^\t\n]+)`)

// validatePatchPaths validates all file paths referenced in a unified diff/patch
// before it is applied. It extracts paths from --- and +++ header lines and
// validates each one to ensure they stay within workspace bounds.
// The special marker /dev/null is allowed for new (+++ /dev/null) or deleted
// (--- /dev/null) files.
func validatePatchPaths(patchContent, workspaceDir string) error {
	if patchContent == "" {
		return nil // empty patch is handled elsewhere
	}

	// Resolve workspace to absolute path
	absWorkdir, err := filepath.Abs(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace path: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(patchContent))
	var invalidPaths []string

	for scanner.Scan() {
		line := scanner.Text()
		matches := patchPathRegex.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}

		path := strings.TrimSpace(matches[1])

		// Allow /dev/null which is a special marker in unified diffs
		// for files that don't exist (either new or deleted)
		if path == devNullPath || path == "" {
			continue
		}

		// Strip a/ or A/ prefix if present (common in git diff output)
		// Check case-insensitively since git may use a/ or A/
		// Handle both "a/" and cases like "aC:/" where the path looks like it starts with a drive letter
		if len(path) >= 2 {
			lower2 := strings.ToLower(path[:2])
			if lower2 == "a/" || lower2 == "a\\" {
				path = path[2:]
			} else if lower2 == "b/" || lower2 == "b\\" {
				path = path[2:]
			} else if len(path) >= 3 {
				// Handle "aC:\..." or "bC:\..." style paths where the drive letter follows the prefix
				// Check if first char is a or b (case-insensitive) and third char is : (drive letter)
				firstChar := strings.ToLower(path[:1])
				thirdChar := path[2]
				if (firstChar == "a" || firstChar == "b") && thirdChar == ':' {
					// This looks like a/ C:\... or b/ C:\... pattern
					// Strip just the first character (the git prefix)
					path = path[1:]
				}
			}
		}

		// Check for empty path after stripping prefix
		if path == "" {
			invalidPaths = append(invalidPaths, fmt.Sprintf("%s: empty path after stripping prefix", matches[1]))
			continue
		}

		// Reject absolute paths - they are almost certainly malicious
		if filepath.IsAbs(path) {
			invalidPaths = append(invalidPaths, fmt.Sprintf("%s: absolute path not allowed", path))
			continue
		}

		// On Windows, paths like "C:..." are absolute but filepath.IsAbs might not catch them
		// in all contexts. Check for common Windows absolute path patterns.
		// Also check for paths that start with a drive letter pattern (e.g., "C:/" or "C:\")
		if len(path) >= 2 && path[1] == ':' && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) {
			invalidPaths = append(invalidPaths, fmt.Sprintf("%s: Windows absolute path not allowed", path))
			continue
		}

		// Reject paths with traversal attempts (contains .. as path component)
		// This catches paths like ../../../etc/passwd or ../outside.txt
		pathParts := strings.Split(filepath.ToSlash(path), "/")
		hasTraversal := false
		for _, part := range pathParts {
			if part == ".." {
				invalidPaths = append(invalidPaths, fmt.Sprintf("%s: path traversal not allowed", path))
				hasTraversal = true
				break
			}
		}
		if hasTraversal {
			continue
		}

		// Reject Unix-style absolute paths (starting with /) - they are dangerous
		// even on Windows, as they could be used to access WSL paths or container paths
		if strings.HasPrefix(path, "/") {
			invalidPaths = append(invalidPaths, fmt.Sprintf("%s: Unix absolute path not allowed", path))
			continue
		}

		// Check if resolved path would escape workspace
		fullPath := filepath.Join(absWorkdir, path)
		cleanPath := filepath.Clean(fullPath)
		rel, err := filepath.Rel(absWorkdir, cleanPath)
		if err != nil {
			invalidPaths = append(invalidPaths, fmt.Sprintf("%s: failed to resolve path: %v", path, err))
			continue
		}
		// Check if relative path starts with .. which means it escapes workspace
		if strings.HasPrefix(rel, "..") {
			invalidPaths = append(invalidPaths, fmt.Sprintf("%s: path escapes workspace boundary", path))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to parse patch content: %w", err)
	}

	if len(invalidPaths) > 0 {
		return fmt.Errorf("patch contains invalid or unsafe paths: %s", strings.Join(invalidPaths, "; "))
	}

	return nil
}

type TextDiffResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Diff    string `json:"diff,omitempty"`
}

func ExecuteTextDiff(operation, file1, file2, text1, text2, workspaceDir string) string {
	encode := func(r TextDiffResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "diff_files":
		f1, err := secureResolve(workspaceDir, file1)
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: fmt.Sprintf("invalid file1: %v", err)})
		}
		f2, err := secureResolve(workspaceDir, file2)
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: fmt.Sprintf("invalid file2: %v", err)})
		}
		first, err := rootedToolReadFile(f1)
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		second, err := rootedToolReadFile(f2)
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		tempDir, err := os.MkdirTemp("", "aurago-diff-*")
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		defer os.RemoveAll(tempDir)
		sourceOne, sourceTwo := f1, f2
		f1, f2 = filepath.Join(tempDir, "first"), filepath.Join(tempDir, "second")
		if err := os.WriteFile(f1, first, 0o600); err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		if err := os.WriteFile(f2, second, 0o600); err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		diff, err := runDiffCommandInDir("first", "second", tempDir)
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		diff = strings.NewReplacer(
			"a/first", "a/"+filepath.ToSlash(sourceOne),
			"b/second", "b/"+filepath.ToSlash(sourceTwo),
			filepath.ToSlash(f1), filepath.ToSlash(sourceOne),
			filepath.ToSlash(f2), filepath.ToSlash(sourceTwo),
			f1, sourceOne, f2, sourceTwo,
		).Replace(diff)
		return encode(TextDiffResult{Status: "success", Diff: diff})

	case "diff_strings":
		// Write strings to temp files to diff them
		tmp1, err := os.CreateTemp("", "diff1-*.txt")
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		defer os.Remove(tmp1.Name())
		tmp1.WriteString(text1)
		tmp1.Close()

		tmp2, err := os.CreateTemp("", "diff2-*.txt")
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		defer os.Remove(tmp2.Name())
		tmp2.WriteString(text2)
		tmp2.Close()

		diff, err := runDiffCommand(tmp1.Name(), tmp2.Name())
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
		return encode(TextDiffResult{Status: "success", Diff: diff})

	default:
		return encode(TextDiffResult{Status: "error", Message: fmt.Sprintf("unknown operation '%s'", operation)})
	}
}

func runDiffCommand(file1, file2 string) (string, error) {
	return runDiffCommandInDir(file1, file2, "")
}

func runDiffCommandInDir(file1, file2, dir string) (string, error) {
	// Try `git diff --no-index`
	cmd := exec.Command("git", "-c", "core.autocrlf=false", "diff", "--no-index", file1, file2)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	// git diff exits with 1 if there are differences, which is normal
	if err != nil && cmd.ProcessState.ExitCode() > 1 {
		// Fallback to regular `diff -u`
		cmd2 := exec.Command("diff", "-u", file1, file2)
		cmd2.Dir = dir
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil && cmd2.ProcessState.ExitCode() > 1 {
			return "", fmt.Errorf("diff failed: %s", string(out2))
		}
		return string(out2), nil
	}
	return string(out), nil
}

func fileApplyPatch(resolved, patchContent, workspaceDir string, encode func(FileEditorResult) string) string {
	if patchContent == "" {
		return encode(FileEditorResult{Status: "error", Message: "patch content is empty"})
	}

	// Validate all paths in the patch before applying
	if workspaceDir != "" {
		if err := validatePatchPaths(patchContent, workspaceDir); err != nil {
			return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("patch validation failed: %v", err)})
		}
	}
	name := filepath.Base(resolved)
	count := 0
	for _, line := range strings.Split(patchContent, "\n") {
		matches := patchPathRegex.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) < 2 {
			continue
		}
		candidate := strings.TrimSpace(matches[1])
		candidate = strings.TrimPrefix(strings.TrimPrefix(candidate, "a/"), "b/")
		if candidate != name {
			return encode(FileEditorResult{Status: "error", Message: "patch must target only the selected file"})
		}
		count++
	}
	if count != 2 {
		return encode(FileEditorResult{Status: "error", Message: "patch must contain one file change"})
	}
	original, err := rootedToolReadFile(resolved)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("cannot read selected file: %v", err)})
	}
	stagingDir, err := os.MkdirTemp("", "aurago-patch-*")
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("cannot stage patch: %v", err)})
	}
	defer os.RemoveAll(stagingDir)
	stagedFile := filepath.Join(stagingDir, name)
	if err := os.WriteFile(stagedFile, original, 0o600); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("cannot stage selected file: %v", err)})
	}

	// Write patch to a temp file
	tmpPatch, err := os.CreateTemp(stagingDir, "patch-*.diff")
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Errorf("failed to create temp patch file: %v", err).Error()})
	}
	defer os.Remove(tmpPatch.Name())
	tmpPatch.WriteString(patchContent)
	tmpPatch.Close()

	// Try `git apply`
	cmd := exec.Command("git", "-c", "core.autocrlf=false", "apply", "--whitespace=nowarn", tmpPatch.Name())
	cmd.Dir = stagingDir
	out, err := cmd.CombinedOutput()

	if err != nil {
		// Fallback to `patch`
		cmd2 := exec.Command("patch", "-p1", "-i", tmpPatch.Name())
		cmd2.Dir = stagingDir
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			// One more fallback, try without -p1
			cmd3 := exec.Command("patch", "-i", tmpPatch.Name())
			cmd3.Dir = stagingDir
			out3, err3 := cmd3.CombinedOutput()
			if err3 != nil {
				return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("failed to apply patch: git apply said: %s \npatch -p1 said: %s \npatch said: %s", string(out), string(out2), string(out3))})
			}
		}
	}

	updated, err := os.ReadFile(stagedFile)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("patched file missing: %v", err)})
	}
	if err := rootedToolWriteFileAtomic(resolved, updated); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("cannot save patched file: %v", err)})
	}
	return encode(FileEditorResult{Status: "success", Message: "Patch applied successfully"})
}
