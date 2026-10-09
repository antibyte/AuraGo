package localwiki

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"aurago/internal/zim"
	"aurago/internal/zim/xapian"
)

// maxArticleBytes bounds the HTML of one article that search and read load.
const maxArticleBytes = 16 << 20

// articleStore is the part of a ZIM archive that search and read use.
// Paths are content-namespace paths without the namespace prefix.
type articleStore interface {
	lookup(path string) (zim.Entry, error)
	resolve(e zim.Entry) (zim.Entry, error)
	titlePrefix(prefix string, limit int) ([]zim.Entry, error)
	readHTML(e zim.Entry) ([]byte, error)
	cacheKey(path string) string
}

// titleIndex suggests titles from X/title/xapian.
type titleIndex interface {
	suggest(ctx context.Context, query string, limit int) ([]xapian.Hit, error)
}

// fulltextIndex runs BM25 searches over X/fulltext/xapian.
type fulltextIndex interface {
	terms(query string) []string
	existingTerms(terms []string) ([]string, error)
	search(ctx context.Context, terms []string, op xapian.Op, limit int) ([]xapian.Hit, error)
}

// searchIndex bundles what search and read need from one open edition.
// titles == nil selects the titleOrdered prefix fallback; fulltext == nil
// is the degraded title-only mode.
type searchIndex struct {
	store    articleStore
	titles   titleIndex
	fulltext fulltextIndex
}

// searchView builds the search view of the library. It is the only place that
// reads the Library fields declared by slice 3.
func (l *Library) searchView() searchIndex {
	ix := searchIndex{store: zimStore{a: l.archive}}
	if l.title != nil {
		ix.titles = xapianTitles{db: l.title, analyzer: l.analyzer}
	}
	if l.Fulltext() {
		ix.fulltext = xapianFulltext{db: l.fulltext, analyzer: l.analyzer}
	}
	return ix
}

// zimStore is the articleStore of an open ZIM archive.
type zimStore struct{ a *zim.Archive }

func (s zimStore) lookup(path string) (zim.Entry, error) {
	return s.a.EntryByPath(s.a.ContentNamespace(), path)
}

func (s zimStore) resolve(e zim.Entry) (zim.Entry, error) { return s.a.Resolve(e) }

// titlePrefix lists the titleOrdered entries starting with prefix (byte-wise).
// Deprecated entries, which a v0 listing of an old archive may contain (no
// redirect and no MIME type), are skipped: they carry no content.
func (s zimStore) titlePrefix(prefix string, limit int) ([]zim.Entry, error) {
	entries, err := s.a.TitlePrefix(prefix, limit)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if e.IsRedirect || e.MimeType != "" {
			out = append(out, e)
		}
	}
	return out, nil
}

// readHTML loads the HTML of a resolved article, refusing articles larger
// than maxArticleBytes before allocating.
func (s zimStore) readHTML(e zim.Entry) ([]byte, error) {
	if e.IsRedirect || !isHTMLEntry(e) {
		return nil, ErrNotArticle
	}
	r, err := s.a.Open(e)
	if err != nil {
		return nil, fmt.Errorf("open article %q: %w", e.Path, err)
	}
	if r.Size() > maxArticleBytes {
		return nil, ErrArticleTooLarge
	}
	raw := make([]byte, r.Size())
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, fmt.Errorf("read article %q: %w", e.Path, err)
	}
	return raw, nil
}

func (s zimStore) cacheKey(path string) string {
	uuid := s.a.UUID()
	return hex.EncodeToString(uuid[:]) + "/" + path
}

func isHTMLEntry(e zim.Entry) bool {
	return strings.HasPrefix(e.MimeType, "text/html")
}

// searchWindow keeps a requested hit count inside what xapian accepts
// (0..MaxSearchWindow; Search rejects a negative limit).
func searchWindow(limit int) int {
	return max(0, min(limit, xapian.MaxSearchWindow))
}

type xapianTitles struct {
	db       *xapian.Database
	analyzer xapian.Analyzer
}

// suggest returns title-index hits. Their paths may name redirects (the
// index collapses a redirect with its target only when it ranks better), so
// callers resolve and deduplicate them.
func (t xapianTitles) suggest(ctx context.Context, query string, limit int) ([]xapian.Hit, error) {
	return xapian.Suggest(ctx, t.db, t.analyzer, query, searchWindow(limit))
}

type xapianFulltext struct {
	db       *xapian.Database
	analyzer xapian.Analyzer
}

func (f xapianFulltext) terms(query string) []string { return f.analyzer.QueryTerms(query) }

// existingTerms drops query terms without postings (slice 2 leaves this to the
// caller; xapian.Search itself is a faithful Xapian AND).
func (f xapianFulltext) existingTerms(terms []string) ([]string, error) {
	return xapian.ExistingTerms(f.db, terms)
}

// search runs one BM25 search. Hit titles come from the full-text index's
// value slot (folded there), so callers take display titles from the
// resolved ZIM entry instead.
func (f xapianFulltext) search(ctx context.Context, terms []string, op xapian.Op, limit int) ([]xapian.Hit, error) {
	hits, _, err := xapian.Search(ctx, f.db, terms, op, 0, searchWindow(limit))
	return hits, err
}
