package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Inline on*="..." handlers must not put escaped values into a JS string literal: the browser decodes
// &#39; back to ' before the handler is parsed, so an apostrophe in the value ends the literal. Use
// cfgJsArg(value), which yields a JSON string literal that is attribute-escaped.
func TestConfigInlineHandlersDoNotInterpolateIntoStringLiterals(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bon[a-z]+="[^"]*'\$\{\s*(?:escapeAttr|escapeHtml|escAttr|esc)\(`),
		regexp.MustCompile(`\bon[a-z]+="[^"]*\'' \+ (?:escapeAttr|escapeHtml|escAttr|esc)\(`),
		regexp.MustCompile(`\bon[a-z]+="[^"]*\$\{JSON\.stringify\(`),
	}
	var files []string
	for _, glob := range []string{"cfg/*.js", "js/config/*.js"} {
		matches, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, pattern := range patterns {
				if pattern.MatchString(line) {
					t.Errorf("%s:%d interpolates a value into an inline handler string literal (use cfgJsArg): %s", file, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}
