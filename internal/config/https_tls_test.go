package config

import (
	"path/filepath"
	"testing"
)

// UsesSelfSignedTLS mirrors the server's TLS mode choice: "selfsigned", or
// any mode but "custom" without a domain (the "auto" fallback). The mode is
// trimmed and lower-cased, so " SelfSigned " counts as "selfsigned".
func TestUsesSelfSignedTLS(t *testing.T) {
	for _, tc := range []struct {
		mode, domain string
		enabled      bool
		want         bool
	}{
		{"selfsigned", "", true, true},
		{"selfsigned", "aurago.example.com", true, true},
		{" SelfSigned ", "aurago.example.com", true, true},
		{"auto", "", true, true},
		{"", "", true, true},
		{" AUTO ", "  ", true, true},
		{"unknown", "", true, true},
		{"auto", "aurago.example.com", true, false},
		{"", "aurago.example.com", true, false},
		{"unknown", "aurago.example.com", true, false},
		{"custom", "", true, false},
		{" Custom ", "", true, false},
		{"selfsigned", "", false, false},
	} {
		cfg := &Config{}
		cfg.Server.HTTPS.Enabled = tc.enabled
		cfg.Server.HTTPS.CertMode = tc.mode
		cfg.Server.HTTPS.Domain = tc.domain
		if got := UsesSelfSignedTLS(cfg); got != tc.want {
			t.Errorf("UsesSelfSignedTLS(enabled=%v, cert_mode=%q, domain=%q) = %v, want %v", tc.enabled, tc.mode, tc.domain, got, tc.want)
		}
	}
	if UsesSelfSignedTLS(nil) {
		t.Error("UsesSelfSignedTLS(nil) = true, want false")
	}
}

func TestNormalizeCertMode(t *testing.T) {
	for in, want := range map[string]string{
		"selfsigned":   "selfsigned",
		" SelfSigned ": "selfsigned",
		"CUSTOM":       "custom",
		"auto":         "auto",
		"":             "",
	} {
		if got := NormalizeCertMode(in); got != want {
			t.Errorf("NormalizeCertMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSelfSignedCertPaths(t *testing.T) {
	dataDir := filepath.Join("srv", "aurago", "data")
	if got, want := SelfSignedCertFile(dataDir), filepath.Join(dataDir, "certs", "selfsigned.crt"); got != want {
		t.Errorf("SelfSignedCertFile = %q, want %q", got, want)
	}
	if got, want := SelfSignedKeyFile(dataDir), filepath.Join(dataDir, "certs", "selfsigned.key"); got != want {
		t.Errorf("SelfSignedKeyFile = %q, want %q", got, want)
	}
}
