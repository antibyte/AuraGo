package agent

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type conflictScanStub struct {
	getByIDCalls int
}

func (v *conflictScanStub) StoreDocument(string, string) ([]string, error) { return nil, nil }
func (v *conflictScanStub) StoreDocumentWithEmbedding(string, string, []float32) (string, error) {
	return "", nil
}
func (v *conflictScanStub) StoreDocumentInCollection(string, string, string) ([]string, error) {
	return nil, nil
}
func (v *conflictScanStub) StoreDocumentWithEmbeddingInCollection(string, string, []float32, string) (string, error) {
	return "", nil
}
func (v *conflictScanStub) StoreBatch([]memory.ArchiveItem) ([]string, error) { return nil, nil }
func (v *conflictScanStub) SearchSimilar(string, int, ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *conflictScanStub) SearchMemoriesOnly(string, int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *conflictScanStub) GetByIDFromCollection(string, string) (string, error) { return "", nil }
func (v *conflictScanStub) GetByID(string) (string, error) {
	v.getByIDCalls++
	return "", fmt.Errorf("missing")
}
func (v *conflictScanStub) DeleteDocument(string) error                         { return nil }
func (v *conflictScanStub) DeleteDocumentFromCollection(string, string) error     { return nil }
func (v *conflictScanStub) Count() int                                            { return 0 }
func (v *conflictScanStub) IsDisabled() bool                                      { return false }
func (v *conflictScanStub) IsReady() bool                                         { return true }
func (v *conflictScanStub) Close() error                                          { return nil }
func (v *conflictScanStub) StoreCheatsheet(string, string, string, ...string) error { return nil }
func (v *conflictScanStub) DeleteCheatsheet(string) error                           { return nil }
func (v *conflictScanStub) RegisterCollections([]string)                            {}

func TestResolveMaintenanceRetentionDefaults(t *testing.T) {
	retention := resolveMaintenanceRetention(nil)
	if retention.PatternsDays != 90 || retention.ArchiveEventsDays != 90 || retention.MoodLogDays != 30 ||
		retention.ErrorPatternsDays != 7 || retention.ProfileStaleDays != 30 || retention.DoneNotesDays != 7 ||
		retention.OperationalIssuesDays != 30 {
		t.Fatalf("unexpected defaults: %+v", retention)
	}
}

func TestResolveMaintenanceRetentionUsesConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Maintenance.Retention.PatternsDays = 14
	cfg.Maintenance.Retention.DoneNotesDays = 3

	retention := resolveMaintenanceRetention(cfg)
	if retention.PatternsDays != 14 {
		t.Fatalf("PatternsDays = %d, want 14", retention.PatternsDays)
	}
	if retention.DoneNotesDays != 3 {
		t.Fatalf("DoneNotesDays = %d, want 3", retention.DoneNotesDays)
	}
	if retention.ArchiveEventsDays != 90 {
		t.Fatalf("ArchiveEventsDays = %d, want default 90", retention.ArchiveEventsDays)
	}
}

func TestRunNightlyMemoryMaintenanceUsesPrefetchedMetasForCuration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-weak", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.50,
		VerificationStatus:   "unverified",
		SourceReliability:    0.50,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	if err := stm.SetMemoryMetaLastAccessed("doc-weak", time.Now().UTC().Add(-60*24*time.Hour)); err != nil {
		t.Fatalf("SetMemoryMetaLastAccessed: %v", err)
	}

	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("meta count = %d, want 1", len(metas))
	}

	cfg := &config.Config{}
	cfg.Consolidation.AutoOptimize = true
	cfg.MemoryAnalysis.AutoConfirm = 0.92

	runNightlyMemoryMaintenance(cfg, logger, nil, stm, nil, nil, 0)

	metas, err = stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta after curation: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("meta count after curation = %d, want 1", len(metas))
	}
	if metas[0].VerificationStatus != "archived" {
		t.Fatalf("VerificationStatus = %q, want archived", metas[0].VerificationStatus)
	}
}

func TestDetectMemoryConflictsAcrossLTMTruncatesPrefetchedMetas(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	metas := make([]memory.MemoryMeta, nightlyMemoryConflictScanLimit+50)
	for i := range metas {
		metas[i].DocID = fmt.Sprintf("doc-%d", i)
	}

	ltm := &conflictScanStub{}
	detectMemoryConflictsAcrossLTM(logger, stm, ltm, metas)

	if ltm.getByIDCalls != nightlyMemoryConflictScanLimit {
		t.Fatalf("getByIDCalls = %d, want %d", ltm.getByIDCalls, nightlyMemoryConflictScanLimit)
	}
}