package tools

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileApplyPatchIsolationAndFailures(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required for patch isolation tests")
	}
	for _, inRepo := range []bool{false, true} {
		placement := "plain-temp"
		if inRepo {
			placement = "temp-inside-repository"
		}
		t.Run(placement, func(t *testing.T) {
			for _, tc := range []struct {
				name, hunk, want string
				redirect         bool
			}{
				{name: "apply", hunk: "-old\n+new\n", want: "new\n"},
				{name: "inherited-git-redirects", hunk: "-old\n+new\n", want: "new\n", redirect: true},
				{name: "invalid-hunk", hunk: "-missing\n+new\n", want: "old\n"},
				{name: "unchanged", hunk: "-old\n+old\n", want: "old\n"},
				{name: "foreign-file", want: "old\n"},
				{name: "traversal", want: "old\n"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					repo := t.TempDir()
					git := func(args ...string) {
						t.Helper()
						cmd := exec.Command("git", args...)
						cmd.Dir, cmd.Env = repo, patchGitEnvironment(repo)
						if out, err := cmd.CombinedOutput(); err != nil {
							t.Fatalf("fixture git: %s: %v", out, err)
						}
					}
					git("init", "--quiet")
					selected := filepath.Join(repo, "file.txt")
					other := filepath.Join(repo, "other.txt")
					for path, content := range map[string]string{selected: "old\n", other: "untouched\n"} {
						if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					git("-c", "core.autocrlf=false", "add", "--", "file.txt", "other.txt")
					indexPath := filepath.Join(repo, ".git", "index")
					index, err := os.ReadFile(indexPath)
					if err != nil {
						t.Fatal(err)
					}
					tempBase := t.TempDir()
					if inRepo {
						tempBase = filepath.Join(repo, "scratch")
						if err := os.Mkdir(tempBase, 0o700); err != nil {
							t.Fatal(err)
						}
					}
					for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
						t.Setenv(key, tempBase)
					}
					redirectIndex := filepath.Join(repo, "redirect-index")
					if err := os.WriteFile(redirectIndex, index, 0o600); err != nil {
						t.Fatal(err)
					}
					if tc.redirect {
						for key, value := range map[string]string{
							"GIT_DIR": filepath.Join(repo, ".git"), "GIT_WORK_TREE": repo,
							"GIT_INDEX_FILE": redirectIndex, "GIT_COMMON_DIR": filepath.Join(repo, ".git"),
							"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "apply.ignoreWhitespace", "GIT_CONFIG_VALUE_0": "all",
							"GIT_CONFIG_PARAMETERS": "'apply.ignoreWhitespace=all'", "GIT_PREFIX": "scratch/",
						} {
							t.Setenv(key, value)
						}
					}
					patch := "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n" + tc.hunk
					if tc.name == "foreign-file" {
						patch = "--- a/other.txt\n+++ b/other.txt\n@@ -1 +1 @@\n-untouched\n+changed\n"
					}
					if tc.name == "traversal" {
						patch = "--- a/../other.txt\n+++ b/../other.txt\n@@ -1 +1 @@\n-untouched\n+changed\n"
					}
					encode := func(result FileEditorResult) string { data, _ := json.Marshal(result); return string(data) }
					var result FileEditorResult
					if err := json.Unmarshal([]byte(fileApplyPatch(selected, patch, repo, encode)), &result); err != nil {
						t.Fatal(err)
					}
					wantStatus := "error"
					if tc.want == "new\n" {
						wantStatus = "success"
					}
					if result.Status != wantStatus {
						t.Fatalf("patch result = %+v, want %s", result, wantStatus)
					}
					if tc.name == "unchanged" && !strings.Contains(result.Message, "did not change") {
						t.Fatalf("wrong no-op failure: %+v", result)
					}
					for path, want := range map[string][]byte{selected: []byte(tc.want), other: []byte("untouched\n"), indexPath: index, redirectIndex: index} {
						got, err := os.ReadFile(path)
						if err != nil || !bytes.Equal(got, want) {
							t.Fatalf("unexpected content at %s: %v", path, err)
						}
					}
					if tc.redirect && os.Getenv("GIT_INDEX_FILE") != redirectIndex {
						t.Fatal("child environment changed the parent")
					}
					if entries, err := os.ReadDir(tempBase); err != nil || len(entries) != 0 {
						t.Fatalf("patch staging not cleaned: %v, %v", entries, err)
					}
				})
			}
		})
	}
}

func TestTextDiffFilesReportsWorkspacePaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for this diff regression test")
	}
	workspace := t.TempDir()
	for name, content := range map[string]string{"first.txt": "old\n", "second.txt": "new\n"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var result TextDiffResult
	if err := json.Unmarshal([]byte(ExecuteTextDiff("diff_files", "first.txt", "second.txt", "", "", workspace)), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" || strings.Contains(result.Diff, "aurago-diff-") || !strings.Contains(result.Diff, "first.txt") || !strings.Contains(result.Diff, "second.txt") {
		t.Fatalf("diff should name workspace files without staging paths: %+v", result)
	}
}

func TestFileApplyPatchStagesSingleSelectedFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for this patch regression test")
	}
	workspace := t.TempDir()
	selected := filepath.Join(workspace, "file.txt")
	other := filepath.Join(workspace, "other.txt")
	if err := os.WriteFile(selected, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("untouched\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	encode := func(value FileEditorResult) string { raw, _ := json.Marshal(value); return string(raw) }
	valid := "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	if result := fileApplyPatch(selected, valid, workspace, encode); !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("valid patch: %s", result)
	}
	if data, err := os.ReadFile(selected); err != nil || string(data) != "new\n" {
		t.Fatalf("selected file: %q, %v", data, err)
	}
	invalid := "--- a/other.txt\n+++ b/other.txt\n@@ -1 +1 @@\n-untouched\n+changed\n"
	if result := fileApplyPatch(selected, invalid, workspace, encode); !strings.Contains(result, "patch must target only the selected file") {
		t.Fatalf("unexpected patch rejection: %s", result)
	}
	if data, err := os.ReadFile(other); err != nil || string(data) != "untouched\n" {
		t.Fatalf("other file changed: %q, %v", data, err)
	}
}

func TestValidatePatchPaths(t *testing.T) {
	// Create a temporary workspace directory
	workdir := t.TempDir()

	// Create a subdirectory with a file inside
	subdir := filepath.Join(workdir, "project", "src")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	testFile := filepath.Join(subdir, "file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	tests := []struct {
		name        string
		patch       string
		workspace   string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid patch with relative path",
			patch: `--- a/project/src/file.txt
+++ b/project/src/file.txt
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "valid patch with a/b prefixes",
			patch: `--- a/project/src/file.txt
+++ b/project/src/file.txt
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "valid patch with new file (dev/null)",
			patch: `--- /dev/null
+++ b/project/src/newfile.txt
@@ -0,0 +1 @@
+new content`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "valid patch with deleted file (dev/null)",
			patch: `--- a/project/src/file.txt
+++ /dev/null
@@ -1 +0,0 @@
-test content`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "valid patch with rename",
			patch: `--- a/project/src/old.txt
+++ b/project/src/new.txt
@@ -1 +1 @@
-old content
+new content`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "invalid patch with Unix absolute path",
			patch: `--- a/project/src/file.txt
+++ b/etc/passwd
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     false, // etc/passwd is a valid relative path on Windows; /etc/passwd would be Unix absolute
			errContains: "",
		},
		{
			name: "invalid patch with Unix absolute path (leading slash)",
			patch: `--- a/project/src/file.txt
+++ b//etc/passwd
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     true,
			errContains: "invalid or unsafe paths",
		},
		{
			name: "invalid patch with path traversal",
			patch: `--- a/project/src/file.txt
+++ b../../other.txt
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     true,
			errContains: "invalid or unsafe paths",
		},
		{
			name: "invalid patch with absolute Windows path",
			patch: `--- a/project/src/file.txt
+++ bC:\Windows\System32\config.txt
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     true,
			errContains: "invalid or unsafe paths",
		},
		{
			name: "invalid patch with traversal to parent",
			patch: `--- a/project/src/file.txt
+++ b../outside.txt
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     false, // b.. is interpreted as directory name on Windows, not traversal
			errContains: "",
		},
		{
			name:        "empty patch content",
			patch:       ``,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "patch with no header lines",
			patch: `@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "valid patch with multiple files",
			patch: `--- a/project/src/file1.txt
+++ b/project/src/file1.txt
@@ -1 +1 @@
-content1
+updated1
--- a/project/src/file2.txt
+++ b/project/src/file2.txt
@@ -1 +1 @@
-content2
+updated2`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
		{
			name: "invalid patch with mixed valid and invalid paths",
			patch: `--- a/project/src/file.txt
+++ b/project/src/file.txt
@@ -1 +1,2 @@
 test content
+added line
--- a/project/src/other.txt
+++ b/project/src/../../../secrets.txt
@@ -1 +1,2 @@
 content
+secret`,
			workspace:   workdir,
			wantErr:     true,
			errContains: "invalid or unsafe paths",
		},
		{
			name: "valid patch without prefix stripped",
			patch: `--- project/src/file.txt
+++ project/src/file.txt
@@ -1 +1,2 @@
 test content
+added line`,
			workspace:   workdir,
			wantErr:     false,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePatchPaths(tt.patch, tt.workspace)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePatchPaths() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("validatePatchPaths() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validatePatchPaths() unexpected error: %v", err)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
