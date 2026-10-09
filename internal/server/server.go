package server

import (
	"aurago/internal/meshcore"
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/acestep"
	"aurago/internal/agent"
	"aurago/internal/agentmail"
	"aurago/internal/bluetooth"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/cyd"
	"aurago/internal/desktop"
	"aurago/internal/desktopstore"
	"aurago/internal/detective"
	"aurago/internal/discord"
	"aurago/internal/dockerutil"
	"aurago/internal/flows"
	"aurago/internal/fritzbox"
	"aurago/internal/gamemaker"
	"aurago/internal/heartbeat"
	"aurago/internal/i18n"
	"aurago/internal/invasion/bridge"
	"aurago/internal/llm"
	"aurago/internal/localllm"
	"aurago/internal/localwiki"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/networkshares"
	"aurago/internal/newspaper"
	"aurago/internal/onvif"
	"aurago/internal/personalradio"
	"aurago/internal/planner"
	"aurago/internal/proxy"
	"aurago/internal/remote"
	"aurago/internal/rocketchat"
	"aurago/internal/rtlsdr"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/sipphone"
	"aurago/internal/speechlab"
	"aurago/internal/speechlab/deployer"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
	"aurago/internal/tsnetnode"
	"aurago/internal/vaultprompt"
	"aurago/internal/virtualcomputers"
	"aurago/internal/warnings"
	"aurago/internal/webhooks"

	a2apkg "aurago/internal/a2a"
)

// normalizeLang converts the config language string to an ISO code for the frontend.
// Deprecated: Use i18n.NormalizeLang instead. This wrapper exists for backward compatibility.
func normalizeLang(lang string) string {
	return i18n.NormalizeLang(lang)
}

// DedicatedInternalLoopbackPort returns the plain-HTTP loopback listener port
// reserved for internal self-calls while the public server runs with HTTPS.
var (
	i18nMu       sync.RWMutex = sync.RWMutex{}
	i18nLangJSON map[string]string
	i18nMetaJSON string
)

// loadI18N reads ui/lang/*/<lang>.json from the embedded FS and prepares per-language JSON blobs.
// Deprecated: Use i18n.Load instead. This wrapper exists for backward compatibility.
func loadI18N(uiFS fs.FS, logger *slog.Logger) {
	i18n.Load(uiFS, logger)
	// Sync to legacy variables for backward compatibility with tests
	i18nMu.Lock()
	i18nLangJSON = nil // Force tests to use i18n package directly
	i18nMetaJSON = ""
	i18nMu.Unlock()
}

// getI18NJSON returns the JSON string for the given language, falling back to "en".
// Deprecated: Use i18n.GetJSON instead. This wrapper exists for backward compatibility.
func getI18NJSON(lang string) string {
	return i18n.GetJSON(lang)
}

// getI18NJSONForSections returns a JSON object with only the requested i18n
// sections (plus common) for the given language.
func getI18NJSONForSections(lang string, sections ...string) string {
	return i18n.GetJSONForSection(lang, sections...)
}

// getI18NMetaJSON returns the _meta section JSON for config_help metadata.
// Deprecated: Use i18n.GetMetaJSON instead. This wrapper exists for backward compatibility.
func getI18NMetaJSON() string {
	return i18n.GetMetaJSON()
}

func panicRecoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				logger.Error("HTTP handler panic", "path", r.URL.Path, "method", r.Method, "panic", recovered)
				if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error":   "internal_server_error",
						"message": "Internal server error",
					})
					return
				}
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Server holds the state and dependencies for the web server and socket bridge.
type Server struct {
	Cfg             *config.Config
	cfgSnapshot     atomic.Pointer[config.Config]
	CfgMu           sync.RWMutex // protects Cfg during hot-reload
	CfgSaveMu       sync.Mutex   // serializes config file writes to prevent TOCTOU races
	httpDrainMu     sync.Mutex
	httpDraining    bool
	httpDrainCtx    context.Context
	httpDrainCancel context.CancelFunc
	httpRequests    sync.WaitGroup
	httpHijacked    map[*drainHTTPConn]struct{}
	lockdownLogOnce sync.Once
	previewGrants   previewGrantRegistry
	SIPConfigMu     sync.Mutex // serializes SIP snapshots, Vault mutations, and config publication

	// serverLifetimeCancel cancels serverCtx; beginHTTPDrain calls it.
	serverLifetimeCancel context.CancelFunc

	// Setup wizard CSRF tokens (short-lived, multi-token support).
	// These live on the Server so tests can construct independent Server
	// instances without racing on a shared package-level map.
	SetupCSRFMu     sync.Mutex
	SetupCSRFTokens map[string]time.Time
	// SetupCSRFCleanupOnce ensures the per-Server CSRF cleanup goroutine is
	// started exactly once. Lives on Server (not package global) so each
	// Server instance has independent cleanup lifecycle for test isolation.
	SetupCSRFCleanupOnce sync.Once
	// One-time bootstrap token that remote setup and first-password requests
	// must present. It is printed to the server log, never served over HTTP.
	setupBootstrapMu    sync.Mutex
	setupBootstrapValue string
	// Exact Telnyx webhook path mounted at startup. Its handler verifies Ed25519
	// signatures itself, so only this registered path skips session auth.
	telnyxWebhookPath         atomic.Pointer[string]
	SetupLocalLLMJobsMu       sync.Mutex
	SetupLocalLLMJobs         map[string]*setupLocalLLMJob
	Logger                    *slog.Logger
	AccessLogger              *slog.Logger
	LLMClient                 llm.ChatClient
	ShortTermMem              *memory.SQLiteMemory
	LongTermMem               memory.VectorDB
	Vault                     *security.Vault
	VaultSecretPrompter       *vaultprompt.Manager
	vaultSecretPromptMu       sync.Mutex
	Registry                  *tools.ProcessRegistry
	CronManager               *tools.CronManager
	BackgroundTasks           *tools.BackgroundTaskManager
	Go2RTC                    *tools.Go2RTCManager
	LocalLLM                  *localllm.Manager
	LocalMusic                *acestep.Manager
	LocalWiki                 *localwiki.Manager
	localWikiSyncMu           sync.Mutex // one step from reading the config snapshot to configuring LocalWiki
	localWikiBeforeConfigure  func()     // test seam: runs in syncLocalWikipediaSettings after the snapshot is read
	localLLMLifecycleCtx      context.Context
	Go2RTCDiscovery           *onvif.Service
	MeshCore                  *meshcore.Manager
	Bluetooth                 *bluetooth.Manager
	NetworkShares             *networkshares.Manager
	SIPPhone                  *sipphone.Manager
	SpeechLab                 *speechlab.Client
	SpeechLabDeployer         *deployer.Manager
	speechLabTurnTokens       *speechLabTurnTokenRegistry
	speechLabTurnTokensMu     sync.Mutex
	SIPBrowserMedia           *sipphone.BrowserMediaService
	VoiceActionRunner         *VoiceActionRunner
	HistoryManager            *memory.HistoryManager
	KG                        *memory.KnowledgeGraph
	InventoryDB               *sql.DB
	InvasionDB                *sql.DB
	Guardian                  *security.Guardian
	LLMGuardian               *security.LLMGuardian
	CoAgentRegistry           *agent.CoAgentRegistry
	BudgetTracker             *budget.Tracker
	TokenManager              *security.TokenManager
	tokenManagerMu            sync.RWMutex // guards TokenManager replacement (backup import)
	CydHub                    *cyd.Hub
	WebhookManager            *webhooks.Manager
	WebhookHandler            *webhooks.Handler
	SSE                       *SSEBroadcaster // shared SSE broadcaster, set by run()
	systemWorldOnce           sync.Once
	systemWorld               *systemWorldRuntime
	MissionManagerV2          *tools.MissionManagerV2
	EmailWatcher              *tools.EmailWatcher
	mcpSessions               mcpSessionSigner
	missionRuns               *missionRunRegistry // cancellable contexts of in-flight local mission runs
	missionRunsOnce           sync.Once
	EggHub                    *bridge.EggHub
	RemoteHub                 *remote.RemoteHub
	agodeskDesktopMu          sync.Mutex
	agodeskDesktop            *agodeskDesktopBroker
	agodeskDevToken           string // loopback-only AgoDesk development opt-in; never exposed to clients
	ProxyManager              *proxy.Manager
	TsNetManager              *tsnetnode.Manager
	tsNetHandler              http.Handler // stored so the UI can restart tsnet without a full server restart
	FileIndexer               *services.FileIndexer
	WorkspaceSearch           *services.WorkspaceSearchService
	MaintenanceScheduler      *agent.MaintenanceController
	MQTTController            *mqtt.MQTTController
	HeartbeatScheduler        *heartbeat.Scheduler
	AgentMailService          *agentmail.Service
	AgentMailMu               sync.Mutex
	CheatsheetDB              *sql.DB
	ImageGalleryDB            *sql.DB
	MediaRegistryDB           *sql.DB
	HomepageRegistryDB        *sql.DB
	ContactsDB                *sql.DB
	PlannerDB                 *sql.DB
	LaunchpadDB               *sql.DB
	SQLConnectionsDB          *sql.DB
	SQLConnectionPool         *sqlconnections.ConnectionPool
	A2AServer                 *a2apkg.Server        // A2A protocol server (nil if disabled)
	A2AClientMgr              *a2apkg.ClientManager // A2A client manager (nil if disabled)
	A2ABridge                 *a2apkg.Bridge        // A2A co-agent bridge (nil if disabled)
	SkillManager              *tools.SkillManager   // Skill Manager for registry and security scanning
	AgentSkillManager         *tools.AgentSkillManager
	SkillsDB                  *sql.DB // Skills registry database
	PreparedMissionsDB        *sql.DB // Prepared missions SQLite database
	MissionHistoryDB          *sql.DB // Mission execution history SQLite database
	PreparationService        *services.MissionPreparationService
	WarningsRegistry          *warnings.Registry // Runtime warnings and health issues
	DaemonSupervisor          *tools.DaemonSupervisor
	DesktopService            *desktop.Service
	desktopPolicyService      atomic.Pointer[desktop.Service]
	DesktopStore              *desktopstore.Service
	DesktopHub                *desktop.Hub
	VirtualComputersDB        *virtualcomputers.Ledger
	VirtualWorkspaceManager   *virtualcomputers.WorkspaceManager
	GameMaker                 *gamemaker.Service
	Detective                 *detective.Service
	Newspaper                 *newspaper.Service
	Flows                     *flows.Service
	flowsCatalog              *flowCatalogEnv
	flowNotify                flowFailureNotifier // flood rule and send slots of flow failure notifications
	flowSecretRate            flowRateLimiter     // per-IP limit of flow secret writes and deletes
	flowStreams               flowStreamLimiter   // open flow run event streams, per run and in total
	flowStreamBeat            time.Duration       // heartbeat of flow run streams; 0 means flowStreamHeartbeat (tests shorten it)
	flowNodeTypesCache        flowNodeTypesCache  // encoded GET /api/desktop/flows/node-types answers, per language
	flowHACache               flowHACache         // Home Assistant entity options of the flow editor, reused for 30 s
	newspaperSkillReady       bool
	PersonalRadio             *personalradio.Service
	RTLSDR                    *rtlsdr.Service
	RTLSDRRuntime             *rtlsdr.Manager
	gameMakerSkills           []gamemaker.SkillInfo
	gameMakerSkillsReady      bool
	DesktopMu                 sync.Mutex
	desktopClosed             bool // shutdown closed Desktop storage; guarded by DesktopMu
	desktopRuns               desktopRunRegistry
	videoStudioMu             sync.Mutex
	videoStudio               *videoStudioManager
	videoStudioClosed         bool
	videoStudioConfigRevoking atomic.Bool
	// IsFirstStart is true if core_memory.md was just freshly created (no prior data).
	IsFirstStart    bool
	StartedAt       time.Time     // server start time for uptime calculation
	ShutdownCh      chan struct{} // signal channel for graceful shutdown
	ready           atomic.Bool   // set to true once the server is accepting connections
	firstStartDone  bool
	muFirstStart    sync.Mutex
	internalToken   string       // per-process crypto token for loopback auth
	loopbackSrv     *http.Server // plain-HTTP server on 127.0.0.1 for cloudflared (HTTPS loopback port)
	loopbackHandler http.Handler // stored handler so hot-reload can restart the listener without a full restart
	spaceAgentHTTPS *http.Server // HTTPS reverse proxy for the managed Space Agent web UI

	// pushRemoteAllowedPaths sends a changed remote_control.allowed_paths to
	// connected agents (nil means (*remote.RemoteHub).PushDefaultAllowedPaths);
	// a field so a test can reach the server goroutine's own recover.
	pushRemoteAllowedPaths func(*remote.RemoteHub)

	backgroundCompletions backgroundCompletionCache
	integrationCtx        context.Context
	rocketChatLifecycleMu sync.Mutex
	rocketChatBot         atomic.Pointer[rocketchat.Bot]
	rocketChatClosed      bool
	haPollerMu            sync.Mutex
	haPoller              atomic.Pointer[homeAssistantPollerRuntime]
	haPollerClosed        bool
	fritzPollerMu         sync.Mutex
	fritzPoller           atomic.Pointer[fritzbox.Poller]
	fritzPollerClosed     bool
	fritzLoopbackSem      chan struct{}
	fritzWidgetMu         sync.Mutex
	fritzWidget           *fritzWidgetCache
	uptimeKumaMu          sync.Mutex
	uptimeKuma            atomic.Pointer[uptimeKumaRuntime]
	uptimeKumaClosed      bool
}

func (s *Server) accessLogger() *slog.Logger {
	if s.AccessLogger != nil {
		return s.AccessLogger
	}
	return s.Logger
}

// currentTokenManager returns the live token store. Backup import may replace
// it, so long-lived holders (the webhook handler) resolve it per request.
func (s *Server) currentTokenManager() *security.TokenManager {
	if s == nil {
		return nil
	}
	s.tokenManagerMu.RLock()
	defer s.tokenManagerMu.RUnlock()
	return s.TokenManager
}

// replaceTokenManager publishes a new token store and retires the previous one
// so a request still holding it cannot rewrite tokens.json with stale data.
func (s *Server) replaceTokenManager(tm *security.TokenManager) {
	s.tokenManagerMu.Lock()
	previous := s.TokenManager
	s.TokenManager = tm
	s.tokenManagerMu.Unlock()
	if previous != nil && previous != tm {
		previous.Retire()
	}
}

// missionRunTracker lazily creates the registry so tests that construct a
// bare &Server{} keep working.
func (s *Server) missionRunTracker() *missionRunRegistry {
	s.missionRunsOnce.Do(func() {
		if s.missionRuns == nil {
			s.missionRuns = newMissionRunRegistry()
		}
	})
	return s.missionRuns
}

// missionRunBaseContext returns the base context for the sync chat branch.
// Requests without a mission keep the detached background context; mission
// runs get a registry-backed context that POST /api/missions/v2/{id}/cancel
// can cancel.
func missionRunBaseContext(s *Server, missionID string) (context.Context, func()) {
	if missionID == "" {
		return context.Background(), func() {}
	}
	parent := s.integrationCtx
	if s.MissionManagerV2 != nil {
		if owner, owned := s.MissionManagerV2.ActiveOwnerContext(missionID); owned {
			parent = owner
			if parent == nil {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				parent = ctx
			}
		}
	}
	return s.missionRunTracker().beginContext(parent, missionID)
}

func (s *Server) initConfigSnapshot() {
	if s == nil || s.Cfg == nil {
		return
	}
	s.bindConfigAuthorization(s.Cfg)
	s.syncPersonalityConfig(s.Cfg)
	s.cfgSnapshot.Store(s.Cfg)
}

// ConfigSnapshot returns the current immutable runtime config pointer.
func (s *Server) ConfigSnapshot() *config.Config {
	if s == nil {
		return nil
	}
	if cfg := s.cfgSnapshot.Load(); cfg != nil {
		return cfg
	}
	return s.Cfg
}

// newRemoteHub builds the Remote Control hub with its config-driven defaults.
// The global remote_control.allowed_paths is read from the config snapshot on
// every call, so a reload needs no restart; replaceConfigSnapshot pushes a
// changed default to connected agents without their own list.
func (s *Server) newRemoteHub(db *sql.DB, vault *security.Vault, logger *slog.Logger, cfg *config.Config) *remote.RemoteHub {
	hub := remote.NewRemoteHub(db, vault, logger)
	hub.DefaultReadOnly = cfg.RemoteControl.ReadOnly
	hub.AutoApprove = cfg.RemoteControl.AutoApprove
	hub.MaxFileSizeMB = cfg.RemoteControl.MaxFileSizeMB
	hub.AuditLogEnabled = cfg.RemoteControl.AuditLog
	hub.DefaultAllowedPaths = func() []string {
		if snapshot := s.ConfigSnapshot(); snapshot != nil {
			return snapshot.RemoteControl.AllowedPaths
		}
		return nil
	}
	return hub
}

func (s *Server) replaceConfigSnapshot(cfg *config.Config) {
	if s == nil || cfg == nil {
		return
	}
	previous := s.ConfigSnapshot()
	if previous != nil && videoStudioConfigRootsChanged(previous, cfg) {
		// Cancel before publishing the new roots. A worker may finish an FFmpeg
		// run after a workspace change, but it must not publish into the new tree.
		s.beginVideoStudioConfigChange()
	}
	if cfg.VirtualDesktop.ReadOnly || !cfg.VirtualDesktop.Enabled {
		s.revokeDesktopRuns()
	}
	if s.GameMaker != nil {
		s.GameMaker.UpdatePolicy(gameMakerPolicy(cfg.GameMaker, cfg.VirtualDesktop.ReadOnly))
	}
	if svc := s.desktopPolicyService.Load(); svc != nil {
		svc.SetReadOnly(cfg.VirtualDesktop.ReadOnly)
	}
	if bot := s.rocketChatBot.Load(); bot != nil {
		bot.CancelIfConfigChanged(cfg)
	}
	if poller := s.haPoller.Load(); poller != nil {
		poller.cancelIfChanged(cfg)
	}
	if poller := s.fritzPoller.Load(); poller != nil {
		poller.CancelIfConfigChanged(cfg)
	}
	if runtime := s.uptimeKuma.Load(); runtime != nil && (runtime.initial != cfg.UptimeKuma || runtime.eggMode != cfg.EggMode.Enabled) {
		runtime.cancel()
	}
	s.bindConfigAuthorization(cfg)
	s.syncPersonalityConfig(cfg)
	RegisterLLMSecrets(cfg)
	s.Cfg = cfg
	s.cfgSnapshot.Store(cfg)
	s.finishVideoStudioConfigChange()
	if hub := s.RemoteHub; hub != nil && previous != nil &&
		!slices.Equal(previous.RemoteControl.AllowedPaths, cfg.RemoteControl.AllowedPaths) {
		// The hub already evaluates the new default for shell checks; agents
		// without their own list get it pushed. Socket writes stay off the
		// config publication path. Nothing else recovers a panic on this
		// goroutine, so it is logged here instead of ending the process.
		logger := s.Logger
		if logger == nil {
			logger = slog.Default()
		}
		push := s.pushRemoteAllowedPaths
		if push == nil {
			push = (*remote.RemoteHub).PushDefaultAllowedPaths
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Recovered from a panic while pushing remote allowed paths",
						"panic", r, "stack", string(debug.Stack()))
				}
			}()
			push(hub)
		}()
	}
	if s.MQTTController != nil {
		s.MQTTController.UpdateConfig(mqttRuntimeSnapshot(cfg))
	}
	if s.MaintenanceScheduler != nil {
		s.MaintenanceScheduler.UpdateConfig(cfg)
	}
	if s.ProxyManager != nil {
		// The next proxy Start/Reload uses the saved domain, ports, filters and
		// Vault credentials instead of the startup config.
		s.ProxyManager.UpdateConfig(cfg)
	}
	if s.LocalMusic != nil {
		s.LocalMusic.Configure(cfg)
	}
	s.syncLocalWikipediaSettings()
	if previous != nil && previous.LocalWikipedia.Enabled != cfg.LocalWikipedia.Enabled {
		// The Wikipedia Desktop app appears or disappears with the switch;
		// the broadcast runs off this (usually CfgMu-locked) path.
		s.announceLocalWikipediaAvailability()
	}
	if s.WarningsRegistry != nil {
		// Provider metadata probes may perform bounded network I/O. Keep config
		// publication non-blocking while still reconciling stale warnings.
		go warnings.RefreshTokenBudgetWarnings(s.WarningsRegistry, cfg, s.Logger)
	}
	speechCfg := effectiveSpeechLabConfig(cfg)
	if s.SpeechLab != nil {
		s.SpeechLab.Reconfigure(speechCfg)
	} else if s.Logger != nil {
		s.Logger.Warn("Speech Lab runtime is unavailable; restart is required after repairing configuration")
	}
	if s.SpeechLabDeployer != nil {
		s.SpeechLabDeployer.Reconfigure(speechCfg, deployer.RuntimeAccess{
			DockerEnabled:  cfg.Docker.Enabled,
			DockerReadOnly: cfg.Docker.ReadOnly,
			Docker:         dockerutil.NewClient(cfg.Docker.Host, 30*time.Second),
		})
	}
}

func (s *Server) bindConfigAuthorization(cfg *config.Config) {
	cfg.RegisterEvomapNode = func(ctx context.Context) (string, string, bool, error) {
		return s.registerEvomapNode(ctx, cfg.Evomap)
	}
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) {
		return cfg, s.ConfigSnapshot()
	}
}

func effectiveSpeechLabConfig(cfg *config.Config) config.SpeechLabConfig {
	if cfg == nil {
		return config.SpeechLabConfig{}
	}
	speechCfg := cfg.SpeechLab
	if speechCfg.Deployment.Mode == "managed" && !cfg.Runtime.IsDocker && strings.EqualFold(strings.TrimRight(speechCfg.BaseURL, "/"), strings.TrimRight(config.DefaultSpeechLabBaseURL, "/")) {
		speechCfg.BaseURL = config.DefaultManagedSpeechLabBaseURL
	}
	return speechCfg
}

// reinitBudgetTracker keeps the existing tracker for in-flight agent runs and
// re-registers the MissionManagerV2 callback after config reloads.
func (s *Server) reinitBudgetTracker(cfg *config.Config) {
	if s.BudgetTracker == nil {
		s.BudgetTracker = budget.NewTracker(cfg, s.Logger, cfg.Directories.DataDir)
	} else {
		s.BudgetTracker.UpdateConfig(cfg)
	}
	if s.BudgetTracker != nil && s.MissionManagerV2 != nil {
		s.BudgetTracker.SetMissionCallback(func(eventType string, spentUSD, limitUSD, percentage float64) {
			s.MissionManagerV2.NotifyBudgetEvent(eventType, spentUSD, limitUSD, percentage)
		})
	}
}

// StartOptions groups the server boot dependencies so startup wiring stays named and testable.
type StartOptions struct {
	Cfg                     *config.Config
	Logger                  *slog.Logger
	AccessLogger            *slog.Logger
	LLMClient               llm.ChatClient
	LocalLLM                *localllm.Manager
	ShortTermMem            *memory.SQLiteMemory
	LongTermMem             memory.VectorDB
	Vault                   *security.Vault
	Registry                *tools.ProcessRegistry
	CronManager             *tools.CronManager
	HistoryManager          *memory.HistoryManager
	KG                      *memory.KnowledgeGraph
	InventoryDB             *sql.DB
	InvasionDB              *sql.DB
	CheatsheetDB            *sql.DB
	ImageGalleryDB          *sql.DB
	RemoteControlDB         *sql.DB
	MediaRegistryDB         *sql.DB
	HomepageRegistryDB      *sql.DB
	ContactsDB              *sql.DB
	PlannerDB               *sql.DB
	LaunchpadDB             *sql.DB
	SQLConnectionsDB        *sql.DB
	SQLConnectionPool       *sqlconnections.ConnectionPool
	BackgroundTasks         *tools.BackgroundTaskManager
	VirtualComputersDB      *virtualcomputers.Ledger
	VirtualWorkspaceManager *virtualcomputers.WorkspaceManager
	EggMissionResultSink    func(result bridge.MissionResultPayload) error
	WarningsRegistry        *warnings.Registry
	IsFirstStart            bool
	ShutdownCh              chan struct{}
	InstallDir              string
}

// firewallGuardNeedsSudoPassword reports whether the Firewall Guard needs the
// Vault sudo password. Root or a NOPASSWD rule (FirewallAccessOK) already gives
// access, and handing the password over there would mean a sudo login on every
// poll. This is the condition the firewall tool uses in dispatchNetwork.
func firewallGuardNeedsSudoPassword(cfg *config.Config) bool {
	return cfg.Agent.SudoEnabled && !cfg.Runtime.FirewallAccessOK
}

func Start(opts StartOptions) error {
	cfg := opts.Cfg
	// cmd/aurago already registered the LLM keys for log scrubbing before it
	// built the LLM client; repeat it here for embedders that call Start
	// directly. replaceConfigSnapshot repeats it for every reloaded config.
	RegisterLLMSecrets(cfg)
	if err := validateRemoteAuthExposure(cfg); err != nil {
		return err
	}
	logger := opts.Logger
	llmClient := opts.LLMClient
	shortTermMem := opts.ShortTermMem
	longTermMem := opts.LongTermMem
	vault := opts.Vault
	registry := opts.Registry
	kg := opts.KG
	inventoryDB := opts.InventoryDB
	cheatsheetDB := opts.CheatsheetDB
	remoteControlDB := opts.RemoteControlDB
	backgroundTasks := opts.BackgroundTasks
	shutdownCh := opts.ShutdownCh
	installDir := opts.InstallDir

	// Central server context: cancelled when shutdown is requested.
	serverCtx, serverCancel := context.WithCancel(context.Background())
	go func() {
		<-shutdownCh
		serverCancel()
	}()

	// Limit concurrent internal loopback callbacks fired by background services
	// (firewall guard, Fritz!Box poller, etc.) to avoid thundering-herd LLM runs.
	const loopbackCallbackConcurrency = 4
	loopbackSem := make(chan struct{}, loopbackCallbackConcurrency)

	startLoginRecordCleaner(shutdownCh)
	s := newServerFromOptions(opts)
	// The self marker proves AuraGo's own container behind a network sidecar
	// (containers_self_proof.go); outside a container it does nothing.
	initContainerSelfMarker(containerRuntimeIsDocker(s), logger)
	s.integrationCtx = serverCtx
	// The HTTP drain also ends serverCtx, so it ends when Serve stops on its
	// own (listener failure) too, not only on shutdownCh.
	s.setServerLifetimeCancel(serverCancel)
	s.fritzLoopbackSem = loopbackSem
	s.MQTTController = mqtt.NewMQTTController(logger)
	mqtt.SetDefaultController(s.MQTTController)
	s.bindMQTTPermissions()
	s.bindRuntimePermissions()
	s.bindDockerSelfIdentity()
	s.configureMQTTRelay()
	defer s.MQTTController.Stop(context.Background())
	s.localLLMLifecycleCtx = serverCtx
	if s.SpeechLabDeployer != nil {
		go func() {
			if err := s.SpeechLabDeployer.AutoStart(serverCtx); err != nil && s.Logger != nil {
				s.Logger.Warn("[SpeechLab] Managed bundle auto-start failed", "code", deployer.Code(err), "error", err)
			}
		}()
	}
	if err := s.initSIP(serverCtx); err != nil {
		// The shutdown path below is not registered yet: withdraw the tool
		// source newServerFromOptions published, or the agent tool would keep
		// a manager that is never started or shut down.
		if s.LocalWiki != nil {
			withdrawLocalWikipediaTool(s.LocalWiki)
		}
		serverCancel()
		return err
	}
	if s.Go2RTC != nil {
		s.Go2RTC.StartBackground(serverCtx)
	}
	if s.LocalMusic != nil {
		s.LocalMusic.Start()
	}
	if s.LocalWiki != nil {
		s.LocalWiki.Start(serverCtx)
	}
	defer func() {
		if s.RTLSDR != nil {
			tools.SetRTLSDRService(nil)
			_ = s.RTLSDR.Close()
		}
		if s.RTLSDRRuntime != nil {
			s.RTLSDRRuntime.Close()
		}
		if s.PersonalRadio != nil {
			_ = s.PersonalRadio.Close()
		}
		if s.LocalMusic != nil {
			s.LocalMusic.Close()
		}
		if acestep.Default() == s.LocalMusic {
			acestep.SetDefault(nil)
		}
		if s.LocalLLM != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := s.LocalLLM.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
				s.Logger.Warn("[LocalLLM] Shutdown cleanup did not complete", "code", safeLocalLLMErrorCode(err))
			}
			shutdownCancel()
		}
		if s.LocalWiki != nil {
			withdrawLocalWikipediaTool(s.LocalWiki)
			wikiCtx, wikiCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := s.LocalWiki.Shutdown(wikiCtx); err != nil {
				s.Logger.Warn("[LocalWikipedia] Shutdown did not complete", "error", err)
			}
			wikiCancel()
		}
		if s.GameMaker != nil {
			_ = s.GameMaker.Close()
		}
		if s.Detective != nil {
			_ = s.Detective.Close()
		}
		if s.Newspaper != nil {
			_ = s.Newspaper.Close()
		}
		if gamemaker.DefaultService() == s.GameMaker {
			gamemaker.SetDefaultService(nil)
		}
		if s.Go2RTC != nil {
			s.Go2RTC.Close()
		}
		if tools.DefaultGo2RTCManager() == s.Go2RTC {
			tools.SetDefaultGo2RTCManager(nil)
		}
		if s.Bluetooth != nil {
			_ = s.Bluetooth.Close()
		}
		if bluetooth.DefaultManager() == s.Bluetooth {
			bluetooth.SetDefaultManager(nil)
		}
		if s.NetworkShares != nil {
			_ = s.NetworkShares.Close()
		}
		if networkshares.DefaultManager() == s.NetworkShares {
			networkshares.SetDefaultManager(nil)
		}
		if s.SIPBrowserMedia != nil {
			_ = s.SIPBrowserMedia.Close()
		}
		if s.SIPPhone != nil {
			_ = s.SIPPhone.Close()
		}
		if sipphone.DefaultManager() == s.SIPPhone {
			sipphone.SetDefaultManager(nil)
		}
	}()
	startHomepageLedgerReconciler(shutdownCh, s)
	if cfg.WorkspaceSearch.Enabled {
		workspaceSearch, err := services.NewWorkspaceSearchService(cfg, &s.CfgMu, logger)
		if err != nil {
			logger.Warn("Failed to initialize workspace search service", "error", err)
		} else {
			s.WorkspaceSearch = workspaceSearch
			if err := s.WorkspaceSearch.Start(serverCtx); err != nil {
				logger.Warn("Failed to start workspace search service", "error", err)
			} else {
				tools.SetFileAccessTracker(func(workspaceDir, path, kind string) {
					if s.WorkspaceSearch == nil {
						return
					}
					if err := s.WorkspaceSearch.TrackAccess(path, kind); err != nil {
						logger.Debug("Failed to track workspace file access", "workspace", workspaceDir, "path", path, "kind", kind, "error", err)
					}
				})
				logger.Info("Workspace search service started", "workspace", cfg.Directories.WorkspaceDir)
			}
		}
	}
	if shortTermMem != nil && s.MissionManagerV2 != nil {
		s.MissionManagerV2.SetAuditRecorder(shortTermMem.UpsertAuditEventByCorrelation)
	}
	// Retrieve the per-process loopback auth token from BackgroundTaskManager.
	// It was generated in main() before server.Start() was called.
	if backgroundTasks != nil {
		s.internalToken = backgroundTasks.InternalToken()
	}
	// Propagate the token to the agent package for invasion_tool loopback calls.
	if s.internalToken != "" {
		agent.SetAgentInternalToken(s.internalToken)
	}
	s.CoAgentRegistry.ConfigureLifecycle(
		time.Duration(cfg.CoAgents.CleanupIntervalMins)*time.Minute,
		time.Duration(cfg.CoAgents.CleanupMaxAgeMins)*time.Minute,
	)

	// Initialize Heartbeat scheduler
	hbRunCfg := agent.RunConfig{
		Config:             cfg,
		Logger:             logger,
		LLMClient:          llmClient,
		ShortTermMem:       shortTermMem,
		HistoryManager:     opts.HistoryManager,
		LongTermMem:        longTermMem,
		KG:                 kg,
		InventoryDB:        inventoryDB,
		InvasionDB:         opts.InvasionDB,
		CheatsheetDB:       cheatsheetDB,
		ImageGalleryDB:     opts.ImageGalleryDB,
		MediaRegistryDB:    opts.MediaRegistryDB,
		HomepageRegistryDB: opts.HomepageRegistryDB,
		ContactsDB:         opts.ContactsDB,
		PlannerDB:          opts.PlannerDB,
		SQLConnectionsDB:   opts.SQLConnectionsDB,
		SQLConnectionPool:  opts.SQLConnectionPool,
		RemoteHub:          s.RemoteHub,
		Vault:              vault,
		Registry:           registry,
		Manifest:           tools.NewManifest(cfg.Directories.ToolsDir),
		CronManager:        opts.CronManager,
		MissionManagerV2:   s.MissionManagerV2,
		CoAgentRegistry:    s.CoAgentRegistry,
		BudgetTracker:      s.BudgetTracker,
		LLMGuardian:        s.LLMGuardian,
		WorkspaceSearch:    s.WorkspaceSearch,
		SessionID:          "heartbeat",
		MessageSource:      "heartbeat",
	}
	s.HeartbeatScheduler = heartbeat.New(cfg, logger, func(prompt string) {
		correlationID := fmt.Sprintf("heartbeat_%d", time.Now().UTC().UnixNano())
		started := time.Now()
		if sessionRequestActive("default") || sessionRequestActive("virtual-desktop") {
			logger.Info("Heartbeat wake-up skipped because an interactive session is active")
			recordHeartbeatAuditFinish(shortTermMem, correlationID, memory.AuditStatusWarning, "Heartbeat wake-up skipped", "Interactive session is active", time.Since(started))
			return
		}
		recordHeartbeatAuditStart(shortTermMem, correlationID)
		agent.Loopback(hbRunCfg, prompt, agent.NoopBroker{})
		recordHeartbeatAuditFinish(shortTermMem, correlationID, memory.AuditStatusSuccess, "Heartbeat wake-up completed", "", time.Since(started))
	})
	s.HeartbeatScheduler.Start()
	s.restartUptimeKumaPoller()

	// Initialize Skill Manager and Agent Skills (classic manager gated by config)
	installedSkills := s.initSkillManagers(serverCtx, installDir)
	s.initGameMaker()
	s.initDetective()
	s.initNewspaper()
	s.initPersonalRadio()
	s.initRTLSDR()
	s.initBluetoothLive(serverCtx)
	// Remote security scanners must not delay the core HTTP readiness check.
	go s.syncAgentSkills(serverCtx, cfg, installedSkills)

	// Initialize Remote Control Hub
	remote.InsecureHostKey = cfg.RemoteControl.SSHInsecureHostKey
	if remoteControlDB != nil {
		s.RemoteHub = s.newRemoteHub(remoteControlDB, vault, logger, cfg)
		s.RemoteHub.OnConnect = func(deviceID, name string) {
			s.MissionManagerV2.NotifyDeviceEvent("device_connected", deviceID, name)
		}
		s.RemoteHub.OnDisconnect = func(deviceID, name string) {
			s.MissionManagerV2.NotifyDeviceEvent("device_disconnected", deviceID, name)
		}
		s.RemoteHub.OnAudit = func(event remote.RemoteAuditEvent) {
			recordRemoteAuditEvent(shortTermMem, event)
		}
		s.RemoteHub.SetEnabled(cfg.RemoteControl.Enabled)
		s.RemoteHub.StartHeartbeatMonitor(30*time.Second, 90*time.Second)
		if err := remote.TrimAuditLog(remoteControlDB, 10000); err != nil {
			logger.Warn("Failed to trim remote audit log", "error", err)
		}
		logger.Info("Remote Control Hub initialized", "insecure_host_key", cfg.RemoteControl.SSHInsecureHostKey)
	}

	// Initialize Security Proxy Manager
	s.ProxyManager = proxy.NewManager(cfg, logger)
	if securityProxyAutoStartAllowed(cfg) {
		logger.Info("Security proxy enabled — starting container automatically")
		go func() {
			if err := s.ProxyManager.Start(); err != nil {
				logger.Warn("Security proxy auto-start failed", "error", err)
			} else {
				logger.Info("Security proxy started")
			}
		}()
	} else if cfg.SecurityProxy.Enabled && !cfg.Docker.Enabled {
		logger.Info("Docker is disabled; skipping security proxy auto-start")
	}

	// Auto-start Homepage dev container (aurago-homepage) if homepage is enabled.
	// HomepageInit is idempotent: it starts a stopped container or creates it fresh.
	if homepageDevAutoStartAllowed(cfg) {
		logger.Info("Homepage dev container enabled — starting automatically")
		go func() {
			homepageCfg := tools.HomepageConfig{
				DockerHost:       cfg.Docker.Host,
				WorkspacePath:    cfg.Homepage.WorkspacePath,
				WebServerPort:    cfg.Homepage.WebServerPort,
				AllowLocalServer: cfg.Homepage.AllowLocalServer,
			}
			const maxRetries = 5
			for attempt := 1; attempt <= maxRetries; attempt++ {
				result := tools.HomepageInit(homepageCfg, logger)
				if strings.Contains(result, `"status":"ok"`) || strings.Contains(result, `"status": "ok"`) {
					logger.Info("Homepage dev container auto-start succeeded", "attempt", attempt)
					return
				}
				logger.Warn("Homepage dev container auto-start failed",
					"attempt", attempt, "max", maxRetries, "result", result)
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt*5) * time.Second)
				}
			}
			logger.Error("Homepage dev container auto-start exhausted all retries")
		}()
	} else if cfg.Homepage.Enabled && !cfg.Docker.Enabled {
		logger.Info("Docker is disabled; skipping Homepage dev container auto-start")
	} else if cfg.Homepage.Enabled && cfg.Homepage.WorkspacePath == "" {
		logger.Warn("Homepage dev container enabled but homepage.workspace_path is not set — skipping auto-start")
	}

	// Auto-start Homepage web server (Caddy) if enabled.
	// Note: webserver_enabled is independent of homepage.enabled (the dev container feature).
	// We only require WorkspacePath to be set so the Docker bind mount has an absolute path.
	if homepageWebServerAutoStartAllowed(cfg) {
		logger.Info("Homepage web server enabled — starting container automatically")
		// Ensure the workspace directory exists so the Docker bind-mount never fails
		// on a fresh system or after the directory was removed. An empty workspace is
		// perfectly valid (Caddy serves an empty directory listing).
		if mkErr := os.MkdirAll(cfg.Homepage.WorkspacePath, 0755); mkErr != nil {
			logger.Warn("Homepage web server: could not create workspace directory, auto-start may fail",
				"path", cfg.Homepage.WorkspacePath, "error", mkErr)
		}
		go func() {
			homepageCfg := tools.HomepageConfig{
				DockerHost:            cfg.Docker.Host,
				WorkspacePath:         cfg.Homepage.WorkspacePath,
				WebServerPort:         cfg.Homepage.WebServerPort,
				WebServerDomain:       cfg.Homepage.WebServerDomain,
				WebServerInternalOnly: cfg.Homepage.WebServerInternalOnly,
				AllowLocalServer:      cfg.Homepage.AllowLocalServer,
			}
			// Pass "" for projectDir and buildDir so detectBuildDir auto-detects
			// the build output (out/dist/build/…) from the workspace filesystem.
			// Retry up to 5 times with increasing delay — Docker may not be ready
			// immediately after a system reboot.
			const maxRetries = 5
			for attempt := 1; attempt <= maxRetries; attempt++ {
				result := tools.HomepageWebServerStart(homepageCfg, "", "", logger)
				if strings.Contains(result, `"status":"ok"`) || strings.Contains(result, `"status": "ok"`) {
					logger.Info("Homepage web server auto-start succeeded", "attempt", attempt, "result", result)
					return
				}
				logger.Warn("Homepage web server auto-start failed",
					"attempt", attempt, "max", maxRetries, "result", result)
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt*5) * time.Second) // 5s, 10s, 15s, 20s
				}
			}
			logger.Error("Homepage web server auto-start exhausted all retries")
		}()
	} else if cfg.Homepage.WebServerEnabled && !cfg.Docker.Enabled {
		logger.Info("Docker is disabled; skipping Homepage web server auto-start")
	} else if cfg.Homepage.WebServerEnabled && cfg.Homepage.WorkspacePath == "" {
		logger.Warn("Homepage web server is enabled but homepage.workspace_path is not set — skipping auto-start")
	}

	// Initialize tsnet Manager (Tailscale embedded node)
	s.TsNetManager = tsnetnode.NewManager(cfg, logger)
	if cfg.Tailscale.TsNet.Enabled {
		logger.Info("tsnet node enabled — will start alongside server")
	}

	// Initialize runtime debug mode from config
	agent.SetDebugMode(cfg.Agent.DebugMode)

	// Initialize Token Manager. A load failure still yields a read-only manager:
	// it validates no token and never rewrites the unreadable file.
	tokenFilePath := filepath.Join(cfg.Directories.DataDir, "tokens.json")
	tm, tmErr := security.NewTokenManager(vault, tokenFilePath)
	if tmErr != nil {
		logger.Error("Token store could not be loaded; token-authenticated clients are rejected and token changes are refused until tokens.json is readable again", "path", tokenFilePath, "error", tmErr)
		if s.WarningsRegistry != nil {
			s.WarningsRegistry.Add(warnings.Warning{
				ID:          "token_store_unavailable",
				Severity:    warnings.SeverityCritical,
				Category:    warnings.CategorySecurity,
				Title:       "API token store could not be loaded",
				Description: "tokens.json could not be read or decrypted. Webhook, device and API tokens are rejected and token changes are refused so the file is not overwritten. Fix the file permissions or the master key, then restart AuraGo.",
				Timestamp:   time.Now(),
			})
		}
	}
	s.TokenManager = tm

	// Initialize Webhook Manager + Handler
	if cfg.Webhooks.Enabled && tm != nil {
		whFilePath := filepath.Join(cfg.Directories.DataDir, "webhooks.json")
		whLogPath := filepath.Join(cfg.Directories.DataDir, "webhook_log.json")
		whMgr, whErr := webhooks.NewManager(whFilePath, whLogPath)
		if whErr != nil {
			logger.Error("Failed to initialize WebhookManager", "error", whErr)
		} else if err := whMgr.MigrateSignatureSecrets(vault); err != nil {
			logger.Error("Failed to migrate webhook signature secrets; public webhook handler disabled", "error", err)
		} else {
			s.WebhookManager = whMgr
			s.WebhookHandler = webhooks.NewHandler(whMgr, tm, vault, s.Guardian, s.LLMGuardian, cfg, logger, cfg.Server.Port, int64(cfg.Webhooks.MaxPayloadSize), cfg.Webhooks.RateLimit)
			s.WebhookHandler.SetTokenManagerSource(s.currentTokenManager)
			s.WebhookHandler.SetRuntimeSource(func() (*config.Config, *security.Guardian, *security.LLMGuardian) {
				s.CfgMu.RLock()
				defer s.CfgMu.RUnlock()
				return s.ConfigSnapshot(), s.Guardian, s.LLMGuardian
			})
			s.WebhookHandler.SetInternalToken(s.internalToken)
			logger.Info("Webhook system initialized", "max_webhooks", webhooks.MaxWebhooks)
		}
	}

	// Start MissionManagerV2 with enhanced callback that reports completion
	missionCallbackV2 := func(prompt string, missionID string) {
		func() {
			callbackCtx := s.MissionManagerV2.Context()
			if owner, owned := s.MissionManagerV2.ActiveOwnerContext(missionID); owned {
				if owner == nil {
					return
				}
				callbackCtx = owner
			}
			recordMissionIssue := func(title, detail string) {
				if s.PlannerDB == nil {
					return
				}
				missionLabel := missionID
				if mission, ok := s.MissionManagerV2.Get(missionID); ok && strings.TrimSpace(mission.Name) != "" {
					missionLabel = strings.TrimSpace(mission.Name)
				}
				if title == "" {
					title = fmt.Sprintf("Mission %s failed", missionLabel)
				}
				issue := planner.OperationalIssue{
					Source:      "mission",
					Context:     missionID,
					Title:       title,
					Detail:      detail,
					Severity:    "error",
					Reference:   missionID,
					Fingerprint: "mission|" + missionID,
					OccurredAt:  time.Now(),
				}
				if issueID, err := planner.RecordOperationalIssue(s.PlannerDB, issue); err != nil {
					logger.Warn("[MissionV2] Failed to record internal operational issue", "mission_id", missionID, "error", err)
				} else if s.MissionManagerV2 != nil {
					s.MissionManagerV2.NotifyPlannerOperationalIssue(issueID, issue.Source, issue.Severity, issue.Title)
				}
			}
			setMissionError := func(title, detail string) {
				if s.missionRunTracker().consumeCancelled(missionID) {
					logger.Info("[MissionV2] Mission run cancelled by user", "mission_id", missionID)
					s.MissionManagerV2.SetResult(missionID, "error", tools.MissionCancelledOutput)
					broadcastMissionState(s)
					return
				}
				recordMissionIssue(title, detail)
				s.MissionManagerV2.SetResult(missionID, "error", detail)
				broadcastMissionState(s)
			}

			defer func() {
				if r := recover(); r != nil {
					logger.Error("[MissionV2] Recovered from panic", "mission_id", missionID, "panic", r)
					setMissionError("", fmt.Sprintf("panic: %v", r))
				}
			}()

			url := InternalAPIURL(cfg) + "/v1/chat/completions"
			payload := map[string]interface{}{
				"model":  "aurago",
				"stream": false,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
			}
			// Add mission ID header for tracking
			body, err := json.Marshal(payload)
			if err != nil {
				logger.Error("[MissionV2] Failed to marshal request payload", "error", err, "mission_id", missionID)
				setMissionError("Mission request preparation failed", err.Error())
				return
			}
			headers := make(http.Header)
			headers.Set("Content-Type", "application/json")
			headers.Set("X-Internal-FollowUp", "true")
			headers.Set("X-Internal-Token", s.internalToken)
			headers.Set("X-Mission-ID", missionID)
			client := NewInternalHTTPClient(35 * time.Minute) // Must exceed the 30-minute agent loop timeout
			resp, err := DoInternalRequestWithStartupRetry(callbackCtx, client, http.MethodPost, url, body, headers, 15*time.Second)
			if err != nil {
				logger.Error("[MissionV2] Execution failed", "error", err, "mission_id", missionID)
				setMissionError("", err.Error())
				return
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				logger.Error("[MissionV2] Failed to read response body", "error", err, "mission_id", missionID)
				setMissionError("", fmt.Sprintf("failed to read response: %v", err))
				return
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Extract the assistant's text from the OpenAI-format response
				output := extractAssistantContent(respBody)
				if strings.EqualFold(resp.Header.Get("X-Aurago-Agent-Error"), "true") {
					logger.Error("[MissionV2] Mission agent loop returned an error response",
						"mission_id", missionID)
					setMissionError("Mission agent loop failed", output)
					return
				}
				toolResults := missionToolResultCount{}
				if rawCount := strings.TrimSpace(resp.Header.Get("X-Aurago-Mission-Tool-Results")); rawCount != "" {
					if count, err := strconv.Atoi(rawCount); err == nil && count >= 0 {
						toolResults = missionToolResultCount{Value: count, Known: true}
					}
				}
				assessment := assessMissionCompletion(output, toolResults)
				reportedSuspicious := strings.EqualFold(resp.Header.Get("X-Aurago-Mission-Suspicious-Completion"), "true")
				reason := strings.TrimSpace(resp.Header.Get("X-Aurago-Mission-Suspicious-Reason"))
				if reason == "" {
					reason = assessment.Reason
				}
				if reportedSuspicious || assessment.Suspicious {
					logger.Warn("[MissionV2] Mission response looked incomplete, refusing success",
						"mission_id", missionID,
						"tool_results", toolResults.Value,
						"tool_results_known", toolResults.Known,
						"reason", reason)
					setMissionError(
						"Mission response looked incomplete",
						missionSuspiciousCompletionDetail(reason, output),
					)
					return
				} else {
					logger.Info("[MissionV2] Mission executed successfully", "mission_id", missionID, "tool_results", toolResults.Value, "tool_results_known", toolResults.Known)
					// A cancel that raced with a successful completion leaves a
					// stale flag behind; discard it so it cannot misclassify a
					// later failure of the same mission.
					s.missionRunTracker().consumeCancelled(missionID)
					s.MissionManagerV2.SetResult(missionID, "success", output)
					if s.PlannerDB != nil {
						if _, err := planner.ResolveOperationalIssue(s.PlannerDB, "mission|"+missionID, "Mission completed successfully with a verified final response.", time.Now()); err != nil {
							logger.Warn("[MissionV2] Failed to resolve mission operational issue", "mission_id", missionID, "error", err)
						}
					}
				}
			} else {
				logger.Error("[MissionV2] Mission returned non-OK status", "status", resp.Status, "mission_id", missionID)
				setMissionError("Mission returned non-OK status", string(respBody))
				return
			}
			broadcastMissionState(s)
		}()
	}
	s.MissionManagerV2.SetCallback(missionCallbackV2)
	if opts.EggMissionResultSink != nil {
		s.MissionManagerV2.SetCompletionCallback(func(missionID, result, output string) {
			if !s.MissionManagerV2.IsSyncedFromMaster(missionID) {
				return
			}
			payload := bridge.MissionResultPayload{
				MissionID: missionID,
				Result:    result,
				Output:    output,
			}
			if result != tools.MissionResultSuccess && result != "success" {
				payload.Error = output
			}
			if err := opts.EggMissionResultSink(payload); err != nil {
				logger.Warn("[MissionV2] Failed to send remote mission result", "mission_id", missionID, "error", err)
			}
		})
	}

	// Set webhook manager for webhook triggers
	if s.WebhookManager != nil {
		s.MissionManagerV2.SetWebhookManager(&missionWebhookAdapter{mgr: s.WebhookManager, logger: logger})
	}

	// Register desired mission filters even while MQTT is disabled. The
	// controller activates them when the integration is enabled later; each
	// delivery passes the relay gate against the live config.
	s.MissionManagerV2.SetMQTTManager(&missionMQTTAdapter{logger: logger, config: s.ConfigSnapshot})

	// Set cheatsheet DB for mission prompt expansion
	if s.CheatsheetDB != nil {
		s.MissionManagerV2.SetCheatsheetDB(s.CheatsheetDB)
	}
	if s.EggHub != nil {
		s.MissionManagerV2.SetRemoteMissionClient(newRemoteMissionClient(s))
	}

	// Initialize Mission Preparation system
	if cfg.MissionPreparation.Enabled {
		prepDB, err := tools.InitPreparedMissionsDB(cfg.SQLite.PreparedMissionsPath)
		if err != nil {
			logger.Error("Failed to initialize prepared missions DB", "error", err)
		} else {
			s.PreparedMissionsDB = prepDB
			s.MissionManagerV2.SetPreparedDB(prepDB)
			s.PreparationService = services.NewMissionPreparationService(
				cfg, &s.CfgMu, prepDB, s.MissionManagerV2, logger,
			)
			s.PreparationService.SetAvailableTools(agent.ToolSummariesFromConfig(cfg))
			s.PreparationService.Start(serverCtx)
			logger.Info("Mission preparation service initialized")
		}
	}

	// Initialize Mission Execution History database
	{
		histDB, err := tools.InitMissionHistoryDB(cfg.SQLite.MissionHistoryPath)
		if err != nil {
			logger.Error("Failed to initialize mission history DB", "error", err)
		} else {
			s.MissionHistoryDB = histDB
			s.MissionManagerV2.SetHistoryDB(histDB)
			logger.Info("Mission execution history initialized")
			// Reconcile zombie "running" entries left by previous crashes
			if reconciled, recErr := tools.ReconcileStaleRunningMarks(histDB, 1*time.Hour, logger); recErr != nil {
				logger.Warn("[MissionHistory] Failed to reconcile stale running missions", "error", recErr)
			} else if reconciled > 0 && s.PlannerDB != nil {
				issue := planner.OperationalIssue{
					Source:     "mission_history",
					Title:      "Stale running missions were marked as failed",
					Detail:     fmt.Sprintf("%d mission run(s) were still marked as running after a restart and were reconciled as failed.", reconciled),
					Severity:   "warning",
					Reference:  "mission_history_reconcile",
					OccurredAt: time.Now(),
				}
				if issueID, err := planner.RecordOperationalIssue(s.PlannerDB, issue); err != nil {
					logger.Warn("[MissionHistory] Failed to record stale mission operational issue", "error", err)
				} else if s.MissionManagerV2 != nil {
					s.MissionManagerV2.NotifyPlannerOperationalIssue(issueID, issue.Source, issue.Severity, issue.Title)
				}
			}
		}
	}

	// Set budget tracker callback for budget threshold mission triggers.
	// Use reinitBudgetTracker so the callback is always registered after a reload too.
	s.reinitBudgetTracker(cfg)

	// EasyDrag flows hook into Mission Control before it starts (flow triggers, startup trigger).
	s.initFlows()
	if err := s.MissionManagerV2.StartContext(serverCtx); err != nil {
		logger.Warn("Failed to start MissionManagerV2", "error", err)
	} else if shouldSeedWelcomeContent(s.IsFirstStart) {
		// Seed bundled example missions only during first-start setup.
		// Deleted examples must stay deleted on later restarts.
		tools.SeedWelcomeMissions(s.MissionManagerV2, installDir, logger)
	}
	s.startFlows(serverCtx)

	if cheatsheetDB != nil && shouldSeedWelcomeContent(s.IsFirstStart) {
		// Seed bundled example cheat sheets only during first-start setup.
		tools.SeedWelcomeCheatsheets(cheatsheetDB, installDir, logger)
	}

	s.configureHomeAssistantPoller()

	// Initialize Notes schema in SQLite (idempotent: CREATE TABLE IF NOT EXISTS)
	if err := shortTermMem.InitNotesTables(); err != nil {
		logger.Warn("Failed to initialize notes schema (notes tool may not work)", "error", err)
	}

	// Initialize Journal schema in SQLite (idempotent: CREATE TABLE IF NOT EXISTS)
	if err := shortTermMem.InitJournalTables(); err != nil {
		logger.Warn("Failed to initialize journal schema (journal tool may not work)", "error", err)
	}

	// Initialize Error Learning schema in SQLite
	if err := shortTermMem.InitErrorLearningTable(); err != nil {
		logger.Warn("Failed to initialize error learning schema", "error", err)
	}

	// Initialize Learned Rules schema in SQLite
	if err := shortTermMem.InitLearnedRulesTable(); err != nil {
		logger.Warn("Failed to initialize learned rules schema", "error", err)
	}

	// Start File Indexer if enabled
	if cfg.Indexing.Enabled {
		s.FileIndexer = services.NewFileIndexer(cfg, &s.CfgMu, longTermMem, shortTermMem, logger)
		s.attachFileKGSyncer()
		s.FileIndexer.Start(serverCtx)
		logger.Info("File indexer started", "directories", cfg.Indexing.Directories)
	}

	// Start Firewall Guard loop if enabled
	if cfg.Firewall.Enabled && cfg.Firewall.Mode == "guard" {
		firewallSudoPass := ""
		if firewallGuardNeedsSudoPassword(cfg) {
			firewallSudoPass, _ = vault.ReadSecret("sudo_password")
		}
		go tools.StartFirewallGuard(serverCtx, cfg, logger, firewallSudoPass, func(prompt string) {
			go func() {
				if err := acquireLoopbackSem(serverCtx, loopbackSem); err != nil {
					return
				}
				defer releaseLoopbackSem(loopbackSem)

				url := InternalAPIURL(cfg) + "/v1/chat/completions"
				payload := map[string]interface{}{
					"model":  "aurago",
					"stream": false,
					"messages": []map[string]string{
						{"role": "user", "content": prompt},
					},
				}
				body, _ := json.Marshal(payload)
				req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
				if err != nil {
					logger.Error("[FirewallGuard] Failed to create request", "error", err)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Internal-FollowUp", "true")
				req.Header.Set("X-Internal-Token", s.internalToken)

				client := NewInternalHTTPClient(10 * time.Minute)
				if resp, err := client.Do(req); err != nil {
					logger.Error("[FirewallGuard] Execution failed", "error", err)
				} else {
					_ = resp.Body.Close()
				}
			}()
		})
	}

	// Start Cloudflare Tunnel if enabled and auto_start is true
	if cloudflareTunnelAutoStartAllowed(cfg) {
		go func() {
			tunnelCfg := cloudflareTunnelRuntimeConfig(cfg)
			result := tools.CloudflareTunnelStart(tunnelCfg, vault, registry, logger)
			logger.Info("[CloudflareTunnel] Auto-start result", "result", result)
		}()
	} else if cfg.CloudflareTunnel.Enabled && cfg.CloudflareTunnel.AutoStart && !cfg.Docker.Enabled {
		logger.Info("[CloudflareTunnel] Docker is disabled; skipping Docker-mode auto-start")
	}

	if cfg.Docker.Enabled {
		// Auto-start Gotenberg container if document_creator is enabled with gotenberg backend
		if cfg.Tools.DocumentCreator.Enabled && strings.EqualFold(cfg.Tools.DocumentCreator.Backend, "gotenberg") {
			go tools.EnsureGotenbergRunning(cfg.Docker.Host, logger)
		}

		// Auto-start Browser Automation sidecar whenever the integration is active in sidecar mode and auto_start is enabled.
		if cfg.BrowserAutomation.Enabled && cfg.Tools.BrowserAutomation.Enabled && cfg.BrowserAutomation.AutoStart && strings.EqualFold(cfg.BrowserAutomation.Mode, "sidecar") {
			if sidecarCfg, err := tools.ResolveBrowserAutomationSidecarConfig(cfg); err == nil {
				go tools.EnsureBrowserAutomationSidecarRunning(cfg.Docker.Host, sidecarCfg, logger)
			} else {
				logger.Warn("[BrowserAutomation] Failed to resolve sidecar config", "error", err)
			}
		}

		// Auto-start Space Agent sidecar whenever the integration is active and auto_start is enabled.
		if cfg.SpaceAgent.Enabled && cfg.SpaceAgent.AutoStart {
			if err := s.ensureSpaceAgentSecrets(cfg); err != nil {
				logger.Warn("[SpaceAgent] Failed to ensure vault secrets", "error", err)
			} else if sidecarCfg, err := tools.ResolveSpaceAgentSidecarConfig(cfg, spaceAgentBridgeBaseURL(s, cfg, nil)); err == nil {
				go tools.EnsureSpaceAgentSidecarRunning(cfg.Docker.Host, sidecarCfg, logger)
			} else {
				logger.Warn("[SpaceAgent] Failed to resolve sidecar config", "error", err)
			}
		}

		// Auto-start Manifest sidecars whenever the integration is active in managed mode and auto_start is enabled.
		if cfg.Manifest.Enabled && cfg.Manifest.AutoStart && strings.EqualFold(cfg.Manifest.Mode, "managed") {
			if err := s.ensureManifestSecrets(cfg); err != nil {
				logger.Warn("[Manifest] Failed to ensure vault secrets", "error", err)
			} else {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cancel()
					if err := tools.EnsureManifestSidecarsRunning(ctx, cfg.Docker.Host, cfg, logger); err != nil {
						logger.Warn("[Manifest] Failed to auto-start sidecars", "error", err)
					}
				}()
			}
		}

		// Auto-start OmniRoute sidecar whenever the integration is active in managed mode and auto_start is enabled.
		if cfg.OmniRoute.Enabled && cfg.OmniRoute.AutoStart && strings.EqualFold(cfg.OmniRoute.Mode, "managed") {
			if err := s.ensureOmniRouteSecrets(cfg); err != nil {
				logger.Warn("[OmniRoute] Failed to ensure vault secrets", "error", err)
			} else {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cancel()
					if err := tools.EnsureOmniRouteSidecarRunning(ctx, cfg.Docker.Host, cfg, logger); err != nil {
						logger.Warn("[OmniRoute] Failed to auto-start sidecar", "error", err)
					}
				}()
			}
		}

		// Auto-start Dograh stack whenever the integration is active in managed mode and auto_start is enabled.
		if cfg.Dograh.Enabled && cfg.Dograh.AutoStart && strings.EqualFold(cfg.Dograh.Mode, "managed") {
			if err := s.ensureDograhSecrets(cfg); err != nil {
				logger.Warn("[Dograh] Failed to ensure vault secrets", "error", err)
			} else {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
					defer cancel()
					if err := tools.EnsureDograhStackRunning(ctx, cfg.Docker.Host, cfg, logger); err != nil {
						logger.Warn("[Dograh] Failed to auto-start stack", "error", err)
					}
				}()
			}
		}

		// Auto-start Ansible sidecar container if enabled in sidecar mode
		if cfg.Ansible.Enabled && cfg.Ansible.Mode == "sidecar" {
			inventoryDir := ""
			if cfg.Ansible.DefaultInventory != "" {
				inventoryDir = filepath.Dir(cfg.Ansible.DefaultInventory)
			}
			go tools.EnsureAnsibleSidecarRunning(cfg.Docker.Host, tools.AnsibleSidecarConfig{
				Token:         cfg.Ansible.Token,
				Timeout:       cfg.Ansible.Timeout,
				Image:         cfg.Ansible.Image,
				ContainerName: cfg.Ansible.ContainerName,
				PlaybooksDir:  cfg.Ansible.PlaybooksDir,
				InventoryDir:  inventoryDir,
				AutoBuild:     cfg.Ansible.AutoBuild,
				DockerfileDir: cfg.Ansible.DockerfileDir,
			}, logger)
		}

		// Auto-start local Ollama embeddings container if enabled
		if cfg.Embeddings.LocalOllama.Enabled {
			go tools.EnsureOllamaEmbeddingsRunning(cfg, logger)
		}

		// Auto-start Piper TTS container if enabled
		if cfg.TTS.Piper.Enabled {
			go tools.EnsurePiperRunning(cfg, logger)
		}
		if strings.EqualFold(strings.TrimSpace(cfg.TTS.Provider), "supertonic") && cfg.TTS.Supertonic.AutoStart {
			go tools.EnsureSupertonicRunning(cfg, logger)
		}

		// Auto-start managed Ollama container if enabled
		if cfg.Ollama.ManagedInstance.Enabled {
			go tools.EnsureOllamaManagedRunning(cfg, logger)
		}
	} else {
		logger.Info("Docker is disabled; skipping managed sidecar auto-start")
	}

	s.configureFritzPoller()

	// Initialize A2A Protocol support
	if cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled {
		if cfg.A2A.Server.Enabled {
			a2aDeps := &a2apkg.ExecutorDeps{
				Config:       cfg,
				Logger:       logger,
				LLMClient:    llmClient,
				ShortTermMem: shortTermMem,
				LongTermMem:  longTermMem,
				Vault:        vault,
				Guardian:     s.Guardian,
				Registry:     registry,
				Manifest:     tools.NewManifest(cfg.Directories.ToolsDir),
				KG:           kg,
				InventoryDB:  inventoryDB,
				Budget:       s.BudgetTracker,
			}
			s.A2AServer = a2apkg.NewServer(cfg, logger, a2aDeps)
			s.A2AServer.StartCleanup(serverCtx)
			logger.Info("A2A server initialized",
				"bindings_rest", cfg.A2A.Server.Bindings.REST,
				"bindings_jsonrpc", cfg.A2A.Server.Bindings.JSONRPC,
				"bindings_grpc", cfg.A2A.Server.Bindings.GRPC,
			)

			// Start gRPC on dedicated port if enabled
			if cfg.A2A.Server.Bindings.GRPC {
				go func() {
					if err := s.A2AServer.StartGRPCServer(serverCtx); err != nil {
						logger.Error("A2A gRPC server failed", "error", err)
					}
				}()
			}

			// Start dedicated HTTP server if configured
			if cfg.A2A.Server.Port > 0 {
				go func() {
					if err := s.A2AServer.StartDedicatedServer(serverCtx); err != nil {
						logger.Error("A2A dedicated server failed", "error", err)
					}
				}()
			}
		}

		if cfg.A2A.Client.Enabled {
			s.A2AClientMgr = a2apkg.NewClientManager(cfg, logger)
			s.A2AClientMgr.Initialize(serverCtx)
			s.A2AClientMgr.StartHealthCheck(serverCtx, 5*time.Minute)
			logger.Info("A2A client manager initialized", "remote_agents", len(cfg.A2A.Client.RemoteAgents))

			// Create bridge for co-agent integration
			if s.CoAgentRegistry != nil {
				s.A2ABridge = a2apkg.NewBridge(s.A2AClientMgr, s.CoAgentRegistry, logger)
			}
		}
	}

	// Start radio ingress after agent dependencies have finished initialization.
	if err := s.initMeshCore(serverCtx); err != nil {
		return err
	}
	defer func() {
		if s.MeshCore != nil {
			_ = s.MeshCore.Close()
			if meshcore.DefaultManager() == s.MeshCore {
				meshcore.SetDefaultManager(nil)
			}
		}
	}()
	return s.run(shutdownCh)
}

func acquireLoopbackSem(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseLoopbackSem(sem chan struct{}) {
	<-sem
}

func newServerFromOptions(opts StartOptions) *Server {
	cfg := opts.Cfg
	logger := opts.Logger
	bluetoothManager := bluetooth.NewManager(logger)
	bluetoothManager.Configure(config.BluetoothRuntimeOptions(cfg))
	bluetoothManager.SeedStatus(cfg.Runtime.Bluetooth)
	bluetooth.SetDefaultManager(bluetoothManager)
	var networkSharesManager *networkshares.Manager
	networkSharesManager, networkSharesErr := networkshares.OpenManager(cfg.SQLite.NetworkSharesPath, logger)
	if networkSharesErr != nil {
		cfg.Runtime.NetworkShares = networkshares.UnavailableStatus("The network share ownership ledger could not be opened.")
		if logger != nil {
			logger.Warn("[NetworkShares] Failed to open ownership ledger", "error", networkSharesErr)
		}
		networkshares.SetDefaultManager(nil)
	} else {
		sudoPassword := ""
		if opts.Vault != nil && cfg.Agent.SudoEnabled {
			sudoPassword, _ = opts.Vault.ReadSecret("sudo_password")
		}
		networkSharesManager.Configure(config.NetworkSharesOptions(cfg, sudoPassword))
		probeCtx, cancelProbe := context.WithTimeout(context.Background(), 5*time.Second)
		cfg.Runtime.NetworkShares = networkSharesManager.Reprobe(probeCtx)
		cancelProbe()
		networkshares.SetDefaultManager(networkSharesManager)
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
	if opts.CronManager != nil {
		if err := opts.CronManager.RefreshRuntimePermissions(); err != nil && logger != nil {
			logger.Warn("Failed to refresh cron runtime permissions", "error", err)
		}
	}

	// Build the server-wide Guardian with the same promptsec options as the agent loop.
	guardian := security.NewGuardianWithOptions(logger, security.GuardianOptions{
		MaxScanBytes:  cfg.Guardian.MaxScanBytes,
		ScanEdgeBytes: cfg.Guardian.ScanEdgeBytes,
		Preset:        cfg.Guardian.PromptSec.Preset,
		Sanitizer: security.PromptSecSanitizerOptions{
			Normalize:   cfg.Guardian.PromptSec.Sanitizer.Normalize,
			Dehomoglyph: cfg.Guardian.PromptSec.Sanitizer.Dehomoglyph,
			Decode:      cfg.Guardian.PromptSec.Sanitizer.Decode,
		},
		Embedding: security.PromptSecEmbeddingOptions{
			Enabled:   cfg.Guardian.PromptSec.Embedding.Enabled,
			Threshold: cfg.Guardian.PromptSec.Embedding.Threshold,
		},
		Policy:       cfg.Guardian.PromptSec.Policy,
		CustomPolicy: security.PromptSecCustomPolicyOptions{DisallowedTasks: cfg.Guardian.PromptSec.CustomPolicy.DisallowedTasks},
		Taint: security.PromptSecTaintOptions{
			Enabled:      cfg.Guardian.PromptSec.Taint.Enabled,
			DefaultLevel: cfg.Guardian.PromptSec.Taint.DefaultLevel,
		},
		LLMJudge: security.PromptSecLLMJudgeOptions{
			Enabled:     cfg.Guardian.PromptSec.LLMJudge.Enabled,
			Mode:        cfg.Guardian.PromptSec.LLMJudge.Mode,
			TimeoutSecs: cfg.Guardian.PromptSec.LLMJudge.TimeoutSecs,
			Policy:      cfg.Guardian.PromptSec.LLMJudge.Policy,
		},
		UseSanitizedOutput: cfg.Guardian.PromptSec.UseSanitizedOutput,
	})
	llmGuardian := security.NewLLMGuardian(cfg, logger)
	if cfg.Guardian.PromptSec.LLMJudge.Enabled {
		guardian.AttachLLMJudge(llmGuardian, security.PromptSecLLMJudgeOptions{
			Enabled:     cfg.Guardian.PromptSec.LLMJudge.Enabled,
			Mode:        cfg.Guardian.PromptSec.LLMJudge.Mode,
			TimeoutSecs: cfg.Guardian.PromptSec.LLMJudge.TimeoutSecs,
			Policy:      cfg.Guardian.PromptSec.LLMJudge.Policy,
		})
	}

	s := &Server{
		Cfg:                     cfg,
		SetupCSRFTokens:         make(map[string]time.Time),
		Logger:                  logger,
		AccessLogger:            opts.AccessLogger,
		LLMClient:               opts.LLMClient,
		LocalLLM:                opts.LocalLLM,
		ShortTermMem:            opts.ShortTermMem,
		LongTermMem:             opts.LongTermMem,
		Vault:                   opts.Vault,
		Registry:                opts.Registry,
		CronManager:             opts.CronManager,
		BackgroundTasks:         opts.BackgroundTasks,
		Bluetooth:               bluetoothManager,
		NetworkShares:           networkSharesManager,
		VirtualComputersDB:      opts.VirtualComputersDB,
		VirtualWorkspaceManager: opts.VirtualWorkspaceManager,
		HistoryManager:          opts.HistoryManager,
		KG:                      opts.KG,
		InventoryDB:             opts.InventoryDB,
		InvasionDB:              opts.InvasionDB,
		CheatsheetDB:            opts.CheatsheetDB,
		ImageGalleryDB:          opts.ImageGalleryDB,
		MediaRegistryDB:         opts.MediaRegistryDB,
		HomepageRegistryDB:      opts.HomepageRegistryDB,
		ContactsDB:              opts.ContactsDB,
		PlannerDB:               opts.PlannerDB,
		LaunchpadDB:             opts.LaunchpadDB,
		SQLConnectionsDB:        opts.SQLConnectionsDB,
		SQLConnectionPool:       opts.SQLConnectionPool,
		Guardian:                guardian,
		LLMGuardian:             llmGuardian,
		CoAgentRegistry:         agent.NewCoAgentRegistry(cfg.CoAgents.MaxConcurrent, logger),
		BudgetTracker:           budget.NewTracker(cfg, logger, cfg.Directories.DataDir),
		IsFirstStart:            opts.IsFirstStart,
		StartedAt:               time.Now(),
		ShutdownCh:              opts.ShutdownCh,
		MissionManagerV2:        tools.NewMissionManagerV2(cfg.Directories.DataDir, opts.CronManager),
		EggHub:                  bridge.NewEggHub(logger),
		WarningsRegistry:        opts.WarningsRegistry,
	}
	s.agodeskDevToken = loadAgodeskDevToken(logger)
	speechCfg := effectiveSpeechLabConfig(cfg)
	if speechLabClient, err := speechlab.NewClient(speechCfg); err != nil {
		logger.Warn("Speech Lab client is unavailable", "error", err)
	} else {
		s.SpeechLab = speechLabClient
	}
	// Keep one stable deployer instance for the full process lifetime. This
	// avoids pointer races during reload and preserves safe cleanup access after
	// Speech Lab is disabled or switched to an external deployment.
	s.SpeechLabDeployer = deployer.NewManager(speechCfg, cfg.Runtime.IsDocker, cfg.Docker.Enabled, cfg.Docker.ReadOnly, cfg.Directories.DataDir, logger, deployer.WithDockerClient(dockerutil.NewClient(cfg.Docker.Host, 30*time.Second)))
	s.initConfigSnapshot()
	if opts.Vault != nil {
		s.VaultSecretPrompter = vaultprompt.NewManager(opts.Vault, 5*time.Minute)
	}
	s.Go2RTC = tools.NewGo2RTCManager(cfg, opts.Vault, opts.MediaRegistryDB, logger)
	s.LocalMusic = acestep.New(cfg, opts.Vault, logger)
	acestep.SetDefault(s.LocalMusic)
	s.Go2RTCDiscovery = onvif.NewService(cfg.Runtime.BroadcastOK)
	tools.SetDefaultGo2RTCManager(s.Go2RTC)
	s.LocalWiki = newLocalWikipediaManager(cfg, logger)
	publishLocalWikipediaTool(s.LocalWiki)
	return s
}

func shouldSeedWelcomeContent(isFirstStart bool) bool {
	return isFirstStart
}

func (s *Server) initSkillManagers(ctx context.Context, installDir string) gamemaker.SkillInstallResult {
	cfg := s.Cfg
	logger := s.Logger
	if cfg == nil || logger == nil {
		return gamemaker.SkillInstallResult{}
	}
	skillsDB, err := tools.InitSkillsDB(cfg.SQLite.SkillsPath)
	if err != nil {
		logger.Warn("Failed to initialize Skills DB", "error", err, "path", cfg.SQLite.SkillsPath)
		return gamemaker.SkillInstallResult{}
	}
	s.SkillsDB = skillsDB

	if cfg.Tools.SkillManager.Enabled {
		s.SkillManager = tools.NewSkillManager(skillsDB, cfg.Directories.SkillsDir, logger)
		tools.SetDefaultSkillManager(s.SkillManager)
		if err := s.SkillManager.SyncFromDisk(); err != nil {
			logger.Warn("Failed to sync skills from disk", "error", err)
		}
		logger.Info("Skill Manager initialized", "skills_dir", cfg.Directories.SkillsDir)
		if shouldSeedWelcomeContent(s.IsFirstStart) {
			tools.SeedWelcomeSkills(s.SkillManager, cfg.Directories.SkillsDir, installDir, logger)
		}
	} else {
		tools.SetDefaultSkillManager(nil)
	}

	if err := tools.MigrateAgentSkillsDB(skillsDB); err != nil {
		logger.Warn("Failed to initialize Agent Skills schema", "error", err)
		return gamemaker.SkillInstallResult{}
	}
	installResult, installErr := gamemaker.InstallBundledSkills(cfg.Directories.AgentSkillsDir)
	if installErr != nil {
		logger.Warn("Failed to install bundled Game Maker Agent Skills", "error", installErr)
		installResult.Ready = false
	}
	s.AgentSkillManager = tools.NewAgentSkillManager(skillsDB, cfg.Directories.AgentSkillsDir, cfg.Directories.WorkspaceDir, logger)
	tools.SetDefaultAgentSkillManager(s.AgentSkillManager)
	// Compile-time packages are checked locally before Game Maker is exposed.
	// Other discovered skills still use the optional background scanners.
	s.gameMakerSkills, s.gameMakerSkillsReady = verifyGameMakerAgentSkills(ctx, s.AgentSkillManager, installResult, logger)
	logger.Info("Agent Skills initialized", "agent_skills_dir", cfg.Directories.AgentSkillsDir, "game_maker_skills_ready", s.gameMakerSkillsReady)
	return installResult
}

func (s *Server) syncAgentSkills(ctx context.Context, cfg *config.Config, installResult gamemaker.SkillInstallResult) {
	if s.AgentSkillManager == nil {
		return
	}
	// Bound optional startup work, while retaining the server shutdown context.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	s.CfgMu.RLock()
	guardian := s.LLMGuardian
	useGuardian := cfg.Tools.SkillManager.ScanWithGuardian
	s.CfgMu.RUnlock()
	bundledOrigins := make(map[string]tools.SkillOrigin, len(installResult.Skills))
	for _, bundledSkill := range installResult.Skills {
		bundledOrigins[bundledSkill.Name] = tools.OriginSystem
	}
	if err := s.AgentSkillManager.SyncFromDiskWithOrigins(ctx, bundledOrigins, guardian, useGuardian, skillSpectorConfig(s)); err != nil {
		s.Logger.Warn("Failed to sync Agent Skills from disk", "error", err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	skills, ready := verifyGameMakerAgentSkills(ctx, s.AgentSkillManager, installResult, s.Logger)
	if s.GameMaker != nil {
		s.GameMaker.SetSkillStatus(skills, ready)
	}
	s.Logger.Info("Agent Skills security verification completed", "game_maker_skills_ready", ready)
}

// runHTTP starts the server in HTTP mode (for local/LAN use)
func (s *Server) runHTTP(mux *http.ServeMux, ttsServer *http.Server, shutdownCh chan struct{}) error {
	addr := fmt.Sprintf("%s:%d", s.Cfg.Server.Host, s.Cfg.Server.Port)
	s.Logger.Info("Starting HTTP server", "host", s.Cfg.Server.Host, "port", s.Cfg.Server.Port, "tls", false)

	// Apply security headers (relaxed for HTTP, but still present).
	// Gzip sits outside access logging so static UI assets compress for clients
	// without wrapping WebSocket/SSE (those are skipped inside gzipMiddleware).
	handler := trustedProxyMiddleware(s, previewHostMiddleware(s, desktopTicketMiddleware(panicRecoveryMiddleware(s.Logger, gzipMiddleware(accessLogMiddleware(s.accessLogger(), securityHeadersMiddleware(authMiddleware(s, mux), false, s.Cfg.Server.HTTPS.BehindProxy), s.Cfg.Server.HTTPS.BehindProxy))))))

	server := newAgentHTTPServer(addr, handler)

	return s.serveWithShutdown(server, nil, ttsServer, shutdownCh)
}

// runHTTPS starts the server with auto-TLS (Let's Encrypt)
func (s *Server) runHTTPS(mux *http.ServeMux, ttsServer *http.Server, tlsCfg *TLSConfig, shutdownCh chan struct{}) error {
	tlsCfg.HTTPSPort = s.Cfg.Server.HTTPS.HTTPSPort
	tlsCfg.HTTPPort = s.Cfg.Server.HTTPS.HTTPPort

	// Apply security headers (strict for HTTPS)
	handler := trustedProxyMiddleware(s, previewHostMiddleware(s, desktopTicketMiddleware(panicRecoveryMiddleware(s.Logger, gzipMiddleware(accessLogMiddleware(s.accessLogger(), securityHeadersMiddleware(authMiddleware(s, mux), true, s.Cfg.Server.HTTPS.BehindProxy), s.Cfg.Server.HTTPS.BehindProxy))))))

	httpsServer, httpServer, err := SetupServers(tlsCfg, handler, s.Logger)
	if err != nil {
		return fmt.Errorf("failed to setup TLS servers: %w", err)
	}

	s.Logger.Info("Starting HTTPS servers",
		"domain", tlsCfg.Domain,
		"https_port", tlsCfg.HTTPSPort,
		"http_port", tlsCfg.HTTPPort,
		"email", tlsCfg.Email,
		"cert_dir", tlsCfg.CertDir)

	return s.serveWithShutdown(httpsServer, httpServer, ttsServer, shutdownCh)
}

// serveWithShutdown handles graceful shutdown for servers
func (s *Server) serveWithShutdown(server, redirectServer, ttsServer *http.Server, shutdownCh chan struct{}) error {
	if server.Handler == nil {
		server.Handler = http.DefaultServeMux
	}
	server.Handler = s.trackHTTP(server.Handler)
	if redirectServer != nil {
		redirectServer.Handler = s.trackHTTP(redirectServer.Handler)
	}
	shutdownDone := make(chan struct{})
	serveDone := make(chan struct{})
	s.announceSetupBootstrap()
	// Start redirect server (if provided) in background
	if redirectServer != nil {
		go func() {
			s.Logger.Info("Starting HTTP redirect server", "addr", redirectServer.Addr)
			if err := redirectServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.Logger.Warn("HTTP redirect server error (non-fatal — disable with http_port: 0 in config)", "error", err)
			}
		}()
	}

	// Graceful shutdown handler
	go func() {
		select {
		case <-shutdownCh:
		case <-serveDone:
		}
		defer close(shutdownDone)
		s.ready.Store(false)
		s.beginHTTPDrain()
		s.Logger.Info("Initiating graceful server shutdown...")
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		// Stop accepting and cancel streams before touching their dependencies.
		for _, listener := range []*http.Server{server, redirectServer, ttsServer, s.loopbackSrv, s.spaceAgentHTTPS} {
			if listener == nil {
				continue
			}
			if err := listener.Shutdown(ctx); err != nil {
				s.Logger.Warn("HTTP drain deadline reached; closing connections", "error", err)
				_ = listener.Close()
			}
		}
		if s.TsNetManager != nil {
			if err := s.TsNetManager.Shutdown(ctx); err != nil {
				s.Logger.Warn("tsnet shutdown did not complete cleanly", "error", err)
			}
		}
		s.httpRequests.Wait()

		// Flow runs use tools, MQTT, mail, MCP, the sandbox, the mission history and the
		// planner. Cancel and join them before any of those stop (see shutdownFlows).
		s.shutdownFlows(ctx)

		// Relay runs can own network, database and tool activity. Cancel and
		// join them before shutting down any of their dependencies.
		if s.MQTTController != nil {
			if err := s.MQTTController.Stop(context.Background()); err != nil {
				s.Logger.Warn("MQTT shutdown did not complete cleanly", "error", err)
			}
		}

		// Shut down Heartbeat scheduler
		if s.HeartbeatScheduler != nil {
			s.HeartbeatScheduler.Stop()
		}
		if s.MaintenanceScheduler != nil {
			// Maintenance owns database-backed ledgers and must finish cancelling
			// its active run before closeRuntimeResources closes those databases.
			// The HTTP shutdown deadline is intentionally not used here: timing
			// out would let a run outlive its storage dependencies.
			if err := s.MaintenanceScheduler.Stop(context.Background()); err != nil {
				s.Logger.Warn("Maintenance scheduler shutdown did not complete cleanly", "error", err)
			}
		}
		s.stopUptimeKumaPoller()
		if s.AgentMailService != nil {
			s.AgentMailService.Stop(ctx)
		}
		// Shut down MCP servers
		tools.ShutdownMCPManager()
		// Shut down Sandbox
		tools.ShutdownSandboxManager()
		// Shut down Looper if running
		shutdownLooper()
		// Shut down Discord bot
		discord.StopBot(s.Logger)
		// Shut down Cloudflare Tunnel (Docker containers won't be killed by KillAll)
		if tools.IsTunnelRunning() {
			tunnelCfg := tools.CloudflareTunnelConfig{DockerHost: s.Cfg.Docker.Host}
			tools.CloudflareTunnelShutdown(tunnelCfg, s.Registry, s.Logger, false)
		}

		s.closeRuntimeResources()

	}()

	// Start main server
	var err error
	ln, listenErr := net.Listen("tcp", server.Addr)
	if listenErr != nil {
		close(serveDone)
		<-shutdownDone
		richErr := fmt.Errorf("server listen error: %w", listenErr)
		if strings.Contains(listenErr.Error(), "permission denied") || strings.Contains(listenErr.Error(), "bind") {
			richErr = fmt.Errorf("%w\n\nHint: Ports below 1024 (80, 443) require root privileges.\n"+
				"To use HTTPS without root: set server.https.https_port to 8443 (or any high port) and\n"+
				"server.https.http_port to 8080 in your config.yaml", richErr)
		}
		return richErr
	}

	s.ready.Store(true)
	s.Logger.Info("Server ready — accepting connections", "addr", server.Addr)

	if server.TLSConfig != nil {
		err = server.ServeTLS(ln, "", "")
	} else {
		err = server.Serve(ln)
	}

	close(serveDone)
	<-shutdownDone
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	s.Logger.Info("Server stopped gracefully")
	return nil
}

// securityHeadersMiddleware adds security headers based on TLS mode
func securityHeadersMiddleware(next http.Handler, tlsActive, behindProxy bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		allowDesktopIframe := strings.HasPrefix(path, "/files/desktop/") ||
			strings.HasPrefix(path, "/api/go2rtc/viewer/") ||
			strings.HasPrefix(path, "/api/game-maker/preview/")

		// Always set these headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Hardware access belongs to the trusted Desktop document, never an
		// embedded app, workspace document or preview served from this origin.
		w.Header().Set("Permissions-Policy", "serial=()")
		if path == "/desktop" || path == "/desktop/" || path == "/desktop.html" {
			w.Header().Set("Permissions-Policy", "serial=(self)")
		}
		if !allowDesktopIframe {
			w.Header().Set("X-Frame-Options", "DENY")
		}
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		if desktopTicketFromRequest(r) != "" {
			w.Header().Set("Referrer-Policy", "no-referrer")
		} else {
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		}

		// Content Security Policy
		// NOTE: unsafe-inline is still required by legacy inline SPA handlers/styles.
		// unsafe-eval is intentionally omitted; CodeMirror and UI bundles are served
		// as prebuilt static assets and must not require runtime eval.
		// TODO: Replace unsafe-inline with nonce-based CSP after moving inline UI
		// handlers/styles into bundled JS/CSS.
		connectSrc := "connect-src 'self' blob: ws: wss: https://api.open-meteo.com https://geocoding-api.open-meteo.com https://de1.api.radio-browser.info https://iptv-org.github.io" + desktopStoreProxyConnectSources(r) + "; "
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: blob: https:; " +
			"font-src 'self' data:; " +
			connectSrc +
			"media-src 'self' data: blob: http: https:; " +
			"worker-src 'self' blob:; " +
			"object-src 'none'; " +
			"form-action 'self'; " +
			"frame-src 'self' http: https:; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self';"
		w.Header().Set("Content-Security-Policy", csp)
		if path == "/" {
			// Measure the main UI's remaining inline usage without breaking legacy controls.
			reportOnly := strings.Replace(csp, "script-src 'self' 'unsafe-inline'", "script-src 'self'", 1)
			reportOnly = strings.Replace(reportOnly, "style-src 'self' 'unsafe-inline'", "style-src 'self'", 1)
			w.Header().Set("Content-Security-Policy-Report-Only", reportOnly)
		}

		if tlsActive {
			// Strict Transport Security (only for HTTPS)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Cache control: static assets get public 1-hour cache; everything else no-store.
		// Files under /files/ are user data behind authentication, never public
		// static assets: the dedicated media mounts set their own private cache,
		// everything else there (the workspace mount) stays no-store. Local
		// Wikipedia content is authenticated too; its handler sets a private
		// cache on the blobs it serves.
		isStaticAsset := !strings.HasPrefix(path, "/files/") &&
			!strings.HasPrefix(path, localWikiDesktopPrefix) &&
			(strings.HasSuffix(path, ".js") ||
				strings.HasSuffix(path, ".css") ||
				strings.HasSuffix(path, ".png") ||
				strings.HasSuffix(path, ".ico") ||
				strings.HasSuffix(path, ".svg") ||
				strings.HasSuffix(path, ".woff") ||
				strings.HasSuffix(path, ".woff2") ||
				strings.HasSuffix(path, ".ttf") ||
				strings.HasSuffix(path, ".map"))
		if path == "/js/desktop/aura-desktop-sdk.js" {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
		} else if isStaticAsset {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		} else if !strings.HasPrefix(path, "/auth/") &&
			!strings.HasPrefix(path, "/api/auth/") &&
			!strings.HasPrefix(path, "/setup") &&
			!strings.HasPrefix(path, "/static/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
		}

		// Keep MIDI input confined to the trusted Desktop document. A separate
		// field preserves other hardware policies installed by this middleware.
		midiPolicy := "midi=()"
		if path == "/desktop" || path == "/desktop/" || path == "/desktop.html" {
			midiPolicy = "midi=(self)"
		}
		w.Header().Add("Permissions-Policy", midiPolicy)
		next.ServeHTTP(w, r)
	})
}

func desktopStoreProxyConnectSources(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	host = strings.Trim(strings.ToLower(host), ".")
	if host == "" || strings.ContainsAny(host, " \t\r\n;/\\") {
		return ""
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if ip.To4() == nil {
			host = "[" + strings.Trim(host, "[]") + "]"
		}
	} else {
		for _, label := range strings.Split(host, ".") {
			if label == "" {
				return ""
			}
			for _, ch := range label {
				if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
					return ""
				}
			}
		}
	}
	return " http://" + host + ":* https://" + host + ":* ws://" + host + ":* wss://" + host + ":*"
}

// statusRecorder wraps http.ResponseWriter to capture the HTTP status code written
// by the downstream handler so accessLogMiddleware can log it after the response.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker so that WebSocket upgrade requests can pass
// through the statusRecorder wrapper without losing hijack support.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack: feature not supported by underlying ResponseWriter")
	}
	return h.Hijack()
}

// Flush implements http.Flusher so SSE / chunked streams work correctly
// through the statusRecorder wrapper.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// accessLogMiddleware logs every HTTP request in a structured format useful for
// security monitoring and incident response. Static asset requests (JS, CSS,
// fonts, images), health probes and the /events SSE stream are skipped.
// High-frequency read-only polling (successful GET/HEAD/OPTIONS on dashboard
// and status paths) is logged at Debug; mutations and every response >= 400
// on those paths always reach the access log.
//
// Log fields:
//   - method, path, status, duration_ms, ip, user_agent
func accessLogMiddleware(logger *slog.Logger, next http.Handler, behindProxy bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip noisy static assets that are irrelevant for security monitoring.
		path := r.URL.Path
		skip := strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".woff") ||
			strings.HasSuffix(path, ".svg") ||
			strings.HasSuffix(path, ".map") ||
			path == "/api/health" ||
			path == "/api/ready" ||
			path == "/events" // long-lived SSE stream: keep the writer unwrapped
		if skip {
			next.ServeHTTP(w, r)
			return
		}

		// High-frequency read-only polling from the UI is logged at Debug.
		// Mutations and failures on these paths are logged like any request.
		quietPoll := isSafeMethod(r.Method) &&
			(strings.HasPrefix(path, "/api/dashboard/") ||
				path == "/api/personality/state" ||
				path == "/api/tsnet/status")

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Milliseconds()

		// Classify log level: 4xx/5xx responses and mutating auth-related
		// requests (login, logout, totp) are logged at Warn level so they can
		// be filtered easily by monitoring tools. Read-only GET requests to
		// auth paths (e.g. /api/auth/status) are Info to avoid log noise.
		isError := rec.status >= 400
		isAuthPath := strings.HasPrefix(path, "/auth/") ||
			strings.HasPrefix(path, "/api/auth/")
		isAuthWarn := isAuthPath && (r.Method != http.MethodGet || isError)

		args := []any{
			"method", r.Method,
			"path", path,
			"status", rec.status,
			"duration_ms", elapsed,
			"ip", ClientIP(r, behindProxy),
			"user_agent", r.UserAgent(),
		}
		switch {
		case isError || isAuthWarn:
			logger.Warn("[Access]", args...)
		case quietPoll:
			logger.Debug("[Access]", args...)
		default:
			logger.Info("[Access]", args...)
		}
	})
}
