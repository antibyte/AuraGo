package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"aurago/internal/integrationstatus"
	"aurago/internal/security"
	"aurago/internal/whatsapp"
)

var (
	whatsAppTestMu   sync.Mutex
	whatsAppTestLast time.Time
	whatsAppTestNow  = time.Now
)

// whatsAppActionTimeout bounds store access and a remote unlink per request.
// The link session itself outlives the request.
const whatsAppActionTimeout = 30 * time.Second

// registerWhatsAppRoutes mounts the admin-only, no-store WhatsApp endpoints.
func registerWhatsAppRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/whatsapp/status", requireAdmin(s, handleWhatsAppStatus(s)))
	mux.Handle("/api/whatsapp/link", requireAdmin(s, handleWhatsAppLink(s)))
	mux.Handle("/api/whatsapp/logout", requireAdmin(s, handleWhatsAppLogout(s)))
	mux.Handle("/api/whatsapp/test", requireAdmin(s, handleWhatsAppTest(s)))
}

func writeWhatsAppJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// whatsAppErrorStatus maps runtime errors to an HTTP status and a fixed code.
// Raw errors may carry JIDs, so they never reach the response; the log gets
// them only through maskedWhatsAppError.
func whatsAppErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, whatsapp.ErrInvalidLinkMethod):
		return http.StatusBadRequest, "invalid_method"
	case errors.Is(err, whatsapp.ErrInvalidPhone):
		return http.StatusBadRequest, "invalid_phone"
	case errors.Is(err, whatsapp.ErrNotConfigured):
		return http.StatusConflict, "not_configured"
	case errors.Is(err, whatsapp.ErrAlreadyLinked):
		return http.StatusConflict, "already_linked"
	case errors.Is(err, whatsapp.ErrLinkInProgress):
		return http.StatusConflict, "link_in_progress"
	case errors.Is(err, whatsapp.ErrNotConnected), errors.Is(err, whatsapp.ErrNoRecipient):
		return http.StatusConflict, "not_connected"
	}
	return http.StatusInternalServerError, "whatsapp_failed"
}

// whatsAppDigitRun matches phone-number-like digit runs inside error text.
var whatsAppDigitRun = regexp.MustCompile(`[0-9]{5,}`)

// maskedWhatsAppError is the only form of a runtime error that may be logged:
// scrubbed of secrets, with every long digit run cut to its last four digits.
func maskedWhatsAppError(err error) string {
	return whatsAppDigitRun.ReplaceAllStringFunc(security.Scrub(err.Error()), func(run string) string {
		return "***" + run[len(run)-4:]
	})
}

func writeWhatsAppFailure(s *Server, w http.ResponseWriter, action string, err error) {
	code, reason := whatsAppErrorStatus(err)
	if code >= http.StatusInternalServerError && s.Logger != nil {
		s.Logger.Warn("WhatsApp request failed", "action", action, "error_code", reason, "error", maskedWhatsAppError(err))
	}
	writeWhatsAppJSON(w, code, map[string]string{"error": reason})
}

func whatsAppMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeWhatsAppJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
}

func handleWhatsAppStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			whatsAppMethodNotAllowed(w, http.MethodGet)
			return
		}
		status := integrationstatus.WhatsAppStatus()
		label := "ok"
		if status.State != integrationstatus.WhatsAppStateHealthy {
			label = status.State
		}
		// Always 200: the Config UI shows every state itself.
		writeWhatsAppJSON(w, http.StatusOK, map[string]any{"status": label, "whatsapp": status})
	}
}

type whatsAppLinkBody struct {
	Method string `json:"method"`
	Phone  string `json:"phone"`
	Force  bool   `json:"force"`
}

func handleWhatsAppLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeWhatsAppJSON(w, http.StatusOK, s.whatsApp().LinkSnapshot())
		case http.MethodPost:
			var body whatsAppLinkBody
			decoder := json.NewDecoder(io.LimitReader(r.Body, 4<<10))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&body); err != nil {
				writeWhatsAppJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), whatsAppActionTimeout)
			defer cancel()
			snapshot, err := s.whatsApp().Link(ctx, whatsapp.LinkRequest{Method: body.Method, Phone: body.Phone}, body.Force)
			if err != nil {
				writeWhatsAppFailure(s, w, "link", err)
				return
			}
			writeWhatsAppJSON(w, http.StatusAccepted, snapshot)
		default:
			whatsAppMethodNotAllowed(w, "GET, POST")
		}
	}
}

func handleWhatsAppLogout(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			whatsAppMethodNotAllowed(w, http.MethodPost)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), whatsAppActionTimeout)
		defer cancel()
		result, err := s.whatsApp().Logout(ctx)
		if err != nil {
			writeWhatsAppFailure(s, w, "logout", err)
			return
		}
		writeWhatsAppJSON(w, http.StatusOK, map[string]any{"status": "unlinked", "remote_removed": result.RemoteRemoved})
	}
}

// handleWhatsAppTest sends the owner-only test message, at most once per
// minute (like /api/telegram/test).
func handleWhatsAppTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			whatsAppMethodNotAllowed(w, http.MethodPost)
			return
		}
		now := whatsAppTestNow()
		whatsAppTestMu.Lock()
		if !whatsAppTestLast.IsZero() && now.Sub(whatsAppTestLast) < time.Minute {
			whatsAppTestMu.Unlock()
			writeWhatsAppJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		whatsAppTestLast = now
		whatsAppTestMu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), whatsAppActionTimeout)
		defer cancel()
		if err := s.whatsApp().SendTest(ctx, now); err != nil {
			writeWhatsAppFailure(s, w, "test", err)
			return
		}
		writeWhatsAppJSON(w, http.StatusOK, map[string]any{"status": "sent", "sent_at": now.UTC().Format(time.RFC3339)})
	}
}
