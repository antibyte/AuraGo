package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/security"
)

// ttsRecordingProvider is a Supertonic-shaped provider that records every
// request body it receives.
type ttsRecordingProvider struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
}

func newTTSRecordingProvider(t *testing.T) *ttsRecordingProvider {
	t.Helper()
	p := &ttsRecordingProvider{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		p.mu.Lock()
		p.bodies = append(p.bodies, string(raw))
		p.mu.Unlock()
		_, _ = io.WriteString(w, "audio")
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *ttsRecordingProvider) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.bodies...)
}

func TestTTSSynthesisScrubsRegisteredSecrets(t *testing.T) {
	const secret = "tts-secret-0123456789abcdef"
	defer security.RegisterScopedSensitiveExact(secret)()

	text := "Der Schluessel lautet " + secret + " und das war alles."

	tests := []struct {
		name       string
		synthesize func(t *testing.T, cfg TTSConfig, text string) error
	}{
		{
			name: "TTSSynthesize",
			synthesize: func(t *testing.T, cfg TTSConfig, text string) error {
				cfg.DataDir = t.TempDir()
				_, err := TTSSynthesize(cfg, text)
				return err
			},
		},
		{
			name: "TTSSynthesizeInMemoryContext",
			synthesize: func(t *testing.T, cfg TTSConfig, text string) error {
				_, _, err := TTSSynthesizeInMemoryContext(t.Context(), cfg, text)
				return err
			},
		},
		{
			name: "TTSSynthesizeInMemory",
			synthesize: func(t *testing.T, cfg TTSConfig, text string) error {
				_, _, err := TTSSynthesizeInMemory(cfg, text)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newTTSRecordingProvider(t)
			cfg := TTSConfig{Provider: "supertonic", Language: "de"}
			cfg.Supertonic.URL = provider.URL

			if err := tt.synthesize(t, cfg, text); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			bodies := provider.recorded()
			if len(bodies) != 1 {
				t.Fatalf("provider requests = %d, want 1", len(bodies))
			}
			if strings.Contains(bodies[0], secret) {
				t.Fatalf("provider request carries the registered secret: %s", bodies[0])
			}
			var request struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(bodies[0]), &request); err != nil {
				t.Fatalf("decode provider request: %v", err)
			}
			const want = "Der Schluessel lautet [redacted] und das war alles."
			if request.Text != want {
				t.Fatalf("provider text = %q, want %q", request.Text, want)
			}
		})
	}
}

func TestTTSSynthesizeCacheKeyUsesScrubbedText(t *testing.T) {
	const secret = "tts-cache-secret-0123456789"
	defer security.RegisterScopedSensitiveExact(secret)()

	provider := newTTSRecordingProvider(t)
	cfg := TTSConfig{Provider: "supertonic", Language: "de", DataDir: t.TempDir()}
	cfg.Supertonic.URL = provider.URL

	withSecret, err := TTSSynthesize(cfg, "Code "+secret)
	if err != nil {
		t.Fatal(err)
	}
	withPlaceholder, err := TTSSynthesize(cfg, "Code [redacted]")
	if err != nil {
		t.Fatal(err)
	}
	if withSecret != withPlaceholder {
		t.Fatalf("cache files differ: %q vs %q; the secret reached the cache key", withSecret, withPlaceholder)
	}
	if got := len(provider.recorded()); got != 1 {
		t.Fatalf("provider requests = %d, want 1 (second call served from the scrubbed cache entry)", got)
	}
}

func TestTTSSynthesisEmptyTextStillErrors(t *testing.T) {
	provider := newTTSRecordingProvider(t)
	cfg := TTSConfig{Provider: "supertonic", Language: "de", DataDir: t.TempDir()}
	cfg.Supertonic.URL = provider.URL

	if _, err := TTSSynthesize(cfg, ""); err == nil || !strings.Contains(err.Error(), "text is required") {
		t.Fatalf("TTSSynthesize(\"\") error = %v, want \"text is required\"", err)
	}
	if _, _, err := TTSSynthesizeInMemoryContext(t.Context(), cfg, ""); err == nil || !strings.Contains(err.Error(), "text is required") {
		t.Fatalf("TTSSynthesizeInMemoryContext(\"\") error = %v, want \"text is required\"", err)
	}
	if got := len(provider.recorded()); got != 0 {
		t.Fatalf("provider requests = %d, want 0", got)
	}
}
