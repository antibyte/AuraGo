package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/rustfs"
)

func registerRustFSRoutes(mux *http.ServeMux, s *Server) {
	register := func(path, action string) {
		handler := requireAdmin(s, handleRustFS(s, action))
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			handler.ServeHTTP(w, r)
		})
	}
	for _, action := range []string{"status", "preflight", "install", "start", "stop", "recreate", "remove"} {
		register("/api/rustfs/"+action, action)
	}
	register("/api/rustfs/operations/", "operation")
}

func handleRustFS(s *Server, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		method := http.MethodPost
		if action == "status" || action == "operation" {
			method = http.MethodGet
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeRustFSJSON(w, http.StatusMethodNotAllowed, map[string]string{"status": "error", "code": "method_not_allowed"})
			return
		}
		if method == http.MethodPost {
			_, bearer := bearerCredential(r.Header.Get("Authorization"))
			if !bearer && !checkCSRFOriginWithPolicy(r, true) {
				writeRustFSJSON(w, http.StatusForbidden, map[string]string{"status": "error", "code": "origin_denied"})
				return
			}
			if !decodeRustFSAction(w, r) {
				return
			}
		}
		if s.RustFS == nil {
			writeRustFSJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "code": "service_not_ready"})
			return
		}
		switch action {
		case "status":
			writeRustFSJSON(w, http.StatusOK, s.RustFS.Status())
		case "preflight":
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			result, err := s.RustFS.Preflight(ctx)
			if err != nil {
				writeRustFSError(w, err)
				return
			}
			writeRustFSJSON(w, http.StatusOK, result)
		case "operation":
			id := strings.TrimPrefix(r.URL.Path, "/api/rustfs/operations/")
			if id == "" || len(id) > 128 || strings.ContainsAny(id, "/\\") {
				writeRustFSJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "code": "invalid_operation"})
				return
			}
			operation, ok := s.RustFS.Operation(id)
			if !ok {
				writeRustFSJSON(w, http.StatusNotFound, map[string]string{"status": "error", "code": "operation_not_found"})
				return
			}
			writeRustFSJSON(w, http.StatusOK, operation)
		default:
			operation, err := s.RustFS.Begin(rustfs.Action(action))
			if err != nil {
				writeRustFSError(w, err)
				return
			}
			writeRustFSJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "operation": operation})
		}
	}
}

func decodeRustFSAction(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body *struct{}
	err := decoder.Decode(&body)
	if err == nil && body == nil {
		err = errors.New("JSON object required")
	}
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("trailing JSON")
		}
	}
	if err == nil {
		return true
	}
	status := http.StatusBadRequest
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		status = http.StatusRequestEntityTooLarge
	}
	writeRustFSJSON(w, status, map[string]string{"status": "error", "code": "invalid_request"})
	return false
}

func writeRustFSError(w http.ResponseWriter, err error) {
	code := rustfs.ErrorCode(err)
	status := http.StatusConflict
	switch code {
	case "docker_disabled", "docker_readonly":
		status = http.StatusForbidden
	case "docker_unavailable":
		status = http.StatusServiceUnavailable
	}
	writeRustFSJSON(w, status, map[string]string{"status": "error", "code": code})
}

func writeRustFSJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
