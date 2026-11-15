package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearchFilesPageFiltersBeforePagination(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	for path, content := range map[string][]byte{
		"search-fixtures/alpha.txt": []byte("four"),
		"search-fixtures/beta.png":  []byte("1234567890"),
		"search-fixtures/gamma.zip": []byte("12345678901234567890"),
	} {
		if err := svc.WriteFileBytes(ctx, path, content, SourceUser); err != nil {
			t.Fatalf("WriteFileBytes(%q): %v", path, err)
		}
	}
	modified := map[string]time.Time{
		"search-fixtures/alpha.txt": time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		"search-fixtures/beta.png":  time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		"search-fixtures/gamma.zip": time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	for name, stamp := range modified {
		path, err := svc.ResolvePath(name)
		if err != nil {
			t.Fatalf("ResolvePath(%q): %v", name, err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("Chtimes(%q): %v", name, err)
		}
	}
	after := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	minimumSize := int64(5)
	page, err := svc.SearchFilesPage(ctx, "search-fixtures", SearchFilesPageOptions{
		Query:         "a",
		Type:          "all",
		ModifiedAfter: &after,
		MinSize:       &minimumSize,
		Limit:         1,
	})
	if err != nil {
		t.Fatalf("SearchFilesPage: %v", err)
	}
	if len(page.Files) != 1 || page.Files[0].Path != "search-fixtures/beta.png" {
		t.Fatalf("filtered first page = %+v, want search-fixtures/beta.png", page.Files)
	}
	if !page.HasMore || page.Truncated || page.NextOffset != 1 {
		t.Fatalf("page metadata = %+v, want more results without truncation", page)
	}

	imagePage, err := svc.SearchFilesPage(ctx, "search-fixtures", SearchFilesPageOptions{Type: "image", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFilesPage image filter: %v", err)
	}
	if len(imagePage.Files) != 1 || imagePage.Files[0].Path != "search-fixtures/beta.png" {
		t.Fatalf("image filter = %+v, want search-fixtures/beta.png", imagePage.Files)
	}

	archivePage, err := svc.SearchFilesPage(ctx, "search-fixtures", SearchFilesPageOptions{Type: "archive", Limit: 10})
	if err != nil {
		t.Fatalf("SearchFilesPage archive filter: %v", err)
	}
	if len(archivePage.Files) != 1 || archivePage.Files[0].Path != "search-fixtures/gamma.zip" {
		t.Fatalf("archive filter = %+v, want search-fixtures/gamma.zip", archivePage.Files)
	}
}

func TestSearchFilesPageValidatesBoundsAndCancellation(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "visible.txt", []byte("ok"), SourceUser); err != nil {
		t.Fatalf("WriteFileBytes: %v", err)
	}
	for name, options := range map[string]SearchFilesPageOptions{
		"query":  {Query: strings.Repeat("é", desktopSearchQueryMaxRunes+1)},
		"type":   {Type: "audio-ish"},
		"limit":  {Limit: desktopSearchPageMax + 1},
		"offset": {Offset: desktopSearchOffsetMax + 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.SearchFilesPage(ctx, "", options); err == nil {
				t.Fatalf("SearchFilesPage(%s) unexpectedly succeeded", name)
			}
		})
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.SearchFilesPage(cancelled, "", SearchFilesPageOptions{}); err == nil {
		t.Fatal("SearchFilesPage with canceled context unexpectedly succeeded")
	}

	root, err := svc.ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "visible.txt")); err != nil {
		t.Fatalf("search fixture missing: %v", err)
	}
}

func TestSearchFilesPageFindsNestedFiles(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "search-fixtures/nested/needle.txt", []byte("inside"), SourceUser); err != nil {
		t.Fatalf("WriteFileBytes nested: %v", err)
	}
	page, err := svc.SearchFilesPage(ctx, "search-fixtures", SearchFilesPageOptions{Query: "needle"})
	if err != nil {
		t.Fatalf("SearchFilesPage: %v", err)
	}
	if len(page.Files) != 1 || page.Files[0].Path != "search-fixtures/nested/needle.txt" {
		t.Fatalf("nested search = %+v, want the rooted nested file", page.Files)
	}
}

func TestSearchFilesPageDoesNotFollowOutsideSymlink(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside-needle.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	searchRoot, err := svc.ResolvePath("search-fixtures")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(searchRoot, "outside-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	page, err := svc.SearchFilesPage(ctx, "search-fixtures", SearchFilesPageOptions{Query: "needle"})
	if err != nil {
		t.Fatalf("SearchFilesPage: %v", err)
	}
	for _, entry := range page.Files {
		if strings.Contains(entry.Path, "outside") {
			t.Fatalf("search returned an outside symlink target: %+v", entry)
		}
	}
	if _, err := svc.SearchFilesPage(ctx, "search-fixtures/outside-link", SearchFilesPageOptions{}); err == nil {
		t.Fatal("SearchFilesPage followed a symlink supplied as the search root")
	}
}

func TestSearchFilesPageTraversalBudgetReportsTruncationWithoutPhantomPage(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	for _, name := range []string{"search-fixtures/alpha.txt", "search-fixtures/beta.png", "search-fixtures/gamma.zip"} {
		if err := svc.WriteFileBytes(ctx, name, []byte("fixture"), SourceUser); err != nil {
			t.Fatalf("WriteFileBytes(%q): %v", name, err)
		}
	}
	workspace, err := os.OpenRoot(svc.Config().WorkspaceDir)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	defer workspace.Close()
	page, err := searchFilesPageRooted(ctx, workspace, "search-fixtures", SearchFilesPageOptions{Type: "all", Limit: 50}, 50, desktopSearchLimits{
		traversal: 1,
		matches:   100,
		depth:     16,
		readBatch: 1,
	})
	if err != nil {
		t.Fatalf("searchFilesPageRooted: %v", err)
	}
	if !page.Truncated {
		t.Fatalf("page metadata = %+v, want traversal truncation", page)
	}
	if page.HasMore {
		t.Fatalf("page metadata = %+v, truncated results do not imply a usable next page", page)
	}
	if len(page.Files) != 1 || page.NextOffset != 1 {
		t.Fatalf("page metadata = %+v, want one inspected entry", page)
	}
}

func TestSearchFilesPageMatchBudgetReportsTruncationWithoutPhantomPage(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	workspacePath := svc.Config().WorkspaceDir
	fixtureDir, err := os.MkdirTemp(workspacePath, "search-match-budget-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(fixtureDir) })
	for _, name := range []string{"needle-a.txt", "needle-b.txt"} {
		if err := os.WriteFile(filepath.Join(fixtureDir, name), []byte("match"), 0o600); err != nil {
			t.Fatalf("write match fixture %q: %v", name, err)
		}
	}
	workspace, err := os.OpenRoot(workspacePath)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	defer workspace.Close()
	page, err := searchFilesPageRooted(ctx, workspace, filepath.Base(fixtureDir), SearchFilesPageOptions{Query: "needle", Type: "all", Limit: 50}, 50, desktopSearchLimits{
		traversal: 10,
		matches:   1,
		depth:     16,
		readBatch: 2,
	})
	if err != nil {
		t.Fatalf("searchFilesPageRooted: %v", err)
	}
	if !page.Truncated {
		t.Fatalf("page metadata = %+v, want match-budget truncation", page)
	}
	if page.HasMore {
		t.Fatalf("page metadata = %+v, truncated results do not imply a usable next page", page)
	}
	if len(page.Files) != 1 || page.NextOffset != 1 {
		t.Fatalf("page metadata = %+v, want one match", page)
	}
}
