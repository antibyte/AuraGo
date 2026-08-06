package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestPixelConfigIncludesCapabilities(t *testing.T) {
	cfg := &config.Config{}
	cfg.ImageGeneration.Enabled = true
	cfg.ImageGeneration.ProviderType = "openai"
	cfg.ImageGeneration.ResolvedModel = "dall-e-3"
	cfg.ImageGeneration.DefaultSize = "1024x1024"
	cfg.ImageGeneration.MaxMonthly = 100

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/pixel/config", nil)
	rr := httptest.NewRecorder()
	handlePixelConfig(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"supports_remove_bg", "supports_upscale", "max_monthly", "daily_count"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("config missing %q: %v", key, body)
		}
	}
	if body["supports_upscale"] != true {
		t.Fatalf("supports_upscale should be true, got %v", body["supports_upscale"])
	}
	if body["supports_remove_bg"] != true {
		t.Fatalf("supports_remove_bg should be true for openai, got %v", body["supports_remove_bg"])
	}
}

func TestPixelUpscaleDisabledWhenImageGenOff(t *testing.T) {
	cfg := &config.Config{}
	cfg.ImageGeneration.Enabled = false

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pixel/upscale", bytes.NewReader([]byte(`{"source_data":"data:image/png;base64,AA=="}`)))
	rr := httptest.NewRecorder()
	handlePixelUpscale(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestPixelUpscaleReturnsPNG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	cfg := &config.Config{}
	cfg.ImageGeneration.Enabled = true
	cfg.Directories.DataDir = t.TempDir()

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"source_data": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBuf.Bytes()),
		"scale":       2,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pixel/upscale", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	handlePixelUpscale(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected status: %v", body)
	}
	if int(body["width"].(float64)) != 16 || int(body["height"].(float64)) != 16 {
		t.Fatalf("expected 16x16 upscale, got %v x %v", body["width"], body["height"])
	}
}

func TestPixelRemoveBGDisabledWithoutImg2Img(t *testing.T) {
	cfg := &config.Config{}
	cfg.ImageGeneration.Enabled = true
	cfg.ImageGeneration.ProviderType = "google"

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pixel/remove-bg", bytes.NewReader([]byte(`{"source_data":"data:image/png;base64,AA=="}`)))
	rr := httptest.NewRecorder()
	handlePixelRemoveBG(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}
