package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/realtimespeech"
	"aurago/internal/security"
)

func newRealtimeSpeechTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `realtime_speech:
  enabled: true
  default_profile: main
  park_after_seconds: 5
  profiles:
    - id: main
      name: Main voice
      provider: openai
      model: gpt-realtime-2.1
      voice: marin
      enabled: true
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("7", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("realtime_speech_profile_main_api_key", "permanent-provider-key"); err != nil {
		t.Fatal(err)
	}
	cfg.ApplyVaultSecrets(vault)
	server := &Server{
		Cfg:    cfg,
		Vault:  vault,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	server.initConfigSnapshot()
	return server, configPath
}

func TestRealtimeSpeechConfigMasksAndPreservesAPIKey(t *testing.T) {
	server, configPath := newRealtimeSpeechTestServer(t)
	getRecorder := httptest.NewRecorder()
	handleRealtimeSpeechConfig(server).ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/realtime-speech/config", nil))
	if strings.Contains(getRecorder.Body.String(), "permanent-provider-key") {
		t.Fatal("GET config leaked the permanent provider key")
	}
	var getBody realtimeSpeechConfigJSON
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getBody); err != nil {
		t.Fatal(err)
	}
	if !getBody.Profiles[0].APIKeySet || getBody.Profiles[0].APIKey != "" {
		t.Fatalf("masked profile = %+v", getBody.Profiles[0])
	}

	update := `{"enabled":true,"default_profile":"main","park_after_seconds":5,"profiles":[{"id":"main","name":"Renamed","provider":"openai","model":"gpt-realtime-2.1","voice":"marin","enabled":true,"api_key":""}]}`
	putRecorder := httptest.NewRecorder()
	handleRealtimeSpeechConfig(server).ServeHTTP(putRecorder, httptest.NewRequest(http.MethodPut, "/api/realtime-speech/config", strings.NewReader(update)))
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", putRecorder.Code, putRecorder.Body.String())
	}
	key, err := server.Vault.ReadSecret("realtime_speech_profile_main_api_key")
	if err != nil || key != "permanent-provider-key" {
		t.Fatalf("preserved key = %q, err=%v", key, err)
	}
	yamlData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(yamlData), "permanent-provider-key") || strings.Contains(string(yamlData), "api_key:") {
		t.Fatalf("config file leaked API key:\n%s", yamlData)
	}
}

func TestRealtimeSpeechConfigClearAndDeleteRemoveVaultSecrets(t *testing.T) {
	server, _ := newRealtimeSpeechTestServer(t)
	clearBody := `{"enabled":false,"default_profile":"","park_after_seconds":5,"profiles":[{"id":"main","name":"Main","provider":"openai","model":"gpt-realtime-2.1","voice":"marin","enabled":false,"clear_api_key":true}]}`
	recorder := httptest.NewRecorder()
	handleRealtimeSpeechConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/realtime-speech/config", strings.NewReader(clearBody)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if value, err := server.Vault.ReadSecret("realtime_speech_profile_main_api_key"); err == nil && value != "" {
		t.Fatalf("cleared key still exists: %q", value)
	}
	if profile, ok := realtimeSpeechProfileByID(server, "main"); !ok || profile.APIKey != "" {
		t.Fatalf("runtime profile retained cleared API key: exists=%v key_set=%v", ok, profile.APIKey != "")
	}

	if err := server.Vault.WriteSecret("realtime_speech_profile_main_api_key", "recreated"); err != nil {
		t.Fatal(err)
	}
	deleteBody := `{"enabled":false,"default_profile":"","park_after_seconds":5,"profiles":[]}`
	recorder = httptest.NewRecorder()
	handleRealtimeSpeechConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/realtime-speech/config", strings.NewReader(deleteBody)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if value, err := server.Vault.ReadSecret("realtime_speech_profile_main_api_key"); err == nil && value != "" {
		t.Fatalf("deleted profile key still exists: %q", value)
	}
}

func TestRealtimeSpeechOpenAISessionNeverReturnsPermanentKey(t *testing.T) {
	server, _ := newRealtimeSpeechTestServer(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer permanent-provider-key" {
			t.Fatalf("server-side authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, "answer-sdp")
	}))
	defer provider.Close()
	client := realtimespeech.NewClient()
	client.OpenAIBaseURL = provider.URL
	registry := realtimespeech.NewRegistry(nil)
	body := `{"client_id":"browser-one","profile_id":"main","surface":"webchat","chat_session_id":"default","offer_sdp":"offer-sdp"}`
	request := httptest.NewRequest(http.MethodPost, "/api/realtime-speech/sessions", strings.NewReader(body))
	request.Header.Set("X-Realtime-Speech-Client-ID", "browser-one")
	recorder := httptest.NewRecorder()
	handleRealtimeSpeechSessions(server, registry, client).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("session status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "permanent-provider-key") {
		t.Fatal("session response leaked permanent provider key")
	}
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["answer_sdp"] != "answer-sdp" || response["transport"] != "webrtc" {
		t.Fatalf("session response = %+v", response)
	}
}

func TestRealtimeSpeechSessionStateRequiresLeaseOwner(t *testing.T) {
	var logs bytes.Buffer
	server := &Server{Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	registry := realtimespeech.NewRegistry(nil)
	session, _, err := registry.Acquire("browser-owner", realtimespeech.Session{
		ProfileID: "main",
		Provider:  realtimespeech.ProviderGemini,
		State:     "listening",
		Surface:   "webchat",
		ClientID:  "browser-owner",
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	update := func(clientID, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(
			http.MethodPatch,
			"/api/realtime-speech/sessions/"+session.ID+"?client_id="+clientID,
			strings.NewReader(body),
		)
		request.Header.Set("X-Realtime-Speech-Client-ID", clientID)
		recorder := httptest.NewRecorder()
		handleRealtimeSpeechSessionByID(server, registry).ServeHTTP(recorder, request)
		return recorder
	}

	recorder := update("browser-owner", `{"state":"parked","resumption_handle":"resume-1"}`)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("owner update status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	updated, ok := registry.Get(session.ID, "browser-owner")
	if !ok || updated.State != "parked" || updated.ResumptionHandle != "resume-1" {
		t.Fatalf("updated session = %+v, ok=%v", updated, ok)
	}

	recorder = update("browser-other", `{"state":"listening"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("non-owner update status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder = update("browser-owner", `{"state":"secret-state"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid state status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	security.RegisterSensitive("provider-secret-marker")
	recorder = update("browser-owner", `{"state":"error","error_message":"Gemini rejected provider-secret-marker"}`)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("error update status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if output := logs.String(); !strings.Contains(output, "Browser provider session failed") ||
		!strings.Contains(output, "provider=gemini") || strings.Contains(output, "provider-secret-marker") {
		t.Fatalf("unsafe or incomplete realtime speech error log: %s", output)
	}
}

func TestRealtimeSpeechMutationsRejectCrossOriginRequests(t *testing.T) {
	server, _ := newRealtimeSpeechTestServer(t)
	request := httptest.NewRequest(
		http.MethodPut,
		"https://aurago.local/api/realtime-speech/config",
		strings.NewReader(`{"enabled":false,"default_profile":"","park_after_seconds":5,"profiles":[]}`),
	)
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handleRealtimeSpeechConfig(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin update status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRealtimeSpeechActionBindsChatAndSuppressesTTS(t *testing.T) {
	server, _ := newRealtimeSpeechTestServer(t)
	registry := realtimespeech.NewRegistry(nil)
	globalBroker := NewSSEBroadcaster()
	globalEvents := globalBroker.subscribe()
	defer globalBroker.unsubscribe(globalEvents)
	session, _, err := registry.Acquire("browser", realtimespeech.Session{
		ProfileID:     "main",
		Provider:      realtimespeech.ProviderOpenAI,
		Surface:       "webchat",
		ChatSessionID: "chat-42",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	webAction := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if !agent.VoiceOutputSuppressed(r.Context()) {
			t.Fatal("action context did not suppress AuraGo TTS")
		}
		if r.Header.Get("X-Session-ID") != "chat-42" {
			t.Fatalf("X-Session-ID = %q", r.Header.Get("X-Session-ID"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		broker := feedbackBrokerForRequestContext(r.Context(), globalBroker, "chat-42", "", false)
		broker.SendLLMStreamDelta("Das Licht ist eingeschaltet.", "", "", 0, "")
		broker.Send("final_response", "Das Licht ist eingeschaltet.")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	body := `{"session_id":"` + session.ID + `","client_id":"browser","request_id":"request-1","request":"Schalte das Licht ein"}`
	request := httptest.NewRequest(http.MethodPost, "/api/realtime-speech/actions", strings.NewReader(body))
	request.Header.Set("X-Realtime-Speech-Client-ID", "browser")
	recorder := httptest.NewRecorder()
	handleRealtimeSpeechActions(server, registry, webAction, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("desktop handler should not be called")
	})).ServeHTTP(recorder, request)
	if !called || recorder.Code != http.StatusOK {
		t.Fatalf("web action called=%v status=%d body=%s", called, recorder.Code, recorder.Body.String())
	}
	responseBody := recorder.Body.String()
	if !strings.Contains(responseBody, `"event":"llm_stream_delta"`) ||
		!strings.Contains(responseBody, `"event":"final_response"`) ||
		!strings.Contains(responseBody, "Das Licht ist eingeschaltet.") {
		t.Fatalf("private action stream is incomplete: %s", responseBody)
	}
	select {
	case event := <-globalEvents:
		t.Fatalf("realtime speech action leaked into the global chat stream: %s", event)
	default:
	}
}

func TestRealtimeSpeechTurnsPersistOnce(t *testing.T) {
	server, _ := newRealtimeSpeechTestServer(t)
	stm, err := memory.NewSQLiteMemory(":memory:", server.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	server.ShortTermMem = stm
	registry := realtimespeech.NewRegistry(nil)
	session, _, err := registry.Acquire("browser", realtimespeech.Session{
		ProfileID:     "main",
		Provider:      realtimespeech.ProviderOpenAI,
		Surface:       "webchat",
		ChatSessionID: "chat-voice",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"session_id":"` + session.ID + `","client_id":"browser","turn_id":"turn-1","user_transcript":"Wie geht es dir?","assistant_transcript":"Mir geht es gut."}`
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/realtime-speech/turns", strings.NewReader(body))
		request.Header.Set("X-Realtime-Speech-Client-ID", "browser")
		recorder := httptest.NewRecorder()
		handleRealtimeSpeechTurns(server, registry).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("turn %d status=%d body=%s", i, recorder.Code, recorder.Body.String())
		}
	}
	messages, err := stm.GetSessionMessages("chat-voice")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Content != "Wie geht es dir?" || messages[1].Content != "Mir geht es gut." {
		t.Fatalf("persisted messages = %+v", messages)
	}
}
