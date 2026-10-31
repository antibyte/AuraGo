package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/agent"
)

func TestSpeechLabStackConflictPreservesCodeAndSanitizesMessage(t *testing.T) {
	for _, code := range []string{"module_not_provisioned", "voice_not_active", "unknown_upstream_code"} {
		t.Run(code, func(t *testing.T) {
			s, _ := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/catalog" {
					_, _ = io.WriteString(w, `{"backends":[{"id":"asr","stage":"asr","available":true},{"id":"tts","stage":"tts","available":true,"default_voice":"M1"}]}`)
					return
				}
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": code, "message": "SECRET upstream diagnostic"})
			}))
			recorder := httptest.NewRecorder()
			handleSpeechLabStack(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/speech-lab/stack", strings.NewReader(`{"asr_id":"asr","tts_id":"tts","voice":"M1"}`)))
			wantCode := code
			if code == "unknown_upstream_code" {
				wantCode = "speech_lab_conflict"
			}
			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusConflict || body["error"] != wantCode || strings.Contains(recorder.Body.String(), "SECRET") {
				t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSpeechLabTTSConflictReachesRESTAndSSE(t *testing.T) {
	for _, code := range []string{"voice_not_active", "module_not_provisioned", "unknown_upstream_code"} {
		t.Run(code, func(t *testing.T) {
			wantCode := code
			if code == "unknown_upstream_code" {
				wantCode = "speech_lab_conflict"
			}
			var readyCalls atomic.Int32
			s, client := speechLabServerForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ready" {
					readyCalls.Add(1)
					_, _ = io.WriteString(w, `{"ready":true,"asr_id":"asr","tts_id":"tts","voice":"M1","asr_ok":true,"tts_ok":true}`)
					return
				}
				if r.URL.Path != "/v1/audio/speech" {
					t.Errorf("unexpected inference request: %s", r.URL.Path)
				}
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "error": "SECRET upstream diagnostic", "active_voice": "private voice"})
			}))
			_, _, _, err := client.Synthesize(context.Background(), "Hallo", "de", "M1", "tts", "M1")
			if err == nil {
				t.Fatal("expected voice conflict")
			}
			recorder := httptest.NewRecorder()
			writeSpeechLabError(recorder, err)
			if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"error":"`+wantCode+`"`) {
				t.Fatalf("REST response = %d %s", recorder.Code, recorder.Body.String())
			}
			base := &chatVoiceOutputCaptureBroker{}
			broker := newChatVoiceOutputTrackingBroker(base)
			s.Cfg.Directories.DataDir = t.TempDir()
			s.Cfg.SpeechLab.ChatOutputEnabled = true
			maybeEmitChatVoiceOutputFallback(context.Background(), s.Cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Die Textantwort bleibt verfügbar.", client)
			if len(base.events) != 1 || base.events[0].event != "speech_lab_error" || !strings.Contains(base.events[0].message, `"code":"`+wantCode+`"`) {
				t.Fatalf("SSE events = %+v", base.events)
			}
			if strings.Contains(recorder.Body.String()+base.events[0].message, "SECRET") || strings.Contains(recorder.Body.String()+base.events[0].message, "private voice") {
				t.Fatal("upstream diagnostics leaked to REST or SSE")
			}
			if broker.hasTTSAudio() {
				t.Fatal("voice conflict emitted audio")
			}
			if readyCalls.Load() != 1 {
				t.Fatalf("chat turn used %d readiness snapshots, want one", readyCalls.Load())
			}
		})
	}
}
