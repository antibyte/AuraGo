package agent

import (
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type analysisOwnershipVector struct {
	fakeVectorDB
	result   memory.VectorStoreResult
	err      error
	deleted  []string
	contents map[string]string
}

var _ memory.OwnershipAwareVectorDB = (*analysisOwnershipVector)(nil)

func (v *analysisOwnershipVector) StoreDocumentOwned(concept, content string, _ memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	if v.contents == nil {
		v.contents = make(map[string]string)
	}
	for _, id := range v.result.CreatedIDs {
		v.contents[id] = concept + "\n\n" + content
	}
	return v.result, v.err
}

func (v *analysisOwnershipVector) GetByID(id string) (string, error) {
	if text, ok := v.contents[id]; ok {
		return text, nil
	}
	return v.fakeVectorDB.GetByID(id)
}

func (v *analysisOwnershipVector) DeleteDocument(id string) error {
	v.deleted = append(v.deleted, id)
	delete(v.contents, id)
	return nil
}

func (v *analysisOwnershipVector) DeleteDocumentIfContentMatches(id, _ string) (bool, error) {
	return true, v.DeleteDocument(id)
}

func TestMemoryAnalysisPreservesHumanCuration(t *testing.T) {
	for _, status := range []string{"confirmed", "archived"} {
		for _, kind := range []string{"fact", "preference", "correction"} {
			for _, ownership := range []string{"reused", "unknown"} {
				t.Run(status+"/"+kind+"/"+ownership, func(t *testing.T) {
					logger := slog.New(slog.NewTextHandler(io.Discard, nil))
					stm, err := memory.NewSQLiteMemory(":memory:", logger)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = stm.Close() })
					if err := stm.UpsertMemoryMetaWithDetails("existing", memory.MemoryMetaUpdate{VerificationStatus: status, SourceType: "user", ExtractionConfidence: .99, SourceReliability: .99}); err != nil {
						t.Fatal(err)
					}
					before, err := stm.GetMemoryMeta("existing")
					if err != nil {
						t.Fatal(err)
					}
					vdb := &analysisOwnershipVector{}
					if ownership == "reused" {
						vdb.result.ReusedIDs = []string{"existing"}
					} else {
						vdb.result.UnknownIDs = []string{"existing"}
					}
					fact := []extractedFact{{Content: "User prefers German", Category: "preference", Confidence: .96}}
					result := memoryAnalysisResult{}
					switch kind {
					case "fact":
						result.Facts = fact
					case "preference":
						result.Preferences = fact
					case "correction":
						result.Corrections = fact
					}
					applyMemoryAnalysisResult(&config.Config{}, logger, stm, vdb, "default", result)
					after, err := stm.GetMemoryMeta("existing")
					if err != nil || !reflect.DeepEqual(before, after) {
						t.Fatalf("curation changed: before=%+v after=%+v err=%v", before, after, err)
					}
				})
			}
		}
	}
}

func TestMemoryAnalysisRollsBackOnlyCreatedIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, fail := range []string{"vector", "metadata"} {
		t.Run(fail, func(t *testing.T) {
			stm, err := memory.NewSQLiteMemory(":memory:", logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = stm.Close() })
			vdb := &analysisOwnershipVector{result: memory.VectorStoreResult{CreatedIDs: []string{"created"}, ReusedIDs: []string{"reused"}, UnknownIDs: []string{"unknown"}}}
			if fail == "vector" {
				vdb.err = errors.New("partial vector failure")
			} else {
				_ = stm.Close()
			}
			_, err = storeMemoryAnalysisDocument(logger, stm, vdb, "topic", "fact", memory.MemoryMetaUpdate{SourceType: "memory_analysis"})
			wantDeleted := []string{"created"}
			if fail == "metadata" {
				wantDeleted = nil
			}
			if err == nil || !reflect.DeepEqual(vdb.deleted, wantDeleted) {
				t.Fatalf("err=%v deleted=%v", err, vdb.deleted)
			}
		})
	}
}

func TestMemoryAnalysisTracksCreatedAndUnknownIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	vdb := &analysisOwnershipVector{result: memory.VectorStoreResult{CreatedIDs: []string{"created"}, UnknownIDs: []string{"unknown"}}}
	_, err = storeMemoryAnalysisDocument(logger, stm, vdb, "topic", "fact", memory.MemoryMetaUpdate{SourceType: "memory_analysis", ExtractionConfidence: .96, SourceReliability: .85})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"created", "unknown"} {
		meta, err := stm.GetMemoryMeta(id)
		if err != nil || meta.SourceType != "memory_analysis" || meta.ExtractionConfidence != .96 {
			t.Fatalf("meta=%+v err=%v", meta, err)
		}
	}
}
