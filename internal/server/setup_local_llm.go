package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

type setupLocalLLMRequest struct {
	Enabled                    bool
	Role                       string
	RegularProvider            string
	AcknowledgementFingerprint string
}

type setupLocalLLMJob struct {
	ID        string    `json:"id"`
	Token     string    `json:"-"`
	State     string    `json:"state"`
	Progress  float64   `json:"progress"`
	ErrorCode string    `json:"error_code,omitempty"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"-"`
}

func parseSetupLocalLLMRequest(patch map[string]interface{}) setupLocalLLMRequest {
	raw, ok := patch["_local_llm_setup"]
	delete(patch, "_local_llm_setup")
	if !ok {
		return setupLocalLLMRequest{}
	}
	value, ok := raw.(map[string]interface{})
	if !ok {
		return setupLocalLLMRequest{}
	}
	request := setupLocalLLMRequest{
		Enabled:         value["enabled"] == true,
		Role:            strings.TrimSpace(stringValue(value["role"])),
		RegularProvider: strings.TrimSpace(stringValue(value["regular_provider"])),
		AcknowledgementFingerprint: strings.TrimSpace(
			stringValue(value["acknowledgement_fingerprint"]),
		),
	}
	if request.Role != "fallback" && request.Role != "primary" {
		request.Role = "test_only"
	}
	if request.RegularProvider == "" {
		request.RegularProvider = "main"
	}
	if request.Enabled {
		local, _ := patch["local_llm"].(map[string]interface{})
		if local == nil {
			local = make(map[string]interface{})
			patch["local_llm"] = local
		}
		local["enabled"] = true
	}
	return request
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func startSetupLocalLLMJob(s *Server, request setupLocalLLMRequest) *setupLocalLLMJob {
	id, idErr := GenerateRandomHex(16)
	token, tokenErr := GenerateRandomHex(32)
	if idErr != nil || tokenErr != nil || s.LocalLLM == nil {
		return nil
	}
	job := &setupLocalLLMJob{
		ID: id, Token: token, State: "queued", Role: request.Role, ExpiresAt: time.Now().Add(6 * time.Hour),
	}
	s.SetupLocalLLMJobsMu.Lock()
	if s.SetupLocalLLMJobs == nil {
		s.SetupLocalLLMJobs = make(map[string]*setupLocalLLMJob)
	}
	s.SetupLocalLLMJobs[id] = job
	pruneSetupLocalLLMJobsLocked(s, time.Now())
	s.SetupLocalLLMJobsMu.Unlock()

	go func() {
		updateSetupLocalLLMJob(s, id, "installing", "", 0)
		ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 6*time.Hour)
		defer cancel()
		if request.AcknowledgementFingerprint != "" {
			if err := s.LocalLLM.AcknowledgeSavedHardware(ctx, request.AcknowledgementFingerprint); err != nil {
				updateSetupLocalLLMJob(s, id, "failed", safeLocalLLMErrorCode(err), 0)
				return
			}
		}
		if err := s.LocalLLM.Install(ctx); err != nil {
			updateSetupLocalLLMJob(s, id, "failed", safeLocalLLMErrorCode(err), s.LocalLLM.Status().Progress)
			return
		}
		if request.Role != "test_only" {
			if err := activateSetupLocalLLMRole(s, request); err != nil {
				updateSetupLocalLLMJob(s, id, "failed", safeLocalLLMErrorCode(err), 1)
				return
			}
		}
		updateSetupLocalLLMJob(s, id, "completed", "", 1)
	}()
	return job
}

func localLLMLifecycleContext(s *Server) context.Context {
	if s != nil && s.localLLMLifecycleCtx != nil {
		return s.localLLMLifecycleCtx
	}
	return context.Background()
}

func activateSetupLocalLLMRole(s *Server, request setupLocalLLMRequest) error {
	status := s.LocalLLM.Status()
	if !localLLMStatusRoutingReady(status) {
		return &setupLocalLLMError{code: "local_llm_desired_state_not_verified"}
	}
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	patch := map[string]interface{}{"local_llm": map[string]interface{}{"enabled": true}}
	switch request.Role {
	case "fallback":
		patch["llm"] = map[string]interface{}{"provider": request.RegularProvider}
		patch["fallback_llm"] = map[string]interface{}{"enabled": true, "provider": config.LocalLLMProviderID}
	case "primary":
		patch["llm"] = map[string]interface{}{"provider": config.LocalLLMProviderID}
		patch["fallback_llm"] = map[string]interface{}{"enabled": true, "provider": request.RegularProvider}
	}
	reloaded, err := applyConfigPatch(s, patch)
	if err != nil {
		return &setupLocalLLMError{code: "local_llm_role_activation_failed"}
	}
	s.CfgMu.Lock()
	reloaded.Runtime = s.Cfg.Runtime
	s.replaceConfigSnapshot(reloaded)
	s.CfgMu.Unlock()
	s.LocalLLM.Configure(reloaded.LocalLLM)
	if manager, ok := s.LLMClient.(*llm.FailoverManager); ok {
		manager.Reconfigure(reloaded)
	}
	return nil
}

type setupLocalLLMError struct{ code string }

func (e *setupLocalLLMError) Error() string { return e.code }

func updateSetupLocalLLMJob(s *Server, id, state, errorCode string, progress float64) {
	s.SetupLocalLLMJobsMu.Lock()
	defer s.SetupLocalLLMJobsMu.Unlock()
	if job := s.SetupLocalLLMJobs[id]; job != nil {
		job.State = state
		job.ErrorCode = errorCode
		job.Progress = progress
		if state == "completed" || state == "failed" {
			job.ExpiresAt = time.Now().Add(15 * time.Minute)
		}
	}
}

func pruneSetupLocalLLMJobsLocked(s *Server, now time.Time) {
	for id, job := range s.SetupLocalLLMJobs {
		if job == nil || now.After(job.ExpiresAt) {
			delete(s.SetupLocalLLMJobs, id)
		}
	}
}

func handleSetupLocalLLMProbe(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !validateSetupCSRFToken(s, r.Header.Get("X-CSRF-Token"), false) {
			jsonError(w, "Invalid setup CSRF token", http.StatusForbidden)
			return
		}
		if s.LocalLLM == nil {
			jsonError(w, "Local LLM manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		var request struct {
			Backend string `json:"backend"`
		}
		if err := decodeLocalLLMBody(w, r, &request); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		profile := s.LocalLLM.ProbeBackend(ctx, request.Backend)
		writeLocalLLMJSON(w, http.StatusOK, map[string]any{
			"compatibility":            profile.Compatibility,
			"backend":                  profile.SelectedBackend,
			"warnings":                 profile.Warnings,
			"hardware_fingerprint":     profile.Fingerprint,
			"acknowledgement_required": profile.AcknowledgementDue,
			"docker_available":         profile.DockerAvailable,
		})
	}
}

func handleSetupLocalLLMJob(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		token := strings.TrimSpace(r.Header.Get("X-Setup-Job-Token"))
		s.SetupLocalLLMJobsMu.Lock()
		pruneSetupLocalLLMJobsLocked(s, time.Now())
		job := s.SetupLocalLLMJobs[id]
		if job == nil || token == "" || token != job.Token {
			s.SetupLocalLLMJobsMu.Unlock()
			jsonError(w, "Setup job not found", http.StatusNotFound)
			return
		}
		copy := *job
		if copy.State == "installing" && s.LocalLLM != nil {
			status := s.LocalLLM.Status()
			copy.Progress = status.Progress
		}
		s.SetupLocalLLMJobsMu.Unlock()
		payload, _ := json.Marshal(copy)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}
}
