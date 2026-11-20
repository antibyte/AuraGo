package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVirtualComputersVolumeRetentionDefaultsAndClamp(t *testing.T) {
	load := func(storage string) VirtualComputersStorage {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\nvirtual_computers:\n  storage:\n"+storage), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		return cfg.VirtualComputers.Storage
	}
	for name, tc := range map[string]struct {
		yaml           string
		quota, ttlDays int
	}{
		"defaults":     {"    mode: managed_garage\n", 2048, 30},
		"explicit":     {"    volume_quota_mb: 512\n    volume_max_ttl_days: 7\n", 512, 7},
		"zero":         {"    volume_quota_mb: 0\n    volume_max_ttl_days: 0\n", 2048, 30},
		"negative":     {"    volume_quota_mb: -5\n    volume_max_ttl_days: -1\n", 2048, 30},
		"clamped low":  {"    volume_quota_mb: 8\n", 64, 30},
		"clamped high": {"    volume_quota_mb: 999999\n    volume_max_ttl_days: 4000\n", 65536, 365},
	} {
		got := load(tc.yaml)
		if got.VolumeQuotaMB != tc.quota || got.VolumeMaxTTLDays != tc.ttlDays {
			t.Errorf("%s: quota=%d ttl_days=%d, want %d/%d", name, got.VolumeQuotaMB, got.VolumeMaxTTLDays, tc.quota, tc.ttlDays)
		}
	}
	if VirtualComputersDefaultVolumeQuotaMB != 2048 || VirtualComputersDefaultVolumeMaxTTLDays != 30 {
		t.Fatal("exported defaults changed")
	}
}
