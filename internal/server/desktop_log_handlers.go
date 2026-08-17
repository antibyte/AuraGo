package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func handleDesktopLogFiles(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		logDir := desktopLogDir(s)
		files, err := listDesktopLogFiles(logDir)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if files == nil {
			files = []desktopLogFileInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"files":   files,
			"log_dir": logDir,
		})
	}
}

func handleDesktopLogTail(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name, path, err := desktopLogPathFromRequest(s, r)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		lines := desktopLogDefaultTail
		if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
			if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 {
				lines = n
			}
		}
		offset := int64(-1)
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			if n, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil {
				offset = n
			}
		}
		result, err := tailDesktopLog(path, name, lines, offset)
		if err != nil {
			if os.IsNotExist(err) {
				jsonError(w, "Log file not available", http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if result.Records == nil {
			result.Records = []desktopLogRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"file":        name,
			"lines":       result.Records,
			"total_lines": result.TotalLines,
			"eof_offset":  result.NewOffset,
			"truncated":   result.Truncated,
		})
	}
}

func handleDesktopLogStream(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name, path, err := desktopLogPathFromRequest(s, r)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		since := int64(0)
		if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
			if n, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && n >= 0 {
				since = n
			}
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			jsonError(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		if err := writeSSEComment(w, flusher, "ping"); err != nil {
			return
		}
		emit := func(evt string, payload interface{}) error {
			msg, marshalErr := json.Marshal(struct {
				Type    string      `json:"type"`
				Payload interface{} `json:"payload"`
			}{evt, payload})
			if marshalErr != nil {
				return marshalErr
			}
			return writeSSEData(w, flusher, string(msg))
		}
		if err := streamDesktopLog(r.Context(), path, name, since, emit); err != nil && r.Context().Err() == nil {
			_ = emit("log_error", map[string]interface{}{"file": name, "error": err.Error()})
		}
	}
}

func handleDesktopLogSearch(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name, path, err := desktopLogPathFromRequest(s, r)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		levels := parseDesktopLogLevels(r.URL.Query().Get("levels"))
		limit := desktopLogSearchDefault
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 {
				limit = n
			}
		}
		matches, err := searchDesktopLog(path, name, query, levels, limit)
		if err != nil {
			if os.IsNotExist(err) {
				jsonError(w, "Log file not available", http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if matches == nil {
			matches = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"file":     name,
			"q":        query,
			"matches":  matches,
			"count":    len(matches),
			"limit":    limit,
			"overflow": len(matches) >= limit,
		})
	}
}

func handleDesktopLogDownload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if desktopVirtualDesktopReadOnly(s) {
			jsonError(w, "desktop_readonly", http.StatusForbidden)
			return
		}
		name, path, err := desktopLogPathFromRequest(s, r)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				jsonError(w, "Log file not available", http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeContentDisposition(name)))
		http.ServeContent(w, r, name, stat.ModTime(), file)
	}
}

func desktopLogPathFromRequest(s *Server, r *http.Request) (string, string, error) {
	name, err := sanitizeDesktopLogName(r.URL.Query().Get("file"))
	if err != nil {
		return "", "", err
	}
	path, err := resolveDesktopLogFile(desktopLogDir(s), name)
	if err != nil {
		return "", "", err
	}
	return name, path, nil
}

func parseDesktopLogLevels(raw string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	levels := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		level := strings.ToUpper(strings.TrimSpace(part))
		if level == "" {
			continue
		}
		levels[level] = struct{}{}
	}
	if len(levels) == 0 {
		return nil
	}
	return levels
}
