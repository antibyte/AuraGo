package memory

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestDocumentCorpusSQLiteFullPreservesPreviousSourceAndIndex(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	doc := chromem.Document{ID: "source", Content: "original complete source", Metadata: cv.addEmbeddingMetadata(nil), Embedding: []float32{1, 0, 0}}
	cv.mu.Lock()
	err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
	cv.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	cv.corpus.db.SetMaxOpenConns(1)
	var pages int
	if err := cv.corpus.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.corpus.db.Exec(`PRAGMA max_page_count=` + strconv.Itoa(pages)); err != nil {
		t.Fatal(err)
	}
	replacement := doc
	replacement.Content = strings.Repeat("new source data ", 100000)
	cv.mu.Lock()
	err = cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{replacement})
	cv.mu.Unlock()
	var sqliteErr interface{ Code() int }
	if !errors.As(err, &sqliteErr) || sqliteErr.Code()&255 != 13 {
		t.Fatalf("expected SQLITE_FULL, got %v", err)
	}
	actual, err := cv.corpusDocument(t.Context(), "aurago_memories", doc.ID)
	if err != nil || actual.Content != doc.Content {
		t.Fatalf("full database changed source: %v", err)
	}
	cached, err := cv.collection.GetByID(t.Context(), doc.ID)
	if err != nil || cached.Content != doc.Content {
		t.Fatalf("full database changed index: %v", err)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	actual, err = cv.corpusDocument(t.Context(), "aurago_memories", doc.ID)
	if err != nil || actual.Content != doc.Content {
		t.Fatalf("restart lost source after SQLITE_FULL: %v", err)
	}
}

// The context cancels at the real commit visibility boundary without a runtime
// fault-injection hook. Uncommitted rows are invisible to its separate reader.
type corpusCommitCancellation struct {
	context.Context
	db     *sql.DB
	cancel context.CancelFunc
	id     string
}

func (c corpusCommitCancellation) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	var committed bool
	if err := c.db.QueryRowContext(context.Background(), `SELECT EXISTS(SELECT 1 FROM documents WHERE id=?)`, c.id).Scan(&committed); err == nil && committed {
		c.cancel()
	}
	return c.Context.Err()
}

func TestDocumentCorpusCancellationAfterCommitRetainsPendingSourceForRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	base, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := corpusCommitCancellation{base, cv.corpus.db, cancel, "committed-before-cancellation"}
	doc := chromem.Document{ID: ctx.id, Content: "complete source before index publication", Metadata: cv.addEmbeddingMetadata(nil), Embedding: []float32{1, 0, 0}}
	cv.mu.Lock()
	err := cv.persistDocumentsLocked(ctx, cv.collection, []chromem.Document{doc})
	cv.mu.Unlock()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wanted cancellation after commit, got %v", err)
	}
	if cv.collection.Count() != 0 {
		t.Fatal("cancelled index mutation was published")
	}
	actual, err := cv.corpusDocument(t.Context(), "aurago_memories", doc.ID)
	if err != nil || actual.Content != doc.Content {
		t.Fatalf("committed source lost: %v", err)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	if err := cv.reconcileCorpus(t.Context()); err != nil {
		t.Fatal(err)
	}
	actual, err = cv.collection.GetByID(t.Context(), doc.ID)
	if err != nil || actual.Content != doc.Content {
		t.Fatalf("restart did not reconcile committed source: %v", err)
	}
}
