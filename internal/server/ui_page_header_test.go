package server

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

// uiPageHeaderCase describes one page template and what the shared app header
// contract demands from it: workspace pages call {{template "app-header" .}},
// entry pages {{template "brand-lockup" .}}. A new standalone page belongs here.
type uiPageHeaderCase struct {
	file  string
	page  string // page key passed to uiTemplateData, == data-workspace-page
	entry bool   // login/setup/404: brand lockup only, no app header
	// h1Keyed pages carry an H1 (pw-page-title, or the dashboard H1) whose
	// data-i18n must equal common.page_<id>.
	h1Keyed bool
}

var uiPageHeaderCases = []uiPageHeaderCase{
	{file: "config.html", page: "config"},
	{file: "dashboard.html", page: "dashboard", h1Keyed: true},
	{file: "knowledge.html", page: "knowledge", h1Keyed: true},
	{file: "skills.html", page: "skills", h1Keyed: true},
	{file: "missions_v2.html", page: "missions", h1Keyed: true},
	{file: "plans.html", page: "plans", h1Keyed: true},
	{file: "containers.html", page: "containers", h1Keyed: true},
	{file: "invasion_control.html", page: "invasion", h1Keyed: true},
	{file: "truenas.html", page: "truenas"},
	{file: "media.html", page: "media", h1Keyed: true},
	{file: "cheatsheets.html", page: "cheatsheets", h1Keyed: true},
	{file: "login.html", page: "login", entry: true},
	{file: "setup.html", page: "setup", entry: true},
	{file: "404.html", page: "404", entry: true},
}

var uiLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

var (
	uiH1Pattern       = regexp.MustCompile(`(?s)<h1\b([^>]*)>`)
	uiDataI18NPattern = regexp.MustCompile(`data-i18n="([^"]+)"`)
	uiEmojiPattern    = regexp.MustCompile("[\U0001F300-\U0001FAFF☀-➿]")
)

func renderUIPageForTest(t *testing.T, c uiPageHeaderCase) string {
	t.Helper()
	ui := os.DirFS(filepath.Join("..", "..", "ui"))
	var tmpl *template.Template
	var err error
	if c.file == "dashboard.html" {
		tmpl, err = parseDashboardTemplate(ui)
	} else {
		tmpl, err = parseUIPage(ui, c.file)
	}
	if err != nil {
		t.Fatalf("parse %s: %v", c.file, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, uiTemplateData("en", c.page)); err != nil {
		t.Fatalf("execute %s: %v", c.file, err)
	}
	return out.String()
}

func TestUIPagesRenderWithSharedHeaderPartial(t *testing.T) {
	t.Parallel()
	for _, c := range uiPageHeaderCases {
		t.Run(c.file, func(t *testing.T) {
			t.Parallel()
			page := renderUIPageForTest(t, c)
			if strings.Contains(page, "<no value>") {
				t.Error("rendered page contains <no value>")
			}
			if strings.Contains(page, "{{template") || strings.Contains(page, "{{define") {
				t.Error("rendered page still contains template actions")
			}
			if c.entry && (strings.Contains(page, "data-app-header") || strings.Contains(page, "data-pw-density-toggle") || strings.Contains(page, `id="radialMenuAnchor"`)) {
				t.Error("entry pages must not carry the app header, density toggle or radial anchor")
			}
			key := "common.page_" + c.page
			if c.entry {
				if got := strings.Count(page, `class="aurago-brand-icon"`); got != 1 {
					t.Errorf("brand-lockup robot icons = %d, want 1", got)
				}
				if !strings.Contains(page, "logo-wordmark-accent") {
					t.Error("entry page lacks the two-tone wordmark")
				}
				return
			}
			for marker, want := range map[string]int{
				"data-app-header":           1,
				`id="radialMenuAnchor"`:     1,
				"data-pw-density-toggle ":   1,
				"data-ui-lang-slot":         1,
				`class="aurago-brand-icon"`: 1,
			} {
				if got := strings.Count(page, marker); got != want {
					t.Errorf("%s occurs %d times, want %d", marker, got, want)
				}
			}
			if !strings.Contains(page, `class="logo-subtitle" data-i18n="`+key+`"`) {
				t.Errorf("subtitle must use %s", key)
			}
			if !strings.Contains(page, `data-i18n-page-section="`+key+`"`) {
				t.Errorf("<html> must carry data-i18n-page-section=%q", key)
			}
			if strings.Contains(page, "data-i18n-page-title=") {
				t.Error("data-i18n-page-title must be replaced by data-i18n-page-section")
			}
			if start := strings.Index(page, `<span class="logo-icon">`); start < 0 {
				t.Error("missing brand-lockup logo-icon")
			} else {
				logo := page[start:]
				logo = logo[:strings.Index(logo, "</span>")]
				if uiEmojiPattern.MatchString(logo) {
					t.Error("logo must be the robot image, not an emoji")
				}
			}
			if c.h1Keyed {
				found := false
				for _, m := range uiH1Pattern.FindAllStringSubmatch(page, -1) {
					if !strings.Contains(m[1], "pw-page-title") && c.page != "dashboard" {
						continue
					}
					found = true
					if k := uiDataI18NPattern.FindStringSubmatch(m[1]); k == nil || k[1] != key {
						t.Errorf("H1 attributes %q: data-i18n must be %s", m[1], key)
					}
				}
				if !found {
					t.Error("page has no keyed H1")
				}
			}
		})
	}
}

// TestUIAppHeaderSlotsCanBeOverridden: the slot blocks are empty by default and
// a page defining the same block replaces the content, independent of parse order.
func TestUIAppHeaderSlotsCanBeOverridden(t *testing.T) {
	t.Parallel()
	partial, err := os.ReadFile(filepath.Join("..", "..", "ui", "partials", "app-header.html"))
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"partials/app-header.html": &fstest.MapFile{Data: partial},
		"page.html":                &fstest.MapFile{Data: []byte(`{{define "app-header-actions"}}<b id="slot-actions"></b>{{end}}{{template "app-header" .}}`)},
		"plain.html":               &fstest.MapFile{Data: []byte(`{{template "app-header" .}}`)},
	}
	render := func(file string) string {
		tmpl, err := parseUIPage(fsys, file)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, uiTemplateData("de", "plans")); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := render("page.html"); !strings.Contains(got, `id="slot-actions"`) || !strings.Contains(got, `data-i18n="common.page_plans"`) || !strings.Contains(got, `<span class="ui-lang-code">de</span>`) {
		t.Errorf("slot override or page parameters missing:\n%s", got)
	}
	if got := render("plain.html"); strings.Contains(got, "slot-actions") || strings.Count(got, "data-app-header") != 1 {
		t.Errorf("default slots must render empty:\n%s", got)
	}
}

// TestUIPageTitleKeysExistInAllLocales: common.page_<id> and common.ui_language are merged into
// every page, so each of the 16 common bundles must define them.
func TestUIPageTitleKeysExistInAllLocales(t *testing.T) {
	t.Parallel()
	keys := []string{"common.ui_language"}
	for _, c := range uiPageHeaderCases {
		if !c.entry {
			keys = append(keys, "common.page_"+c.page)
		}
	}
	for _, lang := range uiLocales {
		raw, err := os.ReadFile(filepath.Join("..", "..", "ui", "lang", "common", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("ui/lang/common/%s.json lacks %s", lang, key)
			}
		}
	}
}
