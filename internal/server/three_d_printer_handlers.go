package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/tools"
)

var threeDPrinterStreamHTTPClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

func handleThreeDPrinterTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		req := tools.ThreeDPrinterRequest{Operation: "test_connection"}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		cfg := tools.BuildThreeDPrinterRuntimeConfig(s.Cfg)
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
					APIKey:         klipperTestAPIKey(req, cfg),
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

func klipperTestAPIKey(req tools.ThreeDPrinterRequest, cfg tools.ThreeDPrinterConfig) string {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey != maskedKey {
		return req.APIKey
	}
	reqURL := strings.TrimSpace(req.URL)
	if reqURL == "" || !strings.EqualFold(strings.TrimSpace(req.Protocol), "klipper") {
		return ""
	}
	id := strings.TrimSpace(req.PrinterID)
	if id == "" {
		return ""
	}
	for _, printer := range cfg.Klipper.Printers {
		if !strings.EqualFold(strings.TrimSpace(printer.ID), id) && !strings.EqualFold(strings.TrimSpace(printer.Name), id) {
			continue
		}
		if strings.TrimSpace(printer.URL) == reqURL {
			return printer.APIKey
		}
	}
	return ""
}

func handleThreeDPrinterCameraSnapshot(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		printerID := threeDPrinterIDFromPath(r.URL.Path, "/api/3d-printers/", "/camera/snapshot")
		cfg := tools.BuildThreeDPrinterRuntimeConfig(s.Cfg)
		if !cfg.Enabled {
			jsonError(w, "3D printer integration is disabled", http.StatusForbidden)
			return
		}
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
		fetchURL := strings.TrimSpace(streamURL)
		if snapshotURL != "" {
			fetchURL = strings.TrimSpace(snapshotURL)
		}
		if fetchURL == "" {
			jsonError(w, "camera snapshot URL was not found", http.StatusBadGateway)
			return
		}
		if err := tools.ValidateThreeDPrinterStreamURL(printer.URL, fetchURL); err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		data, contentType, err := tools.FetchThreeDPrinterSnapshot(ctx, fetchURL)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		result, err := tools.StoreThreeDPrinterMedia(s.Cfg.Directories.DataDir, s.MediaRegistryDB, printer.ID, data, contentType)
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
		cfg := tools.BuildThreeDPrinterRuntimeConfig(s.Cfg)
		if !cfg.Enabled {
			jsonError(w, "3D printer integration is disabled", http.StatusForbidden)
			return
		}
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
		if strings.TrimSpace(streamURL) == "" {
			jsonError(w, "camera stream URL was not found", http.StatusBadGateway)
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
		resp, err := threeDPrinterStreamHTTPClient.Do(proxyReq)
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
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
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
