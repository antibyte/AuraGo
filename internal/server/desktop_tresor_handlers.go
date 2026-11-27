package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tresor"
)

const tresorMaxBody = 50<<20 + 28 // AES-GCM nonce and tag are outside the 50 MiB plaintext.

// tresorError keeps a machine-readable code next to the human message so the app
// can tell a revoked session, a read-only desktop and a TLS problem apart.
func tresorError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}

func tresorUnavailable(s *Server, w http.ResponseWriter, err error) {
	if s != nil && s.Logger != nil {
		s.Logger.Warn("Tresor request failed", "error", err)
	}
	tresorError(w, "tresor_unavailable", "Tresor unavailable", http.StatusServiceUnavailable)
}

func handleDesktopTresor(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if s == nil || s.Cfg == nil {
			tresorError(w, "tresor_unavailable", "Tresor unavailable", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		dataDir := s.Cfg.Directories.DataDir
		desktopEnabled := s.Cfg.VirtualDesktop.Enabled
		s.CfgMu.RUnlock()
		if !authEnabled || strings.TrimSpace(r.Header.Get("Authorization")) != "" || !IsAuthenticated(r, secret) {
			tresorError(w, "admin_required", "Browser admin session required", http.StatusUnauthorized)
			return
		}
		if !IsSecureRequest(r) && !tresorLoopbackRequest(r) {
			tresorError(w, "https_required", "Use HTTPS to open the Tresor", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && !checkCSRFOriginWithPolicy(r, true) {
			tresorError(w, "invalid_origin", "Invalid request origin", http.StatusForbidden)
			return
		}
		if !desktopEnabled {
			tresorError(w, "desktop_disabled", "The Virtual Desktop is disabled in the configuration", http.StatusServiceUnavailable)
			return
		}
		if dataDir == "" {
			tresorError(w, "tresor_unavailable", "Tresor data directory unavailable", http.StatusServiceUnavailable)
			return
		}
		if !checkDesktopOperation(s, w, r, desktopMethodOperation(r.Method)) {
			return
		}
		store, err := tresor.Open(filepath.Join(dataDir, config.TresorDBFilename))
		if err != nil {
			tresorUnavailable(s, w, err)
			return
		}
		defer store.Close()

		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/tresor")
		switch {
		case path == "" && r.Method == http.MethodGet:
			h, err := store.Header(r.Context())
			if errors.Is(err, sql.ErrNoRows) {
				json.NewEncoder(w).Encode(map[string]any{"initialized": false})
			} else if err != nil {
				tresorUnavailable(s, w, err)
			} else {
				json.NewEncoder(w).Encode(map[string]any{"initialized": true, "header": h})
			}
		case path == "" && (r.Method == http.MethodPost || r.Method == http.MethodPut):
			var h tresor.Header
			if !decodeTresorJSON(w, r, &h, 4096) || !validTresorHeader(h) {
				tresorError(w, "invalid_header", "Invalid key envelopes", http.StatusBadRequest)
				return
			}
			if r.Method == http.MethodPost {
				err = publishDesktopResult(r.Context(), func() error { return store.Setup(r.Context(), h) })
			} else {
				var expected int64
				if expected, err = tresorMatchRevision(r); err != nil {
					tresorError(w, "revision_required", "If-Match revision required", http.StatusPreconditionRequired)
					return
				}
				err = publishDesktopResult(r.Context(), func() error { return store.Rewrap(r.Context(), h, expected) })
			}
			tresorMutationResult(s, w, err)
		case path == "/items" && r.Method == http.MethodGet:
			items, err := store.List(r.Context())
			if err != nil {
				tresorUnavailable(s, w, err)
				return
			}
			json.NewEncoder(w).Encode(items)
		case path == "/items" && r.Method == http.MethodPost:
			var item tresor.Record
			if !decodeTresorJSON(w, r, &item, 72<<20) || !validTresorRecord(item) {
				tresorError(w, "invalid_record", "Invalid encrypted record", http.StatusBadRequest)
				return
			}
			tresorMutationResult(s, w, publishDesktopResult(r.Context(), func() error { return store.Create(r.Context(), item) }))
		case strings.HasPrefix(path, "/items/"):
			id := strings.TrimPrefix(path, "/items/")
			if !validTresorID(id) {
				tresorError(w, "invalid_record_id", "Invalid record id", http.StatusBadRequest)
				return
			}
			switch r.Method {
			case http.MethodGet:
				item, err := store.Get(r.Context(), id)
				if errors.Is(err, sql.ErrNoRows) {
					tresorError(w, "not_found", "Record not found", http.StatusNotFound)
					return
				}
				if err != nil {
					tresorUnavailable(s, w, err)
					return
				}
				w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(item.Revision, 10)))
				json.NewEncoder(w).Encode(item)
			case http.MethodPut, http.MethodDelete:
				expected, err := tresorMatchRevision(r)
				if err != nil {
					tresorError(w, "revision_required", "If-Match revision required", http.StatusPreconditionRequired)
					return
				}
				if r.Method == http.MethodDelete {
					err = publishDesktopResult(r.Context(), func() error { return store.Delete(r.Context(), id, expected) })
				} else {
					var item tresor.Record
					if !decodeTresorJSON(w, r, &item, 72<<20) || item.ID != id || !validTresorRecord(item) {
						tresorError(w, "invalid_record", "Invalid encrypted record", http.StatusBadRequest)
						return
					}
					err = publishDesktopResult(r.Context(), func() error { return store.Update(r.Context(), item, expected) })
				}
				tresorMutationResult(s, w, err)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func tresorLoopbackRequest(r *http.Request) bool {
	if r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Forwarded-Proto") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return false
	}
	name := r.Host
	if h, _, err := net.SplitHostPort(name); err == nil {
		name = h
	}
	return strings.EqualFold(name, "localhost") || (net.ParseIP(name) != nil && net.ParseIP(name).IsLoopback())
}

func decodeTresorJSON(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

func validTresorHeader(h tresor.Header) bool {
	return len(h.Salt) == 32 && len(h.PasswordEnvelope) == 60 && len(h.RecoveryEnvelope) == 60 && tresor.ValidKDFLabel(h.KDF)
}

func validTresorID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func validTresorRecord(item tresor.Record) bool {
	return validTresorID(item.ID) && len(item.Meta) >= 28 && len(item.Meta) <= 65536+28 && len(item.Body) >= 28 && len(item.Body) <= tresorMaxBody
}

func tresorMatchRevision(r *http.Request) (int64, error) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 1 {
		return 0, errors.New("invalid revision")
	}
	return n, nil
}

func tresorMutationResult(s *Server, w http.ResponseWriter, err error) {
	if errors.Is(err, tresor.ErrConflict) {
		tresorError(w, "conflict", "Tresor revision conflict", http.StatusPreconditionFailed)
		return
	}
	if err != nil {
		tresorUnavailable(s, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
