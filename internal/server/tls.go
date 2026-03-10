package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
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
	// TLSModeManual - Manual certificates
	TLSModeManual
)

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Mode         TLSMode
	Domain       string
	Email        string
	CertDir      string
	CertFile     string
	KeyFile      string
	HTTPPort     int
	HTTPSPort    int
	BehindProxy  bool
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

	return &TLSConfig{
		Mode:      TLSModeAuto,
		Domain:    cfg.Server.HTTPS.Domain,
		Email:     cfg.Server.HTTPS.Email,
		CertDir:   filepath.Join(dataDir, "certs"),
		HTTPPort:  80,
		HTTPSPort: 443,
	}
}

// SetupServers creates HTTP and HTTPS servers based on TLS configuration
func SetupServers(tlsCfg *TLSConfig, handler http.Handler, logger Logger) (*http.Server, *http.Server, error) {
	if !tlsCfg.IsTLSActive() {
		return nil, nil, fmt.Errorf("TLS not enabled")
	}

	// Create autocert manager
	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(tlsCfg.Domain),
		Cache:      autocert.DirCache(tlsCfg.CertDir),
		Email:      tlsCfg.Email,
	}

	// TLS configuration with secure defaults
	tlsConfig := &tls.Config{
		GetCertificate: certManager.GetCertificate,
		MinVersion:     tls.VersionTLS12,
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
		PreferServerCipherSuites: true,
	}

	// HTTPS server
	httpsAddr := fmt.Sprintf(":%d", tlsCfg.HTTPSPort)
	httpsServer := &http.Server{
		Addr:         httpsAddr,
		Handler:      handler,
		TLSConfig:    tlsConfig,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	// HTTP redirect server (or ACME challenge handler)
	httpAddr := fmt.Sprintf(":%d", tlsCfg.HTTPPort)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      certManager.HTTPHandler(httpRedirectHandler(tlsCfg.Domain)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return httpsServer, httpServer, nil
}

// httpRedirectHandler redirects all HTTP traffic to HTTPS
func httpRedirectHandler(domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle ACME challenges first (certManager will intercept these)
		
		// Build HTTPS URL
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
	// Start HTTP server in goroutine
	go func() {
		logger.Info("Starting HTTP redirect server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP redirect server failed", "error", err)
		}
	}()

	// Start HTTPS server in goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTPS server", "addr", httpsServer.Addr)
		if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
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
	// Check direct TLS
	if r.TLS != nil {
		return true
	}
	
	// Check X-Forwarded-Proto header (reverse proxy)
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	
	// Check X-Scheme header (some proxies)
	if r.Header.Get("X-Scheme") == "https" {
		return true
	}
	
	return false
}
