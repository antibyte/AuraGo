package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestImageGalleryDeleteRemovesCompanionMediaRecords(t *testing.T) {
	tmpDir := t.TempDir()
	imageDir := filepath.Join(tmpDir, "generated_images")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("create image dir: %v", err)
	}
	imagePath := filepath.Join(imageDir, "shared.png")
	if err := os.WriteFile(imagePath, []byte("fake image"), 0644); err != nil {
		t.Fatalf("write image file: %v", err)
	}

	imageDB, err := tools.InitImageGalleryDB(filepath.Join(tmpDir, "image_gallery.db"))
	if err != nil {
		t.Fatalf("init image gallery db: %v", err)
	}
	defer imageDB.Close()
	mediaDB, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	defer mediaDB.Close()

	galleryID, err := tools.SaveGeneratedImage(imageDB, &tools.ImageGenResult{
		Filename: "shared.png",
		Prompt:   "shared image",
		Provider: "openai",
		Model:    "dall-e-3",
	})
	if err != nil {
		t.Fatalf("save gallery image: %v", err)
	}
	mediaID, _, err := tools.RegisterMedia(mediaDB, tools.MediaItem{
		MediaType: "image",
		Filename:  "shared.png",
		FilePath:  imagePath,
		WebPath:   "/files/generated_images/shared.png",
		Prompt:    "shared image",
	})
	if err != nil {
		t.Fatalf("register media image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir

	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		ImageGalleryDB:  imageDB,
		MediaRegistryDB: mediaDB,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/image-gallery/"+strconv.FormatInt(mediaID, 10)+"?source=media_registry", nil)
	rr := httptest.NewRecorder()
	handleImageGalleryByID(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("response status = %q, body = %v", body["status"], body)
	}
	if _, err := tools.GetMedia(mediaDB, mediaID); err == nil {
		t.Fatal("media registry record should be soft-deleted")
	}
	if _, err := tools.GetGeneratedImage(imageDB, galleryID); err == nil {
		t.Fatal("companion image gallery record should be deleted")
	}
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatalf("image file should be deleted, stat err = %v", err)
	}
}

func TestImageGalleryBulkDeleteRemovesSelectedImagesAndCompanions(t *testing.T) {
	tmpDir := t.TempDir()
	imageDir := filepath.Join(tmpDir, "generated_images")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("create image dir: %v", err)
	}
	for _, name := range []string{"media.png", "gallery.png"} {
		if err := os.WriteFile(filepath.Join(imageDir, name), []byte("fake image"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	imageDB, err := tools.InitImageGalleryDB(filepath.Join(tmpDir, "image_gallery.db"))
	if err != nil {
		t.Fatalf("init image gallery db: %v", err)
	}
	defer imageDB.Close()
	mediaDB, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	defer mediaDB.Close()

	mediaCompanionGalleryID, err := tools.SaveGeneratedImage(imageDB, &tools.ImageGenResult{
		Filename: "media.png",
		Prompt:   "media source image",
		Provider: "openai",
		Model:    "dall-e-3",
	})
	if err != nil {
		t.Fatalf("save media companion gallery image: %v", err)
	}
	galleryID, err := tools.SaveGeneratedImage(imageDB, &tools.ImageGenResult{
		Filename: "gallery.png",
		Prompt:   "gallery source image",
		Provider: "openai",
		Model:    "dall-e-3",
	})
	if err != nil {
		t.Fatalf("save gallery image: %v", err)
	}
	mediaID, _, err := tools.RegisterMedia(mediaDB, tools.MediaItem{
		MediaType: "image",
		Filename:  "media.png",
		FilePath:  filepath.Join(imageDir, "media.png"),
		WebPath:   "/files/generated_images/media.png",
		Prompt:    "media source image",
	})
	if err != nil {
		t.Fatalf("register media source image: %v", err)
	}
	galleryCompanionMediaID, _, err := tools.RegisterMedia(mediaDB, tools.MediaItem{
		MediaType: "image",
		Filename:  "gallery.png",
		FilePath:  filepath.Join(imageDir, "gallery.png"),
		WebPath:   "/files/generated_images/gallery.png",
		Prompt:    "gallery source image",
	})
	if err != nil {
		t.Fatalf("register gallery companion image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		ImageGalleryDB:  imageDB,
		MediaRegistryDB: mediaDB,
	}

	payload := `{"items":[{"id":` + strconv.FormatInt(mediaID, 10) + `,"source":"media_registry"},{"id":` + strconv.FormatInt(galleryID, 10) + `,"source":"image_gallery"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/image-gallery/bulk-delete", bytes.NewReader([]byte(payload)))
	rr := httptest.NewRecorder()
	handleImageGalleryBulkDelete(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Status  string                   `json:"status"`
		Deleted int                      `json:"deleted"`
		Failed  []mediaBulkDeleteFailure `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || body.Deleted != 2 || len(body.Failed) != 0 {
		t.Fatalf("response = %+v", body)
	}
	if _, err := tools.GetMedia(mediaDB, mediaID); err == nil {
		t.Fatal("media source image should be soft-deleted")
	}
	if _, err := tools.GetGeneratedImage(imageDB, mediaCompanionGalleryID); err == nil {
		t.Fatal("media companion gallery image should be deleted")
	}
	if _, err := tools.GetGeneratedImage(imageDB, galleryID); err == nil {
		t.Fatal("gallery source image should be deleted")
	}
	if _, err := tools.GetMedia(mediaDB, galleryCompanionMediaID); err == nil {
		t.Fatal("gallery companion media image should be soft-deleted")
	}
}
