package localwiki

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
	"unicode"

	"aurago/internal/zim"
)

const (
	// renderCacheSize is the number of rendered articles kept for paging.
	renderCacheSize = 8
	// renderCacheBytes bounds the memory of the cached articles; an article
	// above half of it is rendered for every read instead of being cached.
	renderCacheBytes = 32 << 20
)

// Read returns one page (at most 8,000 characters) of an article as
// Markdown: the whole article, or one section with its subsections.
func (l *Library) Read(ctx context.Context, req ReadRequest) (Article, error) {
	return boundedCall(ctx, func(ctx context.Context) (Article, error) {
		return l.searchView().read(ctx, req)
	})
}

// Lead returns the Markdown lead of the article at path (prose before the
// first heading, without infobox, tables and images), at most 2,000 characters.
func (l *Library) Lead(ctx context.Context, path string) (string, error) {
	return boundedCall(ctx, func(ctx context.Context) (string, error) {
		return l.searchView().lead(ctx, path)
	})
}

func (ix searchIndex) lead(ctx context.Context, path string) (string, error) {
	e, _, err := ix.findArticle(ctx, path, "")
	if err != nil {
		return "", err
	}
	raw, err := ix.store.readHTML(e)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return leadFromHTML(raw)
}

func (ix searchIndex) read(ctx context.Context, req ReadRequest) (Article, error) {
	if req.Offset < 0 {
		return Article{}, ErrOffsetOutOfRange
	}
	e, redirectedFrom, err := ix.findArticle(ctx, req.Path, req.Title)
	if err != nil {
		return Article{}, err
	}
	art, err := ix.rendered(ctx, e)
	if err != nil {
		return Article{}, err
	}
	text := art.fullMarkdown()
	if strings.TrimSpace(req.Section) != "" {
		i, err := art.findEchoedSection(ctx, req.Section)
		if err != nil {
			return Article{}, err
		}
		text = art.sectionMarkdown(i)
	}
	content, next, err := pageText(text, req.Offset)
	if err != nil {
		return Article{}, err
	}
	return Article{
		Ref:            Ref{Title: e.Title, Path: e.Path},
		RedirectedFrom: redirectedFrom,
		Sections:       art.sectionList(),
		Content:        content,
		NextOffset:     next,
	}, nil
}

// findArticle resolves a path (preferred) or a title to an HTML article and
// reports the title of a followed redirect. A path with spaces is also tried
// with underscores. A title is tried as a path first, then among the title
// suggestions: an exact title (case-insensitive, underscores read as spaces)
// before one equal after folding (accents, qualifier kept) before one with
// the same titleKey (qualifier dropped), so "Berlin (Band)" never resolves
// to "Berlin" while the band's article is among the suggestions. A closed
// archive (an edition swap in progress) is an error of its own, not a
// missing article.
func (ix searchIndex) findArticle(ctx context.Context, path, title string) (zim.Entry, string, error) {
	path, title = cleanArticleRef(path), cleanArticleRef(title)
	var candidates []string
	if path != "" {
		candidates = append(candidates, path)
		rest, ok := strings.CutPrefix(path, "C/")
		if ok && rest != "" {
			candidates = append(candidates, rest)
		} else {
			rest = path
		}
		if strings.ContainsFunc(rest, unicode.IsSpace) {
			candidates = append(candidates, strings.ReplaceAll(strings.Join(strings.Fields(rest), " "), " ", "_"))
		}
	}
	candidates = append(candidates, pathCandidates(title)...)
	for _, p := range candidates {
		e, err := ix.store.lookup(p)
		if err != nil {
			if errors.Is(err, zim.ErrClosed) {
				return zim.Entry{}, "", err
			}
			continue
		}
		return ix.resolveArticle(e)
	}
	if title == "" {
		return zim.Entry{}, "", ErrArticleNotFound
	}
	hits, err := ix.titleHits(ctx, title, titleCandidateLimit)
	if err != nil {
		return zim.Entry{}, "", err
	}
	spaced := func(s string) string { return collapseSpace(strings.ReplaceAll(s, "_", " ")) }
	want := spaced(title)
	folded, key := foldText(want), titleKey(title)
	for _, match := range []func(hitTitle string) bool{
		func(t string) bool { return strings.EqualFold(spaced(t), want) },
		func(t string) bool { return foldText(spaced(t)) == folded },
		func(t string) bool { return key != "" && titleKey(t) == key },
	} {
		for _, h := range hits {
			if !match(h.Title) {
				continue
			}
			e, err := ix.store.lookup(h.Path)
			if err != nil {
				if errors.Is(err, zim.ErrClosed) {
					return zim.Entry{}, "", err
				}
				continue
			}
			return ix.resolveArticle(e)
		}
	}
	return zim.Entry{}, "", ErrArticleNotFound
}

func (ix searchIndex) resolveArticle(e zim.Entry) (zim.Entry, string, error) {
	from := ""
	if e.IsRedirect {
		resolved, err := ix.store.resolve(e)
		if err != nil {
			if errors.Is(err, zim.ErrNotFound) || errors.Is(err, zim.ErrRedirectLoop) {
				return zim.Entry{}, "", ErrArticleNotFound
			}
			return zim.Entry{}, "", fmt.Errorf("resolve redirect %q: %w", e.Path, err)
		}
		from, e = e.Title, resolved
	}
	if !isHTMLEntry(e) {
		return zim.Entry{}, "", ErrNotArticle
	}
	return e, from, nil
}

// cleanArticleRef accepts a title or path as the model may echo it from an
// isolated tool result: wrapper tags and HTML entities are removed.
func cleanArticleRef(s string) string {
	return strings.TrimPrefix(cleanEchoedText(s), "/")
}

// cleanEchoedText removes the isolation wrapper tags and HTML entities a
// model may echo back from an isolated tool result.
func cleanEchoedText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<external_data>")
	s = strings.TrimSuffix(s, "</external_data>")
	return strings.TrimSpace(html.UnescapeString(s))
}

// findEchoedSection resolves a section request as a model may echo it from
// an isolated tool result: cleaned of wrapper tags and entities first (with
// a trailing "…" of a shortened heading dropped next), then verbatim, so a
// heading that really contains an entity or the tag text still matches.
// The error is the one of the cleaned request.
func (a *renderedArticle) findEchoedSection(ctx context.Context, spec string) (int, error) {
	cleaned := cleanEchoedText(spec)
	i, err := a.findSection(ctx, cleaned)
	if err == nil || !errors.Is(err, ErrSectionNotFound) {
		return i, err
	}
	for _, alt := range []string{strings.TrimSpace(strings.TrimSuffix(cleaned, "…")), strings.TrimSpace(spec)} {
		if alt == cleaned || alt == "" {
			continue
		}
		if j, altErr := a.findSection(ctx, alt); altErr == nil {
			return j, nil
		} else if !errors.Is(altErr, ErrSectionNotFound) {
			return 0, altErr
		}
	}
	return i, err
}

// rendered returns the rendered article, from the cache when possible. ctx
// is checked before and while rendering, so a read that waited for its slot
// still ends at the search timeout.
func (ix searchIndex) rendered(ctx context.Context, e zim.Entry) (*renderedArticle, error) {
	key := ix.store.cacheKey(e.Path)
	if art, ok := renderCache.get(key); ok {
		return art, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := ix.store.readHTML(e)
	if err != nil {
		return nil, err
	}
	art, err := renderArticleContext(ctx, raw, e.Title)
	if err != nil {
		return nil, err
	}
	renderCache.put(key, art)
	return art, nil
}

// articleCache is a small LRU of rendered articles keyed by edition (ZIM
// UUID and Library instance, see zimStore.cacheKey) and path, bounded by
// entries and by bytes. Library.Close purges the entries of its edition.
type articleCache struct {
	mu       sync.Mutex
	max      int // entries
	maxBytes int
	bytes    int
	keys     []string // least recently used first
	items    map[string]*renderedArticle
}

func newArticleCache(maxEntries, maxBytes int) *articleCache {
	return &articleCache{max: maxEntries, maxBytes: maxBytes, items: map[string]*renderedArticle{}}
}

var renderCache = newArticleCache(renderCacheSize, renderCacheBytes)

func (c *articleCache) get(key string) (*renderedArticle, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	art, ok := c.items[key]
	if ok {
		c.touch(key)
	}
	return art, ok
}

// put caches art unless it is larger than half the byte budget, then evicts
// the least recently used entries until both limits hold.
func (c *articleCache) put(key string, art *renderedArticle) {
	size := art.memSize()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remove(key)
	if size > c.maxBytes/2 {
		return
	}
	c.items[key] = art
	c.keys = append(c.keys, key)
	c.bytes += size
	for len(c.keys) > c.max || c.bytes > c.maxBytes {
		c.remove(c.keys[0])
	}
}

// purge drops every entry whose key starts with prefix.
func (c *articleCache) purge(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range append([]string(nil), c.keys...) {
		if strings.HasPrefix(key, prefix) {
			c.remove(key)
		}
	}
}

// remove drops key if it is cached; c.mu must be held.
func (c *articleCache) remove(key string) {
	art, ok := c.items[key]
	if !ok {
		return
	}
	delete(c.items, key)
	c.bytes -= art.memSize()
	for i, k := range c.keys {
		if k == key {
			c.keys = append(c.keys[:i], c.keys[i+1:]...)
			break
		}
	}
}

// touch moves key to the most recently used end.
func (c *articleCache) touch(key string) {
	for i, k := range c.keys {
		if k == key {
			c.keys = append(c.keys[:i], c.keys[i+1:]...)
			break
		}
	}
	c.keys = append(c.keys, key)
}
