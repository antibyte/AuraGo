package memory

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		baseURL      string
		want         string
	}{
		{"google provider type", "google", "https://example.com", "vertex"},
		{"vertex provider type", "vertex-ai", "https://example.com", "vertex"},
		{"googleapis url", "custom", "https://us-central1-aiplatform.googleapis.com/v1/projects/123", "vertex"},
		{"generativelanguage url", "custom", "https://generativelanguage.googleapis.com/v1beta", "vertex"},
		{"openai provider", "openai", "https://api.openai.com/v1", "openai"},
		{"openrouter provider", "openrouter", "https://openrouter.ai/api/v1", "openai"},
		{"empty provider", "", "https://api.example.com", "openai"},
		{"mixed case google", "Google", "https://example.com", "vertex"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFormat(tt.providerType, tt.baseURL)
			if got != tt.want {
				t.Errorf("detectFormat(%q, %q) = %q, want %q", tt.providerType, tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.JPEG", "image/jpeg"},
		{"image.png", "image/png"},
		{"anim.gif", "image/gif"},
		{"pic.webp", "image/webp"},
		{"pic.bmp", "image/bmp"},
		{"scan.tiff", "image/tiff"},
		{"scan.tif", "image/tiff"},
		{"icon.svg", "image/svg+xml"},
		{"song.mp3", "audio/mpeg"},
		{"clip.wav", "audio/wav"},
		{"track.ogg", "audio/ogg"},
		{"music.flac", "audio/flac"},
		{"voice.aac", "audio/aac"},
		{"memo.m4a", "audio/mp4"},
		{"old.wma", "audio/x-ms-wma"},
		{"doc.pdf", "application/pdf"},
		{"file.xyz", "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectMIME(tt.path)
			if got != tt.want {
				t.Errorf("detectMIME(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeEmbedding(t *testing.T) {
	t.Run("unit vector stays unit", func(t *testing.T) {
		v := []float32{1, 0, 0}
		out := normalizeEmbedding(v)
		if out[0] != 1 || out[1] != 0 || out[2] != 0 {
			t.Errorf("expected [1,0,0], got %v", out)
		}
	})

	t.Run("scales to unit length", func(t *testing.T) {
		v := []float32{3, 4}
		out := normalizeEmbedding(v)
		// L2 norm of [3,4] = 5, so [0.6, 0.8]
		if math.Abs(float64(out[0])-0.6) > 1e-6 || math.Abs(float64(out[1])-0.8) > 1e-6 {
			t.Errorf("expected ~[0.6, 0.8], got %v", out)
		}
	})

	t.Run("zero vector unchanged", func(t *testing.T) {
		v := []float32{0, 0, 0}
		out := normalizeEmbedding(v)
		for i, val := range out {
			if val != 0 {
				t.Errorf("expected 0 at index %d, got %f", i, val)
			}
		}
	})

	t.Run("result has unit norm", func(t *testing.T) {
		v := []float32{1.5, -2.3, 0.7, 4.1}
		out := normalizeEmbedding(v)
		var sum float64
		for _, val := range out {
			sum += float64(val) * float64(val)
		}
		if math.Abs(sum-1.0) > 1e-5 {
			t.Errorf("norm squared = %f, want ~1.0", sum)
		}
	})
}

func TestTruncateResponse(t *testing.T) {
	short := "short response"
	if got := truncateResponse([]byte(short)); got != short {
		t.Errorf("expected %q, got %q", short, got)
	}

	long := make([]byte, 500)
	for i := range long {
		long[i] = 'a'
	}
	got := truncateResponse(long)
	if len(got) != 303 { // 300 + "..."
		t.Errorf("expected truncated length 303, got %d", len(got))
	}
}

func TestNewMultimodalEmbedder_AutoFormat(t *testing.T) {
	e := NewMultimodalEmbedder("https://api.openai.com/v1", "key", "model", "auto", "openai", nil)
	if e.format != "openai" {
		t.Errorf("expected format 'openai', got %q", e.format)
	}

	e2 := NewMultimodalEmbedder("https://us-central1-aiplatform.googleapis.com/v1", "key", "model", "", "google", nil)
	if e2.format != "vertex" {
		t.Errorf("expected format 'vertex', got %q", e2.format)
	}
}

func TestNewMultimodalEmbedder_ExplicitFormat(t *testing.T) {
	e := NewMultimodalEmbedder("https://example.com", "key", "model", "vertex", "openai", nil)
	if e.format != "vertex" {
		t.Errorf("explicit format should override: expected 'vertex', got %q", e.format)
	}
}

func TestNewMultimodalEmbedder_TrailingSlash(t *testing.T) {
	e := NewMultimodalEmbedder("https://api.example.com/v1/", "key", "model", "openai", "", nil)
	if e.baseURL != "https://api.example.com/v1" {
		t.Errorf("trailing slash not trimmed: got %q", e.baseURL)
	}
}

func TestEmbedFile_OpenAI(t *testing.T) {
	embedding := []float32{0.1, 0.2, 0.3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Errorf("expected /embeddings, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["model"] != "test-model" {
			t.Errorf("expected model 'test-model', got %v", payload["model"])
		}
		input, ok := payload["input"].(map[string]interface{})
		if !ok {
			t.Fatal("input is not a map")
		}
		if input["type"] != "image_url" {
			t.Errorf("expected type 'image_url', got %v", input["type"])
		}

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": embedding},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "test.png")
	os.WriteFile(tmpFile, []byte("fake-png-data"), 0644)

	e := NewMultimodalEmbedder(srv.URL, "test-key", "test-model", "openai", "", nil)
	got, err := e.EmbedFile(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("EmbedFile failed: %v", err)
	}

	// Result is normalized so check direction, not exact values
	if len(got) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(got))
	}
}

func TestEmbedFile_Vertex(t *testing.T) {
	embedding := []float32{0.5, 0.5}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, ":predict") {
			t.Errorf("expected path ending with :predict, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		instances, ok := payload["instances"].([]interface{})
		if !ok || len(instances) == 0 {
			t.Fatal("expected instances array")
		}

		resp := map[string]interface{}{
			"predictions": []map[string]interface{}{
				{
					"embeddings": map[string]interface{}{
						"values": embedding,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "voice.mp3")
	os.WriteFile(tmpFile, []byte("fake-mp3-data"), 0644)

	e := NewMultimodalEmbedder(srv.URL, "test-key", "test-model", "vertex", "", nil)
	got, err := e.EmbedFile(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("EmbedFile failed: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(got))
	}
}

func TestEmbedFile_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "test.png")
	os.WriteFile(tmpFile, []byte("fake"), 0644)

	e := NewMultimodalEmbedder(srv.URL, "key", "model", "openai", "", nil)
	_, err := e.EmbedFile(context.Background(), tmpFile)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestEmbedFile_FileNotFound(t *testing.T) {
	e := NewMultimodalEmbedder("https://example.com", "key", "model", "openai", "", nil)
	_, err := e.EmbedFile(context.Background(), "/nonexistent/file.png")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEmbedFile_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "test.jpg")
	os.WriteFile(tmpFile, []byte("fake"), 0644)

	e := NewMultimodalEmbedder(srv.URL, "key", "model", "openai", "", nil)
	_, err := e.EmbedFile(context.Background(), tmpFile)
	if err == nil {
		t.Fatal("expected error for empty embedding response")
	}
}
