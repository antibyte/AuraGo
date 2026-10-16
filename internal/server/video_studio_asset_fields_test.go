package server

import (
	"testing"

	"aurago/internal/videostudio"
)

func TestVideoStudioCanonicalizeKeepsProjectOwnedNameAndFolder(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	created, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	const assetID = "123e4567-e89b-12d3-a456-426614174111"
	if err := manager.recordMedia(videoStudioMediaRecord{ProjectID: created.ID, Asset: videostudio.Asset{ID: assetID, Name: "imported.png", Folder: "Upload", Kind: videostudio.AssetImage, Width: 10, Height: 10}, Path: "media/" + assetID + "_imported.png"}); err != nil {
		t.Fatal(err)
	}
	incoming := created.Project
	incoming.Assets = []videostudio.Asset{{ID: assetID, Name: "Hero shot", Folder: "Trips/2024"}}
	canonical, err := manager.canonicalizeProject(created.ID, incoming)
	if err != nil {
		t.Fatal(err)
	}
	if got := canonical.Assets[0]; got.Name != "Hero shot" || got.Folder != "Trips/2024" || got.Width != 10 {
		t.Fatalf("project-owned fields not kept or server fields not taken: %+v", got)
	}
	incoming.Assets[0].Name = "  "
	canonical, err = manager.canonicalizeProject(created.ID, incoming)
	if err != nil || canonical.Assets[0].Name != "imported.png" {
		t.Fatalf("a blank name must fall back to the imported name: %+v, %v", canonical.Assets, err)
	}
	incoming.Assets[0].Name = "  Trimmed  "
	canonical, err = manager.canonicalizeProject(created.ID, incoming)
	if err != nil || canonical.Assets[0].Name != "Trimmed" {
		t.Fatalf("the stored name must be trimmed: %+v, %v", canonical.Assets, err)
	}
	incoming.Assets[0].Folder = "Trips/"
	canonical, err = manager.canonicalizeProject(created.ID, incoming)
	if err != nil {
		t.Fatal(err)
	}
	if err := videostudio.Validate(canonical); err == nil {
		t.Fatal("a non-canonical folder must fail validation after canonicalization")
	}
}
