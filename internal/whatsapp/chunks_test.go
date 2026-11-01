package whatsapp

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func fenceLineCount(part string) int {
	n := 0
	for _, line := range strings.Split(part, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			n++
		}
	}
	return n
}

func TestChunkTextEdgeCases(t *testing.T) {
	for _, in := range []string{"", "\n\n", "  \n \t\n"} {
		if got := chunkText(in, 40); got != nil {
			t.Fatalf("chunkText(%q) = %q, want no parts", in, got)
		}
	}
	if got := chunkText("\n\nhello\n\n", 40); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("surrounding newlines must be trimmed, got %q", got)
	}
	exact := strings.Repeat("ä", 40)
	if got := chunkText(exact, 40); len(got) != 1 || got[0] != exact {
		t.Fatalf("text of exactly the limit must be one part, got %q", got)
	}
}

func TestChunkTextPrefersParagraphThenLine(t *testing.T) {
	text := "aaaa aaaa aaaa\n\nbbbb bbbb bbbb\nccc ccc ccc ccc ccc ccc c\ndddd"
	want := []string{
		"aaaa aaaa aaaa", // cut after the blank line although "bbbb ..." would still fit
		"bbbb bbbb bbbb\nccc ccc ccc ccc ccc ccc c",
		"dddd", // no blank line in the window: cut at the line end
	}
	got := chunkText(text, 40)
	if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", want) {
		t.Fatalf("parts:\n got %q\nwant %q", got, want)
	}
	if stripSpace(strings.Join(got, "")) != stripSpace(text) {
		t.Fatalf("the parts must restore the text apart from whitespace at the cuts: %q", got)
	}
	for _, part := range got {
		if utf8.RuneCountInString(part) > 40 {
			t.Fatalf("part over the limit: %q", part)
		}
	}
}

func TestChunkTextReopensCodeFences(t *testing.T) {
	const limit = 40
	var code []string
	for i := 0; i < 14; i++ {
		code = append(code, fmt.Sprintf("v%02d := compute(%02d)", i, i))
	}
	text := "```go\n" + strings.Join(code, "\n") + "\n```"
	if utf8.RuneCountInString(text) <= 3*limit {
		t.Fatalf("fixture must be longer than three limits, got %d runes", utf8.RuneCountInString(text))
	}
	parts := chunkText(text, limit)
	if len(parts) < 4 {
		t.Fatalf("want at least 4 parts, got %d: %q", len(parts), parts)
	}
	var restored []string
	for i, part := range parts {
		if n := utf8.RuneCountInString(part); n > limit {
			t.Fatalf("part %d has %d runes (limit %d): %q", i, n, limit, part)
		}
		if fenceLineCount(part)%2 != 0 {
			t.Fatalf("part %d has unbalanced fences: %q", i, part)
		}
		if strings.TrimSpace(part) == "" {
			t.Fatalf("part %d is whitespace-only", i)
		}
		if !strings.HasPrefix(part, "```go\n") {
			t.Fatalf("part %d must start with the opening fence line: %q", i, part)
		}
		for _, line := range strings.Split(part, "\n") {
			if !strings.HasPrefix(line, "```") {
				restored = append(restored, line)
			}
		}
	}
	if strings.Join(restored, "\n") != strings.Join(code, "\n") {
		t.Fatalf("removing the fence lines must restore the code:\n got %q\nwant %q", restored, code)
	}
}

func TestChunkTextHardSplitsLongLinesOnRunes(t *testing.T) {
	// No space: cut on rune boundaries.
	long := strings.Repeat("äöüß€", 20) // 100 runes, 3 to 4 bytes beyond ASCII each
	parts := chunkText(long, 30)
	if len(parts) != 4 {
		t.Fatalf("want 4 parts, got %d: %q", len(parts), parts)
	}
	for i, part := range parts {
		if !utf8.ValidString(part) {
			t.Fatalf("part %d is not valid UTF-8", i)
		}
		if n := utf8.RuneCountInString(part); n > 30 {
			t.Fatalf("part %d has %d runes", i, n)
		}
	}
	if strings.Join(parts, "") != long {
		t.Fatal("the parts must restore the line")
	}

	// With spaces: cut at the last space inside the window.
	words := "wörter mit ümläuten und viele weitere kurze wörter hier"
	got := chunkText(words, 20)
	want := []string{"wörter mit ümläuten", "und viele weitere", "kurze wörter hier"}
	if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", want) {
		t.Fatalf("word split:\n got %q\nwant %q", got, want)
	}
}
