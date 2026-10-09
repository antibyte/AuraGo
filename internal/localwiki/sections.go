package localwiki

import (
	"strconv"
	"strings"
)

// readPageRunes is the largest Content a single Read returns.
const readPageRunes = 8000

// findSection resolves a section request, in this order: a decimal index of
// an existing section; the heading itself, folded (case and accents) but
// with punctuation and qualifiers kept, so "C++" and "C#" or "Geschichte (ab
// 1900)" and "Geschichte (bis 1900)" stay apart; the heading's titleKey
// (qualifier and punctuation dropped); the first heading that starts with
// the request, folded, then as titleKey. A number that is not a section
// index is matched as heading text ("2020").
func (a *renderedArticle) findSection(spec string) (int, error) {
	spec = strings.TrimSpace(spec)
	if i, err := strconv.Atoi(spec); err == nil && i >= 0 && i < len(a.sections) {
		return i, nil
	}
	want := foldText(collapseSpace(spec))
	if want == "" {
		return 0, a.sectionNotFound(spec)
	}
	wantKey := titleKey(spec)
	match := func(ok func(heading, key string) bool) (int, bool) {
		for i, s := range a.sections {
			if i > 0 && ok(foldText(collapseSpace(s.heading)), titleKey(s.heading)) {
				return i, true
			}
		}
		return 0, false
	}
	for _, ok := range []func(heading, key string) bool{
		func(heading, _ string) bool { return heading == want },
		func(_, key string) bool { return wantKey != "" && key == wantKey },
		func(heading, _ string) bool { return strings.HasPrefix(heading, want) },
		func(_, key string) bool { return wantKey != "" && strings.HasPrefix(key, wantKey) },
	} {
		if i, found := match(ok); found {
			return i, nil
		}
	}
	return 0, a.sectionNotFound(spec)
}

// sectionNotFound builds the error with the article's section list; it is
// only built when a request fails.
func (a *renderedArticle) sectionNotFound(spec string) error {
	return &SectionNotFoundError{Section: spec, Sections: a.sectionList()}
}

// pageText returns up to readPageRunes runes of text starting at the rune
// offset, cut at a paragraph or line break in the second half of the page
// when possible. next is nil on the last page.
func pageText(text string, offset int) (string, *int, error) {
	runes := []rune(text)
	if offset < 0 || offset > len(runes) || (offset == len(runes) && offset > 0) {
		return "", nil, ErrOffsetOutOfRange
	}
	end := offset + readPageRunes
	if end >= len(runes) {
		return strings.TrimLeft(string(runes[offset:]), "\n"), nil, nil
	}
	cut := pageBreak(runes, offset+readPageRunes/2, end)
	next := cut
	return strings.TrimLeft(strings.TrimRight(string(runes[offset:cut]), " \n"), "\n"), &next, nil
}

// pageBreak finds the best cut in runes[min:end]: after a blank line, else
// after a line break, else after a space, else at end.
func pageBreak(runes []rune, min, end int) int {
	for i := end; i > min; i-- {
		if runes[i-1] == '\n' && runes[i-2] == '\n' {
			return i
		}
	}
	for i := end; i > min; i-- {
		if runes[i-1] == '\n' {
			return i
		}
	}
	for i := end; i > min; i-- {
		if runes[i-1] == ' ' {
			return i
		}
	}
	return end
}
