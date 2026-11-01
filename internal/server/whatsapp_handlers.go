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

// whatsAppGuard is the access gate of every WhatsApp endpoint: an admin, and
// while login is disabled, a host name that cannot be a DNS-rebinding page, as
// for the backup routes. The endpoints show the linked number and drive the
// device link, so a rebinding page must not reach them.
func whatsAppGuard(s *Server, next http.Handler) http.Handler {
	return requireAdmin(s, requireRebindingSafeHostWithoutLogin(s, next))
}

// registerWhatsAppRoutes mounts the admin-only, no-store WhatsApp endpoints.
func registerWhatsAppRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/whatsapp/status", whatsAppGuard(s, handleWhatsAppStatus(s)))
	mux.Handle("/api/whatsapp/link", whatsAppGuard(s, handleWhatsAppLink(s)))
	mux.Handle("/api/whatsapp/logout", whatsAppGuard(s, handleWhatsAppLogout(s)))
	mux.Handle("/api/whatsapp/test", whatsAppGuard(s, handleWhatsAppTest(s)))
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

// whatsAppLinkBodyLimit caps the link request body.
const whatsAppLinkBodyLimit = 4 << 10

// whatsAppSameOrigin requires the Origin header on browser-driven mutations,
// even when ordinary authentication is off, like the Kaggle and Decisions POST
// handlers. A Bearer client carries no ambient credential and skips the check.
func whatsAppSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if _, bearer := bearerCredential(r.Header.Get("Authorization")); bearer || checkCSRFOriginWithPolicy(r, true) {
		return true
	}
	writeWhatsAppJSON(w, http.StatusForbidden, map[string]string{"error": "csrf_check_failed"})
	return false
}

// decodeWhatsAppLinkBody reads exactly one bounded JSON object with known
// fields. Trailing data is rejected with 400 invalid_request, an oversize body
// with 413 request_too_large. It writes the failure response itself.
func decodeWhatsAppLinkBody(w http.ResponseWriter, r *http.Request) (whatsAppLinkBody, bool) {
	var body whatsAppLinkBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, whatsAppLinkBodyLimit))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&body)
	if err == nil {
		// A second Decode must find the end of the body.
		var extra struct{}
		if err = decoder.Decode(&extra); err == io.EOF {
			return body, true
		} else if err == nil {
			err = errors.New("trailing data after the JSON object")
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeWhatsAppJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request_too_large"})
		return whatsAppLinkBody{}, false
	}
	writeWhatsAppJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	return whatsAppLinkBody{}, false
}

func handleWhatsAppLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeWhatsAppJSON(w, http.StatusOK, s.whatsApp().LinkSnapshot())
		case http.MethodPost:
			if !whatsAppSameOrigin(w, r) {
				return
			}
			body, ok := decodeWhatsAppLinkBody(w, r)
			if !ok {
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
		if !whatsAppSameOrigin(w, r) {
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
		// Before the rate limit, so a forged request cannot use up the minute.
		if !whatsAppSameOrigin(w, r) {
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
