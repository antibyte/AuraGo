package server

import (
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
