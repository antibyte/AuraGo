package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestResolveMemoryAnalysisSettingsBootstrapMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	got := resolveMemoryAnalysisSettings(&config.Config{}, stm)
	if !got.Enabled || !got.UnifiedMemoryBlock || !got.EffectivenessTracking {
		t.Fatalf("expected always-on adaptive features, got %+v", got)
	}
	if got.Mode != "bootstrap" {
		t.Fatalf("Mode = %q, want bootstrap", got.Mode)
	}
	if got.QueryExpansion || got.LLMReranking {
		t.Fatalf("bootstrap mode should stay conservative, got %+v", got)
	}
}

func TestResolveMemoryAnalysisSettingsStabilizeMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for i := 0; i < 16; i++ {
		docID := "doc-stabilize-" + string(rune('a'+i))
		if err := stm.UpsertMemoryMeta(docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", docID, err)
		}
		if err := stm.RecordMemoryUsage(docID, "ltm_retrieved", "session-stabilize", 0.4, false); err != nil {
			t.Fatalf("RecordMemoryUsage(%s): %v", docID, err)
		}
	}
	if err := stm.RecordMemoryEffectiveness("doc-stabilize-a", false); err != nil {
		t.Fatalf("RecordMemoryEffectiveness: %v", err)
	}
	if err := stm.RecordMemoryEffectiveness("doc-stabilize-a", false); err != nil {
		t.Fatalf("RecordMemoryEffectiveness: %v", err)
	}
	if err := stm.RecordMemoryEffectiveness("doc-stabilize-b", false); err != nil {
		t.Fatalf("RecordMemoryEffectiveness: %v", err)
	}
	if err := stm.RecordMemoryEffectiveness("doc-stabilize-b", false); err != nil {
		t.Fatalf("RecordMemoryEffectiveness: %v", err)
	}

	got := resolveMemoryAnalysisSettings(&config.Config{}, stm)
	if got.Mode != "stabilize" {
		t.Fatalf("Mode = %q, want stabilize", got.Mode)
	}
	if !got.QueryExpansion || !got.LLMReranking || !got.RealTime {
		t.Fatalf("stabilize mode should enable aggressive cleanup features, got %+v", got)
	}
}

func TestResolveMemoryAnalysisSettingsEfficientMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for i := 0; i < 16; i++ {
		docID := "doc-efficient-" + string(rune('a'+i))
		if err := stm.UpsertMemoryMetaWithDetails(docID, memory.MemoryMetaUpdate{
			ExtractionConfidence: 0.95,
			VerificationStatus:   "confirmed",
			SourceType:           "memory_analysis",
			SourceReliability:    0.95,
		}); err != nil {
			t.Fatalf("UpsertMemoryMetaWithDetails(%s): %v", docID, err)
		}
		if err := stm.RecordMemoryUsage(docID, "ltm_retrieved", "session-a", 0.9, false); err != nil {
			t.Fatalf("RecordMemoryUsage(%s): %v", docID, err)
		}
		if err := stm.RecordMemoryEffectiveness(docID, true); err != nil {
			t.Fatalf("RecordMemoryEffectiveness(%s): %v", docID, err)
		}
		if err := stm.RecordMemoryEffectiveness(docID, true); err != nil {
			t.Fatalf("RecordMemoryEffectiveness(%s): %v", docID, err)
		}
	}

	got := resolveMemoryAnalysisSettings(&config.Config{}, stm)
	if got.Mode != "efficient" {
		t.Fatalf("Mode = %q, want efficient", got.Mode)
	}
	if !got.QueryExpansion || got.LLMReranking {
		t.Fatalf("efficient mode should keep expansion but skip reranking, got %+v", got)
	}
}
