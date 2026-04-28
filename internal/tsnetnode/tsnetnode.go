package tsnetnode

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	"tailscale.com/tsnet"
)

// Status represents the current state of the tsnet node.
type Status struct {
	Running         bool     `json:"running"`
	Starting        bool     `json:"starting,omitempty"`         // waiting for interactive auth / cert issuance
	ServingHTTP     bool     `json:"serving_http"`               // true when AuraGo itself is exposed on 443/80
	HomepageServing bool     `json:"homepage_serving,omitempty"` // true when Homepage/Caddy is exposed on 8443
	HTTPFallback    bool     `json:"http_fallback,omitempty"`    // true when AuraGo runs HTTP (no TLS) because HTTPS certs not enabled
	FunnelActive    bool     `json:"funnel_active,omitempty"`    // true when the AuraGo listener is exposed via Funnel
	Hostname        string   `json:"hostname,omitempty"`
	DNS             string   `json:"dns,omitempty"`
	IPs             []string `json:"ips,omitempty"`
	CertDNS         []string `json:"cert_dns,omitempty"`
	Error           string   `json:"error,omitempty"`
	LoginURL        string   `json:"login_url,omitempty"`
}

// Manager manages a tsnet embedded Tailscale node.
type Manager struct {
	cfg    *config.Config
	logger *slog.Logger

	mu           sync.Mutex
	server       *tsnet.Server
	listener     net.Listener // main listener (Funnel or TLS)
	tailnetLn    net.Listener // secondary direct-tailnet TLS listener when Funnel is active
	httpSrv      *http.Server
	homepageLn   net.Listener
	homepageSrv  *http.Server
	running      bool
	starting     bool // true while Start() is blocked waiting for tsnet auth / certs
	servingHTTP  bool // true when an HTTP/HTTPS listener is active
	homepageUp   bool // true when the homepage proxy listener is active
	httpFallback bool // true when serving HTTP (no TLS) instead of HTTPS
	funnelActive bool // true when the AuraGo listener is exposed via Tailscale Funnel
	lastErr      string

	// loginURL is the Tailscale auth URL when the node needs interactive login.
	// It is set once and shown in the UI instead of spamming the log.
	loginMu      sync.Mutex
	loginURL     string
	loginURLSeen bool
}

const (
	tsnetTLSFallbackTimeout = 15 * time.Second
	tsnetTLSStrictTimeout   = 2 * time.Minute
)

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
// The provided handler is AuraGo's authenticated web UI/API handler.
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

	// makeLoginAwareLogFunc returns a tsnet log callback that deduplicates login URLs
	// and routes ordinary messages to the structured logger at the given level
	// (debug=true → Debug, debug=false → Info).
	makeLoginAwareLogFunc := func(debug bool) func(string, ...any) {
		return func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			if strings.Contains(msg, "login.tailscale.com") {
				url := extractLoginURL(msg)
				m.loginMu.Lock()
				if url != "" && url != m.loginURL {
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
			if debug {
				m.logger.Debug("[tsnet] " + msg)
			} else {
				m.logger.Info("[tsnet] " + msg)
			}
		}
	}

	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		Logf:     makeLoginAwareLogFunc(true),
		UserLogf: makeLoginAwareLogFunc(false),
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
		// Network-only mode: node is connected, no listener yet unless Homepage
		// exposure is enabled below via ReconfigureExposure.
		m.mu.Lock()
		m.server = srv
		m.listener = nil
		m.httpSrv = nil
		m.homepageLn = nil
		m.homepageSrv = nil
		m.running = true
		m.starting = false
		m.lastErr = ""
		m.servingHTTP = false
		m.homepageUp = false
		m.httpFallback = false
		m.funnelActive = false
		m.mu.Unlock()
	} else {
		m.mu.Lock()
		m.server = srv
		m.running = true
		m.starting = false
		m.lastErr = ""
		m.mu.Unlock()
	}

	if err := m.ReconfigureExposure(handler); err != nil {
		m.mu.Lock()
		if m.server != nil {
			m.server.Close()
		}
		m.server = nil
		m.running = false
		m.starting = false
		m.mu.Unlock()
		cleanup(err.Error())
		return err
	}

	if !tsCfg.ServeHTTP && !tsCfg.ExposeHomepage {
		m.logger.Info("tsnet node connected (network-only mode — no web services exposed over Tailscale)", "hostname", hostname)
	}

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
	if m.homepageSrv != nil {
		m.homepageSrv.Shutdown(ctx)
	}
	if m.server != nil {
		m.server.Close()
	}

	m.running = false
	m.server = nil
	m.listener = nil
	m.tailnetLn = nil
	m.httpSrv = nil
	m.homepageLn = nil
	m.homepageSrv = nil
	m.servingHTTP = false
	m.homepageUp = false
	m.httpFallback = false
	m.funnelActive = false
	m.logger.Info("tsnet node stopped")
	return nil
}

// Status returns the current status of the tsnet node.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{
		Running:         m.running,
		Starting:        m.starting,
		ServingHTTP:     m.servingHTTP,
		HomepageServing: m.homepageUp,
		HTTPFallback:    m.httpFallback,
		FunnelActive:    m.funnelActive,
		Hostname:        m.cfg.Tailscale.TsNet.Hostname,
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

// ReconfigureExposure reconciles the active Tailscale listeners with the
// current config without disconnecting the node from the tailnet.
func (m *Manager) ReconfigureExposure(handler http.Handler) error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return fmt.Errorf("tsnet node is not running")
	}
	srv := m.server
	if srv == nil {
		m.mu.Unlock()
		return fmt.Errorf("tsnet server reference is nil")
	}
	servingHTTP := m.servingHTTP
	homepageUp := m.homepageUp
	funnelActive := m.funnelActive
	m.mu.Unlock()

	wantMain := m.cfg.Tailscale.TsNet.ServeHTTP
	wantFunnel := wantMain && m.cfg.Tailscale.TsNet.Funnel
	wantHomepage := m.cfg.Tailscale.TsNet.ExposeHomepage && m.cfg.Homepage.WebServerEnabled && m.cfg.Homepage.WebServerPort > 0

	if servingHTTP && (!wantMain || funnelActive != wantFunnel) {
		if err := m.stopMainListener(); err != nil {
			return err
		}
		servingHTTP = false
		funnelActive = false
	}
	if homepageUp && !wantHomepage {
		if err := m.stopHomepageListener(); err != nil {
			return err
		}
		homepageUp = false
	}

	if wantMain && !servingHTTP {
		if err := m.startMainListener(srv, handler); err != nil {
			return err
		}
	}
	if wantHomepage && !homepageUp {
		if err := m.startHomepageListener(srv); err != nil {
			m.logger.Warn("[tsnet] Homepage exposure could not be started", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	if (!wantMain || m.servingHTTP) && (!wantHomepage || m.homepageUp) && (!wantFunnel || m.funnelActive) {
		m.lastErr = ""
	}
	m.mu.Unlock()

	return nil
}

// UpgradeToHTTP keeps backward compatibility for the existing callers.
func (m *Manager) UpgradeToHTTP(handler http.Handler) error {
	return m.ReconfigureExposure(handler)
}

// DowngradeToNetworkOnly stops the HTTP/HTTPS listener but keeps the tsnet
// node connected to the Tailscale network.  It is a no-op when the node is
// already running in network-only mode.
func (m *Manager) DowngradeToNetworkOnly() error {
	m.mu.Lock()
	servingHTTP := m.servingHTTP
	homepageUp := m.homepageUp
	m.mu.Unlock()

	if !servingHTTP && !homepageUp {
		return nil
	}
	if err := m.stopMainListener(); err != nil {
		return err
	}
	if err := m.stopHomepageListener(); err != nil {
		return err
	}
	m.logger.Info("[tsnet] Downgraded to network-only mode (HTTP listener stopped)")
	return nil
}

func (m *Manager) startMainListener(srv *tsnet.Server, handler http.Handler) error {
	wantFunnel := m.cfg.Tailscale.TsNet.Funnel
	usingHTTP := false
	usingFunnel := false

	var (
		ln        net.Listener
		tailnetLn net.Listener
		err       error
	)

	if wantFunnel {
		ln, err = listenFunnelWithTimeout(srv, ":443", 20*time.Second)
		if err != nil {
			// Funnel was explicitly requested but failed — this is a hard error.
			// NEVER silently fall back to TLS or HTTP without explicit user consent.
			// Common reasons: Funnel ACL not granted, Funnel not enabled in admin panel,
			// port 443 already in use, or cert not yet provisioned.
			errMsg := fmt.Errorf("[tsnet] Funnel explicitly enabled but failed: %w", err)
			m.mu.Lock()
			m.lastErr = errMsg.Error()
			m.mu.Unlock()
			return errMsg
		}
		usingFunnel = true

		// Also bind a direct-tailnet TLS listener so that peers inside the tailnet
		// can still reach AuraGo directly (ListenFunnel only handles public/Funnel
		// traffic; direct tailnet peers use the ordinary TLS path).
		if tlsLn, tlsErr := listenTLSWithTimeout(srv, ":443", 10*time.Second); tlsErr == nil {
			tailnetLn = tlsLn
			m.logger.Info("[tsnet] Dual-listener active: Funnel (internet) + TLS (tailnet) on :443")
		} else {
			m.logger.Warn("[tsnet] Funnel active but could not bind tailnet TLS listener — direct tailnet access may not work", "error", tlsErr)
		}
	}

	if ln == nil {
		tlsTimeout := tsnetTLSStrictTimeout
		if m.cfg.Tailscale.TsNet.AllowHTTPFallback {
			tlsTimeout = tsnetTLSFallbackTimeout
		}
		ln, err = listenTLSWithTimeout(srv, ":443", tlsTimeout)
		if err != nil {
			if !m.cfg.Tailscale.TsNet.AllowHTTPFallback {
				return fmt.Errorf("[tsnet] HTTPS not available and allow_http_fallback is disabled: %w", err)
			}
			m.logger.Warn("[tsnet] HTTPS not available — falling back to HTTP on :80",
				"reason", err,
				"hint", "Enable HTTPS in the Tailscale admin panel, or set allow_http_fallback: true in config")
			ln, err = srv.Listen("tcp", ":80")
			usingHTTP = true
			if err != nil {
				return fmt.Errorf("failed to listen on tsnet (HTTP fallback also failed): %w", err)
			}
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
	m.tailnetLn = tailnetLn
	m.httpSrv = httpSrv
	m.servingHTTP = true
	m.httpFallback = usingHTTP
	m.funnelActive = usingFunnel
	if !usingFunnel && !wantFunnel {
		m.lastErr = ""
	}
	m.mu.Unlock()

	proto := "HTTPS"
	if usingFunnel {
		proto = "HTTPS + Funnel"
	} else if usingHTTP {
		proto = "HTTP (fallback — enable HTTPS in Tailscale admin for encrypted access)"
	}
	go func() {
		m.logger.Info("tsnet AuraGo listener started", "protocol", proto)
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet AuraGo listener error", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.servingHTTP = false
			m.httpFallback = false
			m.funnelActive = false
			m.mu.Unlock()
		}
	}()

	// Serve the secondary tailnet listener (present when Funnel is active).
	if tailnetLn != nil {
		go func() {
			if err := httpSrv.Serve(tailnetLn); err != nil && err != http.ErrServerClosed {
				m.logger.Warn("[tsnet] Tailnet TLS listener closed", "error", err)
			}
		}()
	}

	return nil
}

func (m *Manager) stopMainListener() error {
	m.mu.Lock()
	httpSrv := m.httpSrv
	ln := m.listener
	tailnetLn := m.tailnetLn
	m.httpSrv = nil
	m.listener = nil
	m.tailnetLn = nil
	m.servingHTTP = false
	m.httpFallback = false
	m.funnelActive = false
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tsnet AuraGo listener: %w", err)
		}
	}
	if ln != nil {
		if err := ln.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return fmt.Errorf("close tsnet AuraGo listener: %w", err)
		}
	}
	if tailnetLn != nil {
		if err := tailnetLn.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return fmt.Errorf("close tsnet tailnet listener: %w", err)
		}
	}
	return nil
}

func (m *Manager) startHomepageListener(srv *tsnet.Server) error {
	targetURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(m.cfg.Homepage.WebServerPort))
	if err != nil {
		return fmt.Errorf("invalid homepage proxy target: %w", err)
	}

	ln, err := listenTLSWithTimeout(srv, ":8443", tsnetTLSStrictTimeout)
	if err != nil {
		return fmt.Errorf("homepage exposure requires Tailscale HTTPS on :8443: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		m.logger.Warn("[tsnet] Homepage reverse proxy failed", "error", proxyErr)
		http.Error(w, "Homepage backend unavailable", http.StatusBadGateway)
	}

	homepageSrv := &http.Server{
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}

	m.mu.Lock()
	m.homepageLn = ln
	m.homepageSrv = homepageSrv
	m.homepageUp = true
	m.mu.Unlock()

	go func() {
		m.logger.Info("tsnet Homepage listener started", "protocol", "HTTPS", "target", targetURL.String(), "port", 8443)
		if err := homepageSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet Homepage listener error", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.homepageUp = false
			m.mu.Unlock()
		}
	}()

	return nil
}

func (m *Manager) stopHomepageListener() error {
	m.mu.Lock()
	httpSrv := m.homepageSrv
	ln := m.homepageLn
	m.homepageSrv = nil
	m.homepageLn = nil
	m.homepageUp = false
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tsnet Homepage listener: %w", err)
		}
	}
	if ln != nil {
		if err := ln.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return fmt.Errorf("close tsnet Homepage listener: %w", err)
		}
	}
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

func listenFunnelWithTimeout(srv *tsnet.Server, addr string, timeout time.Duration) (net.Listener, error) {
	type result struct {
		ln  net.Listener
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ln, err := srv.ListenFunnel("tcp", addr)
		ch <- result{ln, err}
	}()
	select {
	case r := <-ch:
		return r.ln, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("ListenFunnel timed out after %s", timeout)
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
