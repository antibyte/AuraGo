package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"aurago/internal/tools"
)

// handleThreeDPrinterWidgetStatus exposes configured names and read-only status.
// An omitted ID lists printers without contacting any device.
func handleThreeDPrinterWidgetStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := tools.BuildThreeDPrinterRuntimeConfig(s.Cfg)
		if !cfg.Enabled {
			jsonError(w, "3D printer integration is disabled", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		id := r.URL.Query().Get("printer_id")
		if id == "" {
			printers := []map[string]string{}
			if cfg.ElegooCentauriCarbon.Enabled {
				for _, p := range cfg.ElegooCentauriCarbon.Printers {
					printers = append(printers, map[string]string{"id": p.ID, "name": p.Name})
				}
			}
			if cfg.Klipper.Enabled {
				for _, p := range cfg.Klipper.Printers {
					printers = append(printers, map[string]string{"id": p.ID, "name": p.Name})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"printers": printers, "default_printer": cfg.DefaultPrinter})
			return
		}
		if _, err := tools.ResolveThreeDPrinter(cfg, id); err != nil {
			jsonError(w, "Printer not found", http.StatusNotFound)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		_, _ = w.Write([]byte(tools.ExecuteThreeDPrinter(ctx, cfg, tools.ThreeDPrinterRequest{Operation: "status", PrinterID: id})))
	}
}
