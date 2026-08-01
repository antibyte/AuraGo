package tsnetnode

import (
	"context"
	"crypto/tls"
	"errors"
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
	"sync/atomic"
	"time"

	"aurago/internal/config"

	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
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

type NodeID string

const (
	NodeMain       NodeID = "main"
	NodeManifest   NodeID = "manifest"
	NodeSpaceAgent NodeID = "space_agent"
)

const (
	ErrorAuthKeyMissing     = "TSNET_AUTH_KEY_MISSING"
	ErrorAuthKeyRejected    = "TSNET_AUTH_KEY_REJECTED"
	ErrorLoginRequired      = "TSNET_LOGIN_REQUIRED"
	ErrorNodeKeyExpired     = "TSNET_NODE_KEY_EXPIRED"
	ErrorStateCorrupt       = "TSNET_STATE_CORRUPT"
	ErrorCertUnavailable    = "TSNET_CERT_UNAVAILABLE"
	ErrorFunnelUnavailable  = "TSNET_FUNNEL_UNAVAILABLE"
	ErrorTimeout            = "TSNET_TIMEOUT"
	ErrorOperationConflict  = "TSNET_OPERATION_CONFLICT"
	ErrorNodeNotConfigured  = "TSNET_NODE_NOT_CONFIGURED"
	ErrorBackendUnavailable = "TSNET_BACKEND_UNAVAILABLE"
)

const recoveryMarkerSuffix = ".recovery-backup"

type NodeStatus struct {
	Configured        bool     `json:"configured"`
	BackendState      string   `json:"backend_state,omitempty"`
	Running           bool     `json:"running"`
	ListenerReady     bool     `json:"listener_ready"`
	HaveNodeKey       bool     `json:"have_node_key"`
	NodeKeyExpired    bool     `json:"node_key_expired"`
	KeyExpiry         string   `json:"key_expiry,omitempty"`
	DNS               string   `json:"dns,omitempty"`
	IPs               []string `json:"ips,omitempty"`
	LoginURL          string   `json:"login_url,omitempty"`
	Health            string   `json:"health"`
	ErrorCode         string   `json:"error_code,omitempty"`
	ErrorMessage      string   `json:"error_message,omitempty"`
	KeySource         string   `json:"key_source"`
	CredentialChanged bool     `json:"credential_changed,omitempty"`
}

type OperationStatus struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Node      NodeID    `json:"node,omitempty"`
	State     string    `json:"state"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	ErrorCode string    `json:"error_code,omitempty"`
	Error     string    `json:"error_message,omitempty"`
}

// Status represents the current state of the tsnet node.
type Status struct {
	Ready             bool                  `json:"ready"`
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
	Nodes             map[NodeID]NodeStatus `json:"nodes,omitempty"`
	Operation         *OperationStatus      `json:"operation,omitempty"`
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
	cfg    managerConfigSnapshot
	logger *slog.Logger

	localClientForServer func(*tsnet.Server) (nodeLocalClient, error)
	listenTLSForNode     func(context.Context, *tsnet.Server, string, time.Duration, bool) (net.Listener, error)
	listenFunnelForNode  func(context.Context, *tsnet.Server, string, time.Duration, bool) (net.Listener, error)
	recoveryFS           recoveryFileSystem

	mu               sync.Mutex
	server           *tsnet.Server
	listener         net.Listener // main listener (Funnel or TLS)
	httpSrv          *http.Server
	homepageLn       net.Listener
	homepageSrv      *http.Server
	manifest         childResourceState
	spaceAgent       childResourceState
	running          bool
	starting         bool // true while Start() is blocked waiting for tsnet auth / certs
	servingHTTP      bool // true when an HTTP/HTTPS listener is active
	homepageUp       bool // true when the homepage proxy listener is active
	homepageRetrying bool // true while a background retry is waiting for the homepage backend
	httpFallback     bool // true when serving HTTP (no TLS) instead of HTTPS
	funnelActive     bool // true when the AuraGo listener is exposed via Tailscale Funnel
	lastErr          string
	storeProxyLns    map[string]net.Listener
	storeProxySrvs   map[string]*http.Server
	storeProxySpecs  map[string]StoreAppProxySpec

	// loginURL is the Tailscale auth URL when the node needs interactive login.
	// It is set once and shown in the UI instead of spamming the log.
	loginMu     sync.Mutex
	nodeRuntime map[NodeID]*nodeRuntimeState

	operationMu     sync.Mutex
	operation       *OperationStatus
	operationCancel context.CancelFunc
	operationDone   chan struct{}
	operationPlan   *runtimePlan
	shuttingDown    bool
}

type nodeRuntimeState struct {
	LoginURL          string
	LoginURLSeen      bool
	ErrorCode         string
	ErrorMessage      string
	CredentialChanged bool
}

type childResourceState struct {
	Generation uint64
	Node       *tsnet.Server
	Listener   net.Listener
	Server     *http.Server
	Host       string
	State      string
}

type listenerResult struct {
	ln  net.Listener
	err error
}

type runtimePlan struct {
	Config      managerConfigSnapshot
	Credentials credentialSnapshot
}

type managerConfigSnapshot struct {
	Valid     bool
	Tailscale struct {
		TsNet struct {
			Enabled            bool
			Hostname           string
			StateDir           string
			ServeHTTP          bool
			ExposeHomepage     bool
			ExposeSpaceAgent   bool
			SpaceAgentHostname string
			ExposeManifest     bool
			ManifestHostname   string
			ManifestPort       int
			Funnel             bool
			AllowHTTPFallback  bool
			AuthKey            string
			AuthKeyMain        string
			AuthKeyManifest    string
			AuthKeySpaceAgent  string
		}
	}
	Homepage struct {
		WebServerEnabled bool
		WebServerPort    int
	}
	Manifest struct {
		Enabled  bool
		Port     int
		HostPort int
	}
	SpaceAgent struct {
		Enabled bool
		Port    int
	}
	Runtime struct {
		IsDocker bool
	}
}

type resolvedCredential struct {
	Key    string
	Source string
}

type credentialSnapshot struct {
	Main       resolvedCredential
	Manifest   resolvedCredential
	SpaceAgent resolvedCredential
}

type nodeLocalClient interface {
	Status(context.Context) (*ipnstate.Status, error)
	GetPrefs(context.Context) (*ipn.Prefs, error)
	Start(context.Context, ipn.Options) error
	StartLoginInteractive(context.Context) error
}

type recoveryFileSystem interface {
	Lstat(string) (os.FileInfo, error)
	Rename(string, string) error
	Chmod(string, os.FileMode) error
	Mkdir(string, os.FileMode) error
	MkdirAll(string, os.FileMode) error
	RemoveAll(string) error
	WriteFile(string, []byte, os.FileMode) error
	ReadFile(string) ([]byte, error)
	Remove(string) error
}

type osRecoveryFileSystem struct{}

func (osRecoveryFileSystem) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func (osRecoveryFileSystem) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (osRecoveryFileSystem) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func (osRecoveryFileSystem) Mkdir(path string, mode os.FileMode) error {
	return os.Mkdir(path, mode)
}

func (osRecoveryFileSystem) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (osRecoveryFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (osRecoveryFileSystem) WriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}

func (osRecoveryFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osRecoveryFileSystem) Remove(path string) error {
	return os.Remove(path)
}

var operationSequence atomic.Uint64

const (
	tsnetTLSFallbackTimeout = 15 * time.Second
	tsnetTLSStrictTimeout   = 2 * time.Minute
)

// NewManager creates a new tsnet manager.
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		cfg:    snapshotManagerConfig(cfg),
		logger: logger,
		localClientForServer: func(server *tsnet.Server) (nodeLocalClient, error) {
			return server.LocalClient()
		},
		listenTLSForNode:    listenTLSWithContext,
		listenFunnelForNode: listenFunnelWithContext,
		recoveryFS:          osRecoveryFileSystem{},
		nodeRuntime: map[NodeID]*nodeRuntimeState{
			NodeMain:       {},
			NodeManifest:   {},
			NodeSpaceAgent: {},
		},
	}
}

// UpdateConfig updates the config reference (e.g. after hot-reload).
func (m *Manager) UpdateConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = snapshotManagerConfig(cfg)
}

func snapshotManagerConfig(cfg *config.Config) managerConfigSnapshot {
	var snapshot managerConfigSnapshot
	if cfg == nil {
		return snapshot
	}
	snapshot.Valid = true
	source := cfg.Tailscale.TsNet
	target := &snapshot.Tailscale.TsNet
	target.Enabled = source.Enabled
	target.Hostname = source.Hostname
	target.StateDir = source.StateDir
	target.ServeHTTP = source.ServeHTTP
	target.ExposeHomepage = source.ExposeHomepage
	target.ExposeSpaceAgent = source.ExposeSpaceAgent
	target.SpaceAgentHostname = source.SpaceAgentHostname
	target.ExposeManifest = source.ExposeManifest
	target.ManifestHostname = source.ManifestHostname
	target.ManifestPort = source.ManifestPort
	target.Funnel = source.Funnel
	target.AllowHTTPFallback = source.AllowHTTPFallback
	target.AuthKey = source.AuthKey
	target.AuthKeyMain = source.AuthKeyMain
	target.AuthKeyManifest = source.AuthKeyManifest
	target.AuthKeySpaceAgent = source.AuthKeySpaceAgent
	snapshot.Homepage.WebServerEnabled = cfg.Homepage.WebServerEnabled
	snapshot.Homepage.WebServerPort = cfg.Homepage.WebServerPort
	snapshot.Manifest.Enabled = cfg.Manifest.Enabled
	snapshot.Manifest.Port = cfg.Manifest.Port
	snapshot.Manifest.HostPort = cfg.Manifest.HostPort
	snapshot.SpaceAgent.Enabled = cfg.SpaceAgent.Enabled
	snapshot.SpaceAgent.Port = cfg.SpaceAgent.Port
	snapshot.Runtime.IsDocker = cfg.Runtime.IsDocker
	return snapshot
}

func nodeConfigured(snapshot managerConfigSnapshot, node NodeID) bool {
	if !snapshot.Valid || !snapshot.Tailscale.TsNet.Enabled {
		return false
	}
	switch node {
	case NodeMain:
		return true
	case NodeManifest:
		return snapshot.Tailscale.TsNet.ExposeManifest &&
			snapshot.Manifest.Enabled &&
			snapshot.Manifest.Port > 0
	case NodeSpaceAgent:
		return snapshot.Tailscale.TsNet.ExposeSpaceAgent &&
			snapshot.SpaceAgent.Enabled &&
			snapshot.SpaceAgent.Port > 0
	default:
		return false
	}
}

func IsValidNodeID(node NodeID) bool {
	switch node {
	case NodeMain, NodeManifest, NodeSpaceAgent:
		return true
	default:
		return false
	}
}

func (m *Manager) authKeyForNode(node NodeID) string {
	key, _ := m.authKeyForNodeWithSource(node)
	return key
}

func (m *Manager) authKeyForNodeWithSource(node NodeID) (string, string) {
	credentials := m.runtimeCredentials()
	credential := credentials.forNode(node)
	return credential.Key, credential.Source
}

func (c credentialSnapshot) forNode(node NodeID) resolvedCredential {
	switch node {
	case NodeMain:
		return c.Main
	case NodeManifest:
		return c.Manifest
	case NodeSpaceAgent:
		return c.SpaceAgent
	default:
		return resolvedCredential{Source: "none"}
	}
}

func resolveCredentials(cfg managerConfigSnapshot) credentialSnapshot {
	return credentialSnapshot{
		Main:       resolveCredential(cfg, NodeMain),
		Manifest:   resolveCredential(cfg, NodeManifest),
		SpaceAgent: resolveCredential(cfg, NodeSpaceAgent),
	}
}

func resolveCredential(cfg managerConfigSnapshot, node NodeID) resolvedCredential {
	if !cfg.Valid {
		return resolvedCredential{Source: "none"}
	}
	tsCfg := cfg.Tailscale.TsNet
	var key string
	switch node {
	case NodeMain:
		key = tsCfg.AuthKeyMain
	case NodeManifest:
		key = tsCfg.AuthKeyManifest
	case NodeSpaceAgent:
		key = tsCfg.AuthKeySpaceAgent
	}
	if key = strings.TrimSpace(key); key != "" {
		return resolvedCredential{Key: key, Source: "node_vault"}
	}
	if key = strings.TrimSpace(tsCfg.AuthKey); key != "" {
		return resolvedCredential{Key: key, Source: "shared_vault"}
	}
	if key = strings.TrimSpace(os.Getenv("TS_AUTHKEY")); key != "" {
		return resolvedCredential{Key: key, Source: "environment"}
	}
	return resolvedCredential{Source: "none"}
}

func (m *Manager) MarkCredentialChanged(node NodeID) {
	if !IsValidNodeID(node) {
		return
	}
	m.loginMu.Lock()
	defer m.loginMu.Unlock()
	runtime := m.nodeRuntime[node]
	if runtime == nil {
		runtime = &nodeRuntimeState{}
		m.nodeRuntime[node] = runtime
	}
	runtime.CredentialChanged = true
}

// BeginReauthenticate starts a serialized, node-specific reauthentication
// operation. It never changes or deletes state unless recover_state was
// explicitly requested and confirmed by the administrator.
func (m *Manager) BeginReauthenticate(node NodeID, mode string, confirmNewIdentity bool, handler http.Handler) (string, error) {
	if !IsValidNodeID(node) {
		return "", fmt.Errorf("invalid tsnet node %q", node)
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "normal"
	}
	if mode != "normal" && mode != "recover_state" {
		return "", fmt.Errorf("invalid reauthentication mode %q", mode)
	}
	if mode == "recover_state" && !confirmNewIdentity {
		return "", fmt.Errorf("%s: state recovery requires confirm_new_identity", ErrorStateCorrupt)
	}
	cfg := m.configSnapshot()
	if !nodeConfigured(cfg, node) {
		return "", fmt.Errorf("%s: node %s is disabled or has no valid listener configuration", ErrorNodeNotConfigured, node)
	}
	ctx, operationID, err := m.beginOperation("reauth_"+mode, node)
	if err != nil {
		return "", err
	}
	go func() {
		var operationErr error
		if mode == "recover_state" {
			operationErr = m.recoverNodeState(ctx, node, handler)
		} else {
			operationErr = m.reauthenticateNode(ctx, node, handler)
		}
		m.setNodeError(node, operationErr)
		m.finishOperation(operationID, operationErr)
	}()
	return operationID, nil
}

func (m *Manager) reauthenticateNode(ctx context.Context, node NodeID, handler http.Handler) error {
	key := m.authKeyForNode(node)
	if key == "" {
		return fmt.Errorf("%s: auth key for node %s is missing", ErrorAuthKeyMissing, node)
	}

	m.mu.Lock()
	mainNode := m.server
	manifestNode := m.manifest.Node
	spaceNode := m.spaceAgent.Node
	m.mu.Unlock()

	switch node {
	case NodeMain:
		if mainNode == nil {
			return m.start(ctx, handler)
		}
		return m.authenticateNode(ctx, node, mainNode, key, true)
	case NodeManifest:
		if manifestNode == nil {
			return m.startManifestListener(ctx, nil)
		}
		return m.authenticateNode(ctx, node, manifestNode, key, true)
	case NodeSpaceAgent:
		if spaceNode == nil {
			return m.startSpaceAgentListener(ctx)
		}
		return m.authenticateNode(ctx, node, spaceNode, key, true)
	default:
		return fmt.Errorf("invalid tsnet node %q", node)
	}
}

func (m *Manager) recoverNodeState(ctx context.Context, node NodeID, handler http.Handler) error {
	m.loginMu.Lock()
	runtime := m.nodeRuntime[node]
	currentErrorCode := ""
	if runtime != nil {
		currentErrorCode = runtime.ErrorCode
	}
	m.loginMu.Unlock()
	if currentErrorCode != ErrorStateCorrupt {
		return fmt.Errorf("%s: state recovery is not allowed for current node state", ErrorStateCorrupt)
	}
	if m.authKeyForNode(node) == "" {
		return fmt.Errorf("%s: auth key for node %s is missing", ErrorAuthKeyMissing, node)
	}

	stateDir, err := m.stateDirForNode(node)
	if err != nil {
		return err
	}
	stateDir, err = validateRecoveryStatePathWithFS(stateDir, m.recoveryFS)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrorStateCorrupt, err)
	}
	info, err := m.recoveryFS.Lstat(stateDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: inspect state directory: %w", ErrorStateCorrupt, err)
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s: state directory must not be a symlink", ErrorStateCorrupt)
	}
	originalMode := os.FileMode(0o750)
	if info != nil {
		originalMode = info.Mode().Perm()
	}

	if err := m.stopNodeForRecovery(ctx, node); err != nil {
		return err
	}

	backupDir := fmt.Sprintf("%s.recovery-%d", stateDir, time.Now().UTC().UnixNano())
	if _, statErr := m.recoveryFS.Lstat(stateDir); statErr == nil {
		if err := m.recoveryFS.Rename(stateDir, backupDir); err != nil {
			return fmt.Errorf("%s: move existing state to backup: %w", ErrorStateCorrupt, err)
		}
		if err := m.recoveryFS.Chmod(backupDir, 0o700); err != nil {
			restoreErr := m.restoreRecoveryBackup(stateDir, backupDir, originalMode)
			return joinRecoveryFailure("protect state backup", err, restoreErr)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("%s: inspect state directory: %w", ErrorStateCorrupt, statErr)
	} else {
		if err := m.recoveryFS.Mkdir(backupDir, 0o700); err != nil {
			return fmt.Errorf("%s: create empty state backup: %w", ErrorStateCorrupt, err)
		}
	}
	if err := m.recoveryFS.MkdirAll(stateDir, originalMode); err != nil {
		restoreErr := m.restoreRecoveryBackup(stateDir, backupDir, originalMode)
		return joinRecoveryFailure("create replacement state directory", err, restoreErr)
	}

	startErr := m.startRecoveredNode(ctx, node, handler)
	if startErr == nil {
		startErr = m.writeRecoveryMarker(stateDir, backupDir)
	}
	if startErr == nil {
		return nil
	}

	stopErr := m.stopNodeForRecovery(ctx, node)
	restoreErr := m.restoreRecoveryBackup(stateDir, backupDir, originalMode)
	if restoreErr != nil {
		return fmt.Errorf("%s: recovery failed and backup was retained; recovery=%s; stop=%s; rollback=%s",
			ErrorStateCorrupt,
			safeErrorMessage(startErr),
			safeErrorMessage(stopErr),
			safeErrorMessage(restoreErr))
	}
	if stopErr != nil {
		return fmt.Errorf("%s: recovered node failed validation and cleanup was incomplete: %w", ErrorStateCorrupt, stopErr)
	}
	return fmt.Errorf("%s: recovered node failed validation; original state restored: %w", ErrorStateCorrupt, startErr)
}

func joinRecoveryFailure(action string, actionErr, rollbackErr error) error {
	if rollbackErr == nil {
		return fmt.Errorf("%s: %s failed; original state restored: %w", ErrorStateCorrupt, action, actionErr)
	}
	return fmt.Errorf("%s: %s failed and rollback failed; backup was retained; action=%s; rollback=%s",
		ErrorStateCorrupt,
		action,
		safeErrorMessage(actionErr),
		safeErrorMessage(rollbackErr))
}

func (m *Manager) restoreRecoveryBackup(stateDir, backupDir string, originalMode os.FileMode) error {
	validated, err := validateRecoveryStatePathWithFS(stateDir, m.recoveryFS)
	if err != nil {
		return fmt.Errorf("validate replacement state: %w", err)
	}
	if info, statErr := m.recoveryFS.Lstat(validated); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("replacement state became a symlink")
		}
		if err := m.recoveryFS.RemoveAll(validated); err != nil {
			return fmt.Errorf("remove replacement state: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect replacement state: %w", statErr)
	}
	if err := m.recoveryFS.Chmod(backupDir, originalMode); err != nil {
		return fmt.Errorf("restore state backup permissions: %w", err)
	}
	if err := m.recoveryFS.Rename(backupDir, validated); err != nil {
		return fmt.Errorf("restore state backup: %w", err)
	}
	return nil
}

func (m *Manager) stopNodeForRecovery(ctx context.Context, node NodeID) error {
	switch node {
	case NodeMain:
		return m.stopRuntime(ctx)
	case NodeManifest:
		return m.stopManifestListener(ctx)
	case NodeSpaceAgent:
		return m.stopSpaceAgentListener(ctx)
	default:
		return fmt.Errorf("invalid tsnet node %q", node)
	}
}

func (m *Manager) startRecoveredNode(ctx context.Context, node NodeID, handler http.Handler) error {
	switch node {
	case NodeMain:
		return m.start(ctx, handler)
	case NodeManifest:
		return m.startManifestListener(ctx, nil)
	case NodeSpaceAgent:
		return m.startSpaceAgentListener(ctx)
	default:
		return fmt.Errorf("invalid tsnet node %q", node)
	}
}

func (m *Manager) stateDirForNode(node NodeID) (string, error) {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return "", fmt.Errorf("tsnet config is unavailable")
	}
	base := strings.TrimSpace(cfg.Tailscale.TsNet.StateDir)
	if base == "" {
		base = filepath.Join("data", "tsnet")
	}
	switch node {
	case NodeMain:
		return base, nil
	case NodeManifest:
		return filepath.Join(base, "manifest"), nil
	case NodeSpaceAgent:
		return filepath.Join(base, "space-agent"), nil
	default:
		return "", fmt.Errorf("invalid tsnet node %q", node)
	}
}

func validateRecoveryStatePath(path string) (string, error) {
	return validateRecoveryStatePathWithFS(path, osRecoveryFileSystem{})
}

func validateRecoveryStatePathWithFS(path string, fs recoveryFileSystem) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("state directory is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve state directory: %w", err)
	}
	absolute = filepath.Clean(absolute)
	volumeRoot := filepath.Clean(filepath.VolumeName(absolute) + string(os.PathSeparator))
	if absolute == volumeRoot {
		return "", fmt.Errorf("filesystem root is not a valid state directory")
	}
	home, _ := os.UserHomeDir()
	if home != "" && samePath(absolute, home) {
		return "", fmt.Errorf("home directory is not a valid state directory")
	}
	workingDir, _ := os.Getwd()
	if workingDir != "" && samePath(absolute, workingDir) {
		return "", fmt.Errorf("install directory is not a valid state directory")
	}
	for current := absolute; current != volumeRoot; current = filepath.Dir(current) {
		info, lstatErr := fs.Lstat(current)
		if lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("state path contains symlink %q", current)
		}
		if lstatErr != nil && !os.IsNotExist(lstatErr) {
			return "", fmt.Errorf("inspect state path: %w", lstatErr)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return absolute, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if filepath.VolumeName(left) != "" && strings.EqualFold(filepath.VolumeName(left), filepath.VolumeName(right)) {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func writeRecoveryMarker(stateDir, backupDir string) error {
	return writeRecoveryMarkerWithFS(osRecoveryFileSystem{}, stateDir, backupDir)
}

func (m *Manager) writeRecoveryMarker(stateDir, backupDir string) error {
	return writeRecoveryMarkerWithFS(m.recoveryFS, stateDir, backupDir)
}

func writeRecoveryMarkerWithFS(fs recoveryFileSystem, stateDir, backupDir string) error {
	if filepath.Dir(stateDir) != filepath.Dir(backupDir) || !strings.HasPrefix(filepath.Base(backupDir), filepath.Base(stateDir)+".recovery-") {
		return fmt.Errorf("unsafe recovery backup path")
	}
	marker := stateDir + recoveryMarkerSuffix
	if err := fs.WriteFile(marker, []byte(filepath.Base(backupDir)), 0o600); err != nil {
		return fmt.Errorf("record state backup: %w", err)
	}
	return nil
}

func cleanupRecoveryBackup(stateDir string) error {
	marker := stateDir + recoveryMarkerSuffix
	raw, err := os.ReadFile(marker)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read recovery marker: %w", err)
	}
	name := strings.TrimSpace(string(raw))
	if name == "" || filepath.Base(name) != name || !strings.HasPrefix(name, filepath.Base(stateDir)+".recovery-") {
		return fmt.Errorf("invalid recovery marker")
	}
	backupDir := filepath.Join(filepath.Dir(stateDir), name)
	validatedState, err := validateRecoveryStatePath(stateDir)
	if err != nil {
		return err
	}
	if filepath.Dir(backupDir) != filepath.Dir(validatedState) {
		return fmt.Errorf("recovery backup escaped state parent")
	}
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("remove verified recovery backup: %w", err)
	}
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove recovery marker: %w", err)
	}
	return nil
}

func (m *Manager) beginOperation(kind string, node NodeID) (context.Context, string, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if m.shuttingDown && kind != "stop" {
		return nil, "", fmt.Errorf("%s: manager is shutting down", ErrorOperationConflict)
	}
	if m.operation != nil && (m.operation.State == "pending" || m.operation.State == "running") {
		return nil, "", fmt.Errorf("%s: operation %s is already running", ErrorOperationConflict, m.operation.ID)
	}
	id := fmt.Sprintf("tsnet-%d-%d", time.Now().UnixMilli(), operationSequence.Add(1))
	ctx, cancel := context.WithCancel(context.Background())
	cfg := m.configSnapshot()
	m.operationPlan = &runtimePlan{
		Config:      cfg,
		Credentials: resolveCredentials(cfg),
	}
	m.operation = &OperationStatus{
		ID:        id,
		Type:      kind,
		Node:      node,
		State:     "running",
		StartedAt: time.Now().UTC(),
	}
	m.operationCancel = cancel
	m.operationDone = make(chan struct{})
	return ctx, id, nil
}

func (m *Manager) finishOperation(id string, err error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if m.operation == nil || m.operation.ID != id {
		return
	}
	m.operation.EndedAt = time.Now().UTC()
	if err != nil {
		m.operation.State = "failed"
		m.operation.ErrorCode = classifyError(err)
		m.operation.Error = safeErrorMessage(err)
	} else {
		m.operation.State = "succeeded"
	}
	if m.operationCancel != nil {
		m.operationCancel()
		m.operationCancel = nil
	}
	if m.operationDone != nil {
		close(m.operationDone)
		m.operationDone = nil
	}
	m.operationPlan = nil
}

func (m *Manager) runtimeConfig() *managerConfigSnapshot {
	m.operationMu.Lock()
	if m.operationPlan != nil {
		cfgCopy := m.operationPlan.Config
		m.operationMu.Unlock()
		return &cfgCopy
	}
	m.operationMu.Unlock()
	cfgCopy := m.configSnapshot()
	if !cfgCopy.Valid {
		return nil
	}
	return &cfgCopy
}

func (m *Manager) configSnapshot() managerConfigSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) runtimeCredentials() credentialSnapshot {
	m.operationMu.Lock()
	if m.operationPlan != nil {
		credentials := m.operationPlan.Credentials
		m.operationMu.Unlock()
		return credentials
	}
	m.operationMu.Unlock()
	return resolveCredentials(m.configSnapshot())
}

func (m *Manager) operationSnapshot() *OperationStatus {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if m.operation == nil {
		return nil
	}
	copy := *m.operation
	return &copy
}

// WaitOperation waits for the specified operation to reach a terminal state.
// It exposes only the already-sanitized operation error.
func (m *Manager) WaitOperation(ctx context.Context, operationID string) error {
	for {
		m.operationMu.Lock()
		if m.operation == nil || m.operation.ID != operationID {
			m.operationMu.Unlock()
			return fmt.Errorf("%s: operation is no longer available", ErrorOperationConflict)
		}
		state := m.operation.State
		errorCode := m.operation.ErrorCode
		errorMessage := m.operation.Error
		done := m.operationDone
		m.operationMu.Unlock()

		switch state {
		case "succeeded":
			return nil
		case "failed":
			if errorCode == "" {
				errorCode = ErrorBackendUnavailable
			}
			return fmt.Errorf("%s: %s", errorCode, errorMessage)
		}
		if done == nil {
			continue
		}
		select {
		case <-done:
		case <-ctx.Done():
			return fmt.Errorf("%s: wait for operation: %w", ErrorTimeout, ctx.Err())
		}
	}
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, strings.ToLower(ErrorOperationConflict)):
		return ErrorOperationConflict
	case strings.Contains(msg, strings.ToLower(ErrorNodeNotConfigured)):
		return ErrorNodeNotConfigured
	case strings.Contains(msg, "auth") && (strings.Contains(msg, "invalid") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "rejected")):
		return ErrorAuthKeyRejected
	case strings.Contains(msg, "auth") && strings.Contains(msg, "missing"):
		return ErrorAuthKeyMissing
	case strings.Contains(msg, "expired") && strings.Contains(msg, "node"):
		return ErrorNodeKeyExpired
	case strings.Contains(msg, "needslogin") || strings.Contains(msg, "needs login") || strings.Contains(msg, "login required"):
		return ErrorLoginRequired
	case strings.Contains(msg, "state") && (strings.Contains(msg, "corrupt") || strings.Contains(msg, "decode") || strings.Contains(msg, "invalid")):
		return ErrorStateCorrupt
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out") || errors.Is(err, context.DeadlineExceeded):
		return ErrorTimeout
	case strings.Contains(msg, "funnel"):
		return ErrorFunnelUnavailable
	case strings.Contains(msg, "certificate") || strings.Contains(msg, "cert") || strings.Contains(msg, "https not available"):
		return ErrorCertUnavailable
	default:
		return ErrorBackendUnavailable
	}
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch classifyError(err) {
	case ErrorAuthKeyMissing:
		return "A Tailscale auth key is required for this node"
	case ErrorAuthKeyRejected:
		return "Tailscale rejected the configured auth key"
	case ErrorLoginRequired:
		return "Tailscale login is required"
	case ErrorNodeKeyExpired:
		return "The Tailscale node key has expired"
	case ErrorStateCorrupt:
		return "The Tailscale state could not be initialized safely"
	case ErrorCertUnavailable:
		return "Tailscale HTTPS certificate provisioning is unavailable"
	case ErrorFunnelUnavailable:
		return "Tailscale Funnel is unavailable"
	case ErrorTimeout:
		return "The tsnet operation timed out"
	case ErrorOperationConflict:
		return "Another tsnet operation is already running"
	case ErrorNodeNotConfigured:
		return "The selected tsnet node is not enabled or is not fully configured"
	default:
		return "The tsnet backend is unavailable"
	}
}

func (m *Manager) setNodeError(node NodeID, err error) {
	m.loginMu.Lock()
	defer m.loginMu.Unlock()
	runtime := m.nodeRuntime[node]
	if runtime == nil {
		runtime = &nodeRuntimeState{}
		m.nodeRuntime[node] = runtime
	}
	if err == nil {
		runtime.ErrorCode = ""
		runtime.ErrorMessage = ""
		return
	}
	runtime.ErrorCode = classifyError(err)
	runtime.ErrorMessage = safeErrorMessage(err)
}

func (m *Manager) makeLoginAwareLogFunc(node NodeID, debug bool) func(string, ...any) {
	_ = debug
	return func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if strings.Contains(msg, "login.tailscale.com") {
			url := extractLoginURL(msg)
			m.loginMu.Lock()
			runtime := m.nodeRuntime[node]
			if runtime == nil {
				runtime = &nodeRuntimeState{}
				m.nodeRuntime[node] = runtime
			}
			if url != "" && url != runtime.LoginURL {
				runtime.LoginURL = url
				runtime.LoginURLSeen = false
			}
			should := !runtime.LoginURLSeen
			if should {
				runtime.LoginURLSeen = true
			}
			m.loginMu.Unlock()
			if should {
				m.logger.Warn("[tsnet] Authentication required; open the node-specific login URL in Tailscale settings", "node", node)
			}
			return
		}
		// tsnet backend messages are not a stable or secret-safe interface.
		// Unknown upstream text is deliberately suppressed.
	}
}

func nodeNeedsAuthentication(status *ipnstate.Status) bool {
	if status == nil {
		return true
	}
	if status.BackendState == fmt.Sprint(ipn.Running) && status.HaveNodeKey && status.Self != nil && !status.Self.Expired {
		return false
	}
	return status.BackendState == fmt.Sprint(ipn.NoState) ||
		status.BackendState == fmt.Sprint(ipn.NeedsLogin) ||
		!status.HaveNodeKey ||
		(status.Self != nil && status.Self.Expired)
}

func rejectedAuthKeyError(status *ipnstate.Status) error {
	if status == nil {
		return nil
	}
	for _, health := range status.Health {
		lower := strings.ToLower(health)
		if strings.Contains(lower, "invalid") && (strings.Contains(lower, "key") || strings.Contains(lower, "auth")) {
			return fmt.Errorf("%s: Tailscale rejected the configured auth key", ErrorAuthKeyRejected)
		}
		if strings.Contains(lower, "api key does not exist") {
			return fmt.Errorf("%s: Tailscale rejected the configured auth key", ErrorAuthKeyRejected)
		}
	}
	return nil
}

func authActionErrorCode(err error) string {
	if err == nil {
		return ""
	}
	lower := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled),
		strings.Contains(lower, "timeout"), strings.Contains(lower, "timed out"):
		return ErrorTimeout
	case (strings.Contains(lower, "auth") || strings.Contains(lower, "key")) &&
		(strings.Contains(lower, "invalid") || strings.Contains(lower, "rejected") || strings.Contains(lower, "does not exist")):
		return ErrorAuthKeyRejected
	default:
		return ErrorBackendUnavailable
	}
}

func (m *Manager) ensureNodeAuthenticated(parent context.Context, node NodeID, srv *tsnet.Server, authKey string) error {
	return m.authenticateNode(parent, node, srv, authKey, false)
}

func (m *Manager) authenticateNode(parent context.Context, node NodeID, srv *tsnet.Server, authKey string, force bool) error {
	lc, err := m.localClientForServer(srv)
	if err != nil {
		return fmt.Errorf("%s: obtain local client: %w", ErrorBackendUnavailable, err)
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()

	status, err := lc.Status(ctx)
	if err != nil {
		return fmt.Errorf("%s: read backend status: %w", ErrorBackendUnavailable, err)
	}
	if !force && !nodeNeedsAuthentication(status) {
		return nil
	}
	if strings.TrimSpace(authKey) == "" {
		return fmt.Errorf("%s: node %s requires authentication but no auth key is configured", ErrorAuthKeyMissing, node)
	}

	prefs, err := lc.GetPrefs(ctx)
	if err != nil {
		return fmt.Errorf("%s: read node preferences: %w", ErrorBackendUnavailable, err)
	}
	if prefs == nil {
		prefs = ipn.NewPrefs()
	}
	if err := lc.Start(ctx, ipn.Options{AuthKey: authKey, UpdatePrefs: prefs}); err != nil {
		return fmt.Errorf("%s: apply auth key: %w", authActionErrorCode(err), err)
	}
	if updatedStatus, statusErr := lc.Status(ctx); statusErr == nil {
		status = updatedStatus
		if !nodeNeedsAuthentication(status) && status.BackendState == fmt.Sprint(ipn.Running) {
			m.markNodeAuthenticated(node)
			return nil
		}
	}
	if err := lc.StartLoginInteractive(ctx); err != nil {
		if updatedStatus, statusErr := lc.Status(ctx); statusErr != nil || nodeNeedsAuthentication(updatedStatus) {
			return fmt.Errorf("%s: start login: %w", authActionErrorCode(err), err)
		}
		m.markNodeAuthenticated(node)
		return nil
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if status != nil && status.Self != nil && status.Self.Expired {
				return fmt.Errorf("%s: Tailscale node key is still expired after reauthentication", ErrorNodeKeyExpired)
			}
			return fmt.Errorf("%s: authentication for node %s timed out: %w", ErrorTimeout, node, ctx.Err())
		case <-ticker.C:
			status, err = lc.Status(ctx)
			if err != nil {
				continue
			}
			if authErr := rejectedAuthKeyError(status); authErr != nil {
				return authErr
			}
			if !nodeNeedsAuthentication(status) && status.BackendState == fmt.Sprint(ipn.Running) {
				m.markNodeAuthenticated(node)
				return nil
			}
		}
	}
}

func (m *Manager) markNodeAuthenticated(node NodeID) {
	m.loginMu.Lock()
	if runtime := m.nodeRuntime[node]; runtime != nil {
		runtime.LoginURL = ""
		runtime.CredentialChanged = false
	}
	m.loginMu.Unlock()
}

// Start initializes the tsnet server and begins serving.
// The provided handler is AuraGo's authenticated web UI/API handler.
func (m *Manager) Start(handler http.Handler) error {
	ctx, operationID, err := m.beginOperation("start", NodeMain)
	if err != nil {
		return err
	}
	err = m.start(ctx, handler)
	m.finishOperation(operationID, err)
	return err
}

// BeginStart registers and starts an asynchronous start operation. Registering
// the operation before returning prevents callers from observing an idle gap
// between the API response and the background start.
func (m *Manager) BeginStart(handler http.Handler) (string, error) {
	ctx, operationID, err := m.beginOperation("start", NodeMain)
	if err != nil {
		return "", err
	}
	go func() {
		operationErr := m.start(ctx, handler)
		m.finishOperation(operationID, operationErr)
	}()
	return operationID, nil
}

func (m *Manager) start(ctx context.Context, handler http.Handler) error {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return fmt.Errorf("tsnet config is unavailable")
	}
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tsnet node is already running")
	}
	if m.starting {
		m.mu.Unlock()
		return fmt.Errorf("tsnet node is already starting")
	}
	hasStaleResources := m.server != nil ||
		m.listener != nil ||
		m.httpSrv != nil ||
		m.homepageLn != nil ||
		m.homepageSrv != nil ||
		m.manifest.Node != nil ||
		m.manifest.Listener != nil ||
		m.manifest.Server != nil ||
		m.spaceAgent.Node != nil ||
		m.spaceAgent.Listener != nil ||
		m.spaceAgent.Server != nil ||
		len(m.storeProxyLns) > 0 ||
		len(m.storeProxySrvs) > 0
	m.mu.Unlock()
	if hasStaleResources {
		if err := m.stopRuntime(ctx); err != nil {
			return fmt.Errorf("clean stale tsnet resources before start: %w", err)
		}
	}

	tsCfg := cfg.Tailscale.TsNet
	if !tsCfg.Enabled {
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

	// Mark as starting so GetStatus() can report the state while we wait for auth.
	// We release m.mu here because srv.ListenTLS blocks until the Tailscale node is
	// fully authenticated and a TLS cert has been issued — potentially a very long wait
	// when interactive login is required.  Holding m.mu during that wait would deadlock
	// every concurrent GetStatus() / Stop() call.
	m.mu.Lock()
	if m.running || m.starting {
		m.mu.Unlock()
		return fmt.Errorf("%s: tsnet lifecycle changed while starting", ErrorOperationConflict)
	}
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
		cleanup(safeErrorMessage(err))
		wrapped := fmt.Errorf("failed to create tsnet state directory: %w", err)
		m.setNodeError(NodeMain, wrapped)
		return wrapped
	}

	authKey := m.authKeyForNode(NodeMain)

	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		Logf:     m.makeLoginAwareLogFunc(NodeMain, true),
		UserLogf: m.makeLoginAwareLogFunc(NodeMain, false),
	}

	if authKey != "" {
		srv.AuthKey = authKey
	}

	m.logger.Info("Starting tsnet node", "hostname", hostname)

	// Do not publish the server before Start returns. tsnet.Server.Close is not
	// safe concurrently with its one-time initialization.
	if err := srv.Start(); err != nil {
		cleanup(safeErrorMessage(err))
		wrapped := fmt.Errorf("failed to start tsnet server: %w", err)
		m.setNodeError(NodeMain, wrapped)
		return wrapped
	}
	if err := ctx.Err(); err != nil {
		_ = srv.Close()
		cleanup(safeErrorMessage(err))
		return fmt.Errorf("%s: tsnet start cancelled: %w", ErrorTimeout, err)
	}
	m.mu.Lock()
	m.server = srv
	m.mu.Unlock()
	if err := m.ensureNodeAuthenticated(ctx, NodeMain, srv, authKey); err != nil {
		_ = srv.Close()
		m.mu.Lock()
		m.server = nil
		m.mu.Unlock()
		m.setNodeError(NodeMain, err)
		cleanup(safeErrorMessage(err))
		return err
	}
	m.setNodeError(NodeMain, nil)

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
		m.manifest = childResourceState{Generation: m.manifest.Generation}
		m.spaceAgent = childResourceState{Generation: m.spaceAgent.Generation}
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

	if err := m.reconfigureExposure(ctx, handler); err != nil {
		m.mu.Lock()
		if m.server != nil {
			m.server.Close()
		}
		m.server = nil
		m.running = false
		m.starting = false
		m.mu.Unlock()
		cleanup(safeErrorMessage(err))
		m.setNodeError(NodeMain, err)
		return err
	}
	if err := cleanupRecoveryBackup(stateDir); err != nil {
		m.logger.Warn("[tsnet] Retaining recovery backup after cleanup failure", "node", NodeMain, "error_code", ErrorStateCorrupt)
	}

	if !tsCfg.ServeHTTP && !tsCfg.ExposeHomepage && !tsCfg.ExposeManifest && !tsCfg.ExposeSpaceAgent {
		m.logger.Info("tsnet node connected (network-only mode — no web services exposed over Tailscale)", "hostname", hostname)
	}

	return nil
}

// Stop gracefully shuts down the tsnet node.
func (m *Manager) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return m.StopContext(ctx)
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.operationMu.Lock()
	m.shuttingDown = true
	m.operationMu.Unlock()
	return m.StopContext(ctx)
}

func (m *Manager) StopContext(ctx context.Context) error {
	m.operationMu.Lock()
	cancelCurrent := m.operationCancel
	done := m.operationDone
	m.operationMu.Unlock()
	if cancelCurrent != nil {
		cancelCurrent()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			stopErr := m.stopRuntime(ctx)
			return errors.Join(
				fmt.Errorf("%s: wait for active operation: %w", ErrorTimeout, ctx.Err()),
				stopErr,
			)
		}
	}

	_, operationID, err := m.beginOperation("stop", NodeMain)
	if err != nil {
		return err
	}
	stopErr := m.stopRuntime(ctx)
	m.finishOperation(operationID, stopErr)
	return stopErr
}

func (m *Manager) stopRuntime(ctx context.Context) error {
	m.mu.Lock()
	httpSrv := m.httpSrv
	mainLn := m.listener
	homepageSrv := m.homepageSrv
	homepageLn := m.homepageLn
	manifestResources := m.manifest
	spaceAgentResources := m.spaceAgent
	storeProxySrvs := make(map[string]*http.Server, len(m.storeProxySrvs))
	for id, server := range m.storeProxySrvs {
		storeProxySrvs[id] = server
	}
	storeProxyLns := make(map[string]net.Listener, len(m.storeProxyLns))
	for id, listener := range m.storeProxyLns {
		storeProxyLns[id] = listener
	}
	mainNode := m.server
	m.running = false
	m.starting = false
	m.server = nil
	m.listener = nil
	m.httpSrv = nil
	m.homepageLn = nil
	m.homepageSrv = nil
	m.manifest = childResourceState{Generation: m.manifest.Generation}
	m.spaceAgent = childResourceState{Generation: m.spaceAgent.Generation}
	m.storeProxyLns = nil
	m.storeProxySrvs = nil
	m.storeProxySpecs = nil
	m.servingHTTP = false
	m.homepageUp = false
	m.httpFallback = false
	m.funnelActive = false
	m.mu.Unlock()

	var shutdownErrs []error
	if err := shutdownHTTPResource(ctx, httpSrv, mainLn, "main listener"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	if err := shutdownHTTPResource(ctx, homepageSrv, homepageLn, "Homepage listener"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	if err := shutdownHTTPResource(ctx, manifestResources.Server, manifestResources.Listener, "Manifest listener"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	if err := closeTsnetResource(ctx, manifestResources.Node, "Manifest node"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	if err := shutdownHTTPResource(ctx, spaceAgentResources.Server, spaceAgentResources.Listener, "Space Agent listener"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	if err := closeTsnetResource(ctx, spaceAgentResources.Node, "Space Agent node"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	for id, server := range storeProxySrvs {
		if err := shutdownHTTPResource(ctx, server, storeProxyLns[id], "store app listener "+id); err != nil {
			shutdownErrs = append(shutdownErrs, err)
		}
		delete(storeProxyLns, id)
	}
	for id, listener := range storeProxyLns {
		if err := shutdownHTTPResource(ctx, nil, listener, "store app listener "+id); err != nil {
			shutdownErrs = append(shutdownErrs, err)
		}
	}
	if err := closeTsnetResource(ctx, mainNode, "main node"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	m.logger.Info("tsnet node stopped")
	return errors.Join(shutdownErrs...)
}

func boundedShutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline) <= 10*time.Second {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, 10*time.Second)
}

func shutdownHTTPResource(ctx context.Context, server *http.Server, listener net.Listener, label string) error {
	var shutdownErrs []error
	if server != nil {
		shutdownCtx, cancel := boundedShutdownContext(ctx)
		if err := server.Shutdown(shutdownCtx); err != nil && !isClosedResourceError(err) {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("shutdown %s: %w", label, err))
		}
		cancel()
	}
	if listener != nil {
		if err := listener.Close(); err != nil && !isClosedResourceError(err) {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("close %s: %w", label, err))
		}
	}
	return errors.Join(shutdownErrs...)
}

func closeTsnetResource(ctx context.Context, server *tsnet.Server, label string) error {
	if server == nil {
		return nil
	}
	closeCtx, cancel := boundedShutdownContext(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.Close()
	}()
	select {
	case err := <-done:
		if err != nil && !isClosedResourceError(err) {
			return fmt.Errorf("close %s: %w", label, err)
		}
		return nil
	case <-closeCtx.Done():
		return fmt.Errorf("%s: close %s: %w", ErrorTimeout, label, closeCtx.Err())
	}
}

func isClosedResourceError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return errors.Is(err, net.ErrClosed) ||
		errors.Is(err, http.ErrServerClosed) ||
		strings.Contains(message, "use of closed network connection") ||
		strings.Contains(message, "server closed")
}

// Status returns the current status of the tsnet node.
func (m *Manager) GetStatus() Status {
	cfg := m.configSnapshot()
	m.mu.Lock()
	mainNode := m.server
	manifestNode := m.manifest.Node
	spaceAgentNode := m.spaceAgent.Node
	st := Status{
		Starting:          m.starting,
		ServingHTTP:       m.servingHTTP,
		HomepageServing:   m.homepageUp,
		SpaceAgentServing: m.spaceAgent.State == "ready",
		ManifestServing:   m.manifest.State == "ready",
		HTTPFallback:      m.httpFallback,
		FunnelActive:      m.funnelActive,
		Nodes:             make(map[NodeID]NodeStatus, 3),
	}
	if cfg.Valid {
		st.Hostname = cfg.Tailscale.TsNet.Hostname
	}
	if m.lastErr != "" {
		st.Error = safeErrorMessage(errors.New(m.lastErr))
	}
	for _, spec := range m.storeProxySpecs {
		st.StoreAppProxies = append(st.StoreAppProxies, StoreAppProxyStatus{
			ID:           spec.ID,
			Port:         spec.Port,
			TargetURL:    spec.TargetURL,
			APITargetURL: spec.APITargetURL,
		})
	}
	m.mu.Unlock()

	mainConfigured := nodeConfigured(cfg, NodeMain)
	manifestConfigured := nodeConfigured(cfg, NodeManifest)
	spaceConfigured := nodeConfigured(cfg, NodeSpaceAgent)
	credentials := resolveCredentials(cfg)
	var mainStatus, manifestStatus, spaceStatus NodeStatus
	var statusWG sync.WaitGroup
	statusWG.Add(3)
	go func() {
		defer statusWG.Done()
		mainStatus = m.nodeStatus(NodeMain, mainNode, mainConfigured, st.ServingHTTP || (cfg.Valid && !cfg.Tailscale.TsNet.ServeHTTP), credentials.Main.Source)
	}()
	go func() {
		defer statusWG.Done()
		manifestStatus = m.nodeStatus(NodeManifest, manifestNode, manifestConfigured, st.ManifestServing, credentials.Manifest.Source)
	}()
	go func() {
		defer statusWG.Done()
		spaceStatus = m.nodeStatus(NodeSpaceAgent, spaceAgentNode, spaceConfigured, st.SpaceAgentServing, credentials.SpaceAgent.Source)
	}()
	statusWG.Wait()
	st.Nodes[NodeMain] = mainStatus
	st.Nodes[NodeManifest] = manifestStatus
	st.Nodes[NodeSpaceAgent] = spaceStatus

	st.Running = mainStatus.Running
	st.Ready = mainStatus.Running && (!mainConfigured || mainStatus.ListenerReady)
	st.DNS = mainStatus.DNS
	st.IPs = append(st.IPs, mainStatus.IPs...)
	if mainStatus.DNS != "" {
		st.CertDNS = []string{mainStatus.DNS}
	}
	st.ManifestDNS = manifestStatus.DNS
	st.SpaceAgentDNS = spaceStatus.DNS
	st.LoginURL = mainStatus.LoginURL
	if st.Error == "" && mainStatus.ErrorMessage != "" {
		st.Error = mainStatus.ErrorMessage
	}
	st.Operation = m.operationSnapshot()
	if st.Operation != nil && (st.Operation.State == "pending" || st.Operation.State == "running") {
		st.Starting = true
	}
	return st
}

func (m *Manager) nodeStatus(node NodeID, srv *tsnet.Server, configured, listenerReady bool, keySource string) NodeStatus {
	result := NodeStatus{
		Configured:    configured,
		ListenerReady: listenerReady,
		Health:        "stopped",
		KeySource:     keySource,
	}
	m.loginMu.Lock()
	if runtime := m.nodeRuntime[node]; runtime != nil {
		result.LoginURL = runtime.LoginURL
		result.ErrorCode = runtime.ErrorCode
		result.ErrorMessage = runtime.ErrorMessage
		result.CredentialChanged = runtime.CredentialChanged
	}
	m.loginMu.Unlock()
	if srv == nil {
		if result.ErrorCode != "" {
			result.Health = "error"
		}
		return result
	}
	// A zero-value tsnet.Server is used by focused listener tests. Calling
	// LocalClient on it would initialize a real backend as a side effect of a
	// read-only status request.
	if srv.Dir == "" && srv.Hostname == "" {
		return result
	}
	lc, err := m.localClientForServer(srv)
	if err != nil {
		if result.ErrorCode == "" {
			result.ErrorCode = ErrorBackendUnavailable
			result.ErrorMessage = "Tailscale local client is unavailable"
		}
		result.Health = "error"
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := lc.Status(ctx)
	if err != nil {
		if result.ErrorCode == "" {
			result.ErrorCode = ErrorBackendUnavailable
			result.ErrorMessage = "Tailscale backend status is unavailable"
		}
		result.Health = "error"
		return result
	}
	result.BackendState = status.BackendState
	result.HaveNodeKey = status.HaveNodeKey
	result.Running = status.BackendState == fmt.Sprint(ipn.Running)
	if status.Self != nil {
		result.NodeKeyExpired = status.Self.Expired
		if status.Self.KeyExpiry != nil && !status.Self.KeyExpiry.IsZero() {
			result.KeyExpiry = status.Self.KeyExpiry.UTC().Format(time.RFC3339)
		}
		result.DNS = status.Self.DNSName
		for _, addr := range status.Self.TailscaleIPs {
			result.IPs = append(result.IPs, addr.String())
		}
	}
	switch {
	case result.ErrorCode != "":
		result.Health = "error"
	case result.NodeKeyExpired:
		result.Health = "expired"
		result.ErrorCode = ErrorNodeKeyExpired
		result.ErrorMessage = "Tailscale node key has expired"
	case status.BackendState == fmt.Sprint(ipn.NeedsLogin) || status.BackendState == fmt.Sprint(ipn.NoState) || !status.HaveNodeKey:
		result.Health = "login_required"
		result.ErrorCode = ErrorLoginRequired
		result.ErrorMessage = "Tailscale authentication is required"
	case len(status.Health) > 0:
		result.Health = "degraded"
	case result.Running && result.ListenerReady:
		result.Health = "ready"
	case result.Running:
		result.Health = "degraded"
	default:
		result.Health = "starting"
	}
	if result.Running && len(result.IPs) > 0 {
		m.loginMu.Lock()
		if runtime := m.nodeRuntime[node]; runtime != nil {
			runtime.LoginURL = ""
		}
		m.loginMu.Unlock()
		result.LoginURL = ""
	}
	return result
}

// ReconfigureExposure reconciles the active Tailscale listeners with the
// current config without disconnecting the node from the tailnet.
func (m *Manager) ReconfigureExposure(handler http.Handler) error {
	ctx, operationID, err := m.beginOperation("reconfigure", NodeMain)
	if err != nil {
		return err
	}
	err = m.reconfigureExposure(ctx, handler)
	m.finishOperation(operationID, err)
	return err
}

// BeginReconfigure registers and starts an asynchronous listener reconciliation.
func (m *Manager) BeginReconfigure(handler http.Handler) (string, error) {
	ctx, operationID, err := m.beginOperation("reconfigure", NodeMain)
	if err != nil {
		return "", err
	}
	go func() {
		operationErr := m.reconfigureExposure(ctx, handler)
		m.finishOperation(operationID, operationErr)
	}()
	return operationID, nil
}

func (m *Manager) reconfigureExposure(ctx context.Context, handler http.Handler) error {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return fmt.Errorf("tsnet config is unavailable")
	}
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
	manifestUp := m.manifest.State == "ready"
	activeManifestHost := m.manifest.Host
	manifestHasResources := m.manifest.Node != nil || m.manifest.Listener != nil || m.manifest.Server != nil
	spaceAgentUp := m.spaceAgent.State == "ready"
	activeSpaceAgentHost := m.spaceAgent.Host
	spaceAgentHasResources := m.spaceAgent.Node != nil || m.spaceAgent.Listener != nil || m.spaceAgent.Server != nil
	funnelActive := m.funnelActive
	m.mu.Unlock()

	wantMain := cfg.Tailscale.TsNet.ServeHTTP
	wantFunnel := wantMain && cfg.Tailscale.TsNet.Funnel
	wantHomepage := cfg.Tailscale.TsNet.ExposeHomepage && cfg.Homepage.WebServerEnabled && cfg.Homepage.WebServerPort > 0
	wantManifest := cfg.Tailscale.TsNet.ExposeManifest && cfg.Manifest.Enabled && cfg.Manifest.Port > 0
	wantSpaceAgent := cfg.Tailscale.TsNet.ExposeSpaceAgent && cfg.SpaceAgent.Enabled && cfg.SpaceAgent.Port > 0
	desiredManifestHost := m.effectiveManifestHostname()
	desiredSpaceAgentHost := m.effectiveSpaceAgentHostname()

	if servingHTTP && (!wantMain || funnelActive != wantFunnel) {
		if err := m.stopMainListener(ctx); err != nil {
			return err
		}
		servingHTTP = false
		funnelActive = false
	}
	if homepageUp && !wantHomepage {
		if err := m.stopHomepageListener(ctx); err != nil {
			return err
		}
		homepageUp = false
	}
	if manifestHasResources && (!wantManifest || !manifestUp || activeManifestHost != desiredManifestHost) {
		if err := m.stopManifestListener(ctx); err != nil {
			return err
		}
		manifestUp = false
	}
	if spaceAgentHasResources && (!wantSpaceAgent || !spaceAgentUp || activeSpaceAgentHost != desiredSpaceAgentHost) {
		if err := m.stopSpaceAgentListener(ctx); err != nil {
			return err
		}
		spaceAgentUp = false
	}

	if wantMain && !servingHTTP {
		if err := m.startMainListener(ctx, srv, handler); err != nil {
			return err
		}
	}
	if wantHomepage && !homepageUp {
		if !homepageProxyBackendReachable(cfg.Homepage.WebServerPort, 2*time.Second) {
			err := homepageBackendUnavailableError(cfg.Homepage.WebServerPort)
			m.logger.Warn("[tsnet] Homepage exposure could not be started", "error_code", classifyError(err), "error", safeErrorMessage(err))
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
			m.mu.Unlock()
			m.scheduleHomepageExposureRetry()
		} else if err := m.startHomepageListener(ctx, srv); err != nil {
			m.logger.Warn("[tsnet] Homepage exposure could not be started", "error_code", classifyError(err), "error", safeErrorMessage(err))
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
			m.mu.Unlock()
		}
	}
	if wantManifest && !manifestUp {
		if err := m.startManifestListener(ctx, srv); err != nil {
			m.logger.Warn("[tsnet] Manifest exposure could not be started", "error_code", classifyError(err), "error", safeErrorMessage(err))
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
			m.mu.Unlock()
		}
	}
	if wantSpaceAgent && !spaceAgentUp {
		if err := m.startSpaceAgentListener(ctx); err != nil {
			m.logger.Warn("[tsnet] Space Agent exposure could not be started", "error_code", classifyError(err), "error", safeErrorMessage(err))
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	if (!wantMain || m.servingHTTP) && (!wantHomepage || m.homepageUp) && (!wantManifest || m.manifest.State == "ready") && (!wantSpaceAgent || m.spaceAgent.State == "ready") && (!wantFunnel || m.funnelActive) {
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
		if err := m.stopStoreAppProxy(context.Background(), status.ID); err != nil {
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
			m.logger.Warn("[tsnet] store app proxy stopped", "app_id", spec.ID, "error_code", classifyError(err), "error", safeErrorMessage(err))
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
			logger.Warn("[tsnet] store app proxy backend unavailable", "error_code", classifyError(err), "error", safeErrorMessage(err))
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

func (m *Manager) stopStoreAppProxy(ctx context.Context, id string) error {
	m.mu.Lock()
	httpSrv := m.storeProxySrvs[id]
	ln := m.storeProxyLns[id]
	delete(m.storeProxySrvs, id)
	delete(m.storeProxyLns, id)
	delete(m.storeProxySpecs, id)
	m.mu.Unlock()

	return shutdownHTTPResource(ctx, httpSrv, ln, "store app proxy "+id)
}

func homepageExposureWanted(cfg managerConfigSnapshot) bool {
	return cfg.Valid &&
		cfg.Tailscale.TsNet.ExposeHomepage &&
		cfg.Homepage.WebServerEnabled &&
		cfg.Homepage.WebServerPort > 0
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

		cfg := m.configSnapshot()
		m.mu.Lock()
		if !m.running || m.server == nil || m.homepageUp || !homepageExposureWanted(cfg) {
			m.homepageRetrying = false
			m.mu.Unlock()
			return
		}
		srv := m.server
		m.mu.Unlock()

		ctx, operationID, err := m.beginOperation("reconfigure", NodeMain)
		if err != nil {
			continue
		}
		planCfg := m.runtimeConfig()
		if planCfg == nil || !homepageProxyBackendReachable(planCfg.Homepage.WebServerPort, 2*time.Second) {
			m.finishOperation(operationID, nil)
			continue
		}
		m.mu.Lock()
		stillWanted := m.running && m.server == srv && !m.homepageUp &&
			planCfg.Tailscale.TsNet.ExposeHomepage &&
			planCfg.Homepage.WebServerEnabled &&
			planCfg.Homepage.WebServerPort > 0
		m.mu.Unlock()
		if !stillWanted {
			m.finishOperation(operationID, nil)
			m.mu.Lock()
			m.homepageRetrying = false
			m.mu.Unlock()
			return
		}
		err = m.startHomepageListener(ctx, srv)
		m.finishOperation(operationID, err)
		if err != nil {
			if m.logger != nil {
				m.logger.Warn("[tsnet] Homepage exposure retry failed", "error_code", classifyError(err))
			}
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
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
	return m.ReconfigureExposure(nil)
}

func (m *Manager) startMainListener(ctx context.Context, srv *tsnet.Server, handler http.Handler) error {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return fmt.Errorf("tsnet config is unavailable")
	}
	wantFunnel := cfg.Tailscale.TsNet.Funnel
	usingHTTP := false
	usingFunnel := false

	var (
		ln  net.Listener
		err error
	)

	if wantFunnel {
		ln, err = m.listenFunnelForNode(ctx, srv, ":443", 20*time.Second, true)
		if err != nil {
			// Funnel was explicitly requested but failed — this is a hard error.
			// NEVER silently fall back to TLS or HTTP without explicit user consent.
			// Common reasons: Funnel ACL not granted, Funnel not enabled in admin panel,
			// port 443 already in use, or cert not yet provisioned.
			errMsg := fmt.Errorf("[tsnet] Funnel explicitly enabled but failed: %w", err)
			m.mu.Lock()
			m.lastErr = safeErrorMessage(errMsg)
			m.mu.Unlock()
			return errMsg
		}
		usingFunnel = true
	}

	if ln == nil {
		tlsTimeout := tsnetTLSStrictTimeout
		if cfg.Tailscale.TsNet.AllowHTTPFallback {
			tlsTimeout = tsnetTLSFallbackTimeout
		}
		ln, err = m.listenTLSForNode(ctx, srv, ":443", tlsTimeout, true)
		if err != nil {
			if !cfg.Tailscale.TsNet.AllowHTTPFallback {
				return fmt.Errorf("[tsnet] HTTPS not available and allow_http_fallback is disabled: %w", err)
			}
			replacement, replaceErr := m.rebuildMainNode(ctx, srv)
			if replaceErr != nil {
				return fmt.Errorf("rebuild main tsnet node for HTTP fallback: %w", replaceErr)
			}
			srv = replacement
			m.logger.Warn("[tsnet] HTTPS not available — falling back to HTTP on :80",
				"error_code", classifyError(err),
				"error", safeErrorMessage(err),
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
			m.logger.Error("tsnet AuraGo listener error", "error_code", classifyError(err), "error", safeErrorMessage(err))
			m.setNodeError(NodeMain, err)
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
			m.servingHTTP = false
			m.httpFallback = false
			m.funnelActive = false
			m.mu.Unlock()
		}
	}()

	m.setNodeError(NodeMain, nil)
	return nil
}

func (m *Manager) rebuildMainNode(ctx context.Context, previous *tsnet.Server) (*tsnet.Server, error) {
	if previous != nil {
		_ = previous.Close()
	}
	cfg := m.runtimeConfig()
	if cfg == nil {
		return nil, fmt.Errorf("tsnet config is unavailable")
	}
	tsCfg := cfg.Tailscale.TsNet
	stateDir := strings.TrimSpace(tsCfg.StateDir)
	if stateDir == "" {
		stateDir = "data/tsnet"
	}
	hostname := strings.TrimSpace(tsCfg.Hostname)
	if hostname == "" {
		hostname = "aurago"
	}
	authKey := m.authKeyForNode(NodeMain)
	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		Logf:     m.makeLoginAwareLogFunc(NodeMain, true),
		UserLogf: m.makeLoginAwareLogFunc(NodeMain, false),
	}
	if authKey != "" {
		srv.AuthKey = authKey
	}
	if err := srv.Start(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = srv.Close()
		return nil, fmt.Errorf("%s: replacement tsnet start cancelled: %w", ErrorTimeout, err)
	}
	m.mu.Lock()
	m.server = srv
	m.mu.Unlock()
	if err := m.ensureNodeAuthenticated(ctx, NodeMain, srv, authKey); err != nil {
		_ = srv.Close()
		m.mu.Lock()
		if m.server == srv {
			m.server = nil
		}
		m.mu.Unlock()
		return nil, err
	}
	return srv, nil
}

func (m *Manager) stopMainListener(ctx context.Context) error {
	m.mu.Lock()
	httpSrv := m.httpSrv
	ln := m.listener
	m.httpSrv = nil
	m.listener = nil
	m.servingHTTP = false
	m.httpFallback = false
	m.funnelActive = false
	m.mu.Unlock()

	return shutdownHTTPResource(ctx, httpSrv, ln, "tsnet AuraGo listener")
}

func (m *Manager) startHomepageListener(ctx context.Context, srv *tsnet.Server) error {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return fmt.Errorf("tsnet config is unavailable")
	}
	if !homepageProxyBackendReachable(cfg.Homepage.WebServerPort, 2*time.Second) {
		return homepageBackendUnavailableError(cfg.Homepage.WebServerPort)
	}
	targetURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(cfg.Homepage.WebServerPort))
	if err != nil {
		return fmt.Errorf("invalid homepage proxy target: %w", err)
	}

	ln, err := listenTLSWithFunction(ctx, srv, ":8443", tsnetTLSStrictTimeout, false, listenTLSWithTimeoutFn)
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
		m.logger.Warn("[tsnet] Homepage reverse proxy failed", "error_code", classifyError(proxyErr), "error", safeErrorMessage(proxyErr))
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
			m.logger.Error("tsnet Homepage listener error", "error_code", classifyError(err), "error", safeErrorMessage(err))
			m.mu.Lock()
			m.lastErr = safeErrorMessage(err)
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

func (m *Manager) stopHomepageListener(ctx context.Context) error {
	m.mu.Lock()
	httpSrv := m.homepageSrv
	ln := m.homepageLn
	m.homepageSrv = nil
	m.homepageLn = nil
	m.homepageUp = false
	m.mu.Unlock()

	return shutdownHTTPResource(ctx, httpSrv, ln, "tsnet Homepage listener")
}

func manifestProxyTarget(cfg any) string {
	var snapshot managerConfigSnapshot
	switch value := cfg.(type) {
	case *managerConfigSnapshot:
		if value == nil {
			return "http://127.0.0.1:2099"
		}
		snapshot = *value
	case *config.Config:
		snapshot = snapshotManagerConfig(value)
	default:
		return "http://127.0.0.1:2099"
	}
	port := snapshot.Manifest.Port
	if port <= 0 {
		port = 2099
	}
	if snapshot.Runtime.IsDocker {
		return "http://manifest:" + strconv.Itoa(port)
	}
	hostPort := snapshot.Manifest.HostPort
	if hostPort <= 0 {
		hostPort = port
	}
	return "http://127.0.0.1:" + strconv.Itoa(hostPort)
}

func (m *Manager) effectiveManifestHostname() string {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return "aurago-manifest"
	}
	hostname := strings.TrimSpace(cfg.Tailscale.TsNet.ManifestHostname)
	if hostname != "" {
		return hostname
	}
	base := strings.TrimSpace(cfg.Tailscale.TsNet.Hostname)
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

func (m *Manager) startManifestListener(ctx context.Context, _ *tsnet.Server) error {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return fmt.Errorf("tsnet config is unavailable")
	}
	if !nodeConfigured(*cfg, NodeManifest) {
		return fmt.Errorf("%s: node %s is disabled or has no valid listener configuration", ErrorNodeNotConfigured, NodeManifest)
	}
	if err := m.stopManifestListener(ctx); err != nil {
		return err
	}
	targetURL, err := url.Parse(manifestProxyTarget(cfg))
	if err != nil {
		return fmt.Errorf("invalid Manifest proxy target: %w", err)
	}
	tsCfg := cfg.Tailscale.TsNet
	stateDir := tsCfg.StateDir
	if stateDir == "" {
		stateDir = "data/tsnet"
	}
	manifestStateDir := filepath.Join(stateDir, "manifest")
	if err := os.MkdirAll(manifestStateDir, 0o750); err != nil {
		return fmt.Errorf("create Manifest tsnet state directory: %w", err)
	}
	hostname := m.effectiveManifestHostname()
	authKey := m.authKeyForNode(NodeManifest)
	manifestNode := &tsnet.Server{
		Hostname: hostname,
		Dir:      manifestStateDir,
		Logf:     m.makeLoginAwareLogFunc(NodeManifest, true),
		UserLogf: m.makeLoginAwareLogFunc(NodeManifest, false),
	}
	if authKey != "" {
		manifestNode.AuthKey = authKey
	}
	m.mu.Lock()
	generation := m.manifest.Generation + 1
	m.manifest = childResourceState{
		Generation: generation,
		Host:       hostname,
		State:      "starting",
	}
	m.mu.Unlock()
	if err := manifestNode.Start(); err != nil {
		resources := m.detachChildResources(NodeManifest, generation)
		resources.Node = manifestNode
		_ = m.closeChildResources(ctx, NodeManifest, resources)
		wrapped := fmt.Errorf("start Manifest tsnet node: %w", err)
		m.setNodeError(NodeManifest, wrapped)
		return wrapped
	}
	if err := ctx.Err(); err != nil {
		resources := m.detachChildResources(NodeManifest, generation)
		resources.Node = manifestNode
		_ = m.closeChildResources(context.Background(), NodeManifest, resources)
		return fmt.Errorf("%s: Manifest node start cancelled: %w", ErrorTimeout, err)
	}
	m.mu.Lock()
	if m.manifest.Generation != generation || m.manifest.State != "starting" {
		m.mu.Unlock()
		_ = m.closeChildResources(ctx, NodeManifest, childResourceState{Node: manifestNode})
		return fmt.Errorf("%s: Manifest node start was superseded", ErrorOperationConflict)
	}
	m.manifest.Node = manifestNode
	m.mu.Unlock()
	if err := m.ensureNodeAuthenticated(ctx, NodeManifest, manifestNode, authKey); err != nil {
		resources := m.detachChildResources(NodeManifest, generation)
		_ = m.closeChildResources(ctx, NodeManifest, resources)
		m.setNodeError(NodeManifest, err)
		return err
	}
	port := manifestTsNetPort(cfg.Tailscale.TsNet.ManifestPort)
	ln, err := m.listenTLSForNode(ctx, manifestNode, ":"+strconv.Itoa(port), tsnetTLSStrictTimeout, true)
	if err != nil {
		resources := m.detachChildResources(NodeManifest, generation)
		resources.Listener = ln
		_ = m.closeChildResources(ctx, NodeManifest, resources)
		wrapped := fmt.Errorf("Manifest exposure requires Tailscale HTTPS on dedicated hostname %q:%d: %w", hostname, port, err)
		m.setNodeError(NodeManifest, wrapped)
		return wrapped
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		m.logger.Warn("[tsnet] Manifest reverse proxy failed", "error_code", classifyError(proxyErr), "error", safeErrorMessage(proxyErr))
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
	if m.manifest.Generation != generation || m.manifest.Node != manifestNode {
		m.mu.Unlock()
		_ = m.closeChildResources(ctx, NodeManifest, childResourceState{
			Generation: generation,
			Node:       manifestNode,
			Listener:   ln,
			Server:     manifestSrv,
			Host:       hostname,
		})
		return fmt.Errorf("%s: Manifest listener start was superseded", ErrorOperationConflict)
	}
	m.manifest.Listener = ln
	m.manifest.Server = manifestSrv
	m.manifest.State = "ready"
	m.mu.Unlock()
	m.setNodeError(NodeManifest, nil)
	if err := cleanupRecoveryBackup(manifestStateDir); err != nil {
		m.logger.Warn("[tsnet] Retaining recovery backup after cleanup failure", "node", NodeManifest, "error_code", ErrorStateCorrupt)
	}

	go func() {
		m.logger.Info("tsnet Manifest listener started", "protocol", "HTTPS", "hostname", hostname, "target", targetURL.String(), "port", port)
		if err := manifestSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.handleChildServeExit(NodeManifest, generation, manifestSrv, err)
		}
	}()

	return nil
}

func (m *Manager) stopManifestListener(ctx context.Context) error {
	return m.closeChildResources(ctx, NodeManifest, m.detachChildResources(NodeManifest, 0))
}

func (m *Manager) detachChildResources(node NodeID, generation uint64) childResourceState {
	m.mu.Lock()
	defer m.mu.Unlock()
	var current *childResourceState
	switch node {
	case NodeManifest:
		current = &m.manifest
	case NodeSpaceAgent:
		current = &m.spaceAgent
	default:
		return childResourceState{}
	}
	if generation != 0 && current.Generation != generation {
		return childResourceState{}
	}
	resources := *current
	*current = childResourceState{Generation: current.Generation}
	return resources
}

func (m *Manager) closeChildResources(ctx context.Context, node NodeID, resources childResourceState) error {
	label := "child"
	switch node {
	case NodeManifest:
		label = "Manifest"
	case NodeSpaceAgent:
		label = "Space Agent"
	}
	var shutdownErrs []error
	if err := shutdownHTTPResource(ctx, resources.Server, resources.Listener, label+" listener"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	if err := closeTsnetResource(ctx, resources.Node, label+" node"); err != nil {
		shutdownErrs = append(shutdownErrs, err)
	}
	return errors.Join(shutdownErrs...)
}

func (m *Manager) handleChildServeExit(node NodeID, generation uint64, server *http.Server, serveErr error) {
	m.mu.Lock()
	var current *childResourceState
	switch node {
	case NodeManifest:
		current = &m.manifest
	case NodeSpaceAgent:
		current = &m.spaceAgent
	}
	if current == nil || current.Generation != generation || current.Server != server {
		m.mu.Unlock()
		return
	}
	resources := *current
	*current = childResourceState{Generation: current.Generation}
	m.lastErr = ErrorBackendUnavailable
	m.mu.Unlock()

	m.setNodeError(node, serveErr)
	if m.logger != nil {
		m.logger.Error("tsnet child listener stopped unexpectedly",
			"node", node,
			"error_code", classifyError(serveErr),
			"error", safeErrorMessage(serveErr))
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.closeChildResources(closeCtx, node, resources); err != nil && m.logger != nil {
		m.logger.Warn("tsnet child listener cleanup failed",
			"node", node,
			"error_code", classifyError(err),
			"error", safeErrorMessage(err))
	}
}

func (m *Manager) effectiveSpaceAgentHostname() string {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return "aurago-space-agent"
	}
	hostname := strings.TrimSpace(cfg.Tailscale.TsNet.SpaceAgentHostname)
	if hostname != "" {
		return hostname
	}
	base := strings.TrimSpace(cfg.Tailscale.TsNet.Hostname)
	if base == "" {
		base = "aurago"
	}
	return base + "-space-agent"
}

func (m *Manager) startSpaceAgentListener(ctx context.Context) error {
	cfg := m.runtimeConfig()
	if cfg == nil {
		return fmt.Errorf("tsnet config is unavailable")
	}
	if !nodeConfigured(*cfg, NodeSpaceAgent) {
		return fmt.Errorf("%s: node %s is disabled or has no valid listener configuration", ErrorNodeNotConfigured, NodeSpaceAgent)
	}
	if err := m.stopSpaceAgentListener(ctx); err != nil {
		return err
	}
	targetURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(cfg.SpaceAgent.Port))
	if err != nil {
		return fmt.Errorf("invalid Space Agent proxy target: %w", err)
	}

	tsCfg := cfg.Tailscale.TsNet
	stateDir := tsCfg.StateDir
	if stateDir == "" {
		stateDir = "data/tsnet"
	}
	spaceStateDir := filepath.Join(stateDir, "space-agent")
	if err := os.MkdirAll(spaceStateDir, 0o750); err != nil {
		return fmt.Errorf("create Space Agent tsnet state directory: %w", err)
	}
	hostname := m.effectiveSpaceAgentHostname()
	authKey := m.authKeyForNode(NodeSpaceAgent)
	spaceNode := &tsnet.Server{
		Hostname: hostname,
		Dir:      spaceStateDir,
		Logf:     m.makeLoginAwareLogFunc(NodeSpaceAgent, true),
		UserLogf: m.makeLoginAwareLogFunc(NodeSpaceAgent, false),
	}
	if authKey != "" {
		spaceNode.AuthKey = authKey
	}
	m.mu.Lock()
	generation := m.spaceAgent.Generation + 1
	m.spaceAgent = childResourceState{
		Generation: generation,
		Host:       hostname,
		State:      "starting",
	}
	m.mu.Unlock()
	if err := spaceNode.Start(); err != nil {
		resources := m.detachChildResources(NodeSpaceAgent, generation)
		resources.Node = spaceNode
		_ = m.closeChildResources(ctx, NodeSpaceAgent, resources)
		wrapped := fmt.Errorf("start Space Agent tsnet node: %w", err)
		m.setNodeError(NodeSpaceAgent, wrapped)
		return wrapped
	}
	if err := ctx.Err(); err != nil {
		resources := m.detachChildResources(NodeSpaceAgent, generation)
		resources.Node = spaceNode
		_ = m.closeChildResources(context.Background(), NodeSpaceAgent, resources)
		return fmt.Errorf("%s: Space Agent node start cancelled: %w", ErrorTimeout, err)
	}
	m.mu.Lock()
	if m.spaceAgent.Generation != generation || m.spaceAgent.State != "starting" {
		m.mu.Unlock()
		_ = m.closeChildResources(ctx, NodeSpaceAgent, childResourceState{Node: spaceNode})
		return fmt.Errorf("%s: Space Agent node start was superseded", ErrorOperationConflict)
	}
	m.spaceAgent.Node = spaceNode
	m.mu.Unlock()
	if err := m.ensureNodeAuthenticated(ctx, NodeSpaceAgent, spaceNode, authKey); err != nil {
		resources := m.detachChildResources(NodeSpaceAgent, generation)
		_ = m.closeChildResources(ctx, NodeSpaceAgent, resources)
		m.setNodeError(NodeSpaceAgent, err)
		return err
	}
	ln, err := m.listenTLSForNode(ctx, spaceNode, ":443", tsnetTLSStrictTimeout, true)
	if err != nil {
		resources := m.detachChildResources(NodeSpaceAgent, generation)
		resources.Listener = ln
		_ = m.closeChildResources(ctx, NodeSpaceAgent, resources)
		wrapped := fmt.Errorf("Space Agent exposure requires Tailscale HTTPS on dedicated hostname %q: %w", hostname, err)
		m.setNodeError(NodeSpaceAgent, wrapped)
		return wrapped
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		m.logger.Warn("[tsnet] Space Agent reverse proxy failed", "error_code", classifyError(proxyErr), "error", safeErrorMessage(proxyErr))
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
	if m.spaceAgent.Generation != generation || m.spaceAgent.Node != spaceNode {
		m.mu.Unlock()
		_ = m.closeChildResources(ctx, NodeSpaceAgent, childResourceState{
			Generation: generation,
			Node:       spaceNode,
			Listener:   ln,
			Server:     spaceAgentSrv,
			Host:       hostname,
		})
		return fmt.Errorf("%s: Space Agent listener start was superseded", ErrorOperationConflict)
	}
	m.spaceAgent.Listener = ln
	m.spaceAgent.Server = spaceAgentSrv
	m.spaceAgent.State = "ready"
	m.mu.Unlock()
	m.setNodeError(NodeSpaceAgent, nil)
	if err := cleanupRecoveryBackup(spaceStateDir); err != nil {
		m.logger.Warn("[tsnet] Retaining recovery backup after cleanup failure", "node", NodeSpaceAgent, "error_code", ErrorStateCorrupt)
	}

	go func() {
		m.logger.Info("tsnet Space Agent listener started", "protocol", "HTTPS", "hostname", hostname, "target", targetURL.String(), "port", 443)
		if err := spaceAgentSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			m.handleChildServeExit(NodeSpaceAgent, generation, spaceAgentSrv, err)
		}
	}()

	return nil
}

func (m *Manager) stopSpaceAgentListener(ctx context.Context) error {
	return m.closeChildResources(ctx, NodeSpaceAgent, m.detachChildResources(NodeSpaceAgent, 0))
}

// listenTLSWithTimeout calls srv.ListenTLS with a timeout so that a slow or
// blocked cert-provisioning call does not stall the entire Start() goroutine.
func listenTLSWithTimeout(srv *tsnet.Server, addr string, timeout time.Duration) (net.Listener, error) {
	return listenTLSWithContext(context.Background(), srv, addr, timeout, false)
}

func listenTLSWithContext(parent context.Context, srv *tsnet.Server, addr string, timeout time.Duration, closeServerOnCancel bool) (net.Listener, error) {
	return listenTLSWithFunction(parent, srv, addr, timeout, closeServerOnCancel, func(srv *tsnet.Server, addr string, _ time.Duration) (net.Listener, error) {
		return srv.ListenTLS("tcp", addr)
	})
}

func listenTLSWithFunction(parent context.Context, srv *tsnet.Server, addr string, timeout time.Duration, closeServerOnCancel bool, listen func(*tsnet.Server, string, time.Duration) (net.Listener, error)) (net.Listener, error) {
	ch := make(chan listenerResult, 1)
	go func() {
		ln, err := listen(srv, addr, timeout)
		ch <- listenerResult{ln, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.ln, r.err
	case <-parent.Done():
		if closeServerOnCancel {
			_ = srv.Close()
		}
		go closeLateListener(ch)
		return nil, fmt.Errorf("ListenTLS cancelled: %w", parent.Err())
	case <-timer.C:
		if closeServerOnCancel {
			_ = srv.Close()
		}
		go closeLateListener(ch)
		return nil, fmt.Errorf("ListenTLS timed out after %s (HTTPS cert not ready)", timeout)
	}
}

func listenFunnelWithTimeout(srv *tsnet.Server, addr string, timeout time.Duration) (net.Listener, error) {
	return listenFunnelWithContext(context.Background(), srv, addr, timeout, false)
}

func listenFunnelWithContext(parent context.Context, srv *tsnet.Server, addr string, timeout time.Duration, closeServerOnCancel bool) (net.Listener, error) {
	ch := make(chan listenerResult, 1)
	go func() {
		ln, err := srv.ListenFunnel("tcp", addr)
		ch <- listenerResult{ln, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.ln, r.err
	case <-parent.Done():
		if closeServerOnCancel {
			_ = srv.Close()
		}
		go closeLateListener(ch)
		return nil, fmt.Errorf("ListenFunnel cancelled: %w", parent.Err())
	case <-timer.C:
		if closeServerOnCancel {
			_ = srv.Close()
		}
		go closeLateListener(ch)
		return nil, fmt.Errorf("ListenFunnel timed out after %s", timeout)
	}
}

func closeLateListener(ch <-chan listenerResult) {
	result := <-ch
	if result.ln != nil {
		_ = result.ln.Close()
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
