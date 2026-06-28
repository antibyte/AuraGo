package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func newWorkspaceSearchTestService(t *testing.T, maxFileSizeMB int) (*WorkspaceSearchService, *config.Config) {
	t.Helper()

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	workspaceDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir
	cfg.WorkspaceSearch.Enabled = true
	cfg.WorkspaceSearch.MaxFileSizeMB = maxFileSizeMB
	cfg.WorkspaceSearch.MaxIndexSizeMB = 64
	cfg.WorkspaceSearch.MaxResults = 20
	cfg.WorkspaceSearch.PollIntervalSeconds = 3600
	cfg.WorkspaceSearch.FuzzyThreshold = 0.25
	cfg.WorkspaceSearch.Exclude = []string{".git", "node_modules", "__pycache__", "venv", ".venv", ".env", "*.db", "*.bin"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := NewWorkspaceSearchService(cfg, nil, logger)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() {
		svc.Stop()
		if err := svc.Close(); err != nil {
			t.Fatalf("close service: %v", err)
		}
	})
	return svc, cfg
}

func writeWorkspaceSearchFixture(t *testing.T, workspaceDir, rel, content string) {
	t.Helper()
	path := filepath.Join(workspaceDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture dir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", rel, err)
	}
}

func TestWorkspaceSearchScanSkipsUnsafeAndExcludedContent(t *testing.T) {
	ctx := context.Background()
	svc, cfg := newWorkspaceSearchTestService(t, 1)
	ws := cfg.Directories.WorkspaceDir

	writeWorkspaceSearchFixture(t, ws, "src/app.go", "package main\nfunc main() {}\n")
	writeWorkspaceSearchFixture(t, ws, "docs/readme.md", "needle public\n")
	writeWorkspaceSearchFixture(t, ws, ".env", "needle secret\n")
	writeWorkspaceSearchFixture(t, ws, "node_modules/pkg/a.js", "needle dependency\n")
	writeWorkspaceSearchFixture(t, ws, "cache.db", "needle database\n")
	if err := os.WriteFile(filepath.Join(ws, "binary.bin"), []byte{0, 1, 2, 'n', 'e', 'e', 'd', 'l', 'e'}, 0o644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}
	large := strings.Repeat("x", 1024*1024+64) + "needle large"
	writeWorkspaceSearchFixture(t, ws, "large.txt", large)

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("needle outside\n"), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	symlinkCreated := true
	if err := os.Symlink(outside, filepath.Join(ws, "leak.txt")); err != nil {
		symlinkCreated = false
	}

	if err := svc.Rescan(ctx); err != nil {
		t.Fatalf("rescan: %v", err)
	}

	matches, err := svc.Grep(ctx, WorkspaceSearchRequest{
		Query: "needle",
		Glob:  "**/*",
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	got := map[string]bool{}
	for _, match := range matches.Matches {
		got[match.File] = true
	}
	if !got["docs/readme.md"] {
		t.Fatalf("expected docs/readme.md in grep results, got %#v", got)
	}
	for _, forbidden := range []string{".env", "node_modules/pkg/a.js", "cache.db", "binary.bin", "large.txt"} {
		if got[forbidden] {
			t.Fatalf("grep result included skipped file %s: %#v", forbidden, got)
		}
	}
	if symlinkCreated && got["leak.txt"] {
		t.Fatalf("grep result included symlink escape: %#v", got)
	}

	files, err := svc.Find(ctx, WorkspaceSearchRequest{Query: "large", Limit: 10})
	if err != nil {
		t.Fatalf("find large: %v", err)
	}
	if !containsWorkspaceSearchPath(files, "large.txt") {
		t.Fatalf("large text file should remain path-searchable, got %#v", files)
	}
}

func TestWorkspaceSearchFindGlobGrepAndRegexErrors(t *testing.T) {
	ctx := context.Background()
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ws := cfg.Directories.WorkspaceDir

	writeWorkspaceSearchFixture(t, ws, "src/app.go", "package main\n// Needle\n")
	writeWorkspaceSearchFixture(t, ws, "src/app_test.go", "package main\n")
	writeWorkspaceSearchFixture(t, ws, "docs/readme.md", "needle lowercase\n")

	if err := svc.Rescan(ctx); err != nil {
		t.Fatalf("rescan: %v", err)
	}

	files, err := svc.Find(ctx, WorkspaceSearchRequest{Query: "app", Limit: 10})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(files) == 0 || files[0].Path != "src/app.go" {
		t.Fatalf("find app first result = %#v, want src/app.go first", files)
	}

	globbed, err := svc.Glob(ctx, WorkspaceSearchRequest{Glob: "**/*_test.go", Limit: 10})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(globbed) != 1 || globbed[0].Path != "src/app_test.go" {
		t.Fatalf("glob results = %#v, want only src/app_test.go", globbed)
	}

	matches, err := svc.Grep(ctx, WorkspaceSearchRequest{
		Query:         "needle",
		Glob:          "**/*",
		Mode:          "plain",
		CaseSensitive: false,
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("plain grep: %v", err)
	}
	if len(matches.Matches) != 2 {
		t.Fatalf("plain grep matches = %#v, want 2", matches.Matches)
	}

	if _, err := svc.Grep(ctx, WorkspaceSearchRequest{Query: "[", Mode: "regex"}); err == nil {
		t.Fatal("invalid regex should fail")
	} else if !errors.Is(err, ErrWorkspaceSearchInvalidPattern) {
		t.Fatalf("invalid regex error = %v, want ErrWorkspaceSearchInvalidPattern", err)
	}
}

func TestWorkspaceSearchFrecencyBoostsFindRanking(t *testing.T) {
	ctx := context.Background()
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ws := cfg.Directories.WorkspaceDir

	writeWorkspaceSearchFixture(t, ws, "docs/aaa-target.md", "target\n")
	writeWorkspaceSearchFixture(t, ws, "docs/zzz-target.md", "target\n")

	if err := svc.Rescan(ctx); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	before, err := svc.Find(ctx, WorkspaceSearchRequest{Query: "target", Limit: 10})
	if err != nil {
		t.Fatalf("find before access: %v", err)
	}
	if len(before) < 2 || before[0].Path != "docs/aaa-target.md" {
		t.Fatalf("baseline ordering = %#v, want alphabetical aaa first", before)
	}

	for i := 0; i < 3; i++ {
		if err := svc.TrackAccess("docs/zzz-target.md", "read"); err != nil {
			t.Fatalf("track access: %v", err)
		}
	}
	after, err := svc.Find(ctx, WorkspaceSearchRequest{Query: "target", Limit: 10})
	if err != nil {
		t.Fatalf("find after access: %v", err)
	}
	if len(after) == 0 || after[0].Path != "docs/zzz-target.md" {
		t.Fatalf("frecency ordering = %#v, want zzz-target first", after)
	}
}

func containsWorkspaceSearchPath(files []WorkspaceSearchFileResult, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}
