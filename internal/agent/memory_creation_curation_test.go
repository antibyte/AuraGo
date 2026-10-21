package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

type interveningCurationVector struct {
	analysisOwnershipVector
	stm     *memory.SQLiteMemory
	t       *testing.T
	curated chan error
}

func (v *interveningCurationVector) StoreDocumentOwned(concept, content string, mode memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	result, err := v.analysisOwnershipVector.StoreDocumentOwned(concept, content, mode)
	if err != nil {
		return result, err
	}
	if err := v.stm.RecordMemoryEffectiveness("created", true); err != nil {
		v.t.Fatal(err)
	}
	// Human curation shares the automatic-write lock. Queue it while the
	// vector write is in flight, then observe its commit after publication.
	go func() {
		v.curated <- v.stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "created", Action: "confirm", Reason: "human decision"}, "user", false)
	}()
	return result, nil
}

func TestMemoryAnalysisCreatedIDPreservesInterveningCuration(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	vdb := &interveningCurationVector{stm: stm, t: t, curated: make(chan error, 1), analysisOwnershipVector: analysisOwnershipVector{result: memory.VectorStoreResult{CreatedIDs: []string{"created"}}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := storeMemoryAnalysisDocument(logger, stm, vdb, "topic", "fact", memory.MemoryMetaUpdate{VerificationStatus: "unverified", SourceType: "memory_analysis", ExtractionConfidence: .85}); err != nil {
		t.Fatal(err)
	}
	if err := <-vdb.curated; err != nil {
		t.Fatal(err)
	}
	meta, err := stm.GetMemoryMeta("created")
	if err != nil || meta.VerificationStatus != "confirmed" || meta.ReviewNote != "human decision" || meta.UsefulCount != 1 {
		t.Fatalf("curation lost: %+v %v", meta, err)
	}
}
