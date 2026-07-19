package telegram

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestAnalyzeImageRejectsDownloadedLocalFileForAgnes(t *testing.T) {
	cfg := &config.Config{}
	cfg.Vision.ProviderType = "agnes"

	_, err := AnalyzeImage("downloaded-telegram-photo.jpg", cfg)
	if !errors.Is(err, tools.ErrVisionPublicURLRequired) {
		t.Fatalf("error = %v, want ErrVisionPublicURLRequired", err)
	}
}

func TestAnalyzeImageKeepsTrustedTempFileAndLegacyDefaultModel(t *testing.T) {
	var model string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		model = payload.Model
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"channel image ok"}}]}`)
	}))
	defer server.Close()

	tempImage := filepath.Join(t.TempDir(), "telegram.jpg")
	if err := os.WriteFile(tempImage, []byte("temporary channel image"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Vision.APIKey = "test-key"
	cfg.Vision.BaseURL = server.URL

	analysis, err := AnalyzeImage(tempImage, cfg)
	if err != nil {
		t.Fatalf("AnalyzeImage error = %v", err)
	}
	if analysis != "channel image ok" {
		t.Fatalf("analysis = %q", analysis)
	}
	if model != "google/gemini-2.5-flash-lite-preview-09-2025" {
		t.Fatalf("model = %q", model)
	}
}
