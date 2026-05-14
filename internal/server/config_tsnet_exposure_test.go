package server

import (
	"testing"

	"aurago/internal/config"
)

func TestTsNetExposureConfigChangedDetectsManifestExposureToggle(t *testing.T) {
	oldCfg := config.Config{}
	newCfg := oldCfg
	newCfg.Tailscale.TsNet.ExposeManifest = true

	if !tsnetExposureConfigChanged(oldCfg, newCfg) {
		t.Fatal("tsnetExposureConfigChanged() = false, want true for Manifest exposure toggle")
	}
}

func TestTsNetExposureConfigChangedDetectsManifestRuntimeChange(t *testing.T) {
	oldCfg := config.Config{}
	oldCfg.Tailscale.TsNet.ExposeManifest = true
	oldCfg.Manifest.Enabled = true
	oldCfg.Manifest.Port = 2099
	oldCfg.Manifest.HostPort = 2099

	newCfg := oldCfg
	newCfg.Manifest.HostPort = 3109

	if !tsnetExposureConfigChanged(oldCfg, newCfg) {
		t.Fatal("tsnetExposureConfigChanged() = false, want true for Manifest proxy target change")
	}
}

func TestTsNetHasAnyExposureIncludesManifest(t *testing.T) {
	cfg := config.Config{}
	cfg.Tailscale.TsNet.ExposeManifest = true

	if !tsnetHasAnyExposure(cfg) {
		t.Fatal("tsnetHasAnyExposure() = false, want true when Manifest exposure is enabled")
	}
}
