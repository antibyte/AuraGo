package audit

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Doc contract of the dashboard redesign (P6). Chapter 13 (en/de) describes the 14 sidebar sections under their UI
// labels from ui/lang/dashboard/<lang>.json, the overview, every attention code and the persistence keys, and the two
// files stay line-for-line mirrors. Chapter 21 (en/de) documents GET /api/dashboard/summary and the real system
// metrics shape. README shows the fresh 1440 px dashboard screenshot outside the Chat block. The Local Wikipedia pins
// of both chapters stay in local_wikipedia_docs_test.go.

// dashboardDocSections are the section ids in sidebar order (ui/js/dashboard/dash-router.js SECTIONS); the overview
// has its own "##" heading, every other section a "###" heading below its group.
var dashboardDocSections = []string{
	"overview", "personality", "memory", "user", "knowledge", "filesync", "problems",
	"automations", "system", "audit", "logs", "llm", "prompts", "tools",
}

var dashboardDocGroups = []string{"agent", "knowledge", "operations", "analytics"}

func dashboardDocLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	path := "ui/lang/dashboard/" + lang + ".json"
	text := strings.TrimPrefix(readRepoFile(t, path), string(rune(0xFEFF)))
	var raw map[string]any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	strs := make(map[string]string, len(raw))
	for key, value := range raw {
		if s, ok := value.(string); ok {
			strs[key] = strings.TrimSpace(s)
		}
	}
	return strs
}

func dashboardDocLabel(t *testing.T, locale map[string]string, lang, key string) string {
	t.Helper()
	if locale[key] == "" {
		t.Fatalf("ui/lang/dashboard/%s.json has no value for %s", lang, key)
	}
	return locale[key]
}

func dashboardDocAttentionCodes(t *testing.T) []string {
	t.Helper()
	src := readRepoFile(t, "ui/js/dashboard/dash-overview.js")
	block := regexp.MustCompile(`(?s)const ATTENTION_CODES = Object\.freeze\(\[(.*?)\]\);`).FindStringSubmatch(src)
	if block == nil {
		t.Fatal("ui/js/dashboard/dash-overview.js: ATTENTION_CODES not found")
	}
	var codes []string
	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(block[1], -1) {
		codes = append(codes, m[1])
	}
	if len(codes) < 11 {
		t.Fatalf("ATTENTION_CODES lists %d codes, want at least 11", len(codes))
	}
	return codes
}

// markdownHeadingLines lists "<line>:<level>" for every ATX heading outside fenced code blocks.
func markdownHeadingLines(doc string) []string {
	var out []string
	fenced := false
	for i, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(line, "#") {
			continue
		}
		level := len(line) - len(strings.TrimLeft(line, "#"))
		if level <= 6 && strings.HasPrefix(line[level:], " ") {
			out = append(out, fmt.Sprintf("%d:%d", i+1, level))
		}
	}
	return out
}

// markdownBlock returns doc from start up to (not including) the first end after it.
func markdownBlock(t *testing.T, name, doc, start, end string) string {
	t.Helper()
	i := strings.Index(doc, start)
	if i < 0 {
		t.Fatalf("%s has no %q", name, start)
	}
	j := strings.Index(doc[i+len(start):], end)
	if j < 0 {
		t.Fatalf("%s has no %q after %q", name, end, start)
	}
	return doc[i : i+len(start)+j]
}

func lfDoc(t *testing.T, path string) string {
	t.Helper()
	return strings.ReplaceAll(readRepoFile(t, path), "\r\n", "\n")
}

func TestDashboardDocsManualChapter13(t *testing.T) {
	t.Parallel()
	forbidden := map[string][]string{
		"en": {"eight tabs", "Agent Status Banner", "Quick Status", "## Tab:", "Agent tab", "System tab", "Overview tab"},
		"de": {"acht Tabs", "Agent-Status-Banner", "Quick Status", "Quick-Status", "## Tab:", "Agent-Tab", "System-Tab", "Tab-Navigation"},
	}
	codes := dashboardDocAttentionCodes(t)
	docs := map[string]string{}
	for _, lang := range []string{"en", "de"} {
		name := "manual/" + lang + "/13-dashboard.md"
		doc := lfDoc(t, "documentation/manuals/"+lang+"/13-dashboard.md")
		docs[lang] = doc
		for _, gone := range forbidden[lang] {
			if strings.Contains(doc, gone) {
				t.Errorf("%s still describes the old tab layout: %q", name, gone)
			}
		}
		locale := dashboardDocLocale(t, lang)
		var wants []string
		for _, id := range dashboardDocSections {
			level := "### "
			if id == "overview" {
				level = "## "
			}
			wants = append(wants, "\n"+level+dashboardDocLabel(t, locale, lang, "dashboard.section."+id+".title")+"\n", "`#"+id+"`")
		}
		for _, group := range dashboardDocGroups {
			wants = append(wants, "\n## "+dashboardDocLabel(t, locale, lang, "dashboard.nav.group."+group)+"\n")
		}
		for _, key := range []string{
			"dashboard.overview.attention_title", "dashboard.overview.customize", "dashboard.overview.show_all",
			"dashboard.section.refresh", "dashboard.overview.not_available", "dashboard.overview.pill_unavailable",
			"dashboard.budget_disabled",
		} {
			wants = append(wants, dashboardDocLabel(t, locale, lang, key))
		}
		for _, code := range codes {
			wants = append(wants, "`"+code+"`")
		}
		wants = append(wants,
			"GET /api/dashboard/summary", "(21-api-reference.md#dashboard-api)",
			"`aurago-dash-section`", "`aurago-dash-tab`", "`aurago-dash-hidden-tiles`", "`dash-disclosures`", "`dash-collapse-<card-id>`",
			"`#agent`", "`#cronjobs`", "900 px",
		)
		requireAll(t, name, doc, wants)
	}
	if en, de := strings.Count(docs["en"], "\n"), strings.Count(docs["de"], "\n"); en != de {
		t.Errorf("chapter 13 en/de must stay line-for-line mirrors: en has %d lines, de %d", en, de)
	}
	if en, de := strings.Join(markdownHeadingLines(docs["en"]), " "), strings.Join(markdownHeadingLines(docs["de"]), " "); en != de {
		t.Errorf("chapter 13 en/de headings differ (line:level):\nen: %s\nde: %s", en, de)
	}
}

func TestDashboardDocsAPIReference(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ lang, issuesHeading, adminLine, staleDate string }{
		{"en", "### Operational Issues API", "These endpoints are administrator-only.", "> Updated: 3 October 2026."},
		{"de", "### Operational-Issues-API", "Diese Endpunkte benötigen Administratorrechte.", "> Stand: 3. Oktober 2026."},
	} {
		name := "manual/" + c.lang + "/21-api-reference.md"
		doc := lfDoc(t, "documentation/manuals/"+c.lang+"/21-api-reference.md")
		dashboard := markdownBlock(t, name, doc, "### Dashboard API\n", "\n### Container API")
		requireAll(t, name+" (Dashboard API)", dashboard, []string{
			"GET /api/dashboard/summary",
			`"generated_at"`, `"overall"`, `"attention_more"`, `"badges"`, `"kpis"`, `"recent_activity"`, `"unavailable"`, `"sampled_at"`,
			`"usage_percent"`, `"used_percent"`, `"bytes_sent"`, `"sse_clients"`, `"host_uptime_seconds"`,
			"/api/dashboard/cronjobs", "/api/dashboard/audit", "/api/dashboard/mission-history", "/api/dashboard/compression",
			"/api/dashboard/optimization", "/api/dashboard/memory/curation", "/api/dashboard/memory/hygiene",
			"/api/dashboard/memory/reflection/run", "DELETE /api/dashboard/core-memory/mutate",
		})
		for _, gone := range []string{`"rx_mbps"`, `"action": "add"`} {
			if strings.Contains(dashboard, gone) {
				t.Errorf("%s: the Dashboard API block still shows %s", name, gone)
			}
		}
		issues := markdownBlock(t, name, doc, c.issuesHeading+"\n", "\n---\n")
		requireAll(t, name+" (Operational Issues API)", issues, []string{c.adminLine})
		sse := markdownBlock(t, name, doc, "### SSE Events\n", c.issuesHeading)
		if strings.Contains(sse, "event: system_metrics") {
			t.Errorf("%s: the SSE example must use the data envelope with type and payload", name)
		}
		if strings.Contains(doc, c.staleDate) {
			t.Errorf("%s: refresh the %q line", name, c.staleDate)
		}
	}
}

func TestDashboardDocsReadmeScreenshot(t *testing.T) {
	t.Parallel()
	readme := lfDoc(t, "README.md")
	requireAll(t, "README.md", readme, []string{
		"<summary>Dashboard screenshot</summary>",
		"](documentation/screenshots/dashboard.png)",
		"(documentation/manuals/en/13-dashboard.md)",
	})
	if strings.Contains(readme, "A fresh screenshot is on the to-do list") {
		t.Error("README.md still carries the dashboard screenshot to-do note")
	}
	if chat := markdownBlock(t, "README.md", readme, "<summary>Chat screenshot</summary>", "</details>"); strings.Contains(chat, "dashboard") {
		t.Error("README.md: the dashboard must not be mentioned inside the Chat screenshot block")
	}
	f, err := os.Open(repoPath("documentation", "screenshots", "dashboard.png"))
	if err != nil {
		t.Fatalf("open the dashboard screenshot: %v", err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatalf("documentation/screenshots/dashboard.png is no PNG: %v", err)
	}
	if cfg.Width != 1440 {
		t.Errorf("documentation/screenshots/dashboard.png is %d px wide, want 1440", cfg.Width)
	}
}
