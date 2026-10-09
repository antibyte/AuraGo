package localwiki

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func sampleRendered() *renderedArticle {
	return &renderedArticle{title: "Ulm", sections: []renderedSection{
		{body: "Ulm ist eine Stadt."},
		{heading: "Geschichte", level: 2, body: "Alt."},
		{heading: "Mittelalter", level: 3, body: "Reichsstadt."},
		{heading: "Münster", level: 2, body: "Hoher Turm."},
	}}
}

func TestSectionMarkdownIncludesSubsections(t *testing.T) {
	a := sampleRendered()
	if got := a.sectionMarkdown(1); got != "## Geschichte\n\nAlt.\n\n### Mittelalter\n\nReichsstadt." {
		t.Fatalf("got %q", got)
	}
	if got := a.sectionMarkdown(0); got != "Ulm ist eine Stadt." {
		t.Fatalf("lead = %q", got)
	}
	if got := a.fullMarkdown(); got != "# Ulm\n\nUlm ist eine Stadt.\n\n## Geschichte\n\nAlt.\n\n### Mittelalter\n\nReichsstadt.\n\n## Münster\n\nHoher Turm." {
		t.Fatalf("full = %q", got)
	}
	list := a.sectionList()
	if list[1].Chars != utf8.RuneCountInString(a.sectionMarkdown(1)) || list[3] != (Section{Index: 3, Heading: "Münster", Level: 2, Chars: 23}) {
		t.Fatalf("sections = %+v", list)
	}
}

func TestFindSection(t *testing.T) {
	a := sampleRendered()
	for spec, want := range map[string]int{"0": 0, "2": 2, "Geschichte": 1, "  mittelalter ": 2, "MUNSTER": 3, "Gesch": 1} {
		got, err := a.findSection(spec)
		if err != nil || got != want {
			t.Fatalf("findSection(%q) = %d, %v; want %d", spec, got, err, want)
		}
	}
	for _, spec := range []string{"4", "-1", "Wirtschaft", "!!"} {
		if _, err := a.findSection(spec); !errors.Is(err, ErrSectionNotFound) {
			t.Fatalf("findSection(%q) err = %v", spec, err)
		}
	}
}

func TestPageTextCutsAtParagraphs(t *testing.T) {
	para := strings.Repeat("x", 999)
	text := strings.Repeat(para+"\n\n", 20)
	first, next, err := pageText(text, 0)
	if err != nil || next == nil {
		t.Fatalf("first page: next=%v err=%v", next, err)
	}
	if utf8.RuneCountInString(first) > readPageRunes || !strings.HasSuffix(first, "x") {
		t.Fatalf("first page ends with %q, len %d", first[len(first)-5:], len(first))
	}
	if *next%1001 != 0 {
		t.Fatalf("next = %d, want a paragraph boundary", *next)
	}
	rest, last, err := pageText(text, *next)
	if err != nil || strings.HasPrefix(rest, "\n") {
		t.Fatalf("second page: %q... err %v", rest[:5], err)
	}
	_ = last
	if _, _, err := pageText(text, utf8.RuneCountInString(text)); !errors.Is(err, ErrOffsetOutOfRange) {
		t.Fatalf("offset at end: %v", err)
	}
	if got, next, err := pageText("", 0); got != "" || next != nil || err != nil {
		t.Fatalf("empty text: %q %v %v", got, next, err)
	}
}

func TestPageTextHardCutWithoutBreaks(t *testing.T) {
	text := strings.Repeat("ü", readPageRunes+10)
	got, next, err := pageText(text, 0)
	if err != nil || next == nil || *next != readPageRunes || utf8.RuneCountInString(got) != readPageRunes {
		t.Fatalf("got %d runes next %v err %v", utf8.RuneCountInString(got), next, err)
	}
	tail, end, err := pageText(text, *next)
	if err != nil || end != nil || tail != strings.Repeat("ü", 10) {
		t.Fatalf("tail %q end %v err %v", tail, end, err)
	}
}
