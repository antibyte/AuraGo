package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"aurago/internal/decisions"
)

type decisionsAPIRequest struct {
	State     any                  `json:"state"`
	Questions []decisions.Question `json:"questions"`
	Images    []string             `json:"images,omitempty"`
}

func registerDecisionsRoutes(mux *http.ServeMux, s *Server) {
	for _, action := range []string{"status", "models", "check", "setup", "install", "start", "stop", "test", "evaluate"} {
		mux.Handle("/api/decisions/"+action, requireAdmin(s, handleDecisions(s, action)))
	}
}

func handleDecisions(s *Server, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := http.MethodPost
		if action == "status" || action == "models" {
			method = http.MethodGet
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if method == http.MethodPost {
			_, bearer := bearerCredential(r.Header.Get("Authorization"))
			if !bearer && !checkCSRFOriginWithPolicy(r, true) {
				jsonError(w, "Origin denied", http.StatusForbidden)
				return
			}
		}
		if s.Decisions == nil {
			writeDecisionsError(w, "service_not_ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if action == "setup" {
			s.handleDecisionsSetup(w, r)
			return
		}
		if action == "check" {
			if cfg := s.ConfigSnapshot(); cfg != nil && cfg.Decisions.Backend == "provider" {
				writeDecisionsError(w, "provider_lifecycle_unavailable", http.StatusConflict)
				return
			}
			service := s.Decisions
			if routed, ok := service.(*routedDecisions); ok {
				service = routed.local
			}
			checker, ok := service.(interface {
				CheckInstallation(context.Context) decisions.InstallationCheck
			})
			if !ok {
				writeDecisionsError(w, decisions.CodeRuntimeUnavailable, http.StatusServiceUnavailable)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			writeDecisionsJSON(w, http.StatusOK, checker.CheckInstallation(ctx))
			return
		}
		if action == "models" {
			router, ok := s.Decisions.(*routedDecisions)
			if !ok {
				writeDecisionsError(w, "provider_not_configured", http.StatusConflict)
				return
			}
			models, err := router.Models(r.Context(), r.URL.Query().Get("provider"))
			if err != nil {
				writeDecisionsFailure(w, err)
				return
			}
			writeDecisionsJSON(w, http.StatusOK, map[string]any{"models": models})
			return
		}
		if action == "status" {
			writeDecisionsJSON(w, http.StatusOK, s.decisionsSetupStatus())
			return
		}
		if action == "install" || action == "start" {
			if cfg := s.ConfigSnapshot(); cfg != nil && cfg.Decisions.Backend == "provider" {
				writeDecisionsError(w, "provider_lifecycle_unavailable", http.StatusConflict)
				return
			}
			if cfg := s.ConfigSnapshot(); cfg == nil || !cfg.Decisions.Enabled {
				writeDecisionsError(w, "service_disabled", http.StatusConflict)
				return
			}
			// One HTTP install or start is admitted. A second request must not
			// treat the first call's transitional status as its own acceptance.
			if !s.decisionsHTTPAdmission.TryLock() {
				writeDecisionsError(w, decisions.CodeLifecycleBusy, http.StatusConflict)
				return
			}
			s.cancelDecisionsSetup()
			// Installation and model loading outlive the HTTP request, but never the server.
			// Answer only after a transitional state is visible, or with the
			// operation's own error if that arrives first.
			errCh := make(chan error, 1)
			go func() {
				defer s.decisionsHTTPAdmission.Unlock()
				ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 6*time.Hour)
				defer cancel()
				var err error
				if action == "install" {
					err = s.Decisions.Install(ctx)
				} else {
					err = s.Decisions.Start(ctx)
				}
				if err != nil && s.Logger != nil {
					s.Logger.Warn("Decisions action failed", "action", action, "code", decisions.ErrorCode(err))
				}
				errCh <- err
			}()
			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			deadline := time.NewTimer(2 * time.Second)
			defer deadline.Stop()
			for {
				select {
				case err := <-errCh:
					writeDecisionsLifecycleAccept(w, err)
					return
				case <-deadline.C:
					select {
					case err := <-errCh:
						writeDecisionsLifecycleAccept(w, err)
						return
					default:
					}
					writeDecisionsJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
					return
				case <-ticker.C:
					select {
					case err := <-errCh:
						writeDecisionsLifecycleAccept(w, err)
						return
					default:
					}
					switch s.Decisions.Status().State {
					case "installing", "downloading", "starting", "discovering_devices", "stopping":
						writeDecisionsJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
						return
					}
				}
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		if action == "stop" {
			s.cancelDecisionsSetup()
			if err := s.Decisions.Stop(ctx); err != nil {
				writeDecisionsFailure(w, err)
				return
			}
			writeDecisionsJSON(w, http.StatusOK, s.Decisions.Status())
			return
		}
		var request decisionsAPIRequest
		if action == "test" {
			request = decisionsAPIRequest{State: "The light is on.", Questions: []decisions.Question{{ID: "light", Type: "noul", Instructions: "Is the light on?"}}}
		} else {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				writeDecisionsError(w, "invalid_request", http.StatusBadRequest)
				return
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				writeDecisionsError(w, "invalid_request", http.StatusBadRequest)
				return
			}
		}
		response, err := s.evaluateDecisions(ctx, request)
		if err != nil {
			writeDecisionsFailure(w, err)
			return
		}
		writeDecisionsJSON(w, http.StatusOK, response)
	}
}

func writeDecisionsLifecycleAccept(w http.ResponseWriter, err error) {
	if err != nil {
		writeDecisionsFailure(w, err)
		return
	}
	writeDecisionsJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func writeDecisionsJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		jsonError(w, "Invalid decisions response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeDecisionsError(w http.ResponseWriter, code string, status int) {
	writeDecisionsJSON(w, status, map[string]string{"code": code, "error": code})
}

func decisionsFailureCode(err error) string {
	if errors.Is(err, errDecisionImage) {
		return "invalid_image"
	}
	code := decisions.ErrorCode(err)
	if code == decisions.CodeExternalOutcomeUnknown || code == "configuration_changed" || code == decisions.CodeRuntimeStartTimeout || code == decisions.CodeRuntimeDiscoveryTimeout || code == decisions.CodeRuntimeScoreTimeout {
		return code
	}
	if errors.Is(err, context.Canceled) {
		return "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout"
	}
	return code
}

func writeDecisionsFailure(w http.ResponseWriter, err error) {
	code := decisionsFailureCode(err)
	status := http.StatusServiceUnavailable
	if code == decisions.CodeInvalidRequest || code == "invalid_image" || code == "image_invalid" || code == "image_limit_exceeded" || code == "image_minimum_unmet" || code == decisions.CodeInputTokenBudgetExceeded || code == decisions.CodeCandidateNotScorable {
		status = http.StatusBadRequest
	}
	if code == decisions.CodeInvalidRuntimeResponse || code == decisions.CodeInferenceFailed {
		status = http.StatusBadGateway
	}
	if code == "queue_full" {
		status = http.StatusTooManyRequests
	}
	if code == decisions.CodeProviderRateLimited {
		status = http.StatusTooManyRequests
	}
	if code == decisions.CodeProviderCreditExhausted {
		status = http.StatusPaymentRequired
	}
	if code == decisions.CodeProviderAuthFailed {
		status = http.StatusBadGateway
	}
	if code == "request_timeout" || code == decisions.CodeRuntimeStartTimeout || code == decisions.CodeRuntimeDiscoveryTimeout || code == decisions.CodeRuntimeScoreTimeout {
		status = http.StatusGatewayTimeout
	}
	if code == decisions.CodeExternalOutcomeUnknown || code == decisions.CodeExternalProviderFailed {
		status = http.StatusBadGateway
	}
	if code == "request_cancelled" {
		status = http.StatusRequestTimeout
	}
	if code == "budget_exceeded" {
		status = http.StatusPaymentRequired
	}
	if code == "provider_lifecycle_unavailable" || code == "configuration_changed" || code == decisions.CodeLifecycleBusy || code == decisions.CodeDockerAPITooOld {
		status = http.StatusConflict
	}
	writeDecisionsError(w, code, status)
}
