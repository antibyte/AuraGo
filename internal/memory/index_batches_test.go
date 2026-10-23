package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// batchTestSecret is recognized by the durable-credential policy (key material).
const batchTestSecret = "sk-proj-AAAAAAAAAAAAAAAAAAAAAAAA"

// recordingEmbedder counts provider calls, can fail on a chosen call and keeps the
// context deadline and the text of every call.
type recordingEmbedder struct {
	mu        sync.Mutex
	calls     int
	failOn    int // 1-based call number that fails; 0 = never
	texts     []string
	deadlines []time.Duration // time left until the call's deadline; -1 = none
}

func (r *recordingEmbedder) embed(ctx context.Context, text string) ([]float32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.texts = append(r.texts, text)
	if deadline, ok := ctx.Deadline(); ok {
		r.deadlines = append(r.deadlines, time.Until(deadline))
	} else {
		r.deadlines = append(r.deadlines, -1)
	}
	if r.failOn > 0 && r.calls == r.failOn {
		return nil, errors.New("embedding provider unavailable")
	}
	return []float32{1, 0, 0}, nil
}

func (r *recordingEmbedder) reset(failOn int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls, r.failOn, r.texts, r.deadlines = 0, failOn, nil, nil
}

func (r *recordingEmbedder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *recordingEmbedder) sawText(fragment string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, text := range r.texts {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

func newIndexBatchTestDB(t *testing.T, embedder *recordingEmbedder) *ChromemVectorDB {
	t.Helper()
	cv := newTestChromemVectorDB(t, embedder.embed)
	cv.dataDir = t.TempDir()
	t.Cleanup(func() { cv.Close() })
	return cv
}

// A failure in the middle of the tool guides must keep the batches that were
// already written, leave the content hash unwritten, and let the next attempt
// embed only what is still missing.
func TestIndexToolGuidesKeepsPartialProgressAndResumesWithoutReembedding(t *testing.T) {
	embedder := &recordingEmbedder{failOn: 40}
	cv := newIndexBatchTestDB(t, embedder)
	toolsDir := t.TempDir() // empty: only the embedded manuals are indexed

	if err := cv.IndexToolGuides(toolsDir, false); err == nil {
		t.Fatal("IndexToolGuides succeeded although the provider failed on call 40")
	}
	hashFile := filepath.Join(cv.dataDir, ".tool_guides_hash")
	if _, err := os.Stat(hashFile); !os.IsNotExist(err) {
		t.Fatalf("content hash written after a failed run: %v", err)
	}
	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	// Calls 1..39 succeeded, so two full batches are published; the third batch
	// failed on call 40 and wrote nothing.
	written := (39 / indexDocumentBatchSize) * indexDocumentBatchSize
	if collection.Count() != written {
		t.Fatalf("documents after the failed run = %d, want %d (completed batches stay written)", collection.Count(), written)
	}

	embedder.reset(0)
	if err := cv.IndexToolGuides(toolsDir, false); err != nil {
		t.Fatalf("resumed IndexToolGuides: %v", err)
	}
	total := collection.Count()
	if total <= written {
		t.Fatalf("resumed run published %d documents, want more than the %d already written", total, written)
	}
	if got := embedder.callCount(); got != total-written {
		t.Fatalf("resumed run embedded %d documents, want only the %d that were missing", got, total-written)
	}
	if data, err := os.ReadFile(hashFile); err != nil || len(data) == 0 {
		t.Fatalf("content hash not written after the successful run: %v", err)
	}

	// Everything is current now: a forced run must not call the provider at all.
	embedder.reset(0)
	if err := cv.IndexToolGuides(toolsDir, true); err != nil {
		t.Fatalf("forced IndexToolGuides: %v", err)
	}
	if got := embedder.callCount(); got != 0 {
		t.Fatalf("forced run re-embedded %d unchanged documents", got)
	}
}

// Each batch has its own, batch-sized deadline; there is no single run-wide one.
func TestIndexToolGuidesGivesEveryBatchItsOwnDeadline(t *testing.T) {
	embedder := &recordingEmbedder{}
	cv := newIndexBatchTestDB(t, embedder)
	if err := cv.IndexToolGuides(t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
	limit := calculateBatchTimeout(indexDocumentBatchSize)
	for i, left := range embedder.deadlines {
		if left <= 0 || left > limit {
			t.Fatalf("call %d ran with %v left, want a deadline within the %v batch timeout", i+1, left, limit)
		}
	}
}

// No guide content may reach the embedding provider before the credential policy
// has accepted it.
func TestIndexToolGuidesRejectsCredentialBeforeEmbedding(t *testing.T) {
	embedder := &recordingEmbedder{}
	cv := newIndexBatchTestDB(t, embedder)
	toolsDir := t.TempDir()
	leaky := "---\ndescription: Leaky guide\n---\n\n# Leaky\n\nkey: " + batchTestSecret + "\n"
	if err := os.WriteFile(filepath.Join(toolsDir, "zz-leaky.md"), []byte(leaky), 0o644); err != nil {
		t.Fatal(err)
	}

	err := cv.IndexToolGuides(toolsDir, true)
	if !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("IndexToolGuides error = %v, want ErrAutomaticMemorySkipped", err)
	}
	if embedder.sawText(batchTestSecret) {
		t.Fatal("credential text reached the embedding provider before validation")
	}
	if _, statErr := os.Stat(filepath.Join(cv.dataDir, ".tool_guides_hash")); !os.IsNotExist(statErr) {
		t.Fatalf("content hash written for a rejected run: %v", statErr)
	}
}

// IndexDirectory validates a file's chunks before embedding them, with and
// without the metadata store.
func TestIndexDirectoryRejectsCredentialBeforeEmbedding(t *testing.T) {
	for _, withStore := range []bool{true, false} {
		name := "without metadata store"
		if withStore {
			name = "with metadata store"
		}
		t.Run(name, func(t *testing.T) {
			embedder := &recordingEmbedder{}
			cv := newIndexBatchTestDB(t, embedder)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "leak.md"), []byte("# Notes\n\npassword token "+batchTestSecret+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var stm *SQLiteMemory
			if withStore {
				var err error
				stm, err = NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
				if err != nil {
					t.Fatal(err)
				}
				defer stm.Close()
			}

			err := cv.IndexDirectory(dir, "docs", stm, true)
			if !errors.Is(err, ErrAutomaticMemorySkipped) {
				t.Fatalf("IndexDirectory error = %v, want ErrAutomaticMemorySkipped", err)
			}
			if embedder.callCount() != 0 || embedder.sawText(batchTestSecret) {
				t.Fatalf("embedding provider was called %d times for a rejected credential file", embedder.callCount())
			}
		})
	}
}

// The derived index is rebuilt from the corpus after a restart, so the batches a
// failed run already published must be recognized and kept there as well.
func TestIndexToolGuidesResumesFromCorpusAfterRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	toolsDir := t.TempDir()
	embedder := &recordingEmbedder{failOn: 40}
	first := newCorpusTestVectorDB(t, dir)
	first.embeddingFunc = embedder.embed
	if err := first.IndexToolGuides(toolsDir, false); err == nil {
		t.Fatal("IndexToolGuides succeeded although the provider failed on call 40")
	}
	written := (39 / indexDocumentBatchSize) * indexDocumentBatchSize
	if got := first.Count(); got != written {
		t.Fatalf("corpus documents after the failed run = %d, want %d", got, written)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	embedder.reset(0)
	second := newCorpusTestVectorDB(t, dir)
	second.embeddingFunc = embedder.embed
	if err := second.IndexToolGuides(toolsDir, false); err != nil {
		t.Fatalf("IndexToolGuides after restart: %v", err)
	}
	total := second.Count()
	if got := embedder.callCount(); total <= written || got != total-written {
		t.Fatalf("after restart embedded %d of %d documents, want only the %d that were missing", got, total, total-written)
	}
}

// A guide whose content is unchanged but whose metadata moved is rewritten with
// the stored vector: the index is corrected without a provider call.
func TestIndexToolGuidesRewritesMovedGuidesWithoutReembedding(t *testing.T) {
	embedder := &recordingEmbedder{}
	cv := newIndexBatchTestDB(t, embedder)
	if err := cv.IndexToolGuides(t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
	movedDir := t.TempDir()
	embedder.reset(0)
	if err := cv.IndexToolGuides(movedDir, true); err != nil {
		t.Fatal(err)
	}
	if got := embedder.callCount(); got != 0 {
		t.Fatalf("moved guides were embedded %d times, want a rewrite with the stored vectors", got)
	}
	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	results, err := collection.QueryEmbedding(t.Context(), []float32{1, 0, 0}, collection.Count(), map[string]string{"tool_name": "docker"}, nil)
	if err != nil || len(results) == 0 {
		t.Fatalf("docker guide documents: %d %v", len(results), err)
	}
	for _, result := range results {
		if got := result.Metadata["path"]; got != filepath.Join(movedDir, "docker.md") {
			t.Fatalf("%s path metadata = %q, want the new tools directory", result.ID, got)
		}
		if doc, err := collection.GetByID(t.Context(), result.ID); err != nil || len(doc.Embedding) != 3 {
			t.Fatalf("%s lost its vector after the rewrite: %v", result.ID, err)
		}
	}
}
