package localwiki

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"

	"aurago/internal/zim"
)

// renderCacheSize is the number of rendered articles kept for paging.
const renderCacheSize = 8

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
	art, err := ix.rendered(e)
	if err != nil {
		return Article{}, err
	}
	text := art.fullMarkdown()
	if strings.TrimSpace(req.Section) != "" {
		i, err := art.findSection(req.Section)
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
// reports the title of a followed redirect. A closed archive (an edition swap
// in progress) is an error of its own, not a missing article.
func (ix searchIndex) findArticle(ctx context.Context, path, title string) (zim.Entry, string, error) {
	path, title = cleanArticleRef(path), cleanArticleRef(title)
	var candidates []string
	if path != "" {
		candidates = append(candidates, path)
		if rest, ok := strings.CutPrefix(path, "C/"); ok && rest != "" {
			candidates = append(candidates, rest)
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
	if title != "" {
		hits, err := ix.titleHits(ctx, title, titleCandidateLimit)
		if err != nil {
			return zim.Entry{}, "", err
		}
		key := titleKey(title)
		for _, h := range hits {
			if titleKey(h.Title) != key {
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
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<external_data>")
	s = strings.TrimSuffix(s, "</external_data>")
	s = strings.TrimSpace(html.UnescapeString(s))
	return strings.TrimPrefix(s, "/")
}

// rendered returns the rendered article, from the cache when possible.
func (ix searchIndex) rendered(e zim.Entry) (*renderedArticle, error) {
	key := ix.store.cacheKey(e.Path)
	if art, ok := renderCache.get(key); ok {
		return art, nil
	}
	raw, err := ix.store.readHTML(e)
	if err != nil {
		return nil, err
	}
	art, err := renderArticle(raw, e.Title)
	if err != nil {
		return nil, err
	}
	renderCache.put(key, art)
	return art, nil
}

// articleCache is a small LRU of rendered articles keyed by ZIM UUID and path.
type articleCache struct {
	mu    sync.Mutex
	max   int
	keys  []string
	items map[string]*renderedArticle
}

var renderCache = &articleCache{max: renderCacheSize, items: map[string]*renderedArticle{}}

func (c *articleCache) get(key string) (*renderedArticle, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	art, ok := c.items[key]
	if ok {
		c.touch(key)
	}
	return art, ok
}

func (c *articleCache) put(key string, art *renderedArticle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok && len(c.keys) >= c.max {
		oldest := c.keys[0]
		c.keys = c.keys[1:]
		delete(c.items, oldest)
	}
	c.items[key] = art
	c.touch(key)
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
