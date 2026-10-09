package localwiki

import (
	"errors"
	"fmt"
)

// Ref names an article by its display title and its path in the content namespace.
type Ref struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// SearchHit is one search result. Lead is set only for the first withLeads results.
type SearchHit struct {
	Ref
	Snippet string `json:"snippet"`
	Lead    string `json:"lead,omitempty"`
}

// SearchResult is the answer of Library.Search.
type SearchResult struct {
	Edition  Edition     `json:"edition"`
	Fulltext bool        `json:"fulltext"`
	Results  []SearchHit `json:"results"`
}

// ReadRequest selects an article by Path (preferred) or Title, optionally one
// section (heading text or decimal index) and a rune offset into the content.
type ReadRequest struct {
	Title   string
	Path    string
	Section string
	Offset  int
}

// Section describes one heading of a rendered article. Index 0 is the lead.
type Section struct {
	Index   int    `json:"index"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	Chars   int    `json:"chars"`
}

// Article is one page of a rendered article.
type Article struct {
	Ref
	RedirectedFrom string    `json:"redirected_from,omitempty"`
	Sections       []Section `json:"sections"`
	Content        string    `json:"content"`
	NextOffset     *int      `json:"next_offset"`
}

var (
	ErrQueryEmpty       = errors.New("localwiki: query is empty")
	ErrQueryTooLong     = errors.New("localwiki: query is too long")
	ErrSearchBusy       = errors.New("localwiki: too many concurrent searches")
	ErrArticleNotFound  = errors.New("localwiki: article not found")
	ErrNotArticle       = errors.New("localwiki: entry is not an HTML article")
	ErrArticleTooLarge  = errors.New("localwiki: article is too large")
	ErrSectionNotFound  = errors.New("localwiki: section not found")
	ErrOffsetOutOfRange = errors.New("localwiki: offset out of range")
)

// SectionNotFoundError reports an unknown section together with the sections
// the article has, so callers can offer valid choices. Sections holds at most
// the first 100; Total is the article's section count (0 when unknown, then
// len(Sections) counts).
type SectionNotFoundError struct {
	Section  string
	Sections []Section
	Total    int
}

func (e *SectionNotFoundError) Error() string {
	return fmt.Sprintf("localwiki: section %q not found", e.Section)
}

func (e *SectionNotFoundError) Unwrap() error { return ErrSectionNotFound }
