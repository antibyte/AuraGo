package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/testutil"
)

func TestAgnesImageModelAndEndpointNormalization(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"":                        defaultAgnesImageModel,
		"agnes-2.0-flash":         defaultAgnesImageModel,
		"not-an-image-model":      defaultAgnesImageModel,
		"agnes-image-2.0-flash":   "agnes-image-2.0-flash",
		" Agnes-Image-Future-v1 ": "Agnes-Image-Future-v1",
	} {
		if got := normalizeAgnesImageModel(input); got != want {
			t.Fatalf("normalizeAgnesImageModel(%q) = %q, want %q", input, got, want)
		}
	}

	for input, want := range map[string]string{
		"https://example.com":                       "https://example.com/v1/images/generations",
		"https://example.com/v1":                    "https://example.com/v1/images/generations",
		"https://example.com/v1/images/generations": "https://example.com/v1/images/generations",
		"https://example.com/images/generations":    "https://example.com/v1/images/generations",
	} {
		if got := agnesImageEndpoint(input); got != want {
			t.Fatalf("agnesImageEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGenerateAgnesImageUsesProviderSpecificPayload(t *testing.T) {
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %q, want /v1/images/generations", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		raw, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if unmarshalErr := json.Unmarshal(raw, &payload); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngData)}},
		})
	}))
	defer server.Close()

	oldClient := imageGenHTTPClient
	imageGenHTTPClient = server.Client()
	defer func() { imageGenHTTPClient = oldClient }()

	result, err := GenerateImage(ImageGenConfig{
		ProviderType: "agnes",
		BaseURL:      server.URL + "/v1",
		APIKey:       "test-key",
		Model:        defaultAgnesImageModel,
		DataDir:      t.TempDir(),
	}, "a tiny sunrise", ImageGenOptions{Size: "1024x1024"})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}
	if payload["return_base64"] != true {
		t.Fatalf("return_base64 = %v, want true", payload["return_base64"])
	}
	if _, exists := payload["response_format"]; exists {
		t.Fatalf("Agnes AI request must not contain top-level response_format: %v", payload)
	}
	if result.Provider != "agnes" || result.Model != defaultAgnesImageModel {
		t.Fatalf("provider/model = %q/%q", result.Provider, result.Model)
	}
	if _, err := os.Stat(result.filePath); err != nil {
		t.Fatalf("generated image was not saved: %v", err)
	}
}

func TestGenerateAgnesImagePlacesEditOptionsInExtraBody(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.png")
	sourceData := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if err := os.WriteFile(sourcePath, sourceData, 0600); err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(sourceData)}},
		})
	}))
	defer server.Close()

	oldClient := imageGenHTTPClient
	imageGenHTTPClient = server.Client()
	defer func() { imageGenHTTPClient = oldClient }()

	_, _, err := generateAgnesImage(ImageGenConfig{
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   defaultAgnesImageModel,
	}, "restyle", ImageGenOptions{SourceImage: sourcePath})
	if err != nil {
		t.Fatalf("generateAgnesImage failed: %v", err)
	}
	if _, exists := payload["return_base64"]; exists {
		t.Fatalf("image-to-image request should use extra_body, got %v", payload)
	}
	extraBody, ok := payload["extra_body"].(map[string]interface{})
	if !ok {
		t.Fatalf("extra_body missing: %v", payload)
	}
	if extraBody["response_format"] != "b64_json" {
		t.Fatalf("extra_body.response_format = %v", extraBody["response_format"])
	}
	images, ok := extraBody["image"].([]interface{})
	if !ok || len(images) != 1 {
		t.Fatalf("extra_body.image = %#v", extraBody["image"])
	}
}

func TestGenerateAgnesImageNormalizesChatModelAndAcceptsURLResponse(t *testing.T) {
	pngData := append(
		[]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a},
		bytes.Repeat([]byte{0x01}, 120)...,
	)
	var payload map[string]interface{}
	var serverURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]string{{"url": serverURL + "/generated.png"}},
			})
		case "/generated.png":
			_, _ = w.Write(pngData)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	defer server.Close()

	oldClient := imageGenHTTPClient
	imageGenHTTPClient = server.Client()
	defer func() { imageGenHTTPClient = oldClient }()

	result, err := GenerateImage(ImageGenConfig{
		ProviderType: "agnes",
		BaseURL:      server.URL,
		APIKey:       "test-key",
		Model:        "agnes-2.0-flash",
		DataDir:      t.TempDir(),
	}, "a mountain lake", ImageGenOptions{})
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}
	if payload["model"] != defaultAgnesImageModel || result.Model != defaultAgnesImageModel {
		t.Fatalf("request/result model = %v/%q, want %q", payload["model"], result.Model, defaultAgnesImageModel)
	}
	if payload["size"] != "1K" || result.Size != "1K" {
		t.Fatalf("request/result size = %v/%q, want 1K", payload["size"], result.Size)
	}
	saved, err := os.ReadFile(result.filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, pngData) {
		t.Fatal("URL response image was not saved unchanged")
	}
}

func TestDownloadImageRejectsOversizedBody(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write(bytes.Repeat([]byte("x"), 32))
	}))
	defer server.Close()

	oldClient := imageGenHTTPClient
	imageGenHTTPClient = server.Client()
	defer func() { imageGenHTTPClient = oldClient }()

	if _, err := downloadImageWithLimit(server.URL, 16); err == nil {
		t.Fatal("oversized image download unexpectedly succeeded")
	}
}
