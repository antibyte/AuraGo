package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestToolPermissionHelpCoversRenderedFields(t *testing.T) {
	source := string(mustReadUIFile(t, "js/config/main.js"))
	match := regexp.MustCompile(`(?s)const TOOLS_SKIP_KEYS = new Set\(\[(.*?)\]\);`).FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatal("cannot find Tool Permissions section filter")
	}
	skip := map[string]bool{}
	for _, key := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(match[1], -1) {
		skip[key[1]] = true
	}
	var paths []string
	var visit func(reflect.Type, string)
	visit = func(typ reflect.Type, prefix string) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			key := strings.Split(field.Tag.Get("yaml"), ",")[0]
			if key == "" || key == "-" || strings.Split(field.Tag.Get("json"), ",")[0] == "-" || (prefix == "tools" && skip[key]) {
				continue
			}
			path := prefix + "." + key
			if field.Type.Kind() == reflect.Struct {
				visit(field.Type, path)
			} else {
				paths = append(paths, "help."+path)
			}
		}
	}
	visit(reflect.TypeOf(config.Config{}.Tools), "tools")
	if len(paths) == 0 {
		t.Fatal("no Tool Permissions fields found")
	}
	files, err := filepath.Glob(filepath.Join("lang", "help", "*.json"))
	if err != nil || len(files) != 16 {
		t.Fatalf("help locales = %d, want 16: %v", len(files), err)
	}
	var english map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/help/en.json"), &english); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			var values map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, file), &values); err != nil {
				t.Fatal(err)
			}
			for _, key := range paths {
				if strings.TrimSpace(values[key]) == "" {
					t.Errorf("missing description for %s", key)
				} else if filepath.Base(file) != "en.json" && values[key] == english[key] {
					t.Errorf("untranslated description for %s", key)
				}
			}
		})
	}
	t.Logf("checked %d Tool Permissions fields in all 16 locales", len(paths))
}

func TestToolPermissionHelpIsVisibleBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, locale := range []string{"de", "en"} {
		t.Run(locale, func(t *testing.T) {
			page := browser.MustPage(configRefreshFixtureOrigin(t, locale, true) + "/config#tools")
			defer page.MustClose()
			waitForJSBool(t, page, `() => !!document.querySelector('#content [data-path="tools.memory.enabled"]')`)
			for _, width := range []int{390, 1440} {
				page.MustSetViewport(width, 900, 1, width == 390)
				for _, theme := range []string{"dark", "light"} {
					page.MustEval(`theme => { document.documentElement.dataset.theme=theme; document.body.dataset.theme=theme; }`, theme)
					missing := page.MustEval(`() => [...document.querySelectorAll('#content [data-path^="tools."]')].filter(control => {
					 const help = control.closest('.field-group')?.querySelector('.field-help');
					 return !help?.textContent.trim() || getComputedStyle(help).display === 'none';
					}).map(control => control.dataset.path)`)
					if len(missing.Arr()) != 0 {
						t.Errorf("%dpx %s: missing visible help: %s", width, theme, missing.String())
					}
					if page.MustEval(`() => { const el=document.getElementById('content'); return el.scrollWidth>el.clientWidth+1; }`).Bool() {
						t.Errorf("%dpx %s: Tool Permissions overflows horizontally", width, theme)
					}
				}
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "tool-permissions-help-"+locale+".png"), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
