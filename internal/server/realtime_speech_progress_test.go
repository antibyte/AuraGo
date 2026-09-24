package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/realtimespeech"
	"aurago/internal/speechlab"
	"aurago/ui"
)

func TestRealtimeSpeechProgressAudioUsesActiveSpeechLabVoice(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	server, _ := newRealtimeSpeechTestServer(t)
	server.Cfg.Server.UILanguage = "de"
	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ready": true, "asr_id": "asr-a", "tts_id": "tts-a", "asr_ok": true, "tts_ok": true, "voice": "Serena",
			})
		case "/v1/audio/speech":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["input"] != "Ich schaue mir das an. Einen Moment." || body["voice"] != "Serena" {
				t.Fatalf("unexpected progress synthesis: %#v", body)
			}
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("X-S2S-TTS-ID", "tts-a")
			w.Header().Set("X-S2S-Voice", "Serena")
			_, _ = w.Write(realtimeSpeechLabTestWAV())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(lab.Close)
	server.Cfg.SpeechLab = config.SpeechLabConfig{Enabled: true, BaseURL: lab.URL, TimeoutSeconds: 2, Language: "de"}
	client, err := speechlab.NewClient(server.Cfg.SpeechLab)
	if err != nil {
		t.Fatal(err)
	}
	server.SpeechLab = client
	server.initConfigSnapshot()
	registry := realtimespeech.NewRegistry(nil)
	session, _, err := registry.Acquire("browser", realtimespeech.Session{Provider: realtimespeech.ProviderSpeechLab, Surface: "webchat"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.BeginAction("action-1", session.ID, "browser", session.ChatSessionID); err != nil {
		t.Fatal(err)
	}
	defer registry.EndAction("action-1")
	requestBody := `{"session_id":"` + session.ID + `","client_id":"browser","request_id":"action-1","kind":"ack"}`
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/realtime-speech/progress-audio", strings.NewReader(requestBody))
	request.Header.Set("X-Realtime-Speech-Client-ID", "browser")
	request.Header.Set("Origin", "https://aurago.local")
	response := httptest.NewRecorder()
	handleRealtimeSpeechProgressAudio(server, registry).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "audio/wav" || response.Body.Len() == 0 {
		t.Fatalf("progress audio status=%d type=%q bytes=%d body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.Len(), response.Body.String())
	}
}

func TestRealtimeSpeechProgressAudioRejectsOtherOriginsAndSessions(t *testing.T) {
	server, _ := newRealtimeSpeechTestServer(t)
	registry := realtimespeech.NewRegistry(nil)
	session, _, err := registry.Acquire("browser", realtimespeech.Session{Provider: realtimespeech.ProviderOpenAI}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.BeginAction("action-1", session.ID, "browser", session.ChatSessionID); err != nil {
		t.Fatal(err)
	}
	defer registry.EndAction("action-1")
	handler := handleRealtimeSpeechProgressAudio(server, registry)
	for _, test := range []struct {
		name   string
		origin string
		client string
		kind   string
		status int
	}{
		{"cross origin", "https://other.example", "browser", "ack", http.StatusForbidden},
		{"other session", "https://aurago.local", "other", "ack", http.StatusNotFound},
		{"invalid kind", "https://aurago.local", "browser", "arbitrary", http.StatusBadRequest},
		{"no tts", "https://aurago.local", "browser", "ack", http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := `{"session_id":"` + session.ID + `","client_id":"` + test.client + `","request_id":"action-1","kind":"` + test.kind + `"}`
			request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/realtime-speech/progress-audio", strings.NewReader(body))
			request.Header.Set("Origin", test.origin)
			request.Header.Set("X-Realtime-Speech-Client-ID", test.client)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
		})
	}
	unknownText := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/realtime-speech/progress-audio",
		strings.NewReader(`{"session_id":"`+session.ID+`","client_id":"browser","request_id":"action-1","kind":"ack","text":"untrusted"}`))
	unknownText.Header.Set("X-Realtime-Speech-Client-ID", "browser")
	unknownResponse := httptest.NewRecorder()
	handler.ServeHTTP(unknownResponse, unknownText)
	if unknownResponse.Code != http.StatusBadRequest {
		t.Fatalf("arbitrary text status=%d want=%d", unknownResponse.Code, http.StatusBadRequest)
	}
	registry.EndAction("action-1")
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/realtime-speech/progress-audio",
		strings.NewReader(`{"session_id":"`+session.ID+`","client_id":"browser","request_id":"action-1","kind":"ack"}`))
	request.Header.Set("X-Realtime-Speech-Client-ID", "browser")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("completed action status=%d want=%d", response.Code, http.StatusNotFound)
	}
}
