package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

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
		diff, err := runDiffCommand(f1, f2)
		if err != nil {
			return encode(TextDiffResult{Status: "error", Message: err.Error()})
		}
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
	// Try `git diff --no-index`
	cmd := exec.Command("git", "diff", "--no-index", file1, file2)
	out, err := cmd.CombinedOutput()
	// git diff exits with 1 if there are differences, which is normal
	if err != nil && cmd.ProcessState.ExitCode() > 1 {
		// Fallback to regular `diff -u`
		cmd2 := exec.Command("diff", "-u", file1, file2)
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil && cmd2.ProcessState.ExitCode() > 1 {
			return "", fmt.Errorf("diff failed: %s", string(out2))
		}
		return string(out2), nil
	}
	return string(out), nil
}

func fileApplyPatch(resolved, patchContent string, encode func(FileEditorResult) string) string {
	if patchContent == "" {
		return encode(FileEditorResult{Status: "error", Message: "patch content is empty"})
	}

	// Write patch to a temp file
	tmpPatch, err := os.CreateTemp("", "patch-*.diff")
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Errorf("failed to create temp patch file: %v", err).Error()})
	}
	defer os.Remove(tmpPatch.Name())
	tmpPatch.WriteString(patchContent)
	tmpPatch.Close()

	// Try `git apply`
	cmd := exec.Command("git", "apply", "--whitespace=nowarn", tmpPatch.Name())
	cmd.Dir = filepath.Dir(resolved) // run in same directory just in case it assumes paths
	out, err := cmd.CombinedOutput()

	if err != nil {
		// Fallback to `patch`
		cmd2 := exec.Command("patch", "-p1", "-i", tmpPatch.Name())
		cmd2.Dir = filepath.Dir(resolved)
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			// One more fallback, try without -p1
			cmd3 := exec.Command("patch", "-i", tmpPatch.Name())
			cmd3.Dir = filepath.Dir(resolved)
			out3, err3 := cmd3.CombinedOutput()
			if err3 != nil {
				return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("failed to apply patch: git apply said: %s \npatch -p1 said: %s \npatch said: %s", string(out), string(out2), string(out3))})
			}
		}
	}

	return encode(FileEditorResult{Status: "success", Message: "Patch applied successfully"})
}
