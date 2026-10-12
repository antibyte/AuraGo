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
	for _, action := range []string{"status", "install", "start", "stop", "test", "evaluate"} {
		mux.Handle("/api/decisions/"+action, requireAdmin(s, handleDecisions(s, action)))
	}
}

func handleDecisions(s *Server, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := http.MethodPost
		if action == "status" {
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
		if action == "status" {
			writeDecisionsJSON(w, http.StatusOK, s.Decisions.Status())
			return
		}
		if action == "install" || action == "start" {
			if cfg := s.ConfigSnapshot(); cfg == nil || !cfg.Decisions.Enabled {
				writeDecisionsError(w, "service_disabled", http.StatusConflict)
				return
			}
			// Installation and model loading outlive the HTTP request, but never the server.
			go func() {
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
			}()
			writeDecisionsJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		if action == "stop" {
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
	if errors.Is(err, context.Canceled) {
		return "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout"
	}
	return decisions.ErrorCode(err)
}

func writeDecisionsFailure(w http.ResponseWriter, err error) {
	code := decisionsFailureCode(err)
	status := http.StatusServiceUnavailable
	if code == decisions.CodeInvalidRequest || code == "invalid_image" || code == decisions.CodeInputTokenBudgetExceeded || code == decisions.CodeCandidateNotScorable {
		status = http.StatusBadRequest
	}
	if code == decisions.CodeInvalidRuntimeResponse || code == decisions.CodeInferenceFailed {
		status = http.StatusBadGateway
	}
	if code == "queue_full" {
		status = http.StatusTooManyRequests
	}
	if code == "request_timeout" {
		status = http.StatusGatewayTimeout
	}
	if code == "request_cancelled" {
		status = http.StatusRequestTimeout
	}
	writeDecisionsError(w, code, status)
}
