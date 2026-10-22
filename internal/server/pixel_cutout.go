package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"log/slog"
	"net/http"
	"sync"

	"aurago/internal/pixelmatte"
)

// pixelCutoutBodyLimit bounds the cutout request like the other Pixel image
// endpoints: a 32 MiB image as base64 plus the JSON envelope.
const pixelCutoutBodyLimit = int64(44 << 20)

// pixelCutoutManagers holds the matting manager of each server: the manager
// owns the single worker slot and the single-flight downloads, so every
// handler of one server must see the same one. Tests store fakes here.
var pixelCutoutManagers sync.Map // *Server -> *pixelmatte.Manager

func pixelCutoutManager(s *Server) *pixelmatte.Manager {
	if existing, ok := pixelCutoutManagers.Load(s); ok {
		return existing.(*pixelmatte.Manager)
	}
	dataDir := ""
	if s.Cfg != nil {
		s.CfgMu.RLock()
		dataDir = s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()
	}
	manager := pixelmatte.NewManager(pixelmatte.Options{DataDir: dataDir, Logger: s.Logger})
	actual, _ := pixelCutoutManagers.LoadOrStore(s, manager)
	return actual.(*pixelmatte.Manager)
}

// pixelCutoutLifetime is the context of background downloads: it ends when
// the server shuts down, not when the request that started the download does.
func pixelCutoutLifetime(s *Server) context.Context {
	if s.integrationCtx != nil {
		return s.integrationCtx
	}
	return context.Background()
}

func pixelCutoutLogger(s *Server) *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// pixelCutoutWrite sends a JSON body with the given status code.
func pixelCutoutWrite(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// pixelCutoutFail sends {"status": status, "error": message}; the error key
// is the one jsonError uses, status tells the client what to do (not_ready:
// prepare the model, busy: retry later).
func pixelCutoutFail(w http.ResponseWriter, code int, status, message string) {
	pixelCutoutWrite(w, code, map[string]string{"status": status, "error": message})
}

func pixelCutoutMethod(w http.ResponseWriter, method string) {
	w.Header().Set("Allow", method)
	pixelCutoutFail(w, http.StatusMethodNotAllowed, "error", "Method not allowed")
}

// handlePixelCutoutStatus returns GET /api/pixel/cutout/status: whether the
// local AI cutout can run here, whether the ONNX Runtime is present, whether a
// cutout is running and the state of every model. It never downloads.
func handlePixelCutoutStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
			return
		}
		if r.Method != http.MethodGet {
			pixelCutoutMethod(w, http.MethodGet)
			return
		}
		pixelCutoutWrite(w, http.StatusOK, pixelCutoutManager(s).Status())
	}
}

// handlePixelCutoutPrepare returns POST /api/pixel/cutout/prepare: starts the
// background download of the ONNX Runtime and the model (explicit request
// only; one download per model, repeated calls join it).
func handlePixelCutoutPrepare(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopExecute) {
			return
		}
		if r.Method != http.MethodPost {
			pixelCutoutMethod(w, http.MethodPost)
			return
		}
		var req struct {
			Model string `json:"model"`
		}
		if err := decodeDesktopJSON(w, r, &req, desktopSmallJSONBodyLimit); err != nil || !pixelmatte.ValidKind(req.Model) {
			pixelCutoutFail(w, http.StatusBadRequest, "error", "model must be standard or precise")
			return
		}
		state, err := pixelCutoutManager(s).Prepare(pixelCutoutLifetime(s), pixelmatte.Kind(req.Model))
		switch {
		case errors.Is(err, pixelmatte.ErrUnavailable):
			pixelCutoutFail(w, http.StatusConflict, "unavailable", "AI cutout is not available on this platform")
		case err != nil:
			pixelCutoutFail(w, http.StatusBadRequest, "error", "model must be standard or precise")
		case state == pixelmatte.PrepareReady:
			pixelCutoutWrite(w, http.StatusOK, map[string]string{"status": string(pixelmatte.PrepareReady)})
		default:
			pixelCutoutWrite(w, http.StatusAccepted, map[string]string{"status": string(pixelmatte.PrepareDownloading)})
		}
	}
}

// handlePixelCutout returns POST /api/pixel/cutout: runs the local matting
// model on the posted image and returns the matte (8-bit grayscale PNG of the
// same size) as a data URL. Image data is never logged.
func handlePixelCutout(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopExecute) {
			return
		}
		if r.Method != http.MethodPost {
			pixelCutoutMethod(w, http.MethodPost)
			return
		}
		// The single worker slot is checked before anything is read or decoded for this request, so a queue of
		// requests behind a running cutout costs nothing. Manager.Cutout stays the authority when it takes the slot.
		manager := pixelCutoutManager(s)
		if manager.Busy() {
			pixelCutoutFail(w, http.StatusTooManyRequests, "busy", "Another cutout is running")
			return
		}
		var req struct {
			Model      string `json:"model"`
			SourceData string `json:"source_data"`
		}
		if err := decodeDesktopJSON(w, r, &req, pixelCutoutBodyLimit); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				pixelCutoutFail(w, http.StatusRequestEntityTooLarge, "error", "Image request is too large")
				return
			}
			pixelCutoutFail(w, http.StatusBadRequest, "error", "invalid request")
			return
		}
		if !pixelmatte.ValidKind(req.Model) {
			pixelCutoutFail(w, http.StatusBadRequest, "error", "model must be standard or precise")
			return
		}
		if req.SourceData == "" {
			pixelCutoutFail(w, http.StatusBadRequest, "error", "source_data is required")
			return
		}
		data, err := pixelDecodeDataURL(req.SourceData)
		if err != nil {
			pixelCutoutFail(w, http.StatusBadRequest, "error", "Invalid image encoding")
			return
		}
		// Header only: the worker subprocess decodes the pixels in its own process, so the server never holds the
		// decoded bitmap (up to 128 MB) of a request that may still be turned away.
		source, _, err := pixelValidateImageHeader(data, pixelmatte.MaxImageBytes)
		if err != nil {
			pixelCutoutFail(w, http.StatusBadRequest, "error", err.Error())
			return
		}

		result, err := manager.Cutout(r.Context(), pixelmatte.Kind(req.Model), data)
		switch {
		case err == nil:
		case errors.Is(err, pixelmatte.ErrNotReady):
			pixelCutoutFail(w, http.StatusConflict, "not_ready", "The AI cutout model is not downloaded yet")
			return
		case errors.Is(err, pixelmatte.ErrUnavailable):
			pixelCutoutFail(w, http.StatusConflict, "unavailable", "AI cutout is not available on this platform")
			return
		case errors.Is(err, pixelmatte.ErrBusy):
			pixelCutoutFail(w, http.StatusTooManyRequests, "busy", "Another cutout is running")
			return
		case errors.Is(err, pixelmatte.ErrTimeout):
			pixelCutoutFail(w, http.StatusGatewayTimeout, "timeout", "The cutout took too long")
			return
		case r.Context().Err() != nil:
			return // the client went away or the desktop run was revoked
		default:
			pixelCutoutLogger(s).Error("Pixel AI cutout failed", "model", req.Model, "error", err)
			pixelCutoutFail(w, http.StatusInternalServerError, "error", "Background matting failed")
			return
		}

		matte, _, err := image.DecodeConfig(bytes.NewReader(result.PNG))
		if err != nil || matte.Width != source.Width || matte.Height != source.Height {
			pixelCutoutLogger(s).Error("Pixel AI cutout returned an unusable matte", "model", req.Model, "decode_error", err)
			pixelCutoutFail(w, http.StatusInternalServerError, "error", "Background matting failed")
			return
		}
		pixelCutoutWrite(w, http.StatusOK, map[string]any{
			"status": "ok",
			"mask":   "data:image/png;base64," + base64.StdEncoding.EncodeToString(result.PNG),
			"width":  matte.Width,
			"height": matte.Height,
			"ms":     result.Duration.Milliseconds(),
		})
	}
}
