package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/office"
)

func handleDesktopOfficeDocument(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			path := r.URL.Query().Get("path")
			data, entry, err := svc.ReadFileBytes(r.Context(), path)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			doc, err := office.DecodeDocument(entry.Name, data)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			doc.Path = entry.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "entry": entry, "document": doc, "office_version": officeVersionForEntry(entry, data)})
		case http.MethodPut, http.MethodPost:
			var body officeDocumentSaveRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			path := strings.TrimSpace(body.Path)
			if path == "" {
				path = r.URL.Query().Get("path")
			}
			if path == "" {
				jsonError(w, "path is required", http.StatusBadRequest)
				return
			}
			doc := body.Document
			if doc.Text == "" && body.Text != "" {
				doc.Text = body.Text
			}
			if doc.Text == "" && body.Content != "" {
				doc.Text = body.Content
			}
			if doc.Title == "" {
				doc.Title = body.Title
			}
			if doc.HTML == "" {
				doc.HTML = body.HTML
			}
			data, _, err := office.EncodeDocument(path, doc)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			officeVersion, err := writeOfficeFileBytesChecked(r.Context(), svc, path, data, body.OfficeVersion)
			if err != nil {
				if isOfficeConflictError(err) {
					jsonError(w, err.Error(), http.StatusConflict)
					return
				}
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_document", "path": path}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": path, "office_version": officeVersion})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopOfficeWorkbook(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			path := r.URL.Query().Get("path")
			data, entry, err := svc.ReadFileBytes(r.Context(), path)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			workbook, err := office.DecodeWorkbook(entry.Name, data)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			workbook.Path = entry.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "entry": entry, "workbook": workbook, "office_version": officeVersionForEntry(entry, data)})
		case http.MethodPut, http.MethodPost:
			var body officeWorkbookSaveRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			path := strings.TrimSpace(body.Path)
			if path == "" {
				path = r.URL.Query().Get("path")
			}
			if path == "" {
				jsonError(w, "path is required", http.StatusBadRequest)
				return
			}
			var workbook office.Workbook
			if len(body.Workbook) > 0 && string(body.Workbook) != "null" {
				if err := json.Unmarshal(body.Workbook, &workbook); err != nil {
					jsonError(w, "Invalid workbook JSON", http.StatusBadRequest)
					return
				}
			} else {
				jsonError(w, "workbook is required", http.StatusBadRequest)
				return
			}
			data, err := encodeWorkbookForPath(path, workbook, body.Sheet)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			officeVersion, err := writeOfficeFileBytesChecked(r.Context(), svc, path, data, body.OfficeVersion)
			if err != nil {
				if isOfficeConflictError(err) {
					jsonError(w, err.Error(), http.StatusConflict)
					return
				}
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_workbook", "path": path}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": path, "office_version": officeVersion})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDesktopOfficeExport(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		format := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("format")), "."))
		if format == "" {
			jsonError(w, "format is required", http.StatusBadRequest)
			return
		}
		data, entry, err := svc.ReadFileBytes(r.Context(), path)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		outputName := strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name)) + "." + format
		var output []byte
		var mimeType string
		switch format {
		case "docx", "html", "htm", "md", "txt":
			doc, err := office.DecodeDocument(entry.Name, data)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			output, mimeType, err = office.EncodeDocument(outputName, doc)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "xlsx":
			workbook, err := office.DecodeWorkbook(entry.Name, data)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			output, err = office.EncodeWorkbook(workbook)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case "csv":
			workbook, err := office.DecodeWorkbook(entry.Name, data)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			output, err = office.EncodeCSV(workbook, r.URL.Query().Get("sheet"))
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			mimeType = "text/csv; charset=utf-8"
		default:
			jsonError(w, fmt.Sprintf("unsupported export format %q", format), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(outputName, `"`, "")))
		http.ServeContent(w, r, outputName, entry.ModTime, bytes.NewReader(output))
	}
}

type officeVersion struct {
	Path     string `json:"path"`
	Modified string `json:"modified"`
	ModTime  string `json:"mod_time"`
	Size     int64  `json:"size"`
	ETag     string `json:"etag"`
}

type officeDocumentSaveRequest struct {
	Path          string          `json:"path"`
	Title         string          `json:"title"`
	Text          string          `json:"text"`
	Content       string          `json:"content"`
	HTML          string          `json:"html"`
	Document      office.Document `json:"document"`
	OfficeVersion *officeVersion  `json:"office_version"`
}

type officeWorkbookSaveRequest struct {
	Path          string          `json:"path"`
	Workbook      json.RawMessage `json:"workbook"`
	Sheet         string          `json:"sheet"`
	OfficeVersion *officeVersion  `json:"office_version"`
}

type officeConflictError struct {
	message string
}

func (e officeConflictError) Error() string {
	return e.message
}

func isOfficeConflictError(err error) bool {
	var conflict officeConflictError
	return errors.As(err, &conflict)
}

func officeVersionForEntry(entry desktop.FileEntry, data []byte) officeVersion {
	modified := entry.ModTime.UTC().Format(time.RFC3339Nano)
	etagHash := sha256.New()
	_, _ = etagHash.Write([]byte(entry.Path))
	_, _ = etagHash.Write([]byte{0})
	_, _ = etagHash.Write(data)
	return officeVersion{
		Path:     entry.Path,
		Modified: modified,
		ModTime:  modified,
		Size:     entry.Size,
		ETag:     fmt.Sprintf("%x", etagHash.Sum(nil)),
	}
}

func writeOfficeFileBytesChecked(ctx context.Context, svc *desktop.Service, path string, data []byte, expected *officeVersion) (*officeVersion, error) {
	entry, err := svc.WriteFileBytesConditional(ctx, path, data, desktop.SourceUser, func(current desktop.FileWriteState) error {
		return checkOfficeVersion(current, expected)
	})
	if err != nil {
		return nil, err
	}
	version := officeVersionForEntry(entry, data)
	return &version, nil
}

func checkOfficeVersion(current desktop.FileWriteState, expected *officeVersion) error {
	if !current.Exists {
		if expected == nil {
			return nil
		}
		return officeConflictError{message: "office file changed; reload before saving"}
	}
	if expected == nil {
		return officeConflictError{message: "office file changed; reload before saving"}
	}
	currentVersion := officeVersionForEntry(current.Entry, current.Data)
	if strings.TrimSpace(expected.ETag) != "" {
		if strings.TrimSpace(expected.ETag) == currentVersion.ETag {
			return nil
		}
		return officeConflictError{message: "office file changed; reload before saving"}
	}
	matchesModified := strings.TrimSpace(expected.Modified) == currentVersion.Modified || strings.TrimSpace(expected.ModTime) == currentVersion.ModTime
	if expected.Size == currentVersion.Size && matchesModified {
		return nil
	}
	return officeConflictError{message: "office file changed; reload before saving"}
}

func encodeWorkbookForPath(path string, workbook office.Workbook, sheet string) ([]byte, error) {
	if strings.EqualFold(filepath.Ext(path), ".csv") {
		return office.EncodeCSV(workbook, sheet)
	}
	return office.EncodeWorkbook(workbook)
}
