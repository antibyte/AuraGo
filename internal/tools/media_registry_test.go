package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInitMediaRegistryDB(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test_media.db"))
	if err != nil {
		t.Fatalf("InitMediaRegistryDB failed: %v", err)
	}
	defer db.Close()

	// Verify table exists
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM media_items").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

func TestRegisterAndGetMedia(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	item := MediaItem{
		MediaType:  "image",
		SourceTool: "generate_image",
		Filename:   "test_sunset.png",
		FilePath:   "/data/images/test_sunset.png",
		WebPath:    "/files/generated_images/test_sunset.png",
		Format:     "png",
		Provider:   "openai",
		Model:      "dall-e-3",
		Prompt:     "a sunset over mountains",
		Tags:       []string{"sunset", "landscape"},
	}

	id, _, regErr := RegisterMedia(db, item)
	if regErr != nil {
		t.Fatalf("RegisterMedia failed: %v", regErr)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, getErr := GetMedia(db, id)
	if getErr != nil {
		t.Fatalf("GetMedia failed: %v", getErr)
	}
	if got.Filename != "test_sunset.png" {
		t.Errorf("filename = %q, want %q", got.Filename, "test_sunset.png")
	}
	if got.MediaType != "image" {
		t.Errorf("media_type = %q, want %q", got.MediaType, "image")
	}
	if got.Prompt != "a sunset over mountains" {
		t.Errorf("prompt = %q, want %q", got.Prompt, "a sunset over mountains")
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags len = %d, want 2", len(got.Tags))
	}
}

func TestRegisterMediaDedup(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	item := MediaItem{
		MediaType:  "tts",
		SourceTool: "tts",
		Filename:   "hello.mp3",
		Hash:       "abc123",
		Tags:       []string{"greeting"},
	}

	id1, _, _ := RegisterMedia(db, item)
	id2, _, _ := RegisterMedia(db, item) // duplicate hash

	if id1 != id2 {
		t.Errorf("expected dedup to return same ID: got %d and %d", id1, id2)
	}
}

func TestSearchMedia(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterMedia(db, MediaItem{MediaType: "image", Filename: "sunset.png", Prompt: "a beautiful sunset", Tags: []string{"sunset", "landscape"}})
	RegisterMedia(db, MediaItem{MediaType: "image", Filename: "cat.png", Description: "a cute cat", Tags: []string{"cat", "animal"}})
	RegisterMedia(db, MediaItem{MediaType: "tts", Filename: "hello.mp3", Prompt: "hello world", Tags: []string{"greeting"}})

	results, _, searchErr := SearchMedia(db, "sunset", "", nil, 10, 0)
	if searchErr != nil {
		t.Fatalf("SearchMedia failed: %v", searchErr)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'sunset', got %d", len(results))
	}

	results, _, _ = SearchMedia(db, "hello", "tts", nil, 10, 0)
	if len(results) != 1 {
		t.Errorf("expected 1 TTS result for 'hello', got %d", len(results))
	}

	results, _, _ = SearchMedia(db, "", "image", nil, 10, 0)
	if len(results) != 2 {
		t.Errorf("expected 2 image results, got %d", len(results))
	}

	// Exact tag matching: "sun" should NOT match "sunset"
	results, _, _ = SearchMedia(db, "", "", []string{"sun"}, 10, 0)
	if len(results) != 0 {
		t.Errorf("exact tag search: expected 0 results for 'sun', got %d (should not match 'sunset')", len(results))
	}

	// Exact tag matching: "sunset" SHOULD match
	results, _, _ = SearchMedia(db, "", "", []string{"sunset"}, 10, 0)
	if len(results) != 1 {
		t.Errorf("exact tag search: expected 1 result for 'sunset', got %d", len(results))
	}
}

func TestDispatchMediaRegistryInfersDocumentType(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	workspaceDir := t.TempDir()
	resp := DispatchMediaRegistry(db, workspaceDir, "register", "", "", "News PDF", nil, "", 0, 10, 0, "ki-news-latest.pdf", "ki-news-latest.pdf", "")
	if !strings.Contains(resp, `"status":"success"`) {
		t.Fatalf("register response = %s", resp)
	}

	results, _, err := SearchMedia(db, "", "document", nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchMedia failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 document result, got %d", len(results))
	}
	if results[0].MediaType != "document" {
		t.Fatalf("media_type = %q, want %q", results[0].MediaType, "document")
	}
}

func TestInferMediaType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		filePath string
		want     string
	}{
		{name: "pdf document", filename: "report.pdf", want: "document"},
		{name: "docx document", filePath: "/tmp/report.docx", want: "document"},
		{name: "audio mp3", filename: "voice.mp3", want: "audio"},
		{name: "image png", filename: "image.png", want: "image"},
		{name: "video mp4", filename: "movie.mp4", want: "video"},
		{name: "video mkv", filePath: "/tmp/movie.mkv", want: "video"},
		{name: "unknown defaults to image", filename: "blob.bin", want: "image"},
		{name: "empty defaults to image", want: "image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferMediaType(tt.filename, tt.filePath)
			if got != tt.want {
				t.Fatalf("inferMediaType(%q, %q) = %q, want %q", tt.filename, tt.filePath, got, tt.want)
			}
		})
	}
}

func TestDispatchMediaRegistryRejectsEmptyRegister(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	resp := DispatchMediaRegistry(db, t.TempDir(), "register", "", "", "", nil, "", 0, 10, 0, "", "", "")
	if !strings.Contains(resp, `"status":"error"`) {
		t.Fatalf("empty register should fail, got %s", resp)
	}
	if !strings.Contains(resp, "filename") {
		t.Fatalf("error should mention required media identity, got %s", resp)
	}
}

func TestInitMediaRegistryDBRepairsLegacyDocumentType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := InitMediaRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	if _, _, err := RegisterMedia(db, MediaItem{
		MediaType: "image",
		Filename:  "legacy.pdf",
		FilePath:  "/files/legacy.pdf",
	}); err != nil {
		t.Fatalf("register legacy item: %v", err)
	}
	db.Close()

	db, err = InitMediaRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("re-init db: %v", err)
	}
	defer db.Close()

	results, _, err := SearchMedia(db, "", "document", nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchMedia failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 repaired document result, got %d", len(results))
	}
	if results[0].MediaType != "document" {
		t.Fatalf("media_type = %q, want %q", results[0].MediaType, "document")
	}
}

func TestTagMedia(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, _ := RegisterMedia(db, MediaItem{MediaType: "image", Filename: "test.png", Tags: []string{"original"}})

	// Add tags
	if err := TagMedia(db, id, []string{"new-tag"}, "add"); err != nil {
		t.Fatalf("TagMedia add failed: %v", err)
	}
	item, _ := GetMedia(db, id)
	if len(item.Tags) != 2 {
		t.Errorf("after add: tags len = %d, want 2", len(item.Tags))
	}

	// Remove tags
	if err := TagMedia(db, id, []string{"original"}, "remove"); err != nil {
		t.Fatalf("TagMedia remove failed: %v", err)
	}
	item, _ = GetMedia(db, id)
	if len(item.Tags) != 1 || item.Tags[0] != "new-tag" {
		t.Errorf("after remove: tags = %v, want [new-tag]", item.Tags)
	}

	// Set tags
	if err := TagMedia(db, id, []string{"a", "b", "c"}, "set"); err != nil {
		t.Fatalf("TagMedia set failed: %v", err)
	}
	item, _ = GetMedia(db, id)
	if len(item.Tags) != 3 {
		t.Errorf("after set: tags len = %d, want 3", len(item.Tags))
	}
}

func TestDeleteMedia(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, _ := RegisterMedia(db, MediaItem{MediaType: "image", Filename: "del.png"})

	if err := DeleteMedia(db, id); err != nil {
		t.Fatalf("DeleteMedia failed: %v", err)
	}

	// Should not appear in list
	items, _, _ := ListMedia(db, "", 100, 0)
	for _, item := range items {
		if item.ID == id {
			t.Error("deleted item should not appear in list")
		}
	}
}

func TestDeleteMediaImagesByFilename(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	imageID, _, err := RegisterMedia(db, MediaItem{MediaType: "image", Filename: "shared.png"})
	if err != nil {
		t.Fatalf("register image: %v", err)
	}
	audioID, _, err := RegisterMedia(db, MediaItem{MediaType: "audio", Filename: "shared.png"})
	if err != nil {
		t.Fatalf("register audio: %v", err)
	}
	otherImageID, _, err := RegisterMedia(db, MediaItem{MediaType: "image", Filename: "other.png"})
	if err != nil {
		t.Fatalf("register other image: %v", err)
	}

	deleted, err := DeleteMediaImagesByFilename(db, "shared.png")
	if err != nil {
		t.Fatalf("DeleteMediaImagesByFilename failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted rows = %d, want 1", deleted)
	}
	if _, err := GetMedia(db, imageID); err == nil {
		t.Fatal("expected shared image media item to be deleted")
	}
	if _, err := GetMedia(db, audioID); err != nil {
		t.Fatalf("expected same-filename audio media item to remain: %v", err)
	}
	if _, err := GetMedia(db, otherImageID); err != nil {
		t.Fatalf("expected other image media item to remain: %v", err)
	}
}

func TestMediaStats(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterMedia(db, MediaItem{MediaType: "image", Filename: "a.png"})
	RegisterMedia(db, MediaItem{MediaType: "image", Filename: "b.png"})
	RegisterMedia(db, MediaItem{MediaType: "tts", Filename: "c.mp3"})

	stats, statsErr := MediaStats(db)
	if statsErr != nil {
		t.Fatalf("MediaStats failed: %v", statsErr)
	}
	if stats["total_count"] != int64(3) {
		t.Errorf("total_count = %v (%T), want 3", stats["total_count"], stats["total_count"])
	}
}

func TestDispatchMediaRegistryRejectsPathTraversal(t *testing.T) {
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	workspaceDir := t.TempDir()
	resp := DispatchMediaRegistry(db, workspaceDir, "register", "", "", "Bad", nil, "", 0, 10, 0, "evil.txt", "../../../etc/passwd", "")
	if !strings.Contains(resp, `"status":"error"`) {
		t.Fatalf("expected error for path traversal, got: %s", resp)
	}
}
