package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

type interveningCurationVector struct {
	analysisOwnershipVector
	stm *memory.SQLiteMemory
	t   *testing.T
}

func (v *interveningCurationVector) StoreDocumentOwned(concept, content string, mode memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	result, err := v.analysisOwnershipVector.StoreDocumentOwned(concept, content, mode)
	if err != nil {
		return result, err
	}
	if err := v.stm.RecordMemoryEffectiveness("created", true); err != nil {
		v.t.Fatal(err)
	}
	if err := v.stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "created", Action: "confirm", Reason: "human decision"}, "user", false); err != nil {
		v.t.Fatal(err)
	}
	return result, nil
}

func TestMemoryAnalysisCreatedIDPreservesInterveningCuration(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	vdb := &interveningCurationVector{stm: stm, t: t, analysisOwnershipVector: analysisOwnershipVector{result: memory.VectorStoreResult{CreatedIDs: []string{"created"}}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := storeMemoryAnalysisDocument(logger, stm, vdb, "topic", "fact", memory.MemoryMetaUpdate{VerificationStatus: "unverified", SourceType: "memory_analysis", ExtractionConfidence: .85}); err != nil {
		t.Fatal(err)
	}
	meta, err := stm.GetMemoryMeta("created")
	if err != nil || meta.VerificationStatus != "confirmed" || meta.ReviewNote != "human decision" || meta.UsefulCount != 1 {
		t.Fatalf("curation lost: %+v %v", meta, err)
	}
}
