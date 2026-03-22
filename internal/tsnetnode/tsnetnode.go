package tsnetnode

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	"tailscale.com/tsnet"
)

// Status represents the current state of the tsnet node.
type Status struct {
	Running      bool     `json:"running"`
	Starting     bool     `json:"starting,omitempty"`      // waiting for interactive auth / cert issuance
	ServingHTTP  bool     `json:"serving_http"`            // true only when an HTTP/HTTPS listener is active
	HTTPFallback bool     `json:"http_fallback,omitempty"` // true when running HTTP (no TLS) because HTTPS certs not enabled
	Hostname     string   `json:"hostname,omitempty"`
	DNS          string   `json:"dns,omitempty"`
	IPs          []string `json:"ips,omitempty"`
	CertDNS      []string `json:"cert_dns,omitempty"`
	Error        string   `json:"error,omitempty"`
	LoginURL     string   `json:"login_url,omitempty"`
}

// Manager manages a tsnet embedded Tailscale node.
type Manager struct {
	cfg    *config.Config
	logger *slog.Logger

	mu           sync.Mutex
	server       *tsnet.Server
	listener     net.Listener
	httpSrv      *http.Server
	running      bool
	starting     bool // true while Start() is blocked waiting for tsnet auth / certs
	servingHTTP  bool // true when an HTTP/HTTPS listener is active
	httpFallback bool // true when serving HTTP (no TLS) instead of HTTPS
	lastErr      string

	// loginURL is the Tailscale auth URL when the node needs interactive login.
	// It is set once and shown in the UI instead of spamming the log.
	loginMu      sync.Mutex
	loginURL     string
	loginURLSeen bool
}

// NewManager creates a new tsnet manager.
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{cfg: cfg, logger: logger}
}

// UpdateConfig updates the config reference (e.g. after hot-reload).
func (m *Manager) UpdateConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
}

// Start initializes the tsnet server and begins serving.
// The provided handler will be served over HTTPS via the Tailscale cert.
func (m *Manager) Start(handler http.Handler) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tsnet node is already running")
	}
	if m.starting {
		m.mu.Unlock()
		return fmt.Errorf("tsnet node is already starting")
	}

	tsCfg := m.cfg.Tailscale.TsNet
	if !tsCfg.Enabled {
		m.mu.Unlock()
		return fmt.Errorf("tsnet is not enabled in config")
	}

	stateDir := tsCfg.StateDir
	if stateDir == "" {
		stateDir = "data/tsnet"
	}

	hostname := tsCfg.Hostname
	if hostname == "" {
		hostname = "aurago"
	}

	authKey := tsCfg.AuthKey

	// Mark as starting so GetStatus() can report the state while we wait for auth.
	// We release m.mu here because srv.ListenTLS blocks until the Tailscale node is
	// fully authenticated and a TLS cert has been issued — potentially a very long wait
	// when interactive login is required.  Holding m.mu during that wait would deadlock
	// every concurrent GetStatus() / Stop() call.
	m.starting = true
	m.mu.Unlock()

	// ── From here m.mu is NOT held ─────────────────────────────────────────────

	cleanup := func(err string) {
		m.mu.Lock()
		m.starting = false
		if err != "" {
			m.lastErr = err
		}
		m.mu.Unlock()
	}

	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		cleanup(err.Error())
		return fmt.Errorf("failed to create tsnet state directory: %w", err)
	}

	if authKey == "" {
		authKey = os.Getenv("TS_AUTHKEY")
	}

	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		Logf: func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			// Capture the Tailscale login URL and show it once instead of spamming the log.
			if strings.Contains(msg, "login.tailscale.com") {
				url := extractLoginURL(msg)
				m.loginMu.Lock()
				newURL := url != "" && url != m.loginURL
				if newURL {
					m.loginURL = url
					m.loginURLSeen = false
				}
				should := !m.loginURLSeen
				if should {
					m.loginURLSeen = true
				}
				m.loginMu.Unlock()
				if should {
					m.logger.Warn("[tsnet] Authentication required – visit the URL in Tailscale settings to connect", "url", url)
				}
				return
			}
			m.logger.Debug("[tsnet] " + msg)
		},
		// UserLogf handles user-facing messages (e.g. "To start this tsnet server, go to: …").
		// We route them through the same deduplication logic to avoid log spam.
		UserLogf: func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			if strings.Contains(msg, "login.tailscale.com") {
				url := extractLoginURL(msg)
				m.loginMu.Lock()
				newURL := url != "" && url != m.loginURL
				if newURL {
					m.loginURL = url
					m.loginURLSeen = false
				}
				should := !m.loginURLSeen
				if should {
					m.loginURLSeen = true
				}
				m.loginMu.Unlock()
				if should {
					m.logger.Warn("[tsnet] Authentication required – visit the URL in Tailscale settings to connect", "url", url)
				}
				return
			}
			m.logger.Info("[tsnet] " + msg)
		},
	}

	if authKey != "" {
		srv.AuthKey = authKey
	}

	m.logger.Info("Starting tsnet node", "hostname", hostname, "state_dir", stateDir)

	// Store the server reference now (before the blocking ListenTLS call) so that
	// GetStatus() can query the local Tailscale client to detect when authentication
	// completes and clear the pending loginURL.
	m.mu.Lock()
	m.server = srv
	m.mu.Unlock()

	// Start the tsnet server
	if err := srv.Start(); err != nil {
		m.mu.Lock()
		m.server = nil
		m.mu.Unlock()
		cleanup(err.Error())
		return fmt.Errorf("failed to start tsnet server: %w", err)
	}

	// ── Node is now in the Tailscale network ──────────────────────────────────
	// By default (serve_http: false) we just joined the network — no HTTP
	// listener is started and AuraGo is NOT exposed to other nodes.
	// Set serve_http: true in the config to also bind an HTTP/HTTPS server on
	// the Tailscale interface (the behaviour that existed before this feature).

	if !tsCfg.ServeHTTP {
		// Network-only mode: node is connected, no listener.
		m.mu.Lock()
		m.server = srv
		m.listener = nil
		m.httpSrv = nil
		m.running = true
		m.starting = false
		m.lastErr = ""
		m.servingHTTP = false
		m.httpFallback = false
		m.mu.Unlock()
		m.logger.Info("tsnet node connected (network-only mode — web UI not exposed over Tailscale)", "hostname", hostname)
		return nil
	}

	// serve_http: true — also start an HTTP/HTTPS listener.
	// Try HTTPS first (requires Tailscale HTTPS certificates to be enabled in the admin panel).
	// Fall back to plain HTTP on port 80 on any error — the exact error text from
	// the Tailscale control plane is not stable and string-matching is unreliable.
	usingHTTP := false
	ln, err := listenTLSWithTimeout(srv, ":443", 15*time.Second)
	if err != nil {
		m.logger.Warn("[tsnet] HTTPS not available — falling back to HTTP on :80",
			"reason", err,
			"hint", "Enable HTTPS in the Tailscale admin panel for encrypted access")
		ln, err = srv.Listen("tcp", ":80")
		usingHTTP = true
		if err != nil {
			srv.Close()
			m.mu.Lock()
			m.server = nil
			m.mu.Unlock()
			cleanup(err.Error())
			return fmt.Errorf("failed to listen on tsnet (HTTP fallback also failed): %w", err)
		}
	}

	httpSrv := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}
	if !usingHTTP {
		httpSrv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	// Re-acquire m.mu to commit the final running state.
	m.mu.Lock()
	m.server = srv
	m.listener = ln
	m.httpSrv = httpSrv
	m.running = true
	m.starting = false
	m.lastErr = ""
	m.servingHTTP = true
	m.httpFallback = usingHTTP
	m.mu.Unlock()

	proto := "HTTPS"
	if usingHTTP {
		proto = "HTTP (fallback — enable HTTPS in Tailscale admin for encrypted access)"
	}
	go func() {
		m.logger.Info("tsnet server listening", "hostname", hostname, "protocol", proto)
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet server error", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.running = false
			m.mu.Unlock()
		}
	}()

	return nil
}

// Stop gracefully shuts down the tsnet node.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("tsnet node is not running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.httpSrv != nil {
		m.httpSrv.Shutdown(ctx)
	}
	if m.server != nil {
		m.server.Close()
	}

	m.running = false
	m.server = nil
	m.listener = nil
	m.httpSrv = nil
	m.servingHTTP = false
	m.httpFallback = false
	m.logger.Info("tsnet node stopped")
	return nil
}

// Status returns the current status of the tsnet node.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{
		Running:      m.running,
		Starting:     m.starting,
		ServingHTTP:  m.servingHTTP,
		HTTPFallback: m.httpFallback,
		Hostname:     m.cfg.Tailscale.TsNet.Hostname,
	}

	if m.lastErr != "" {
		st.Error = m.lastErr
	}

	// Query the local Tailscale client whenever we have a server reference,
	// even while still in the starting phase (waiting for auth / cert issuance).
	// This allows us to detect when the user completes authentication and clear
	// the pending loginURL so the browser banner disappears promptly.
	if m.server != nil {
		lc, err := m.server.LocalClient()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			status, err := lc.Status(ctx)
			if err == nil && status.Self != nil {
				for _, ip := range status.Self.TailscaleIPs {
					st.IPs = append(st.IPs, ip.String())
				}
				if status.Self.DNSName != "" {
					st.DNS = status.Self.DNSName
					st.CertDNS = []string{status.Self.DNSName}
				}
				// Node is authenticated – clear the pending login URL.
				if len(st.IPs) > 0 {
					m.loginMu.Lock()
					m.loginURL = ""
					m.loginMu.Unlock()
				}
			}
		}
	}

	m.loginMu.Lock()
	st.LoginURL = m.loginURL
	m.loginMu.Unlock()

	return st
}

// UpgradeToHTTP attaches an HTTP/HTTPS listener to an already-running
// network-only tsnet node without disconnecting from Tailscale.
// It is a no-op when the node is already serving HTTP.
func (m *Manager) UpgradeToHTTP(handler http.Handler) error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return fmt.Errorf("tsnet node is not running")
	}
	if m.servingHTTP {
		m.mu.Unlock()
		return nil // already serving, nothing to do
	}
	srv := m.server
	if srv == nil {
		m.mu.Unlock()
		return fmt.Errorf("tsnet server reference is nil")
	}
	m.mu.Unlock()

	usingHTTP := false
	ln, err := listenTLSWithTimeout(srv, ":443", 15*time.Second)
	if err != nil {
		m.logger.Warn("[tsnet] HTTPS not available — falling back to HTTP on :80",
			"reason", err,
			"hint", "Enable HTTPS in the Tailscale admin panel for encrypted access")
		ln, err = srv.Listen("tcp", ":80")
		usingHTTP = true
		if err != nil {
			return fmt.Errorf("failed to start tsnet listener (HTTP fallback also failed): %w", err)
		}
	}

	httpSrv := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}
	if !usingHTTP {
		httpSrv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	m.mu.Lock()
	m.listener = ln
	m.httpSrv = httpSrv
	m.servingHTTP = true
	m.httpFallback = usingHTTP
	m.mu.Unlock()

	proto := "HTTPS"
	if usingHTTP {
		proto = "HTTP (fallback — enable HTTPS in Tailscale admin for encrypted access)"
	}
	go func() {
		m.logger.Info("tsnet HTTP server started (upgraded from network-only)", "protocol", proto)
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet HTTP server error", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.servingHTTP = false
			m.mu.Unlock()
		}
	}()
	return nil
}

// DowngradeToNetworkOnly stops the HTTP/HTTPS listener but keeps the tsnet
// node connected to the Tailscale network.  It is a no-op when the node is
// already running in network-only mode.
func (m *Manager) DowngradeToNetworkOnly() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.servingHTTP {
		return nil // already network-only
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.httpSrv != nil {
		m.httpSrv.Shutdown(ctx) //nolint:errcheck
		m.httpSrv = nil
	}
	if m.listener != nil {
		m.listener.Close()
		m.listener = nil
	}
	m.servingHTTP = false
	m.httpFallback = false
	m.logger.Info("[tsnet] Downgraded to network-only mode (HTTP listener stopped)")
	return nil
}

// listenTLSWithTimeout calls srv.ListenTLS with a timeout so that a slow or
// blocked cert-provisioning call does not stall the entire Start() goroutine.
func listenTLSWithTimeout(srv *tsnet.Server, addr string, timeout time.Duration) (net.Listener, error) {
	type result struct {
		ln  net.Listener
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ln, err := srv.ListenTLS("tcp", addr)
		ch <- result{ln, err}
	}()
	select {
	case r := <-ch:
		return r.ln, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("ListenTLS timed out after %s (HTTPS cert not ready)", timeout)
	}
}

// extractLoginURL pulls a https://login.tailscale.com/… URL out of a log message.
func extractLoginURL(msg string) string {
	const prefix = "https://login.tailscale.com"
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return ""
	}
	end := idx + len(prefix)
	for end < len(msg) {
		c := msg[end]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '"' || c == '\'' {
			break
		}
		end++
	}
	return msg[idx:end]
}
