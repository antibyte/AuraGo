package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/realtimespeech"
	"aurago/internal/speechlab"
	"aurago/internal/tools"
)

// Progress audio uses the active session and a server-owned phrase. Client text
// cannot turn this endpoint into an arbitrary speech synthesis service.
func handleRealtimeSpeechProgressAudio(s *Server, registry *realtimespeech.Registry) http.HandlerFunc {
	limit := make(chan struct{}, 2)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		var body struct {
			SessionID string `json:"session_id"`
			ClientID  string `json:"client_id"`
			RequestID string `json:"request_id"`
			Kind      string `json:"kind"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		clientID, err := realtimeSpeechClientID(r, body.ClientID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, ok := registry.Get(strings.TrimSpace(body.SessionID), clientID)
		if !ok {
			jsonError(w, "Realtime speech session not found", http.StatusNotFound)
			return
		}
		chatSessionID, active := registry.ActionSession(strings.TrimSpace(body.RequestID), clientID)
		if !active || chatSessionID != session.ChatSessionID {
			jsonError(w, "Realtime speech action not found", http.StatusNotFound)
			return
		}
		key := "backend.workflow_feedback"
		switch body.Kind {
		case "ack":
		case "wait":
			key = "backend.workflow_wait"
		default:
			jsonError(w, "Invalid progress kind", http.StatusBadRequest)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Speech output unavailable", http.StatusServiceUnavailable)
			return
		}
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		default:
			jsonError(w, "Speech output busy", http.StatusTooManyRequests)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		tts := buildChatVoiceOutputTTSConfig(cfg, cfg.Server.UILanguage, s.SpeechLab)
		if session.Provider == realtimespeech.ProviderSpeechLab {
			tts.Provider = "speech_lab"
		} else if !chatVoiceOutputTTSConfigured(cfg) {
			jsonError(w, "Speech output unavailable", http.StatusServiceUnavailable)
			return
		}
		if isSpeechLabTTSProvider(tts.Provider) {
			client := tts.SpeechLab.Client
			if client == nil {
				client, err = speechlab.NewClient(cfg.SpeechLab)
				if err != nil {
					jsonError(w, "Speech Lab unavailable", http.StatusServiceUnavailable)
					return
				}
			}
			ready, readyErr := client.Require(ctx, false, true)
			if readyErr != nil {
				jsonError(w, "Speech Lab unavailable", http.StatusServiceUnavailable)
				return
			}
			tts.SpeechLab.Client, tts.SpeechLab.ExpectedTTSID, tts.SpeechLab.Voice = client, ready.TTSID, ready.Voice
		}
		data, ext, err := tools.TTSSynthesizeInMemoryContext(ctx, tts, i18n.T(cfg.Server.UILanguage, key))
		if ctx.Err() != nil {
			return
		}
		if err != nil || len(data) > 8<<20 {
			jsonError(w, "Speech output unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", chatVoiceAudioMIMEType("progress."+strings.TrimPrefix(ext, ".")))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
	}
}
