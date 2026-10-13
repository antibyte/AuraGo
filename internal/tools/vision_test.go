package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func TestAnalyzeImageWithPromptTrimsAuthorizationKey(t *testing.T) {
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
	sample := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	sample.Set(0, 0, color.NRGBA{R: 12, G: 34, B: 56, A: 255})
	var raw bytes.Buffer
	if err := png.Encode(&raw, sample); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imgPath, raw.Bytes(), 0o644); err != nil {
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
	originalValidator := validatePublicImageURL
	defer func() {
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"public-url-ok"}}]}`)
	}))
	defer server.Close()

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

const analyzeImageTestReference = "data:image/png;base64,QUJD"

func TestAnalyzeImageUsesTheProviderClient(t *testing.T) {
	const imageReference = analyzeImageTestReference
	var requestURL, authorization, imageURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = "http://" + r.Host + r.URL.Path
		authorization = r.Header.Get("Authorization")
		var payload struct {
			Messages []struct {
				Content []struct {
					Type     string `json:"type"`
					ImageURL *struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		for _, message := range payload.Messages {
			for _, part := range message.Content {
				if part.Type == "image_url" && part.ImageURL != nil {
					imageURL = part.ImageURL.URL
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"described"}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`)
	}))

	cfg := &config.Config{}
	cfg.Vision.ProviderType = "openai"
	cfg.Vision.APIKey = "  sk-vision \n"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = "gpt-4o"

	got, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), imageReference, "Describe it", cfg)
	if err != nil {
		t.Fatalf("analyzeImageReferenceWithPromptContext: %v", err)
	}
	if got != "described" {
		t.Fatalf("content = %q, want described", got)
	}
	if requestURL != server.URL+"/chat/completions" {
		t.Fatalf("request URL = %q, want %q", requestURL, server.URL+"/chat/completions")
	}
	if authorization != "Bearer sk-vision" {
		t.Fatalf("Authorization = %q, want Bearer sk-vision", authorization)
	}
	if imageURL != imageReference {
		t.Fatalf("image_url = %q, want %q", imageURL, imageReference)
	}
}

func TestAnalyzeImageHidesProviderErrorBodies(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "secret-provider-body", http.StatusBadRequest)
	}))

	cfg := &config.Config{}
	cfg.Vision.ProviderType = "openai"
	cfg.Vision.APIKey = "sk-vision"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = "gpt-4o"

	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), analyzeImageTestReference, "Describe it", cfg)
	if err == nil {
		t.Fatal("expected vision API error")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("error = %q, want status 400", err.Error())
	}
	if strings.Contains(err.Error(), "secret-provider-body") {
		t.Fatalf("error leaked provider body: %s", err.Error())
	}
	if strings.Contains(err.Error(), server.URL) {
		t.Fatalf("error leaked base URL: %s", err.Error())
	}
}

func TestAnalyzeImageEmptyModelUsesTheMainModelOnTheSameOrigin(t *testing.T) {
	var model string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		model = payload.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))

	cfg := &config.Config{}
	cfg.Vision.ProviderType = "openai"
	cfg.Vision.APIKey = "sk-vision"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = "  "
	cfg.LLM.BaseURL = server.URL
	cfg.LLM.Model = "llava"

	if _, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), analyzeImageTestReference, "Describe it", cfg); err != nil {
		t.Fatalf("analyzeImageReferenceWithPromptContext: %v", err)
	}
	if model != "llava" {
		t.Fatalf("model = %q, want llava", model)
	}
	if model == DefaultVisionModel {
		t.Fatalf("model fell back to %s", DefaultVisionModel)
	}
}

func TestAnalyzeImageRefusesAnEmptyModelOnAnotherOrigin(t *testing.T) {
	hits := 0
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		t.Errorf("unexpected HTTP request to %s", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))

	cfg := &config.Config{}
	cfg.Vision.ProviderType = "ollama"
	cfg.Vision.APIKey = "local-key"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = ""
	cfg.LLM.BaseURL = "https://api.example.test/v1"
	cfg.LLM.Model = "llava"

	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), analyzeImageTestReference, "Describe this image", cfg)
	if err == nil || !strings.Contains(err.Error(), "vision model is not configured") {
		t.Fatalf("error = %v, want vision model is not configured", err)
	}
	if hits != 0 {
		t.Fatalf("HTTP requests = %d, want 0", hits)
	}
}

func TestAnalyzeImageEmptyTypeUsesDefaultModelWithoutInheritingAnthropic(t *testing.T) {
	var method, path, model string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		model = payload.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"seen"}}]}`)
	}))

	cfg := &config.Config{}
	cfg.Vision.ProviderType = ""
	cfg.Vision.APIKey = "sk-vision"
	cfg.Vision.BaseURL = server.URL
	cfg.Vision.Model = ""
	cfg.LLM.ProviderType = "anthropic"
	cfg.LLM.BaseURL = "https://api.anthropic.com/v1"
	cfg.LLM.Model = "llava"

	got, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), analyzeImageTestReference, "Describe it", cfg)
	if err != nil {
		t.Fatalf("analyzeImageReferenceWithPromptContext: %v", err)
	}
	if got != "seen" {
		t.Fatalf("content = %q, want seen", got)
	}
	if method != http.MethodPost || path != "/chat/completions" {
		t.Fatalf("request = %s %s, want POST /chat/completions", method, path)
	}
	if model != DefaultVisionModel {
		t.Fatalf("model = %q, want %s", model, DefaultVisionModel)
	}
	if model == "llava" {
		t.Fatal("model inherited the main model llava")
	}
}

func TestPrepareVisionPromptLeavesGeneralCountsAlone(t *testing.T) {
	for _, prompt := range []string{
		"how many people are in the room",
		"wie viele Äpfel",
	} {
		got, structured := PrepareVisionPrompt(prompt)
		if structured || got != prompt {
			t.Fatalf("general count prompt changed: got %q structured=%v", got, structured)
		}
	}
}

func TestPrepareVisionPromptStillStructuresVehicleCounts(t *testing.T) {
	for _, prompt := range []string{
		"Wie viele PKW sind sichtbar?",
		"how many cars",
	} {
		got, structured := PrepareVisionPrompt(prompt)
		if !structured || !strings.Contains(got, visionCountSchemaMarker) {
			t.Fatalf("vehicle count prompt not structured: %q structured=%v", got, structured)
		}
	}
}
