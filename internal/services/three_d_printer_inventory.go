package services

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/inventory"
)

// SyncThreeDPrinterDevices ensures configured 3D printers appear in the device registry.
func SyncThreeDPrinterDevices(db *sql.DB, cfg config.ThreeDPrintersConfig) (created int, updated int, err error) {
	if db == nil {
		return 0, 0, nil
	}
	devices, err := inventory.ListAllDevices(db)
	if err != nil {
		return 0, 0, fmt.Errorf("list devices: %w", err)
	}
	byName := make(map[string]inventory.DeviceRecord, len(devices))
	for _, device := range devices {
		name := strings.ToLower(strings.TrimSpace(device.Name))
		if name != "" {
			byName[name] = device
		}
	}

	for _, printer := range cfg.ElegooCentauriCarbon.Printers {
		record, ok := elegooPrinterDeviceRecord(printer)
		if !ok {
			continue
		}
		synced, c, u, syncErr := syncThreeDPrinterDevice(db, byName, record)
		if syncErr != nil {
			return created, updated, syncErr
		}
		if c {
			created++
		}
		if u {
			updated++
		}
		byName[strings.ToLower(synced.Name)] = synced
	}
	for _, printer := range cfg.Klipper.Printers {
		record, ok := klipperPrinterDeviceRecord(printer)
		if !ok {
			continue
		}
		synced, c, u, syncErr := syncThreeDPrinterDevice(db, byName, record)
		if syncErr != nil {
			return created, updated, syncErr
		}
		if c {
			created++
		}
		if u {
			updated++
		}
		byName[strings.ToLower(synced.Name)] = synced
	}
	return created, updated, nil
}

func elegooPrinterDeviceRecord(printer config.ElegooCentauriCarbonPrinterConfig) (inventory.DeviceRecord, bool) {
	name := printerDeviceName(printer.Name, printer.ID)
	host, port, ok := printerEndpoint(printer.URL)
	if name == "" || !ok {
		return inventory.DeviceRecord{}, false
	}
	return inventory.DeviceRecord{
		Name:        name,
		Type:        "printer",
		Protocol:    inventory.ProtocolNone,
		IPAddress:   host,
		Port:        port,
		Description: "3D printer (Elegoo Centauri Carbon / SDCP)",
		Tags:        []string{"3d-printer", "elegoo-centauri-carbon"},
	}, true
}

func klipperPrinterDeviceRecord(printer config.KlipperPrinterConfig) (inventory.DeviceRecord, bool) {
	name := printerDeviceName(printer.Name, printer.ID)
	host, port, ok := printerEndpoint(printer.URL)
	if name == "" || !ok {
		return inventory.DeviceRecord{}, false
	}
	return inventory.DeviceRecord{
		Name:        name,
		Type:        "printer",
		Protocol:    inventory.ProtocolNone,
		IPAddress:   host,
		Port:        port,
		Description: "3D printer (Klipper / Moonraker)",
		Tags:        []string{"3d-printer", "klipper"},
	}, true
}

func syncThreeDPrinterDevice(db *sql.DB, byName map[string]inventory.DeviceRecord, record inventory.DeviceRecord) (inventory.DeviceRecord, bool, bool, error) {
	key := strings.ToLower(strings.TrimSpace(record.Name))
	existing, exists := byName[key]
	if !exists {
		id, err := inventory.CreateDevice(db, record.Name, record.Type, record.Protocol, record.IPAddress, record.Port, record.Username, record.VaultSecretID, record.CredentialID, record.Description, record.Tags, record.MACAddress)
		if err != nil {
			return inventory.DeviceRecord{}, false, false, fmt.Errorf("create 3D printer device %q: %w", record.Name, err)
		}
		record.ID = id
		return record, true, false, nil
	}
	if existing.Type != "printer" || !hasTag(existing.Tags, "3d-printer") {
		return existing, false, false, nil
	}
	merged := existing
	merged.Type = "printer"
	merged.Protocol = record.Protocol
	merged.IPAddress = record.IPAddress
	merged.Port = record.Port
	merged.Description = record.Description
	merged.Tags = mergeTags(existing.Tags, record.Tags)
	if sameDeviceRecord(existing, merged) {
		return existing, false, false, nil
	}
	if err := inventory.UpdateDevice(db, merged); err != nil {
		return inventory.DeviceRecord{}, false, false, fmt.Errorf("update 3D printer device %q: %w", record.Name, err)
	}
	return merged, false, true, nil
}

func printerDeviceName(name, id string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(id)
}

func printerEndpoint(raw string) (host string, port int, ok bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return "", 0, false
	}
	port = defaultPortForScheme(parsed.Scheme)
	if rawPort := parsed.Port(); rawPort != "" {
		if parsedPort, err := strconv.Atoi(rawPort); err == nil && parsedPort > 0 {
			port = parsedPort
		}
	}
	return parsed.Hostname(), port, port > 0
}

func defaultPortForScheme(scheme string) int {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http", "ws":
		return 80
	case "https", "wss":
		return 443
	default:
		return 0
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), want) {
			return true
		}
	}
	return false
}

func mergeTags(existing []string, required []string) []string {
	seen := make(map[string]bool, len(existing)+len(required))
	out := make([]string, 0, len(existing)+len(required))
	for _, tag := range append(existing, required...) {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func sameDeviceRecord(a, b inventory.DeviceRecord) bool {
	if a.Type != b.Type || a.Protocol != b.Protocol || a.IPAddress != b.IPAddress || a.Port != b.Port || a.Description != b.Description {
		return false
	}
	if len(a.Tags) != len(b.Tags) {
		return false
	}
	for i := range a.Tags {
		if a.Tags[i] != b.Tags[i] {
			return false
		}
	}
	return true
}
