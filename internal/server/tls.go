package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/config"

	"golang.org/x/crypto/acme/autocert"
)

// TLSMode represents the TLS operation mode
type TLSMode int

const (
	// TLSModeDisabled - HTTP only
	TLSModeDisabled TLSMode = iota
	// TLSModeAuto - Let's Encrypt with HTTP redirect
	TLSModeAuto
	// TLSModeCustom - User-provided certificate files
	TLSModeCustom
	// TLSModeSelfSigned - Auto-generated self-signed certificate
	TLSModeSelfSigned
)

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Mode        TLSMode
	Domain      string
	Email       string
	CertDir     string
	CertFile    string
	KeyFile     string
	HTTPPort    int
	HTTPSPort   int
	BehindProxy bool
}

// IsTLSActive returns true if TLS is enabled
func (t *TLSConfig) IsTLSActive() bool {
	return t.Mode != TLSModeDisabled
}

// NewTLSConfigFromConfig creates TLS config from application config
func NewTLSConfigFromConfig(cfg *config.Config, dataDir string) *TLSConfig {
	if !cfg.Server.HTTPS.Enabled {
		return &TLSConfig{Mode: TLSModeDisabled}
	}

	certDir := filepath.Join(dataDir, "certs")

	switch cfg.Server.HTTPS.CertMode {
	case "custom":
		return &TLSConfig{
			Mode:      TLSModeCustom,
			Domain:    cfg.Server.HTTPS.Domain,
			CertFile:  cfg.Server.HTTPS.CertFile,
			KeyFile:   cfg.Server.HTTPS.KeyFile,
			CertDir:   certDir,
			HTTPPort:  cfg.Server.HTTPS.HTTPPort,
			HTTPSPort: cfg.Server.HTTPS.HTTPSPort,
		}
	case "selfsigned":
		return &TLSConfig{
			Mode:      TLSModeSelfSigned,
			Domain:    cfg.Server.HTTPS.Domain,
			CertDir:   certDir,
			CertFile:  filepath.Join(certDir, "selfsigned.crt"),
			KeyFile:   filepath.Join(certDir, "selfsigned.key"),
			HTTPPort:  cfg.Server.HTTPS.HTTPPort,
			HTTPSPort: cfg.Server.HTTPS.HTTPSPort,
		}
	default: // "auto" or empty
		// If no domain is set, fall back to self-signed instead of Let's Encrypt.
		// This is the sensible default when HTTPS is enabled but no public domain is available
		// (e.g., when using Cloudflare Tunnel which handles HTTPS at the edge).
		if cfg.Server.HTTPS.Domain == "" {
			return &TLSConfig{
				Mode:      TLSModeSelfSigned,
				Domain:    cfg.Server.HTTPS.Domain,
				CertDir:   certDir,
				CertFile:  filepath.Join(certDir, "selfsigned.crt"),
				KeyFile:   filepath.Join(certDir, "selfsigned.key"),
				HTTPPort:  cfg.Server.HTTPS.HTTPPort,
				HTTPSPort: cfg.Server.HTTPS.HTTPSPort,
			}
		}
		return &TLSConfig{
			Mode:      TLSModeAuto,
			Domain:    cfg.Server.HTTPS.Domain,
			Email:     cfg.Server.HTTPS.Email,
			CertDir:   certDir,
			HTTPPort:  cfg.Server.HTTPS.HTTPPort,
			HTTPSPort: cfg.Server.HTTPS.HTTPSPort,
		}
	}
}

// secureTLSConfig returns a tls.Config with strong defaults.
func secureTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}
}

// SetupServers creates HTTP and HTTPS servers based on TLS configuration
func SetupServers(tlsCfg *TLSConfig, handler http.Handler, logger Logger) (*http.Server, *http.Server, error) {
	if !tlsCfg.IsTLSActive() {
		return nil, nil, fmt.Errorf("TLS not enabled")
	}

	switch tlsCfg.Mode {
	case TLSModeAuto:
		return setupAutoTLS(tlsCfg, handler, logger)
	case TLSModeCustom:
		return setupCustomTLS(tlsCfg, handler, logger)
	case TLSModeSelfSigned:
		return setupSelfSignedTLS(tlsCfg, handler, logger)
	default:
		return nil, nil, fmt.Errorf("unknown TLS mode: %d", tlsCfg.Mode)
	}
}

// setupAutoTLS creates servers with Let's Encrypt auto-TLS.
func setupAutoTLS(tlsCfg *TLSConfig, handler http.Handler, logger Logger) (*http.Server, *http.Server, error) {
	if tlsCfg.Domain == "" {
		return nil, nil, fmt.Errorf("domain is required for Let's Encrypt auto-TLS")
	}
	// ACME HTTP-01 challenge requires port 80. Override 0 with the standard default.
	if tlsCfg.HTTPPort <= 0 {
		tlsCfg.HTTPPort = 80
	}

	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(tlsCfg.Domain),
		Cache:      autocert.DirCache(tlsCfg.CertDir),
		Email:      tlsCfg.Email,
	}

	tc := secureTLSConfig()
	tc.GetCertificate = certManager.GetCertificate

	httpsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", tlsCfg.HTTPSPort),
		Handler:      handler,
		TLSConfig:    tc,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", tlsCfg.HTTPPort),
		Handler:      certManager.HTTPHandler(httpRedirectHandler(tlsCfg.Domain)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return httpsServer, httpServer, nil
}

// setupCustomTLS creates servers with user-provided certificate files.
func setupCustomTLS(tlsCfg *TLSConfig, handler http.Handler, logger Logger) (*http.Server, *http.Server, error) {
	if tlsCfg.CertFile == "" || tlsCfg.KeyFile == "" {
		return nil, nil, fmt.Errorf("cert_file and key_file are required for custom TLS mode")
	}

	cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	tc := secureTLSConfig()
	tc.Certificates = []tls.Certificate{cert}

	httpsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", tlsCfg.HTTPSPort),
		Handler:      handler,
		TLSConfig:    tc,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	// HTTP redirect server is optional for custom certs — skip if HTTPPort is 0.
	if tlsCfg.HTTPPort <= 0 {
		return httpsServer, nil, nil
	}
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", tlsCfg.HTTPPort),
		Handler:      httpRedirectHandler(tlsCfg.Domain),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return httpsServer, httpServer, nil
}

// setupSelfSignedTLS generates a self-signed certificate and creates servers.
func setupSelfSignedTLS(tlsCfg *TLSConfig, handler http.Handler, logger Logger) (*http.Server, *http.Server, error) {
	// Generate or load existing self-signed cert
	if err := ensureSelfSignedCert(tlsCfg.CertFile, tlsCfg.KeyFile, tlsCfg.Domain, logger); err != nil {
		return nil, nil, fmt.Errorf("failed to create self-signed certificate: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load self-signed certificate: %w", err)
	}

	tc := secureTLSConfig()
	tc.Certificates = []tls.Certificate{cert}

	httpsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", tlsCfg.HTTPSPort),
		Handler:      handler,
		TLSConfig:    tc,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	// HTTP redirect server is optional for self-signed certs — skip if HTTPPort is 0.
	if tlsCfg.HTTPPort <= 0 {
		return httpsServer, nil, nil
	}
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", tlsCfg.HTTPPort),
		Handler:      httpRedirectHandler(tlsCfg.Domain),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return httpsServer, httpServer, nil
}

// ensureSelfSignedCert generates a self-signed certificate if it doesn't exist yet.
func ensureSelfSignedCert(certFile, keyFile, domain string, logger Logger) error {
	// If both files exist, skip generation
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			logger.Info("Self-signed certificate already exists, reusing", "cert", certFile)
			return nil
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(certFile)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	logger.Info("Generating self-signed certificate", "domain", domain)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"AuraGo Self-Signed"}, CommonName: "AuraGo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		BasicConstraintsValid: true,
	}

	// Add SANs — domain, localhost, loopback and all local network interface IPs
	if domain != "" {
		template.DNSNames = append(template.DNSNames, domain)
	}
	template.DNSNames = append(template.DNSNames, "localhost")
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))

	// Enumerate all network interface addresses so browsers accept the cert
	// when accessed via any local IP (e.g. 192.168.x.x on a LAN).
	if ifaces, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range ifaces {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			template.IPAddresses = append(template.IPAddresses, ip)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate PEM
	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write cert PEM: %w", err)
	}

	// Write private key PEM
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return fmt.Errorf("failed to write key PEM: %w", err)
	}

	logger.Info("Self-signed certificate generated", "cert", certFile, "valid_until", template.NotAfter.Format("2006-01-02"))
	return nil
}

// RegenerateSelfSignedCert forces re-generation of the self-signed cert by deleting existing files first.
func RegenerateSelfSignedCert(dataDir, domain string, logger Logger) error {
	certDir := filepath.Join(dataDir, "certs")
	certFile := filepath.Join(certDir, "selfsigned.crt")
	keyFile := filepath.Join(certDir, "selfsigned.key")

	os.Remove(certFile)
	os.Remove(keyFile)

	return ensureSelfSignedCert(certFile, keyFile, domain, logger)
}

// GetCertInfo returns basic info about a certificate file (for display in the UI).
func GetCertInfo(certFile string) (map[string]interface{}, error) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	info := map[string]interface{}{
		"subject":    cert.Subject.CommonName,
		"issuer":     cert.Issuer.CommonName,
		"not_before": cert.NotBefore.Format(time.RFC3339),
		"not_after":  cert.NotAfter.Format(time.RFC3339),
		"dns_names":  cert.DNSNames,
		"is_ca":      cert.IsCA,
		"expired":    time.Now().After(cert.NotAfter),
	}

	return info, nil
}

// httpRedirectHandler redirects all HTTP traffic to HTTPS
func httpRedirectHandler(domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := "https://"
		if domain != "" {
			target += domain
		} else {
			target += r.Host
		}
		target += r.URL.Path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		w.Header().Set("Connection", "close")
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}

// StartTLSServers starts both HTTP and HTTPS servers with graceful shutdown support
func StartTLSServers(httpsServer, httpServer *http.Server, logger Logger, shutdownCh chan struct{}) error {
	go func() {
		logger.Info("Starting HTTP redirect server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP redirect server failed", "error", err)
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTPS server", "addr", httpsServer.Addr)
		if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-shutdownCh:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		httpsServer.Shutdown(ctx)
		httpServer.Shutdown(ctx)
		return nil
	case err := <-errChan:
		return err
	}
}

// Logger interface for TLS operations
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// IsSecureRequest checks if request was made over HTTPS (direct or via proxy)
func IsSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	if r.Header.Get("X-Scheme") == "https" {
		return true
	}
	return false
}
