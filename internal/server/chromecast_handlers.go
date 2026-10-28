package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

// handleChromecastDiscover triggers an mDNS scan for Chromecast devices and
// returns the discovered devices as JSON.
// GET /api/chromecast/discover
func handleChromecastDiscover(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		raw := tools.ChromecastDiscover(s.Logger)

		// Forward the JSON string from ChromecastDiscover as-is
		w.Header().Set("Content-Type", "application/json")

		// Validate that it's proper JSON before sending
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			jsonError(w, "discovery returned invalid data", http.StatusInternalServerError)
			return
		}

		w.Write([]byte(raw))
	}
}

var runChromecastTestAnnouncement = agent.ChromecastTestAnnouncement

// handleChromecastTest speaks a fixed sentence on one saved Chromecast device.
// POST /api/chromecast/test
func handleChromecastTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg
		enabled := cfg != nil && cfg.Chromecast.Enabled
		readOnly := cfg != nil && cfg.Chromecast.ReadOnly
		s.CfgMu.RUnlock()
		if !enabled || readOnly {
			jsonError(w, "Chromecast test is available only when Chromecast is enabled and not read-only", http.StatusConflict)
			return
		}
		device, ok := chromecastInventoryDevice(s, strings.TrimSpace(body.DeviceID))
		if !ok {
			jsonError(w, "device not found", http.StatusNotFound)
			return
		}
		raw := runChromecastTestAnnouncement(cfg, device.IPAddress, device.Port, s.InventoryDB, s.Logger)
		writeChromecastToolResult(w, raw)
	}
}

func chromecastInventoryDevice(s *Server, id string) (inventory.DeviceRecord, bool) {
	if s == nil || s.InventoryDB == nil || id == "" {
		return inventory.DeviceRecord{}, false
	}
	devices, err := inventory.QueryDevices(s.InventoryDB, "", "chromecast", "")
	if err != nil {
		return inventory.DeviceRecord{}, false
	}
	for _, device := range devices {
		if device.ID == id && strings.EqualFold(device.Type, "chromecast") {
			return device, true
		}
	}
	return inventory.DeviceRecord{}, false
}

func writeChromecastToolResult(w http.ResponseWriter, raw string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		jsonError(w, "test returned invalid data", http.StatusInternalServerError)
		return
	}
	code := http.StatusOK
	if status, _ := parsed["status"].(string); status != "success" {
		code = http.StatusBadGateway
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(raw))
}
