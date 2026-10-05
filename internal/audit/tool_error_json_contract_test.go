package audit

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// toolErrorJSONSprintfBaseline is a ratchet: the number of hand-built error
// envelopes in internal/agent and internal/tools that interpolate %v or %s
// into a JSON string value. Such output breaks on quotes/backslashes (the
// result becomes unclassified) or can append a second "status" key. Lower the
// baseline whenever call sites move to toolErrorJSON/toolErrorf
// (internal/agent) or tools.ErrorJSON/ErrorJSONf (internal/tools).
const toolErrorJSONSprintfBaseline = 262

var (
	sprintfBacktickLiteral = regexp.MustCompile("Sprintf\\(\\s*`([^`]*)`")
	statusErrorKey         = regexp.MustCompile(`"status"\s*:\s*"error"`)
	formatVerb             = regexp.MustCompile(`^%[-+# 0]*[0-9]*(?:\.[0-9]+)?([a-zA-Z%])`)
)

// interpolatesIntoJSONString reports whether format places %v or %s inside a
// JSON string literal.
func interpolatesIntoJSONString(format string) bool {
	inString := false
	for i := 0; i < len(format); i++ {
		switch c := format[i]; {
		case c == '\\' && inString:
			i++
		case c == '"':
			inString = !inString
		case c == '%':
			if m := formatVerb.FindStringSubmatch(format[i:]); m != nil {
				if inString && (m[1] == "v" || m[1] == "s") {
					return true
				}
				i += len(m[0]) - 1
			}
		}
	}
	return false
}

func countHandFormattedToolErrors(t *testing.T, repoRoot string) (int, map[string]int) {
	t.Helper()
	perFile := map[string]int{}
	total := 0
	for _, root := range []string{"internal/agent", "internal/tools"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, filepath.FromSlash(root)), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			for scanner.Scan() {
				for _, match := range sprintfBacktickLiteral.FindAllStringSubmatch(scanner.Text(), -1) {
					if statusErrorKey.MatchString(match[1]) && interpolatesIntoJSONString(match[1]) {
						total++
						perFile[filepath.ToSlash(rel)]++
					}
				}
			}
			return scanner.Err()
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return total, perFile
}

func TestToolErrorJSONIsNotHandFormatted(t *testing.T) {
	total, perFile := countHandFormattedToolErrors(t, filepath.Join("..", ".."))
	if total > toolErrorJSONSprintfBaseline {
		files := make([]string, 0, len(perFile))
		for file, count := range perFile {
			files = append(files, fmt.Sprintf("%s=%d", file, count))
		}
		sort.Strings(files)
		t.Fatalf("%d hand-built error JSON envelopes interpolate %%v/%%s into a JSON string (ratchet baseline %d). Use toolErrorJSON/toolErrorf in internal/agent or tools.ErrorJSON/ErrorJSONf in internal/tools.\n%s",
			total, toolErrorJSONSprintfBaseline, strings.Join(files, "\n"))
	}
	if total < toolErrorJSONSprintfBaseline {
		t.Logf("hand-built error JSON envelopes fell to %d; lower toolErrorJSONSprintfBaseline from %d", total, toolErrorJSONSprintfBaseline)
	}
}

func TestInterpolatesIntoJSONStringDetectsOnlyQuotedVerbs(t *testing.T) {
	for _, item := range []struct {
		format string
		want   bool
	}{
		{`{"status":"error","message":"%v"}`, true},
		{`{"status":"error","message":"failed: %s"}`, true},
		{`{"status":"error","message":%q}`, false},
		{`{"status":"error","count":%d}`, false},
		{`{"status":"error","detail":%s}`, false},
		{`{"status":"error","message":"100%% done"}`, false},
	} {
		if got := interpolatesIntoJSONString(item.format); got != item.want {
			t.Fatalf("interpolatesIntoJSONString(%s) = %v, want %v", item.format, got, item.want)
		}
	}
}
