package bookshelf

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

type memoryAdapter struct {
	mu           sync.Mutex
	directories  map[string][]Entry
	listErrors   map[string]error
	files        map[string][]byte
	revisions    map[string]string
	opens        int
	changeOnOpen bool
	statError    error
	statCalls    map[string]int
}

func newMemoryAdapter() *memoryAdapter {
	return &memoryAdapter{
		directories: make(map[string][]Entry), listErrors: make(map[string]error),
		files: make(map[string][]byte), revisions: make(map[string]string), statCalls: make(map[string]int),
	}
}

func (a *memoryAdapter) List(_ context.Context, _ Source, dir string) ([]Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.listErrors[dir]; err != nil {
		return nil, err
	}
	return append([]Entry(nil), a.directories[dir]...), nil
}

func (a *memoryAdapter) Stat(_ context.Context, _ Source, path string) (Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.statError != nil {
		return Entry{}, a.statError
	}
	data, ok := a.files[path]
	if !ok {
		return Entry{}, ErrNotFound
	}
	a.statCalls[path]++
	return Entry{Path: path, Format: normalizeFormat("", path), Size: int64(len(data)), Revision: a.revisions[path]}, nil
}

func (a *memoryAdapter) Open(_ context.Context, _ Source, path string) (io.ReadCloser, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, ok := a.files[path]
	if !ok {
		return nil, ErrNotFound
	}
	a.opens++
	if a.changeOnOpen {
		a.changeOnOpen = false
		a.revisions[path] = "changed-during-read"
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

func (a *memoryAdapter) setFile(path string, data []byte, revision string) Entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.files[path] = append([]byte(nil), data...)
	a.revisions[path] = revision
	return Entry{Path: path, Format: normalizeFormat("", path), Size: int64(len(data)), Revision: revision}
}

func (a *memoryAdapter) openCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.opens
}

func (a *memoryAdapter) setStatError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.statError = err
}

func newTestService(t *testing.T, adapter SourceAdapter, maxCache int64) *Service {
	t.Helper()
	svc, err := New(context.Background(), Options{
		Root: t.TempDir(), Adapters: map[string]SourceAdapter{ProviderLocal: adapter},
		MaxFilesPerScan: 100, MaxBookBytes: 1 << 20, MaxCacheBytes: maxCache, MinFreeBytes: 1,
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func addTestSource(t *testing.T, svc *Service, recursive bool) Source {
	t.Helper()
	source, err := svc.AddSource(context.Background(), Source{Name: "Library", Provider: ProviderLocal, Recursive: recursive})
	if err != nil {
		t.Fatalf("AddSource(): %v", err)
	}
	return source
}

func TestScanPartialFailurePreservesUnseenBooks(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	first := adapter.setFile("first.epub", []byte("first"), "1")
	old := adapter.setFile("sub/old.pdf", []byte("old"), "1")
	adapter.directories[""] = []Entry{first, {Path: "sub", IsDir: true}}
	adapter.directories["sub"] = []Entry{old}
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, true)

	initial, err := svc.Scan(ctx, source.ID)
	if err != nil || !initial.Complete || initial.Added != 2 {
		t.Fatalf("initial Scan() = %#v, %v", initial, err)
	}
	oldBook, err := svc.GetBook(ctx, bookID(source.ID, "sub/old.pdf"))
	if err != nil {
		t.Fatal(err)
	}

	adapter.listErrors["sub"] = errors.New("temporary network error")
	partial, err := svc.Scan(ctx, source.ID)
	if err != nil || partial.Complete || len(partial.Issues) == 0 {
		t.Fatalf("partial Scan() = %#v, %v", partial, err)
	}
	stillThere, err := svc.GetBook(ctx, oldBook.ID)
	if err != nil || !stillThere.Available {
		t.Fatalf("partial scan lost existing book: %#v, %v", stillThere, err)
	}

	delete(adapter.listErrors, "sub")
	adapter.directories["sub"] = nil
	complete, err := svc.Scan(ctx, source.ID)
	if err != nil || !complete.Complete || complete.Removed != 1 {
		t.Fatalf("complete Scan() = %#v, %v", complete, err)
	}
	missing, err := svc.GetBook(ctx, oldBook.ID)
	if err != nil || missing.Available {
		t.Fatalf("complete scan did not mark missing book: %#v, %v", missing, err)
	}
}

func TestPrepareCachesValidatedReadOnlySnapshot(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("immutable-book-content"), "revision-one")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "A Book"})
	if err != nil {
		t.Fatal(err)
	}

	first, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	body, err := io.ReadAll(first.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "immutable-book-content" || first.Revision != "revision-one" {
		t.Fatalf("unexpected prepared snapshot: body=%q revision=%q", body, first.Revision)
	}
	preparedPath := first.Path
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatalf("second Prepare(): %v", err)
	}
	defer second.Close()
	if second.Path != preparedPath || adapter.openCount() != 1 {
		t.Fatalf("cache was not reused: path=%q opens=%d", second.Path, adapter.openCount())
	}
}

func TestPrepareRejectsChangedSourceRevision(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("same-sized-content"), "revision-one")
	adapter.changeOnOpen = true
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "A Book"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Prepare(ctx, book.ID); !errors.Is(err, ErrStaleRendition) {
		t.Fatalf("Prepare() error = %v, want ErrStaleRendition", err)
	}
}

func TestProgressUsesRevisionAndRenditionCompareAndSwap(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("book"), "content-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "A Book"})
	if err != nil {
		t.Fatal(err)
	}
	locator := json.RawMessage(`{"epubcfi":"/6/4"}`)
	progress, err := svc.SaveProgress(ctx, book.ID, 0, "content-r1", locator, 0.25)
	if err != nil || progress.Revision != 1 {
		t.Fatalf("SaveProgress() = %#v, %v", progress, err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 0, "content-r1", locator, 0.5); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale progress revision error = %v, want ErrConflict", err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 1, "old-content", locator, 0.5); !errors.Is(err, ErrStaleRendition) {
		t.Fatalf("stale rendition error = %v, want ErrStaleRendition", err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 1, "content-r1", json.RawMessage(`[]`), 0.5); err == nil {
		t.Fatal("SaveProgress accepted a non-object locator")
	}
}

func TestCatalogRejectsTraversalAndSearchEscapesWildcards(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	if _, err := svc.AddBook(ctx, source.ID, Entry{Path: "../escape.epub", Size: 1}, Metadata{Title: "Escape"}); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("AddBook traversal error = %v, want ErrInvalidPath", err)
	}
	if _, err := svc.AddBook(ctx, source.ID, Entry{Path: "C:escape.epub", Size: 1}, Metadata{Title: "Drive-relative"}); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("AddBook drive-relative path error = %v, want ErrInvalidPath", err)
	}
	entry := adapter.setFile("wildcard.epub", []byte("book"), "r1")
	if _, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "100%_ Real"}); err != nil {
		t.Fatal(err)
	}
	ordinary := adapter.setFile("ordinary.epub", []byte("text"), "r1")
	if _, err := svc.AddBook(ctx, source.ID, ordinary, Metadata{Title: "Ordinary"}); err != nil {
		t.Fatal(err)
	}
	page, err := svc.Query(ctx, Query{Q: "%_"})
	if err != nil {
		t.Fatalf("Query() with escaped search characters: %v", err)
	}
	if page.Total != 1 || page.Books[0].Title != "100%_ Real" {
		t.Fatalf("wildcards were treated as SQL operators: %#v", page)
	}
}

func TestCoverValidatesImageAndRevision(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("book"), "r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "A Book"})
	if err != nil {
		t.Fatal(err)
	}
	var pngData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCover(ctx, book.ID, book.Revision, "image/svg+xml", bytes.NewReader(pngData.Bytes())); err == nil {
		t.Fatal("SetCover accepted a MIME/content mismatch")
	}
	updated, err := svc.SetCover(ctx, book.ID, book.Revision, "image/png", bytes.NewReader(pngData.Bytes()))
	if err != nil || updated.Revision != book.Revision+1 || updated.CoverURL == "" {
		t.Fatalf("SetCover() = %#v, %v", updated, err)
	}
	file, mime, err := svc.OpenCover(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if mime != "image/png" {
		t.Fatalf("OpenCover() mime = %q", mime)
	}
	if _, err := svc.SetCover(ctx, book.ID, book.Revision, "image/png", bytes.NewReader(pngData.Bytes())); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale cover revision error = %v, want ErrConflict", err)
	}
}

func testEPUB(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	mimetype := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	entry, err := writer.CreateHeader(mimetype)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"META-INF/container.xml": `<container version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"/>`,
		"OEBPS/content.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml"><body>Converted text</body></html>`,
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestManagedRenditionIsBoundToPreparedRevision(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.mobi", []byte("original-mobi"), "original-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	baseRevision := prepared.Revision
	_ = prepared.Close()
	output := testEPUB(t)
	rendition, err := svc.AttachRendition(ctx, book.ID, book.Revision, baseRevision, "epub", bytes.NewReader(output))
	if err != nil {
		t.Fatalf("AttachRendition(): %v", err)
	}
	if !rendition.Available || rendition.Format != "epub" || rendition.Size != int64(len(output)) {
		t.Fatalf("unexpected rendition metadata: %#v", rendition)
	}
	bookWithRendition, err := svc.GetBook(ctx, book.ID)
	if err != nil || len(bookWithRendition.Renditions) != 1 || !bookWithRendition.Renditions[0].Available {
		t.Fatalf("GetBook rendition metadata = %#v, %v", bookWithRendition, err)
	}
	lease, err := svc.OpenRendition(ctx, book.ID, baseRevision, "epub")
	if err != nil {
		t.Fatalf("OpenRendition(): %v", err)
	}
	opened, err := io.ReadAll(lease.File)
	_ = lease.Close()
	if err != nil || !bytes.Equal(opened, output) {
		t.Fatalf("OpenRendition content mismatch: %v", err)
	}

	changed := adapter.setFile("book.mobi", []byte("new-mobi-content"), "original-r2")
	if _, err := svc.AddBook(ctx, source.ID, changed, Metadata{Title: "Original"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenRendition(ctx, book.ID, baseRevision, "epub"); !errors.Is(err, ErrStaleRendition) {
		t.Fatalf("old rendition error = %v, want ErrStaleRendition", err)
	}
	current, err := svc.GetBook(ctx, book.ID)
	if err != nil || len(current.Renditions) != 0 {
		t.Fatalf("changed source retained derived rendition: %#v, %v", current.Renditions, err)
	}
}

func TestUnversionedSourceFingerprintRetiresProgress(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("first payload"), "")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}
	firstLease, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshot := firstLease.Revision
	firstCachePath := firstLease.Path
	_ = firstLease.Close()
	updatedBook, err := svc.GetBook(ctx, book.ID)
	if err != nil || updatedBook.Revision != book.Revision+1 || updatedBook.ContentRevision != firstSnapshot {
		t.Fatalf("Prepare did not record unversioned fingerprint: %#v, %v", updatedBook, err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 0, firstSnapshot, json.RawMessage(`{"position":4}`), 0.4); err != nil {
		t.Fatal(err)
	}
	updatedBook, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCachePinned(ctx, book.ID, updatedBook.Revision, true); err != nil {
		t.Fatalf("pin first unverified snapshot: %v", err)
	}

	adapter.setFile("book.epub", []byte("other payload"), "") // Same length, different content.
	secondLease, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer secondLease.Close()
	latest, err := svc.GetBook(ctx, book.ID)
	if err != nil || latest.ContentRevision == firstSnapshot || latest.Progress != nil || latest.ReadingState != "unread" {
		t.Fatalf("source change did not retire stale progress: %#v, %v", latest, err)
	}
	oldSnapshot, err := os.ReadFile(firstCachePath)
	if err != nil || string(oldSnapshot) != "first payload" {
		t.Fatalf("new unverified bytes overwrote the prior pinned cache object: %q, %v", oldSnapshot, err)
	}
	var pinCount int
	if err := svc.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cache_pins WHERE book_id=?`, book.ID).Scan(&pinCount); err != nil || pinCount != 0 {
		t.Fatalf("stale pin was not retired after content change: pins=%d, %v", pinCount, err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 0, firstSnapshot, json.RawMessage(`{"position":4}`), 0.4); !errors.Is(err, ErrStaleRendition) {
		t.Fatalf("old snapshot progress error = %v, want ErrStaleRendition", err)
	}
}

func TestPrepareReadOnlyPreservesUnverifiedCatalogHistory(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("same-size content"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	baseRevision := lease.Revision
	_ = lease.Close()
	if _, err := svc.SaveProgress(ctx, book.ID, 0, baseRevision, json.RawMessage(`{"position":4}`), 0.4); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachRenditionVersioned(ctx, book.ID, book.Revision, baseRevision, "epub", "converter-r1", bytes.NewReader(testEPUB(t))); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCachePinned(ctx, book.ID, book.Revision, true); err != nil {
		t.Fatal(err)
	}
	// Simulate an older catalog record with no recorded source fingerprint,
	// while its progress, derived rendition, and pin still need preservation.
	if _, err := svc.db.ExecContext(ctx, `UPDATE books SET content_revision='' WHERE id=?`, book.ID); err != nil {
		t.Fatal(err)
	}
	adapter.setFile("book.epub", []byte("same-size content"), "")
	before, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	var pinsBefore int
	if err := svc.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cache_pins WHERE book_id=?`, book.ID).Scan(&pinsBefore); err != nil {
		t.Fatal(err)
	}

	prepared, err := svc.PrepareReadOnly(ctx, book.ID)
	if err != nil {
		t.Fatalf("PrepareReadOnly(): %v", err)
	}
	if prepared.Revision == "" || !strings.HasPrefix(prepared.Revision, "unverified:") {
		t.Fatalf("read-only preparation did not return a content fingerprint: %q", prepared.Revision)
	}
	_ = prepared.Close()

	after, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	var pinsAfter int
	if err := svc.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cache_pins WHERE book_id=?`, book.ID).Scan(&pinsAfter); err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || after.ContentRevision != before.ContentRevision ||
		after.Progress == nil || before.Progress == nil || after.Progress.Revision != before.Progress.Revision ||
		after.Progress.RenditionRevision != before.Progress.RenditionRevision ||
		len(after.Renditions) != len(before.Renditions) || pinsAfter != pinsBefore || pinsAfter != 1 {
		t.Fatalf("read-only prepare mutated catalog history: before=%#v after=%#v", before, after)
	}
}

func TestSetMaxBookBytesUpdatesPreparationLimit(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("bounded source content"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetMaxBookBytes(4); err != nil {
		t.Fatalf("SetMaxBookBytes(): %v", err)
	}
	if _, err := svc.Prepare(ctx, book.ID); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("Prepare() with small configured limit = %v, want ErrLimitExceeded", err)
	}
	if adapter.openCount() != 0 {
		t.Fatal("oversized source was opened after its stat exceeded the configured limit")
	}
	if err := svc.SetMaxBookBytes(MaxSourceBookBytes + 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("SetMaxBookBytes() accepted an unsafe limit: %v", err)
	}
	if err := svc.SetMaxBookBytes(1024); err != nil {
		t.Fatalf("raising the configured limit: %v", err)
	}
	lease, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatalf("Prepare() after raising limit: %v", err)
	}
	_ = lease.Close()
}

func TestScanSkipsOversizedEntriesAndPreservesKnownBooks(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	known := adapter.setFile("known.epub", []byte("known bytes"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, known, Metadata{Title: "Known book"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 0, "source-r1", json.RawMessage(`{"position":9}`), 0.7); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetMaxBookBytes(4); err != nil {
		t.Fatal(err)
	}
	oversizedKnown := adapter.setFile("known.epub", []byte("changed and oversized"), "source-r2")
	oversizedNew := adapter.setFile("new.epub", []byte("also oversized"), "new-r1")
	adapter.directories[""] = []Entry{oversizedKnown, oversizedNew}

	result, err := svc.Scan(ctx, source.ID)
	if err != nil {
		t.Fatalf("Scan(): %v", err)
	}
	if !result.Complete || result.Seen != 2 || result.Added != 0 || result.Updated != 0 || result.Removed != 0 || len(result.Issues) != 2 {
		t.Fatalf("unexpected bounded scan result: %#v", result)
	}
	current, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !current.Available || current.Size != known.Size || current.ContentRevision != "source-r1" || current.Progress == nil || current.Progress.Revision != 1 {
		t.Fatalf("oversized observation changed or hid known catalog history: %#v", current)
	}
	if _, err := svc.GetBook(ctx, bookID(source.ID, "new.epub")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("new oversized entry was cataloged: %v", err)
	}
}

func TestCollectionBookIDsPreservesMembershipOrder(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	var ids []string
	for _, name := range []string{"one.epub", "two.epub"} {
		entry := adapter.setFile(name, []byte(name), "r1")
		book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: name})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, book.ID)
	}
	collection, err := svc.CreateCollection(ctx, "Favorites", 0)
	if err != nil {
		t.Fatal(err)
	}
	collection, err = svc.SetCollectionBooks(ctx, collection.ID, collection.Revision, ids)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.CollectionBookIDs(ctx, collection.ID)
	if err != nil || len(got) != len(ids) || got[0] != ids[0] || got[1] != ids[1] {
		t.Fatalf("CollectionBookIDs() = %v, %v", got, err)
	}
}

func TestAttachRenditionRejectsMalformedEPUB(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.mobi", []byte("original"), "r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachRendition(ctx, book.ID, book.Revision, "r1", "epub", bytes.NewReader([]byte("not a zip"))); err == nil {
		t.Fatal("AttachRendition accepted malformed EPUB bytes")
	}
}
