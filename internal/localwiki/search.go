package localwiki

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/zim"
	"aurago/internal/zim/xapian"
)

const (
	maxConcurrentSearches = 4
	searchTimeout         = 5 * time.Second
	defaultSearchLimit    = 5
	maxSearchResults      = 30 // the Desktop asks for up to 30; the agent tool narrows to 1-10
	maxSearchLeads        = 3  // only the top results carry their lead
	maxSuggestions        = 10
	titleCandidateLimit   = 20
	// maxCompletions is how many of a typed word's most frequent completions
	// a suggestion offers as whole titles ("Berl" -> Berlin, Berliner, ...).
	maxCompletions = 4
)

// searchSlots bounds concurrent searches, suggestions and reads process-wide;
// only one edition is open at a time.
var searchSlots = make(chan struct{}, maxConcurrentSearches)

// acquireSearchSlot waits for a free slot until ctx ends.
func acquireSearchSlot(ctx context.Context) (func(), error) {
	select {
	case searchSlots <- struct{}{}:
		return func() { <-searchSlots }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %w", ErrSearchBusy, ctx.Err())
	}
}

// boundedCall runs fn with the 5 s search timeout inside one search slot.
func boundedCall[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	release, err := acquireSearchSlot(ctx)
	if err != nil {
		return zero, err
	}
	defer release()
	return fn(ctx)
}

// clampLimit maps a requested result count to 1..max, 0 or less meaning the default.
func clampLimit(limit, max int) int {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	return min(limit, max)
}

// Search finds articles for query: exact or near-exact title matches (also
// via redirects) first, then full-text BM25 hits (all terms, filled up with
// any term), then further title suggestions. limit is 1..30 (0 or less means
// 5). The first withLeads results (at most 3) carry their Markdown lead.
// When the time runs out while snippets are extracted, the hits are returned
// without the snippets and leads still missing.
func (l *Library) Search(ctx context.Context, query string, limit int, withLeads int) (SearchResult, error) {
	hits, err := boundedCall(ctx, func(ctx context.Context) ([]SearchHit, error) {
		return l.searchView().search(ctx, query, limit, withLeads)
	})
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Edition: l.Edition(), Fulltext: l.Fulltext(), Results: hits}, nil
}

// Suggest returns up to limit (at most 10) title suggestions for a typed prefix.
// For a single word, titles that start with it come first (see
// prefixSuggestions), then the title index's libzim-style ranking.
func (l *Library) Suggest(ctx context.Context, query string, limit int) ([]Ref, error) {
	return boundedCall(ctx, func(ctx context.Context) ([]Ref, error) {
		return l.searchView().suggest(ctx, query, limit)
	})
}

func (ix searchIndex) search(ctx context.Context, query string, limit, withLeads int) ([]SearchHit, error) {
	query, err := normalizeQuery(query)
	if err != nil {
		return nil, err
	}
	m := newHitMerger(ix.store, clampLimit(limit, maxSearchResults))
	for _, path := range pathCandidates(query) {
		if _, err := m.add(path); err != nil {
			return nil, err
		}
	}
	titles, err := ix.titleHits(ctx, query, max(titleCandidateLimit, m.limit))
	if err != nil {
		return nil, err
	}
	key := titleKey(query)
	for _, h := range titles {
		if titleKey(h.Title) == key {
			if _, err := m.add(h.Path); err != nil {
				return nil, err
			}
		}
	}
	if ix.fulltext != nil {
		hits, err := ix.fulltextHits(ctx, query, m.limit)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			if _, err := m.add(h.Path); err != nil {
				return nil, err
			}
		}
	}
	for _, h := range titles {
		if _, err := m.add(h.Path); err != nil {
			return nil, err
		}
	}
	withLeads = min(withLeads, maxSearchLeads)
	words := matchWords(query)
	results := make([]SearchHit, len(m.refs))
	for i, ref := range m.refs {
		results[i].Ref = ref
	}
	for i := range results {
		if ctx.Err() != nil {
			// Out of time: the hits found so far are still the answer; the
			// remaining ones keep their titles without snippet or lead.
			break
		}
		raw, err := ix.store.readHTML(m.entries[i])
		if err != nil {
			if errors.Is(err, zim.ErrClosed) {
				return nil, err
			}
			continue // a damaged or oversized article keeps its title without snippet
		}
		results[i].Snippet = snippetFromHTML(raw, words)
		if i < withLeads {
			if lead, err := leadFromHTML(raw); err == nil {
				results[i].Lead = lead
			}
		}
	}
	return results, nil
}

func (ix searchIndex) suggest(ctx context.Context, query string, limit int) ([]Ref, error) {
	query, err := normalizeQuery(query)
	if err != nil {
		return nil, err
	}
	m := newHitMerger(ix.store, clampLimit(limit, maxSuggestions))
	if !strings.Contains(query, " ") {
		if err := ix.prefixSuggestions(ctx, m, query); err != nil {
			return nil, err
		}
		if len(m.refs) >= m.limit {
			return m.refs, nil // the title index could add nothing
		}
	}
	hits, err := ix.titleHits(ctx, query, titleCandidateLimit)
	if err != nil {
		return nil, err
	}
	for _, h := range hits {
		if _, err := m.add(h.Path); err != nil {
			return nil, err
		}
	}
	return m.refs, nil
}

// prefixSuggestions adds, for a single typed word, the titles a type-ahead
// expects before the title index's ranking, which like libzim prefers titles
// that contain the typed word as a whole word ("Berl" ranks "Berl Broder"
// and "Antonie Berl" above "Berlin"): a title equal to the word (as typed or
// with a capital first letter), then the titles of the word's most frequent
// completions in the title index ("Berlin", "Berliner"), then the other
// titles that start with it, in title order. Deprecated entries are skipped
// (zimStore.titlePrefix) and redirects collapse into their targets
// (hitMerger). Without a title index, or for one character, only the
// title-ordered part runs.
func (ix searchIndex) prefixSuggestions(ctx context.Context, m *hitMerger, word string) error {
	variants := []string{word}
	if upper := upperFirst(word); upper != word {
		variants = append(variants, upper)
	}
	var listed []zim.Entry
	for _, v := range variants {
		entries, err := ix.store.titlePrefix(v, m.limit)
		if err != nil {
			return fmt.Errorf("title prefix search: %w", err)
		}
		listed = append(listed, entries...)
	}
	for _, e := range listed {
		if e.Title == word || e.Title == variants[len(variants)-1] {
			if _, err := m.add(e.Path); err != nil {
				return err
			}
		}
	}
	if ix.titles != nil && utf8.RuneCountInString(word) > 1 {
		terms, err := ix.titles.completions(ctx, foldText(word), maxCompletions)
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		// A failing title index only costs the completions.
		for _, term := range terms {
			typed, folded := completionPaths(word, term)
			found := false
			for _, path := range typed {
				listed, err := m.add(path)
				if err != nil {
					return err
				}
				found = found || listed
			}
			// The folded spelling only stands in when the typed one names no
			// article: "Mün" must not also reach the redirect "Munchen".
			if !found && folded != "" {
				if _, err := m.add(folded); err != nil {
					return err
				}
			}
		}
	}
	for _, e := range listed {
		if _, err := m.add(e.Path); err != nil {
			return err
		}
	}
	return nil
}

// completionPaths turns a folded completion of the typed word into likely
// article paths: typed is the typed text plus the rest of the completion (so
// "Mün" completes to "München" although the index folds it to "munchen"),
// also with a capital first letter; folded is the completion itself
// capitalised, for a spelling the typed text cannot give ("BERL" ->
// "Berlin"), and "" when it is one of typed.
func completionPaths(word, term string) (typed []string, folded string) {
	if rest, ok := strings.CutPrefix(term, foldText(word)); ok {
		typed = append(typed, word+rest)
		if upper := upperFirst(word + rest); upper != word+rest {
			typed = append(typed, upper)
		}
	}
	if folded = upperFirst(term); slices.Contains(typed, folded) {
		folded = ""
	}
	return typed, folded
}

// titleHits queries the title index; without one (or when it fails) it
// falls back to the titleOrdered list, as typed and with a capital first letter.
// A one-character query always uses the titleOrdered list: in the title index
// it would expand to the 100 most frequent terms and score all their documents
// (slice 2 contract note 7).
func (ix searchIndex) titleHits(ctx context.Context, query string, limit int) ([]xapian.Hit, error) {
	if ix.titles != nil && utf8.RuneCountInString(query) > 1 {
		hits, err := ix.titles.suggest(ctx, query, limit)
		if err == nil {
			return hits, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	prefixes := []string{query}
	if upper := upperFirst(query); upper != query {
		prefixes = append(prefixes, upper)
	}
	var out []xapian.Hit
	seen := map[string]bool{}
	for _, prefix := range prefixes {
		entries, err := ix.store.titlePrefix(prefix, limit)
		if err != nil {
			return nil, fmt.Errorf("title prefix search: %w", err)
		}
		for _, e := range entries {
			if !seen[e.Path] {
				seen[e.Path] = true
				out = append(out, xapian.Hit{Path: e.Path, Title: e.Title})
			}
		}
	}
	return out, nil
}

// fulltextHits drops terms without postings (xapian.ExistingTerms; repeated
// words stay repeated, as libzim weights them), runs an AND search and fills
// up with OR results when it finds too few.
func (ix searchIndex) fulltextHits(ctx context.Context, query string, limit int) ([]xapian.Hit, error) {
	terms, err := ix.fulltext.existingTerms(ix.fulltext.terms(query))
	if err != nil {
		return nil, fmt.Errorf("full-text terms: %w", err)
	}
	if len(terms) == 0 {
		return nil, nil
	}
	hits, err := ix.fulltext.search(ctx, terms, xapian.OpAnd, limit)
	if err != nil {
		return nil, fmt.Errorf("full-text search: %w", err)
	}
	if len(hits) >= limit || len(terms) == 1 {
		return hits, nil
	}
	more, err := ix.fulltext.search(ctx, terms, xapian.OpOr, limit+len(hits))
	if err != nil {
		return nil, fmt.Errorf("full-text search: %w", err)
	}
	have := map[uint32]bool{}
	for _, h := range hits {
		have[h.DocID] = true
	}
	for _, h := range more {
		if len(hits) >= limit {
			break
		}
		if !have[h.DocID] {
			have[h.DocID] = true
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// hitMerger collects results in priority order, resolving redirects and
// keeping each resolved article once. Display titles come from the resolved
// ZIM entry, never from an index.
type hitMerger struct {
	store   articleStore
	limit   int
	seen    map[string]bool
	refs    []Ref
	entries []zim.Entry
}

func newHitMerger(store articleStore, limit int) *hitMerger {
	return &hitMerger{store: store, limit: limit, seen: map[string]bool{}, refs: []Ref{}}
}

// add appends the article behind path unless it is missing, not an HTML
// article or already listed. listed reports whether path names an article
// that is in the list now (added by this call or before); a full list looks
// nothing up and reports false. Only a closed archive is an error: then every
// further lookup fails too.
func (m *hitMerger) add(path string) (listed bool, err error) {
	if len(m.refs) >= m.limit || path == "" {
		return false, nil
	}
	e, err := m.store.lookup(path)
	if err != nil {
		return false, closedArchive(err)
	}
	resolved, err := m.store.resolve(e)
	if err != nil {
		return false, closedArchive(err)
	}
	if !isHTMLEntry(resolved) {
		return false, nil
	}
	if m.seen[resolved.Path] {
		return true, nil
	}
	m.seen[resolved.Path] = true
	m.refs = append(m.refs, Ref{Title: resolved.Title, Path: resolved.Path})
	m.entries = append(m.entries, resolved)
	return true, nil
}

// closedArchive passes on zim.ErrClosed and drops every other entry error
// (missing path, broken redirect): that candidate is skipped.
func closedArchive(err error) error {
	if errors.Is(err, zim.ErrClosed) {
		return err
	}
	return nil
}
