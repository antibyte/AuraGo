package memory

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAutomaticMemoryPublicationRetryDefersOnlyItsPendingFact(t *testing.T) {
	for _, content := range []string{"The NAS uses XFS for its data volume.", strings.Repeat("The NAS uses XFS for its data volume. ", 150)} {
		name := "single document"
		if len(content) > 4000 {
			name = "chunked document"
		}
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) {
				calls.Add(1)
				return []float32{1, 0, 0}, nil
			})
			if _, err := stm.db.Exec(`INSERT INTO memory_maintenance_meta(key,value) VALUES('unrelated.status','not-json')`); err != nil {
				t.Fatal(err)
			}
			if _, err := vector.corpus.db.Exec(`CREATE TRIGGER fail_activation BEFORE UPDATE OF pending ON documents WHEN NEW.pending=0 BEGIN SELECT RAISE(ABORT,'activation unavailable'); END`); err != nil {
				t.Fatal(err)
			}
			details := MemoryMetaUpdate{SourceType: "chat", SourceSessionID: "retry-session", VerificationStatus: "unverified"}
			write := func(concept, body string) (AutomaticMemoryStoreResult, error) {
				return StoreAutomaticMemoryDocumentWithOptionsContext(t.Context(), stm, vector, concept, body, details, AutomaticMemoryWriteOptions{Validate: func() error { return nil }})
			}
			first, err := write("NAS storage", content)
			if err == nil || len(first.CreatedIDs) == 0 {
				t.Fatalf("first publication=%+v err=%v", first, err)
			}
			if _, err := vector.corpus.db.Exec(`DROP TRIGGER fail_activation`); err != nil {
				t.Fatal(err)
			}
			before := calls.Load()
			repeated, err := write("NAS storage", content)
			if !errors.Is(err, ErrAutomaticMemoryPublicationPending) || len(repeated.CreatedIDs) != 0 || calls.Load() != before {
				t.Fatalf("retry=%+v err=%v embedding calls=%d before=%d", repeated, err, calls.Load(), before)
			}
			if _, err := write("Preferred editor", "The preferred editor is Vim."); err != nil {
				t.Fatalf("pending fact blocked an unrelated fact: %v", err)
			}
			before = calls.Load()
			if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != before {
				t.Fatal("recovery unnecessarily embedded prepared documents")
			}
			var count int
			if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection='aurago_memories'`).Scan(&count); err != nil || count != len(first.CreatedIDs)+1 {
				t.Fatalf("corpus count=%d want=%d err=%v", count, len(first.CreatedIDs)+1, err)
			}
		})
	}
}

func TestAutomaticMemoryPublicationRetryIgnoresRecycledReceipt(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	_, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	if _, err := vector.corpus.db.Exec(`UPDATE documents SET metadata=json_remove(metadata,'$.memory_publication_id'),pending=0,blocked_reason='' WHERE id=?`, receipts[0].ID); err != nil {
		t.Fatal(err)
	}
	stored, err := StoreAutomaticMemoryDocumentWithOptionsContext(t.Context(), stm, vector,
		"[preference] User prefers local tools", "Use local tools for private files.",
		MemoryMetaUpdate{SourceType: "chat", SourceSessionID: "default"}, AutomaticMemoryWriteOptions{Validate: func() error { return nil }})
	if err != nil || len(stored.CreatedIDs) != 1 {
		t.Fatalf("foreign receipt blocked a new write: %+v err=%v", stored, err)
	}
	if _, err := vector.corpusDocument(t.Context(), "aurago_memories", receipts[0].ID); err != nil {
		t.Fatalf("foreign document was removed: %v", err)
	}
}
