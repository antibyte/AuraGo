package server

import (
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

// The server and the egg config generator (which pins the master certificate)
// must agree on when the master serves its self-signed certificate. The exact
// cert_mode values keep their behaviour; others are trimmed and lower-cased.
func TestNewTLSConfigFromConfigMatchesSharedCertModeHelpers(t *testing.T) {
	dataDir := filepath.Join("srv", "aurago", "data")
	for _, tc := range []struct {
		mode, domain string
		wantMode     TLSMode
		wantHTTPPort int
	}{
		{"custom", "aurago.example.com", TLSModeCustom, 80},
		{"custom", "", TLSModeCustom, 0},
		{" Custom ", "", TLSModeCustom, 0},
		{"selfsigned", "aurago.example.com", TLSModeSelfSigned, 80},
		{"selfsigned", "", TLSModeSelfSigned, 0},
		{" SelfSigned ", "aurago.example.com", TLSModeSelfSigned, 80},
		{"auto", "aurago.example.com", TLSModeAuto, 80},
		{"", "aurago.example.com", TLSModeAuto, 80},
		{"auto", "", TLSModeSelfSigned, 0},
		{"", "", TLSModeSelfSigned, 0},
		{"unknown", "", TLSModeSelfSigned, 0},
	} {
		cfg := &config.Config{}
		cfg.Server.HTTPS.Enabled = true
		cfg.Server.HTTPS.CertMode = tc.mode
		cfg.Server.HTTPS.Domain = tc.domain
		cfg.Server.HTTPS.HTTPPort = 80
		cfg.Server.HTTPS.HTTPSPort = 443
		got := NewTLSConfigFromConfig(cfg, dataDir)
		if got.Mode != tc.wantMode || got.HTTPPort != tc.wantHTTPPort || got.HTTPSPort != 443 {
			t.Errorf("cert_mode %q, domain %q: mode %v, ports %d/%d; want mode %v, ports %d/443",
				tc.mode, tc.domain, got.Mode, got.HTTPPort, got.HTTPSPort, tc.wantMode, tc.wantHTTPPort)
		}
		if (got.Mode == TLSModeSelfSigned) != config.UsesSelfSignedTLS(cfg) {
			t.Errorf("cert_mode %q, domain %q: server mode %v disagrees with config.UsesSelfSignedTLS", tc.mode, tc.domain, got.Mode)
		}
		if got.Mode == TLSModeSelfSigned &&
			(got.CertFile != config.SelfSignedCertFile(dataDir) || got.KeyFile != config.SelfSignedKeyFile(dataDir)) {
			t.Errorf("cert_mode %q: self-signed files %q, %q are not the shared paths", tc.mode, got.CertFile, got.KeyFile)
		}
	}
}
