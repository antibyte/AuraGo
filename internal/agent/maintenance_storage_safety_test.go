package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type maintenanceOwnedVector struct {
	hierarchyVectorDB
	fail         bool
	forceCreated int
}

func (v *maintenanceOwnedVector) StoreDocumentOwned(concept, content string, mode memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	if mode == memory.VectorStoreDeduplicate {
		if _, ok := v.stored[concept]; ok {
			return memory.VectorStoreResult{ReusedIDs: []string{concept}}, nil
		}
	}
	if mode == memory.VectorStoreForceCreate {
		v.forceCreated++
	}
	id := fmt.Sprintf("new_%d", len(v.stored))
	v.stored[id] = content
	if v.fail {
		return memory.VectorStoreResult{CreatedIDs: []string{id}}, fmt.Errorf("partial store failure")
	}
	return memory.VectorStoreResult{CreatedIDs: []string{id}}, nil
}

func (v *maintenanceOwnedVector) DeleteDocumentIfContentMatches(id, expected string) (bool, error) {
	if fmt.Sprintf("%x", sha256.Sum256([]byte(v.stored[id]))) != expected {
		return false, nil
	}
	delete(v.stored, id)
	return true, nil
}

func TestMaintenanceConsolidationPreservesReusedMemoryOnPartialWrite(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	if err := stm.UpsertMemoryMetaWithDetails("existing", memory.MemoryMetaUpdate{VerificationStatus: "confirmed", SourceType: "user", ExtractionConfidence: .99, SourceReliability: .99}); err != nil {
		t.Fatal(err)
	}
	before, err := stm.GetMemoryMeta("existing")
	if err != nil {
		t.Fatal(err)
	}
	vdb := &maintenanceOwnedVector{hierarchyVectorDB: hierarchyVectorDB{stored: map[string]string{"existing": "preserved content"}}, fail: true}
	_, _, err = storeConsolidationFacts(logger, stm, vdb, []helperConsolidationFact{{Concept: "existing", Content: "duplicate"}, {Concept: "new", Content: "new content"}})
	if err == nil {
		t.Fatal("expected partial write error")
	}
	if len(vdb.stored) != 1 || vdb.stored["existing"] != "preserved content" {
		t.Fatalf("rollback lost existing vector: %v", vdb.stored)
	}
	after, err := stm.GetMemoryMeta("existing")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("reused metadata changed: before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestMaintenanceCompressionForceCreatesBeforeRetirement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	if err := stm.UpsertMemoryMeta("original"); err != nil {
		t.Fatal(err)
	}
	meta, err := stm.GetMemoryMeta("original")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Consolidation.OptimizeThreshold = adjustedMemoryPriority(meta, time.Now()) - 1
	vdb := &maintenanceOwnedVector{hierarchyVectorDB: hierarchyVectorDB{stored: map[string]string{"original": "original\n\n" + strings.Repeat("Long original memory. ", 30)}}}
	result := autoOptimizeMemoryWithContext(context.Background(), cfg, logger, &countingMaintenanceLLMClient{}, vdb, stm, nil, []memory.MemoryMeta{meta})
	if len(result.Errors) != 0 {
		t.Fatalf("optimization failed: %v", result.Errors)
	}
	if vdb.forceCreated != 1 || len(vdb.stored) != 1 {
		t.Fatalf("force-created=%d vectors=%v", vdb.forceCreated, vdb.stored)
	}
	if _, exists := vdb.stored["original"]; exists {
		t.Fatal("original was not retired")
	}
}

func TestManualMemoryOptimizationLegacyReplacementIsPartialAndPreservesOriginal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	if err := stm.UpsertMemoryMeta("original"); err != nil {
		t.Fatal(err)
	}
	original := "original\n\n" + strings.Repeat("Long original memory. ", 30)
	vdb := &hierarchyVectorDB{stored: map[string]string{"original": original}}
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	raw := runMemoryOrchestrator(memoryOrchestratorArgs{ThresholdLow: -100, ThresholdMedium: 100}, cfg, logger, &countingMaintenanceLLMClient{}, vdb, stm, nil)
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "partial" || vdb.stored["original"] != original || len(vdb.stored) != 1 {
		t.Fatalf("result=%s vectors=%v", raw, vdb.stored)
	}
}
