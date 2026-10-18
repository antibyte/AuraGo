package ui

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// dashboardSectionIDs lists the dashboard sections in sidebar order (P2 contract §1).
var dashboardSectionIDs = []string{
	"overview", "personality", "memory", "user", "knowledge", "filesync", "problems",
	"automations", "system", "audit", "logs", "llm", "prompts", "tools",
}

// dashboardSectionPartialFiles returns the section partials as ui-relative, slash-separated, sorted paths.
func dashboardSectionPartialFiles(t *testing.T) []string {
	t.Helper()
	files, err := fs.Glob(Content, "dashboard/sections/*.html")
	if err != nil {
		t.Fatalf("glob dashboard section partials: %v", err)
	}
	sort.Strings(files)
	return files
}

// readDashboardShell returns ui/dashboard.html alone (head, header, sidebar, scripts, modals), LF line endings.
func readDashboardShell(t *testing.T) string {
	t.Helper()
	return normalizeAssetText(mustReadUIFile(t, "dashboard.html"))
}

// readDashboardMarkup returns the whole dashboard markup: the shell followed by every section partial
// (sorted by file name). Tests that used to read dashboard.html as one file use this.
func readDashboardMarkup(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(readDashboardShell(t))
	for _, file := range dashboardSectionPartialFiles(t) {
		b.WriteString("\n")
		b.WriteString(normalizeAssetText(mustReadUIFile(t, file)))
	}
	return b.String()
}

// readDashboardSection returns ui/dashboard/sections/<id>.html, LF line endings.
func readDashboardSection(t *testing.T, id string) string {
	t.Helper()
	return normalizeAssetText(mustReadUIFile(t, "dashboard/sections/"+id+".html"))
}

// dashboardDivEnd returns the end offset of the <div> element that starts at start.
func dashboardDivEnd(markup string, start int) (int, bool) {
	depth := 0
	for i := start; i < len(markup); {
		switch {
		case strings.HasPrefix(markup[i:], "<div"):
			depth++
			i += len("<div")
		case strings.HasPrefix(markup[i:], "</div>"):
			depth--
			i += len("</div>")
			if depth == 0 {
				return i, true
			}
		default:
			i++
		}
	}
	return 0, false
}

// dashboardCardMarkup returns the complete <div> element whose opening tag carries id="<cardID>".
func dashboardCardMarkup(t *testing.T, markup, cardID string) string {
	t.Helper()
	at := strings.Index(markup, `id="`+cardID+`"`)
	if at < 0 {
		t.Fatalf("dashboard markup has no element id=%q", cardID)
	}
	start := strings.LastIndex(markup[:at], "<div")
	if start < 0 || strings.Contains(markup[start:at], ">") {
		t.Fatalf("id=%q is not on a <div> opening tag", cardID)
	}
	end, ok := dashboardDivEnd(markup, start)
	if !ok {
		t.Fatalf("element id=%q is not closed", cardID)
	}
	return markup[start:end]
}
