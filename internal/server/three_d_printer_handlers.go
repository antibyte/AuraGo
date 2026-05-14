package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func handleThreeDPrinterTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		req := tools.ThreeDPrinterRequest{Operation: "test_connection"}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		cfg := buildThreeDPrinterRuntimeConfig(s.Cfg)
		if strings.TrimSpace(req.URL) != "" {
			id := strings.TrimSpace(req.PrinterID)
			if id == "" {
				id = "test-printer"
			}
			cfg.Enabled = true
			cfg.DefaultPrinter = id
			if strings.EqualFold(strings.TrimSpace(req.Protocol), "klipper") {
				cfg.Klipper.Enabled = true
				cfg.Klipper.Printers = []tools.KlipperPrinter{{
					ID:             id,
					Name:           id,
					URL:            req.URL,
					APIKey:         req.APIKey,
					TimeoutSeconds: req.TimeoutSeconds,
					WebcamName:     req.WebcamName,
				}}
			} else {
				cfg.ElegooCentauriCarbon.Enabled = true
				cfg.ElegooCentauriCarbon.Printers = []tools.ElegooCentauriCarbonPrinter{{
					ID:             id,
					Name:           id,
					URL:            req.URL,
					MainboardID:    req.MainboardID,
					TimeoutSeconds: req.TimeoutSeconds,
				}}
			}
			req.PrinterID = id
		}
		_, _ = w.Write([]byte(tools.ExecuteThreeDPrinter(ctx, cfg, req)))
	}
}

func handleThreeDPrinterCameraSnapshot(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		printerID := threeDPrinterIDFromPath(r.URL.Path, "/api/3d-printers/", "/camera/snapshot")
		cfg := buildThreeDPrinterRuntimeConfig(s.Cfg)
		printer, err := tools.ResolveThreeDPrinter(cfg, printerID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		streamURL, snapshotURL, err := tools.ResolveThreeDPrinterCameraURLs(ctx, printer)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := tools.ValidateThreeDPrinterStreamURL(printer.URL, streamURL); err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		fetchURL := streamURL
		if snapshotURL != "" {
			if err := tools.ValidateThreeDPrinterStreamURL(printer.URL, snapshotURL); err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			fetchURL = snapshotURL
		}
		data, contentType, err := tools.FetchThreeDPrinterSnapshot(ctx, fetchURL)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		result, err := tools.StoreThreeDPrinterMedia(s.Cfg.Directories.DataDir, printer.ID, data, contentType)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}

func handleThreeDPrinterCameraStream(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		printerID := threeDPrinterIDFromPath(r.URL.Path, "/api/3d-printers/", "/camera/stream")
		cfg := buildThreeDPrinterRuntimeConfig(s.Cfg)
		printer, err := tools.ResolveThreeDPrinter(cfg, printerID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		streamURL, _, err := tools.ResolveThreeDPrinterCameraURLs(ctx, printer)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := tools.ValidateThreeDPrinterStreamURL(printer.URL, streamURL); err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, streamURL, nil)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		resp, err := (&http.Client{Timeout: 0}).Do(proxyReq)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			jsonError(w, "camera stream returned HTTP "+resp.Status, http.StatusBadGateway)
			return
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "multipart/x-mixed-replace"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, resp.Body)
	}
}

func threeDPrinterIDFromPath(path, prefix, suffix string) string {
	value := strings.TrimPrefix(path, prefix)
	value = strings.TrimSuffix(value, suffix)
	value = strings.Trim(value, "/")
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func buildThreeDPrinterRuntimeConfig(cfg *config.Config) tools.ThreeDPrinterConfig {
	if cfg == nil {
		return tools.ThreeDPrinterConfig{}
	}
	printers := make([]tools.ElegooCentauriCarbonPrinter, 0, len(cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers))
	for _, printer := range cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers {
		printers = append(printers, tools.ElegooCentauriCarbonPrinter{
			ID:             printer.ID,
			Name:           printer.Name,
			URL:            printer.URL,
			MainboardID:    printer.MainboardID,
			TimeoutSeconds: printer.TimeoutSeconds,
		})
	}
	klipperPrinters := make([]tools.KlipperPrinter, 0, len(cfg.ThreeDPrinters.Klipper.Printers))
	for _, printer := range cfg.ThreeDPrinters.Klipper.Printers {
		klipperPrinters = append(klipperPrinters, tools.KlipperPrinter{
			ID:             printer.ID,
			Name:           printer.Name,
			URL:            printer.URL,
			APIKey:         printer.APIKey,
			TimeoutSeconds: printer.TimeoutSeconds,
			WebcamName:     printer.WebcamName,
		})
	}
	return tools.ThreeDPrinterConfig{
		Enabled:        cfg.ThreeDPrinters.Enabled,
		ReadOnly:       cfg.ThreeDPrinters.ReadOnly,
		DefaultPrinter: cfg.ThreeDPrinters.DefaultPrinter,
		DataDir:        cfg.Directories.DataDir,
		ElegooCentauriCarbon: tools.ElegooCentauriCarbonConfig{
			Enabled:  cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled,
			Printers: printers,
		},
		Klipper: tools.KlipperConfig{
			Enabled:  cfg.ThreeDPrinters.Klipper.Enabled,
			Printers: klipperPrinters,
		},
	}
}
