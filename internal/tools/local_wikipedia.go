package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/security"
	"aurago/internal/zim"
)

const (
	// localWikipediaLeads is the number of search results that carry their lead.
	localWikipediaLeads = 3
	// The tool offers 1-10 results, 5 by default; the library allows more for the Desktop.
	localWikipediaDefaultLimit = 5
	localWikipediaMaxLimit     = 10
	// localWikipediaMaxSections bounds the section list of one answer: an
	// article can have hundreds of thousands of headings. Later sections stay
	// readable by index; sections_total tells how many there are.
	localWikipediaMaxSections = 100
	// localWikipediaHeadingRunes bounds one listed heading (including the "…").
	localWikipediaHeadingRunes = 200
)

// LocalWikipediaLibrary is the read-only view of an open edition that the
// local_wikipedia tool uses; *localwiki.Library implements it.
type LocalWikipediaLibrary interface {
	Edition() localwiki.Edition
	Fulltext() bool
	Search(ctx context.Context, query string, limit int, withLeads int) (localwiki.SearchResult, error)
	Read(ctx context.Context, req localwiki.ReadRequest) (localwiki.Article, error)
}

// LocalWikipediaSource hands out refcounted handles on the open edition.
// When ok is false no edition is open and release must not be called.
type LocalWikipediaSource interface {
	AcquireLibrary() (lib LocalWikipediaLibrary, release func(), ok bool)
}

var (
	localWikipediaMu     sync.RWMutex
	localWikipediaSource LocalWikipediaSource
)

// SetLocalWikipediaSource publishes the server-owned edition source to the
// agent tool; nil withdraws it (shutdown, tests).
func SetLocalWikipediaSource(src LocalWikipediaSource) {
	localWikipediaMu.Lock()
	defer localWikipediaMu.Unlock()
	localWikipediaSource = src
}

func currentLocalWikipediaSource() LocalWikipediaSource {
	localWikipediaMu.RLock()
	defer localWikipediaMu.RUnlock()
	return localWikipediaSource
}

// LocalWikipediaManagerSource adapts the server-owned manager; nil stays nil.
func LocalWikipediaManagerSource(m *localwiki.Manager) LocalWikipediaSource {
	if m == nil {
		return nil
	}
	return localWikipediaManagerSource{m: m}
}

type localWikipediaManagerSource struct{ m *localwiki.Manager }

// AcquireLibrary turns the manager's no-op release of a failed Acquire
// (disabled, loading, nothing readable) into nil, as the interface promises.
func (s localWikipediaManagerSource) AcquireLibrary() (LocalWikipediaLibrary, func(), bool) {
	lib, release, ok := s.m.Acquire()
	if !ok || lib == nil {
		return nil, nil, false
	}
	return lib, release, true
}

// LocalWikipediaAvailable reports whether an edition is open for the tool.
func LocalWikipediaAvailable() bool {
	src := currentLocalWikipediaSource()
	if src == nil {
		return false
	}
	_, release, ok := src.AcquireLibrary()
	if ok {
		release()
	}
	return ok
}

// LocalWikipediaRequest is a decoded local_wikipedia tool call. Offset counts
// characters (Unicode code points) of the selected Markdown, as next_offset does.
type LocalWikipediaRequest struct {
	Operation string
	Query     string
	Limit     int
	Title     string
	Path      string
	Section   string
	Offset    int
}

type localWikipediaEdition struct {
	Language string `json:"language"`
	Variant  string `json:"variant"`
	Date     string `json:"date"`
}

type localWikipediaHit struct {
	Title   string `json:"title"`
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
	Lead    string `json:"lead,omitempty"`
}

type localWikipediaSearchOutput struct {
	Status    string                `json:"status"`
	Operation string                `json:"operation"`
	Edition   localWikipediaEdition `json:"edition"`
	Fulltext  bool                  `json:"fulltext"`
	Results   []localWikipediaHit   `json:"results"`
}

type localWikipediaSection struct {
	Index   int    `json:"index"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	Chars   int    `json:"chars"`
}

// localWikipediaSectionList is the bounded section list of an answer;
// Total and Truncated are only set when entries were left out.
type localWikipediaSectionList struct {
	Sections  []localWikipediaSection `json:"sections"`
	Total     int                     `json:"sections_total,omitempty"`
	Truncated bool                    `json:"sections_truncated,omitempty"`
}

type localWikipediaReadOutput struct {
	Status         string                `json:"status"`
	Operation      string                `json:"operation"`
	Edition        localWikipediaEdition `json:"edition"`
	Title          string                `json:"title"`
	Path           string                `json:"path"`
	RedirectedFrom string                `json:"redirected_from,omitempty"`
	localWikipediaSectionList
	Content    string `json:"content"`
	NextOffset *int   `json:"next_offset"`
}

type localWikipediaFailure struct {
	Status    string                  `json:"status"`
	Code      string                  `json:"code"`
	Message   string                  `json:"message"`
	Sections  []localWikipediaSection `json:"sections,omitempty"`
	Total     int                     `json:"sections_total,omitempty"`
	Truncated bool                    `json:"sections_truncated,omitempty"`
}

// ExecuteLocalWikipedia runs the read-only local_wikipedia tool. The status
// envelope is trusted local data; every article text field is isolated as
// external data.
func ExecuteLocalWikipedia(ctx context.Context, cfg *config.Config, req LocalWikipediaRequest) string {
	if cfg == nil || !cfg.LocalWikipedia.Enabled || !cfg.LocalWikipedia.AgentAccess {
		return localWikipediaError("policy_denied", "local_wikipedia_disabled", "Local Wikipedia is not enabled for agent access.")
	}
	src := currentLocalWikipediaSource()
	if src == nil {
		return localWikipediaNotInstalled()
	}
	lib, release, ok := src.AcquireLibrary()
	if !ok {
		return localWikipediaNotInstalled()
	}
	defer release()
	switch strings.ToLower(strings.TrimSpace(req.Operation)) {
	case "search":
		return localWikipediaSearch(ctx, lib, req)
	case "read":
		return localWikipediaRead(ctx, lib, req)
	default:
		return localWikipediaError("error", "invalid_request", "operation must be search or read.")
	}
}

func localWikipediaSearch(ctx context.Context, lib LocalWikipediaLibrary, req LocalWikipediaRequest) string {
	if strings.TrimSpace(req.Query) == "" {
		return localWikipediaError("error", "invalid_request", "search requires query: key terms or a likely article title.")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = localWikipediaDefaultLimit
	}
	limit = min(limit, localWikipediaMaxLimit)
	result, err := lib.Search(ctx, req.Query, limit, localWikipediaLeads)
	if err != nil {
		return localWikipediaErrorFor(err)
	}
	out := localWikipediaSearchOutput{
		Status:    "success",
		Operation: "search",
		Edition:   localWikipediaEditionOf(result.Edition),
		Fulltext:  result.Fulltext,
		Results:   make([]localWikipediaHit, 0, len(result.Results)),
	}
	for _, hit := range result.Results {
		out.Results = append(out.Results, localWikipediaHit{
			Title:   security.IsolateExternalData(hit.Title),
			Path:    security.IsolateExternalData(hit.Path),
			Snippet: security.IsolateExternalData(hit.Snippet),
			Lead:    security.IsolateExternalData(hit.Lead),
		})
	}
	return localWikipediaJSON(out)
}

func localWikipediaRead(ctx context.Context, lib LocalWikipediaLibrary, req LocalWikipediaRequest) string {
	if strings.TrimSpace(req.Title) == "" && strings.TrimSpace(req.Path) == "" {
		return localWikipediaError("error", "invalid_request", "read requires title or path; use a path returned by search.")
	}
	if req.Offset < 0 {
		return localWikipediaError("error", "invalid_request", "offset must be 0 or a next_offset value.")
	}
	art, err := lib.Read(ctx, localwiki.ReadRequest{Title: req.Title, Path: req.Path, Section: req.Section, Offset: req.Offset})
	if err != nil {
		return localWikipediaErrorFor(err)
	}
	return localWikipediaJSON(localWikipediaReadOutput{
		Status:                    "success",
		Operation:                 "read",
		Edition:                   localWikipediaEditionOf(lib.Edition()),
		Title:                     security.IsolateExternalData(art.Title),
		Path:                      security.IsolateExternalData(art.Path),
		RedirectedFrom:            security.IsolateExternalData(art.RedirectedFrom),
		localWikipediaSectionList: localWikipediaSections(art.Sections),
		Content:                   security.IsolateExternalData(art.Content),
		NextOffset:                art.NextOffset,
	})
}

func localWikipediaEditionOf(e localwiki.Edition) localWikipediaEdition {
	return localWikipediaEdition{Language: e.Language, Variant: string(e.Variant), Date: e.Date}
}

// localWikipediaSections lists at most localWikipediaMaxSections sections
// with isolated, length-bounded headings.
func localWikipediaSections(sections []localwiki.Section) localWikipediaSectionList {
	shown := sections[:min(len(sections), localWikipediaMaxSections)]
	list := localWikipediaSectionList{Sections: make([]localWikipediaSection, 0, len(shown))}
	for _, s := range shown {
		list.Sections = append(list.Sections, localWikipediaSection{
			Index:   s.Index,
			Heading: security.IsolateExternalData(localWikipediaShortHeading(s.Heading)),
			Level:   s.Level,
			Chars:   s.Chars,
		})
	}
	if len(shown) < len(sections) {
		list.Total, list.Truncated = len(sections), true
	}
	return list
}

func localWikipediaShortHeading(heading string) string {
	if utf8.RuneCountInString(heading) <= localWikipediaHeadingRunes {
		return heading
	}
	runes := []rune(heading)[:localWikipediaHeadingRunes-1]
	return strings.TrimRight(string(runes), " ") + "…"
}

// localWikipediaErrorFor maps library errors to fixed local messages; error
// text from the archive never reaches the model. A cancellation while waiting
// for a search slot counts as cancelled, not busy.
func localWikipediaErrorFor(err error) string {
	var sectionErr *localwiki.SectionNotFoundError
	switch {
	case errors.As(err, &sectionErr):
		list := localWikipediaSections(sectionErr.Sections)
		return localWikipediaJSON(localWikipediaFailure{
			Status: "error", Code: "section_not_found",
			Message:  "The article has no such section; use a heading or index from sections.",
			Sections: list.Sections, Total: list.Total, Truncated: list.Truncated,
		})
	case errors.Is(err, localwiki.ErrSectionNotFound):
		return localWikipediaError("error", "section_not_found", "The article has no such section; read it without section to list its sections.")
	case errors.Is(err, localwiki.ErrQueryEmpty), errors.Is(err, localwiki.ErrQueryTooLong):
		return localWikipediaError("error", "invalid_request", "query must have 1 to 200 characters and at most 16 words; use key terms or a likely article title.")
	case errors.Is(err, localwiki.ErrOffsetOutOfRange):
		return localWikipediaError("error", "invalid_request", "offset is past the end; use next_offset from the previous page.")
	case errors.Is(err, localwiki.ErrArticleNotFound):
		return localWikipediaError("error", "article_not_found", "No article has this title or path; search first and read a returned path.")
	case errors.Is(err, context.Canceled):
		return localWikipediaError("cancelled", "cancelled", "The local Wikipedia request was cancelled.")
	case errors.Is(err, localwiki.ErrSearchBusy):
		return localWikipediaError("error", "busy", "Too many local Wikipedia requests are running; retry shortly.")
	case errors.Is(err, context.DeadlineExceeded):
		return localWikipediaError("error", "timeout", "The local Wikipedia request timed out; use fewer or more specific terms.")
	case errors.Is(err, localwiki.ErrNotArticle), errors.Is(err, localwiki.ErrArticleTooLarge):
		return localWikipediaError("error", "article_unavailable", "This entry cannot be read as an article.")
	case errors.Is(err, zim.ErrClosed):
		return localWikipediaError("error", "edition_unavailable", "The local Wikipedia edition is being replaced or closed; retry shortly.")
	default:
		return localWikipediaError("error", "local_wikipedia_failed", "The local Wikipedia edition could not answer this request.")
	}
}

func localWikipediaNotInstalled() string {
	return localWikipediaError("needs_setup", "local_wikipedia_not_installed", "No local Wikipedia edition is open: none is installed, or it is still loading; an administrator can install one under Config > Local Wikipedia.")
}

func localWikipediaError(status, code, message string) string {
	return localWikipediaJSON(localWikipediaFailure{Status: status, Code: code, Message: message})
}

// localWikipediaJSON encodes without HTML escaping so the isolation tags stay
// readable; IsolateExternalData has already escaped the article text.
func localWikipediaJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return `Tool Output: {"status":"error","code":"local_wikipedia_failed","message":"result could not be encoded"}`
	}
	return "Tool Output: " + strings.TrimRight(buf.String(), "\n")
}
