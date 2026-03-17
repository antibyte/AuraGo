package tsnetnode

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"aurago/internal/config"

	"tailscale.com/tsnet"
)

// Status represents the current state of the tsnet node.
type Status struct {
	Running  bool     `json:"running"`
	Hostname string   `json:"hostname,omitempty"`
	DNS      string   `json:"dns,omitempty"`
	IPs      []string `json:"ips,omitempty"`
	CertDNS  []string `json:"cert_dns,omitempty"`
	Error    string   `json:"error,omitempty"`
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
	lastErr  string
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
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("tsnet node is already running")
	}

	tsCfg := m.cfg.Tailscale.TsNet
	if !tsCfg.Enabled {
		return fmt.Errorf("tsnet is not enabled in config")
	}

	stateDir := tsCfg.StateDir
	if stateDir == "" {
		stateDir = "data/tsnet"
	}
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return fmt.Errorf("failed to create tsnet state directory: %w", err)
	}

	hostname := tsCfg.Hostname
	if hostname == "" {
		hostname = "aurago"
	}

	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		Logf:     func(format string, args ...any) { m.logger.Debug(fmt.Sprintf("[tsnet] "+format, args...)) },
	}

	// Retrieve auth key from vault if available
	authKey := os.Getenv("TS_AUTHKEY")
	if authKey != "" {
		srv.AuthKey = authKey
	}

	m.logger.Info("Starting tsnet node", "hostname", hostname, "state_dir", stateDir)

	// Start the tsnet server
	if err := srv.Start(); err != nil {
		m.lastErr = err.Error()
		return fmt.Errorf("failed to start tsnet server: %w", err)
	}

	// Get a TLS listener with automatic Tailscale certificates
	ln, err := srv.ListenTLS("tcp", ":443")
	if err != nil {
		srv.Close()
		m.lastErr = err.Error()
		return fmt.Errorf("failed to listen TLS on tsnet: %w", err)
	}

	httpSrv := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}

	m.server = srv
	m.listener = ln
	m.httpSrv = httpSrv
	m.running = true
	m.lastErr = ""

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
			}
		}
	}

	return st
}
