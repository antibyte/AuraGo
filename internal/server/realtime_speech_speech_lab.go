package server

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/realtimespeech"
	"aurago/internal/speechlab"
)

func realtimeSpeechProfileReadyWithoutKey(profile config.RealtimeSpeechProfile) bool {
	return strings.EqualFold(strings.TrimSpace(profile.Provider), realtimespeech.ProviderSpeechLab)
}

func requireRealtimeSpeechLab(s *Server, ctx context.Context, needASR, needTTS bool) (speechlab.Ready, error) {
	if s == nil || s.SpeechLab == nil {
		return speechlab.Ready{}, &speechlab.NotReadyError{}
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.SpeechLab.Active() {
		return speechlab.Ready{}, &speechlab.NotReadyError{}
	}
	return s.SpeechLab.Require(ctx, needASR, needTTS)
}

func realtimeSpeechLabSession(registry *realtimespeech.Registry, sessionID, clientID string) (realtimespeech.Session, bool) {
	session, ok := registry.Get(strings.TrimSpace(sessionID), clientID)
	if !ok || !strings.EqualFold(session.Provider, realtimespeech.ProviderSpeechLab) {
		return realtimespeech.Session{}, false
	}
	return session, true
}

func handleRealtimeSpeechLabTranscribe(s *Server, registry *realtimespeech.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		clientID, err := realtimeSpeechClientID(r, r.Header.Get("X-Realtime-Speech-Client-ID"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID == "" {
			sessionID = strings.TrimSpace(r.Header.Get("X-Realtime-Speech-Session-ID"))
		}
		if _, ok := realtimeSpeechLabSession(registry, sessionID, clientID); !ok {
			jsonError(w, "Realtime speech session not found", http.StatusNotFound)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, speechlab.MaxASRBytes+64*1024)
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			jsonError(w, "Expected a multipart WAV upload", http.StatusBadRequest)
			return
		}
		reader, err := r.MultipartReader()
		if err != nil {
			jsonError(w, "Invalid multipart upload", http.StatusBadRequest)
			return
		}
		var wav []byte
		for {
			part, nextErr := reader.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				jsonError(w, "Invalid multipart upload", http.StatusBadRequest)
				return
			}
			if part.FormName() != "audio" {
				_ = part.Close()
				continue
			}
			if !isWAVMediaType(part.Header.Get("Content-Type")) {
				_ = part.Close()
				jsonError(w, "Speech Lab live speech requires WAV audio", http.StatusBadRequest)
				return
			}
			wav, err = io.ReadAll(io.LimitReader(part, speechlab.MaxASRBytes+1))
			_ = part.Close()
			if err != nil || len(wav) > speechlab.MaxASRBytes {
				jsonError(w, "Speech Lab audio exceeds 8 MiB", http.StatusRequestEntityTooLarge)
				return
			}
			break
		}
		metrics, err := speechlab.AnalyzePCM16WAV(wav)
		if err != nil {
			jsonError(w, "Speech Lab live speech requires mono PCM16 WAV at 16 kHz", http.StatusBadRequest)
			return
		}
		if metrics.Duration > 120*time.Second {
			jsonError(w, "Speech Lab live speech is limited to 120 seconds", http.StatusRequestEntityTooLarge)
			return
		}
		ready, err := requireRealtimeSpeechLab(s, r.Context(), true, false)
		if err != nil {
			writeSpeechLabError(w, err)
			return
		}
		result, err := s.SpeechLab.Transcribe(r.Context(), wav, s.ConfigSnapshot().SpeechLab.Language, ready.ASRID)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Info("Speech Lab live-speech transcription",
					"error", err,
					"error_code", speechlab.ErrorCode(err),
					"asr_id", ready.ASRID,
					"audio_duration_ms", metrics.Duration.Milliseconds(),
					"audio_sample_count", metrics.SampleCount,
					"audio_peak_level", metrics.PeakLevel,
					"audio_rms_level", metrics.RMSLevel,
				)
			}
			writeSpeechLabError(w, err)
			return
		}
		writeSpeechLabJSON(w, http.StatusOK, map[string]any{
			"transcription": result.Text,
			"asr_id":        result.ASRID,
		})
	}
}

func handleRealtimeSpeechLabSynthesize(s *Server, registry *realtimespeech.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
		var body struct {
			SessionID string `json:"session_id"`
			ClientID  string `json:"client_id"`
			Text      string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		clientID, err := realtimeSpeechClientID(r, body.ClientID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, ok := realtimeSpeechLabSession(registry, body.SessionID, clientID); !ok {
			jsonError(w, "Realtime speech session not found", http.StatusNotFound)
			return
		}
		text := strings.TrimSpace(body.Text)
		if text == "" {
			jsonError(w, "Text is required", http.StatusBadRequest)
			return
		}
		ready, err := requireRealtimeSpeechLab(s, r.Context(), false, true)
		if err != nil {
			writeSpeechLabError(w, err)
			return
		}
		wav, _, _, err := s.SpeechLab.Synthesize(r.Context(), text, s.ConfigSnapshot().SpeechLab.Language, ready.Voice, ready.TTSID, ready.Voice)
		if err != nil {
			writeSpeechLabError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "audio/wav")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(wav)
	}
}
