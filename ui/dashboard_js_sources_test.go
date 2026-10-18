package ui

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// readDashboardJS returns every ui/js/dashboard/**/*.js file, sorted by path, with LF line endings, as one string.
// Presence and count pins read this instead of a single file, so moving code between dashboard scripts (the P4
// section modules) keeps them meaningful; pins that must hold within one file read that file.
func readDashboardJS(t *testing.T) string {
	t.Helper()
	var files []string
	if err := fs.WalkDir(Content, "js/dashboard", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".js") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk js/dashboard: %v", err)
	}
	sort.Strings(files)
	var b strings.Builder
	for _, file := range files {
		data, err := fs.ReadFile(Content, file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		b.WriteString(normalizeAssetText(data))
		b.WriteString("\n")
	}
	return b.String()
}

// TestDashboardSectionScriptsLoadInPageOrder: the registry and the disclosure state load after dash-overview.js and before main.js; the 14
// section modules load deferred, cache-busted and in sidebar order between widgets-knowledge.js and
// dashboard-events.js, so main.js's helpers exist when a module runs and every module has registered before
// dashboard-events.js boots the router on DOMContentLoaded.
func TestDashboardSectionScriptsLoadInPageOrder(t *testing.T) {
	t.Parallel()
	order := []string{"/js/dashboard/dash-overview.js", "/js/dashboard/dash-sections.js", "/js/dashboard/dash-disclosures.js", "/js/dashboard/main.js", "/js/dashboard/widgets-knowledge.js"}
	for _, id := range dashboardSectionIDs {
		order = append(order, "/js/dashboard/sections/"+id+".js")
	}
	order = append(order, "/js/dashboard/dashboard-events.js")
	shell := readDashboardShell(t)
	assertDashboardScriptOrder(t, shell, order)
	// Decision 3: the emptied file is deleted, and nothing loads it any more.
	if strings.Contains(shell, "dashboard-widgets") {
		t.Error("dashboard.html still loads dashboard-widgets.js")
	}
	if _, err := fs.Stat(Content, "js/dashboard/dashboard-widgets.js"); err == nil {
		t.Error("ui/js/dashboard/dashboard-widgets.js must be gone: its code lives in the section modules")
	}
}

// assertDashboardScriptOrder checks that every src has exactly one deferred, cache-busted script tag in the shell
// and that the tags appear in the given order.
func assertDashboardScriptOrder(t *testing.T, shell string, order []string) {
	t.Helper()
	previous := -1
	for _, src := range order {
		tag := `<script src="` + src + `?v={{.BuildVersion}}" defer></script>`
		if n := strings.Count(shell, tag); n != 1 {
			t.Errorf("dashboard.html has %d tags %s, want 1", n, tag)
			continue
		}
		at := strings.Index(shell, tag)
		if at < previous {
			t.Errorf("%s loads out of order", src)
		}
		previous = at
	}
}
