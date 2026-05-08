package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"aurago/internal/office"
)

func handleDesktopViewerContent(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		path := r.URL.Query().Get("path")
		if strings.TrimSpace(path) == "" {
			jsonError(w, "path is required", http.StatusBadRequest)
			return
		}
		data, entry, err := svc.ReadFileBytes(r.Context(), path)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ext := strings.ToLower(filepath.Ext(entry.Name))
		switch ext {
		case ".pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(entry.Name, `"`, "")))
			http.ServeContent(w, r, entry.Name, entry.ModTime, bytes.NewReader(data))
		case ".md":
			content := string(data)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"type":    "markdown",
				"content": content,
				"entry":   entry,
			})
		case ".docx":
			doc, decodeErr := office.DecodeDocument(entry.Name, data)
			if decodeErr != nil {
				jsonError(w, decodeErr.Error(), http.StatusBadRequest)
				return
			}
			htmlContent := office.DocumentToHTML(doc)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"type":    "document",
				"content": htmlContent,
				"entry":   entry,
			})
		case ".xlsx", ".xlsm", ".csv":
			workbook, decodeErr := office.DecodeWorkbook(entry.Name, data)
			if decodeErr != nil {
				jsonError(w, decodeErr.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   "ok",
				"type":     "spreadsheet",
				"workbook": workbook,
				"entry":    entry,
			})
		default:
			jsonError(w, fmt.Sprintf("unsupported viewer file type %q", ext), http.StatusBadRequest)
		}
	}
}
