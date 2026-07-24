package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestTranscribeAudioUsesInMemoryMultimodalPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content []struct {
					InputAudio *struct {
						Data   string `json:"data"`
						Format string `json:"format"`
					} `json:"input_audio"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		audio := payload.Messages[0].Content[1].InputAudio
		if audio == nil || audio.Data == "" || audio.Format != "wav" {
			t.Errorf("unexpected audio payload: %#v", audio)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "Hallo"}}},
		})
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Whisper.Mode = "multimodal"
	cfg.Whisper.BaseURL = server.URL
	cfg.Whisper.Model = "test-model"
	cfg.Whisper.APIKey = "test-key"
	text, _, err := TranscribeAudio(context.Background(), "call.wav", []byte("RIFF-in-memory"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Fatal("expected isolated transcription")
	}
}

func TestTranscribeAudioHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	cfg := &config.Config{}
	cfg.Whisper.Mode = "multimodal"
	cfg.Whisper.BaseURL = server.URL
	cfg.Whisper.Model = "test-model"
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, err := TranscribeAudio(ctx, "call.wav", []byte("RIFF-in-memory"), cfg)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("transcription request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled transcription returned no error")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled transcription did not stop")
	}
}

func TestTranscribeAudioUsesMistralVoxtralEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("path = %q, want /audio/transcriptions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mistral-key" {
			t.Errorf("Authorization = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart form: %v", err)
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
			return
		}
		if got := r.FormValue("model"); got != "voxtral-mini-latest" {
			t.Errorf("model = %q, want voxtral-mini-latest", got)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("missing audio file: %v", err)
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		_ = file.Close()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "voxtral-mini-2507",
			"text":  "Hallo von Mistral",
		})
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Whisper.ProviderType = "mistral"
	cfg.Whisper.Mode = "multimodal"
	cfg.Whisper.BaseURL = server.URL
	cfg.Whisper.APIKey = "mistral-key"
	cfg.Whisper.Model = "voxtral-mini-tts-2603"

	text, _, err := TranscribeAudio(context.Background(), "call.wav", []byte("RIFF-in-memory"), cfg)
	if err != nil {
		t.Fatalf("TranscribeAudio: %v", err)
	}
	if text == "" {
		t.Fatal("expected Mistral transcription")
	}
}
