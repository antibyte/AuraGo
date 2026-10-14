package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
	"github.com/sashabaranov/go-openai"
)

func writeTelegramTestJPEG(t *testing.T) string {
	t.Helper()
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "telegram.jpg")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

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
	var imageURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model    string                         `json:"model"`
			Messages []openai.ChatCompletionMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		model = payload.Model
		for _, message := range payload.Messages {
			for _, part := range message.MultiContent {
				if part.ImageURL != nil {
					imageURL = part.ImageURL.URL
				}
			}
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"channel image ok"}}]}`)
	}))
	defer server.Close()

	tempImage := writeTelegramTestJPEG(t)
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
	if !strings.HasPrefix(imageURL, "data:image/jpeg;base64,") {
		t.Fatal("request did not include a JPEG data URI")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(imageURL, "data:image/jpeg;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("request contains invalid JPEG: %v", err)
	}
}

func TestAnalyzeImageContextCancelsTrustedTempFileRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tempImage := writeTelegramTestJPEG(t)
	cfg := &config.Config{}
	cfg.Vision.APIKey = "test-key"
	cfg.Vision.BaseURL = "http://127.0.0.1:1"

	_, err := AnalyzeImageContext(ctx, tempImage, cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AnalyzeImageContext error = %v, want context.Canceled", err)
	}
}

func TestAnalyzeImageRejectsInvalidJPEGBeforeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid image reached the provider")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "invalid.jpg")
	if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Vision.APIKey = "test-key"
	cfg.Vision.BaseURL = server.URL
	if _, err := AnalyzeImage(path, cfg); err == nil {
		t.Fatal("invalid image was accepted")
	}
}
