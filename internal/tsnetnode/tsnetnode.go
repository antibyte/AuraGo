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
	Running  bool     `json:"running"`
	Starting bool     `json:"starting,omitempty"` // waiting for interactive auth / cert issuance
	Hostname string   `json:"hostname,omitempty"`
	DNS      string   `json:"dns,omitempty"`
	IPs      []string `json:"ips,omitempty"`
	CertDNS  []string `json:"cert_dns,omitempty"`
	Error    string   `json:"error,omitempty"`
	LoginURL string   `json:"login_url,omitempty"`
}

// Manager manages a tsnet embedded Tailscale node.
type Manager struct {
	cfg    *config.Config
	logger *slog.Logger

	mu       sync.Mutex
	server   *tsnet.Server
	listener net.Listener
	httpSrv  *http.Server
	running  bool
	starting bool // true while Start() is blocked waiting for tsnet auth / certs
	lastErr  string

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

	// Start the tsnet server
	if err := srv.Start(); err != nil {
		cleanup(err.Error())
		return fmt.Errorf("failed to start tsnet server: %w", err)
	}

	// Get a TLS listener with automatic Tailscale certificates.
	// This call blocks until the node is authenticated and a cert has been issued.
	ln, err := srv.ListenTLS("tcp", ":443")
	if err != nil {
		srv.Close()
		cleanup(err.Error())
		return fmt.Errorf("failed to listen TLS on tsnet: %w", err)
	}

	httpSrv := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}

	// Re-acquire m.mu to commit the final running state.
	m.mu.Lock()
	m.server = srv
	m.listener = ln
	m.httpSrv = httpSrv
	m.running = true
	m.starting = false
	m.lastErr = ""
	m.mu.Unlock()

	go func() {
		m.logger.Info("tsnet HTTPS server listening", "hostname", hostname)
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet HTTP server error", "error", err)
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
	m.logger.Info("tsnet node stopped")
	return nil
}

// Status returns the current status of the tsnet node.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{
		Running:  m.running,
		Starting: m.starting,
		Hostname: m.cfg.Tailscale.TsNet.Hostname,
	}

	if m.lastErr != "" {
		st.Error = m.lastErr
	}

	if m.running && m.server != nil {
		lc, err := m.server.LocalClient()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
