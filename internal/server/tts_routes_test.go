package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestTTSRoutes(t *testing.T) {
	phonon := &config.Config{}
	phonon.TTS.Provider = "phonon"
	got := tools.EffectiveTTSRoutes(phonon)
	if len(got) != 4 {
		t.Fatalf("routes = %+v", got)
	}
	for _, route := range got[:3] {
		if route.Provider != "phonon" || !route.Ready {
			t.Fatalf("phonon route = %+v", route)
		}
	}
	if got[3].Code != "live_speech" || got[3].Ready || got[3].Provider != "" {
		t.Fatalf("live speech = %+v", got[3])
	}

	lab := &config.Config{}
	lab.TTS.Provider = "phonon"
	lab.SpeechLab.Enabled = true
	lab.SpeechLab.BaseURL = "http://127.0.0.1:9"
	lab.SpeechLab.ChatOutputEnabled = true
	got = tools.EffectiveTTSRoutes(lab)
	if got[0].Code != "web_chat" || got[0].Provider != "speech_lab" || !got[0].Ready {
		t.Fatalf("web chat = %+v", got[0])
	}
	if got[1].Provider != "phonon" || !got[1].Ready || got[2].Provider != "phonon" || !got[2].Ready {
		t.Fatalf("tool/glass = %+v %+v", got[1], got[2])
	}

	unready := &config.Config{}
	unready.TTS.Provider = "speech_lab"
	unready.SpeechLab.BaseURL = "http://127.0.0.1:9"
	got = tools.EffectiveTTSRoutes(unready)
	if got[1].Ready || got[1].Provider != "speech_lab" {
		t.Fatalf("unready tool = %+v", got[1])
	}

	secret := &config.Config{}
	secret.TTS.Provider = "openai"
	secret.TTS.OpenAI.APIKey = "vault-secret-value"
	secret.TTS.OpenAI.VoiceID = "coral"
	secret.RealtimeSpeech.Enabled = true
	secret.RealtimeSpeech.DefaultProfile = "room"
	secret.RealtimeSpeech.Profiles = []config.RealtimeSpeechProfile{{
		ID: "room", Provider: "openai", Voice: "marin", Enabled: true, APIKey: "live-secret-value",
	}}
	body, err := json.Marshal(tools.EffectiveTTSRoutes(secret))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	for _, hidden := range []string{"vault-secret-value", "live-secret-value", "coral", "marin", "http"} {
		if strings.Contains(encoded, hidden) {
			t.Fatalf("route payload leaked %q: %s", hidden, encoded)
		}
	}
	if routes := tools.EffectiveTTSRoutes(secret); !routes[3].Ready || routes[3].Provider != "openai" {
		t.Fatalf("live speech with a key = %+v", routes[3])
	}
	secret.RealtimeSpeech.Profiles[0].APIKey = ""
	if routes := tools.EffectiveTTSRoutes(secret); routes[3].Ready {
		t.Fatalf("live speech without a key = %+v", routes[3])
	}
	secret.RealtimeSpeech.Profiles[0].Provider = "speech_lab"
	if routes := tools.EffectiveTTSRoutes(secret); !routes[3].Ready || routes[3].Provider != "speech_lab" {
		t.Fatalf("keyless live speech = %+v", routes[3])
	}

	s := &Server{}
	s.Cfg = phonon
	recorder := httptest.NewRecorder()
	handleTTSRoutes(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/tts/routes", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d cache %q", recorder.Code, recorder.Header().Get("Cache-Control"))
	}
}
