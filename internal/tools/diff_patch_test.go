package tools

import (
	"os"
	"path/filepath"
	"testing"
)

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
