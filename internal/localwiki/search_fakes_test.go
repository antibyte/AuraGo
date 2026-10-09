package localwiki

import (
	"context"
	"sort"
	"strings"

	"aurago/internal/zim"
	"aurago/internal/zim/xapian"
)

// fakeEntry is one entry of fakeArticleStore; redirectTo marks a redirect.
type fakeEntry struct {
	path, title, html, redirectTo string
}

// fakeArticleStore is an in-memory articleStore.
type fakeArticleStore struct {
	byPath map[string]zim.Entry
	html   map[string]string
	target map[string]string
	sorted []zim.Entry
	reads  int
	// prefixes records the titlePrefix scans; afterRead runs after each readHTML.
	prefixes  []string
	afterRead func()
}

func newFakeArticleStore(articles ...fakeEntry) *fakeArticleStore {
	s := &fakeArticleStore{byPath: map[string]zim.Entry{}, html: map[string]string{}, target: map[string]string{}}
	for i, a := range articles {
		e := zim.Entry{Index: uint32(i), Namespace: 'C', Path: a.path, Title: a.title}
		if a.redirectTo != "" {
			e.IsRedirect = true
			s.target[a.path] = a.redirectTo
		} else {
			e.MimeType = "text/html"
			s.html[a.path] = a.html
		}
		s.byPath[a.path] = e
		s.sorted = append(s.sorted, e)
	}
	sort.Slice(s.sorted, func(i, j int) bool { return s.sorted[i].Title < s.sorted[j].Title })
	return s
}

func (s *fakeArticleStore) lookup(path string) (zim.Entry, error) {
	e, ok := s.byPath[path]
	if !ok {
		return zim.Entry{}, zim.ErrNotFound
	}
	return e, nil
}

func (s *fakeArticleStore) resolve(e zim.Entry) (zim.Entry, error) {
	for depth := 0; e.IsRedirect; depth++ {
		if depth == 8 {
			return zim.Entry{}, zim.ErrRedirectLoop
		}
		next, ok := s.byPath[s.target[e.Path]]
		if !ok {
			return zim.Entry{}, zim.ErrNotFound
		}
		e = next
	}
	return e, nil
}

func (s *fakeArticleStore) titlePrefix(prefix string, limit int) ([]zim.Entry, error) {
	s.prefixes = append(s.prefixes, prefix)
	var out []zim.Entry
	for _, e := range s.sorted {
		if strings.HasPrefix(e.Title, prefix) && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *fakeArticleStore) readHTML(e zim.Entry) ([]byte, error) {
	s.reads++
	if s.afterRead != nil {
		defer s.afterRead()
	}
	if e.IsRedirect || !isHTMLEntry(e) {
		return nil, ErrNotArticle
	}
	return []byte(s.html[e.Path]), nil
}

func (s *fakeArticleStore) cacheKey(path string) string { return "fake/" + path }

// fakeTitleIndex answers suggestions and completions from fixed tables keyed
// by query and prefix.
type fakeTitleIndex struct {
	hits     map[string][]xapian.Hit
	terms    map[string][]string
	err      error
	suggests *int // counts suggest calls when set
}

func (t fakeTitleIndex) completions(_ context.Context, prefix string, n int) ([]string, error) {
	if t.err != nil {
		return nil, t.err
	}
	terms := t.terms[prefix]
	return terms[:min(n, len(terms))], nil
}

func (t fakeTitleIndex) suggest(_ context.Context, query string, limit int) ([]xapian.Hit, error) {
	if t.suggests != nil {
		*t.suggests++
	}
	if t.err != nil {
		return nil, t.err
	}
	hits := t.hits[query]
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// fakeFulltextIndex records the searches it receives.
type fakeFulltextIndex struct {
	freq     map[string]uint32
	and, or  []xapian.Hit
	searches []fakeFulltextCall
}

type fakeFulltextCall struct {
	terms []string
	op    xapian.Op
	limit int
}

func (f *fakeFulltextIndex) terms(query string) []string {
	return strings.Fields(strings.ToLower(query))
}

// existingTerms mirrors xapian.ExistingTerms: order and repeats kept, terms without postings dropped.
func (f *fakeFulltextIndex) existingTerms(terms []string) ([]string, error) {
	var out []string
	for _, term := range terms {
		if f.freq[term] > 0 {
			out = append(out, term)
		}
	}
	return out, nil
}

func (f *fakeFulltextIndex) search(_ context.Context, terms []string, op xapian.Op, limit int) ([]xapian.Hit, error) {
	f.searches = append(f.searches, fakeFulltextCall{terms: append([]string(nil), terms...), op: op, limit: limit})
	hits := f.and
	if op == xapian.OpOr {
		hits = f.or
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
