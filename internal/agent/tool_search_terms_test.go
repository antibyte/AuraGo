package agent

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

func TestToolSearchTermsLoad(t *testing.T) {
	terms, err := loadToolSearchTerms()
	if err != nil {
		t.Fatalf("embedded tool search terms: %v", err)
	}
	known := make(map[string]bool)
	for _, name := range builtinToolNames(allBuiltinToolFeatureFlags()) {
		known[name] = true
	}
	for _, entries := range toolCategoryDef {
		for _, entry := range entries {
			known[entry.Name] = true
		}
	}
	for name, phrases := range terms {
		if !known[name] {
			t.Errorf("tool_search_terms lists %q, which is not a known tool", name)
		}
		if len(phrases) == 0 {
			t.Errorf("tool_search_terms lists %q without a phrase", name)
		}
	}
	if got := toolSearchTermsFor("manage_notes"); !slices.Contains(got, "Notiz speichern") || !slices.Contains(got, "save note") {
		t.Fatalf("manage_notes terms = %q, want the German and English phrases", got)
	}
}

func TestParseToolSearchTermFiles(t *testing.T) {
	file := func(content string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(content)} }
	t.Run("merges files and keeps English first", func(t *testing.T) {
		terms, err := parseToolSearchTermFiles(fstest.MapFS{
			"terms/a.yaml":   file("# comment\nalpha:\n  en: [\"one\", \" two \"]\n  de: [\"Eins\", \"\"]\n"),
			"terms/b.yaml":   file("# only a comment\n"),
			"terms/c.yaml":   file("beta:\n  de: [\"Drei\"]\n"),
			"terms/skip.txt": file("not: yaml: at all"),
		}, "terms")
		if err != nil {
			t.Fatal(err)
		}
		if got := terms["alpha"]; !slices.Equal(got, []string{"one", "two", "Eins"}) {
			t.Fatalf("alpha = %q", got)
		}
		if got := terms["beta"]; !slices.Equal(got, []string{"Drei"}) || len(terms) != 2 {
			t.Fatalf("terms = %v", terms)
		}
	})
	for name, files := range map[string]fstest.MapFS{
		"duplicate across files": {"terms/a.yaml": file("alpha:\n  en: [\"x\"]\n"), "terms/b.yaml": file("alpha:\n  de: [\"y\"]\n")},
		"duplicate within file":  {"terms/a.yaml": file("alpha:\n  en: [\"x\"]\nalpha:\n  en: [\"y\"]\n")},
		"unknown language":       {"terms/a.yaml": file("alpha:\n  fr: [\"x\"]\n")},
		"malformed yaml":         {"terms/a.yaml": file("alpha: [\n")},
	} {
		t.Run(name, func(t *testing.T) {
			if terms, err := parseToolSearchTermFiles(files, "terms"); err == nil || strings.TrimSpace(err.Error()) == "" {
				t.Fatalf("want an error, got terms=%v err=%v", terms, err)
			}
		})
	}
}

// TestToolSearchTermsCoverEveryBuiltinTool holds the term files to their
// format: every tool of the category table and every native schema lists 3 to
// 12 English and 3 to 12 German phrases, and the files list no other tool.
// Phrases shared between tools are legitimate ("restart" fits several), so they
// are only logged.
func TestToolSearchTermsCoverEveryBuiltinTool(t *testing.T) {
	const (
		minPerLanguage = 3
		maxPerLanguage = 12
		maxTermRunes   = 48
	)
	known := make(map[string]bool)
	for _, name := range builtinToolNames(allBuiltinToolFeatureFlags()) {
		known[name] = true
	}
	for _, entries := range toolCategoryDef {
		for _, entry := range entries {
			known[entry.Name] = true
		}
	}

	files, err := fs.ReadDir(toolSearchTermFiles, toolSearchTermsDir)
	if err != nil {
		t.Fatalf("read embedded tool search terms: %v", err)
	}
	lists := make(map[string]toolSearchTermLists)
	for _, file := range files {
		if file.IsDir() || path.Ext(file.Name()) != ".yaml" {
			continue
		}
		raw, err := toolSearchTermFiles.Open(path.Join(toolSearchTermsDir, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		decoder := yaml.NewDecoder(raw)
		decoder.KnownFields(true)
		var fileLists map[string]toolSearchTermLists
		err = decoder.Decode(&fileLists)
		raw.Close()
		if err != nil {
			t.Fatalf("%s: %v", file.Name(), err)
		}
		for name, list := range fileLists {
			if _, dup := lists[name]; dup {
				t.Errorf("%s lists %q, which another file already lists", file.Name(), name)
			}
			lists[name] = list
		}
	}

	owners := make(map[string][]string)
	for name := range known {
		if _, ok := lists[name]; !ok {
			t.Errorf("tool %q has no search terms", name)
		}
	}
	for name, list := range lists {
		if !known[name] {
			t.Errorf("search terms list %q, which is neither in toolCategoryDef nor a native tool schema", name)
			continue
		}
		seen := make(map[string]bool)
		for lang, phrases := range map[string][]string{"en": list.EN, "de": list.DE} {
			if len(phrases) < minPerLanguage || len(phrases) > maxPerLanguage {
				t.Errorf("%s: %d %s phrases, want %d to %d", name, len(phrases), lang, minPerLanguage, maxPerLanguage)
			}
			for _, phrase := range phrases {
				switch {
				case strings.TrimSpace(phrase) == "":
					t.Errorf("%s: empty %s phrase", name, lang)
					continue
				case phrase != strings.TrimSpace(phrase):
					t.Errorf("%s: %s phrase %q has surrounding whitespace", name, lang, phrase)
				case utf8.RuneCountInString(phrase) > maxTermRunes:
					t.Errorf("%s: %s phrase %q is longer than %d characters", name, lang, phrase, maxTermRunes)
				}
				key := strings.ToLower(strings.TrimSpace(phrase))
				if seen[key] {
					t.Errorf("%s: phrase %q appears more than once", name, phrase)
					continue
				}
				seen[key] = true
				owners[key] = append(owners[key], name)
			}
		}
	}

	shared := make([]string, 0)
	for phrase, tools := range owners {
		if len(tools) > 1 {
			sort.Strings(tools)
			shared = append(shared, fmt.Sprintf("%q: %s", phrase, strings.Join(tools, ", ")))
		}
	}
	sort.Strings(shared)
	for _, line := range shared {
		t.Logf("phrase shared between tools: %s", line)
	}
}
