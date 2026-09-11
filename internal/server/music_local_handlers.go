package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func registerLocalMusicRoutes(mux *http.ServeMux, s *Server) {
	for _, route := range []string{"status", "probe", "action"} {
		mux.Handle("/api/music-generation/local/"+route, requireAdmin(s, handleLocalMusic(s, route)))
	}
}

func localMusicStopPending(s *Server) bool {
	if s.LocalMusic == nil {
		return false
	}
	return s.LocalMusic.StopPending()
}

func handleLocalMusic(s *Server, route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if route == "status" && r.Method != http.MethodGet || route != "status" && r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LocalMusic == nil {
			jsonError(w, "acestep_unavailable", http.StatusServiceUnavailable)
			return
		}
		if route != "status" {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") && !checkCSRFOrigin(r) {
				jsonError(w, "csrf_check_failed", http.StatusForbidden)
				return
			}
			s.CfgMu.RLock()
			writable := s.Cfg.Docker.Enabled && !s.Cfg.Docker.ReadOnly
			s.CfgMu.RUnlock()
			if !writable {
				jsonError(w, "docker_write_disabled", http.StatusForbidden)
				return
			}
			action := "probe"
			if route == "action" {
				var body struct {
					Action string `json:"action"`
				}
				decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&body); err != nil {
					jsonError(w, "Invalid JSON", http.StatusBadRequest)
					return
				}
				if decoder.Decode(&struct{}{}) != io.EOF {
					jsonError(w, "Invalid JSON", http.StatusBadRequest)
					return
				}
				action = body.Action
			}
			if err := s.LocalMusic.Action(action); err != nil {
				jsonError(w, err.Error(), http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		_ = json.NewEncoder(w).Encode(s.LocalMusic.Status())
	}
}
