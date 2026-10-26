package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/qrcode"
	"aurago/internal/tools"
)

func handleQR(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data:; sandbox")
		op := strings.TrimPrefix(r.URL.Path, "/api/qr/")
		method := http.MethodPost
		if op == "presets" {
			method = http.MethodGet
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			qrHTTPError(w, 405, "method_not_allowed", "Method not allowed")
			return
		}
		operation, scope := desktopRead, desktopScopeRead
		if op != "presets" && op != "scan" && op != "config" {
			operation, scope = desktopWrite, desktopScopeWrite
		}
		if !requireDesktopOperation(s, w, r, scope, operation) {
			return
		}
		s.CfgMu.RLock()
		enabled := s.Cfg != nil && s.Cfg.VirtualDesktop.Enabled
		network := s.Cfg != nil && s.Cfg.Agent.AllowNetworkRequests
		s.CfgMu.RUnlock()
		if !enabled {
			qrHTTPError(w, 403, "desktop_disabled", "Desktop is disabled")
			return
		}
		if op == "presets" {
			qrHTTPJSON(w, qrcode.Presets())
			return
		}
		if op == "import-image" && !network {
			qrHTTPError(w, 403, "network_disabled", "Network requests are disabled")
			return
		}
		release, admitted := tools.AcquireQRWork()
		if !admitted {
			qrHTTPError(w, 429, "qr_busy", "QR processing is busy; try again")
			return
		}
		defer release()
		switch op {
		case "config":
			r.Body = http.MaxBytesReader(w, r.Body, qrcode.MaxResourceBytes)
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				qrHTTPError(w, 400, "invalid_spec", "Invalid or oversized QR configuration")
				return
			}
			spec, err := qrcode.ParseConfig(r.Context(), raw)
			if err != nil {
				qrHTTPError(w, 400, "invalid_spec", "Invalid or oversized QR configuration")
				return
			}
			data, err := qrcode.ExportConfig(r.Context(), spec)
			if err != nil {
				qrHTTPError(w, 422, "invalid_spec", err.Error())
				return
			}
			qrHTTPBytes(w, "application/json", "qr.qr.json", data)
		case "render":
			var spec qrcode.QRSpec
			if err := decodeQRJSON(w, r, &spec, 32<<20); err != nil {
				qrHTTPError(w, 400, "invalid_spec", "Invalid or oversized QR configuration")
				return
			}
			result, err := qrcode.Render(r.Context(), spec, tools.QRRenderOptions())
			if err != nil {
				qrHTTPError(w, 422, "invalid_qr", err.Error())
				return
			}
			metadata, _ := json.Marshal(result.Metadata)
			if len(metadata) < 8192 {
				w.Header().Set("X-QR-Metadata", string(metadata))
			}
			w.Header().Set("X-QR-Width", strconv.Itoa(result.Metadata.Width))
			w.Header().Set("X-QR-Height", strconv.Itoa(result.Metadata.Height))
			w.Header().Set("X-QR-Verified", strconv.FormatBool(result.Metadata.Verified))
			qrHTTPBytes(w, result.Metadata.MIME, "qr."+result.Metadata.Extension, result.Data)
		case "scan":
			r.Body = http.MaxBytesReader(w, r.Body, qrcode.MaxResourceBytes)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				qrHTTPError(w, 413, "image_too_large", "Image is too large or incomplete")
				return
			}
			result, err := qrcode.Decode(r.Context(), data)
			if err != nil {
				qrHTTPError(w, 422, "qr_not_found", err.Error())
				return
			}
			qrHTTPJSON(w, result)
		case "batch":
			var req struct {
				Spec        qrcode.QRSpec `json:"spec"`
				CSV         string        `json:"csv"`
				DryRun      bool          `json:"dry_run"`
				ContentType string        `json:"content_type"`
			}
			if err := decodeQRJSON(w, r, &req, 40<<20); err != nil {
				qrHTTPError(w, 400, "invalid_batch", "Invalid or oversized batch")
				return
			}
			kind := req.ContentType
			if kind == "" {
				kind = req.Spec.Content.Type
			}
			var result qrcode.BatchResult
			var err error
			if req.DryRun {
				result, err = qrcode.ValidateBatch(r.Context(), []byte(req.CSV), req.Spec, qrcode.BatchOptions{ContentType: kind}, tools.QRRenderOptions())
			} else {
				result, err = qrcode.Batch(r.Context(), []byte(req.CSV), req.Spec, qrcode.BatchOptions{ContentType: kind}, tools.QRRenderOptions())
			}
			if !req.DryRun && len(result.Errors) > 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Batch validation failed", "code": "invalid_batch", "details": result})
				return
			}
			if err != nil {
				qrHTTPError(w, 422, "invalid_batch", err.Error())
				return
			}
			if req.DryRun || len(result.Errors) > 0 {
				qrHTTPJSON(w, result)
				return
			}
			qrHTTPBytes(w, "application/zip", "qr-batch.zip", result.Data)
		case "import-image":
			var req struct {
				URL string `json:"url"`
			}
			if err := decodeQRJSON(w, r, &req, 8192); err != nil {
				qrHTTPError(w, 400, "invalid_image_url", "Invalid image URL")
				return
			}
			result, err := tools.ImportQRImage(r.Context(), req.URL)
			if err != nil {
				qrHTTPError(w, 422, "image_import_failed", err.Error())
				return
			}
			qrHTTPJSON(w, result)
		default:
			qrHTTPError(w, 404, "not_found", "Unknown QR operation")
		}
	}
}

func decodeQRJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON object")
	}
	return nil
}

func qrHTTPJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func qrHTTPError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
func qrHTTPBytes(w http.ResponseWriter, mime, filename string, data []byte) {
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
