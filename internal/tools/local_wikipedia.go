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

// WithdrawLocalWikipediaManager clears the published source only while it is
// still the one adapted from m, so a server shutting down after another
// publisher took over (a restart, a test server) leaves the newer source alone.
// The check and the clear happen under one lock.
func WithdrawLocalWikipediaManager(m *localwiki.Manager) {
	if m == nil {
		return
	}
	localWikipediaMu.Lock()
	defer localWikipediaMu.Unlock()
	if cur, ok := localWikipediaSource.(localWikipediaManagerSource); ok && cur.m == m {
		localWikipediaSource = nil
	}
}

// PublishedLocalWikipediaManager returns the manager whose source is
// published to the tool, nil when none is (another source or none at all).
func PublishedLocalWikipediaManager() *localwiki.Manager {
	if cur, ok := currentLocalWikipediaSource().(localWikipediaManagerSource); ok {
		return cur.m
	}
	return nil
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
// OutputBudget is the agent's inline limit in bytes (0 = 6,000): the answer is
// sized to stay below it as the model sees it (see localWikipediaModelBytes).
type LocalWikipediaRequest struct {
	Operation    string
	Query        string
	Limit        int
	Title        string
	Path         string
	Section      string
	Offset       int
	OutputBudget int
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
	budget := localWikipediaBudget(req.OutputBudget)
	switch strings.ToLower(strings.TrimSpace(req.Operation)) {
	case "search":
		return localWikipediaSearch(ctx, lib, req, budget)
	case "read":
		return localWikipediaRead(ctx, lib, req, budget)
	default:
		return localWikipediaError("error", "invalid_request", "operation must be search or read.")
	}
}

// localWikipediaSearch answers a search; leads, snippets and at last results
// shrink until the answer fits budget (see localWikipediaFitSearch).
func localWikipediaSearch(ctx context.Context, lib LocalWikipediaLibrary, req LocalWikipediaRequest, budget int) string {
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
		return localWikipediaErrorFor(err, budget)
	}
	return localWikipediaFitSearch(localWikipediaSearchOutput{
		Status:    "success",
		Operation: "search",
		Edition:   localWikipediaEditionOf(result.Edition),
		Fulltext:  result.Fulltext,
	}, result.Results, budget)
}

// localWikipediaRead answers one page of an article or section. The page is
// sized to budget: the section list gets at most a third of it, and a page
// that does not fit is read again with fewer characters (offsets and
// next_offset stay rune offsets, so pages of any size chain).
func localWikipediaRead(ctx context.Context, lib LocalWikipediaLibrary, req LocalWikipediaRequest, budget int) string {
	if strings.TrimSpace(req.Title) == "" && strings.TrimSpace(req.Path) == "" {
		return localWikipediaError("error", "invalid_request", "read requires title or path; use a path returned by search.")
	}
	if req.Offset < 0 {
		return localWikipediaError("error", "invalid_request", "offset must be 0 or a next_offset value.")
	}
	// Every character costs at least one byte, so a page never needs more
	// characters than the budget has bytes.
	readReq := localwiki.ReadRequest{Title: req.Title, Path: req.Path, Section: req.Section, Offset: req.Offset,
		PageRunes: min(localWikipediaPageRunes, budget)}
	art, err := lib.Read(ctx, readReq)
	if err != nil {
		return localWikipediaErrorFor(err, budget)
	}
	edition := localWikipediaEditionOf(lib.Edition())
	answer := func(art localwiki.Article, sections int) string {
		return localWikipediaJSON(localWikipediaReadOutput{
			Status:                    "success",
			Operation:                 "read",
			Edition:                   edition,
			Title:                     security.IsolateExternalData(art.Title),
			Path:                      security.IsolateExternalData(art.Path),
			RedirectedFrom:            security.IsolateExternalData(art.RedirectedFrom),
			localWikipediaSectionList: localWikipediaSections(art.Sections, sections),
			Content:                   security.IsolateExternalData(art.Content),
			NextOffset:                art.NextOffset,
		})
	}
	sections := min(len(art.Sections), localWikipediaMaxSections)
	if out := answer(art, sections); localWikipediaModelBytes(out) <= budget {
		return out
	}
	frame := art
	frame.Content = ""
	sections = localWikipediaFitSections(sections, budget/3, func(n int) string { return answer(frame, n) })
	base := localWikipediaModelBytes(answer(frame, sections))
	for attempt := 0; ; attempt++ {
		out := answer(art, sections)
		size := localWikipediaModelBytes(out)
		runes := utf8.RuneCountInString(art.Content)
		if size <= budget || attempt == localWikipediaReadAttempts || runes <= localWikipediaMinPageRunes {
			return out
		}
		// Scale the page by the bytes this page cost per character; the margin
		// covers a next part that is denser.
		page := int(float64(runes) * float64(budget-base) / float64(max(size-base, 1)) * 0.95)
		readReq.PageRunes = min(max(page, localWikipediaMinPageRunes), runes-1)
		if art, err = lib.Read(ctx, readReq); err != nil {
			return localWikipediaErrorFor(err, budget)
		}
	}
}

func localWikipediaEditionOf(e localwiki.Edition) localWikipediaEdition {
	return localWikipediaEdition{Language: e.Language, Variant: string(e.Variant), Date: e.Date}
}

// localWikipediaSections lists the first limit sections (at most
// localWikipediaMaxSections) with isolated, length-bounded headings.
func localWikipediaSections(sections []localwiki.Section, limit int) localWikipediaSectionList {
	shown := sections[:max(min(len(sections), localWikipediaMaxSections, limit), 0)]
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
// for a search slot counts as cancelled, not busy. The section list of
// section_not_found is shortened to fit budget.
func localWikipediaErrorFor(err error, budget int) string {
	var sectionErr *localwiki.SectionNotFoundError
	switch {
	case errors.As(err, &sectionErr):
		answer := func(n int) string {
			list := localWikipediaSections(sectionErr.Sections, n)
			if total := max(sectionErr.Total, len(sectionErr.Sections)); total > len(list.Sections) {
				// The library error itself carries only the first sections.
				list.Total, list.Truncated = total, true
			}
			return localWikipediaJSON(localWikipediaFailure{
				Status: "error", Code: "section_not_found",
				Message:  "The article has no such section; use a heading or index from sections.",
				Sections: list.Sections, Total: list.Total, Truncated: list.Truncated,
			})
		}
		return answer(localWikipediaFitSections(min(len(sectionErr.Sections), localWikipediaMaxSections), budget, answer))
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
