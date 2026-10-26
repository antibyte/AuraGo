package memory

import (
	"aurago/internal/chunking"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const precheckCredential = "api_key=fixture-actual-api-credential-7485"

func newCountingChromemVectorDB(t *testing.T) (*ChromemVectorDB, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		return []float32{1, 0, 0}, nil
	})
	t.Cleanup(func() { cv.Close() })
	return cv, &calls
}

// A credential-bearing file chunk must be refused locally; an external provider
// must never see it.
func TestReplaceIndexedFileRejectsCredentialBeforeEmbedding(t *testing.T) {
	cv, calls := newCountingChromemVectorDB(t)
	s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	path := filepath.Join(t.TempDir(), "deploy.env")
	state := FileIndexState{LastModified: time.Now().UTC(), ContentHash: "first", IndexFingerprint: "v1"}

	ids, err := cv.ReplaceIndexedFile(t.Context(), s, path, "file_index", "file", "deploy notes\n"+precheckCredential+"\n", nil, chunking.DefaultOptions(), nil, state)
	if !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("credential file error = %v, want ErrAutomaticMemorySkipped", err)
	}
	if len(ids) != 0 {
		t.Fatalf("rejected file returned document IDs %v", ids)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("embedding provider was called %d times for a rejected credential chunk", got)
	}
	if active, err := s.GetFileEmbeddingDocIDs(path, "file_index"); err != nil || len(active) != 0 {
		t.Fatalf("rejected file published a pointer: %v %v", active, err)
	}

	// A clean file still embeds, so the early check is not a blanket refusal.
	state.ContentHash = "second"
	if ids, err := cv.ReplaceIndexedFile(t.Context(), s, path, "file_index", "file", "plain deployment notes", nil, chunking.DefaultOptions(), nil, state); err != nil || len(ids) == 0 {
		t.Fatalf("clean file ids=%v err=%v", ids, err)
	}
	if calls.Load() == 0 {
		t.Fatal("clean file was never embedded")
	}
}

// Corrections embed the replacement value before they stage it; the credential
// policy of normal memory writes must run first, at preview and at apply time.
func TestMemoryCorrectionRejectsCredentialBeforeEmbedding(t *testing.T) {
	c, _, oldID := correctionFixture(t)
	cv := c.Vector.(*ChromemVectorDB)
	var calls atomic.Int64
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		return []float32{1, 0, 0}, nil
	}
	change := func(value string) []MemoryCorrectionChange {
		return []MemoryCorrectionChange{{Reference: MemoryReference{Store: "ltm", ID: oldID}, Operation: "replace", Value: value}}
	}

	if _, err := c.Preview(t.Context(), "chat", "turn", "User corrected their preference", change("User prefers "+precheckCredential), correctionTestEvidence()); !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("preview of a credential value error = %v, want ErrAutomaticMemorySkipped", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("preview embedded a credential value (%d provider calls)", got)
	}

	// A stored plan is replayed without preview; tamper it to prove Apply re-checks.
	plan, err := c.Preview(t.Context(), "chat", "turn", "User corrected their preference", change(correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	calls.Store(0)
	plan.Items[0].Value = "User prefers " + precheckCredential
	if err := saveMemoryCorrection(t.Context(), c.Memory.db, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Apply(t.Context(), plan.ID, "chat"); !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("apply of a credential plan error = %v, want ErrAutomaticMemorySkipped", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("apply embedded a credential value (%d provider calls)", got)
	}

	// The embedding choke point refuses on its own, independent of the plan checks.
	plan.Items[0].State = "pending"
	if _, err := cv.prepareCorrectionVectors(t.Context(), plan); !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("prepareCorrectionVectors error = %v, want ErrAutomaticMemorySkipped", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("prepareCorrectionVectors embedded a credential value (%d provider calls)", got)
	}
}
