package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/bookshelf"
	"aurago/internal/config"
)

func newBookshelfImportFixture(t *testing.T, maxBytes int64) (*desktopBookshelfRuntime, string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	importsDir := filepath.Join(root, "imports")
	if err := os.MkdirAll(importsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	server := &Server{Cfg: &config.Config{VirtualDesktop: config.VirtualDesktopConfig{Enabled: true, BookshelfEnabled: true}}}
	adapter := &desktopBookshelfAdapter{s: server, importsDir: importsDir}
	service, err := bookshelf.New(ctx, bookshelf.Options{
		Root: filepath.Join(root, "catalog"), Adapters: map[string]bookshelf.SourceAdapter{
			bookshelf.ProviderLocal: adapter,
		},
		MaxFilesPerScan: 10, MaxBookBytes: maxBytes, MaxCacheBytes: 4 << 20, MinFreeBytes: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.shutdownDesktopBookshelf)
	if _, err := service.AddSource(ctx, bookshelf.Source{
		ID: bookshelfImportsSourceID, Name: "Bookshelf uploads", Provider: bookshelf.ProviderLocal, MountID: bookshelfImportsMountID,
	}); err != nil {
		t.Fatal(err)
	}
	runtime := &desktopBookshelfRuntime{service: service, adapter: adapter, importsDir: importsDir, jobs: make(map[string]*bookshelfJob), tickets: make(map[string]*bookshelfTicket)}
	runtime.maxBookBytes.Store(maxBytes)
	server.bookshelf = runtime
	return runtime, importsDir
}

func bookshelfImportRequest(t *testing.T, file []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "boundary.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(file); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/bookshelf/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestBookshelfImportAcceptsExactLimitAndReportsOversizedFile(t *testing.T) {
	for _, test := range []struct {
		name       string
		file       []byte
		wantStatus int
	}{
		{name: "exact limit", file: []byte("four"), wantStatus: http.StatusOK},
		{name: "one byte over", file: []byte("fives"), wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, importsDir := newBookshelfImportFixture(t, 4)
			response := httptest.NewRecorder()
			(&Server{}).bookshelfImport(response, bookshelfImportRequest(t, test.file), runtime)
			if response.Code != test.wantStatus {
				t.Fatalf("upload status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			entries, err := os.ReadDir(importsDir)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantStatus == http.StatusOK {
				if len(entries) != 1 || strings.HasSuffix(entries[0].Name(), ".tmp") {
					t.Fatalf("exact-limit upload left unexpected imports: %v", entries)
				}
				return
			}
			if len(entries) != 0 {
				t.Fatalf("over-limit upload left partial imports: %v", entries)
			}
			var body struct {
				Error    string `json:"error"`
				MaxBytes int64  `json:"max_bytes"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error != "limit_exceeded" || body.MaxBytes != 4 {
				t.Fatalf("upload error = %+v, want limit_exceeded at 4 bytes", body)
			}
		})
	}
}

func TestBookshelfImportMultipartOverflowReportsLimitAndMalformedInputIsBadRequest(t *testing.T) {
	runtime, importsDir := newBookshelfImportFixture(t, 4)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "boundary.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("title", strings.Repeat("x", (1<<20)+1024)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/bookshelf/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	(&Server{}).bookshelfImport(response, request, runtime)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("multipart overflow status = %d, want 413: %s", response.Code, response.Body.String())
	}
	var limited struct {
		Error    string `json:"error"`
		MaxBytes int64  `json:"max_bytes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &limited); err != nil {
		t.Fatal(err)
	}
	if limited.Error != "limit_exceeded" || limited.MaxBytes != 4 {
		t.Fatalf("multipart overflow error = %+v, want limit_exceeded at 4 bytes", limited)
	}
	if entries, err := os.ReadDir(importsDir); err != nil || len(entries) != 0 {
		t.Fatalf("multipart overflow left imports %v (err %v)", entries, err)
	}

	malformed := httptest.NewRequest(http.MethodPost, "/api/desktop/bookshelf/imports", strings.NewReader("not multipart"))
	malformed.Header.Set("Content-Type", "application/json")
	malformedResponse := httptest.NewRecorder()
	(&Server{}).bookshelfImport(malformedResponse, malformed, runtime)
	if malformedResponse.Code != http.StatusBadRequest {
		t.Fatalf("malformed input status = %d, want 400: %s", malformedResponse.Code, malformedResponse.Body.String())
	}
	var malformedBody map[string]any
	if err := json.Unmarshal(malformedResponse.Body.Bytes(), &malformedBody); err != nil {
		t.Fatal(err)
	}
	if malformedBody["error"] != "invalid_multipart" {
		t.Fatalf("malformed input error = %v, want invalid_multipart", malformedBody)
	}
}

func TestBookshelfReaderUsesIsolatedVersionedShell(t *testing.T) {
	previous := uiFiles
	uiFiles = os.DirFS("../../ui")
	t.Cleanup(func() { uiFiles = previous })

	cfg := &config.Config{VirtualDesktop: config.VirtualDesktopConfig{Enabled: true, BookshelfEnabled: true}}
	server := &Server{Cfg: cfg}
	request := httptest.NewRequest("GET", "/api/desktop/bookshelf/reader", nil)
	request.Host = "bookshelf.test:8443"
	request.TLS = &tls.ConnectionState{}
	response := httptest.NewRecorder()
	handleDesktopBookshelf(server).ServeHTTP(response, request)

	if response.Code != 200 {
		t.Fatalf("reader status = %d: %s", response.Code, response.Body.String())
	}
	policy := response.Header().Get("Content-Security-Policy")
	for _, required := range []string{
		"script-src https://bookshelf.test:8443/js/desktop/apps/bookshelf/reader-host.js https://bookshelf.test:8443/js/vendor/foliate/reader.bundle.js",
		"connect-src blob:",
		"frame-src data: blob:",
		"frame-ancestors 'self'",
	} {
		if !strings.Contains(policy, required) {
			t.Errorf("reader CSP missing %q: %s", required, policy)
		}
	}
	for _, forbidden := range []string{"script-src 'self'", "connect-src https:", "unsafe-eval", "https://" + request.Host + "/api/"} {
		if strings.Contains(policy, forbidden) {
			t.Errorf("reader CSP contains %q: %s", forbidden, policy)
		}
	}
	if response.Header().Get("Permissions-Policy") == "" {
		t.Fatal("reader response did not deny device APIs")
	}
	body := response.Body.String()
	if !strings.Contains(body, `meta name="bookshelf-version"`) || !strings.Contains(body, "reader-host.js?v=") {
		t.Fatalf("reader shell lacks versioned host asset: %s", body)
	}
	if strings.Contains(body, "reader.bundle.js") {
		t.Fatal("trusted reader shell must delegate the engine to its isolated data frame")
	}
}

func TestBookshelfMutationsRequireExactOriginAndHonorReadOnly(t *testing.T) {
	cases := []struct {
		name       string
		readonly   bool
		origin     string
		wantStatus int
	}{
		{name: "missing origin", readonly: false, wantStatus: 403},
		{name: "wrong origin", readonly: false, origin: "https://evil.test", wantStatus: 403},
		{name: "readonly catalog", readonly: true, origin: "https://bookshelf.test", wantStatus: 403},
		{name: "allowed mutation", readonly: false, origin: "https://bookshelf.test", wantStatus: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := &config.Config{VirtualDesktop: config.VirtualDesktopConfig{Enabled: true, BookshelfEnabled: true, ReadOnly: test.readonly}}
			server := &Server{Cfg: cfg}
			request := httptest.NewRequest("POST", "https://bookshelf.test/api/desktop/bookshelf/books/abc/state", strings.NewReader(`{"state":"reading"}`))
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			allowed := bookshelfAuthorize(server, response, request, true)
			if test.wantStatus == 0 {
				if !allowed {
					t.Fatalf("mutation rejected: %d %s", response.Code, response.Body.String())
				}
				return
			}
			if allowed || response.Code != test.wantStatus {
				t.Fatalf("allowed=%v status=%d, want rejected with %d", allowed, response.Code, test.wantStatus)
			}
		})
	}
}

func TestBookshelfPrepareIsReadOnlyWhileCatalogChangesAreWrites(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/desktop/bookshelf/books/book-1/prepare", nil)
	if bookshelfRouteWrites(request, "/books/book-1/prepare") {
		t.Fatal("preparing a bounded content cache must remain available in desktop read-only mode")
	}
	request = httptest.NewRequest("POST", "/api/desktop/bookshelf/books/book-1/release?ticket=ticket", nil)
	if bookshelfRouteWrites(request, "/books/book-1/release") {
		t.Fatal("releasing a private content lease must remain available in desktop read-only mode")
	}
	if !bookshelfRouteWrites(request, "/books/book-1/convert") {
		t.Fatal("conversion creates a managed rendition and must be a catalog write")
	}
	if !bookshelfRouteWrites(request, "/books/resolve") {
		t.Fatal("explicit local catalog resolution must be treated as a write")
	}
}

func TestBookshelfReleaseRequiresAdminAndExactOriginButAllowsReadOnly(t *testing.T) {
	s, _, adminToken, readerToken := newBearerSchemeTestServer(t)
	s.Cfg.VirtualDesktop.ReadOnly = true
	cases := []struct {
		name       string
		token      string
		origin     string
		wantStatus int
	}{
		{name: "admin can release in read-only mode", token: adminToken, origin: "https://bookshelf.test", wantStatus: http.StatusOK},
		{name: "non-admin cannot release", token: readerToken, origin: "https://bookshelf.test", wantStatus: http.StatusForbidden},
		{name: "missing origin rejected", token: adminToken, wantStatus: http.StatusForbidden},
		{name: "wrong origin rejected", token: adminToken, origin: "https://evil.test", wantStatus: http.StatusForbidden},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://bookshelf.test/api/desktop/bookshelf/books/book-1/release?ticket=ticket", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			allowed := bookshelfAuthorize(s, response, request, bookshelfRouteWrites(request, "/books/book-1/release"))
			if test.wantStatus == http.StatusOK {
				if !allowed {
					t.Fatalf("authorized release rejected: %d %s", response.Code, response.Body.String())
				}
				return
			}
			if allowed || response.Code != test.wantStatus {
				t.Fatalf("release allowed=%v status=%d, want rejected with %d", allowed, response.Code, test.wantStatus)
			}
		})
	}
}

func TestBookshelfCollectionBooksPatchReordersWithRevision(t *testing.T) {
	ctx := context.Background()
	adapter := &bookshelfOfflineTestAdapter{content: []byte("book"), revision: "source-revision"}
	service, err := bookshelf.New(ctx, bookshelf.Options{
		Root: filepath.Join(t.TempDir(), "catalog"),
		Adapters: map[string]bookshelf.SourceAdapter{
			bookshelf.ProviderWebDAV: adapter,
		},
		MaxFilesPerScan: 10, MaxBookBytes: 1 << 20, MaxCacheBytes: 4 << 20, MinFreeBytes: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddSource(ctx, bookshelf.Source{ID: "remote-books", Name: "Remote", Provider: bookshelf.ProviderWebDAV}); err != nil {
		t.Fatal(err)
	}
	first, err := service.AddBook(ctx, "remote-books", bookshelf.Entry{Path: "first.epub", Name: "First", Format: "epub", Size: 4, Revision: "source-revision"}, bookshelf.Metadata{Title: "First", Format: "epub"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.AddBook(ctx, "remote-books", bookshelf.Entry{Path: "second.epub", Name: "Second", Format: "epub", Size: 4, Revision: "source-revision"}, bookshelf.Metadata{Title: "Second", Format: "epub"})
	if err != nil {
		t.Fatal(err)
	}
	collection, err := service.CreateCollection(ctx, "Ordered", 0)
	if err != nil {
		t.Fatal(err)
	}
	collection, err = service.SetCollectionBooks(ctx, collection.ID, collection.Revision, []string{first.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &desktopBookshelfRuntime{service: service}
	server := &Server{}
	body := `{"book_ids":["` + second.ID + `","` + first.ID + `"]}`
	path := "/collections/" + collection.ID + "/books"

	missingRevision := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	missingResponse := httptest.NewRecorder()
	server.bookshelfCollectionRoute(missingResponse, missingRevision, runtime, strings.TrimPrefix(path, "/collections/"))
	if missingResponse.Code != http.StatusConflict {
		t.Fatalf("missing If-Match status = %d, want %d: %s", missingResponse.Code, http.StatusConflict, missingResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	request.Header.Set("If-Match", strconv.FormatInt(collection.Revision, 10))
	response := httptest.NewRecorder()
	server.bookshelfCollectionRoute(response, request, runtime, strings.TrimPrefix(path, "/collections/"))
	if response.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var result struct {
		Collection bookshelf.Collection `json:"collection"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Collection.Revision != collection.Revision+1 {
		t.Fatalf("response collection revision = %d, want %d", result.Collection.Revision, collection.Revision+1)
	}
	got, err := service.CollectionBookIDs(ctx, collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != second.ID || got[1] != first.ID {
		t.Fatalf("persisted order = %v, want [%s %s]", got, second.ID, first.ID)
	}
}
