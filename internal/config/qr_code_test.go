package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQRCodeDefaultsAndExplicitWriteGrant(t *testing.T) {
	for _, tc := range []struct {
		name, yaml        string
		enabled, readOnly bool
	}{
		{"defaults", "server:\n  ui_language: de\n", false, true},
		{"explicit", "tools:\n  qr_code:\n    enabled: true\n    readonly: false\n", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Tools.QRCode.Enabled != tc.enabled || cfg.Tools.QRCode.ReadOnly != tc.readOnly {
				t.Fatalf("QR gates = %+v", cfg.Tools.QRCode)
			}
		})
	}
}
