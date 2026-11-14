package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

// desktopWriterPDFRender is the renderer; a variable so that tests run without a browser.
var desktopWriterPDFRender = tools.RenderHTMLPDF

const desktopWriterPDFTimeout = 2 * time.Minute

// handleDesktopWriterPDF renders the Writer's packed print pages (one self-contained HTML page) to a PDF. Without
// ?path the response is the PDF. With it the PDF becomes a file in the workspace: create only, or over an existing
// file with overwrite=1. An existing file is refused before rendering, so the client can ask and send again.
func handleDesktopWriterPDF(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("path")
		scope, operation := desktopScopeRead, desktopRead
		if target != "" {
			scope, operation = desktopScopeWrite, desktopWrite
		}
		if !requireDesktopOperation(s, w, r, scope, operation) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// Read the page before any refusal: a browser that is still uploading megabytes reports a refusal that
		// closes the connection as a network failure, not as the answer.
		page, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 50<<20))
		if err != nil {
			jsonError(w, "Document exceeds export limit", http.StatusRequestEntityTooLarge)
			return
		}
		if len(bytes.TrimSpace(page)) == 0 {
			jsonError(w, "Print pages required", http.StatusBadRequest)
			return
		}
		overwrite := r.URL.Query().Get("overwrite") == "1"
		if target != "" {
			if !strings.EqualFold(filepath.Ext(target), ".pdf") {
				jsonError(w, "PDF path required", http.StatusBadRequest)
				return
			}
			if _, _, err := svc.ReadFileBytes(r.Context(), target); err == nil {
				if !overwrite {
					jsonError(w, "pdf_exists", http.StatusConflict)
					return
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), desktopWriterPDFTimeout)
		defer cancel()
		data, err := desktopWriterPDFRender(ctx, s.ConfigSnapshot(), string(page))
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Writer PDF was not rendered", "error", err)
			}
			if errors.Is(err, tools.ErrHTMLPDFUnavailable) {
				jsonError(w, "pdf_unavailable", http.StatusServiceUnavailable)
				return
			}
			jsonError(w, "pdf_failed", http.StatusBadGateway)
			return
		}
		if target == "" {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Disposition", `attachment; filename="document.pdf"`)
			w.Write(data)
			return
		}
		entry, err := svc.WriteFileBytesConditional(r.Context(), target, data, desktop.SourceUser, func(current desktop.FileWriteState) error {
			if current.Exists && !overwrite {
				return officeConflictError{message: "pdf_exists"}
			}
			return nil
		})
		if err != nil {
			code := http.StatusBadRequest
			if isOfficeConflictError(err) {
				code = http.StatusConflict
			}
			jsonError(w, err.Error(), code)
			return
		}
		broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": entry.Path}, CreatedAt: time.Now().UTC()})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "path": entry.Path})
	}
}
