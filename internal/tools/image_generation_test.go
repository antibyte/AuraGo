package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitImageGalleryDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_gallery.db")

	db, err := InitImageGalleryDB(dbPath)
	if err != nil {
		t.Fatalf("InitImageGalleryDB failed: %v", err)
	}
	defer db.Close()

	// Verify table exists by inserting a dummy row
	_, err = db.Exec(`INSERT INTO generated_images (prompt, provider, model, filename) VALUES (?, ?, ?, ?)`,
		"test prompt", "openai", "dall-e-3", "test.png")
	if err != nil {
		t.Fatalf("insert into generated_images failed: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM generated_images").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestSaveAndGetGeneratedImage(t *testing.T) {
	db, err := InitImageGalleryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	result := &ImageGenResult{
		Filename:       "img_test.png",
		WebPath:        "/files/generated_images/img_test.png",
		Prompt:         "a sunset over mountains",
		EnhancedPrompt: "a vivid sunset over snow-capped mountains",
		Model:          "dall-e-3",
		Provider:       "openai",
		Size:           "1024x1024",
		Quality:        "standard",
		Style:          "natural",
		DurationMs:     1500,
		CostEstimate:   0.04,
	}

	id, err := SaveGeneratedImage(db, result)
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	record, err := GetGeneratedImage(db, id)
	if err != nil {
		t.Fatalf("GetGeneratedImage failed: %v", err)
	}
	if record.Prompt != "a sunset over mountains" {
		t.Errorf("prompt mismatch: %q", record.Prompt)
	}
	if record.EnhancedPrompt != "a vivid sunset over snow-capped mountains" {
		t.Errorf("enhanced prompt mismatch: %q", record.EnhancedPrompt)
	}
	if record.Provider != "openai" {
		t.Errorf("provider mismatch: %q", record.Provider)
	}
	if record.Model != "dall-e-3" {
		t.Errorf("model mismatch: %q", record.Model)
	}
	if record.Size != "1024x1024" {
		t.Errorf("size mismatch: %q", record.Size)
	}
}

func TestSaveGeneratedImage_NilDB(t *testing.T) {
	id, err := SaveGeneratedImage(nil, &ImageGenResult{})
	if err != nil {
		t.Errorf("expected no error for nil db, got: %v", err)
	}
	if id != 0 {
		t.Errorf("expected 0 id for nil db, got %d", id)
	}
}

func TestListGeneratedImages(t *testing.T) {
	db, err := InitImageGalleryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	// Insert test records
	providers := []string{"openai", "openai", "stability", "ideogram"}
	prompts := []string{"cat on roof", "dog in park", "mountain landscape", "abstract art"}
	for i := range providers {
		_, err := SaveGeneratedImage(db, &ImageGenResult{
			Filename: "img_" + prompts[i] + ".png",
			Prompt:   prompts[i],
			Provider: providers[i],
			Model:    "test-model",
		})
		if err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	// List all
	records, total, err := ListGeneratedImages(db, "", "", 50, 0)
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if total != 4 {
		t.Errorf("expected total 4, got %d", total)
	}
	if len(records) != 4 {
		t.Errorf("expected 4 records, got %d", len(records))
	}

	// Filter by provider
	records, total, err = ListGeneratedImages(db, "openai", "", 50, 0)
	if err != nil {
		t.Fatalf("list by provider failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2 for openai, got %d", total)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records for openai provider, got %d", len(records))
	}

	// Search by prompt
	records, total, err = ListGeneratedImages(db, "", "mountain", 50, 0)
	if err != nil {
		t.Fatalf("list by search failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1 for mountain search, got %d", total)
	}

	// Pagination
	records, total, err = ListGeneratedImages(db, "", "", 2, 0)
	if err != nil {
		t.Fatalf("list paginated failed: %v", err)
	}
	if total != 4 {
		t.Errorf("total should still be 4, got %d", total)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records with limit 2, got %d", len(records))
	}

	// Nil DB
	records, total, err = ListGeneratedImages(nil, "", "", 50, 0)
	if err != nil {
		t.Errorf("expected nil for nil db, got: %v", err)
	}
	if total != 0 || records != nil {
		t.Errorf("expected empty for nil db")
	}
}

func TestDeleteGeneratedImage(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitImageGalleryDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	// Create a fake image file
	imgDir := filepath.Join(tmpDir, "generated_images")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "del_test.png"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	id, err := SaveGeneratedImage(db, &ImageGenResult{
		Filename: "del_test.png",
		Prompt:   "delete me",
		Provider: "openai",
		Model:    "dall-e-3",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := DeleteGeneratedImage(db, id, tmpDir); err != nil {
		t.Fatalf("DeleteGeneratedImage failed: %v", err)
	}

	// Verify DB record is gone
	_, err = GetGeneratedImage(db, id)
	if err == nil {
		t.Error("expected error after deletion, got nil")
	}

	// Verify file is gone
	if _, err := os.Stat(filepath.Join(imgDir, "del_test.png")); !os.IsNotExist(err) {
		t.Error("expected image file to be deleted")
	}
}

func TestDeleteGeneratedImagesByFilename(t *testing.T) {
	db, err := InitImageGalleryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	firstID, err := SaveGeneratedImage(db, &ImageGenResult{
		Filename: "shared.png",
		Prompt:   "first duplicate",
		Provider: "openai",
		Model:    "dall-e-3",
	})
	if err != nil {
		t.Fatalf("save first image: %v", err)
	}
	secondID, err := SaveGeneratedImage(db, &ImageGenResult{
		Filename: "other.png",
		Prompt:   "keep me",
		Provider: "openai",
		Model:    "dall-e-3",
	})
	if err != nil {
		t.Fatalf("save second image: %v", err)
	}

	deleted, err := DeleteGeneratedImagesByFilename(db, "shared.png")
	if err != nil {
		t.Fatalf("DeleteGeneratedImagesByFilename failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted rows = %d, want 1", deleted)
	}
	if _, err := GetGeneratedImage(db, firstID); err == nil {
		t.Fatal("expected shared.png record to be deleted")
	}
	if _, err := GetGeneratedImage(db, secondID); err != nil {
		t.Fatalf("expected other.png record to remain: %v", err)
	}
}

func TestImageGalleryMonthlyCount(t *testing.T) {
	db, err := InitImageGalleryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	count, err := ImageGalleryMonthlyCount(db)
	if err != nil {
		t.Fatalf("monthly count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for empty db, got %d", count)
	}

	// Insert a record (created_at defaults to now)
	SaveGeneratedImage(db, &ImageGenResult{
		Filename: "test.png", Prompt: "test", Provider: "openai", Model: "dall-e-3",
	})

	count, err = ImageGalleryMonthlyCount(db)
	if err != nil {
		t.Fatalf("monthly count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 after insert, got %d", count)
	}

	// Nil DB
	count, err = ImageGalleryMonthlyCount(nil)
	if err != nil {
		t.Errorf("expected no error for nil db, got: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for nil db, got %d", count)
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		expect string
	}{
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, "png"},
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "jpeg"},
		{"WebP", append([]byte("RIFF"), append([]byte{0, 0, 0, 0}, []byte("WEBP")...)...), "webp"},
		{"GIF", []byte("GIF89a"), "gif"},
		{"unknown defaults to png", []byte{0x00, 0x00, 0x00, 0x00}, "png"},
		{"short data defaults to png", []byte{0x01}, "png"},
		{"empty defaults to png", []byte{}, "png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectFormat(tc.data)
			if got != tc.expect {
				t.Errorf("detectFormat(%v) = %q, want %q", tc.data, got, tc.expect)
			}
		})
	}
}

func TestTruncateError(t *testing.T) {
	short := "short error"
	if got := truncateError(short); got != short {
		t.Errorf("expected unchanged short string, got %q", got)
	}

	long := ""
	for i := 0; i < 600; i++ {
		long += "x"
	}
	got := truncateError(long)
	if len(got) != 503 { // 500 + "..."
		t.Errorf("expected truncated to 503 chars, got %d", len(got))
	}
	if got[500:] != "..." {
		t.Errorf("expected ... suffix, got %q", got[500:])
	}
}

func TestSizeToAspectRatio(t *testing.T) {
	tests := []struct {
		size   string
		expect string
	}{
		{"1024x1024", "1:1"},
		{"1152x896", "4:3"},
		{"1344x768", "16:9"},
		{"768x1344", "9:16"},
		{"896x1152", "3:4"},
		{"invalid", ""},
		{"999x999", "1:1"}, // default fallback
	}
	for _, tc := range tests {
		t.Run(tc.size, func(t *testing.T) {
			got := sizeToAspectRatio(tc.size)
			if got != tc.expect {
				t.Errorf("sizeToAspectRatio(%q) = %q, want %q", tc.size, got, tc.expect)
			}
		})
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		expect   float64
	}{
		{"openai", "dall-e-3", 0.04},
		{"openai", "dall-e-2", 0.02},
		{"openai", "unknown", 0.04},
		{"openrouter", "flux-pro", 0.03},
		{"stability", "sd3-core", 0.03},
		{"ideogram", "v2", 0.05},
		{"google", "imagen-3", 0.04},
		{"google-imagen", "imagen-3", 0.04},
		{"unknown", "test", 0.04},
	}
	for _, tc := range tests {
		t.Run(tc.provider+"_"+tc.model, func(t *testing.T) {
			got := estimateCost(tc.provider, tc.model)
			if got != tc.expect {
				t.Errorf("estimateCost(%q, %q) = %f, want %f", tc.provider, tc.model, got, tc.expect)
			}
		})
	}
}

func TestResolveSourceImagePath_Absolute(t *testing.T) {
	// Absolute paths are rejected for security (path traversal prevention)
	abs := filepath.Join(t.TempDir(), "test.png")
	got := ResolveSourceImagePath(abs, "/ws", "/data")
	if got != "" {
		t.Errorf("expected empty string for absolute path, got %q", got)
	}
}

func TestResolveSourceImagePath_Traversal(t *testing.T) {
	// Directory traversal should be rejected
	got := ResolveSourceImagePath("../../etc/passwd", t.TempDir(), t.TempDir())
	if got != "" {
		t.Errorf("expected empty string for traversal path, got %q", got)
	}
}

func TestResolveSourceImagePath_WorkspaceFirst(t *testing.T) {
	wsDir := t.TempDir()
	dataDir := t.TempDir()

	// Create file in workspace
	if err := os.WriteFile(filepath.Join(wsDir, "photo.png"), []byte("ws"), 0644); err != nil {
		t.Fatal(err)
	}

	got := ResolveSourceImagePath("photo.png", wsDir, dataDir)
	expect := filepath.Join(wsDir, "photo.png")
	if got != expect {
		t.Errorf("expected workspace path %q, got %q", expect, got)
	}
}

func TestResolveSourceImagePath_DataDirFallback(t *testing.T) {
	wsDir := t.TempDir()
	dataDir := t.TempDir()

	// Create file in data dir only
	if err := os.WriteFile(filepath.Join(dataDir, "photo.png"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	got := ResolveSourceImagePath("photo.png", wsDir, dataDir)
	expect := filepath.Join(dataDir, "photo.png")
	if got != expect {
		t.Errorf("expected data dir path %q, got %q", expect, got)
	}
}

func TestResolveSourceImagePath_GeneratedImagesFallback(t *testing.T) {
	wsDir := t.TempDir()
	dataDir := t.TempDir()
	genDir := filepath.Join(dataDir, "generated_images")
	os.MkdirAll(genDir, 0755)

	if err := os.WriteFile(filepath.Join(genDir, "img_test.png"), []byte("gen"), 0644); err != nil {
		t.Fatal(err)
	}

	got := ResolveSourceImagePath("img_test.png", wsDir, dataDir)
	expect := filepath.Join(genDir, "img_test.png")
	if got != expect {
		t.Errorf("expected generated_images path %q, got %q", expect, got)
	}
}

func TestResolveSourceImagePath_NotFound(t *testing.T) {
	got := ResolveSourceImagePath("nonexistent.png", t.TempDir(), t.TempDir())
	if got != "nonexistent.png" {
		t.Errorf("expected original path returned when not found, got %q", got)
	}
}

func TestGenerateImage_UnsupportedProvider(t *testing.T) {
	cfg := ImageGenConfig{
		ProviderType: "nonexistent",
		Model:        "test",
		DataDir:      t.TempDir(),
	}
	_, err := GenerateImage(cfg, "test prompt", ImageGenOptions{})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if got := err.Error(); got != `unsupported image generation provider type: "nonexistent"` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSaveImageData(t *testing.T) {
	dataDir := t.TempDir()
	imgData := []byte("fake image data")

	fullPath, err := saveImageData(imgData, "png", dataDir)
	if err != nil {
		t.Fatalf("saveImageData failed: %v", err)
	}
	if fullPath == "" {
		t.Fatal("expected non-empty path")
	}
	if filepath.Ext(fullPath) != ".png" {
		t.Errorf("expected .png extension, got %q", filepath.Ext(fullPath))
	}

	// Verify file exists on disk (saveImageData now returns the full absolute path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != "fake image data" {
		t.Errorf("file content mismatch")
	}
}

func TestSaveImageData_CustomFormat(t *testing.T) {
	dataDir := t.TempDir()
	fullPath, err := saveImageData([]byte("test"), "jpeg", dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(fullPath) != ".jpeg" {
		t.Errorf("expected .jpeg extension, got %q", filepath.Ext(fullPath))
	}
}

func TestBuildMultipartForm(t *testing.T) {
	fields := map[string]string{"prompt": "hello", "model": "sd3"}
	files := map[string][]byte{"image": {0x89, 0x50}}

	buf, contentType, err := buildMultipartForm(fields, files)
	if err != nil {
		t.Fatalf("buildMultipartForm failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty buffer")
	}
	if contentType == "" {
		t.Error("expected non-empty content type")
	}
}
