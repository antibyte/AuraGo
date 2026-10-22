package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/ocr"
	"aurago/internal/tools"
)

func registerOCRRoutes(mux *http.ServeMux, s *Server) {
	for _, action := range []string{"status", "setup", "start", "stop", "test"} {
		mux.Handle("/api/ocr/"+action, requireAdmin(s, handleOCRAdmin(s, action)))
	}
	mux.HandleFunc("/api/desktop/ocr/", handleDesktopOCR(s))
}
func ocrHTTPError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, ocr.ErrDisabled):
		code = http.StatusForbidden
	case errors.Is(err, ocr.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ocr.ErrNotReady):
		code = http.StatusServiceUnavailable
	case errors.Is(err, ocr.ErrBusy):
		code = http.StatusConflict
	case errors.Is(err, ocr.ErrLimit):
		code = http.StatusRequestEntityTooLarge
	}
	jsonError(w, "OCR: "+publicOCRError(err), code)
}
func publicOCRError(err error) string {
	for _, known := range []error{ocr.ErrDisabled, ocr.ErrNotFound, ocr.ErrNotReady, ocr.ErrBusy, ocr.ErrLimit, ocr.ErrInvalid} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return "ocr_runtime_failed"
}
func handleOCRAdmin(s *Server, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := http.MethodPost
		if action == "status" {
			method = http.MethodGet
		}
		if r.Method != method {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if s.OCR == nil {
			ocrHTTPError(w, ocr.ErrNotReady)
			return
		}
		if action == "status" {
			writeJSON(w, s.OCR.Status())
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.OCR.Enabled {
			ocrHTTPError(w, ocr.ErrDisabled)
			return
		}
		switch action {
		case "setup", "start":
			if !cfg.Docker.Enabled || cfg.Docker.ReadOnly {
				jsonError(w, "ocr_docker_control_required", 403)
				return
			}
			status := s.OCR.Status()
			if status.State == "starting" || status.State == "installing" {
				ocrHTTPError(w, ocr.ErrBusy)
				return
			}
			go func() { _ = s.OCR.Start(localLLMLifecycleContext(s), action == "setup") }()
			w.WriteHeader(http.StatusAccepted)
			writeJSON(w, map[string]string{"state": "accepted"})
		case "stop":
			if !cfg.Docker.Enabled || cfg.Docker.ReadOnly {
				jsonError(w, "ocr_docker_control_required", 403)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			if err := s.OCR.Stop(ctx); err != nil {
				ocrHTTPError(w, err)
				return
			}
			writeJSON(w, s.OCR.Status())
		case "test":
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			img := image.NewGray(image.Rect(0, 0, 128, 128))
			for i := range img.Pix {
				img.Pix[i] = 255
			}
			img.Set(64, 64, color.Gray{Y: 0})
			var data bytes.Buffer
			_ = png.Encode(&data, img)
			file, err := s.OCR.Stage("system:test", "test.png", &data)
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			job, err := s.OCR.Submit("system:test", []string{file.ID}, "ocr")
			if err == nil {
				_, err = tools.WaitOCRText(ctx, s.OCR, "system:test", job.ID)
			}
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "version": ocr.Version, "model_revision": ocr.ModelRevision})
		}
	}
}

func desktopOCROwner(r *http.Request) string {
	owner := desktopRequestSessionHash(r)
	if owner == "" {
		owner = "local-no-auth"
	}
	return "desktop:" + owner
}
func handleDesktopOCR(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		if s.OCR == nil {
			ocrHTTPError(w, ocr.ErrNotReady)
			return
		}
		if r.URL.Path == "/api/desktop/ocr/status" && r.Method == http.MethodGet {
			writeJSON(w, s.OCR.Status())
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.OCR.Enabled {
			ocrHTTPError(w, ocr.ErrDisabled)
			return
		}
		owner := desktopOCROwner(r)
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/ocr/"), "/"), "/")
		if len(parts) == 1 && parts[0] == "uploads" && r.Method == http.MethodPost {
			maxBytes := ocr.MaxFileBytes
			if n := int64(cfg.VirtualDesktop.MaxFileSizeMB) << 20; n > 0 {
				if n < maxBytes {
					maxBytes = n
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(64<<10))
			reader, err := r.MultipartReader()
			if err != nil {
				ocrHTTPError(w, ocr.ErrInvalid)
				return
			}
			part, err := reader.NextPart()
			if err != nil || part.FormName() != "file" {
				ocrHTTPError(w, ocr.ErrInvalid)
				return
			}
			defer part.Close()
			file, err := s.OCR.StageBounded(owner, part.FileName(), part, maxBytes)
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			writeJSON(w, file)
			return
		}
		if len(parts) == 1 && parts[0] == "jobs" {
			if r.Method == http.MethodGet {
				writeJSON(w, s.OCR.List(owner))
				return
			}
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", 405)
				return
			}
			var body struct {
				Uploads []string `json:"uploads"`
				Paths   []string `json:"paths"`
				Mode    string   `json:"mode"`
			}
			if err := decodeDesktopJSON(w, r, &body, 64<<10); err != nil {
				ocrHTTPError(w, ocr.ErrInvalid)
				return
			}
			if len(body.Uploads)+len(body.Paths) == 0 || len(body.Uploads)+len(body.Paths) > ocr.MaxFiles {
				ocrHTTPError(w, ocr.ErrLimit)
				return
			}
			if len(body.Paths) > 0 {
				service, _, err := s.getDesktopService(r.Context())
				if err != nil {
					ocrHTTPError(w, err)
					return
				}
				for _, path := range body.Paths {
					content, entry, err := service.ReadFileBytes(r.Context(), path)
					if err != nil {
						ocrHTTPError(w, err)
						return
					}
					file, err := s.OCR.Stage(owner, entry.Name, bytes.NewReader(content))
					if err != nil {
						ocrHTTPError(w, err)
						return
					}
					body.Uploads = append(body.Uploads, file.ID)
				}
			}
			job, err := s.OCR.Submit(owner, body.Uploads, body.Mode)
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			writeJSON(w, job)
			return
		}
		if len(parts) < 2 || parts[0] != "jobs" {
			jsonError(w, "Not found", 404)
			return
		}
		key := parts[1]
		if len(parts) == 2 && r.Method == http.MethodGet {
			job, err := s.OCR.Get(owner, key)
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			writeJSON(w, job)
			return
		}
		if len(parts) == 3 && parts[2] == "cancel" && r.Method == http.MethodPost {
			if err := s.OCR.Cancel(owner, key); err != nil {
				ocrHTTPError(w, err)
				return
			}
			writeJSON(w, map[string]string{"state": "cancelled"})
			return
		}
		if len(parts) != 3 || r.Method != http.MethodGet {
			jsonError(w, "Not found", 404)
			return
		}
		fileID := r.URL.Query().Get("file_id")
		if parts[2] == "source" {
			source, mime, err := s.OCR.Source(owner, key, fileID)
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			defer source.Close()
			w.Header().Set("Content-Type", mime)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
			info, err := source.Stat()
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			http.ServeContent(w, r, "source", info.ModTime(), source)
			return
		}
		if parts[2] == "result" {
			file, err := s.OCR.Result(owner, key, fileID)
			if err != nil {
				ocrHTTPError(w, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, file)
			return
		}
		if parts[2] == "export" {
			exportOCR(w, r, s.OCR, owner, key, fileID)
			return
		}
		jsonError(w, "Not found", 404)
	}
}

func ocrFileText(file ocr.File, format string) string {
	var text strings.Builder
	if file.State != "completed" {
		fmt.Fprintf(&text, "[OCR incomplete: state=%s; error=%s; returned_pages=%d]\n\n", file.State, file.Error, len(file.Pages))
	}
	for _, page := range file.Pages {
		if page.Source == "error" || page.Note != "" {
			fmt.Fprintf(&text, "[OCR warning: page=%d; source=%s; note=%s]\n\n", page.Index+1, page.Source, page.Note)
		}
		if format == "md" {
			text.WriteString(page.Markdown)
		} else {
			text.WriteString(page.Text)
		}
		text.WriteString("\n\n")
	}
	return text.String()
}
func exportOCR(w http.ResponseWriter, r *http.Request, manager *ocr.Manager, owner, key, fileID string) {
	format := r.URL.Query().Get("format")
	if format == "zip" {
		job, err := manager.Get(owner, key)
		if err != nil {
			ocrHTTPError(w, err)
			return
		}
		if job.FinishedAt == nil {
			ocrHTTPError(w, ocr.ErrBusy)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="ocr-results.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		archive := zip.NewWriter(w)
		defer archive.Close()
		add := func(name string, data []byte) error {
			file, err := archive.Create(name)
			if err != nil {
				return err
			}
			_, err = file.Write(data)
			return err
		}
		if add("LICENSE-MODELS.txt", ocr.ModelLicense) != nil {
			return
		}
		if add("NOTICE.txt", []byte(ocr.Attribution+"\n"+ocr.ModelLicenseURL+"\n")) != nil {
			return
		}
		for index, summary := range job.Files {
			file, err := manager.Result(owner, key, summary.ID)
			if err != nil {
				return
			}
			base := fmt.Sprintf("%02d-%s", index+1, sanitizeUploadFilename(strings.TrimSuffix(file.Name, filepath.Ext(file.Name))))
			data, _ := json.MarshalIndent(file, "", "  ")
			for _, item := range []struct {
				ext  string
				data []byte
			}{{"json", data}, {"txt", []byte(ocrFileText(file, "text"))}, {"md", []byte(ocrFileText(file, "md"))}} {
				if add(base+"."+item.ext, item.data) != nil {
					return
				}
			}
		}
		return
	}
	if format != "text" && format != "md" && format != "json" {
		ocrHTTPError(w, ocr.ErrInvalid)
		return
	}
	file, err := manager.Result(owner, key, fileID)
	if err != nil {
		ocrHTTPError(w, err)
		return
	}
	var data []byte
	extension := format
	if format == "json" {
		data, _ = json.MarshalIndent(map[string]any{"file": file, "attribution": ocr.Attribution, "license_url": ocr.ModelLicenseURL, "license": string(ocr.ModelLicense)}, "", "  ")
	} else {
		if format == "text" {
			extension = "txt"
		}
		data = []byte(ocrFileText(file, format) + "\n" + ocr.Attribution + "\n" + ocr.ModelLicenseURL + "\n")
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", `attachment; filename="ocr-result.`+extension+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}
