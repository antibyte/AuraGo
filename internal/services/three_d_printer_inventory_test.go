package services

import (
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/inventory"
)

func TestSyncThreeDPrinterDevicesCreatesAndUpdatesAutoEntry(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()

	cfg := config.ThreeDPrintersConfig{
		Klipper: config.KlipperConfig{
			Printers: []config.KlipperPrinterConfig{{
				ID:   "voron",
				Name: "Voron 2.4",
				URL:  "http://192.168.6.60:7125",
			}},
		},
	}
	created, updated, err := SyncThreeDPrinterDevices(db, cfg)
	if err != nil {
		t.Fatalf("SyncThreeDPrinterDevices() error = %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("created=%d updated=%d, want 1/0", created, updated)
	}

	cfg.Klipper.Printers[0].URL = "http://192.168.6.61:7125"
	created, updated, err = SyncThreeDPrinterDevices(db, cfg)
	if err != nil {
		t.Fatalf("second SyncThreeDPrinterDevices() error = %v", err)
	}
	if created != 0 || updated != 1 {
		t.Fatalf("created=%d updated=%d, want 0/1", created, updated)
	}
	devices, err := inventory.QueryDevices(db, "3d-printer", "printer", "Voron 2.4")
	if err != nil {
		t.Fatalf("QueryDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices=%d, want 1: %#v", len(devices), devices)
	}
	if devices[0].IPAddress != "192.168.6.61" {
		t.Fatalf("IPAddress=%q, want updated URL host", devices[0].IPAddress)
	}
	if devices[0].Protocol != "none" {
		t.Fatalf("Protocol=%q, want none", devices[0].Protocol)
	}
}

func TestSyncThreeDPrinterDevicesDoesNotOverwriteManualSameNameDevice(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	if _, err := inventory.CreateDevice(db, "Shared Name", "server", "ssh", "10.0.0.5", 22, "", "", "", "manual device", []string{"manual"}, ""); err != nil {
		t.Fatalf("CreateDevice() error = %v", err)
	}

	cfg := config.ThreeDPrintersConfig{
		ElegooCentauriCarbon: config.ElegooCentauriCarbonConfig{
			Printers: []config.ElegooCentauriCarbonPrinterConfig{{
				ID:   "elegoo",
				Name: "Shared Name",
				URL:  "ws://192.168.6.50/websocket",
			}},
		},
	}
	created, updated, err := SyncThreeDPrinterDevices(db, cfg)
	if err != nil {
		t.Fatalf("SyncThreeDPrinterDevices() error = %v", err)
	}
	if created != 0 || updated != 0 {
		t.Fatalf("created=%d updated=%d, want 0/0", created, updated)
	}
	devices, err := inventory.QueryDevices(db, "", "", "Shared Name")
	if err != nil {
		t.Fatalf("QueryDevices() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Type != "server" || devices[0].IPAddress != "10.0.0.5" {
		t.Fatalf("manual device was overwritten: %#v", devices)
	}
}
