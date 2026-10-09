package localwiki

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// readPageRunes is the largest Content a single Read returns.
const readPageRunes = 8000

// findSection resolves a section request, in this order: a decimal index of
// an existing section; the heading itself, folded (case and accents) but
// with punctuation and qualifiers kept, so "C++" and "C#" or "Geschichte (ab
// 1900)" and "Geschichte (bis 1900)" stay apart; the heading's titleKey
// (qualifier and punctuation dropped); the first heading that starts with
// the request, folded, then as titleKey. A number that is not a section
// index is matched as heading text ("2020"). The folded headings and keys
// come precomputed from layout; ctx is checked while scanning, so a lookup in
// an article with hundreds of thousands of sections still ends at the search
// timeout.
func (a *renderedArticle) findSection(ctx context.Context, spec string) (int, error) {
	a.layout()
	spec = strings.TrimSpace(spec)
	if i, err := strconv.Atoi(spec); err == nil && i >= 0 && i < len(a.sections) {
		return i, nil
	}
	want := foldText(collapseSpace(spec))
	if want == "" {
		return 0, a.sectionNotFound(spec)
	}
	wantKey := titleKey(spec)
	for _, ok := range []func(i int) bool{
		func(i int) bool { return a.headingFolded[i] == want },
		func(i int) bool { return wantKey != "" && a.headingKeys[i] == wantKey },
		func(i int) bool { return strings.HasPrefix(a.headingFolded[i], want) },
		func(i int) bool { return wantKey != "" && strings.HasPrefix(a.headingKeys[i], wantKey) },
	} {
		for i := 1; i < len(a.sections); i++ {
			if i%sectionScanCheck == 0 {
				if err := ctx.Err(); err != nil {
					return 0, err
				}
			}
			if ok(i) {
				return i, nil
			}
		}
	}
	return 0, a.sectionNotFound(spec)
}

const (
	// sectionScanCheck is how many headings findSection compares between two
	// ctx checks.
	sectionScanCheck = 4096
	// maxErrorSections bounds the sections a SectionNotFoundError carries.
	maxErrorSections = 100
)

// sectionNotFound builds the error with the first maxErrorSections sections
// and the article's section count; it is only built when a request fails.
func (a *renderedArticle) sectionNotFound(spec string) error {
	a.layout()
	return &SectionNotFoundError{
		Section:  spec,
		Sections: slices.Clone(a.list[:min(len(a.list), maxErrorSections)]),
		Total:    len(a.list),
	}
}

// pageText returns up to readPageRunes runes of text starting at the rune
// offset, cut at a paragraph or line break in the second half of the page
// when possible. next (a rune offset) is nil on the last page. It works on
// byte indexes, so a page of a long article costs no copy of the article.
func pageText(text string, offset int) (string, *int, error) {
	if offset < 0 {
		return "", nil, ErrOffsetOutOfRange
	}
	start, ok := advanceRunes(text, 0, offset)
	if !ok || (start == len(text) && offset > 0) {
		return "", nil, ErrOffsetOutOfRange
	}
	mid, _ := advanceRunes(text, start, readPageRunes/2)
	end, ok := advanceRunes(text, mid, readPageRunes-readPageRunes/2)
	if !ok || end == len(text) {
		return strings.TrimLeft(text[start:], "\n"), nil, nil
	}
	cut := pageBreak(text, mid, end)
	next := offset + utf8.RuneCountInString(text[start:cut])
	return strings.TrimLeft(strings.TrimRight(text[start:cut], " \n"), "\n"), &next, nil
}

// advanceRunes returns the byte index n runes after byte index from; ok is
// false (and the index len(s)) when s ends first.
func advanceRunes(s string, from, n int) (int, bool) {
	i := from
	for ; n > 0; n-- {
		if i >= len(s) {
			return len(s), false
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return i, true
}

// pageBreak finds the best cut in the byte range (min, end] of text: after a
// blank line, else after a line break, else after a space, else at end. The
// separators are ASCII, so byte positions are rune boundaries; min >= 1.
func pageBreak(text string, min, end int) int {
	if i := strings.LastIndex(text[min-1:end], "\n\n"); i >= 0 {
		return min - 1 + i + 2
	}
	if i := strings.LastIndexByte(text[min:end], '\n'); i >= 0 {
		return min + i + 1
	}
	if i := strings.LastIndexByte(text[min:end], ' '); i >= 0 {
		return min + i + 1
	}
	return end
}
