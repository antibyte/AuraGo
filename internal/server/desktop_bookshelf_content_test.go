package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
			contentRequest.Header.Set("Range", "bytes=2-6")
			contentResponse := httptest.NewRecorder()
			server.bookshelfServeContent(contentResponse, contentRequest, runtime, book.ID)
			if contentResponse.Code != 206 || strings.TrimSpace(contentResponse.Body.String()) != "fline" {
				t.Fatalf("range status/body = %d/%q, want 206/%q", contentResponse.Code, contentResponse.Body.String(), "fline")
			}

			var ticket *bookshelfTicket
			for _, current := range runtime.tickets {
				ticket = current
			}
			server.revokeDesktopBookshelfSourceTickets(runtime, "remote-books")
			if ticket == nil || !ticket.revoked.Load() {
				t.Fatal("source revocation did not invalidate the active ticket")
			}
			if err := (&bookshelfContentReader{ctx: ctx, server: server, ticket: ticket, source: bytes.NewReader(nil)}).check(); !errors.Is(err, bookshelf.ErrSourceRevoked) {
				t.Fatalf("revoked range reader check = %v, want source revoked", err)
			}
		})
	}
}
