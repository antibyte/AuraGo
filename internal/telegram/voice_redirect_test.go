package telegram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/httporigin"
)

// Same-origin redirect policy: the voice transcription clients upload the audio with the
// Whisper provider key, so they must not follow a redirect off the configured
// origin. Server A is the configured provider and answers with a 307 to server
// B on another loopback port; B must never be reached.
func TestVoiceTranscriptionDoesNotFollowCrossOriginRedirect(t *testing.T) {
	for name, transcribe := range map[string]func(string, *config.Config) (string, error){
		"whisper":    TranscribeVoice,
		"multimodal": TranscribeMultimodal,
	} {
		t.Run(name, func(t *testing.T) {
			var secondHits atomic.Int32
			second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				secondHits.Add(1)
			}))
			defer second.Close()
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", second.URL+r.URL.Path)
				w.WriteHeader(http.StatusTemporaryRedirect)
			}))
			defer provider.Close()

			audioPath := filepath.Join(t.TempDir(), "voice.mp3")
			if err := os.WriteFile(audioPath, []byte("SECRET-AUDIO"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{}
			cfg.Whisper.BaseURL = provider.URL + "/v1"
			cfg.Whisper.APIKey = "sk-whisper"
			cfg.Whisper.Model = "whisper-1"

			_, err := transcribe(audioPath, cfg)
			if hits := secondHits.Load(); hits != 0 {
				t.Fatalf("second origin received %d request(s)", hits)
			}
			if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
				t.Fatalf("error = %v, want httporigin.ErrCrossOriginRedirect", err)
			}
		})
	}
}
