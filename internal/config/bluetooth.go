package config

import (
	"time"

	"aurago/internal/bluetooth"
)

// BluetoothRuntimeOptions translates persisted settings into runtime-only
// Bluetooth service options.
func BluetoothRuntimeOptions(cfg *Config) bluetooth.Options {
	if cfg == nil {
		return bluetooth.Options{}
	}
	return bluetooth.Options{
		Enabled:            cfg.Bluetooth.Enabled,
		ReadOnly:           cfg.Bluetooth.ReadOnly,
		AllowPlayback:      cfg.Bluetooth.AllowPlayback,
		ScanTimeout:        time.Duration(cfg.Bluetooth.ScanTimeoutSeconds) * time.Second,
		DefaultDevice:      cfg.Bluetooth.DefaultDevice,
		AudioBackend:       cfg.Bluetooth.AudioBackend,
		IsDocker:           cfg.Runtime.IsDocker,
		WorkspaceDirectory: cfg.Directories.WorkspaceDir,
		DataDirectory:      cfg.Directories.DataDir,
	}
}
