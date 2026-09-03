package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"aurago/internal/desktop"
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
		if entryName := strings.TrimSpace(r.URL.Query().Get("entry")); entryName != "" {
			serveDesktopViewerArchiveEntry(w, r, svc, path, entryName)
			return
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".pdf" {
			file, entry, _, err := svc.OpenPreviewFile(r.Context(), path)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer file.Close()
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeContentDisposition(entry.Name)))
			http.ServeContent(w, r, entry.Name, entry.ModTime, file)
			return
		}

		data, entry, err := svc.ReadFileBytes(r.Context(), path)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		switch ext {
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

func serveDesktopViewerArchiveEntry(w http.ResponseWriter, r *http.Request, svc *desktop.Service, zipPath, entryName string) {
	archive, err := svc.ReadArchiveEntry(r.Context(), zipPath, entryName)
	if err != nil {
		writeDesktopArchiveEntryError(w, err)
		return
	}
	ext := strings.ToLower(filepath.Ext(archive.Name))
	entry := desktop.FileEntry{
		Name:     archive.Name,
		Path:     zipPath,
		Type:     "file",
		Size:     archive.Size,
		ModTime:  archive.ModTime,
		Modified: archive.ModTime,
		MIMEType: archive.MIMEType,
	}
	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeContentDisposition(archive.Name)))
		http.ServeContent(w, r, archive.Name, archive.ModTime, bytes.NewReader(archive.Data))
		return
	}
	switch ext {
	case ".md":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"type":    "markdown",
			"content": string(archive.Data),
			"entry":   entry,
		})
	case ".docx":
		doc, decodeErr := office.DecodeDocument(archive.Name, archive.Data)
		if decodeErr != nil {
			jsonError(w, decodeErr.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"type":    "document",
			"content": office.DocumentToHTML(doc),
			"entry":   entry,
		})
	case ".xlsx", ".xlsm", ".csv":
		workbook, decodeErr := office.DecodeWorkbook(archive.Name, archive.Data)
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
