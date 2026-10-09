package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/localwiki"
	"aurago/internal/zim"
)

const (
	localWikiDesktopPrefix = "/api/desktop/local-wikipedia/"
	localWikiMaxQueryRunes = 200
	localWikiSuggestLimit  = 10
	localWikiSearchDefault = 20
	localWikiSearchMax     = 30
	localWikiQueryTimeout  = 6 * time.Second
)

// localWikiReader is what the Desktop API needs from an open edition.
type localWikiReader interface {
	Edition() localwiki.Edition
	Fulltext() bool
	Search(ctx context.Context, query string, limit int, withLeads int) (localwiki.SearchResult, error)
	Suggest(ctx context.Context, query string, limit int) ([]localwiki.Ref, error)
	Content(path string) (localwiki.ContentItem, error)
	Random() (localwiki.Ref, error)
	Main() (localwiki.Ref, error)
}

var _ localWikiReader = (*localwiki.Library)(nil)

// localWikiDesktopBackend is the manager as the Desktop API sees it; tests replace it.
type localWikiDesktopBackend interface {
	Status() localwiki.Status
	AcquireReader() (localWikiReader, func(), bool)
}

type localWikiManagerBackend struct{ manager *localwiki.Manager }

func (b localWikiManagerBackend) Status() localwiki.Status { return b.manager.Status() }

// AcquireReader takes a refcounted handle on the open edition; callers must
// release it. It fails while the storage directory is first loaded and when
// no readable edition is installed.
func (b localWikiManagerBackend) AcquireReader() (localWikiReader, func(), bool) {
	lib, release, ok := b.manager.Acquire()
	if !ok {
		return nil, func() {}, false
	}
	if release == nil {
		release = func() {}
	}
	if lib == nil {
		release()
		return nil, func() {}, false
	}
	return lib, release, true
}

// localWikiDesktopEdition is the edition as non-admin Desktop users see it:
// no file names, hashes, UUIDs or sizes.
type localWikiDesktopEdition struct {
	Language     string            `json:"language"`
	Variant      localwiki.Variant `json:"variant"`
	Date         string            `json:"date"`
	ArticleCount int               `json:"article_count"`
}

func localWikiProjectEdition(edition localwiki.Edition) *localWikiDesktopEdition {
	return &localWikiDesktopEdition{Language: edition.Language, Variant: edition.Variant, Date: edition.Date, ArticleCount: edition.ArticleCount}
}

// localWikiDesktopStatus is the non-admin subset of localwiki.Status; paths,
// free space, selections, languages and the English recommendation stay on
// the admin API. Readable and Loading are what the app decides on: readable
// means an edition is served (in every state), loading means the storage
// directory is still being loaded and the status should be polled.
type localWikiDesktopStatus struct {
	State           string                   `json:"state"`
	Progress        float64                  `json:"progress"`
	Edition         *localWikiDesktopEdition `json:"edition"`
	Fulltext        bool                     `json:"fulltext"`
	Readable        bool                     `json:"readable"`
	Loading         bool                     `json:"loading"`
	UpdateAvailable *localwiki.UpdateInfo    `json:"update_available"`
	ErrorCode       string                   `json:"error_code,omitempty"`
	CanManage       bool                     `json:"can_manage"`
}

func registerLocalWikipediaDesktopRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/local-wikipedia/", s.handleLocalWikipediaDesktop)
}

func (s *Server) handleLocalWikipediaDesktop(w http.ResponseWriter, r *http.Request) {
	var backend localWikiDesktopBackend
	if s != nil && s.LocalWiki != nil {
		backend = localWikiManagerBackend{manager: s.LocalWiki}
	}
	s.serveLocalWikipediaDesktop(w, r, backend)
}

// localWikiCanManage reports whether the caller may use the admin config API:
// browser sessions are the admin identity, bearer tokens need the admin scope.
func localWikiCanManage(s *Server, r *http.Request) bool {
	return go2RTCRequestIsAdmin(s, r)
}

func localWikiDesktopJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func localWikiDesktopError(w http.ResponseWriter, status int, code, message string) {
	localWikiDesktopJSON(w, status, map[string]string{"error": message, "code": code})
}

func (s *Server) logLocalWikiDesktop(message string, err error) {
	if s != nil && s.Logger != nil {
		s.Logger.Warn("[LocalWikipedia] desktop "+message, "error", err)
	}
}

// localWikiReaderRoute reports whether route needs the open edition.
func localWikiReaderRoute(route string) bool {
	switch route {
	case "suggest", "search", "random", "main":
		return true
	}
	return false
}

func (s *Server) serveLocalWikipediaDesktop(w http.ResponseWriter, r *http.Request, backend localWikiDesktopBackend) {
	if !authenticateDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		localWikiDesktopError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	// Every route reads; the check keeps the shared Desktop admission order.
	if !checkDesktopOperation(s, w, r, desktopRead) {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		localWikiDesktopError(w, http.StatusServiceUnavailable, "desktop_unavailable", "Desktop unavailable")
		return
	}
	if !cfg.LocalWikipedia.Enabled {
		localWikiDesktopJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Local Wikipedia is switched off", "code": "disabled", "can_manage": localWikiCanManage(s, r)})
		return
	}
	if backend == nil {
		localWikiDesktopError(w, http.StatusServiceUnavailable, "unavailable", "Local Wikipedia is not available")
		return
	}
	route := strings.TrimPrefix(r.URL.Path, localWikiDesktopPrefix)
	if route == "status" {
		s.writeLocalWikiDesktopStatus(w, r, backend.Status())
		return
	}
	if !localWikiReaderRoute(route) {
		localWikiDesktopError(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	reader, release, ok := backend.AcquireReader()
	if !ok {
		localWikiDesktopError(w, http.StatusConflict, "not_ready", "No Wikipedia edition is ready")
		return
	}
	defer release()
	switch route {
	case "suggest":
		s.serveLocalWikiSuggest(w, r, reader)
	case "search":
		s.serveLocalWikiSearch(w, r, reader)
	case "random":
		s.serveLocalWikiRef(w, reader.Random)
	case "main":
		s.serveLocalWikiRef(w, reader.Main)
	}
}

func (s *Server) writeLocalWikiDesktopStatus(w http.ResponseWriter, r *http.Request, status localwiki.Status) {
	out := localWikiDesktopStatus{
		State:           status.State,
		Progress:        status.Progress,
		Fulltext:        status.Fulltext,
		Readable:        status.Readable,
		Loading:         status.Loading,
		UpdateAvailable: status.UpdateAvailable,
		ErrorCode:       status.ErrorCode,
		CanManage:       localWikiCanManage(s, r),
	}
	if status.Edition != nil {
		out.Edition = localWikiProjectEdition(*status.Edition)
	}
	localWikiDesktopJSON(w, http.StatusOK, out)
}

func localWikiQuery(r *http.Request) (string, bool) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	return query, utf8.RuneCountInString(query) <= localWikiMaxQueryRunes
}

func (s *Server) serveLocalWikiSuggest(w http.ResponseWriter, r *http.Request, reader localWikiReader) {
	query, ok := localWikiQuery(r)
	if !ok {
		localWikiDesktopError(w, http.StatusBadRequest, "query_too_long", "The search text is too long")
		return
	}
	if query == "" {
		localWikiDesktopJSON(w, http.StatusOK, map[string]any{"results": []localwiki.Ref{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), localWikiQueryTimeout)
	defer cancel()
	refs, err := reader.Suggest(ctx, query, localWikiSuggestLimit)
	if err != nil {
		s.localWikiQueryError(w, err)
		return
	}
	if refs == nil {
		refs = []localwiki.Ref{}
	}
	if len(refs) > localWikiSuggestLimit {
		refs = refs[:localWikiSuggestLimit]
	}
	localWikiDesktopJSON(w, http.StatusOK, map[string]any{"results": refs})
}

func (s *Server) serveLocalWikiSearch(w http.ResponseWriter, r *http.Request, reader localWikiReader) {
	query, ok := localWikiQuery(r)
	if !ok {
		localWikiDesktopError(w, http.StatusBadRequest, "query_too_long", "The search text is too long")
		return
	}
	limit := localWikiSearchDefault
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			localWikiDesktopError(w, http.StatusBadRequest, "bad_limit", "limit must be a positive number")
			return
		}
		limit = min(n, localWikiSearchMax)
	}
	edition := localWikiProjectEdition(reader.Edition())
	if query == "" {
		localWikiDesktopJSON(w, http.StatusOK, map[string]any{"edition": edition, "fulltext": reader.Fulltext(), "results": []localwiki.SearchHit{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), localWikiQueryTimeout)
	defer cancel()
	// No leads: the Desktop shows snippets and opens the article itself.
	result, err := reader.Search(ctx, query, limit, 0)
	if err != nil {
		s.localWikiQueryError(w, err)
		return
	}
	hits := result.Results
	if hits == nil {
		hits = []localwiki.SearchHit{}
	}
	for i := range hits {
		hits[i].Lead = ""
	}
	localWikiDesktopJSON(w, http.StatusOK, map[string]any{"edition": edition, "fulltext": result.Fulltext, "results": hits})
}

func (s *Server) localWikiQueryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, localwiki.ErrQueryTooLong):
		// Slice 4 also caps queries at 16 words.
		localWikiDesktopError(w, http.StatusBadRequest, "query_too_long", "The search text is too long")
	case errors.Is(err, localwiki.ErrQueryEmpty):
		localWikiDesktopError(w, http.StatusBadRequest, "query_empty", "The search text is empty")
	case errors.Is(err, localwiki.ErrSearchBusy):
		// Every search slot stayed taken; the error also wraps the context
		// error, so it is checked before the timeout.
		w.Header().Set("Retry-After", "1")
		localWikiDesktopError(w, http.StatusServiceUnavailable, "busy", "Too many searches are running")
	case errors.Is(err, context.DeadlineExceeded):
		localWikiDesktopError(w, http.StatusGatewayTimeout, "timeout", "The search took too long")
	case errors.Is(err, context.Canceled):
		localWikiDesktopError(w, http.StatusServiceUnavailable, "request_cancelled", "The request was cancelled")
	default:
		s.logLocalWikiDesktop("search failed", err)
		localWikiDesktopError(w, http.StatusInternalServerError, "search_failed", "The search failed")
	}
}

func (s *Server) serveLocalWikiRef(w http.ResponseWriter, pick func() (localwiki.Ref, error)) {
	ref, err := pick()
	switch {
	case err == nil:
		localWikiDesktopJSON(w, http.StatusOK, ref)
	case errors.Is(err, zim.ErrNotFound):
		localWikiDesktopError(w, http.StatusNotFound, "not_found", "No article available")
	default:
		s.logLocalWikiDesktop("article lookup failed", err)
		localWikiDesktopError(w, http.StatusInternalServerError, "lookup_failed", "The article could not be found")
	}
}
