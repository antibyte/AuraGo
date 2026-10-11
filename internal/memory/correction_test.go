package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

const correctionOld = "User prefers black coffee"
const correctionNew = "User prefers green tea"

func correctionTestEvidence() []CorrectionEvidence {
	return []CorrectionEvidence{{Reference: "user:current", Kind: "user", Quote: "I prefer green tea", ContentHash: "runtime-test-hash"}}
}

func correctionFixture(t *testing.T) (*MemoryCorrectionService, MemoryReference, string) {
	t.Helper()
	s := automaticTestStore(t)
	v := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	g := newTestKG(t)
	id, err := s.AddCoreMemoryFact(correctionOld)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpsertProfileEntry("prefs", "drink", correctionOld, "analysis"); err != nil {
		t.Fatal(err)
	}
	if err = g.AddNode(fmt.Sprintf("core_fact_%d", id), correctionOld, map[string]string{"source": "core_memory", "content": correctionOld}); err != nil {
		t.Fatal(err)
	}
	stored, err := StoreAutomaticMemoryDocument(s, v, "preference", correctionOld, MemoryMetaUpdate{SourceType: "user", VerificationStatus: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	return &MemoryCorrectionService{Memory: s, Vector: v, Graph: g, Access: CorrectionAccess{true, true, true}}, MemoryReference{Store: "core", ID: fmt.Sprint(id)}, stored.CreatedIDs[0]
}

func correctionFixtureChanges(ref MemoryReference, vectorID, operation, value string) []MemoryCorrectionChange {
	return []MemoryCorrectionChange{
		{Reference: ref, Operation: operation, Value: value},
		{Reference: MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}, Operation: operation, Value: value},
		{Reference: MemoryReference{Store: "ltm", ID: vectorID}, Operation: operation, Value: value},
	}
}

func TestMemoryCorrectionFourStoresAndReplay(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "User corrected their preference", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 4 {
		t.Fatalf("items=%+v", p.Items)
	}
	p, err = c.Apply(t.Context(), p.ID, "chat")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "applied" {
		t.Fatalf("status=%s", p.Status)
	}
	for _, item := range p.Items {
		if item.State != "done" || correctionIdentity(item.Before) != correctionIdentity(correctionOld) {
			t.Fatalf("receipt=%+v", item)
		}
	}
	if !strings.Contains(c.Memory.GetUserProfileSummary(99), normalizeProfileValue(correctionNew)) {
		t.Fatal("corrected profile not active")
	}
	if _, err = c.Vector.GetByIDFromCollection(oldID, "aurago_memories"); err != nil {
		t.Fatal("old vector lost", err)
	}
	oldMeta, _ := c.Memory.GetMemoryMeta(oldID)
	if !IsMemoryArchived(oldMeta) {
		t.Fatal("old vector active")
	}
	count := c.Vector.Count()
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	if c.Vector.Count() != count {
		t.Fatal("replay duplicated vectors")
	}
	if err = c.Memory.UpsertProfileEntry("prefs", "drink", correctionOld, "analysis"); err != nil {
		t.Fatal(err)
	}
	if err = c.Memory.EnforceProfileSizeLimit(0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Memory.GetUserProfileSummary(1), normalizeProfileValue(correctionNew)) {
		t.Fatal("analysis or cleanup replaced correction")
	}
	if _, err = c.Memory.AddCoreMemoryFact(correctionOld); err == nil {
		t.Fatal("old core fact resurrected")
	}
	if _, err = StoreAutomaticMemoryDocument(c.Memory, c.Vector, "[preference:drink] "+correctionOld, "source:memory_analysis session:old", MemoryMetaUpdate{SourceType: "memory_analysis"}); err == nil {
		t.Fatal("old extraction resurrected fact")
	}
	if _, err = c.Graph.UpdateNode("core_fact_"+ref.ID, correctionOld, map[string]string{"content": correctionOld}); err == nil {
		t.Fatal("old graph fact resurrected")
	}
	var currentID string
	for _, item := range p.Items {
		if item.Reference.Store == "ltm" {
			currentID = item.ReplacementID
		}
	}
	next, err := c.Preview(t.Context(), "chat", "new-turn", "User changed their preference again", correctionFixtureChanges(ref, currentID, "replace", correctionOld), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), next.ID, "chat"); err != nil {
		t.Fatal("new evidenced correction blocked", err)
	}
}

func TestMemoryCorrectionArchiveAndProtection(t *testing.T) {
	for _, mode := range []string{"archive", "protected", "readonly", "profile-disabled", "stale", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			c, ref, vectorID := correctionFixture(t)
			p, err := c.Preview(t.Context(), "chat", "turn", "Withdraw incorrect preference", correctionFixtureChanges(ref, vectorID, "archive", ""), correctionTestEvidence())
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			switch mode {
			case "protected":
				_, err = c.Memory.db.Exec(`UPDATE memory_meta SET protected=1 WHERE doc_id=?`, vectorID)
			case "readonly":
				c.Access.GraphWrite = false
			case "profile-disabled":
				c.Access.ProfileEnabled = false
			case "stale":
				err = c.Memory.UpsertProfileEntry("prefs", "drink", "another value", "analysis")
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			p, err = c.Apply(ctx, p.ID, "chat")
			if mode != "archive" {
				if err == nil {
					t.Fatal("expected denial")
				}
				facts, _ := c.Memory.GetCoreMemoryFacts()
				if len(facts) != 1 || facts[0].Fact != correctionOld {
					t.Fatal("preflight wrote facts")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			facts, _ := c.Memory.GetCoreMemoryFacts()
			if len(facts) != 0 {
				t.Fatal("core remains")
			}
			profile, _ := c.Memory.GetProfileEntries("")
			if len(profile) != 0 {
				t.Fatal("profile remains")
			}
			node, _ := c.Graph.GetNode("core_fact_" + ref.ID)
			if node.Properties["content"] != "" || strings.Contains(node.Label, "coffee") {
				t.Fatal("graph remains")
			}
			meta, _ := c.Memory.GetMemoryMeta(vectorID)
			if !IsMemoryArchived(meta) {
				t.Fatal("vector remains active")
			}
			if _, err = c.Memory.MemoryCorrection(p.ID, "another-session"); err == nil {
				t.Fatal("cross-session history disclosed")
			}
		})
	}
}

type failingCorrectionVector struct {
	*ChromemVectorDB
	fail bool
}

func (v *failingCorrectionVector) StoreCorrectionDocument(ctx context.Context, id, text string) error {
	if v.fail {
		return errors.New("embedding unavailable")
	}
	return v.ChromemVectorDB.StoreCorrectionDocument(ctx, id, text)
}

func TestMemoryCorrectionEmbeddingFailureAndResume(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	v := &failingCorrectionVector{c.Vector.(*ChromemVectorDB), true}
	c.Vector = v
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	p, err = c.Apply(t.Context(), p.ID, "chat")
	if err == nil || p.Status != "incomplete" {
		t.Fatal("failure reported as success")
	}
	facts, _ := c.Memory.GetCoreMemoryFacts()
	if facts[0].Fact != correctionOld {
		t.Fatal("original changed before embedding")
	}
	for _, item := range p.Items {
		if item.ReplacementID != "" {
			meta, _ := c.Memory.GetMemoryMeta(item.ReplacementID)
			if !IsMemoryArchived(meta) {
				t.Fatal("staging visible")
			}
		}
	}
	v.fail = false
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryCorrectionLocalFailureUsesDurableReceipt(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Memory.db.Exec(`CREATE TRIGGER fail_profile_correction BEFORE UPDATE ON user_profile BEGIN SELECT RAISE(ABORT,'injected failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	p, err = c.Apply(t.Context(), p.ID, "chat")
	if err == nil {
		t.Fatal("missing injected failure")
	}
	if p.Items[0].State != "done" {
		t.Fatal("committed core receipt lost")
	}
	for _, item := range p.Items {
		if item.Reference.Store == "profile" && item.State == "done" {
			t.Fatal("rolled back profile marked done")
		}
	}
	if _, err = c.Memory.db.Exec(`DROP TRIGGER fail_profile_correction`); err != nil {
		t.Fatal(err)
	}
	// A fresh coordinator resumes from the durable plan, not transient local state.
	next := *c
	if _, err = next.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryCorrectionMigrationBacksUpPopulatedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddCoreMemoryFact(correctionOld); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER core_correction_insert; DROP TRIGGER core_correction_update; DROP TABLE memory_corrections; DROP TABLE memory_profile_corrections; DROP TABLE memory_correction_guards`); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups, err := filepath.Glob(path + ".memory-corrections-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v %v", backups, err)
	}
	db, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var fact, integrity string
	if err = db.QueryRow(`SELECT fact FROM core_memory`).Scan(&fact); err != nil || fact != correctionOld {
		t.Fatal("backup lost facts", err)
	}
	if err = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("inconsistent backup", err)
	}
}

func TestMemoryCorrectionFreshEvidenceCanRestoreArchivedTargets(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "Withdraw preference", correctionFixtureChanges(ref, oldID, "archive", ""), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	changes := []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: correctionNew}, {Reference: MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}, Operation: "replace", Value: correctionNew}, {Reference: MemoryReference{Store: "ltm", ID: oldID}, Operation: "replace", Value: correctionNew}}
	next, err := c.Preview(t.Context(), "chat", "new-turn", "New explicit preference", changes, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), next.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Memory.GetUserProfileSummary(1), "green tea") {
		t.Fatal("profile tombstone could not be corrected")
	}
}

func TestMemoryCorrectionReadActivityDoesNotInvalidatePreview(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Memory.db.Exec(`UPDATE memory_meta SET access_count=access_count+1,last_accessed=CURRENT_TIMESTAMP WHERE doc_id=?`, oldID); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal("ordinary read blocked correction", err)
	}
}

func TestMemoryCorrectionKeepsMultilineHistoryAndArchivedVectors(t *testing.T) {
	c, _, oldID := correctionFixture(t)
	const replacement = "User prefers green tea\n\nOnly without added sugar"
	p, err := c.Preview(t.Context(), "chat", "turn", "User clarified preference", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "ltm", ID: oldID}, Operation: "replace", Value: replacement}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	var correctedID string
	for _, item := range p.Items {
		if item.Reference.Store == "ltm" {
			correctedID = item.ReplacementID
		}
	}
	next, err := c.Preview(t.Context(), "chat", "later", "Withdraw preference", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "ltm", ID: correctedID}, Operation: "archive"}}, correctionTestEvidence())
	if err != nil || next.Items[0].Before != replacement {
		t.Fatal("multiline correction lost its first paragraph", err)
	}
	if _, err = c.Apply(t.Context(), next.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Memory.ApplyMemoryBudgetEnforcement(1, c.Vector); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{oldID, correctedID} {
		if _, err = c.Vector.GetByIDFromCollection(id, "aurago_memories"); err != nil {
			t.Fatal("budget removed correction history", err)
		}
	}
}

func TestMemoryCorrectionKeepsTechnicalProvenanceForNewObservations(t *testing.T) {
	s := automaticTestStore(t)
	v := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0}, nil })
	stored, err := StoreAutomaticMemoryDocument(s, v, "device OS", "The server runs version one", MemoryMetaUpdate{SourceType: "system_metrics", VerificationStatus: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	c := &MemoryCorrectionService{Memory: s, Vector: v, Access: CorrectionAccess{MemoryWrite: true}}
	id := stored.CreatedIDs[0]
	for _, value := range []string{"The server runs version two", "The server runs version three"} {
		evidence := []CorrectionEvidence{{Reference: "tool:runtime", Kind: "tool", Quote: value, ContentHash: contentSHA256(value)}}
		p, err := c.Preview(t.Context(), "chat", value, "Fresh system observation", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "ltm", ID: id}, Operation: "replace", Value: value}}, evidence)
		if err != nil {
			t.Fatal("a previous correction lost technical provenance", err)
		}
		if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
			t.Fatal(err)
		}
		id = p.Items[0].ReplacementID
		meta, err := s.GetMemoryMeta(id)
		if err != nil || meta.SourceType != "memory_correction:system_metrics" {
			t.Fatal("replacement source not correction-owned", err)
		}
	}
}
