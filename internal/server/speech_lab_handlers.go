package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"aurago/internal/sipphone"
	"aurago/internal/speechlab"
	"aurago/internal/speechlab/deployer"
)

type speechLabStatusResponse struct {
	Enabled            bool                 `json:"enabled"`
	Reachable          bool                 `json:"reachable"`
	Ready              bool                 `json:"ready"`
	ASRID              string               `json:"asr_id,omitempty"`
	TTSID              string               `json:"tts_id,omitempty"`
	ASROK              bool                 `json:"asr_ok"`
	TTSOK              bool                 `json:"tts_ok"`
	Message            string               `json:"message,omitempty"`
	Language           string               `json:"language,omitempty"`
	Voice              string               `json:"voice,omitempty"`
	SIPEnabled         bool                 `json:"sip_enabled"`
	ChatInputEnabled   bool                 `json:"chat_input_enabled"`
	ChatOutputEnabled  bool                 `json:"chat_output_enabled"`
	AdvancedUIURL      string               `json:"advanced_ui_url,omitempty"`
	EnvironmentManaged bool                 `json:"environment_managed"`
	ActiveOperations   int64                `json:"active_operations"`
	Deployment         deployer.PublicState `json:"deployment"`
	Warnings           []string             `json:"warnings"`
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
	mux.Handle("/api/speech-lab/deployment/install", requireAdmin(s, handleSpeechLabDeployment(s, "install")))
	mux.Handle("/api/speech-lab/deployment/start", requireAdmin(s, handleSpeechLabDeployment(s, "start")))
	mux.Handle("/api/speech-lab/deployment/stop", requireAdmin(s, handleSpeechLabDeployment(s, "stop")))
	mux.Handle("/api/speech-lab/deployment/update", requireAdmin(s, handleSpeechLabDeployment(s, "update")))
	mux.Handle("/api/speech-lab/deployment", requireAdmin(s, handleSpeechLabDeployment(s, "remove")))
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
			Warnings:           []string{},
		}
		if result.AdvancedUIURL == "" {
			result.Warnings = append(result.Warnings, "advanced_ui_url_missing")
		}
		if s.SpeechLabDeployer != nil {
			result.Deployment = s.SpeechLabDeployer.PublicStatus()
		} else {
			result.Deployment = deployer.PublicState{Mode: cfg.SpeechLab.Deployment.Mode, Managed: cfg.SpeechLab.Deployment.Mode == "managed", State: "disabled", RequestedBundle: cfg.SpeechLab.Deployment.Bundle}
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

func handleSpeechLabDeployment(s *Server, action string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if action == "remove" && r.Method != http.MethodDelete || action != "remove" && r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SpeechLabDeployer == nil {
			writeSpeechLabJSON(w, http.StatusConflict, map[string]string{"error": "speech_lab_bundle_unavailable", "message": "Speech Lab is configured as an external stack"})
			return
		}
		var release func()
		if s.SIPPhone != nil {
			var reserveErr error
			release, reserveErr = s.SIPPhone.ReserveSpeechLabStackChange()
			if reserveErr != nil {
				if errors.Is(reserveErr, sipphone.ErrBusy) {
					writeSpeechLabJSON(w, http.StatusConflict, map[string]string{"error": "speech_lab_busy", "message": "Speech Lab deployment is blocked while a SIP call or stack operation is active"})
					return
				}
				writeSpeechLabJSON(w, http.StatusConflict, map[string]string{"error": "speech_lab_busy", "message": "Speech Lab deployment reservation failed"})
				return
			}
			defer release()
		}
		if action == "install" || action == "update" || action == "remove" {
			var request struct {
				Confirm bool `json:"confirm"`
			}
			if r.Body != nil {
				decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				_ = decoder.Decode(&request)
			}
			if !request.Confirm {
				writeSpeechLabJSON(w, http.StatusBadRequest, map[string]string{"error": "confirmation_required", "message": "Administrator confirmation is required"})
				return
			}
		}
		var err error
		switch action {
		case "install":
			err = s.SpeechLabDeployer.Install(r.Context())
		case "update":
			err = s.SpeechLabDeployer.Update(r.Context())
		case "start":
			err = s.SpeechLabDeployer.Start(r.Context())
		case "stop":
			err = s.SpeechLabDeployer.Stop(r.Context())
		case "remove":
			err = s.SpeechLabDeployer.Remove(r.Context())
		default:
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			status := http.StatusBadGateway
			if code := deployer.Code(err); code == "speech_lab_deployment_busy" {
				status = http.StatusConflict
			}
			if code := deployer.Code(err); code == "speech_lab_docker_unavailable" {
				status = http.StatusServiceUnavailable
			}
			writeSpeechLabJSON(w, status, map[string]any{"error": deployer.Code(err), "message": "Speech Lab deployment failed", "deployment": s.SpeechLabDeployer.PublicStatus()})
			return
		}
		writeSpeechLabJSON(w, http.StatusOK, map[string]any{"ok": true, "deployment": s.SpeechLabDeployer.PublicStatus()})
	})
}

// speechLabBrowserURLForRequest returns only the explicit browser-facing lab
// address. Request hosts are not configuration and may be reverse-proxy or
// attacker controlled, so the status endpoint never synthesizes a URL.
func speechLabBrowserURLForRequest(configured string, _ *http.Request) string {
	return strings.TrimRight(strings.TrimSpace(configured), "/")
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
	var deploymentErr *deployer.Error
	if errors.As(err, &deploymentErr) {
		status := http.StatusBadGateway
		if deploymentErr.Code == "speech_lab_docker_unavailable" || deploymentErr.Code == "speech_lab_not_ready" {
			status = http.StatusServiceUnavailable
		}
		writeSpeechLabJSON(w, status, map[string]string{"error": deploymentErr.Code, "message": "Speech Lab deployment failed"})
		return
	}
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
