package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"

	"aurago/internal/sipphone"
	"aurago/internal/speechlab"
)

const defaultSpeechLabBrowserPort = "8766"

type speechLabStatusResponse struct {
	Enabled            bool   `json:"enabled"`
	Reachable          bool   `json:"reachable"`
	Ready              bool   `json:"ready"`
	ASRID              string `json:"asr_id,omitempty"`
	TTSID              string `json:"tts_id,omitempty"`
	ASROK              bool   `json:"asr_ok"`
	TTSOK              bool   `json:"tts_ok"`
	Message            string `json:"message,omitempty"`
	Language           string `json:"language,omitempty"`
	Voice              string `json:"voice,omitempty"`
	SIPEnabled         bool   `json:"sip_enabled"`
	ChatInputEnabled   bool   `json:"chat_input_enabled"`
	ChatOutputEnabled  bool   `json:"chat_output_enabled"`
	AdvancedUIURL      string `json:"advanced_ui_url,omitempty"`
	EnvironmentManaged bool   `json:"environment_managed"`
	ActiveOperations   int64  `json:"active_operations"`
}

func registerSpeechLabRoutes(mux *http.ServeMux, s *Server) {
	if mux == nil || s == nil {
		return
	}
	mux.HandleFunc("/api/speech-lab/status", handleSpeechLabStatus(s))
	mux.Handle("/api/speech-lab/capability", requireAdmin(s, handleSpeechLabRaw(s, "capability")))
	mux.Handle("/api/speech-lab/catalog", requireAdmin(s, handleSpeechLabRaw(s, "catalog")))
	mux.Handle("/api/speech-lab/suggestions", requireAdmin(s, handleSpeechLabRaw(s, "suggestions")))
	mux.Handle("/api/speech-lab/stack", requireAdmin(s, handleSpeechLabStack(s)))
}

func handleSpeechLabStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Runtime configuration is unavailable", http.StatusServiceUnavailable)
			return
		}
		result := speechLabStatusResponse{
			Enabled: cfg.SpeechLab.Enabled, Language: cfg.SpeechLab.Language,
			SIPEnabled: cfg.SpeechLab.SIPEnabled, ChatInputEnabled: cfg.SpeechLab.ChatInputEnabled,
			ChatOutputEnabled:  cfg.SpeechLab.ChatOutputEnabled,
			AdvancedUIURL:      speechLabBrowserURLForRequest(cfg.SpeechLab.AdvancedUIURL, r),
			EnvironmentManaged: strings.TrimSpace(os.Getenv("AURAGO_SPEECH_LAB_BASE_URL")) != "",
		}
		if s.SpeechLab != nil {
			result.ActiveOperations = s.SpeechLab.ActiveOperations()
		}
		if !cfg.SpeechLab.Active() || s.SpeechLab == nil {
			result.Message = "Speech Lab is disabled"
			writeSpeechLabJSON(w, http.StatusOK, result)
			return
		}
		ready, err := s.SpeechLab.Ready(r.Context())
		if err != nil {
			result.Message = "Speech Lab is unreachable"
			writeSpeechLabJSON(w, http.StatusOK, result)
			return
		}
		result.Reachable = true
		result.Ready = ready.Ready
		result.ASRID, result.TTSID = ready.ASRID, ready.TTSID
		result.Voice = ready.Voice
		result.ASROK, result.TTSOK = ready.ASROK, ready.TTSOK
		if ready.Ready {
			result.Message = "Speech Lab is ready"
		} else {
			result.Message = "Speech Lab is not ready"
		}
		writeSpeechLabJSON(w, http.StatusOK, result)
	}
}

// speechLabBrowserURLForRequest resolves the browser-facing lab address. The
// gateway URL cannot be reused because it normally contains a Docker-only host.
// Standard deployments publish the lab UI on port 8766 of the same host that
// serves AuraGo. AdvancedUIURL remains an expert override for reverse proxies
// and non-standard port mappings.
func speechLabBrowserURLForRequest(configured string, r *http.Request) string {
	if configured = strings.TrimRight(strings.TrimSpace(configured), "/"); configured != "" {
		return configured
	}
	host := effectiveRequestHost(r)
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\?#@ \t\r\n") {
		return ""
	}
	return "http://" + net.JoinHostPort(host, defaultSpeechLabBrowserPort) + "/"
}

func handleSpeechLabRaw(s *Server, resource string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SpeechLab == nil {
			jsonError(w, "Speech Lab is unavailable", http.StatusServiceUnavailable)
			return
		}
		var raw json.RawMessage
		var err error
		switch resource {
		case "capability":
			raw, err = s.SpeechLab.Capability(r.Context())
		case "catalog":
			raw, _, err = s.SpeechLab.Catalog(r.Context())
		case "suggestions":
			raw, err = s.SpeechLab.Suggestions(r.Context(), r.URL.Query())
		default:
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeSpeechLabError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})
}

func handleSpeechLabStack(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SpeechLab == nil {
			jsonError(w, "Speech Lab is unavailable", http.StatusServiceUnavailable)
			return
		}
		var release func()
		if s.SIPPhone != nil {
			var err error
			release, err = s.SIPPhone.ReserveSpeechLabStackChange()
			if err != nil {
				if errors.Is(err, sipphone.ErrBusy) {
					jsonError(w, "Speech Lab stack cannot change while SIP is busy", http.StatusConflict)
					return
				}
				jsonError(w, "Speech Lab stack reservation failed", http.StatusConflict)
				return
			}
			defer release()
		}
		var request speechlab.StackRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			jsonError(w, "Invalid Speech Lab stack request", http.StatusBadRequest)
			return
		}
		result, ready, err := s.SpeechLab.ActivateStack(r.Context(), request)
		if err != nil {
			if strings.Contains(err.Error(), "busy") {
				jsonError(w, "Speech Lab stack is busy", http.StatusConflict)
				return
			}
			writeSpeechLabError(w, err)
			return
		}
		writeSpeechLabJSON(w, http.StatusOK, map[string]any{"ok": result.OK, "message": result.Message, "ready": ready})
	})
}

func writeSpeechLabError(w http.ResponseWriter, err error) {
	var notReady *speechlab.NotReadyError
	if errors.As(err, &notReady) {
		writeSpeechLabJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": speechlab.ErrorCode(err), "message": notReady.Error(), "ready": notReady.Status,
		})
		return
	}
	writeSpeechLabJSON(w, http.StatusBadGateway, map[string]string{
		"error": speechlab.ErrorCode(err), "message": "Speech Lab request failed",
	})
}

func writeSpeechLabJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
