package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/testutil"
)

func TestTTSElevenLabsEscapesTheVoiceIDAndSendsRealJSON(t *testing.T) {
	// The quote and the umlaut must survive. U+007F is here because fmt's %q writes it as \x7f,
	// an escape JSON does not have, so the old body could not be parsed.
	text := "Sie sagt \"Grüße\" \x7f"
	type request struct {
		path string
		body []byte
	}
	seen := make(chan request, 1)
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen <- request{path: r.URL.EscapedPath(), body: body}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3fixture-audio"))
	}))
	defer server.Close()
	routeVendorHostTo(t, &ttsHTTPClient, "api.elevenlabs.io", server)

	cfg := TTSConfig{}
	cfg.ElevenLabs.APIKey = "fixture-key"
	cfg.ElevenLabs.VoiceID = "team/voice"
	cfg.ElevenLabs.ModelID = "model-x"
	audio, err := ttsElevenLabs(cfg, text)
	if err != nil || string(audio) != "ID3fixture-audio" {
		t.Fatalf("ttsElevenLabs = %q, %v", audio, err)
	}
	got := <-seen
	// PathEscape writes the slash as %2F, so the id stays one path segment.
	if got.path != "/v1/text-to-speech/team%2Fvoice" {
		t.Errorf("path = %q", got.path)
	}
	var body struct {
		Text          string `json:"text"`
		ModelID       string `json:"model_id"`
		VoiceSettings struct {
			Stability       float64 `json:"stability"`
			SimilarityBoost float64 `json:"similarity_boost"`
		} `json:"voice_settings"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatalf("the body is not JSON: %v\n%s", err, got.body)
	}
	if body.Text != text || body.ModelID != "model-x" || body.VoiceSettings.Stability != 0.5 || body.VoiceSettings.SimilarityBoost != 0.75 {
		t.Errorf("body = %+v", body)
	}
}

func TestTTSMiniMaxRefusesABodyOverTheLimit(t *testing.T) {
	// Valid JSON with valid hex, so only the size limit can refuse it. The hex alone fills the limit,
	// and the JSON around it goes past it.
	body := `{"data":{"audio":"` + strings.Repeat("ab", int(maxHTTPResponseSize)/2) + `"},"base_resp":{"status_code":0}}`
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	routeVendorHostTo(t, &ttsHTTPClient, "api.minimax.io", server)

	cfg := TTSConfig{}
	cfg.MiniMax.APIKey = "fixture-key"
	audio, err := ttsMiniMax(cfg, "text")
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("ttsMiniMax = %d bytes, %v; want the size limit error", len(audio), err)
	}
}

func TestOpenAITTSVoicesListsThirteen(t *testing.T) {
	want := []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer", "verse", "marin", "cedar"}
	got := OpenAITTSVoices()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenAITTSVoices = %v, want %v", got, want)
	}
	got[0] = "changed"
	if OpenAITTSVoices()[0] != "alloy" {
		t.Fatal("callers share the list")
	}
}
