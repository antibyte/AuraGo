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

func handleDesktopTresor(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if s == nil || s.Cfg == nil {
			jsonError(w, "Tresor unavailable", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()
		if !authEnabled || strings.TrimSpace(r.Header.Get("Authorization")) != "" || !IsAuthenticated(r, secret) {
			jsonError(w, "Browser admin session required", http.StatusUnauthorized)
			return
		}
		if !IsSecureRequest(r) && !tresorLoopbackRequest(r) {
			jsonError(w, "Use HTTPS to open the Tresor", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && !checkCSRFOriginWithPolicy(r, true) {
			jsonError(w, "Invalid request origin", http.StatusForbidden)
			return
		}
		if dataDir == "" {
			jsonError(w, "Tresor data directory unavailable", http.StatusServiceUnavailable)
			return
		}
		if !checkDesktopOperation(s, w, r, desktopMethodOperation(r.Method)) {
			return
		}
		store, err := tresor.Open(filepath.Join(dataDir, config.TresorDBFilename))
		if err != nil {
			jsonError(w, "Tresor unavailable", http.StatusServiceUnavailable)
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
				jsonError(w, "Tresor unavailable", http.StatusServiceUnavailable)
			} else {
				json.NewEncoder(w).Encode(map[string]any{"initialized": true, "header": h})
			}
		case path == "" && (r.Method == http.MethodPost || r.Method == http.MethodPut):
			var h tresor.Header
			if !decodeTresorJSON(w, r, &h, 4096) || !validTresorHeader(h) {
				jsonError(w, "Invalid key envelopes", http.StatusBadRequest)
				return
			}
			if r.Method == http.MethodPost {
				err = publishDesktopResult(r.Context(), func() error { return store.Setup(r.Context(), h) })
			} else {
				var expected int64
				if expected, err = tresorMatchRevision(r); err != nil {
					jsonError(w, "If-Match revision required", http.StatusPreconditionRequired)
					return
				}
				err = publishDesktopResult(r.Context(), func() error { return store.Rewrap(r.Context(), h, expected) })
			}
			tresorMutationResult(w, err)
		case path == "/items" && r.Method == http.MethodGet:
			items, err := store.List(r.Context())
			if err != nil {
				jsonError(w, "Tresor unavailable", http.StatusServiceUnavailable)
				return
			}
			json.NewEncoder(w).Encode(items)
		case path == "/items" && r.Method == http.MethodPost:
			var item tresor.Record
			if !decodeTresorJSON(w, r, &item, 72<<20) || !validTresorRecord(item) {
				jsonError(w, "Invalid encrypted record", http.StatusBadRequest)
				return
			}
			tresorMutationResult(w, publishDesktopResult(r.Context(), func() error { return store.Create(r.Context(), item) }))
		case strings.HasPrefix(path, "/items/"):
			id := strings.TrimPrefix(path, "/items/")
			if !validTresorID(id) {
				jsonError(w, "Invalid record id", http.StatusBadRequest)
				return
			}
			switch r.Method {
			case http.MethodGet:
				item, err := store.Get(r.Context(), id)
				if errors.Is(err, sql.ErrNoRows) {
					jsonError(w, "Record not found", http.StatusNotFound)
					return
				}
				if err != nil {
					jsonError(w, "Tresor unavailable", http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(item.Revision, 10)))
				json.NewEncoder(w).Encode(item)
			case http.MethodPut, http.MethodDelete:
				expected, err := tresorMatchRevision(r)
				if err != nil {
					jsonError(w, "If-Match revision required", http.StatusPreconditionRequired)
					return
				}
				if r.Method == http.MethodDelete {
					err = publishDesktopResult(r.Context(), func() error { return store.Delete(r.Context(), id, expected) })
				} else {
					var item tresor.Record
					if !decodeTresorJSON(w, r, &item, 72<<20) || item.ID != id || !validTresorRecord(item) {
						jsonError(w, "Invalid encrypted record", http.StatusBadRequest)
						return
					}
					err = publishDesktopResult(r.Context(), func() error { return store.Update(r.Context(), item, expected) })
				}
				tresorMutationResult(w, err)
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
	return len(h.Salt) == 32 && len(h.PasswordEnvelope) == 60 && len(h.RecoveryEnvelope) == 60
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

func tresorMutationResult(w http.ResponseWriter, err error) {
	if errors.Is(err, tresor.ErrConflict) {
		jsonError(w, "Tresor revision conflict", http.StatusPreconditionFailed)
		return
	}
	if err != nil {
		jsonError(w, "Tresor unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
