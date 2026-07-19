package config

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/bluetooth"
)

func TestBluetoothConfigDefaultsAndFeatureAvailability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("agent:\n  system_language: Deutsch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Bluetooth.Enabled || !cfg.Bluetooth.ReadOnly || cfg.Bluetooth.AllowPlayback {
		t.Fatalf("Bluetooth safe defaults = %+v", cfg.Bluetooth)
	}
	if cfg.Bluetooth.ScanTimeoutSeconds != 10 || cfg.Bluetooth.AudioBackend != "auto" {
		t.Fatalf("Bluetooth runtime defaults = %+v", cfg.Bluetooth)
	}

	cfg.Runtime.Bluetooth = bluetooth.Status{
		Usable: true,
		Audio:  bluetooth.AudioStatus{Usable: true, Backend: "pipewire"},
	}
	availability := ComputeFeatureAvailability(cfg.Runtime, false)
	if !availability["bluetooth"].Available || !availability["bluetooth_audio"].Available {
		t.Fatalf("Bluetooth availability = %+v", availability)
	}
	options := BluetoothRuntimeOptions(cfg)
	if !options.Enabled || !options.ReadOnly || options.AudioBackend != "auto" {
		t.Fatalf("Bluetooth runtime options = %+v", options)
	}
}
