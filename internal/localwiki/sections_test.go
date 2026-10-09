package localwiki

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
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
		got, err := a.findSection(context.Background(), spec)
		if err != nil || got != want {
			t.Fatalf("findSection(%q) = %d, %v; want %d", spec, got, err, want)
		}
	}
	for _, spec := range []string{"4", "-1", "Wirtschaft", "!!"} {
		if _, err := a.findSection(context.Background(), spec); !errors.Is(err, ErrSectionNotFound) {
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

func TestFindSectionPrefersTheExactHeading(t *testing.T) {
	a := &renderedArticle{title: "X", sections: []renderedSection{
		{body: "Lead."},
		{heading: "Geschichte (bis 1900)", level: 2, body: "Alt."},
		{heading: "Geschichte (ab 1900)", level: 2, body: "Neu."},
		{heading: "C", level: 2, body: "c"},
		{heading: "C++", level: 2, body: "cpp"},
		{heading: "C#", level: 2, body: "cs"},
		{heading: "2020", level: 2, body: "Jahr."},
	}}
	for spec, want := range map[string]int{
		"Geschichte (ab 1900)":    2,
		"geschichte  (BIS 1900)":  1,
		"Geschichte":              1, // titleKey: the first heading without its qualifier
		"Geschichte (ab":          2, // prefix of the folded heading
		"C":                       3,
		"C++":                     4,
		"c#":                      5,
		"2020":                    6, // not a section index: matched as heading text
		"6":                       6,
		"Geschichte (seit 1900)":  1, // no exact heading: titleKey fallback
		"  Geschichte (ab 1900) ": 2,
	} {
		got, err := a.findSection(context.Background(), spec)
		if err != nil || got != want {
			t.Fatalf("findSection(%q) = %d, %v; want %d", spec, got, err, want)
		}
	}
	var notFound *SectionNotFoundError
	if _, err := a.findSection(context.Background(), "7"); !errors.As(err, &notFound) || len(notFound.Sections) != len(a.sections) || notFound.Section != "7" {
		t.Fatalf("findSection(7) err = %v, want SectionNotFoundError listing the sections", err)
	}
	if _, err := a.findSection(context.Background(), "   "); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("empty section err = %v", err)
	}
	if notFound.Total != len(a.sections) {
		t.Fatalf("total = %d, want %d", notFound.Total, len(a.sections))
	}
}

func manySections(n int) *renderedArticle {
	a := &renderedArticle{title: "Viele", sections: []renderedSection{{body: "Lead."}}}
	for i := 1; i < n; i++ {
		a.sections = append(a.sections, renderedSection{heading: fmt.Sprintf("Abschnitt %d", i), level: 2, body: "Text."})
	}
	return a
}

func TestSectionNotFoundCarriesAtMostAHundredSections(t *testing.T) {
	a := manySections(250)
	var notFound *SectionNotFoundError
	if _, err := a.findSection(context.Background(), "Gibt es nicht"); !errors.As(err, &notFound) {
		t.Fatalf("err = %v", err)
	}
	if len(notFound.Sections) != maxErrorSections || notFound.Total != 250 || notFound.Sections[99].Heading != "Abschnitt 99" {
		t.Fatalf("error carries %d sections, total %d", len(notFound.Sections), notFound.Total)
	}
	if got, err := a.findSection(context.Background(), "abschnitt 249"); err != nil || got != 249 {
		t.Fatalf("findSection(abschnitt 249) = %d, %v", got, err)
	}
}

func TestFindSectionStopsWhenTheContextEnds(t *testing.T) {
	a := manySections(3 * sectionScanCheck)
	a.layout()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.findSection(ctx, "Gibt es nicht"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got, err := a.findSection(ctx, "5"); err != nil || got != 5 {
		t.Fatalf("an index needs no scan: %d, %v", got, err)
	}
}

func TestSectionListIsACopy(t *testing.T) {
	a := sampleRendered()
	list := a.sectionList()
	list[1].Heading = "changed"
	if again := a.sectionList(); again[1].Heading != "Geschichte" {
		t.Fatalf("sectionList shares its slice: %+v", again)
	}
}

// pageTextReference is the original rune-slice implementation of pageText.
func pageTextReference(text string, offset int) (string, *int, error) {
	runes := []rune(text)
	if offset < 0 || offset > len(runes) || (offset == len(runes) && offset > 0) {
		return "", nil, ErrOffsetOutOfRange
	}
	end := offset + readPageRunes
	if end >= len(runes) {
		return strings.TrimLeft(string(runes[offset:]), "\n"), nil, nil
	}
	lo, cut, found := offset+readPageRunes/2, end, false
	for _, sep := range []func(i int) bool{
		func(i int) bool { return runes[i-1] == '\n' && runes[i-2] == '\n' },
		func(i int) bool { return runes[i-1] == '\n' },
		func(i int) bool { return runes[i-1] == ' ' },
	} {
		for i := end; i > lo && !found; i-- {
			if sep(i) {
				cut, found = i, true
			}
		}
	}
	return strings.TrimLeft(strings.TrimRight(string(runes[offset:cut]), " \n"), "\n"), &cut, nil
}

func TestPageTextMatchesTheRuneReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"a", "ü", "日", "😀", " ", "\n", "\n\n", "wort "}
	for n := range 200 {
		var b strings.Builder
		size, runes := rng.IntN(3*readPageRunes), 0
		for runes < size {
			p := pieces[rng.IntN(len(pieces))]
			if n%3 == 0 && strings.ContainsAny(p, " \n") {
				p = "x" // some texts without breaks: hard cuts
			}
			b.WriteString(p)
			runes += utf8.RuneCountInString(p)
		}
		text := b.String()
		for _, offset := range []int{0, rng.IntN(runes + 1), runes / 2, runes, runes + 1, -1} {
			got, gotNext, gotErr := pageText(text, offset)
			want, wantNext, wantErr := pageTextReference(text, offset)
			if got != want || !errors.Is(gotErr, wantErr) || (gotNext == nil) != (wantNext == nil) || (gotNext != nil && *gotNext != *wantNext) {
				t.Fatalf("text %d offset %d: got (%d runes, %v, %v), want (%d runes, %v, %v)",
					n, offset, utf8.RuneCountInString(got), gotNext, gotErr, utf8.RuneCountInString(want), wantNext, wantErr)
			}
		}
	}
}
