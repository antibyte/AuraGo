package localwiki

import (
	"strconv"
	"strings"
)

// readPageRunes is the largest Content a single Read returns.
const readPageRunes = 8000

// findSection resolves a section request: a decimal index, else a heading
// compared case- and accent-insensitively, else the first heading that
// starts with the request.
func (a *renderedArticle) findSection(spec string) (int, error) {
	spec = strings.TrimSpace(spec)
	notFound := &SectionNotFoundError{Section: spec, Sections: a.sectionList()}
	if i, err := strconv.Atoi(spec); err == nil {
		if i < 0 || i >= len(a.sections) {
			return 0, notFound
		}
		return i, nil
	}
	want := titleKey(spec)
	if want == "" {
		return 0, notFound
	}
	for i, s := range a.sections {
		if i > 0 && titleKey(s.heading) == want {
			return i, nil
		}
	}
	for i, s := range a.sections {
		if i > 0 && strings.HasPrefix(titleKey(s.heading), want) {
			return i, nil
		}
	}
	return 0, notFound
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
