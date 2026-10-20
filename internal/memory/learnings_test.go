package memory

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/security"
)

func learningSource(session, quote string) LearningSource {
	return LearningSource{Reference: "user:current", Session: session, Kind: "user", Quote: quote, Hash: learningHash(quote)}
}

func TestLearningNeverPersistsRegisteredSecrets(t *testing.T) {
	s := newTestSTM(t)
	secret := "learning-fixture-sensitive-9a7c42"
	security.RegisterSensitive(secret)
	d := LearningDraft{Statement: "Keep the backend portable", Rationale: secret, Kind: "decision", Confidence: .99}
	if _, err := s.ObserveLearning(t.Context(), d, LearningScope{}, learningSource("s1", d.Statement), ""); err == nil {
		t.Fatal("sensitive rationale accepted")
	}
	d.Rationale = "Portable deployment"
	l, err := s.ObserveLearning(t.Context(), d, LearningScope{}, learningSource("s1", d.Statement), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReviewLearning(t.Context(), l.ID, l.Revision, l.Scope, "archive", "", secret); err == nil {
		t.Fatal("sensitive revision reason accepted")
	}
	if err := s.RecordLearningDeviation(t.Context(), "run", "hash", secret, map[string]int{l.ID: l.Revision}); err == nil {
		t.Fatal("sensitive deviation accepted")
	}
	history, err := s.LearningHistory(t.Context(), l.ID, l.Scope)
	if err != nil || len(history) != 1 {
		t.Fatal("rejected update changed history", err)
	}
}

func TestLearningEvidenceIsolationAndHistory(t *testing.T) {
	s := newTestSTM(t)
	ctx := context.Background()
	scope := LearningScope{Project: "a", Resource: "main.go"}
	d := LearningDraft{Statement: "Keep the Go backend portable", Rationale: "Single binary", Kind: "preference", Confidence: 0.99}
	l, err := s.ObserveLearning(ctx, d, scope, learningSource("s1", "I prefer portable Go"), "hash1")
	if err != nil || l.Status != "proposed" {
		t.Fatalf("first: %+v %v", l, err)
	}
	duplicate, err := s.ObserveLearning(ctx, d, scope, learningSource("s1", "I prefer portable Go"), "hash1")
	if err != nil || duplicate.Revision != l.Revision {
		t.Fatal("duplicate changed revision", err)
	}
	l, err = s.ObserveLearning(ctx, d, scope, learningSource("s1", "Again, portable Go"), "hash1")
	if err != nil || l.Status != "proposed" {
		t.Fatal("one session promoted preference", err)
	}
	l, err = s.ObserveLearning(ctx, d, scope, learningSource("s2", "I still prefer portable Go"), "hash1")
	if err != nil || l.Status != "active" {
		t.Fatal("independent evidence failed", err)
	}
	for _, other := range []LearningScope{{}, {Project: "b", Resource: "main.go"}, {Project: "a", Resource: "other.go"}} {
		list, err := s.ListLearnings(ctx, other, "Go", "active", 3)
		if err != nil || len(list) != 0 {
			t.Fatalf("scope leak: %+v %v", list, err)
		}
		if _, err := s.Learning(ctx, l.ID, other); err != sql.ErrNoRows {
			t.Fatal("ID leaked", err)
		}
	}
	if err := s.RevalidateLearningResource(ctx, scope, "hash2"); err != nil {
		t.Fatal(err)
	}
	l, err = s.Learning(ctx, l.ID, scope)
	if err != nil || l.Status != "revalidation" {
		t.Fatal("drift not recorded", err)
	}
	if _, err = s.ReviewLearning(ctx, l.ID, l.Revision-1, scope, "confirm", "", "reviewed"); err == nil {
		t.Fatal("stale review accepted")
	}
	l, err = s.ReviewLearning(ctx, l.ID, l.Revision, scope, "archive", "", "obsolete")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.ObserveLearning(ctx, d, scope, learningSource("s3", "I prefer portable Go"), "hash2")
	if err != nil || again.Status != "archived" || again.Revision != l.Revision {
		t.Fatal("archive resurrected", err)
	}
	history, err := s.LearningHistory(ctx, l.ID, scope)
	if err != nil || len(history) != l.Revision {
		t.Fatal("incomplete history", len(history), l.Revision, err)
	}
}

func TestLearningConflictsAndCancellation(t *testing.T) {
	s := newTestSTM(t)
	ctx := context.Background()
	scope := LearningScope{Project: "a"}
	d := LearningDraft{Statement: "Use port 443", Kind: "decision", Confidence: .99, Condition: &LearningCondition{Target: "server", Field: "port", Value: "443"}}
	l, err := s.ObserveLearning(ctx, d, scope, learningSource("s1", "Use port 443"), "")
	if err != nil || l.Status != "active" {
		t.Fatal(err)
	}
	d.Statement = "Use port 80"
	d.Condition = &LearningCondition{Target: "server", Field: "port", Value: "80"}
	l, err = s.ObserveLearning(ctx, d, scope, learningSource("s2", "Use port 80"), "")
	if err != nil || l.Status != "proposed" {
		t.Fatal("conflict promoted", err)
	}
	if _, err = s.ReviewLearning(ctx, l.ID, l.Revision, scope, "confirm", "", "review"); err == nil {
		t.Fatal("conflict confirmed without resolution")
	}
	d.Alternatives = []string{"invented option"}
	if _, err = s.ObserveLearning(ctx, d, scope, learningSource("s2", "Use port 80"), ""); err == nil {
		t.Fatal("invented alternative accepted")
	}
	d.Alternatives = nil
	source := learningSource("s3", "Use port 80")
	source.Kind = "technical"
	if _, err = s.ObserveLearning(ctx, d, scope, source, ""); err == nil {
		t.Fatal("machine authorized user decision")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.ObserveLearning(cancelled, d, scope, learningSource("s4", "Use port 80"), ""); err == nil {
		t.Fatal("cancelled write accepted")
	}
}

func TestLearningMigrationBacksUpCopyAndRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`INSERT INTO core_memory(fact,normalized_fact) VALUES('keep me','keep me'); DROP TABLE memory_learnings; DROP TABLE memory_learning_revisions; DROP TABLE memory_learning_metrics; DELETE FROM memory_schema_meta WHERE key='learnings'; PRAGMA user_version=11`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	files, err := filepath.Glob(path + ".learnings-v1-*.bak")
	if err != nil || len(files) != 1 {
		t.Fatal("backup missing", files, err)
	}
	copyDB, err := sql.Open("sqlite", files[0])
	if err != nil {
		t.Fatal(err)
	}
	var integrity, fact string
	if err = copyDB.QueryRow(`PRAGMA quick_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("bad backup", err)
	}
	if err = copyDB.QueryRow(`SELECT fact FROM core_memory`).Scan(&fact); err != nil || fact != "keep me" {
		t.Fatal("lost existing data", err)
	}
	copyDB.Close()
	// Rehearse the actual migration on the consistent copy, not a new empty store.
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	backupBytes, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, backupBytes, 0600); err != nil {
		t.Fatal(err)
	}
	copyStore, err := NewSQLiteMemory(copyPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	copyStore.Close()
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	files, _ = filepath.Glob(path + ".learnings-v1-*.bak")
	if len(files) != 1 {
		t.Fatal("migration repeated")
	}
}
