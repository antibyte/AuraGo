package tools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestAnalyzeImageWithPromptTrimsAuthorizationKey(t *testing.T) {
	originalClient := visionHTTPClient
	defer func() { visionHTTPClient = originalClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
