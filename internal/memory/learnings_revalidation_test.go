package memory

import (
	"context"
	"fmt"
	"testing"
)

func TestLearningRevalidationCoversEveryMatchingResource(t *testing.T) {
	s := newTestSTM(t)
	scope := LearningScope{Project: "a", Resource: "main.go"}
	var entries []Learning
	for i := 0; i < 205; i++ {
		entryScope, fingerprint := scope, "before"
		switch i {
		case 201:
			entryScope.Project = "other"
		case 202:
			entryScope.Resource = "other.go"
		case 203:
			fingerprint = ""
		}
		quote := fmt.Sprintf("Keep decision %d", i)
		l, err := s.ObserveLearning(t.Context(), LearningDraft{Statement: quote, Kind: "decision", Confidence: .99}, entryScope, learningSource("human", quote), fingerprint)
		if err != nil {
			t.Fatal(err)
		}
		if i == 204 {
			l, err = s.ReviewLearning(t.Context(), l.ID, l.Revision, l.Scope, "archive", "", "Retired")
			if err != nil {
				t.Fatal(err)
			}
		}
		entries = append(entries, l)
	}
	lastID := ""
	for _, l := range entries[:201] {
		lastID = max(lastID, l.ID)
	}
	if _, err := s.db.Exec(`CREATE TEMP TRIGGER reject_last_learning_revision BEFORE INSERT ON memory_learning_revisions WHEN NEW.id='` + lastID + `' AND NEW.revision=2 BEGIN SELECT RAISE(ABORT,'fixture failure on second page'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.RevalidateLearningResource(t.Context(), scope, "failed-change"); err == nil {
		t.Fatal("injected write failure ignored")
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_last_learning_revision`); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.RevalidateLearningResource(cancelled, scope, "cancelled-change"); err == nil {
		t.Fatal("cancelled revalidation succeeded")
	}
	for _, original := range entries {
		current, err := s.Learning(t.Context(), original.ID, original.Scope)
		if err != nil || current.Revision != original.Revision || current.Fingerprint != original.Fingerprint {
			t.Fatal("failed revalidation retained partial writes", current, err)
		}
	}
	for n, fingerprint := range []string{"changed", "changed-again", "changed-again"} {
		if err := s.RevalidateLearningResource(t.Context(), scope, fingerprint); err != nil {
			t.Fatal(err)
		}
		for i, original := range entries {
			current, err := s.Learning(t.Context(), original.ID, original.Scope)
			if err != nil {
				t.Fatal(err)
			}
			if i >= 201 {
				if current.Revision != original.Revision || current.Status != original.Status || current.Fingerprint != original.Fingerprint {
					t.Fatalf("unrelated entry changed: %+v", current)
				}
				continue
			}
			wantRevision := original.Revision + min(n+1, 2)
			if current.Revision != wantRevision || current.Status != "revalidation" || current.Fingerprint != fingerprint {
				t.Errorf("entry %d after change %d: revision=%d status=%s fingerprint=%s", i, n, current.Revision, current.Status, current.Fingerprint)
			}
		}
	}
	l := entries[0]
	if _, err := s.ReviewLearning(t.Context(), l.ID, l.Revision+1, l.Scope, "confirm", "", "Reviewed first change"); err == nil {
		t.Error("stale review survived a second file change")
	}
	metrics, err := s.LearningMetrics(t.Context())
	if err != nil || metrics["revalidations"] != 402 {
		t.Error("revalidation metrics include duplicates or omit changed entries", metrics, err)
	}
}
