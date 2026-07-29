package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/localllm"
)

func registerLocalLLMRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/local-llm/status", requireAdmin(s, handleLocalLLMStatus(s)))
	mux.Handle("/api/local-llm/probe", requireAdmin(s, handleLocalLLMProbe(s)))
	mux.Handle("/api/local-llm/install", requireAdmin(s, handleLocalLLMInstall(s)))
	mux.Handle("/api/local-llm/action", requireAdmin(s, handleLocalLLMAction(s)))
	mux.Handle("/api/local-llm/role", requireAdmin(s, handleLocalLLMRole(s)))
	mux.Handle("/api/local-llm/acknowledgement", requireAdmin(s, handleLocalLLMAcknowledgement(s)))
}

func handleLocalLLMStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LocalLLM == nil {
			jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		status := s.LocalLLM.Status()
		s.CfgMu.RLock()
		status.Role = localLLMRole(s.Cfg)
		status.ConfigRevision, _ = configFileRevision(s.Cfg.ConfigPath)
		s.CfgMu.RUnlock()
		writeLocalLLMJSON(w, http.StatusOK, status)
	}
}

func handleLocalLLMProbe(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LocalLLM == nil {
			jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		s.LocalLLM.Probe(ctx)
		statusRequest := r.Clone(r.Context())
		statusRequest.Method = http.MethodGet
		handleLocalLLMStatus(s)(w, statusRequest)
	}
}

func handleLocalLLMInstall(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LocalLLM == nil {
			jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 6*time.Hour)
			defer cancel()
			if err := s.LocalLLM.Install(ctx); err != nil {
				s.Logger.Warn("[LocalLLM] Installation failed", "code", safeLocalLLMErrorCode(err))
			}
		}()
		writeLocalLLMJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	}
}

func handleLocalLLMAction(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LocalLLM == nil {
			jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		var request struct {
			Action string `json:"action"`
		}
		if err := decodeLocalLLMBody(w, r, &request); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()
		var result any = map[string]string{"status": "ok"}
		var err error
		switch request.Action {
		case "start":
			err = s.LocalLLM.Start(ctx)
		case "stop":
			err = s.LocalLLM.Stop(ctx, false)
		case "recreate":
			err = s.LocalLLM.Recreate(ctx)
		case "smoke_test":
			err = s.LocalLLM.SmokeTest(ctx)
		case "benchmark":
			result, err = s.LocalLLM.Benchmark(ctx)
		default:
			jsonError(w, "Unknown local LLM action", http.StatusBadRequest)
			return
		}
		if err != nil {
			jsonError(w, safeLocalLLMErrorCode(err), http.StatusConflict)
			return
		}
		writeLocalLLMJSON(w, http.StatusOK, result)
	}
}

func handleLocalLLMAcknowledgement(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			Fingerprint string `json:"fingerprint"`
		}
		if err := decodeLocalLLMBody(w, r, &request); err != nil {
			return
		}
		if s.LocalLLM == nil {
			jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		// Re-probe the saved backend so a stale UI status can never persist an
		// acknowledgement for the previous hardware/backend fingerprint.
		s.LocalLLM.Probe(ctx)
		if err := s.LocalLLM.Acknowledge(request.Fingerprint); err != nil {
			jsonError(w, safeLocalLLMErrorCode(err), http.StatusConflict)
			return
		}
		writeLocalLLMJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
	}
}

func handleLocalLLMRole(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			Role            string `json:"role"`
			RegularProvider string `json:"regular_provider"`
			ConfigRevision  string `json:"config_revision"`
		}
		if err := decodeLocalLLMBody(w, r, &request); err != nil {
			return
		}
		if request.Role != "test_only" && request.Role != "fallback" && request.Role != "primary" {
			jsonError(w, "Invalid local LLM role", http.StatusBadRequest)
			return
		}

		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		currentRevision, err := configFileRevision(s.Cfg.ConfigPath)
		if err != nil {
			jsonError(w, "Failed to read configuration revision", http.StatusInternalServerError)
			return
		}
		if request.ConfigRevision == "" || request.ConfigRevision != currentRevision {
			jsonError(w, "Configuration changed; reload before changing the local LLM role", http.StatusConflict)
			return
		}

		s.CfgMu.RLock()
		current := *s.Cfg
		s.CfgMu.RUnlock()
		regular := current.FindProvider(strings.TrimSpace(request.RegularProvider))
		if regular == nil || strings.EqualFold(regular.ID, config.LocalLLMProviderID) {
			jsonError(w, "A regular provider is required", http.StatusBadRequest)
			return
		}
		regularType := strings.ToLower(strings.TrimSpace(regular.Type))
		regularLocal := regularType == "ollama" || regularType == "llamacpp" || regularType == "lmstudio"
		if strings.TrimSpace(regular.APIKey) == "" &&
			(!regularLocal || strings.TrimSpace(regular.BaseURL) == "" || strings.TrimSpace(regular.Model) == "") {
			jsonError(w, "The regular fallback provider is not fully configured", http.StatusBadRequest)
			return
		}
		if request.Role != "test_only" {
			if s.LocalLLM == nil {
				jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
				return
			}
			status := s.LocalLLM.Status()
			if !localLLMStatusRoutingReady(status) {
				jsonError(w, "local_llm_desired_state_not_verified", http.StatusConflict)
				return
			}
		}

		patch := map[string]any{"local_llm": map[string]any{"enabled": true}}
		switch request.Role {
		case "test_only":
			patch["llm"] = map[string]any{"provider": regular.ID}
			if strings.EqualFold(current.FallbackLLM.Provider, config.LocalLLMProviderID) ||
				strings.EqualFold(current.FallbackLLM.Provider, regular.ID) {
				patch["fallback_llm"] = map[string]any{"enabled": false, "provider": ""}
			}
		case "fallback":
			patch["llm"] = map[string]any{"provider": regular.ID}
			patch["fallback_llm"] = map[string]any{
				"enabled": true, "provider": config.LocalLLMProviderID,
			}
		case "primary":
			patch["llm"] = map[string]any{"provider": config.LocalLLMProviderID}
			patch["fallback_llm"] = map[string]any{
				"enabled": true, "provider": regular.ID,
			}
		}
		reloaded, err := applyConfigPatch(s, patch)
		if err != nil {
			jsonError(w, "Failed to update local LLM routing", http.StatusBadRequest)
			return
		}
		s.CfgMu.Lock()
		reloaded.Runtime = current.Runtime
		s.replaceConfigSnapshot(reloaded)
		s.CfgMu.Unlock()
		if s.LocalLLM != nil {
			s.LocalLLM.Configure(reloaded.LocalLLM)
		}
		if manager, ok := s.LLMClient.(*llm.FailoverManager); ok {
			manager.Reconfigure(reloaded)
		}
		revision, _ := configFileRevision(reloaded.ConfigPath)
		writeLocalLLMJSON(w, http.StatusOK, map[string]string{
			"status": "ok", "role": request.Role, "config_revision": revision,
		})
	}
}

func localLLMStatusRoutingReady(status localllm.Status) bool {
	offloadReady := status.Backend == "cpu" || status.GPUOffloadVerified
	return status.State == "running" &&
		!status.OperationInProgress &&
		!status.PendingRestart &&
		status.ToolCallVerified &&
		status.MemoryProfileVerified &&
		offloadReady &&
		status.DesiredFingerprint != "" &&
		status.DesiredFingerprint == status.AppliedFingerprint &&
		status.DesiredFingerprint == status.VerifiedFingerprint
}

func localLLMRole(cfg *config.Config) string {
	if cfg == nil {
		return "test_only"
	}
	if strings.EqualFold(cfg.LLM.Provider, config.LocalLLMProviderID) {
		return "primary"
	}
	if cfg.FallbackLLM.Enabled && strings.EqualFold(cfg.FallbackLLM.Provider, config.LocalLLMProviderID) {
		return "fallback"
	}
	return "test_only"
}

func configFileRevision(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func safeLocalLLMErrorCode(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if pos := strings.IndexAny(text, ": "); pos > 0 {
		text = text[:pos]
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 96 {
		return "local_llm_error"
	}
	if !strings.ContainsAny(text, "_-") {
		return "local_llm_error"
	}
	for _, char := range text {
		if !(char == '_' || char == '-' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return "local_llm_error"
		}
	}
	return text
}

func decodeLocalLLMBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return err
	}
	return nil
}

func writeLocalLLMJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
