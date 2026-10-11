package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestGalleryGeneratedMetadataIsReadableByAgent(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, _, err := RegisterMedia(db, MediaItem{MediaType: "image", SourceTool: "generate_image", Filename: "cover.png", Prompt: "A blue moon over mountains", Tags: []string{"cover", "noisemaker"}})
	if err != nil {
		t.Fatal(err)
	}
	item, err := GetMedia(db, id)
	if err != nil || item.Description != item.Prompt {
		t.Fatalf("generation metadata: %+v %v", item, err)
	}
	if err := UpdateMedia(db, id, "A yellow sunset", []string{"landscape"}); err != nil {
		t.Fatal(err)
	}
	result := DispatchMediaRegistryWithGallery(db, nil, "", "search", "yellow sunset", "image", "", nil, "", 0, 20, 0, "", "", "")
	if !strings.Contains(result, "A yellow sunset") || !strings.Contains(result, "landscape") {
		t.Fatal("agent cannot read image metadata:", result)
	}
	id, _, err = RegisterMedia(db, MediaItem{MediaType: "audio", Filename: "song.mp3", Prompt: "music"})
	if err != nil {
		t.Fatal(err)
	}
	item, _ = GetMedia(db, id)
	if item.Description != "" {
		t.Fatal("image defaults changed audio")
	}
}

func TestGalleryVisionRejectsUnsupportedInput(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("<svg onload='bad'>"), make([]byte, (20<<20)+1)} {
		if _, _, _, err := AnalyzeGalleryImage(context.Background(), data, "describe", &config.Config{}); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
}
