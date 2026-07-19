package tools

import (
	"aurago/internal/testutil"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestAnalyzeImageWithPromptTrimsAuthorizationKey(t *testing.T) {
	originalClient := visionHTTPClient
	defer func() { visionHTTPClient = originalClient }()

	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`)
	}))
	defer server.Close()

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "image.png")
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Vision.APIKey = "  test-key \n"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = "google/gemini-2.5-flash-lite-preview-09-2025"
	cfg.Directories.WorkspaceDir = dir

	got, promptTokens, completionTokens, err := AnalyzeImageWithPrompt(imgPath, "Describe this image", cfg)
	if err != nil {
		t.Fatalf("AnalyzeImageWithPrompt error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("content = %q, want %q", got, "ok")
	}
	if promptTokens != 12 || completionTokens != 7 {
		t.Fatalf("usage = (%d, %d), want (12, 7)", promptTokens, completionTokens)
	}
}

func TestAnalyzeImageWithPromptRejectsLocalFilesForAgnes(t *testing.T) {
	cfg := &config.Config{}
	cfg.Vision.ProviderType = "agnes"
	cfg.Directories.WorkspaceDir = t.TempDir()

	_, _, _, err := AnalyzeImageWithPrompt("image.png", "Describe it", cfg)
	if !errors.Is(err, ErrVisionPublicURLRequired) {
		t.Fatalf("error = %v, want ErrVisionPublicURLRequired", err)
	}
}

func TestAnalyzeImageURLWithPromptSendsPublicURLUnchanged(t *testing.T) {
	originalClient := visionHTTPClient
	originalValidator := validatePublicImageURL
	defer func() {
		visionHTTPClient = originalClient
		validatePublicImageURL = originalValidator
	}()
	validatePublicImageURL = func(rawURL string) error {
		if rawURL != "https://cdn.example.test/image.png?signature=keep-me" {
			t.Fatalf("validated URL = %q", rawURL)
		}
		return nil
	}

	var imageURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content []struct {
					ImageURL *struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		for _, part := range payload.Messages[0].Content {
			if part.ImageURL != nil {
				imageURL = part.ImageURL.URL
			}
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"public-url-ok"}}]}`)
	}))
	defer server.Close()
	visionHTTPClient = server.Client()

	cfg := &config.Config{}
	cfg.Vision.ProviderType = "agnes"
	cfg.Vision.APIKey = "test-key"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = "agnes-2.0-flash"

	got, _, _, err := AnalyzeImageURLWithPrompt("https://cdn.example.test/image.png?signature=keep-me", "Describe it", cfg)
	if err != nil {
		t.Fatalf("AnalyzeImageURLWithPrompt error = %v", err)
	}
	if got != "public-url-ok" {
		t.Fatalf("content = %q", got)
	}
	if imageURL != "https://cdn.example.test/image.png?signature=keep-me" {
		t.Fatalf("payload image URL = %q", imageURL)
	}
}
