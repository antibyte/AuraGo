package ui

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// uiPageKeys maps page template files to the page key the server passes to
// uiTemplateData (equal to data-workspace-page for workspace pages).
var uiPageKeys = map[string]string{
	"missions_v2.html":      "missions",
	"invasion_control.html": "invasion",
	"404.html":              "404",
	"setup.html":            "setup",
	"login.html":            "login",
	"config.html":           "config",
	"dashboard.html":        "dashboard",
	"knowledge.html":        "knowledge",
	"skills.html":           "skills",
	"plans.html":            "plans",
	"containers.html":       "containers",
	"truenas.html":          "truenas",
	"media.html":            "media",
	"cheatsheets.html":      "cheatsheets",
}

// renderUIPage executes a page template the way the server does: parsed
// together with partials/app-header.html (and, for the dashboard, its section
// partials) and fed with Lang, BuildVersion, Page and the template data JSON.
func renderUIPage(t *testing.T, file string) string {
	t.Helper()
	return renderUIPageWith(t, file, uiPageOptions{})
}

// uiPageOptions overrides the template data renderUIPageWith feeds a page.
// Zero values keep the renderUIPage defaults.
type uiPageOptions struct {
	Lang             string
	BuildVersion     string
	TemplateDataJSON string // raw JSON for the template data script
	I18N             map[string]string
}

// renderUIPageWith is renderUIPage with custom Lang, BuildVersion, template
// data JSON and I18N strings, for tests that feed a page its own fixtures.
func renderUIPageWith(t *testing.T, file string, opts uiPageOptions) string {
	t.Helper()
	files := []string{file}
	if file == "dashboard.html" {
		files = append(files, dashboardSectionPartialFiles(t)...)
	}
	files = append(files, "partials/app-header.html")
	tmpl, err := template.ParseFS(Content, files...)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	page, ok := uiPageKeys[file]
	if !ok {
		page = strings.TrimSuffix(file, ".html")
	}
	lang, build, dataJSON, i18n := opts.Lang, opts.BuildVersion, opts.TemplateDataJSON, opts.I18N
	if lang == "" {
		lang = "en"
	}
	if build == "" {
		build = "test"
	}
	if dataJSON == "" {
		dataJSON = `{"systemLang":"en","i18n":{}}`
	}
	if i18n == nil {
		i18n = map[string]string{}
	}
	data := map[string]any{
		"Lang":             lang,
		"BuildVersion":     build,
		"Page":             page,
		"I18N":             i18n,
		"TemplateDataJSON": template.JS(dataJSON),
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute %s: %v", file, err)
	}
	return out.String()
}

func TestRenderUIPageIncludesSharedPartial(t *testing.T) {
	t.Parallel()

	raw := mustReadUIFile(t, "partials/app-header.html")
	for _, name := range []string{"app-header", "brand-lockup", "app-header-lead", "app-header-status", "app-header-actions"} {
		if !bytes.Contains(raw, []byte(`{{define "`+name+`"}}`)) {
			t.Errorf("partials/app-header.html does not define %q", name)
		}
	}
	if page := renderUIPage(t, "knowledge.html"); !strings.Contains(page, `data-workspace-page="knowledge"`) {
		t.Error("knowledge.html rendered without its page body")
	}
}
