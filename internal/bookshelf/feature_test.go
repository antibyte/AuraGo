package bookshelf

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"testing"
)

func TestQuerySortStatusAndManualReadingState(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	books := make(map[string]Book)
	for _, item := range []struct {
		path, title, author string
	}{
		{"a.epub", "Alpha", "Zed"}, {"b.epub", "Bravo", "Able"}, {"c.epub", "Charlie", "Moe"},
	} {
		entry := adapter.setFile(item.path, []byte(item.path), "revision-1")
		book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: item.title, Author: item.author})
		if err != nil {
			t.Fatal(err)
		}
		books[item.title] = book
	}
	if _, err := svc.SaveProgress(ctx, books["Bravo"].ID, 0, "revision-1", json.RawMessage(`{"position":1}`), 0.2); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveProgress(ctx, books["Charlie"].ID, 0, "revision-1", json.RawMessage(`{"position":2}`), 1); err != nil {
		t.Fatal(err)
	}
	page, err := svc.Query(ctx, Query{Sort: "author", Limit: 2})
	if err != nil || page.Total != 3 || len(page.Books) != 2 || page.Books[0].Title != "Bravo" || page.Books[1].Title != "Charlie" {
		t.Fatalf("author sort should cover full catalog: %#v, %v", page, err)
	}
	page, err = svc.Query(ctx, Query{Status: "reading", Offset: 0, Limit: 1})
	if err != nil || page.Total != 1 || len(page.Books) != 1 || page.Books[0].ID != books["Bravo"].ID {
		t.Fatalf("reading filter should count before paging: %#v, %v", page, err)
	}
	page, err = svc.Query(ctx, Query{Status: "finished", Sort: "progress"})
	if err != nil || page.Total != 1 || page.Books[0].ID != books["Charlie"].ID {
		t.Fatalf("finished filter mismatch: %#v, %v", page, err)
	}
	current, err := svc.GetBook(ctx, books["Bravo"].ID)
	if err != nil {
		t.Fatal(err)
	}
	manual, err := svc.SetReadingState(ctx, current.ID, current.Revision, "finished")
	if err != nil || !manual.ReadingStateManual || manual.ReadingState != "finished" {
		t.Fatalf("SetReadingState() = %#v, %v", manual, err)
	}
	if _, err := svc.SetReadingState(ctx, current.ID, current.Revision, "unread"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale reading-state update error = %v, want ErrConflict", err)
	}
	if _, err := svc.SaveProgress(ctx, current.ID, 1, "revision-1", json.RawMessage(`{"position":3}`), 0.4); err != nil {
		t.Fatal(err)
	}
	manual, err = svc.GetBook(ctx, current.ID)
	if err != nil || manual.ReadingState != "finished" || !manual.ReadingStateManual {
		t.Fatalf("progress overwrote explicit reading state: %#v, %v", manual, err)
	}
	if _, err := svc.Query(ctx, Query{Sort: "title; DROP TABLE books"}); err == nil {
		t.Fatal("Query accepted an unrecognized sort expression")
	}
	if _, err := svc.Query(ctx, Query{Status: "maybe"}); err == nil {
		t.Fatal("Query accepted an unrecognized status")
	}
}

func TestQueryCollectionSortUsesMembershipOrder(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	books := make(map[string]Book)
	for _, item := range []struct {
		path, title string
	}{
		{"a.epub", "Alpha"}, {"b.epub", "Bravo"}, {"c.epub", "Charlie"},
	} {
		entry := adapter.setFile(item.path, []byte(item.path), "revision-1")
		book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: item.title})
		if err != nil {
			t.Fatal(err)
		}
		books[item.title] = book
	}
	collection, err := svc.CreateCollection(ctx, "Reading order", 0)
	if err != nil {
		t.Fatal(err)
	}
	order := []string{books["Charlie"].ID, books["Alpha"].ID, books["Bravo"].ID}
	collection, err = svc.SetCollectionBooks(ctx, collection.ID, collection.Revision, order)
	if err != nil {
		t.Fatal(err)
	}

	for _, query := range []Query{
		{CollectionID: collection.ID, Sort: "collection", Limit: 3},
		{CollectionID: collection.ID, Limit: 3},
	} {
		page, err := svc.Query(ctx, query)
		if err != nil || page.Total != len(order) || len(page.Books) != len(order) {
			t.Fatalf("Query(%#v) = %#v, %v", query, page, err)
		}
		for i, id := range order {
			if page.Books[i].ID != id {
				t.Fatalf("Query(%#v) book[%d] = %q, want collection member %q", query, i, page.Books[i].ID, id)
			}
		}
	}
	page, err := svc.Query(ctx, Query{CollectionID: collection.ID, Sort: "collection", Offset: 1, Limit: 1})
	if err != nil || len(page.Books) != 1 || page.Books[0].ID != order[1] {
		t.Fatalf("paged collection Query() = %#v, %v; want second member %q", page, err, order[1])
	}
	if _, err := svc.Query(ctx, Query{Sort: "collection", Limit: 1}); err == nil {
		t.Fatal("collection sort without collection filter should be rejected")
	}
}

func TestRenditionAnnotationsStayHistoricalAndRejectStaleCreation(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.mobi", []byte("original source"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	output := testEPUB(t)
	firstRendition, err := svc.AttachRendition(ctx, book.ID, book.Revision, "source-r1", "epub", bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	locator := json.RawMessage(`{"epubcfi":"/6/4"}`)
	annotation, err := svc.CreateAnnotationForRendition(ctx, book.ID, "highlight", firstRendition.Revision, locator, "Keep this", "yellow")
	if err != nil || annotation.RenditionRevision != firstRendition.Revision {
		t.Fatalf("CreateAnnotationForRendition() = %#v, %v", annotation, err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 0, firstRendition.Revision, locator, 0.3); err != nil {
		t.Fatalf("progress on managed rendition rejected: %v", err)
	}
	changed := adapter.setFile("book.mobi", []byte("new source bytes"), "source-r2")
	if _, err := svc.AddBook(ctx, source.ID, changed, Metadata{Title: "Original"}); err != nil {
		t.Fatal(err)
	}
	annotations, err := svc.ListAnnotations(ctx, book.ID)
	if err != nil || len(annotations) != 1 || annotations[0].RenditionRevision != firstRendition.Revision {
		t.Fatalf("historical annotation was lost or rebound: %#v, %v", annotations, err)
	}
	if _, err := svc.CreateAnnotationForRendition(ctx, book.ID, "bookmark", firstRendition.Revision, locator, "", ""); !errors.Is(err, ErrStaleRendition) {
		t.Fatalf("stale annotation revision error = %v, want ErrStaleRendition", err)
	}
}

func TestRenditionCacheIsVersionedByConverterFingerprint(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.mobi", []byte("original"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Converted"})
	if err != nil {
		t.Fatal(err)
	}
	output := testEPUB(t)
	r1, err := svc.AttachRenditionVersioned(ctx, book.ID, book.Revision, "source-r1", "epub", "converter-v1", bytes.NewReader(output))
	if err != nil || r1.ConverterVersion != "converter-v1" {
		t.Fatalf("AttachRenditionVersioned v1 = %#v, %v", r1, err)
	}
	if _, err := svc.OpenRenditionVersion(ctx, book.ID, "source-r1", "epub", "converter-v2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old converter output was reused: %v", err)
	}
	if _, err := svc.AttachRenditionVersioned(ctx, book.ID, book.Revision, "source-r1", "epub", "converter-v2", bytes.NewReader(output)); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.GetBook(ctx, book.ID)
	if err != nil || len(updated.Renditions) != 1 || updated.Renditions[0].ConverterVersion != "converter-v2" {
		t.Fatalf("converter fingerprint did not replace cached rendition: %#v, %v", updated.Renditions, err)
	}
	lease, err := svc.OpenRenditionVersion(ctx, book.ID, "source-r1", "epub", "converter-v2")
	if err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
}

func TestPinnedCacheSurvivesOfflineButHonorsRevocation(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("cached book bytes"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Pinned"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := svc.SetCachePinned(ctx, book.ID, book.Revision, true)
	if err != nil || info.PinnedBytes != int64(len("cached book bytes")) || info.PinnedBooks != 1 {
		t.Fatalf("pin cache = %#v, %v", info, err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil || !book.CachePinned {
		t.Fatalf("book not marked pinned: %#v, %v", book, err)
	}
	adapter.setStatError(ErrSourceUnavailable)
	lease, err := svc.Prepare(ctx, book.ID)
	if err != nil {
		t.Fatalf("pinned offline Prepare() = %v", err)
	}
	body, err := io.ReadAll(lease.File)
	_ = lease.Close()
	if err != nil || string(body) != "cached book bytes" {
		t.Fatalf("pinned snapshot bytes = %q, %v", body, err)
	}
	adapter.setStatError(os.ErrPermission)
	if _, err := svc.Prepare(ctx, book.ID); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("permission denial must block pinned bytes: %v", err)
	}
	adapter.setStatError(ErrSourceRevoked)
	if _, err := svc.Prepare(ctx, book.ID); !errors.Is(err, ErrSourceRevoked) {
		t.Fatalf("revoked source error = %v, want ErrSourceRevoked", err)
	}
	adapter.setStatError(nil)
	if _, err := svc.SetCachePinned(ctx, book.ID, book.Revision, false); err != nil {
		t.Fatal(err)
	}
	info, err = svc.CacheInfo(ctx)
	if err != nil || info.PinnedBooks != 0 || info.PinnedBytes != 0 {
		t.Fatalf("unpin cache = %#v, %v", info, err)
	}
}

func TestSourceRefreshIntervalIsOptInBoundedAndRevisioned(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	svc := newTestService(t, adapter, 1<<20)
	if _, err := svc.AddSource(ctx, Source{Name: "Too frequent", Provider: ProviderLocal, RefreshMinutes: 14}); err == nil {
		t.Fatal("AddSource accepted a refresh interval below the minimum")
	}
	source, err := svc.AddSource(ctx, Source{Name: "Scheduled", Provider: ProviderLocal, RefreshMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := svc.GetSource(ctx, source.ID)
	if err != nil || loaded.RefreshMinutes != 15 {
		t.Fatalf("GetSource refresh interval = %#v, %v", loaded, err)
	}
	loaded.RefreshMinutes = 10_080
	updated, err := svc.UpdateSource(ctx, loaded)
	if err != nil || updated.RefreshMinutes != 10_080 || updated.Generation != loaded.Generation+1 {
		t.Fatalf("UpdateSource refresh interval = %#v, %v", updated, err)
	}
	updated.RefreshMinutes = 10_081
	if _, err := svc.UpdateSource(ctx, updated); err == nil {
		t.Fatal("UpdateSource accepted a refresh interval above the maximum")
	}
	loaded, err = svc.GetSource(ctx, source.ID)
	if err != nil || loaded.RefreshMinutes != 10_080 {
		t.Fatalf("invalid update changed persisted interval: %#v, %v", loaded, err)
	}
}

func TestExtractMetadataForEPUBFB2AndCBZ(t *testing.T) {
	ctx := context.Background()
	cover := testPNG(t)
	epub := zipBytes(t, map[string][]byte{
		"META-INF/container.xml": []byte(`<container><rootfiles><rootfile full-path="OEBPS/book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"OEBPS/book.opf":         []byte(`<package xmlns:dc="http://purl.org/dc/elements/1.1/"><metadata><dc:title>Archive Book</dc:title><dc:creator>Author One</dc:creator><dc:subject>Fiction</dc:subject><meta property="belongs-to-collection">Series A</meta></metadata><manifest><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/></manifest></package>`),
		"OEBPS/cover.png":        cover,
	})
	parsed, err := ExtractMetadata(ctx, bytes.NewReader(epub), int64(len(epub)), "epub")
	if err != nil || parsed.Metadata.Title != "Archive Book" || parsed.Metadata.Author != "Author One" || parsed.Metadata.Series != "Series A" || len(parsed.Metadata.Tags) != 1 || !bytes.Equal(parsed.EmbeddedCover, cover) {
		t.Fatalf("EPUB metadata = %#v, %v", parsed, err)
	}
	fb2 := []byte(`<FictionBook xmlns:l="http://www.w3.org/1999/xlink"><description><title-info><book-title>FB2 Book</book-title><author><first-name>Ada</first-name><last-name>Lovelace</last-name></author><sequence name="Math Reads"/><genre>science</genre><coverpage><image l:href="#cover"/></coverpage></title-info><publish-info><book-name>Ignored</book-name></publish-info></description><binary id="cover" content-type="image/png">` + base64.StdEncoding.EncodeToString(cover) + `</binary></FictionBook>`)
	parsed, err = ExtractMetadata(ctx, bytes.NewReader(fb2), int64(len(fb2)), "fb2")
	if err != nil || parsed.Metadata.Title != "FB2 Book" || parsed.Metadata.Author != "Ada Lovelace" || parsed.Metadata.Series != "Math Reads" || !bytes.Equal(parsed.EmbeddedCover, cover) {
		t.Fatalf("FB2 metadata = %#v, %v", parsed, err)
	}
	cbz := zipBytes(t, map[string][]byte{
		"ComicInfo.xml":   []byte(`<ComicInfo><Title>Comic 1</Title><Series>Panels</Series><Writer>Writer</Writer><Summary>Issue summary</Summary><Genre>Action, Mystery</Genre></ComicInfo>`),
		"pages/cover.png": cover,
	})
	parsed, err = ExtractMetadata(ctx, bytes.NewReader(cbz), int64(len(cbz)), "cbz")
	if err != nil || parsed.Metadata.Title != "Comic 1" || parsed.Metadata.Author != "Writer" || parsed.Metadata.Series != "Panels" || len(parsed.Metadata.Tags) != 2 || !bytes.Equal(parsed.EmbeddedCover, cover) {
		t.Fatalf("CBZ metadata = %#v, %v", parsed, err)
	}
	malicious := zipBytes(t, map[string][]byte{"../escape.png": cover})
	if _, err := ExtractMetadata(ctx, bytes.NewReader(malicious), int64(len(malicious)), "cbz"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("archive traversal error = %v, want ErrInvalidPath", err)
	}
}

func TestDiscoveredMetadataPreservesOverridesAndTypographicFallbackIsStable(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("book"), "r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Filename"})
	if err != nil {
		t.Fatal(err)
	}
	wanted := "My edition"
	book, err = svc.UpdateMetadataOverride(ctx, book.ID, book.Revision, MetadataOverride{Title: &wanted})
	if err != nil {
		t.Fatal(err)
	}
	book, err = svc.UpdateDiscoveredMetadata(ctx, book.ID, book.Revision, Metadata{Title: "Embedded title", Author: "Embedded author", Format: "epub"})
	if err != nil || book.Title != wanted || book.Author != "Embedded author" {
		t.Fatalf("discovered metadata replaced an override: %#v, %v", book, err)
	}
	fallback := FallbackCover(book)
	if fallback.Monogram == "" || fallback.ColorIndex < 0 || fallback.PatternIndex < 0 || fallback != FallbackCover(book) {
		t.Fatalf("typographic cover fallback is missing or unstable: %#v", fallback)
	}
	tooMany := make([]string, 129)
	for i := range tooMany {
		tooMany[i] = "same"
	}
	if _, err := svc.UpdateDiscoveredMetadata(ctx, book.ID, book.Revision, Metadata{Format: "epub", Tags: tooMany}); err == nil {
		t.Fatal("discovered metadata normalized an oversized tag list before validation")
	}
}

func TestOpenRenditionVersionDoesNotRetireHistoryOnUnverifiedChange(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	adapter.setFile("book.mobi", []byte("original source"), "")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	entry, err := adapter.Stat(ctx, source, "book.mobi")
	if err != nil {
		t.Fatal(err)
	}
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
	if _, err := svc.SaveProgress(ctx, book.ID, 0, baseRevision, json.RawMessage(`{"position":1}`), 0.25); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	rendition, err := svc.AttachRenditionVersioned(ctx, book.ID, book.Revision, baseRevision, "epub", "converter-v1", bytes.NewReader(testEPUB(t)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAnnotationForRendition(ctx, book.ID, "highlight", rendition.Revision, json.RawMessage(`{"epubcfi":"/6/2"}`), "Keep this", "yellow"); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCachePinned(ctx, book.ID, book.Revision, true); err != nil {
		t.Fatal(err)
	}
	before, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}

	// No stable provider revision is available, so validation uses a content
	// digest. A changed same-size snapshot must be rejected without persisting
	// that fingerprint or retiring the prior read-only history.
	adapter.setFile("book.mobi", []byte("changed--source"), "")
	if _, err := svc.OpenRenditionVersion(ctx, book.ID, baseRevision, "epub", "converter-v1"); !errors.Is(err, ErrStaleRendition) {
		t.Fatalf("OpenRenditionVersion() error = %v, want ErrStaleRendition", err)
	}
	after, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	annotations, err := svc.ListAnnotations(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	var pins, renditions, progress int
	for query, target := range map[string]*int{
		`SELECT COUNT(*) FROM cache_pins WHERE book_id=?`: &pins,
		`SELECT COUNT(*) FROM renditions WHERE book_id=?`: &renditions,
		`SELECT COUNT(*) FROM progress WHERE book_id=?`:   &progress,
	} {
		if err := svc.db.QueryRowContext(ctx, query, book.ID).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if after.Revision != before.Revision || after.ContentRevision != before.ContentRevision ||
		after.Progress == nil || before.Progress == nil || after.Progress.Revision != before.Progress.Revision ||
		len(after.Renditions) != 1 || pins != 1 || renditions != 1 || progress != 1 || len(annotations) != 1 {
		t.Fatalf("readonly rendition validation mutated history: before=%#v after=%#v pins=%d renditions=%d progress=%d annotations=%#v", before, after, pins, renditions, progress, annotations)
	}
}

func TestUpdateSourceIdentityChangeRetiresSnapshotButKeepsAnnotationHistory(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.mobi", []byte("original source"), "source-r1")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveProgress(ctx, book.ID, 0, "source-r1", json.RawMessage(`{"position":1}`), 0.25); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	rendition, err := svc.AttachRenditionVersioned(ctx, book.ID, book.Revision, "source-r1", "epub", "converter-v1", bytes.NewReader(testEPUB(t)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAnnotationForRendition(ctx, book.ID, "highlight", rendition.Revision, json.RawMessage(`{"epubcfi":"/6/2"}`), "Historical note", "yellow"); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCachePinned(ctx, book.ID, book.Revision, true); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	source, err = svc.GetSource(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	source.Path = "new-root"
	updated, err := svc.UpdateSource(ctx, source)
	if err != nil || updated.Generation != source.Generation+1 {
		t.Fatalf("UpdateSource() = %#v, %v", updated, err)
	}
	book, err = svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	annotations, err := svc.ListAnnotations(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	var pins, renditions, progress int
	for query, target := range map[string]*int{
		`SELECT COUNT(*) FROM cache_pins WHERE book_id=?`: &pins,
		`SELECT COUNT(*) FROM renditions WHERE book_id=?`: &renditions,
		`SELECT COUNT(*) FROM progress WHERE book_id=?`:   &progress,
	} {
		if err := svc.db.QueryRowContext(ctx, query, book.ID).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if book.Available || book.ContentRevision != "" || book.Progress != nil || len(book.Renditions) != 0 ||
		pins != 0 || renditions != 0 || progress != 0 || len(annotations) != 1 || annotations[0].RenditionRevision != rendition.Revision {
		t.Fatalf("source identity change did not retire only snapshot-bound state: book=%#v pins=%d renditions=%d progress=%d annotations=%#v", book, pins, renditions, progress, annotations)
	}
}

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
