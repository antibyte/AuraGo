package config

import (
	"path/filepath"
	"strings"
)

// NormalizeCertMode returns server.https.cert_mode trimmed and lower-cased,
// the form every reader of the mode compares against ("auto", "custom",
// "selfsigned").
func NormalizeCertMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

// UsesSelfSignedTLS reports whether the server serves its self-signed
// certificate: HTTPS is enabled and cert_mode is "selfsigned", or any mode
// other than "custom" (normally "auto" or empty) is set without a domain, where
// Let's Encrypt cannot work. The server's TLS setup and the egg config
// generator, which pins this certificate, both decide by it.
func UsesSelfSignedTLS(cfg *Config) bool {
	if cfg == nil || !cfg.Server.HTTPS.Enabled {
		return false
	}
	switch NormalizeCertMode(cfg.Server.HTTPS.CertMode) {
	case "selfsigned":
		return true
	case "custom":
		return false
	default:
		return strings.TrimSpace(cfg.Server.HTTPS.Domain) == ""
	}
}

// SelfSignedCertFile is where the server keeps its self-signed certificate.
func SelfSignedCertFile(dataDir string) string {
	return filepath.Join(dataDir, "certs", "selfsigned.crt")
}

// SelfSignedKeyFile is where the server keeps the self-signed certificate's
// private key.
func SelfSignedKeyFile(dataDir string) string {
	return filepath.Join(dataDir, "certs", "selfsigned.key")
}
