package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"strings"

	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

func chromecastAdvertiseHost(cfg *config.Config) (string, error) {
	explicit := ""
	if cfg != nil {
		explicit = cfg.Chromecast.AdvertiseHost
	}
	fallback := ""
	if strings.TrimSpace(explicit) == "" {
		fallback = getLocalIP(cfg)
	}
	return tools.ChromecastAdvertiseHost(explicit, fallback)
}

func chromecastControlConfig(ctx context.Context, cfg *config.Config, host string, inventoryDB *sql.DB, logger *slog.Logger) tools.ChromecastConfig {
	ccCfg := tools.ChromecastConfig{
		Context:      ctx,
		ServerHost:   host,
		KnownDevices: chromecastKnownDevices(inventoryDB, logger),
	}
	if cfg == nil {
		return ccCfg
	}
	ccCfg.ServerPort = cfg.Chromecast.TTSPort
	if ccCfg.ServerPort == 0 {
		ccCfg.ServerPort = cfg.Server.Port
	}
	ccCfg.MediaHostAllowlist = cfg.Chromecast.MediaHostAllowlist
	ccCfg.ReadOnly = cfg.Chromecast.ReadOnly
	return ccCfg
}

func chromecastKnownDevices(inventoryDB *sql.DB, logger *slog.Logger) []tools.ChromecastDevice {
	var records []inventory.DeviceRecord
	if inventoryDB != nil {
		found, err := inventory.QueryDevices(inventoryDB, "", "chromecast", "")
		if err != nil {
			if logger != nil {
				logger.Warn("chromecast inventory lookup failed", "error", err)
			}
		} else {
			records = found
		}
	}
	discovered, err := discoverChromecastDevices(logger)
	if err != nil {
		discovered = nil
	}
	return mergeChromecastKnownDevices(records, discovered)
}

func mergeChromecastKnownDevices(records []inventory.DeviceRecord, discovered []tools.ChromecastDevice) []tools.ChromecastDevice {
	order := make([]string, 0, len(records)+len(discovered))
	byIP := make(map[string]tools.ChromecastDevice, len(records)+len(discovered))
	add := func(device tools.ChromecastDevice) {
		ip := net.ParseIP(strings.TrimSpace(device.Addr))
		if ip == nil {
			return
		}
		key := ip.String()
		device.Addr = key
		existing, ok := byIP[key]
		if !ok {
			order = append(order, key)
			byIP[key] = device
			return
		}
		if device.Group {
			existing.Group = true
		}
		if device.AudioOnly {
			existing.AudioOnly = true
		}
		if existing.Name == "" {
			existing.Name = device.Name
		}
		if existing.FriendlyName == "" {
			existing.FriendlyName = device.FriendlyName
		}
		if existing.UUID == "" {
			existing.UUID = device.UUID
		}
		if existing.Model == "" {
			existing.Model = device.Model
		}
		if existing.Capabilities == "" {
			existing.Capabilities = device.Capabilities
		}
		if existing.Port == 0 {
			existing.Port = device.Port
		}
		byIP[key] = existing
	}
	for _, record := range records {
		add(tools.ChromecastDevice{Name: record.Name, Addr: record.IPAddress, Port: record.Port})
	}
	for _, device := range discovered {
		add(device)
	}
	out := make([]tools.ChromecastDevice, 0, len(order))
	for _, key := range order {
		out = append(out, byIP[key])
	}
	return out
}

// ChromecastTestAnnouncement speaks a fixed sentence on one inventory device.
func ChromecastTestAnnouncement(cfg *config.Config, addr string, port int, inventoryDB *sql.DB, logger *slog.Logger) string {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg == nil || !cfg.Chromecast.Enabled || cfg.Chromecast.ReadOnly {
		data, _ := json.Marshal(map[string]string{
			"status":  "error",
			"message": "Chromecast test is available only when Chromecast is enabled and not read-only",
		})
		return string(data)
	}
	host, err := chromecastAdvertiseHost(cfg)
	if err != nil {
		data, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
		return string(data)
	}
	ccCfg := chromecastControlConfig(context.Background(), cfg, host, inventoryDB, logger)
	ttsCfg := buildRuntimeTTSConfig(cfg, "")
	return tools.ChromecastSpeak(addr, port, "AuraGo Test.", ttsCfg, ccCfg, logger)
}
