package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/testutil"
)

func cloudTTSFixture(provider string) TTSConfig {
	cfg := TTSConfig{Provider: provider, Language: "de"}
	cfg.XAI.APIKey = "fixture-cloud-key"
	cfg.OpenAI.APIKey = "fixture-cloud-key"
	cfg.StepFun.APIKey = "fixture-cloud-key"
	cfg.Resemble.APIKey = "fixture-cloud-key"
	cfg.Resemble.VoiceID = "voice-uuid"
	return cfg
}

func TestTTSCloudProvidersRequestPlaybackAndCache(t *testing.T) {
	for _, tc := range []struct{ provider, host, path, voice, model string }{
		{"xai", "api.x.ai", "/v1/tts", "eve", ""},
		{"openai", "api.openai.com", "/v1/audio/speech", "coral", "gpt-4o-mini-tts"},
		{"stepfun", "api.stepfun.com", "/v1/audio/speech", "jingdiannvsheng", "step-tts-mini"},
		{"resemble", "f.cluster.resemble.ai", "/synthesize", "voice-uuid", ""},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			calls := 0
			wantVoice, wantModel := tc.voice, tc.model
			audio := []byte("ID3fixture-audio")
			server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != tc.path || r.Header.Get("Authorization") != "Bearer fixture-cloud-key" {
					t.Error("incorrect speech endpoint, method or credentials")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				switch tc.provider {
				case "xai":
					format, _ := body["output_format"].(map[string]any)
					if body["text"] != "Hallo" || body["voice_id"] != wantVoice || body["language"] != "de" || format["codec"] != "mp3" {
						t.Errorf("incorrect xAI speech payload: %v", body)
					}
				case "resemble":
					if body["data"] != "Hallo" || body["voice_uuid"] != wantVoice || body["output_format"] != "mp3" {
						t.Errorf("incorrect Resemble speech payload: %v", body)
					}
				default:
					if body["input"] != "Hallo" || body["voice"] != wantVoice || body["model"] != wantModel || body["response_format"] != "mp3" {
						t.Errorf("incorrect speech payload: %v", body)
					}
				}
				if tc.provider == "resemble" {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "audio_content": base64.StdEncoding.EncodeToString(audio)})
				} else {
					w.Header().Set("Content-Type", "audio/mpeg")
					_, _ = w.Write(audio)
				}
			}))
			defer server.Close()
			routeVendorHostTo(t, &ttsHTTPClient, tc.host, server)
			cfg := cloudTTSFixture(tc.provider)
			cfg.Provider = " " + strings.ToUpper(tc.provider) + " "
			cfg.DataDir = t.TempDir()
			first, err := TTSSynthesize(cfg, "Hallo")
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(cfg.DataDir, "tts", first))
			if err != nil || string(got) != string(audio) || !strings.HasSuffix(first, ".mp3") {
				t.Fatalf("invalid cached audio: %q, %v", first, err)
			}
			second, err := TTSSynthesize(cfg, "Hallo")
			if err != nil || second != first || calls != 1 {
				t.Fatalf("same settings did not reuse speech cache: calls=%d err=%v", calls, err)
			}
			wantVoice = "custom-voice"
			switch tc.provider {
			case "xai":
				cfg.XAI.VoiceID = wantVoice
			case "openai":
				cfg.OpenAI.VoiceID = wantVoice
			case "stepfun":
				cfg.StepFun.VoiceID = wantVoice
			case "resemble":
				cfg.Resemble.VoiceID = wantVoice
			}
			third, err := TTSSynthesize(cfg, "Hallo")
			if err != nil || third == first || calls != 2 {
				t.Fatalf("changed voice did not invalidate cache: calls=%d err=%v", calls, err)
			}
			if tc.model != "" {
				wantModel = "custom-model"
				if tc.provider == "openai" {
					cfg.OpenAI.ModelID = wantModel
				} else {
					cfg.StepFun.ModelID = wantModel
				}
				fourth, err := TTSSynthesize(cfg, "Hallo")
				if err != nil || fourth == third || calls != 3 {
					t.Fatalf("changed model did not invalidate cache: calls=%d err=%v", calls, err)
				}
			}
			data, ext, err := TTSSynthesizeInMemory(cfg, "Hallo")
			if err != nil || string(data) != string(audio) || ext != ".mp3" {
				t.Fatalf("transient speech failed: ext=%q err=%v", ext, err)
			}
		})
	}
}

func TestTTSCloudRejectsFailuresAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, provider, body, contentType string
		status                            int
	}{
		{"http-error", "xai", "provider error", "text/plain", http.StatusUnauthorized},
		{"empty-audio", "openai", "", "audio/mpeg", http.StatusOK},
		{"json-not-audio", "stepfun", `{}`, "application/json", http.StatusOK},
		{"resemble-failure", "resemble", `{"success":false,"audio_content":"SUQz"}`, "application/json", http.StatusOK},
		{"resemble-invalid-json", "resemble", `{`, "application/json", http.StatusOK},
		{"resemble-invalid-base64", "resemble", `{"success":true,"audio_content":"!"}`, "application/json", http.StatusOK},
		{"resemble-empty-audio", "resemble", `{"success":true,"audio_content":""}`, "application/json", http.StatusOK},
		{"oversized", "openai", strings.Repeat("a", int(maxHTTPResponseSize)+1), "audio/mpeg", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			withTTSTestClient(t, server, func() {
				cfg := cloudTTSFixture(tc.provider)
				cfg.DataDir = t.TempDir()
				if _, err := TTSSynthesize(cfg, "Hallo"); err == nil {
					t.Fatal("invalid response was accepted")
				}
				files, err := os.ReadDir(TTSAudioDir(cfg.DataDir))
				if err != nil || len(files) != 0 {
					t.Fatalf("failed synthesis left cache files: %v", err)
				}
			})
		})
	}
	for _, provider := range []string{"xai", "openai", "stepfun", "resemble"} {
		if _, err := ttsCloud(TTSConfig{Provider: provider}, "Hallo"); err == nil {
			t.Errorf("%s accepted missing credentials", provider)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := TTSSynthesizeInMemoryContext(ctx, cloudTTSFixture(provider), "Hallo")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s ignored cancellation: %v", provider, err)
		}
	}
	cfg := cloudTTSFixture("resemble")
	cfg.Resemble.VoiceID = ""
	if _, err := ttsCloud(cfg, "Hallo"); err == nil {
		t.Fatal("Resemble accepted a missing voice UUID")
	}
}

func TestTTSCloudNeverFollowsCredentialRedirects(t *testing.T) {
	for _, tc := range []struct{ provider, host string }{
		{"xai", "api.x.ai"}, {"openai", "api.openai.com"},
		{"stepfun", "api.stepfun.com"}, {"resemble", "f.cluster.resemble.ai"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			sink := newCredentialedRedirectSink(t)
			provider := newCrossOriginRedirectProvider(t, sink.server.URL)
			routeVendorHostTo(t, &ttsHTTPClient, tc.host, provider)
			_, err := ttsCloud(cloudTTSFixture(tc.provider), "Hallo")
			assertCrossOriginRedirectRejected(t, err)
			sink.assertUntouched(t)
		})
	}
}
