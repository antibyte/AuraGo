package ui

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var storymakerKeyLiteral = regexp.MustCompile(`'(storymaker\.[A-Za-z0-9_.]+)'`)

func storymakerAppSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "storymaker*.js"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no storymaker app modules found: %v", err)
	}
	sources := map[string]string{}
	for _, path := range entries {
		sources[filepath.ToSlash(path)] = readDesktopAssetText(t, filepath.ToSlash(path))
	}
	return sources
}

func storymakerPlayerRealmSources(t *testing.T) map[string]string {
	t.Helper()
	sources := storymakerAppSources(t)
	entries, err := fs.ReadDir(Content, "js/storymaker")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") || strings.HasSuffix(entry.Name(), ".bundle.js") {
			continue
		}
		sources["js/storymaker/"+entry.Name()] = readDesktopAssetText(t, "js/storymaker/"+entry.Name())
	}
	return sources
}

// The spec's static guard: untrusted story text never reaches innerHTML, insertAdjacentHTML or a template string; no dynamic code, no
// browser dialogs; every app module keeps to the 1100-line budget and its shape.
func TestStorymakerSourcesAreSafeAndSmall(t *testing.T) {
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
	sources := storymakerPlayerRealmSources(t)
	if len(sources) < 30 {
		t.Fatalf("only %d storymaker scripts found", len(sources))
	}
	for path, source := range sources {
		code := storymakerCode(source)
		for label, pattern := range forbidden {
			if label == "template string" && strings.HasPrefix(path, "js/storymaker/") {
				continue // the player realm has its own Node guard for template strings
			}
			if pattern.MatchString(code) {
				t.Errorf("%s uses %s", path, label)
			}
		}
		if strings.HasPrefix(path, "js/desktop/apps/") {
			lines := strings.Count(source, "\n")
			if lines >= 1100 {
				t.Errorf("%s has %d lines, the desktop budget is 1100", path, lines)
			}
			if !strings.Contains(source, "'use strict'") || !strings.HasPrefix(source, "//") {
				t.Errorf("%s must start with a purpose comment and run in strict mode", path)
			}
			if !strings.Contains(source, "StorymakerParts") && !strings.Contains(source, "StorymakerApp") {
				t.Errorf("%s is not part of the StorymakerParts namespace", path)
			}
		}
	}
}

// Nothing loads from the Internet, no stylesheet or script names an external host, and the app keeps off the class names that
// cookie-banner and ad filters hide.
func TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses(t *testing.T) {
	t.Parallel()
	external := regexp.MustCompile(`(?i)https?://[a-z0-9.-]+\.[a-z]{2,}`)
	filtered := regexp.MustCompile(`(?i)^\.?(cc-|cookie|consent|gdpr|adsbox|ad-banner|banner-ad|sponsor)`)
	allowed := []string{"http://www.w3.org/2000/svg"}
	files := map[string]string{"css/desktop-app-storymaker.css": readDesktopAssetText(t, "css/desktop-app-storymaker.css")}
	for path, source := range storymakerAppSources(t) {
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
				if !strings.HasPrefix(name, "stm-") {
					t.Errorf("%s uses the class %q: every class of the app starts with stm-", path, name)
				}
				if filtered.MatchString(name) {
					t.Errorf("%s uses the filtered class %q", path, name)
				}
			}
		}
	}
}

// Every key the app names exists, and so do the keys it builds from a list (issue codes, job errors, export phases, browser sources).
func TestStorymakerAppUsesOnlyKeysItHas(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "storymaker", "en")
	used := 0
	for path, source := range storymakerAppSources(t) {
		for _, match := range storymakerKeyLiteral.FindAllStringSubmatch(source, -1) {
			key := match[1]
			if strings.HasSuffix(key, ".") || strings.HasSuffix(key, "_") {
				continue // the prefix of a family built from a list, checked below
			}
			used++
			if _, ok := english[key]; !ok {
				t.Errorf("%s uses %s, which lang/storymaker/en.json lacks", path, key)
			}
			if strings.HasPrefix(key, "storymaker.catalog.") {
				t.Errorf("%s names the catalog key %s: those belong to the catalog directory", path, key)
			}
		}
	}
	if used < 150 {
		t.Fatalf("found only %d key literals in the app modules, the pattern no longer matches", used)
	}
	core := storymakerAppSources(t)["js/desktop/apps/storymaker-core.js"]
	exporter := storymakerAppSources(t)["js/desktop/apps/storymaker-export.js"]
	families := map[string][]string{
		"storymaker.issue.":         storymakerList(t, core, "ISSUE_CODES"),
		"storymaker.job_error.":     storymakerList(t, core, "JOB_ERROR_CODES"),
		"storymaker.export.phase_":  storymakerList(t, exporter, "PHASES"),
		"storymaker.export.source_": {"config", "system", "pinned"},
	}
	// The list of issue codes is the contract's: twenty-seven codes, no more, no less.
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified line_unread dead_stretch first_frame_empty text_too_small fade_between_worlds entry_monotony transition_variety cue_late")
	if strings.Join(families["storymaker.issue."], " ") != strings.Join(want, " ") {
		t.Errorf("ISSUE_CODES = %v, want the 27 codes of the contract", families["storymaker.issue."])
	}
	for prefix, names := range families {
		if len(names) == 0 {
			t.Fatalf("no names found for %s", prefix)
		}
		for _, name := range names {
			if _, ok := english[prefix+name]; !ok {
				t.Errorf("lang/storymaker/en.json lacks %s%s", prefix, name)
			}
		}
	}
	for _, key := range []string{"storymaker.issue.unknown", "storymaker.job_error.unknown", "storymaker.export.source_unknown"} {
		if _, ok := english[key]; !ok {
			t.Errorf("lang/storymaker/en.json lacks the fallback %s", key)
		}
	}
}

// storymakerList reads `const NAME = ['a', 'b', ...];` from a source file.
func storymakerList(t *testing.T, source, name string) []string {
	t.Helper()
	match := regexp.MustCompile(`const ` + name + ` = \[([^\]]*)\]`).FindStringSubmatch(source)
	if match == nil {
		t.Fatalf("no list %s in the source", name)
	}
	var out []string
	for _, item := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(match[1], -1) {
		out = append(out, item[1])
	}
	return out
}

// The loader lists exactly the storymaker scripts that exist: nothing on disk the loader skips, nothing listed that is missing.
func TestStorymakerLoaderListsEveryScript(t *testing.T) {
	t.Parallel()
	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        storymaker: {")
	if start < 0 {
		t.Fatal("module-loader.js has no storymaker entry")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	listed := map[string]bool{}
	for _, match := range regexp.MustCompile(`/js/desktop/apps/(storymaker[\w-]*\.js)'`).FindAllStringSubmatch(entry, -1) {
		listed[match[1]] = true
	}
	onDisk, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "storymaker*.js"))
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
}
