package bookshelf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestVersionlessRescanAndAddBookPreservePreparedState(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("same bytes"), "")
	adapter.directories[""] = []Entry{entry}
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	if result, err := svc.Scan(ctx, source.ID); err != nil || !result.Complete || result.Added != 1 {
		t.Fatalf("initial Scan() = %#v, %v", result, err)
	}

	id := bookID(source.ID, entry.Path)
	lease, err := svc.Prepare(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	contentRevision := lease.Revision
	if _, err := svc.SaveProgress(ctx, id, 0, contentRevision, json.RawMessage(`{"epubcfi":"chapter-1"}`), 0.4); err != nil {
		_ = lease.Close()
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	book, err := svc.GetBook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachRenditionVersioned(ctx, id, book.Revision, contentRevision, "epub", "converter-v1", bytes.NewReader(testEPUB(t))); err != nil {
		t.Fatal(err)
	}
	book, err = svc.GetBook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetCachePinned(ctx, id, book.Revision, true); err != nil {
		t.Fatal(err)
	}
	before, err := svc.GetBook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	assertState := func(stage string) {
		t.Helper()
		current, err := svc.GetBook(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if current.Revision != before.Revision || current.ContentRevision != before.ContentRevision || current.Progress == nil || len(current.Renditions) != 1 || !current.CachePinned {
			t.Fatalf("%s discarded versionless book state: before=%#v current=%#v", stage, before, current)
		}
	}
	if before.Progress == nil || len(before.Renditions) != 1 || !before.CachePinned {
		t.Fatalf("test setup did not retain complete book state: %#v", before)
	}

	if result, err := svc.Scan(ctx, source.ID); err != nil || !result.Complete {
		t.Fatalf("unchanged versionless Scan() = %#v, %v", result, err)
	}
	assertState("Scan")
	if _, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "book"}); err != nil {
		t.Fatalf("unchanged versionless AddBook() = %v", err)
	}
	assertState("AddBook")
}

func TestSetCachePinnedAcceptsItsUnverifiedFingerprintUpdate(t *testing.T) {
	ctx := context.Background()
	adapter := newMemoryAdapter()
	entry := adapter.setFile("book.epub", []byte("same bytes"), "")
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}

	info, err := svc.SetCachePinned(ctx, book.ID, book.Revision, true)
	if err != nil {
		t.Fatalf("first versionless pin = %v", err)
	}
	current, err := svc.GetBook(ctx, book.ID)
	if err != nil || !current.CachePinned || current.ContentRevision == "" || info.PinnedBooks != 1 {
		t.Fatalf("first versionless pin state = book %#v, cache %#v, %v", current, info, err)
	}
}

type blockingStatAdapter struct {
	*memoryAdapter
	started chan Source
	release chan struct{}
	once    sync.Once
}

func (a *blockingStatAdapter) Stat(ctx context.Context, source Source, path string) (Entry, error) {
	var waitErr error
	a.once.Do(func() {
		select {
		case a.started <- source:
		case <-ctx.Done():
			waitErr = ctx.Err()
			return
		}
		select {
		case <-a.release:
		case <-ctx.Done():
			waitErr = ctx.Err()
		}
	})
	if waitErr != nil {
		return Entry{}, waitErr
	}
	return a.memoryAdapter.Stat(ctx, source, path)
}

func TestResolveLocalCannotReapplyAfterSourceIdentityChange(t *testing.T) {
	ctx := context.Background()
	memory := newMemoryAdapter()
	entry := memory.setFile("book.epub", []byte("old source bytes"), "old-source-revision")
	adapter := &blockingStatAdapter{memoryAdapter: memory, started: make(chan Source, 1), release: make(chan struct{})}
	defer func() {
		select {
		case <-adapter.release:
		default:
			close(adapter.release)
		}
	}()
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}
	type resolveResult struct {
		book Book
		err  error
	}
	resolved := make(chan resolveResult, 1)
	go func() {
		result, resolveErr := svc.ResolveLocal(ctx, source.ID, entry.Path)
		resolved <- resolveResult{book: result, err: resolveErr}
	}()
	select {
	case <-adapter.started:
	case <-time.After(5 * time.Second):
		t.Fatal("ResolveLocal did not reach blocking Stat")
	}
	updatedSource, err := svc.GetSource(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	updatedSource.Path = "new-library-root"
	updated := make(chan error, 1)
	updateStarted := make(chan struct{})
	go func() {
		close(updateStarted)
		_, updateErr := svc.UpdateSource(ctx, updatedSource)
		updated <- updateErr
	}()
	select {
	case <-updateStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateSource did not start")
	}
	select {
	case err := <-updated:
		t.Fatalf("UpdateSource completed during ResolveLocal Stat: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(adapter.release)
	select {
	case result := <-resolved:
		if result.err != nil {
			t.Fatalf("ResolveLocal() = %v", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ResolveLocal did not finish after Stat barrier release")
	}
	select {
	case err := <-updated:
		if err != nil {
			t.Fatalf("UpdateSource() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateSource did not finish after ResolveLocal released the source lock")
	}
	currentBook, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	currentSource, err := svc.GetSource(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if currentSource.Generation != source.Generation+1 || currentSource.Path != "new-library-root" || currentBook.Available || currentBook.ContentRevision != "" {
		t.Fatalf("source change failed to invalidate the resolved book: source=%#v book=%#v", currentSource, currentBook)
	}
}

func TestSetCachePinnedStillRejectsConcurrentBookEditDuringPreparation(t *testing.T) {
	ctx := context.Background()
	memory := newMemoryAdapter()
	entry := memory.setFile("book.epub", []byte("same bytes"), "")
	adapter := &blockingStatAdapter{memoryAdapter: memory, started: make(chan Source, 1), release: make(chan struct{})}
	defer func() {
		select {
		case <-adapter.release:
		default:
			close(adapter.release)
		}
	}()
	svc := newTestService(t, adapter, 1<<20)
	source := addTestSource(t, svc, false)
	book, err := svc.AddBook(ctx, source.ID, entry, Metadata{Title: "Book"})
	if err != nil {
		t.Fatal(err)
	}
	pinned := make(chan error, 1)
	go func() {
		_, pinErr := svc.SetCachePinned(ctx, book.ID, book.Revision, true)
		pinned <- pinErr
	}()
	select {
	case <-adapter.started:
	case <-time.After(5 * time.Second):
		t.Fatal("pin did not reach blocking Stat")
	}
	if _, err := svc.SetReadingState(ctx, book.ID, book.Revision, "reading"); err != nil {
		t.Fatal(err)
	}
	close(adapter.release)
	select {
	case err := <-pinned:
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("SetCachePinned() error = %v, want ErrConflict", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetCachePinned did not finish after Stat barrier release")
	}
	current, err := svc.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ReadingState != "reading" || current.CachePinned {
		t.Fatalf("concurrent book edit was lost or pin was written: %#v", current)
	}
}
