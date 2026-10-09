package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/localwiki"
)

// registerLocalWikipediaRoutes mounts the admin API of the config section.
// The desktop API (/api/desktop/local-wikipedia/) is separate.
func registerLocalWikipediaRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/local-wikipedia/catalog", requireAdmin(s, handleLocalWikipediaCatalog(s)))
	mux.Handle("/api/local-wikipedia/status", requireAdmin(s, handleLocalWikipediaStatus(s)))
	mux.Handle("/api/local-wikipedia/install", requireAdmin(s, handleLocalWikipediaInstall(s)))
	mux.Handle("/api/local-wikipedia/cancel", requireAdmin(s, handleLocalWikipediaCancel(s)))
	mux.Handle("/api/local-wikipedia/delete", requireAdmin(s, handleLocalWikipediaDelete(s)))
	mux.Handle("/api/local-wikipedia/check-update", requireAdmin(s, handleLocalWikipediaCheckUpdate(s)))
}

// localWikipediaManager returns the manager, or nil. Requests only read it:
// every published config snapshot configures it (replaceConfigSnapshot calls
// syncLocalWikipediaSettings), including changes made outside the Local
// Wikipedia section such as the system language in the setup wizard.
func (s *Server) localWikipediaManager() *localwiki.Manager {
	if s == nil {
		return nil
	}
	return s.LocalWiki
}

// localWikipediaHandler enforces the method, same-origin mutations without a
// Bearer token, and the presence of the manager.
func localWikipediaHandler(s *Server, method string, serve func(http.ResponseWriter, *http.Request, *localwiki.Manager)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if method != http.MethodGet {
			if _, isBearer := bearerCredential(r.Header.Get("Authorization")); !isBearer && !checkCSRFOrigin(r) {
				jsonError(w, "csrf_check_failed", http.StatusForbidden)
				return
			}
		}
		manager := s.localWikipediaManager()
		if manager == nil {
			writeLocalWikipediaError(w, http.StatusServiceUnavailable, "localwiki_unavailable", nil)
			return
		}
		serve(w, r, manager)
	}
}

// handleLocalWikipediaStatus passes the manager status through unchanged
// (readable, error_code and the operation-specific recommendation included).
func handleLocalWikipediaStatus(s *Server) http.HandlerFunc {
	return localWikipediaHandler(s, http.MethodGet, func(w http.ResponseWriter, r *http.Request, m *localwiki.Manager) {
		writeLocalWikipediaJSON(w, http.StatusOK, m.Status())
	})
}

func handleLocalWikipediaCatalog(s *Server) http.HandlerFunc {
	return localWikipediaHandler(s, http.MethodGet, func(w http.ResponseWriter, r *http.Request, m *localwiki.Manager) {
		language := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang")))
		if language == "" {
			language = m.Settings().SystemLanguage
		} else if !localwiki.SupportedLanguage(language) {
			writeLocalWikipediaError(w, http.StatusBadRequest, localwiki.CodeUnknownLanguage, nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		info, err := m.Catalog(ctx, language)
		if err != nil {
			writeLocalWikipediaManagerError(s, w, err)
			return
		}
		writeLocalWikipediaJSON(w, http.StatusOK, info)
	})
}

func handleLocalWikipediaInstall(s *Server) http.HandlerFunc {
	return localWikipediaHandler(s, http.MethodPost, func(w http.ResponseWriter, r *http.Request, m *localwiki.Manager) {
		var request localwiki.InstallRequest
		if !decodeLocalWikipediaBody(w, r, &request) {
			return
		}
		switch request.ReplaceMode {
		case "", localwiki.ReplaceKeepOld, localwiki.ReplaceDeleteOldFirst:
		default:
			writeLocalWikipediaError(w, http.StatusBadRequest, "invalid_request", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		if err := m.Install(ctx, request); err != nil {
			writeLocalWikipediaManagerError(s, w, err)
			return
		}
		writeLocalWikipediaJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	})
}

func handleLocalWikipediaCancel(s *Server) http.HandlerFunc {
	return localWikipediaHandler(s, http.MethodPost, func(w http.ResponseWriter, r *http.Request, m *localwiki.Manager) {
		if err := m.Cancel(); err != nil {
			writeLocalWikipediaManagerError(s, w, err)
			return
		}
		writeLocalWikipediaJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	})
}

func handleLocalWikipediaDelete(s *Server) http.HandlerFunc {
	return localWikipediaHandler(s, http.MethodPost, func(w http.ResponseWriter, r *http.Request, m *localwiki.Manager) {
		if err := m.Delete(); err != nil {
			writeLocalWikipediaManagerError(s, w, err)
			return
		}
		writeLocalWikipediaJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})
}

func handleLocalWikipediaCheckUpdate(s *Server) http.HandlerFunc {
	return localWikipediaHandler(s, http.MethodPost, func(w http.ResponseWriter, r *http.Request, m *localwiki.Manager) {
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		if err := m.CheckUpdate(ctx); err != nil {
			writeLocalWikipediaManagerError(s, w, err)
			return
		}
		writeLocalWikipediaJSON(w, http.StatusOK, m.Status())
	})
}

// localWikipediaErrorStatus maps a manager error code to its HTTP status.
func localWikipediaErrorStatus(code string) int {
	switch code {
	case localwiki.CodeInsufficientDiskSpace, localwiki.CodeDataDirInvalid:
		return http.StatusUnprocessableEntity
	case localwiki.CodeBusy, localwiki.CodeDisabled, localwiki.CodeFreeSpaceUnknown,
		localwiki.CodeAlreadyInstalled, localwiki.CodeNoOperation:
		return http.StatusConflict
	case localwiki.CodeCatalogUnreachable:
		return http.StatusBadGateway
	case localwiki.CodeUnknownLanguage:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// writeLocalWikipediaManagerError answers with a stable error code; details
// (paths, hosts) stay in the log. An insufficient-space refusal also carries
// the numbers the config UI shows and whether deleting the installed edition
// first would make the download fit.
func writeLocalWikipediaManagerError(s *Server, w http.ResponseWriter, err error) {
	code := localwiki.ErrorCode(err)
	var extra map[string]any
	var space *localwiki.InsufficientSpaceError
	if errors.As(err, &space) {
		extra = map[string]any{"required_bytes": space.Required, "free_bytes": space.Available, "can_delete_old": space.CanDeleteOld}
	}
	if s != nil && s.Logger != nil {
		level := s.Logger.Debug
		if code == localwiki.CodeInternal || code == localwiki.CodeCatalogUnreachable || code == localwiki.CodeDataDirInvalid {
			level = s.Logger.Warn
		}
		level("[LocalWikipedia] Request refused", "code", code, "error", err)
	}
	writeLocalWikipediaError(w, localWikipediaErrorStatus(code), code, extra)
}

func writeLocalWikipediaError(w http.ResponseWriter, status int, code string, extra map[string]any) {
	body := map[string]any{"error": code, "error_code": code, "recommendation": localwiki.Recommendation(code)}
	for key, value := range extra {
		body[key] = value
	}
	writeLocalWikipediaJSON(w, status, body)
}

// decodeLocalWikipediaBody accepts an empty body as an empty request and
// otherwise exactly one JSON object without unknown fields or trailing data.
func decodeLocalWikipediaBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err == nil {
		// Anything but the end of the body after the object is refused.
		if err = decoder.Decode(&json.RawMessage{}); errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = errors.New("trailing data after the request object")
		}
	} else if errors.Is(err, io.EOF) {
		err = nil // empty body
	}
	if err != nil {
		writeLocalWikipediaError(w, http.StatusBadRequest, "invalid_request", nil)
		return false
	}
	return true
}

func writeLocalWikipediaJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
