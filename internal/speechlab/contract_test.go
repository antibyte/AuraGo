package speechlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSpeechLabVoiceModes(t *testing.T) {
	for _, tt := range []struct {
		name, mode, voice, defaultVoice, code string
		voices                                []string
	}{
		{"fixed default", "fixed", "M1", "M1", "", []string{"M1", "F1"}},
		{"fixed alternative", "fixed", "F1", "M1", "speech_lab_invalid_voice", []string{"M1", "F1"}},
		{"legacy fixed", "", "F1", "M1", "speech_lab_invalid_voice", []string{"M1", "F1"}},
		{"fixed missing default", "fixed", "", "", "speech_lab_invalid_voice", []string{"M1"}},
		{"request alternative", "request", "F1", "M1", "", []string{"M1", "F1"}},
		{"restart alternative", "restart", "F1", "M1", "", []string{"M1", "F1"}},
		{"normalized mode", " RESTART ", "F1", "M1", "", []string{"M1", "F1"}},
		{"unknown mode", "future", "M1", "M1", "speech_lab_voice_mode_unsupported", []string{"M1"}},
		{"unknown voice", "request", "unlisted", "M1", "speech_lab_invalid_voice", []string{"M1"}},
		{"default-only catalog", "request", "M1", "M1", "", nil},
		{"empty voices reject arbitrary", "request", "arbitrary", "M1", "speech_lab_invalid_voice", nil},
		{"omitted voice", "restart", "", "M1", "", []string{"M1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			catalog := Catalog{Backends: []CatalogBackend{
				{ID: "asr", Stage: "asr", Available: true},
				{ID: "tts", Stage: "tts", Available: true, VoiceMode: tt.mode, DefaultVoice: tt.defaultVoice, Voices: tt.voices},
			}}
			err := validateStackRequest(catalog, StackRequest{ASRID: "asr", TTSID: "tts", Voice: tt.voice})
			if tt.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var responseErr *ResponseError
			if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusBadRequest || ErrorCode(err) != tt.code {
				t.Fatalf("validation = %v, want 400 / %s", err, tt.code)
			}
		})
	}
}

func TestSpeechLabCatalogNormalizesMissingVoiceMode(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"backends":[{"id":"fixed","stage":"tts"},{"id":"restart","stage":"tts","voice_mode":" RESTART "}]}`)
	}))
	_, catalog, err := client.Catalog(context.Background())
	if err != nil || catalog.Backends[0].VoiceMode != "fixed" || catalog.Backends[1].VoiceMode != "restart" {
		t.Fatalf("catalog = %+v, error = %v", catalog, err)
	}
}

func TestSpeechLabResponseErrorsPreserveConflictsWithoutUpstreamText(t *testing.T) {
	for _, tt := range []struct {
		name, body, code   string
		status, wantStatus int
	}{
		{"voice", `{"code":"voice_not_active","error":"SECRET upstream text"}`, "voice_not_active", 409, 409},
		{"module", `{"error_code":"module_not_provisioned","message":"SECRET upstream text"}`, "module_not_provisioned", 409, 409},
		{"unknown", `{"code":"SECRET unknown","error":"SECRET upstream text"}`, "speech_lab_conflict", 409, 409},
		{"malformed", `SECRET not JSON`, "speech_lab_conflict", 409, 409},
		{"unavailable", `{"code":"voice_not_active","error":"SECRET upstream text"}`, "speech_lab_unavailable", 503, 503},
		{"bad gateway", `{"error":"SECRET upstream text"}`, "speech_lab_unavailable", 500, 502},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == catalogPath {
					_, _ = io.WriteString(w, `{"backends":[{"id":"asr","stage":"asr","available":true},{"id":"tts","stage":"tts","available":true,"default_voice":"M1"}]}`)
					return
				}
				if r.URL.Path == readyPath {
					t.Error("failed activation must not preflight another stack")
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			_, _, stackErr := client.ActivateStack(context.Background(), StackRequest{ASRID: "asr", TTSID: "tts", Voice: "M1"})
			_, _, _, ttsErr := client.Synthesize(context.Background(), "Hallo", "de", "M1", "tts", "M1")
			_, asrErr := client.Transcribe(context.Background(), testPCM16WAV(), "de", "asr")
			for _, err := range []error{stackErr, ttsErr, asrErr} {
				var responseErr *ResponseError
				if !errors.As(err, &responseErr) || responseErr.StatusCode != tt.wantStatus || ErrorCode(err) != tt.code {
					t.Fatalf("response = %v, want %d / %s", err, tt.wantStatus, tt.code)
				}
				if strings.Contains(err.Error(), "SECRET") {
					t.Fatalf("upstream text leaked: %v", err)
				}
			}
		})
	}
}

func TestSpeechLabRejectsInvalidVoiceBeforeStackMutation(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != catalogPath {
			t.Fatalf("invalid voice reached %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Catalog{Backends: []CatalogBackend{
			{ID: "asr", Stage: "asr", Available: true},
			{ID: "tts", Stage: "tts", Available: true, DefaultVoice: "M1", Voices: []string{"M1", "F1"}},
		}})
	}))
	_, _, err := client.ActivateStack(context.Background(), StackRequest{ASRID: "asr", TTSID: "tts", Voice: "F1"})
	if ErrorCode(err) != "speech_lab_invalid_voice" {
		t.Fatalf("error = %v", err)
	}
}

func TestSpeechLabBoundsTTSErrorBodies(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, strings.Repeat("x", maxJSONBytes+1))
	}))
	_, _, _, err := client.Synthesize(context.Background(), "Hallo", "de", "M1", "tts", "M1")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized TTS error body = %v", err)
	}
}
