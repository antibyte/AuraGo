package ui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var chibiSenseiKeyLiteral = regexp.MustCompile(`'(chibi_sensei\.[A-Za-z0-9_.]+)'`)

func chibiSenseiAppSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "chibi-sensei", "*.js"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no chibi-sensei app modules found: %v", err)
	}
	sources := map[string]string{}
	for _, path := range entries {
		sources[filepath.ToSlash(path)] = readDesktopAssetText(t, filepath.ToSlash(path))
	}
	return sources
}

// The static guard: untrusted text (rows, names, documents, server messages) never reaches innerHTML, insertAdjacentHTML or a
// template string; no dynamic code, no browser dialogs, no browser storage; every module keeps to the 1100-line budget and its
// shape; only the API module talks to the network.
func TestChibiSenseiSourcesAreSafeAndSmall(t *testing.T) {
	t.Parallel()
	forbidden := map[string]*regexp.Regexp{
		"innerHTML":          regexp.MustCompile(`\binnerHTML\b`),
		"outerHTML":          regexp.MustCompile(`\bouterHTML\b`),
		"insertAdjacentHTML": regexp.MustCompile(`\binsertAdjacentHTML\b`),
		"document.write":     regexp.MustCompile(`\bdocument\s*\.\s*write(ln)?\s*\(`),
		"eval":               regexp.MustCompile(`\beval\s*\(`),
		"new Function":       regexp.MustCompile(`\bnew\s+Function\b`),
		"alert":              regexp.MustCompile(`(^|[^.\w])(window\s*\.\s*)?alert\s*\(`),
		"confirm":            regexp.MustCompile(`(^|[^.\w])(window\s*\.\s*)?confirm\s*\(`),
		"prompt":             regexp.MustCompile(`(^|[^.\w])(window\s*\.\s*)?prompt\s*\(`),
		"template string":    regexp.MustCompile("`"),
		"localStorage":       regexp.MustCompile(`\blocalStorage\b|\bsessionStorage\b`),
		"cookie":             regexp.MustCompile(`document\s*\.\s*cookie`),
	}
	network := map[string]*regexp.Regexp{
		"fetch":       regexp.MustCompile(`\bfetch\s*\(`),
		"EventSource": regexp.MustCompile(`new\s+(window\.)?EventSource`),
	}
	sources := chibiSenseiAppSources(t)
	if len(sources) != 12 {
		t.Fatalf("found %d chibi-sensei scripts, want exactly 12", len(sources))
	}
	const apiPath = "js/desktop/apps/chibi-sensei/chibi-sensei-api.js"
	for path, source := range sources {
		code := storymakerCode(source)
		for label, pattern := range forbidden {
			if pattern.MatchString(code) {
				t.Errorf("%s uses %s", path, label)
			}
		}
		for label, pattern := range network {
			if path != apiPath && pattern.MatchString(code) {
				t.Errorf("%s uses %s: only chibi-sensei-api.js may touch the network", path, label)
			}
		}
		if lines := strings.Count(source, "\n"); lines >= 1100 {
			t.Errorf("%s has %d lines, the desktop budget is 1100", path, lines)
		}
		if !strings.Contains(source, "'use strict'") || !strings.HasPrefix(source, "//") {
			t.Errorf("%s must start with a purpose comment and run in strict mode", path)
		}
		if !strings.Contains(source, "ChibiSenseiParts") && !strings.Contains(source, "ChibiSenseiApp") {
			t.Errorf("%s is not part of the ChibiSenseiParts namespace", path)
		}
	}
}

// Nothing loads from the Internet, no script or stylesheet names an external host, the SVG namespace is the only URL, and the app
// keeps off the class names that cookie-banner and ad filters hide.
func TestChibiSenseiAppLoadsNothingExternalAndAvoidsFilteredClasses(t *testing.T) {
	t.Parallel()
	external := regexp.MustCompile(`(?i)https?://[a-z0-9.-]+\.[a-z]{2,}`)
	filtered := regexp.MustCompile(`(?i)^\.?(cc-|cookie|consent|gdpr|adsbox|ad-banner|banner-ad|sponsor)`)
	allowed := []string{"http://www.w3.org/2000/svg"}
	files := map[string]string{"css/desktop-app-chibi-sensei.css": readDesktopAssetText(t, "css/desktop-app-chibi-sensei.css")}
	for path, source := range chibiSenseiAppSources(t) {
		files[path] = source
	}
	for path, source := range files {
		text := source
		for _, ok := range allowed {
			text = strings.ReplaceAll(text, ok, "")
		}
		if match := external.FindString(text); match != "" {
			t.Errorf("%s names an external address: %s", path, match)
		}
		if strings.HasSuffix(path, ".css") {
			for _, class := range regexp.MustCompile(`\.[a-z][\w-]*`).FindAllString(storymakerCode(text), -1) {
				if filtered.MatchString(class) {
					t.Errorf("%s uses the filtered class %s", path, class)
				}
			}
			continue
		}
		for _, c := range regexp.MustCompile(`class: '([^']*)'`).FindAllStringSubmatch(text, -1) {
			for _, name := range strings.Fields(c[1]) {
				if !strings.HasPrefix(name, "csn-") {
					t.Errorf("%s uses the class %q: every class of the app starts with csn-", path, name)
				}
				if filtered.MatchString(name) {
					t.Errorf("%s uses the filtered class %q", path, name)
				}
			}
		}
	}
}

// Every key the app names exists, and so do the keys it builds from a list (error codes, check ids).
func TestChibiSenseiAppUsesOnlyKeysItHas(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "chibi_sensei", "en")
	used := 0
	sources := chibiSenseiAppSources(t)
	for path, source := range sources {
		for _, match := range chibiSenseiKeyLiteral.FindAllStringSubmatch(source, -1) {
			key := match[1]
			if strings.HasSuffix(key, ".") || strings.HasSuffix(key, "_") {
				continue // the prefix of a family built from a list, checked below
			}
			used++
			if _, ok := english[key]; !ok {
				t.Errorf("%s uses %s, which lang/chibi_sensei/en.json lacks", path, key)
			}
		}
	}
	if used < 200 {
		t.Fatalf("found only %d key literals in the app modules, the pattern no longer matches", used)
	}
	core := sources["js/desktop/apps/chibi-sensei/chibi-sensei-core.js"]
	families := map[string][]string{
		"chibi_sensei.error.": storymakerList(t, core, "ERROR_CODES"),
		"chibi_sensei.check.": storymakerList(t, core, "CHECK_IDS"),
	}
	for prefix, names := range families {
		if len(names) == 0 {
			t.Fatalf("no names found for %s", prefix)
		}
		for _, name := range names {
			if _, ok := english[prefix+name]; !ok {
				t.Errorf("lang/chibi_sensei/en.json lacks %s%s", prefix, name)
			}
		}
	}
	for _, key := range []string{
		"chibi_sensei.check.unknown", "chibi_sensei.error.generic", "chibi_sensei.flag.other",
		"chibi_sensei.suggest.generic", "chibi_sensei.import.issue_generic",
	} {
		if _, ok := english[key]; !ok {
			t.Errorf("lang/chibi_sensei/en.json lacks the fallback %s", key)
		}
	}
}

// The loader lists exactly the chibi-sensei scripts that exist, the core first and the composition root last.
func TestChibiSenseiLoaderListsEveryScript(t *testing.T) {
	t.Parallel()
	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        'chibi-sensei': {")
	if start < 0 {
		t.Fatal("module-loader.js has no chibi-sensei entry")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	var order []string
	listed := map[string]bool{}
	for _, match := range regexp.MustCompile(`/js/desktop/apps/chibi-sensei/(chibi-sensei[\w-]*\.js)'`).FindAllStringSubmatch(entry, -1) {
		order = append(order, match[1])
		listed[match[1]] = true
	}
	onDisk, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "chibi-sensei", "*.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range onDisk {
		name := filepath.Base(path)
		if !listed[name] {
			t.Errorf("%s exists but the loader does not load it", name)
		}
		delete(listed, name)
	}
	for name := range listed {
		t.Errorf("the loader lists %s, which does not exist", name)
	}
	if len(order) == 0 || order[0] != "chibi-sensei-core.js" || order[len(order)-1] != "chibi-sensei.js" {
		t.Errorf("the loader order is %v: the core must come first and chibi-sensei.js last", order)
	}
}
