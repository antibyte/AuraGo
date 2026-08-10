package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/tools"
)

func BenchmarkNativeToolSchemaSnapshotCached(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	skillsDir := b.TempDir()
	manifest := tools.NewManifest(b.TempDir())
	flags := ToolFeatureFlags{}
	dynamicToolSchemaCache.Clear()
	_ = BuildNativeToolSchemaSnapshot(skillsDir, manifest, flags, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildNativeToolSchemaSnapshot(skillsDir, manifest, flags, logger)
	}
}
