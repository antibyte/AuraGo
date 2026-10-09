package server

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/desktop"
	"aurago/internal/localwiki"
	"aurago/internal/zim"
)

const (
	localWikiDesktopPrefix = "/api/desktop/local-wikipedia/"
	localWikiContentPrefix = localWikiDesktopPrefix + "content/"
	localWikiMaxQueryRunes = 200
	localWikiSuggestLimit  = 10
	localWikiSearchDefault = 20
	localWikiSearchMax     = 30
	localWikiQueryTimeout  = 6 * time.Second
)

// localWikipediaContentCSP keeps ZIM documents inert: no script runs, forms
// cannot submit, link pings and beacons cannot reach AuraGo with the session
// (connect-src 'none') and only the Desktop may frame them. allow-same-origin
// keeps the session cookie on image and stylesheet requests (SameSite=Strict
// treats an opaque origin as cross-site) and lets the app read the article
// title. Every content answer carries it: blobs, redirects and error pages.
const localWikipediaContentCSP = "sandbox allow-same-origin allow-popups allow-popups-to-escape-sandbox; default-src 'self'; script-src 'none'; connect-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; form-action 'none'; frame-ancestors 'self'"

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
	route := strings.TrimPrefix(r.URL.Path, localWikiDesktopPrefix)
	content := strings.HasPrefix(route, "content/")
	// Content answers land in the article frame, so their errors are framable HTML.
	fail := func(status int, code, message string) {
		if content {
			writeLocalWikiContentError(w, status, code)
			return
		}
		localWikiDesktopError(w, status, code, message)
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		fail(http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	// Every route reads; the check keeps the shared Desktop admission order.
	if !checkDesktopOperation(s, w, r, desktopRead) {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		fail(http.StatusServiceUnavailable, "desktop_unavailable", "Desktop unavailable")
		return
	}
	if !cfg.LocalWikipedia.Enabled {
		if content {
			writeLocalWikiContentError(w, http.StatusServiceUnavailable, "disabled")
			return
		}
		localWikiDesktopJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Local Wikipedia is switched off", "code": "disabled", "can_manage": localWikiCanManage(s, r)})
		return
	}
	if backend == nil {
		fail(http.StatusServiceUnavailable, "unavailable", "Local Wikipedia is not available")
		return
	}
	if route == "status" {
		s.writeLocalWikiDesktopStatus(w, r, backend.Status())
		return
	}
	if !content && !localWikiReaderRoute(route) {
		localWikiDesktopError(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	reader, release, ok := backend.AcquireReader()
	if !ok {
		fail(http.StatusConflict, "not_ready", "No Wikipedia edition is ready")
		return
	}
	defer release()
	switch {
	case route == "suggest":
		s.serveLocalWikiSuggest(w, r, reader)
	case route == "search":
		s.serveLocalWikiSearch(w, r, reader)
	case route == "random":
		s.serveLocalWikiRef(w, reader.Random)
	case route == "main":
		s.serveLocalWikiRef(w, reader.Main)
	case content:
		s.serveLocalWikiContent(w, r, reader, strings.TrimPrefix(route, "content/"))
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

// serveLocalWikiContent streams one blob of the open edition. Paths are lookup
// keys in the archive's content namespace, never filesystem paths.
func (s *Server) serveLocalWikiContent(w http.ResponseWriter, r *http.Request, reader localWikiReader, path string) {
	item, err := reader.Content(path)
	if err != nil {
		if errors.Is(err, zim.ErrNotFound) || errors.Is(err, zim.ErrRedirectLoop) || errors.Is(err, localwiki.ErrInvalidPath) {
			writeLocalWikiContentError(w, http.StatusNotFound, "not_found")
			return
		}
		s.logLocalWikiDesktop("content failed", err)
		writeLocalWikiContentError(w, http.StatusInternalServerError, "content_failed")
		return
	}
	if item.Path != "" && item.Path != path {
		target, ok := localWikiContentURL(item.Path)
		if !ok {
			writeLocalWikiContentError(w, http.StatusNotFound, "not_found")
			return
		}
		header := w.Header()
		setLocalWikiContentHeaders(header)
		header.Set("Location", target)
		header.Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusFound)
		return
	}
	if item.Reader == nil {
		s.logLocalWikiDesktop("content failed", errors.New("edition returned no reader"))
		writeLocalWikiContentError(w, http.StatusInternalServerError, "content_failed")
		return
	}
	mimeType := item.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	header := w.Header()
	setLocalWikiContentHeaders(header)
	header.Set("Content-Type", mimeType)
	// Overrides the authenticated no-store default: the blob cannot change
	// within an edition, and the ETag carries the edition UUID.
	header.Set("Cache-Control", "private, max-age=86400")
	header.Del("Pragma")
	if item.ETag != "" {
		header.Set("ETag", item.ETag)
	}
	http.ServeContent(w, r, "", time.Time{}, item.Reader)
}

// setLocalWikiContentHeaders sets the headers every content answer carries.
func setLocalWikiContentHeaders(header http.Header) {
	header.Set("Content-Security-Policy", localWikipediaContentCSP)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
}

// localWikiContentURL builds the browser address of a content path. Dot
// segments are refused: browsers would normalise them out of the content route.
func localWikiContentURL(path string) (string, bool) {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "." || part == ".." {
			return "", false
		}
		parts[i] = url.PathEscape(part)
	}
	return localWikiContentPrefix + strings.Join(parts, "/"), true
}

// writeLocalWikiContentError answers inside the article frame with an inert page
// whose marker tells the Desktop app what went wrong.
func writeLocalWikiContentError(w http.ResponseWriter, status int, code string) {
	header := w.Header()
	setLocalWikiContentHeaders(header)
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	escaped := html.EscapeString(code)
	_, _ = io.WriteString(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="aurago-local-wikipedia-error" content="`+escaped+`"><title>`+escaped+`</title></head><body></body></html>`)
}

// localWikipediaAvailable reports whether the Desktop shows the Wikipedia app:
// the integration is switched on and its manager exists (like flowsAvailable,
// the app never shows when the manager could not be built).
func (s *Server) localWikipediaAvailable() bool {
	if s == nil || s.LocalWiki == nil {
		return false
	}
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.LocalWikipedia.Enabled
}

// publishLocalWikipediaAvailability tells open desktops that the Wikipedia app
// appeared or disappeared, so start menus refresh at once.
func (s *Server) publishLocalWikipediaAvailability(available bool) {
	if s == nil {
		return
	}
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{
		"operation": "app_availability", "app_id": "local-wikipedia", "available": available,
	}, CreatedAt: time.Now().UTC()})
}

// announceLocalWikipediaAvailability runs after a config publication that
// flipped local_wikipedia.enabled. Publications usually hold CfgMu, so the
// broadcast (DesktopMu, the Desktop hub and the SSE broadcaster) runs on its
// own goroutine and announces what the snapshot current by then grants.
func (s *Server) announceLocalWikipediaAvailability() {
	if s == nil || s.LocalWiki == nil {
		return
	}
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		// Nothing else recovers a panic on this goroutine.
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[LocalWikipedia] Recovered from a panic while announcing the Desktop app",
					"panic", r, "stack", string(debug.Stack()))
			}
		}()
		s.publishLocalWikipediaAvailability(s.localWikipediaAvailable())
	}()
}
