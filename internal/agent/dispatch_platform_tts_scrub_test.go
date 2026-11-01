package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

// The tts action turns the model's text into audio that leaves AuraGo: a cloud provider hears
// it, and a chat channel can send it on as a voice note. A registered secret must be redacted
// before the synthesizer sees it. The provider's HTTP request is the seam.
func TestDispatchTTSScrubsRegisteredSecretsBeforeSynthesis(t *testing.T) {
	const secret = "tts-secret-value-4711"
	defer security.RegisterScopedSensitiveExact(secret)()

	var mu sync.Mutex
	var bodies []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("fixture audio"))
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = provider.URL
	cfg.Directories.DataDir = t.TempDir()

	out, ok := dispatchPlatform(context.Background(), ToolCall{Action: "tts", Text: "the key is " + secret + ", read it aloud"}, &DispatchContext{
		Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("tts was not synthesized: ok=%v out=%s", ok, out)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(bodies))
	}
	if strings.Contains(bodies[0], secret) || !strings.Contains(bodies[0], "[redacted]") || !strings.Contains(bodies[0], "read it aloud") {
		t.Fatalf("provider request = %q, want the secret redacted and the rest of the text kept", bodies[0])
	}
}
