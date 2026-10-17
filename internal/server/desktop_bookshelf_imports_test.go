package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"aurago/internal/bookshelf"
)

func TestBookshelfImportedPDFPreparesAndServesRangesWithoutLosingProgress(t *testing.T) {
	ctx := context.Background()
	runtime, importsDir := newBookshelfImportFixture(t, 1<<20)
	adapter := runtime.adapter.(*desktopBookshelfAdapter)
	server := adapter.s
	content := []byte("%PDF-1.7\nexisting uploaded book\n%%EOF")
	response := httptest.NewRecorder()
	server.bookshelfImport(response, bookshelfImportRequest(t, content), runtime)
	if response.Code != http.StatusOK {
		t.Fatalf("import = %d: %s", response.Code, response.Body.String())
	}
	var imported struct {
		Book bookshelf.Book `json:"book"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	book := imported.Book
	digest := sha256.Sum256(content)
	if book.ContentRevision != "sha256:"+hex.EncodeToString(digest[:]) {
		t.Fatalf("upload revision = %q, want original SHA-256 scheme", book.ContentRevision)
	}
	if _, err := runtime.service.SaveProgress(ctx, book.ID, 0, book.ContentRevision, json.RawMessage(`{"page":2}`), 0.5); err != nil {
		t.Fatal(err)
	}
	// A fresh adapter models a restart with a catalog from the earlier uploader.
	adapter.importHashes = nil
	preparedResponse := httptest.NewRecorder()
	server.bookshelfPrepare(preparedResponse, httptest.NewRequest(http.MethodPost, "/prepare", nil), runtime, book.ID)
	if preparedResponse.Code != http.StatusOK {
		t.Fatalf("prepare uploaded PDF = %d: %s", preparedResponse.Code, preparedResponse.Body.String())
	}
	var prepared struct {
		ContentURL string `json:"content_url"`
	}
	if err := json.Unmarshal(preparedResponse.Body.Bytes(), &prepared); err != nil {
		t.Fatal(err)
	}
	for _, byteRange := range []string{"bytes=0-3", "bytes=4-7"} {
		request := httptest.NewRequest(http.MethodGet, prepared.ContentURL, nil)
		request.Header.Set("Range", byteRange)
		response := httptest.NewRecorder()
		server.bookshelfServeContent(response, request, runtime, book.ID)
		if response.Code != http.StatusPartialContent || response.Body.Len() != 4 {
			t.Fatalf("range %s = %d: %s", byteRange, response.Code, response.Body.String())
		}
	}
	scan, err := runtime.service.Scan(ctx, bookshelfImportsSourceID)
	if err != nil || !scan.Complete || scan.Updated != 0 {
		t.Fatalf("unchanged import scan = %+v, %v", scan, err)
	}
	after, err := runtime.service.GetBook(ctx, book.ID)
	if err != nil || after.ContentRevision != book.ContentRevision || after.Progress == nil || after.Progress.Fraction != 0.5 {
		t.Fatalf("import lost revision/progress: %+v, %v", after, err)
	}
	// A same-size edit must invalidate the cached fingerprint and active ticket.
	filename := filepath.Join(importsDir, book.Path)
	changed := append([]byte(nil), content...)
	changed[10] ^= 1
	if err := os.WriteFile(filename, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	modified := time.Now().Add(time.Minute)
	if err := os.Chtimes(filename, modified, modified); err != nil {
		t.Fatal(err)
	}
	stale := httptest.NewRecorder()
	server.bookshelfServeContent(stale, httptest.NewRequest(http.MethodGet, prepared.ContentURL, nil), runtime, book.ID)
	if stale.Code != http.StatusConflict {
		t.Fatalf("changed import served = %d: %s", stale.Code, stale.Body.String())
	}
	if _, err := runtime.service.Prepare(ctx, book.ID); !errors.Is(err, bookshelf.ErrStaleRendition) {
		t.Fatalf("changed import prepare = %v", err)
	}
}

func TestBookshelfImportFingerprintIsBoundedAndReused(t *testing.T) {
	runtime, importsDir := newBookshelfImportFixture(t, 1<<20)
	adapter := runtime.adapter.(*desktopBookshelfAdapter)
	filename := filepath.Join(importsDir, "book.pdf")
	if err := os.WriteFile(filename, []byte("book"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry, err := statBookshelfImport(importsDir, "book.pdf")
	if err != nil {
		t.Fatal(err)
	}
	first, err := adapter.importEntry(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	// A cancelled read proves a cache hit does not read the full book again.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	again, err := adapter.importEntry(ctx, entry)
	if err != nil || again.Revision != first.Revision {
		t.Fatalf("cached fingerprint = %+v, %v", again, err)
	}
	oldInfo, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(importsDir, "replacement.pdf")
	if err := os.WriteFile(replacement, []byte("edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, oldInfo.ModTime(), oldInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filename); err != nil {
		t.Fatal(err)
	}
	replaced, err := adapter.importEntry(context.Background(), entry)
	if err != nil || replaced.Revision == first.Revision {
		t.Fatalf("replacement with identical metadata reused old fingerprint: %+v, %v", replaced, err)
	}
	adapter.importHashes = nil
	if _, err := adapter.importEntry(ctx, entry); !errors.Is(err, context.Canceled) {
		t.Fatalf("uncached fingerprint ignored cancellation: %v", err)
	}
	adapter.importHashes = make(map[string]bookshelfImportHash)
	for i := 0; i < 256; i++ {
		adapter.importHashes[strconv.Itoa(i)] = bookshelfImportHash{}
	}
	if _, err := adapter.importEntry(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	if len(adapter.importHashes) != 256 || adapter.importHashes[entry.Path].revision == "" {
		t.Fatal("fingerprint cache exceeded its bound or discarded unrelated entries")
	}
	entry.Size = bookshelf.MaxSourceBookBytes + 1
	if _, err := adapter.importEntry(context.Background(), entry); !errors.Is(err, bookshelf.ErrLimitExceeded) {
		t.Fatalf("oversized fingerprint = %v", err)
	}
}
