package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestSpeechLabTranscribeAndSynthesize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/ready"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ready": true, "asr_id": "fw-tiny", "tts_id": "supertonic",
				"asr_ok": true, "tts_ok": true, "message": "ok",
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "transcriptions"):
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			if _, ok := r.MultipartForm.File["file"]; !ok {
				http.Error(w, "missing file", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"text": "hallo welt", "asr_id": "fw-tiny"})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "speech"):
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if payload["input"] == nil {
				http.Error(w, "missing input", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("X-S2S-TTS-ID", "supertonic")
			w.Header().Set("X-S2S-Voice", "M1")
			_, _ = w.Write(PCMToWAV(make([]byte, 320), 16000, 2, 1))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.SpeechLabConfig{
		Enabled: true,
		BaseURL: server.URL,
	}
	ready, err := SpeechLabCheckReady(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !ready.Ready || ready.ASRID != "fw-tiny" {
		t.Fatalf("ready=%+v", ready)
	}
	text, err := SpeechLabTranscribe(context.Background(), PCMToWAV(make([]byte, 320), 16000, 2, 1), "de", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hallo welt" {
		t.Fatalf("text=%q", text)
	}
	audio, ext, err := SpeechLabSynthesize(context.Background(), "Hallo", "de", "M1", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".wav" || len(audio) < 4 {
		t.Fatalf("audio ext=%s len=%d", ext, len(audio))
	}
}

func TestSpeechLabInactive(t *testing.T) {
	_, err := SpeechLabTranscribe(context.Background(), []byte("x"), "de", config.SpeechLabConfig{})
	if err == nil {
		t.Fatal("expected error when disabled")
	}
}
