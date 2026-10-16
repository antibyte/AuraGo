package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"aurago/internal/bookshelf"
)

const bookshelfContentTicketIdleTTL = 24 * time.Hour
const bookshelfInitialContentTicketTTL = 5 * time.Minute

func (s *Server) bookshelfListBooks(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	query := r.URL.Query()
	offset, _ := strconv.Atoi(query.Get("offset"))
	limit, _ := strconv.Atoi(query.Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if offset > 1_000_000 {
		offset = 1_000_000
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	page, err := runtime.service.Query(r.Context(), bookshelf.Query{Q: query.Get("q"), SourceID: query.Get("source_id"), CollectionID: query.Get("collection_id"), Status: query.Get("status"), Sort: query.Get("sort"), Offset: offset, Limit: limit})
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, page)
}

func (s *Server) bookshelfResolveGet(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeBookshelfError(w, bookshelf.ErrInvalidPath)
		return
	}
	book, err := runtime.service.FindLocal(r.Context(), bookshelfWorkspaceSourceID, p)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"book": book})
}

func (s *Server) bookshelfResolvePost(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
		writeBookshelfError(w, err)
		return
	}
	if body.Path == "" {
		writeBookshelfError(w, bookshelf.ErrInvalidPath)
		return
	}
	book, err := runtime.service.ResolveLocal(r.Context(), bookshelfWorkspaceSourceID, body.Path)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"book": book})
}

func (s *Server) bookshelfBookRoute(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, rest string) {
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	id, err := url.PathUnescape(parts[0])
	if err != nil {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			book, err := runtime.service.GetBook(r.Context(), id)
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			writeJSON(w, map[string]any{"book": book})
		case http.MethodPatch:
			var override bookshelf.MetadataOverride
			if err := decodeDesktopJSON(w, r, &override, desktopSmallJSONBodyLimit); err != nil {
				writeBookshelfError(w, err)
				return
			}
			revision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			book, err := runtime.service.UpdateMetadataOverride(r.Context(), id, revision, override)
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			writeJSON(w, map[string]any{"book": book})
		case http.MethodDelete:
			book, err := runtime.service.GetBook(r.Context(), id)
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			if expected, parseErr := parseBookshelfRevision(r.Header.Get("If-Match")); parseErr == nil && expected != book.Revision {
				writeBookshelfError(w, bookshelf.ErrConflict)
				return
			}
			if err := runtime.service.RemoveBook(r.Context(), id, book.Revision); err != nil {
				writeBookshelfError(w, err)
				return
			}
			writeJSON(w, map[string]bool{"removed": true})
		default:
			w.Header().Set("Allow", "GET, PATCH, DELETE")
			writeBookshelfErrorStatus(w, "method_not_allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "state":
			if r.Method == http.MethodPatch {
				var body struct {
					State string `json:"state"`
				}
				if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
					writeBookshelfError(w, err)
					return
				}
				revision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				book, err := runtime.service.SetReadingState(r.Context(), id, revision, body.State)
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				writeJSON(w, map[string]any{"book": book})
				return
			}
		case "cache":
			if r.Method == http.MethodPatch {
				var body struct {
					Pinned bool `json:"pinned"`
				}
				if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
					writeBookshelfError(w, err)
					return
				}
				revision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				info, err := runtime.service.SetCachePinned(r.Context(), id, revision, body.Pinned)
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				writeJSON(w, info)
				return
			}
		case "enrich":
			if r.Method == http.MethodPost {
				book, err := s.enrichBookshelfMetadata(r.Context(), runtime, id)
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				writeJSON(w, map[string]any{"book": book})
				return
			}
		case "convert":
			if r.Method == http.MethodPost {
				s.bookshelfStartConversion(w, r, runtime, id)
				return
			}
		case "prepare":
			if r.Method != http.MethodPost {
				break
			}
			s.bookshelfPrepare(w, r, runtime, id)
			return
		case "release":
			if r.Method != http.MethodPost {
				break
			}
			s.bookshelfReleaseTicket(w, r, runtime, id)
			return
		case "content":
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				s.bookshelfServeContent(w, r, runtime, id)
				return
			}
		case "cover":
			if r.Method == http.MethodGet {
				s.bookshelfServeCover(w, r, runtime, id)
				return
			}
			if r.Method == http.MethodPut {
				s.bookshelfSetCover(w, r, runtime, id)
				return
			}
			if r.Method == http.MethodDelete {
				book, err := runtime.service.GetBook(r.Context(), id)
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				if _, err := runtime.service.ClearCover(r.Context(), id, book.Revision); err != nil {
					writeBookshelfError(w, err)
					return
				}
				writeJSON(w, map[string]bool{"removed": true})
				return
			}
		case "progress":
			if r.Method == http.MethodPatch {
				s.bookshelfSaveProgress(w, r, runtime, id)
				return
			}
		case "annotations":
			if r.Method == http.MethodGet {
				annotations, err := runtime.service.ListAnnotations(r.Context(), id)
				if err != nil {
					writeBookshelfError(w, err)
					return
				}
				writeJSON(w, map[string]any{"annotations": annotations})
				return
			}
			if r.Method == http.MethodPost {
				s.bookshelfCreateAnnotation(w, r, runtime, id)
				return
			}
		}
	}
	if len(parts) == 3 && parts[1] == "annotations" {
		if r.Method == http.MethodGet {
			s.bookshelfGetAnnotation(w, r, runtime, id, parts[2])
			return
		}
	}
	writeBookshelfErrorStatus(w, "not_found", http.StatusNotFound)
}

func (s *Server) bookshelfPrepare(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	book, err := runtime.service.GetBook(r.Context(), id)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	source, err := runtime.service.GetSource(r.Context(), book.SourceID)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	sourceGeneration := source.Generation
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = book.Format
	}
	if format != book.Format && format != "epub" {
		writeBookshelfError(w, bookshelf.ErrUnsupported)
		return
	}
	var lease *bookshelf.Lease
	baseRevision := ""
	if format == "epub" && book.Format != "epub" {
		for _, rendition := range book.Renditions {
			if rendition.Format != "epub" || !rendition.Available {
				continue
			}
			lease, err = runtime.service.OpenRendition(r.Context(), id, rendition.BaseRevision, "epub")
			if err == nil {
				baseRevision = rendition.BaseRevision
				break
			}
		}
		if lease == nil && err == nil {
			err = bookshelf.ErrNotFound
		}
	} else {
		cfg := s.ConfigSnapshot()
		if cfg != nil && cfg.VirtualDesktop.ReadOnly {
			lease, err = runtime.service.PrepareReadOnly(r.Context(), id)
		} else {
			lease, err = runtime.service.Prepare(r.Context(), id)
		}
		if err == nil {
			baseRevision = lease.Revision
		}
	}
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	if lease.Size <= 0 || lease.Size > runtime.maxBookBytes.Load() {
		_ = lease.Close()
		writeBookshelfError(w, bookshelf.ErrLimitExceeded)
		return
	}
	book, err = runtime.service.GetBook(r.Context(), id)
	if err != nil {
		_ = lease.Close()
		writeBookshelfError(w, err)
		return
	}
	source, err = runtime.service.GetSource(r.Context(), book.SourceID)
	if err != nil {
		_ = lease.Close()
		writeBookshelfError(w, err)
		return
	}
	if source.Generation != sourceGeneration {
		_ = lease.Close()
		writeBookshelfError(w, bookshelf.ErrStaleRendition)
		return
	}
	ticketID := randomBookshelfID()
	ticket := &bookshelfTicket{BookID: id, SourceID: book.SourceID, SourceGeneration: source.Generation, Path: book.Path, Size: lease.Size, SourceSize: book.Size, BookRevision: book.Revision, CatalogRevision: book.ContentRevision, Revision: lease.Revision, BaseRevision: baseRevision, Format: bookshelfContentType(format), Pinned: book.CachePinned, Expires: time.Now().Add(bookshelfInitialContentTicketTTL), Lease: lease}
	s.bookshelfMu.Lock()
	if s.bookshelf != runtime || runtime.closed {
		s.bookshelfMu.Unlock()
		_ = lease.Close()
		writeBookshelfError(w, errBookshelfDisabled)
		return
	}
	evicted := pruneBookshelfTickets(runtime)
	if len(runtime.tickets) >= 128 {
		s.bookshelfMu.Unlock()
		closeBookshelfTickets(evicted)
		_ = lease.Close()
		writeBookshelfErrorStatus(w, "limit_exceeded", http.StatusTooManyRequests)
		return
	}
	runtime.tickets[ticketID] = ticket
	ticket.mu.Lock()
	s.scheduleBookshelfTicketExpiryLocked(runtime, ticketID, ticket)
	ticket.mu.Unlock()
	s.bookshelfMu.Unlock()
	closeBookshelfTickets(evicted)
	writeJSON(w, map[string]any{"content_url": "/api/desktop/bookshelf/books/" + url.PathEscape(id) + "/content?ticket=" + url.QueryEscape(ticketID), "revision": lease.Revision, "format": format, "size": lease.Size, "book_revision": book.Revision})
}

func (s *Server) bookshelfReleaseTicket(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, bookID string) {
	ticketID := strings.TrimSpace(r.URL.Query().Get("ticket"))
	if ticketID != "" {
		s.bookshelfMu.Lock()
		ticket := runtime.tickets[ticketID]
		if ticket != nil && ticket.BookID == bookID {
			ticket.revoked.Store(true)
			delete(runtime.tickets, ticketID)
		} else {
			ticket = nil
		}
		s.bookshelfMu.Unlock()
		if ticket != nil {
			closeBookshelfTickets([]*bookshelfTicket{ticket})
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// enrichBookshelfMetadata performs bounded archive parsing only after an
// explicit write-authorized request. Preparing a reader never changes catalog
// metadata, which keeps content reads available under global read-only mode.
func (s *Server) enrichBookshelfMetadata(ctx context.Context, runtime *desktopBookshelfRuntime, bookID string) (bookshelf.Book, error) {
	book, err := runtime.service.GetBook(ctx, bookID)
	if err != nil {
		return bookshelf.Book{}, err
	}
	if !bookshelfContainsString([]string{"epub", "fb2", "fb2.zip", "cbz"}, strings.ToLower(book.Format)) {
		return bookshelf.Book{}, bookshelf.ErrUnsupported
	}
	lease, err := runtime.service.Prepare(ctx, bookID)
	if err != nil {
		return bookshelf.Book{}, err
	}
	defer lease.Close()
	if lease.Size <= 0 || lease.Size > runtime.maxBookBytes.Load() {
		return bookshelf.Book{}, bookshelf.ErrLimitExceeded
	}
	extracted, err := bookshelf.ExtractMetadata(ctx, lease.File, lease.Size, book.Format)
	if err != nil {
		return bookshelf.Book{}, err
	}
	book, err = runtime.service.GetBook(ctx, bookID)
	if err != nil {
		return bookshelf.Book{}, err
	}
	book, err = runtime.service.UpdateDiscoveredMetadata(ctx, bookID, book.Revision, extracted.Metadata)
	if err != nil {
		return bookshelf.Book{}, err
	}
	if len(extracted.EmbeddedCover) > 0 && book.CoverURL == "" {
		book, err = runtime.service.SetCover(ctx, bookID, book.Revision, extracted.CoverMIME, bytes.NewReader(extracted.EmbeddedCover))
		if err != nil {
			return bookshelf.Book{}, err
		}
	}
	return book, nil
}

// pruneBookshelfTickets requires Server.bookshelfMu and returns leases to close
// after releasing that global map lock.
func pruneBookshelfTickets(runtime *desktopBookshelfRuntime) []*bookshelfTicket {
	var evicted []*bookshelfTicket
	now := time.Now()
	for id, ticket := range runtime.tickets {
		if !ticket.mu.TryLock() {
			continue
		}
		if ticket.revoked.Load() || ticket.Lease == nil || now.After(ticket.Expires) {
			ticket.revoked.Store(true)
			stopBookshelfTicketExpiryLocked(ticket)
			evicted = append(evicted, ticket)
			delete(runtime.tickets, id)
		}
		ticket.mu.Unlock()
	}
	for len(runtime.tickets) >= 128 {
		removed := false
		for id, ticket := range runtime.tickets {
			if !ticket.mu.TryLock() {
				continue
			}
			ticket.revoked.Store(true)
			stopBookshelfTicketExpiryLocked(ticket)
			evicted = append(evicted, ticket)
			delete(runtime.tickets, id)
			ticket.mu.Unlock()
			removed = true
			break
		}
		if !removed {
			break
		}
	}
	return evicted
}

func stopBookshelfTicketExpiryLocked(ticket *bookshelfTicket) {
	ticket.expiryGeneration++
	if ticket.expiryTimer != nil {
		ticket.expiryTimer.Stop()
		ticket.expiryTimer = nil
	}
}

// scheduleBookshelfTicketExpiryLocked requires ticket.mu and fences callbacks
// from timers that raced with a successful content request.
func (s *Server) scheduleBookshelfTicketExpiryLocked(runtime *desktopBookshelfRuntime, id string, ticket *bookshelfTicket) {
	if ticket.expiryTimer != nil {
		ticket.expiryTimer.Stop()
	}
	ticket.expiryGeneration++
	generation := ticket.expiryGeneration
	ticket.expiryTimer = time.AfterFunc(time.Until(ticket.Expires), func() {
		s.expireBookshelfTicket(runtime, id, ticket, generation)
	})
}

func (s *Server) expireBookshelfTicket(runtime *desktopBookshelfRuntime, id string, ticket *bookshelfTicket, generation uint64) {
	ticket.mu.Lock()
	if ticket.expiryGeneration != generation || ticket.revoked.Load() || ticket.Lease == nil {
		ticket.mu.Unlock()
		return
	}
	if time.Now().Before(ticket.Expires) {
		s.scheduleBookshelfTicketExpiryLocked(runtime, id, ticket)
		ticket.mu.Unlock()
		return
	}
	ticket.revoked.Store(true)
	stopBookshelfTicketExpiryLocked(ticket)
	_ = ticket.Lease.Close()
	ticket.Lease = nil
	ticket.mu.Unlock()

	s.bookshelfMu.Lock()
	if runtime.tickets[id] == ticket {
		delete(runtime.tickets, id)
	}
	s.bookshelfMu.Unlock()
}

func bookshelfTicketExpired(ticket *bookshelfTicket, now time.Time) bool {
	ticket.mu.Lock()
	defer ticket.mu.Unlock()
	return ticket.revoked.Load() || ticket.Lease == nil || now.After(ticket.Expires)
}

func (s *Server) bookshelfServeContent(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, bookID string) {
	ticketID := r.URL.Query().Get("ticket")
	s.bookshelfMu.Lock()
	ticket := runtime.tickets[ticketID]
	s.bookshelfMu.Unlock()
	if ticket == nil || ticket.BookID != bookID || bookshelfTicketExpired(ticket, time.Now()) {
		writeBookshelfErrorStatus(w, "content_ticket_expired", http.StatusGone)
		return
	}
	book, err := runtime.service.GetBook(r.Context(), bookID)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	source, err := runtime.service.GetSource(r.Context(), ticket.SourceID)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	if source.Generation != ticket.SourceGeneration || book.SourceID != ticket.SourceID || book.Path != ticket.Path || book.ContentRevision != ticket.CatalogRevision || ticket.SourceSize != book.Size {
		writeBookshelfError(w, bookshelf.ErrStaleRendition)
		return
	}
	if ticket.CatalogRevision == "" && ticket.BaseRevision != "" && strings.HasPrefix(ticket.BaseRevision, "unverified:") && book.Revision != ticket.BookRevision {
		writeBookshelfError(w, bookshelf.ErrStaleRendition)
		return
	}
	adapter := runtime.adapter
	if adapter == nil {
		adapter = &desktopBookshelfAdapter{s: s, importsDir: runtime.importsDir}
	}
	entry, err := adapter.Stat(r.Context(), source, ticket.Path)
	if errors.Is(err, bookshelf.ErrSourceUnavailable) && ticket.Pinned && book.CachePinned && source.Generation == ticket.SourceGeneration && ticket.BaseRevision == book.ContentRevision {
		// Reuse only a pinned snapshot after the adapter has reported a temporary
		// availability failure. Revocation and unclassified errors stay blocked.
		err = nil
	} else if err != nil {
		writeBookshelfError(w, err)
		return
	} else if entry.Size != ticket.SourceSize || entry.Revision != "" && entry.Revision != ticket.BaseRevision || entry.Revision == "" && !strings.HasPrefix(ticket.BaseRevision, "unverified:") {
		writeBookshelfError(w, bookshelf.ErrStaleRendition)
		return
	}
	ticket.mu.Lock()
	defer ticket.mu.Unlock()
	now := time.Now()
	if ticket.revoked.Load() || ticket.BookID != bookID || now.After(ticket.Expires) || ticket.Lease == nil || !s.bookshelfEnabled() {
		writeBookshelfErrorStatus(w, "content_ticket_expired", http.StatusGone)
		return
	}
	w.Header().Set("Content-Type", ticket.Format)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": path.Base(ticket.Path)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, path.Base(ticket.Path), time.Time{}, &bookshelfContentReader{ctx: r.Context(), server: s, ticket: ticket, source: ticket.Lease.File})
	if r.Context().Err() == nil && !ticket.revoked.Load() && s.bookshelfEnabled() {
		ticket.Expires = time.Now().Add(bookshelfContentTicketIdleTTL)
		s.scheduleBookshelfTicketExpiryLocked(runtime, ticketID, ticket)
	}
}

type bookshelfContentReader struct {
	ctx    context.Context
	server *Server
	ticket *bookshelfTicket
	source io.ReadSeeker
}

func (r *bookshelfContentReader) check() error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if r.ticket == nil || r.ticket.revoked.Load() || r.server == nil || !r.server.bookshelfEnabled() {
		return bookshelf.ErrSourceRevoked
	}
	return nil
}

func (r *bookshelfContentReader) Read(p []byte) (int, error) {
	if err := r.check(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}

func (r *bookshelfContentReader) Seek(offset int64, whence int) (int64, error) {
	if err := r.check(); err != nil {
		return 0, err
	}
	return r.source.Seek(offset, whence)
}

func (s *Server) bookshelfServeCover(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	file, contentType, err := runtime.service.OpenCover(r.Context(), id)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'")
	http.ServeContent(w, r, "cover", info.ModTime(), file)
}

func (s *Server) bookshelfSetCover(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, 13<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeBookshelfError(w, bookshelf.ErrLimitExceeded)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("cover")
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	defer file.Close()
	book, err := runtime.service.GetBook(r.Context(), id)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	if revision, parseErr := parseBookshelfRevision(r.Header.Get("If-Match")); parseErr == nil && revision != book.Revision {
		writeBookshelfError(w, bookshelf.ErrConflict)
		return
	}
	updated, err := runtime.service.SetCover(r.Context(), id, book.Revision, header.Header.Get("Content-Type"), file)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"book": updated})
}

func (s *Server) bookshelfSaveProgress(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	var body struct {
		RenditionRevision string          `json:"rendition_revision"`
		Locator           json.RawMessage `json:"locator"`
		Fraction          float64         `json:"fraction"`
	}
	if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
		writeBookshelfError(w, err)
		return
	}
	revision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	progress, err := runtime.service.SaveProgress(r.Context(), id, revision, body.RenditionRevision, body.Locator, body.Fraction)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"progress": progress})
}

func (s *Server) bookshelfCreateAnnotation(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, bookID string) {
	var body struct {
		Kind              string          `json:"kind"`
		RenditionRevision string          `json:"rendition_revision"`
		Locator           json.RawMessage `json:"locator"`
		Text              string          `json:"text"`
		Color             string          `json:"color"`
	}
	if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
		writeBookshelfError(w, err)
		return
	}
	annotation, err := runtime.service.CreateAnnotationForRendition(r.Context(), bookID, body.Kind, body.RenditionRevision, body.Locator, body.Text, body.Color)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"annotation": annotation})
}

func (s *Server) bookshelfGetAnnotation(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, bookID, annotationID string) {
	annotations, err := runtime.service.ListAnnotations(r.Context(), bookID)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	for _, annotation := range annotations {
		if annotation.ID == annotationID {
			writeJSON(w, map[string]any{"annotation": annotation})
			return
		}
	}
	writeBookshelfError(w, bookshelf.ErrNotFound)
}

func (s *Server) bookshelfAnnotationRoute(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	parts := strings.Split(strings.Trim(id, "/"), "/")
	if len(parts) != 1 || parts[0] == "" {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	annotationID, err := url.PathUnescape(parts[0])
	if err != nil {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	if r.Method == http.MethodDelete {
		revision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		if err := runtime.service.DeleteAnnotation(r.Context(), annotationID, revision); err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]bool{"removed": true})
		return
	}
	if r.Method == http.MethodPatch {
		var body struct {
			Locator json.RawMessage `json:"locator"`
			Text    string          `json:"text"`
			Color   string          `json:"color"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			writeBookshelfError(w, err)
			return
		}
		revision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		if len(body.Locator) == 0 {
			body.Locator = json.RawMessage("null")
		}
		annotation, err := runtime.service.UpdateAnnotation(r.Context(), annotationID, revision, body.Locator, body.Text, body.Color)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]any{"annotation": annotation})
		return
	}
	writeBookshelfErrorStatus(w, "method_not_allowed", http.StatusMethodNotAllowed)
}
