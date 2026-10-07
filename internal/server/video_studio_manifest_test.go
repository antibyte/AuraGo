package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/videostudio"
)

func TestVideoStudioRecordMediaRollsBackOnManifestWriteFailure(t *testing.T) {
	manager := newLifecycleVideoStudioManager(t, nil, t.TempDir())
	t.Cleanup(manager.close)

	original := videoStudioMediaRecord{
		ProjectID: lifecycleProjectID,
		Asset:     videostudio.Asset{ID: "123e4567-e89b-12d3-a456-426614174001", Name: "original.png"},
		Path:      "media/123e4567-e89b-12d3-a456-426614174001_original.png",
		SHA256:    "original-digest",
	}
	if err := manager.recordMedia(original); err != nil {
		t.Fatalf("record original media: %v", err)
	}
	persistedPath := manager.path
	manager.path = filepath.Join(t.TempDir(), "missing-directory", "state.json")

	updated := original
	updated.Asset.Name = "updated.png"
	updated.SHA256 = "updated-digest"
	if err := manager.recordMedia(updated); err == nil {
		t.Fatal("recordMedia update succeeded with an unavailable manifest directory")
	}
	got, ok := manager.mediaRecord(original.ProjectID, original.Asset.ID)
	if !ok || got != original {
		t.Fatalf("media after failed update = (%+v, exists=%v), want original %+v", got, ok, original)
	}

	added := videoStudioMediaRecord{
		ProjectID: lifecycleProjectID,
		Asset:     videostudio.Asset{ID: "123e4567-e89b-12d3-a456-426614174002", Name: "new.png"},
		Path:      "media/123e4567-e89b-12d3-a456-426614174002_new.png",
		SHA256:    "new-digest",
	}
	if err := manager.recordMedia(added); err == nil {
		t.Fatal("recordMedia insert succeeded with an unavailable manifest directory")
	}
	if _, ok := manager.mediaRecord(added.ProjectID, added.Asset.ID); ok {
		t.Fatal("new media entry remained in memory after its manifest write failed")
	}
	if err := manager.removeMedia(original.ProjectID, original.Asset.ID); err == nil {
		t.Fatal("removeMedia succeeded with an unavailable manifest directory")
	}
	got, ok = manager.mediaRecord(original.ProjectID, original.Asset.ID)
	if !ok || got != original {
		t.Fatalf("media after failed removal = (%+v, exists=%v), want original %+v", got, ok, original)
	}

	data, err := os.ReadFile(persistedPath)
	if err != nil {
		t.Fatalf("read last durable manifest: %v", err)
	}
	var state videoStudioDiskState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode last durable manifest: %v", err)
	}
	if len(state.Media) != 1 || state.Media[0] != original {
		t.Fatalf("durable media manifest = %+v, want only original entry %+v", state.Media, original)
	}
}
