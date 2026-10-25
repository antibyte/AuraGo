package agent

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
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
