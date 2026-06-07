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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	"tailscale.com/tsnet"
)

const (
	defaultManifestTsNetPort    = 443
	legacyManifestTsNetPort     = 8444
	storeAppProxyBackendTimeout = 5 * time.Second
)

var tcpDialTimeout = net.DialTimeout
var listenTLSWithTimeoutFn = listenTLSWithTimeout
var homepageExposureRetryDelay = 5 * time.Second

// Status represents the current state of the tsnet node.
type Status struct {
	Running           bool                  `json:"running"`
	Starting          bool                  `json:"starting,omitempty"`            // waiting for interactive auth / cert issuance
	ServingHTTP       bool                  `json:"serving_http"`                  // true when AuraGo itself is exposed on 443/80
	HomepageServing   bool                  `json:"homepage_serving,omitempty"`    // true when Homepage/Caddy is exposed on 8443
	SpaceAgentServing bool                  `json:"space_agent_serving,omitempty"` // true when Space Agent is exposed over HTTPS
	SpaceAgentDNS     string                `json:"space_agent_dns,omitempty"`     // MagicDNS name of the dedicated Space Agent node
	ManifestServing   bool                  `json:"manifest_serving,omitempty"`    // true when Manifest is exposed over HTTPS
	ManifestDNS       string                `json:"manifest_dns,omitempty"`        // MagicDNS name of the dedicated Manifest node
	HTTPFallback      bool                  `json:"http_fallback,omitempty"`       // true when AuraGo runs HTTP (no TLS) because HTTPS certs not enabled
	FunnelActive      bool                  `json:"funnel_active,omitempty"`       // true when the AuraGo listener is exposed via Funnel
	Hostname          string                `json:"hostname,omitempty"`
	DNS               string                `json:"dns,omitempty"`
	IPs               []string              `json:"ips,omitempty"`
	CertDNS           []string              `json:"cert_dns,omitempty"`
	Error             string                `json:"error,omitempty"`
	LoginURL          string                `json:"login_url,omitempty"`
	StoreAppProxies   []StoreAppProxyStatus `json:"store_app_proxies,omitempty"`
}

// StoreAppProxySpec describes one managed HTTPS proxy for a desktop store app.
type StoreAppProxySpec struct {
	ID           string `json:"id"`
	Port         int    `json:"port"`
	TargetURL    string `json:"target_url"`
	APITargetURL string `json:"api_target_url,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// StoreAppProxyStatus is the public status for one active store app proxy.
type StoreAppProxyStatus struct {
	ID           string `json:"id"`
	Port         int    `json:"port"`
	TargetURL    string `json:"target_url"`
	APITargetURL string `json:"api_target_url,omitempty"`
}

// Manager manages a tsnet embedded Tailscale node.
type Manager struct {
	cfg    *config.Config
	logger *slog.Logger

	mu               sync.Mutex
	server           *tsnet.Server
	listener         net.Listener // main listener (Funnel or TLS)
	tailnetLn        net.Listener // secondary direct-tailnet TLS listener when Funnel is active
	httpSrv          *http.Server
	homepageLn       net.Listener
	homepageSrv      *http.Server
	manifestNode     *tsnet.Server
	manifestLn       net.Listener
	manifestSrv      *http.Server
	manifestHost     string
	spaceAgentNode   *tsnet.Server
	spaceAgentLn     net.Listener
	spaceAgentSrv    *http.Server
	spaceAgentHost   string
	running          bool
	starting         bool // true while Start() is blocked waiting for tsnet auth / certs
	servingHTTP      bool // true when an HTTP/HTTPS listener is active
	homepageUp       bool // true when the homepage proxy listener is active
	homepageRetrying bool // true while a background retry is waiting for the homepage backend
	manifestUp       bool // true when the Manifest proxy listener is active
	spaceAgentUp     bool // true when the dedicated Space Agent node is active
	httpFallback     bool // true when serving HTTP (no TLS) instead of HTTPS
	funnelActive     bool // true when the AuraGo listener is exposed via Tailscale Funnel
	lastErr          string
	storeProxyLns    map[string]net.Listener
	storeProxySrvs   map[string]*http.Server
	storeProxySpecs  map[string]StoreAppProxySpec

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

func (m *Manager) makeLoginAwareLogFunc(debug bool) func(string, ...any) {
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
		m.manifestNode = nil
		m.manifestLn = nil
		m.manifestSrv = nil
		m.manifestHost = ""
		m.spaceAgentNode = nil
		m.spaceAgentLn = nil
		m.spaceAgentSrv = nil
		m.spaceAgentHost = ""
		m.running = true
		m.starting = false
		m.lastErr = ""
		m.servingHTTP = false
		m.homepageUp = false
		m.manifestUp = false
		m.spaceAgentUp = false
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

	if !tsCfg.ServeHTTP && !tsCfg.ExposeHomepage && !tsCfg.ExposeManifest && !tsCfg.ExposeSpaceAgent {
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
	if m.manifestSrv != nil {
		m.manifestSrv.Shutdown(ctx)
	}
	if m.manifestNode != nil {
		m.manifestNode.Close()
	}
	if m.spaceAgentSrv != nil {
		m.spaceAgentSrv.Shutdown(ctx)
	}
	if m.spaceAgentNode != nil {
		m.spaceAgentNode.Close()
	}
	for _, srv := range m.storeProxySrvs {
		if srv != nil {
			srv.Shutdown(ctx)
		}
	}
	for _, ln := range m.storeProxyLns {
		if ln != nil {
			ln.Close()
		}
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
	m.manifestNode = nil
	m.manifestLn = nil
	m.manifestSrv = nil
	m.manifestHost = ""
	m.spaceAgentNode = nil
	m.spaceAgentLn = nil
	m.spaceAgentSrv = nil
	m.spaceAgentHost = ""
	m.storeProxyLns = nil
	m.storeProxySrvs = nil
	m.storeProxySpecs = nil
	m.servingHTTP = false
	m.homepageUp = false
	m.manifestUp = false
	m.spaceAgentUp = false
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
		Running:           m.running,
		Starting:          m.starting,
		ServingHTTP:       m.servingHTTP,
		HomepageServing:   m.homepageUp,
		SpaceAgentServing: m.spaceAgentUp,
		ManifestServing:   m.manifestUp,
		HTTPFallback:      m.httpFallback,
		FunnelActive:      m.funnelActive,
		Hostname:          m.cfg.Tailscale.TsNet.Hostname,
	}

	if m.lastErr != "" {
		st.Error = m.lastErr
	}
	for _, spec := range m.storeProxySpecs {
		st.StoreAppProxies = append(st.StoreAppProxies, StoreAppProxyStatus{
			ID:           spec.ID,
			Port:         spec.Port,
			TargetURL:    spec.TargetURL,
			APITargetURL: spec.APITargetURL,
		})
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
	if m.spaceAgentNode != nil {
		lc, err := m.spaceAgentNode.LocalClient()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			status, err := lc.Status(ctx)
			if err == nil && status.Self != nil && status.Self.DNSName != "" {
				st.SpaceAgentDNS = status.Self.DNSName
			}
		}
	}
	if m.manifestNode != nil {
		lc, err := m.manifestNode.LocalClient()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			status, err := lc.Status(ctx)
			if err == nil && status.Self != nil && status.Self.DNSName != "" {
				st.ManifestDNS = status.Self.DNSName
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
	manifestUp := m.manifestUp
	activeManifestHost := m.manifestHost
	spaceAgentUp := m.spaceAgentUp
	activeSpaceAgentHost := m.spaceAgentHost
	funnelActive := m.funnelActive
	m.mu.Unlock()

	wantMain := m.cfg.Tailscale.TsNet.ServeHTTP
	wantFunnel := wantMain && m.cfg.Tailscale.TsNet.Funnel
	wantHomepage := m.cfg.Tailscale.TsNet.ExposeHomepage && m.cfg.Homepage.WebServerEnabled && m.cfg.Homepage.WebServerPort > 0
	wantManifest := m.cfg.Tailscale.TsNet.ExposeManifest && m.cfg.Manifest.Enabled && m.cfg.Manifest.Port > 0
	wantSpaceAgent := m.cfg.Tailscale.TsNet.ExposeSpaceAgent && m.cfg.SpaceAgent.Enabled && m.cfg.SpaceAgent.Port > 0
	desiredManifestHost := m.effectiveManifestHostname()
	desiredSpaceAgentHost := m.effectiveSpaceAgentHostname()

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
	if manifestUp && (!wantManifest || activeManifestHost != desiredManifestHost) {
		if err := m.stopManifestListener(); err != nil {
			return err
		}
		manifestUp = false
	}
	if spaceAgentUp && (!wantSpaceAgent || activeSpaceAgentHost != desiredSpaceAgentHost) {
		if err := m.stopSpaceAgentListener(); err != nil {
			return err
		}
		spaceAgentUp = false
	}

	if wantMain && !servingHTTP {
		if err := m.startMainListener(srv, handler); err != nil {
			return err
		}
	}
	if wantHomepage && !homepageUp {
		if !homepageProxyBackendReachable(m.cfg.Homepage.WebServerPort, 2*time.Second) {
			err := homepageBackendUnavailableError(m.cfg.Homepage.WebServerPort)
			m.logger.Warn("[tsnet] Homepage exposure could not be started", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
			m.scheduleHomepageExposureRetry()
		} else if err := m.startHomepageListener(srv); err != nil {
			m.logger.Warn("[tsnet] Homepage exposure could not be started", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
		}
	}
	if wantManifest && !manifestUp {
		if err := m.startManifestListener(srv); err != nil {
			m.logger.Warn("[tsnet] Manifest exposure could not be started", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
		}
	}
	if wantSpaceAgent && !spaceAgentUp {
		if err := m.startSpaceAgentListener(); err != nil {
			m.logger.Warn("[tsnet] Space Agent exposure could not be started", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	if (!wantMain || m.servingHTTP) && (!wantHomepage || m.homepageUp) && (!wantManifest || m.manifestUp) && (!wantSpaceAgent || m.spaceAgentUp) && (!wantFunnel || m.funnelActive) {
		m.lastErr = ""
	}
	m.mu.Unlock()

	return nil
}

// ReconcileStoreAppProxies reconciles per-app HTTPS proxies for desktop store
// containers on the existing AuraGo tsnet node. These proxies never use Funnel.
func (m *Manager) ReconcileStoreAppProxies(specs []StoreAppProxySpec) error {
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
	if m.storeProxyLns == nil {
		m.storeProxyLns = map[string]net.Listener{}
	}
	if m.storeProxySrvs == nil {
		m.storeProxySrvs = map[string]*http.Server{}
	}
	if m.storeProxySpecs == nil {
		m.storeProxySpecs = map[string]StoreAppProxySpec{}
	}
	want := map[string]StoreAppProxySpec{}
	for _, spec := range specs {
		normalized, ok := normalizeStoreProxySpec(spec)
		if ok {
			want[normalized.ID] = normalized
		}
	}
	var toStop []StoreAppProxyStatus
	for id, active := range m.storeProxySpecs {
		desired, ok := want[id]
		if ok && desired.Port == active.Port && desired.TargetURL == active.TargetURL && desired.APITargetURL == active.APITargetURL {
			continue
		}
		toStop = append(toStop, StoreAppProxyStatus{ID: id, Port: active.Port, TargetURL: active.TargetURL, APITargetURL: active.APITargetURL})
	}
	m.mu.Unlock()

	for _, status := range toStop {
		if err := m.stopStoreAppProxy(status.ID); err != nil {
			return err
		}
	}

	for _, desired := range want {
		m.mu.Lock()
		_, active := m.storeProxySpecs[desired.ID]
		m.mu.Unlock()
		if active {
			continue
		}
		if err := m.startStoreAppProxy(srv, desired); err != nil {
			return err
		}
	}
	return nil
}

func normalizeStoreProxySpec(spec StoreAppProxySpec) (StoreAppProxySpec, bool) {
	spec.ID = strings.ToLower(strings.TrimSpace(spec.ID))
	spec.TargetURL = strings.TrimSpace(spec.TargetURL)
	spec.APITargetURL = strings.TrimSpace(spec.APITargetURL)
	if !spec.Enabled || spec.ID == "" || spec.Port <= 0 || spec.TargetURL == "" {
		return StoreAppProxySpec{}, false
	}
	if _, err := url.ParseRequestURI(spec.TargetURL); err != nil {
		return StoreAppProxySpec{}, false
	}
	if spec.APITargetURL != "" {
		if _, err := url.ParseRequestURI(spec.APITargetURL); err != nil {
			return StoreAppProxySpec{}, false
		}
	}
	return spec, true
}

func (m *Manager) startStoreAppProxy(srv *tsnet.Server, spec StoreAppProxySpec) error {
	handler, err := newStoreAppProxyHandler(spec, m.logger)
	if err != nil {
		return err
	}
	ln, err := listenTLSWithTimeoutFn(srv, ":"+strconv.Itoa(spec.Port), tsnetTLSFallbackTimeout)
	if err != nil {
		return fmt.Errorf("start store app proxy %s: %w", spec.ID, err)
	}
	httpSrv := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Warn("[tsnet] store app proxy stopped", "app_id", spec.ID, "error", err)
		}
	}()
	m.mu.Lock()
	if m.storeProxyLns == nil {
		m.storeProxyLns = map[string]net.Listener{}
	}
	if m.storeProxySrvs == nil {
		m.storeProxySrvs = map[string]*http.Server{}
	}
	if m.storeProxySpecs == nil {
		m.storeProxySpecs = map[string]StoreAppProxySpec{}
	}
	m.storeProxyLns[spec.ID] = ln
	m.storeProxySrvs[spec.ID] = httpSrv
	m.storeProxySpecs[spec.ID] = spec
	m.mu.Unlock()
	m.logger.Info("[tsnet] store app proxy started", "app_id", spec.ID, "port", spec.Port, "target", spec.TargetURL)
	return nil
}

func newStoreAppProxyHandler(spec StoreAppProxySpec, logger *slog.Logger) (http.Handler, error) {
	target, err := url.Parse(spec.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("parse store app proxy target: %w", err)
	}
	uiProxy := newStoreAppReverseProxy(target, logger)
	if spec.APITargetURL == "" {
		return uiProxy, nil
	}
	apiTarget, err := url.Parse(spec.APITargetURL)
	if err != nil {
		return nil, fmt.Errorf("parse store app API proxy target: %w", err)
	}
	apiProxy := newStoreAppReverseProxy(apiTarget, logger)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if storeAppProxyUsesAPITarget(req.URL.Path) {
			apiProxy.ServeHTTP(w, req)
			return
		}
		uiProxy.ServeHTTP(w, req)
	}), nil
}

func newStoreAppReverseProxy(target *url.URL, logger *slog.Logger) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: storeAppProxyBackendTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   storeAppProxyBackendTimeout,
		ResponseHeaderTimeout: storeAppProxyBackendTimeout,
		ExpectContinueTimeout: time.Second,
	}
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		forwardedHost := req.Host
		originalDirector(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", forwardedHost)
		sanitizeStoreAppProxyRequest(req)
	}
	proxy.ModifyResponse = sanitizeStoreAppProxyResponse
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		if logger != nil {
			logger.Warn("[tsnet] store app proxy backend unavailable", "target", target.String(), "path", req.URL.Path, "error", err)
		}
		http.Error(w, "Store app backend unavailable", http.StatusBadGateway)
	}
	return proxy
}

func storeAppProxyUsesAPITarget(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func sanitizeStoreAppProxyRequest(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Del("If-None-Match")
	req.Header.Del("If-Modified-Since")
	req.Header.Del("If-Range")
	req.Header.Set("Cache-Control", "no-cache")
}

func sanitizeStoreAppProxyResponse(resp *http.Response) error {
	if resp == nil {
		return nil
	}
	resp.Header.Del("X-Frame-Options")
	stripFrameAncestorsHeader(resp.Header, "Content-Security-Policy")
	stripFrameAncestorsHeader(resp.Header, "Content-Security-Policy-Report-Only")
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		resp.Header.Set("Cache-Control", "no-store")
		resp.Header.Del("ETag")
		resp.Header.Del("Last-Modified")
	}
	return nil
}

func stripFrameAncestorsHeader(header http.Header, name string) {
	values := header.Values(name)
	if len(values) == 0 {
		return
	}
	header.Del(name)
	for _, value := range values {
		cleaned := stripFrameAncestorsDirective(value)
		if cleaned != "" {
			header.Add(name, cleaned)
		}
	}
}

func stripFrameAncestorsDirective(value string) string {
	directives := strings.Split(value, ";")
	kept := make([]string, 0, len(directives))
	for _, directive := range directives {
		trimmed := strings.TrimSpace(directive)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 && strings.EqualFold(fields[0], "frame-ancestors") {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, "; ")
}

func (m *Manager) stopStoreAppProxy(id string) error {
	m.mu.Lock()
	httpSrv := m.storeProxySrvs[id]
	ln := m.storeProxyLns[id]
	delete(m.storeProxySrvs, id)
	delete(m.storeProxyLns, id)
	delete(m.storeProxySpecs, id)
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown store app proxy %s: %w", id, err)
		}
	}
	if ln != nil {
		if err := ln.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return fmt.Errorf("close store app proxy %s: %w", id, err)
		}
	}
	return nil
}

func (m *Manager) homepageExposureWantedLocked() bool {
	return m.cfg != nil &&
		m.cfg.Tailscale.TsNet.ExposeHomepage &&
		m.cfg.Homepage.WebServerEnabled &&
		m.cfg.Homepage.WebServerPort > 0
}

func (m *Manager) scheduleHomepageExposureRetry() {
	m.mu.Lock()
	if m.homepageRetrying {
		m.mu.Unlock()
		return
	}
	m.homepageRetrying = true
	m.mu.Unlock()

	go m.retryHomepageExposure()
}

func (m *Manager) retryHomepageExposure() {
	for {
		time.Sleep(homepageExposureRetryDelay)

		m.mu.Lock()
		if !m.running || m.server == nil || m.homepageUp || !m.homepageExposureWantedLocked() {
			m.homepageRetrying = false
			m.mu.Unlock()
			return
		}
		srv := m.server
		port := m.cfg.Homepage.WebServerPort
		m.mu.Unlock()

		if !homepageProxyBackendReachable(port, 2*time.Second) {
			continue
		}

		if err := m.startHomepageListener(srv); err != nil {
			if m.logger != nil {
				m.logger.Warn("[tsnet] Homepage exposure retry failed", "error", err)
			}
			m.mu.Lock()
			m.lastErr = err.Error()
			m.mu.Unlock()
			continue
		}

		m.mu.Lock()
		m.homepageRetrying = false
		m.lastErr = ""
		m.mu.Unlock()
		return
	}
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
	manifestUp := m.manifestUp
	spaceAgentUp := m.spaceAgentUp
	m.mu.Unlock()

	if !servingHTTP && !homepageUp && !manifestUp && !spaceAgentUp {
		return nil
	}
	if err := m.stopMainListener(); err != nil {
		return err
	}
	if err := m.stopHomepageListener(); err != nil {
		return err
	}
	if err := m.stopManifestListener(); err != nil {
		return err
	}
	if err := m.stopSpaceAgentListener(); err != nil {
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
	if !homepageProxyBackendReachable(m.cfg.Homepage.WebServerPort, 2*time.Second) {
		return homepageBackendUnavailableError(m.cfg.Homepage.WebServerPort)
	}
	targetURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(m.cfg.Homepage.WebServerPort))
	if err != nil {
		return fmt.Errorf("invalid homepage proxy target: %w", err)
	}

	ln, err := listenTLSWithTimeoutFn(srv, ":8443", tsnetTLSStrictTimeout)
	if err != nil {
		return fmt.Errorf("homepage exposure requires Tailscale HTTPS on :8443: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
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

func homepageBackendUnavailableError(port int) error {
	return fmt.Errorf("homepage backend http://127.0.0.1:%d is not reachable; the tsnet homepage listener will retry after the homepage web server starts", port)
}

func homepageProxyBackendReachable(port int, timeout time.Duration) bool {
	if port <= 0 {
		return false
	}
	conn, err := tcpDialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
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

func manifestProxyTarget(cfg *config.Config) string {
	if cfg == nil {
		return "http://127.0.0.1:2099"
	}
	port := cfg.Manifest.Port
	if port <= 0 {
		port = 2099
	}
	if cfg.Runtime.IsDocker {
		return "http://manifest:" + strconv.Itoa(port)
	}
	hostPort := cfg.Manifest.HostPort
	if hostPort <= 0 {
		hostPort = port
	}
	return "http://127.0.0.1:" + strconv.Itoa(hostPort)
}

func (m *Manager) effectiveManifestHostname() string {
	hostname := strings.TrimSpace(m.cfg.Tailscale.TsNet.ManifestHostname)
	if hostname != "" {
		return hostname
	}
	base := strings.TrimSpace(m.cfg.Tailscale.TsNet.Hostname)
	if base == "" {
		base = "aurago"
	}
	return base + "-manifest"
}

func manifestTsNetPort(port int) int {
	if port <= 0 || port == legacyManifestTsNetPort {
		return defaultManifestTsNetPort
	}
	return port
}

func (m *Manager) startManifestListener(_ *tsnet.Server) error {
	targetURL, err := url.Parse(manifestProxyTarget(m.cfg))
	if err != nil {
		return fmt.Errorf("invalid Manifest proxy target: %w", err)
	}
	tsCfg := m.cfg.Tailscale.TsNet
	stateDir := tsCfg.StateDir
	if stateDir == "" {
		stateDir = "data/tsnet"
	}
	manifestStateDir := filepath.Join(stateDir, "manifest")
	if err := os.MkdirAll(manifestStateDir, 0o750); err != nil {
		return fmt.Errorf("create Manifest tsnet state directory: %w", err)
	}
	hostname := m.effectiveManifestHostname()
	authKey := tsCfg.AuthKey
	if authKey == "" {
		authKey = os.Getenv("TS_AUTHKEY")
	}
	manifestNode := &tsnet.Server{
		Hostname: hostname,
		Dir:      manifestStateDir,
		Logf:     m.makeLoginAwareLogFunc(true),
		UserLogf: m.makeLoginAwareLogFunc(false),
	}
	if authKey != "" {
		manifestNode.AuthKey = authKey
	}
	if err := manifestNode.Start(); err != nil {
		return fmt.Errorf("start Manifest tsnet node: %w", err)
	}
	port := manifestTsNetPort(m.cfg.Tailscale.TsNet.ManifestPort)
	ln, err := listenTLSWithTimeout(manifestNode, ":"+strconv.Itoa(port), tsnetTLSStrictTimeout)
	if err != nil {
		manifestNode.Close()
		return fmt.Errorf("Manifest exposure requires Tailscale HTTPS on dedicated hostname %q:%d: %w", hostname, port, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		m.logger.Warn("[tsnet] Manifest reverse proxy failed", "error", proxyErr)
		http.Error(w, "Manifest backend unavailable", http.StatusBadGateway)
	}

	manifestSrv := &http.Server{
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}

	m.mu.Lock()
	m.manifestNode = manifestNode
	m.manifestLn = ln
	m.manifestSrv = manifestSrv
	m.manifestHost = hostname
	m.manifestUp = true
	m.mu.Unlock()

	go func() {
		m.logger.Info("tsnet Manifest listener started", "protocol", "HTTPS", "hostname", hostname, "target", targetURL.String(), "port", port)
		if err := manifestSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet Manifest listener error", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.manifestUp = false
			m.mu.Unlock()
		}
	}()

	return nil
}

func (m *Manager) stopManifestListener() error {
	m.mu.Lock()
	manifestNode := m.manifestNode
	httpSrv := m.manifestSrv
	ln := m.manifestLn
	m.manifestNode = nil
	m.manifestSrv = nil
	m.manifestLn = nil
	m.manifestHost = ""
	m.manifestUp = false
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tsnet Manifest listener: %w", err)
		}
	}
	if manifestNode != nil {
		manifestNode.Close()
	}
	if ln != nil {
		if err := ln.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return fmt.Errorf("close tsnet Manifest listener: %w", err)
		}
	}
	return nil
}

func (m *Manager) effectiveSpaceAgentHostname() string {
	hostname := strings.TrimSpace(m.cfg.Tailscale.TsNet.SpaceAgentHostname)
	if hostname != "" {
		return hostname
	}
	base := strings.TrimSpace(m.cfg.Tailscale.TsNet.Hostname)
	if base == "" {
		base = "aurago"
	}
	return base + "-space-agent"
}

func (m *Manager) startSpaceAgentListener() error {
	targetURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(m.cfg.SpaceAgent.Port))
	if err != nil {
		return fmt.Errorf("invalid Space Agent proxy target: %w", err)
	}

	tsCfg := m.cfg.Tailscale.TsNet
	stateDir := tsCfg.StateDir
	if stateDir == "" {
		stateDir = "data/tsnet"
	}
	spaceStateDir := filepath.Join(stateDir, "space-agent")
	if err := os.MkdirAll(spaceStateDir, 0o750); err != nil {
		return fmt.Errorf("create Space Agent tsnet state directory: %w", err)
	}
	hostname := m.effectiveSpaceAgentHostname()
	authKey := tsCfg.AuthKey
	if authKey == "" {
		authKey = os.Getenv("TS_AUTHKEY")
	}
	spaceNode := &tsnet.Server{
		Hostname: hostname,
		Dir:      spaceStateDir,
		Logf:     m.makeLoginAwareLogFunc(true),
		UserLogf: m.makeLoginAwareLogFunc(false),
	}
	if authKey != "" {
		spaceNode.AuthKey = authKey
	}
	if err := spaceNode.Start(); err != nil {
		return fmt.Errorf("start Space Agent tsnet node: %w", err)
	}

	ln, err := listenTLSWithTimeout(spaceNode, ":443", tsnetTLSStrictTimeout)
	if err != nil {
		spaceNode.Close()
		return fmt.Errorf("Space Agent exposure requires Tailscale HTTPS on dedicated hostname %q: %w", hostname, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		m.logger.Warn("[tsnet] Space Agent reverse proxy failed", "error", proxyErr)
		http.Error(w, "Space Agent backend unavailable", http.StatusBadGateway)
	}

	spaceAgentSrv := &http.Server{
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}

	m.mu.Lock()
	m.spaceAgentNode = spaceNode
	m.spaceAgentLn = ln
	m.spaceAgentSrv = spaceAgentSrv
	m.spaceAgentHost = hostname
	m.spaceAgentUp = true
	m.mu.Unlock()

	go func() {
		m.logger.Info("tsnet Space Agent listener started", "protocol", "HTTPS", "hostname", hostname, "target", targetURL.String(), "port", 443)
		if err := spaceAgentSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.logger.Error("tsnet Space Agent listener error", "error", err)
			m.mu.Lock()
			m.lastErr = err.Error()
			m.spaceAgentUp = false
			m.mu.Unlock()
		}
	}()

	return nil
}

func (m *Manager) stopSpaceAgentListener() error {
	m.mu.Lock()
	spaceNode := m.spaceAgentNode
	httpSrv := m.spaceAgentSrv
	ln := m.spaceAgentLn
	m.spaceAgentNode = nil
	m.spaceAgentSrv = nil
	m.spaceAgentLn = nil
	m.spaceAgentHost = ""
	m.spaceAgentUp = false
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tsnet Space Agent listener: %w", err)
		}
	}
	if spaceNode != nil {
		spaceNode.Close()
	}
	if ln != nil {
		if err := ln.Close(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "closed") {
			return fmt.Errorf("close tsnet Space Agent listener: %w", err)
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
