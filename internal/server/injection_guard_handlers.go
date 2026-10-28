package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"aurago/internal/config"
)

func validateInjectionGuardActivation(s *Server, cfg *config.Config) error {
	if err := config.ValidateInjectionGuardConfig(cfg); err != nil {
		return err
	}
	previous := s.ConfigSnapshot()
	if cfg.Guardian.InjectionGuard.Enabled && (previous == nil || !previous.Guardian.InjectionGuard.Enabled) {
		if s.InjectionGuard == nil || !s.InjectionGuard.Status().Ready {
			return fmt.Errorf("injection_guard_not_ready")
		}
	}
	return nil
}

func handleInjectionGuard(s *Server, operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := http.MethodPost
		if operation == "status" {
			method = http.MethodGet
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			jsonError(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InjectionGuard == nil {
			jsonError(w, "injection_guard_unavailable", http.StatusServiceUnavailable)
			return
		}
		var err error
		switch operation {
		case "status":
		case "download":
			err = s.InjectionGuard.Prepare()
		case "cancel":
			s.InjectionGuard.CancelPrepare()
		case "test":
			err = s.InjectionGuard.SelfTest(r.Context())
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			jsonError(w, "injection_guard_action_failed", http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if operation == "download" {
			w.WriteHeader(http.StatusAccepted)
		}
		_ = json.NewEncoder(w).Encode(s.InjectionGuard.Status())
	}
}
