package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"aurago/internal/fileutil"
)

var configFileOverride string

func serviceConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "AuraGo", "Remote", "config.json")
	case "linux":
		return "/var/lib/aurago-remote/config.json"
	default:
		base, _ := os.UserHomeDir()
		return filepath.Join(base, "Library", "Application Support", "AuraGo", "Remote", "config.json")
	}
}

func readStoredConfig(path string) (*clientConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg clientConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid client configuration: %w", err)
	}
	return &cfg, nil
}

func saveConfig(cfg clientConfig) error {
	path := configPath()
	dir := filepath.Dir(path)
	if err := secureConfigDirectory(dir); err != nil {
		return fmt.Errorf("protect configuration directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileContext(context.Background(), path, data, 0600)
}

func prepareServiceConfig(cfg clientConfig, explicitSupervisor, explicitToken bool) error {
	target := configFileOverride
	if target == "" {
		target = serviceConfigPath()
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	previous, err := readStoredConfig(target)
	if err != nil {
		return err
	}
	if previous != nil {
		if cfg.SupervisorURL == "" {
			cfg.SupervisorURL = previous.SupervisorURL
		}
		if cfg.DeviceName == "" {
			cfg.DeviceName = previous.DeviceName
		}
		if previous.DeviceID != "" && previous.SharedKey != "" {
			cfg.DeviceID, cfg.SharedKey = previous.DeviceID, previous.SharedKey
			if !explicitToken {
				// Reinstalling a personalized download must not replace an existing identity.
				cfg.EnrollToken = previous.EnrollToken
				if !explicitSupervisor {
					cfg.SupervisorURL = previous.SupervisorURL
					if cfg.EnrollToken != "" && previous.EnrollmentURL != "" {
						cfg.SupervisorURL = previous.EnrollmentURL
					}
				}
			}
		}
	}
	u, err := url.Parse(cfg.SupervisorURL)
	if err != nil || u.Scheme != "wss" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("WSS_REQUIRED: configure a valid WSS supervisor before installing")
	}
	if previous != nil && previous.DeviceID != "" && cfg.EnrollToken != "" {
		// Keep the old endpoint with its identity until replacement enrollment succeeds.
		cfg.EnrollmentURL, cfg.SupervisorURL = cfg.SupervisorURL, previous.SupervisorURL
	}
	configFileOverride = target
	return saveConfig(cfg)
}
