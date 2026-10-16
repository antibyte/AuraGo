package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/bookshelf"
	"aurago/internal/config"
	"aurago/internal/tools"
)

type bookshelfOfflineTestAdapter struct {
	mu       sync.RWMutex
	offline  bool
	content  []byte
	revision string
}

func (a *bookshelfOfflineTestAdapter) List(context.Context, bookshelf.Source, string) ([]bookshelf.Entry, error) {
	return nil, nil
}

func (a *bookshelfOfflineTestAdapter) Stat(_ context.Context, _ bookshelf.Source, name string) (bookshelf.Entry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.offline {
		return bookshelf.Entry{}, bookshelf.ErrSourceUnavailable
	}
	return bookshelf.Entry{Path: name, Name: filepath.Base(name), Format: "epub", Size: int64(len(a.content)), Revision: a.revision}, nil
}

func (a *bookshelfOfflineTestAdapter) Open(context.Context, bookshelf.Source, string) (io.ReadCloser, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.offline {
		return nil, bookshelf.ErrSourceUnavailable
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), a.content...))), nil
}

func (a *bookshelfOfflineTestAdapter) setOffline(value bool) {
	a.mu.Lock()
	a.offline = value
	a.mu.Unlock()
}

func TestBookshelfEffectiveMaxBytesHonorsDesktopLimitAndHardCeiling(t *testing.T) {
	cases := []struct {
		name string
		mb   int
		want int64
	}{
		{name: "desktop default", want: 50 << 20},
		{name: "lower desktop policy", mb: 2, want: 2 << 20},
		{name: "Bookshelf hard ceiling", mb: 512, want: bookshelf.MaxSourceBookBytes},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := &config.Config{VirtualDesktop: config.VirtualDesktopConfig{MaxFileSizeMB: test.mb}}
			if got := bookshelfEffectiveMaxBytes(cfg); got != test.want {
				t.Fatalf("effective limit = %d, want %d", got, test.want)
			}
		})
	}
}

func TestBookshelfStorageErrorsDistinguishRevocationFromOffline(t *testing.T) {
	if err := bookshelfSourceProviderError(tools.ErrStorageUnavailable); !errors.Is(err, bookshelf.ErrSourceUnavailable) {
		t.Fatalf("temporary storage failure = %v, want unavailable marker", err)
	}
	if err := bookshelfSourceProviderError(tools.ErrStorageAccessDenied); !errors.Is(err, bookshelf.ErrSourceRevoked) {
		t.Fatalf("policy denial = %v, want revoked marker", err)
	}
	unknown := errors.New("unexpected provider failure")
	if err := bookshelfSourceProviderError(unknown); errors.Is(err, bookshelf.ErrSourceUnavailable) || errors.Is(err, bookshelf.ErrSourceRevoked) {
		t.Fatalf("unknown provider failure was classified as a cache fallback: %v", err)
	}
}

func TestBookshelfPrepareServesReadOnlyRevisionBoundSnapshots(t *testing.T) {
	cases := []struct {
		name          string
		sourceRev     string
		pin           bool
		offlineAtRead bool
	}{
		{name: "pinned temporary offline fallback", sourceRev: "source-revision-1", pin: true, offlineAtRead: true},
		{name: "unverified source remains read-only", sourceRev: ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			adapter := &bookshelfOfflineTestAdapter{content: []byte("offline-readable-book"), revision: test.sourceRev}
			service, err := bookshelf.New(ctx, bookshelf.Options{
				Root:         filepath.Join(t.TempDir(), "catalog"),
				Adapters:     map[string]bookshelf.SourceAdapter{bookshelf.ProviderWebDAV: adapter},
				MaxBookBytes: 1 << 20, MaxCacheBytes: 4 << 20, MinFreeBytes: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.AddSource(ctx, bookshelf.Source{ID: "remote-books", Name: "Remote books", Provider: bookshelf.ProviderWebDAV, MountID: "reader"}); err != nil {
				_ = service.Close()
				t.Fatal(err)
			}
			book, err := service.AddBook(ctx, "remote-books", bookshelf.Entry{
				Path: "book.epub", Name: "Offline book", Format: "epub", Size: int64(len(adapter.content)), Revision: test.sourceRev,
			}, bookshelf.Metadata{Title: "Offline book", Format: "epub"})
			if err != nil {
				_ = service.Close()
				t.Fatal(err)
			}
			if test.pin {
				if _, err := service.SetCachePinned(ctx, book.ID, book.Revision, true); err != nil {
					_ = service.Close()
					t.Fatal(err)
				}
				book, err = service.GetBook(ctx, book.ID)
				if err != nil || !book.CachePinned {
					_ = service.Close()
					t.Fatalf("pinned book = %#v, %v", book, err)
				}
			}

			cfg := &config.Config{VirtualDesktop: config.VirtualDesktopConfig{Enabled: true, BookshelfEnabled: true, ReadOnly: true, MaxFileSizeMB: 1}}
			server := &Server{Cfg: cfg}
			server.cfgSnapshot.Store(cfg)
			runtime := &desktopBookshelfRuntime{
				service: service, adapter: adapter, importsDir: filepath.Join(t.TempDir(), "imports"),
				jobs: make(map[string]*bookshelfJob), tickets: make(map[string]*bookshelfTicket),
			}
			runtime.maxBookBytes.Store(1 << 20)
			server.bookshelf = runtime
			defer server.shutdownDesktopBookshelf()
			bookBefore, err := service.GetBook(ctx, book.ID)
			if err != nil {
				t.Fatal(err)
			}

			if test.offlineAtRead {
				adapter.setOffline(true)
			}
			prepareRequest := httptest.NewRequest("POST", "/api/desktop/bookshelf/books/"+book.ID+"/prepare", nil)
			prepareResponse := httptest.NewRecorder()
			server.bookshelfPrepare(prepareResponse, prepareRequest, runtime, book.ID)
			if prepareResponse.Code != 200 {
				t.Fatalf("prepare status = %d: %s", prepareResponse.Code, prepareResponse.Body.String())
			}
			var prepared struct {
				ContentURL string `json:"content_url"`
			}
			if err := json.Unmarshal(prepareResponse.Body.Bytes(), &prepared); err != nil {
				t.Fatal(err)
			}
			if prepared.ContentURL == "" {
				t.Fatalf("prepare response did not contain a content URL: %s", prepareResponse.Body.String())
			}
			bookAfter, err := service.GetBook(ctx, book.ID)
			if err != nil {
				t.Fatal(err)
			}
			if bookAfter.Revision != bookBefore.Revision || bookAfter.ContentRevision != bookBefore.ContentRevision {
				t.Fatalf("read-only prepare changed catalog state: before=%#v after=%#v", bookBefore, bookAfter)
			}

			contentRequest := httptest.NewRequest("GET", prepared.ContentURL, nil)
			ticketID := contentRequest.URL.Query().Get("ticket")
			server.bookshelfMu.Lock()
			ticket := runtime.tickets[ticketID]
			server.bookshelfMu.Unlock()
			if ticket == nil {
				t.Fatal("prepared content ticket was not retained")
			}
			initialRemaining := time.Until(ticket.Expires)
			if initialRemaining <= 0 || initialRemaining > bookshelfInitialContentTicketTTL {
				t.Fatalf("unconsumed ticket lifetime = %s, want at most %s", initialRemaining, bookshelfInitialContentTicketTTL)
			}
			// Model the first successful reader request six minutes ago. Its
			// completed transfer renewed the ticket, so later PDF ranges remain valid.
			simulatedIssue := time.Now().Add(-6 * time.Minute)
			ticket.mu.Lock()
			ticket.Expires = simulatedIssue.Add(bookshelfContentTicketIdleTTL)
			server.scheduleBookshelfTicketExpiryLocked(runtime, ticketID, ticket)
			ticket.mu.Unlock()
			if bookshelfTicketExpired(ticket, time.Now()) {
				t.Fatal("content ticket expired six minutes after issue")
			}
			contentRequest.Header.Set("Range", "bytes=2-6")
			contentResponse := httptest.NewRecorder()
			server.bookshelfServeContent(contentResponse, contentRequest, runtime, book.ID)
			if contentResponse.Code != 206 || strings.TrimSpace(contentResponse.Body.String()) != "fline" {
				t.Fatalf("range status/body = %d/%q, want 206/%q", contentResponse.Code, contentResponse.Body.String(), "fline")
			}

			// Drive the ticket close to its idle deadline, then prove a later
			// authorized range request renews the deadline and fences its old timer.
			ticket.mu.Lock()
			ticket.Expires = time.Now().Add(time.Minute)
			oldGeneration := ticket.expiryGeneration
			ticket.mu.Unlock()
			renewalRequest := httptest.NewRequest("GET", prepared.ContentURL, nil)
			renewalRequest.Header.Set("Range", "bytes=0-2")
			renewalResponse := httptest.NewRecorder()
			server.bookshelfServeContent(renewalResponse, renewalRequest, runtime, book.ID)
			if renewalResponse.Code != http.StatusPartialContent || renewalResponse.Body.String() != "off" {
				t.Fatalf("renewal range status/body = %d/%q, want 206/%q", renewalResponse.Code, renewalResponse.Body.String(), "off")
			}
			ticket.mu.Lock()
			remaining := time.Until(ticket.Expires)
			ticket.mu.Unlock()
			if remaining < 23*time.Hour {
				t.Fatalf("successful content request did not renew the idle expiry: remaining=%s", remaining)
			}
			server.expireBookshelfTicket(runtime, ticketID, ticket, oldGeneration)
			if ticket.revoked.Load() || ticket.Lease == nil {
				t.Fatal("stale expiry timer revoked a renewed ticket")
			}

			// Simulate an admitted transfer crossing its idle deadline. Pruning
			// must skip the locked ticket; source revocation must still set its
			// atomic gate before waiting to close the lease.
			ticket.mu.Lock()
			ticket.Expires = time.Now().Add(-time.Second)
			if _, err := ticket.Lease.File.Seek(0, io.SeekStart); err != nil {
				ticket.mu.Unlock()
				t.Fatal(err)
			}
			activeReader := &bookshelfContentReader{ctx: ctx, server: server, ticket: ticket, source: ticket.Lease.File}
			buf := make([]byte, len(adapter.content))
			n, err := activeReader.Read(buf)
			if err != nil || string(buf[:n]) != string(adapter.content) {
				ticket.mu.Unlock()
				t.Fatalf("active transfer failed after idle expiry: read=%q err=%v", buf[:n], err)
			}
			pruned := make(chan []*bookshelfTicket, 1)
			go func() {
				server.bookshelfMu.Lock()
				evicted := pruneBookshelfTickets(runtime)
				server.bookshelfMu.Unlock()
				pruned <- evicted
			}()
			select {
			case evicted := <-pruned:
				if len(evicted) != 0 || ticket.revoked.Load() {
					ticket.mu.Unlock()
					t.Fatalf("pruning evicted an active transfer: evicted=%d revoked=%v", len(evicted), ticket.revoked.Load())
				}
			case <-time.After(time.Second):
				ticket.mu.Unlock()
				<-pruned
				t.Fatal("ticket pruning blocked behind an active transfer")
			}
			revoked := make(chan struct{})
			go func() {
				server.revokeDesktopBookshelfSourceTickets(runtime, "remote-books")
				close(revoked)
			}()
			deadline := time.NewTimer(time.Second)
			for !ticket.revoked.Load() {
				select {
				case <-deadline.C:
					ticket.mu.Unlock()
					<-revoked
					t.Fatal("source revocation could not reach an active transfer")
				default:
					time.Sleep(time.Millisecond)
				}
			}
			deadline.Stop()
			ticket.mu.Unlock()
			<-revoked
			if !ticket.revoked.Load() {
				t.Fatal("source revocation did not invalidate the active ticket")
			}
			if err := (&bookshelfContentReader{ctx: ctx, server: server, ticket: ticket, source: bytes.NewReader(nil)}).check(); !errors.Is(err, bookshelf.ErrSourceRevoked) {
				t.Fatalf("revoked range reader check = %v, want source revoked", err)
			}

			cleanupResponse := httptest.NewRecorder()
			server.bookshelfPrepare(cleanupResponse, httptest.NewRequest("POST", "/api/desktop/bookshelf/books/"+book.ID+"/prepare", nil), runtime, book.ID)
			if cleanupResponse.Code != 200 {
				t.Fatalf("prepare for idle cleanup status = %d: %s", cleanupResponse.Code, cleanupResponse.Body.String())
			}
			var cleanupPrepared struct {
				ContentURL string `json:"content_url"`
			}
			if err := json.Unmarshal(cleanupResponse.Body.Bytes(), &cleanupPrepared); err != nil {
				t.Fatal(err)
			}
			cleanupRequest := httptest.NewRequest("GET", cleanupPrepared.ContentURL, nil)
			cleanupTicketID := cleanupRequest.URL.Query().Get("ticket")
			server.bookshelfMu.Lock()
			cleanupTicket := runtime.tickets[cleanupTicketID]
			server.bookshelfMu.Unlock()
			if cleanupTicket == nil || cleanupTicket.Lease == nil {
				t.Fatal("second prepared ticket did not retain a cache lease")
			}
			leaseFile := cleanupTicket.Lease.File
			cleanupTicket.mu.Lock()
			cleanupTicket.Expires = time.Now().Add(-time.Second)
			generation := cleanupTicket.expiryGeneration
			cleanupTicket.mu.Unlock()
			server.expireBookshelfTicket(runtime, cleanupTicketID, cleanupTicket, generation)
			server.bookshelfMu.Lock()
			remainingTicket := runtime.tickets[cleanupTicketID]
			server.bookshelfMu.Unlock()
			if remainingTicket != nil || !cleanupTicket.revoked.Load() || cleanupTicket.Lease != nil {
				t.Fatal("idle expiry did not revoke and remove the ticket lease without another prepare")
			}
			if _, err := leaseFile.Stat(); err == nil {
				t.Fatal("expired cache lease file remains open")
			}

			busyTickets := make([]*bookshelfTicket, 128)
			server.bookshelfMu.Lock()
			for i := range busyTickets {
				busyTickets[i] = &bookshelfTicket{Expires: time.Now().Add(-time.Hour)}
				busyTickets[i].mu.Lock()
				runtime.tickets[randomBookshelfID()] = busyTickets[i]
			}
			server.bookshelfMu.Unlock()
			capacityResponse := httptest.NewRecorder()
			server.bookshelfPrepare(capacityResponse, httptest.NewRequest("POST", "/api/desktop/bookshelf/books/"+book.ID+"/prepare", nil), runtime, book.ID)
			for _, busy := range busyTickets {
				busy.mu.Unlock()
			}
			if capacityResponse.Code != http.StatusTooManyRequests {
				t.Fatalf("prepare with 128 active tickets returned %d: %s", capacityResponse.Code, capacityResponse.Body.String())
			}
			server.bookshelfMu.Lock()
			runtime.tickets = make(map[string]*bookshelfTicket)
			server.bookshelfMu.Unlock()

			releasePrepareResponse := httptest.NewRecorder()
			server.bookshelfPrepare(releasePrepareResponse, httptest.NewRequest("POST", "/api/desktop/bookshelf/books/"+book.ID+"/prepare", nil), runtime, book.ID)
			if releasePrepareResponse.Code != http.StatusOK {
				t.Fatalf("prepare for explicit release status = %d: %s", releasePrepareResponse.Code, releasePrepareResponse.Body.String())
			}
			var releasePrepared struct {
				ContentURL string `json:"content_url"`
			}
			if err := json.Unmarshal(releasePrepareResponse.Body.Bytes(), &releasePrepared); err != nil {
				t.Fatal(err)
			}
			releaseURL := strings.Replace(releasePrepared.ContentURL, "/content?", "/release?", 1)
			releaseRequest := httptest.NewRequest("POST", releaseURL, nil)
			releaseTicketID := releaseRequest.URL.Query().Get("ticket")
			server.bookshelfMu.Lock()
			releaseTicket := runtime.tickets[releaseTicketID]
			server.bookshelfMu.Unlock()
			if releaseTicket == nil || releaseTicket.Lease == nil {
				t.Fatal("prepared release ticket did not retain a cache lease")
			}
			releaseFile := releaseTicket.Lease.File
			wrongBookRelease := httptest.NewRecorder()
			server.bookshelfBookRoute(wrongBookRelease, releaseRequest, runtime, "different-book/release")
			server.bookshelfMu.Lock()
			stillPresent := runtime.tickets[releaseTicketID] == releaseTicket
			server.bookshelfMu.Unlock()
			if wrongBookRelease.Code != http.StatusNoContent || !stillPresent || releaseTicket.revoked.Load() {
				t.Fatalf("mismatched-book release status=%d present=%v revoked=%v", wrongBookRelease.Code, stillPresent, releaseTicket.revoked.Load())
			}
			releaseResponse := httptest.NewRecorder()
			server.bookshelfBookRoute(releaseResponse, releaseRequest, runtime, book.ID+"/release")
			server.bookshelfMu.Lock()
			stillPresent = runtime.tickets[releaseTicketID] == releaseTicket
			server.bookshelfMu.Unlock()
			if releaseResponse.Code != http.StatusNoContent || stillPresent || !releaseTicket.revoked.Load() || releaseTicket.Lease != nil || releaseTicket.expiryTimer != nil {
				t.Fatalf("ticket release status=%d present=%v revoked=%v lease=%v timer=%v", releaseResponse.Code, stillPresent, releaseTicket.revoked.Load(), releaseTicket.Lease != nil, releaseTicket.expiryTimer != nil)
			}
			if _, err := releaseFile.Stat(); err == nil {
				t.Fatal("released cache lease file remains open")
			}
			repeatRelease := httptest.NewRecorder()
			server.bookshelfBookRoute(repeatRelease, releaseRequest, runtime, book.ID+"/release")
			if repeatRelease.Code != http.StatusNoContent {
				t.Fatalf("repeated release status = %d, want 204", repeatRelease.Code)
			}
		})
	}
}
