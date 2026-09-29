package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestMaintenanceBaselineLogsConflictFailure(t *testing.T) {
	stm, _ := maintenanceRegressionStores(t)
	if err := stm.UpsertMemoryMetaWithDetails("missing-active", memory.MemoryMetaUpdate{VerificationStatus: "confirmed"}); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	err := runNightlyMemoryBaselineWithContext(t.Context(), &config.Config{}, logger, stm, &baselineConflictVectorDB{})
	if err == nil || !strings.Contains(logs.String(), "memory_conflict_scan") || !strings.Contains(logs.String(), "read memory conflict document") {
		t.Fatalf("baseline error must retain its failing step: error=%v logs=%s", err, logs.String())
	}
}

type baselineConflictVectorDB struct {
	conflictScanStub
	documents map[string]string
	readIDs   []string
	searches  int
}

func (v *baselineConflictVectorDB) GetByID(id string) (string, error) {
	v.readIDs = append(v.readIDs, id)
	if text, ok := v.documents[id]; ok {
		return text, nil
	}
	return "", fmt.Errorf("document not found: %s", id)
}

func (v *baselineConflictVectorDB) SearchMemoriesOnly(string, int) ([]string, []string, error) {
	v.searches++
	return nil, nil, nil
}

func TestMaintenanceConflictScanSkipsArchivedBeforeLimit(t *testing.T) {
	for _, prefetched := range []bool{false, true} {
		t.Run(fmt.Sprintf("prefetched=%t", prefetched), func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			stm, err := memory.NewSQLiteMemory(":memory:", logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = stm.Close() })
			ltm := &baselineConflictVectorDB{documents: make(map[string]string)}
			for _, status := range []string{"archived", "confirmed"} {
				for i := 0; i < nightlyMemoryConflictScanLimit+1; i++ {
					id := fmt.Sprintf("%s-%03d", status, i)
					if err := stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{VerificationStatus: status}); err != nil {
						t.Fatal(err)
					}
					if status == "confirmed" {
						ltm.documents[id] = "A stable memory without conflict signals."
					}
				}
			}
			var metas []memory.MemoryMeta
			if prefetched {
				metas, err = stm.GetAllMemoryMeta(nightlyMemoryMetaFetchLimit, 0)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := detectMemoryConflictsAcrossLTMWithContext(context.Background(), logger, stm, ltm, metas); err != nil {
				t.Fatal(err)
			}
			if len(ltm.readIDs) != nightlyMemoryConflictScanLimit {
				t.Fatalf("read %d documents, want %d active documents", len(ltm.readIDs), nightlyMemoryConflictScanLimit)
			}
			for _, id := range ltm.readIDs {
				if !strings.HasPrefix(id, "confirmed-") {
					t.Fatalf("read archived document %q", id)
				}
			}
		})
	}
}

func TestMaintenanceBaselineSkipsMemoryArchivedDuringCuration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.UpsertMemoryMetaWithDetails("stale", memory.MemoryMetaUpdate{
		VerificationStatus: "unverified", ExtractionConfidence: 0.50, SourceReliability: 0.50,
	}); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryMetaLastAccessed("stale", time.Now().UTC().Add(-60*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	ltm := &baselineConflictVectorDB{}
	if err := runNightlyMemoryBaselineWithContext(context.Background(), &config.Config{}, logger, stm, ltm); err != nil {
		t.Fatal(err)
	}
	meta, err := stm.GetMemoryMeta("stale")
	if err != nil || !memory.IsMemoryArchived(meta) || len(ltm.readIDs) != 0 {
		t.Fatalf("archived=%t, reads=%v, error=%v", memory.IsMemoryArchived(meta), ltm.readIDs, err)
	}
}

func TestMemoryConflictScanHonorsCurrentArchiveStateAndErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	for _, status := range []string{"archived", "confirmed"} {
		if err := stm.UpsertMemoryMetaWithDetails(status, memory.MemoryMetaUpdate{VerificationStatus: status}); err != nil {
			t.Fatal(err)
		}
	}
	ltm := &baselineConflictVectorDB{}
	for _, fallback := range []string{"", "User prefers German"} {
		if err := detectMemoryConflictsForDocIDs(logger, stm, ltm, []string{"archived"}, fallback); err != nil {
			t.Fatal(err)
		}
	}
	if len(ltm.readIDs) != 0 || ltm.searches != 0 {
		t.Fatalf("archived memory used: reads=%v, searches=%d", ltm.readIDs, ltm.searches)
	}
	for _, id := range []string{"confirmed", "missing-metadata"} {
		if err := detectMemoryConflictsForDocIDs(logger, stm, ltm, []string{id}, ""); err == nil {
			t.Fatalf("missing active document %q must remain an error", id)
		}
	}
	if len(ltm.readIDs) != 2 {
		t.Fatalf("active reads=%v, want two", ltm.readIDs)
	}
	if err := detectMemoryConflictsForDocIDs(logger, stm, ltm, []string{"confirmed"}, "User prefers German"); err != nil {
		t.Fatal(err)
	}
	if ltm.searches != 1 {
		t.Fatalf("active fallback must be checked, searches=%d", ltm.searches)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := detectMemoryConflictsForDocIDs(logger, stm, ltm, []string{"confirmed"}, "User prefers German"); err == nil {
		t.Fatal("metadata read failure must remain an error")
	}
}
