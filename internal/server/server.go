package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/discord"
	"aurago/internal/fritzbox"
	"aurago/internal/heartbeat"
	"aurago/internal/i18n"
	"aurago/internal/invasion/bridge"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/proxy"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
	"aurago/internal/tsnetnode"
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
func DedicatedInternalLoopbackPort(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	if cfg.CloudflareTunnel.LoopbackPort > 0 {
		return cfg.CloudflareTunnel.LoopbackPort
	}
	if !cfg.Server.HTTPS.Enabled {
		return 0
	}
	port := cfg.Server.Port
	if port <= 0 || port == cfg.Server.HTTPS.HTTPSPort {
		return 0
	}
	if cfg.Server.HTTPS.HTTPPort > 0 && port == cfg.Server.HTTPS.HTTPPort {
		return 0
	}
	return port
}

// InternalAPIURL returns the base URL for internal (loopback) API calls.
// HTTPS instances prefer a dedicated 127.0.0.1 plain-HTTP listener so scheduled
// automation is not coupled to the public TLS listener. If no dedicated
// listener is configured, the function falls back to the active public scheme.
// This is the single source of truth for all internal API URL construction.
func InternalAPIURL(cfg *config.Config) string {
	if port := DedicatedInternalLoopbackPort(cfg); port > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	scheme := "http"
	port := cfg.Server.Port
	if cfg.Server.HTTPS.Enabled {
		scheme = "https"
		if cfg.Server.HTTPS.HTTPSPort > 0 {
			port = cfg.Server.HTTPS.HTTPSPort
		} else {
			port = 443
		}
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
}

// NewInternalHTTPClient returns an http.Client configured for internal loopback
// API calls. It skips TLS verification for fallback HTTPS self-calls because
// InternalAPIURL always resolves to 127.0.0.1 and the server may use a
// self-signed certificate.
func NewInternalHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // SECURE: Only for 127.0.0.1 internal API
			},
			ForceAttemptHTTP2: false,
			DisableKeepAlives: true,
		},
	}
}

// i18nStore holds the parsed translations from ui/lang/ keyed by language code.
// Each value is the raw JSON string for that language, ready for template injection.
// Deprecated: These variables are kept for backward compatibility with tests.
// Use the i18n package functions directly instead.
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
func getI18NJSON(lang string) template.JS {
	return i18n.GetJSON(lang)
}

// getI18NMetaJSON returns the _meta section JSON for config_help metadata.
// Deprecated: Use i18n.GetMetaJSON instead. This wrapper exists for backward compatibility.
func getI18NMetaJSON() template.JS {
	return i18n.GetMetaJSON()
}

// Server holds the state and dependencies for the web server and socket bridge.
type Server struct {
	Cfg                *config.Config
	CfgMu              sync.RWMutex // protects Cfg during hot-reload
	CfgSaveMu          sync.Mutex   // serializes config file writes to prevent TOCTOU races
	Logger             *slog.Logger
	AccessLogger       *slog.Logger
	LLMClient          llm.ChatClient
	ShortTermMem       *memory.SQLiteMemory
	LongTermMem        memory.VectorDB
	Vault              *security.Vault
	Registry           *tools.ProcessRegistry
	CronManager        *tools.CronManager
	BackgroundTasks    *tools.BackgroundTaskManager
	HistoryManager     *memory.HistoryManager
	KG                 *memory.KnowledgeGraph
	InventoryDB        *sql.DB
	InvasionDB         *sql.DB
	Guardian           *security.Guardian
	LLMGuardian        *security.LLMGuardian
	CoAgentRegistry    *agent.CoAgentRegistry
	BudgetTracker      *budget.Tracker
	TokenManager       *security.TokenManager
	WebhookManager     *webhooks.Manager
	WebhookHandler     *webhooks.Handler
	SSE                *SSEBroadcaster // shared SSE broadcaster, set by run()
	MissionManagerV2   *tools.MissionManagerV2
	EggHub             *bridge.EggHub
	RemoteHub          *remote.RemoteHub
	ProxyManager       *proxy.Manager
	TsNetManager       *tsnetnode.Manager
	tsNetHandler       http.Handler // stored so the UI can restart tsnet without a full server restart
	FileIndexer        *services.FileIndexer
	HeartbeatScheduler *heartbeat.Scheduler
	UptimeKumaPoller   *tools.UptimeKumaPoller
	CheatsheetDB       *sql.DB
	ImageGalleryDB     *sql.DB
	MediaRegistryDB    *sql.DB
	HomepageRegistryDB *sql.DB
	ContactsDB         *sql.DB
	PlannerDB          *sql.DB
	SQLConnectionsDB   *sql.DB
	SQLConnectionPool  *sqlconnections.ConnectionPool
	A2AServer          *a2apkg.Server        // A2A protocol server (nil if disabled)
	A2AClientMgr       *a2apkg.ClientManager // A2A client manager (nil if disabled)
	A2ABridge          *a2apkg.Bridge        // A2A co-agent bridge (nil if disabled)
	SkillManager       *tools.SkillManager   // Skill Manager for registry and security scanning
	SkillsDB           *sql.DB               // Skills registry database
	PreparedMissionsDB *sql.DB               // Prepared missions SQLite database
	MissionHistoryDB   *sql.DB               // Mission execution history SQLite database
	PreparationService *services.MissionPreparationService
	WarningsRegistry   *warnings.Registry // Runtime warnings and health issues
	DaemonSupervisor   *tools.DaemonSupervisor
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
}

func (s *Server) accessLogger() *slog.Logger {
	if s.AccessLogger != nil {
		return s.AccessLogger
	}
	return s.Logger
}

// reinitBudgetTracker recreates the BudgetTracker from the current config and
// re-registers the MissionManagerV2 callback. Must be called whenever the config
// is reloaded so that budget threshold mission triggers keep firing.
func (s *Server) reinitBudgetTracker(cfg *config.Config) {
	s.BudgetTracker = budget.NewTracker(cfg, s.Logger, cfg.Directories.DataDir)
	if s.BudgetTracker != nil && s.MissionManagerV2 != nil {
		s.BudgetTracker.SetMissionCallback(func(eventType string, spentUSD, limitUSD, percentage float64) {
			s.MissionManagerV2.NotifyBudgetEvent(eventType, spentUSD, limitUSD, percentage)
		})
	}
}

// StartOptions groups the server boot dependencies so startup wiring stays named and testable.
type StartOptions struct {
	Cfg                  *config.Config
	Logger               *slog.Logger
	AccessLogger         *slog.Logger
	LLMClient            llm.ChatClient
	ShortTermMem         *memory.SQLiteMemory
	LongTermMem          memory.VectorDB
	Vault                *security.Vault
	Registry             *tools.ProcessRegistry
	CronManager          *tools.CronManager
	HistoryManager       *memory.HistoryManager
	KG                   *memory.KnowledgeGraph
	InventoryDB          *sql.DB
	InvasionDB           *sql.DB
	CheatsheetDB         *sql.DB
	ImageGalleryDB       *sql.DB
	RemoteControlDB      *sql.DB
	MediaRegistryDB      *sql.DB
	HomepageRegistryDB   *sql.DB
	ContactsDB           *sql.DB
	PlannerDB            *sql.DB
	SQLConnectionsDB     *sql.DB
	SQLConnectionPool    *sqlconnections.ConnectionPool
	BackgroundTasks      *tools.BackgroundTaskManager
	EggMissionResultSink func(result bridge.MissionResultPayload) error
	WarningsRegistry     *warnings.Registry
	IsFirstStart         bool
	ShutdownCh           chan struct{}
	InstallDir           string
}

func Start(opts StartOptions) error {
	cfg := opts.Cfg
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

	startLoginRecordCleaner(shutdownCh)
	s := newServerFromOptions(opts)
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
		SessionID:          "heartbeat",
		MessageSource:      "heartbeat",
	}
	s.HeartbeatScheduler = heartbeat.New(cfg, logger, func(prompt string) {
		agent.Loopback(hbRunCfg, prompt, agent.NoopBroker{})
	})
	s.HeartbeatScheduler.Start()
	s.restartUptimeKumaPoller()

	// Initialize Skill Manager (always; gated by config in handlers)
	if cfg.Tools.SkillManager.Enabled {
		skillsDB, err := tools.InitSkillsDB(cfg.SQLite.SkillsPath)
		if err != nil {
			logger.Warn("Failed to initialize Skills DB", "error", err, "path", cfg.SQLite.SkillsPath)
		} else {
			s.SkillsDB = skillsDB
			s.SkillManager = tools.NewSkillManager(skillsDB, cfg.Directories.SkillsDir, logger)
			tools.SetDefaultSkillManager(s.SkillManager)
			if err := s.SkillManager.SyncFromDisk(); err != nil {
				logger.Warn("Failed to sync skills from disk", "error", err)
			}
			logger.Info("Skill Manager initialized", "skills_dir", cfg.Directories.SkillsDir)
			// Seed bundled example skills on first start (idempotent)
			tools.SeedWelcomeSkills(s.SkillManager, cfg.Directories.SkillsDir, installDir, logger)
		}
	}

	// Initialize Remote Control Hub
	remote.InsecureHostKey = cfg.RemoteControl.SSHInsecureHostKey
	if remoteControlDB != nil {
		s.RemoteHub = remote.NewRemoteHub(remoteControlDB, vault, logger)
		s.RemoteHub.DefaultReadOnly = cfg.RemoteControl.ReadOnly
		s.RemoteHub.AutoApprove = cfg.RemoteControl.AutoApprove
		s.RemoteHub.MaxFileSizeMB = cfg.RemoteControl.MaxFileSizeMB
		s.RemoteHub.AuditLogEnabled = cfg.RemoteControl.AuditLog
		s.RemoteHub.OnConnect = func(deviceID, name string) {
			s.MissionManagerV2.NotifyDeviceEvent("device_connected", deviceID, name)
		}
		s.RemoteHub.OnDisconnect = func(deviceID, name string) {
			s.MissionManagerV2.NotifyDeviceEvent("device_disconnected", deviceID, name)
		}
		s.RemoteHub.StartHeartbeatMonitor(30*time.Second, 90*time.Second)
		if err := remote.TrimAuditLog(remoteControlDB, 10000); err != nil {
			logger.Warn("Failed to trim remote audit log", "error", err)
		}
		logger.Info("Remote Control Hub initialized", "insecure_host_key", cfg.RemoteControl.SSHInsecureHostKey)
	}

	// Initialize Security Proxy Manager
	s.ProxyManager = proxy.NewManager(cfg, logger)
	if cfg.SecurityProxy.Enabled {
		logger.Info("Security proxy enabled — starting container automatically")
		go func() {
			if err := s.ProxyManager.Start(); err != nil {
				logger.Warn("Security proxy auto-start failed", "error", err)
			} else {
				logger.Info("Security proxy started")
			}
		}()
	}

	// Auto-start Homepage dev container (aurago-homepage) if homepage is enabled.
	// HomepageInit is idempotent: it starts a stopped container or creates it fresh.
	if cfg.Homepage.Enabled && cfg.Homepage.WorkspacePath != "" {
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
	} else if cfg.Homepage.Enabled && cfg.Homepage.WorkspacePath == "" {
		logger.Warn("Homepage dev container enabled but homepage.workspace_path is not set — skipping auto-start")
	}

	// Auto-start Homepage web server (Caddy) if enabled.
	// Note: webserver_enabled is independent of homepage.enabled (the dev container feature).
	// We only require WorkspacePath to be set so the Docker bind mount has an absolute path.
	if cfg.Homepage.WebServerEnabled && cfg.Homepage.WorkspacePath != "" {
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

	// Initialize Token Manager
	tokenFilePath := filepath.Join(cfg.Directories.DataDir, "tokens.json")
	tm, tmErr := security.NewTokenManager(vault, tokenFilePath)
	if tmErr != nil {
		logger.Warn("Failed to initialize TokenManager, webhooks will be disabled", "error", tmErr)
	}
	s.TokenManager = tm

	// Initialize Webhook Manager + Handler
	if cfg.Webhooks.Enabled && tm != nil {
		whFilePath := filepath.Join(cfg.Directories.DataDir, "webhooks.json")
		whLogPath := filepath.Join(cfg.Directories.DataDir, "webhook_log.json")
		whMgr, whErr := webhooks.NewManager(whFilePath, whLogPath)
		if whErr != nil {
			logger.Error("Failed to initialize WebhookManager", "error", whErr)
		} else {
			s.WebhookManager = whMgr
			s.WebhookHandler = webhooks.NewHandler(whMgr, tm, vault, s.Guardian, s.LLMGuardian, cfg, logger, cfg.Server.Port, int64(cfg.Webhooks.MaxPayloadSize), cfg.Webhooks.RateLimit)
			logger.Info("Webhook system initialized", "max_webhooks", webhooks.MaxWebhooks)
		}
	}

	// Start MissionManagerV2 with enhanced callback that reports completion
	missionCallbackV2 := func(prompt string, missionID string) {
		go func() {
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
				if _, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
					Source:      "mission",
					Context:     missionID,
					Title:       title,
					Detail:      detail,
					Severity:    "error",
					Reference:   missionID,
					Fingerprint: "mission|" + missionID,
					OccurredAt:  time.Now(),
				}); err != nil {
					logger.Warn("[MissionV2] Failed to record internal operational issue", "mission_id", missionID, "error", err)
				}
			}
			setMissionError := func(title, detail string) {
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
			req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
			if err != nil {
				logger.Error("[MissionV2] Failed to create request", "error", err, "mission_id", missionID)
				setMissionError("Mission request creation failed", err.Error())
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-FollowUp", "true")
			req.Header.Set("X-Internal-Token", s.internalToken)
			req.Header.Set("X-Mission-ID", missionID)

			client := NewInternalHTTPClient(35 * time.Minute) // Must exceed the 30-minute agent loop timeout
			resp, err := client.Do(req)
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
				toolResultCount, _ := strconv.Atoi(resp.Header.Get("X-Aurago-Mission-Tool-Results"))
				suspiciousCompletion := strings.EqualFold(resp.Header.Get("X-Aurago-Mission-Suspicious-Completion"), "true") ||
					missionResponseLooksIncomplete(output, toolResultCount)
				if suspiciousCompletion {
					logger.Warn("[MissionV2] Mission response looked incomplete, refusing success",
						"mission_id", missionID,
						"tool_results", toolResultCount)
					setMissionError(
						"Mission response looked incomplete",
						"Mission response looked incomplete: no executed tools were recorded and the final assistant reply resembled a progress update instead of a finished result.\n\n"+output,
					)
					return
				} else {
					logger.Info("[MissionV2] Mission executed successfully", "mission_id", missionID, "tool_results", toolResultCount)
					s.MissionManagerV2.SetResult(missionID, "success", output)
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

	// Set MQTT manager for MQTT message triggers
	if cfg.MQTT.Enabled {
		s.MissionManagerV2.SetMQTTManager(&missionMQTTAdapter{logger: logger})
	}

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
			s.PreparationService.Start(context.Background())
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
				if _, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
					Source:     "mission_history",
					Title:      "Stale running missions were marked as failed",
					Detail:     fmt.Sprintf("%d mission run(s) were still marked as running after a restart and were reconciled as failed.", reconciled),
					Severity:   "warning",
					Reference:  "mission_history_reconcile",
					OccurredAt: time.Now(),
				}); err != nil {
					logger.Warn("[MissionHistory] Failed to record stale mission operational issue", "error", err)
				}
			}
		}
	}

	// Set budget tracker callback for budget threshold mission triggers.
	// Use reinitBudgetTracker so the callback is always registered after a reload too.
	s.reinitBudgetTracker(cfg)

	if err := s.MissionManagerV2.Start(); err != nil {
		logger.Warn("Failed to start MissionManagerV2", "error", err)
	} else {
		// Seed bundled example missions on first start (idempotent)
		tools.SeedWelcomeMissions(s.MissionManagerV2, installDir, logger)
	}

	// Seed bundled example cheat sheets on first start (idempotent)
	if cheatsheetDB != nil {
		tools.SeedWelcomeCheatsheets(cheatsheetDB, installDir, logger)
	}

	// Start Home Assistant Poller
	if cfg.HomeAssistant.Enabled && cfg.HomeAssistant.URL != "" && cfg.HomeAssistant.AccessToken != "" {
		haCfg := tools.HAConfig{
			URL:         cfg.HomeAssistant.URL,
			AccessToken: cfg.HomeAssistant.AccessToken,
		}
		// Context from server could be passed, but Background is safe for background daemon
		go tools.StartHomeAssistantPoller(context.Background(), haCfg, s.MissionManagerV2, logger)
	}

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

	// Start File Indexer if enabled
	if cfg.Indexing.Enabled {
		s.FileIndexer = services.NewFileIndexer(cfg, &s.CfgMu, longTermMem, shortTermMem, logger)
		s.attachFileKGSyncer()
		s.FileIndexer.Start(context.Background())
		logger.Info("File indexer started", "directories", cfg.Indexing.Directories)
	}

	// Start Firewall Guard loop if enabled
	if cfg.Firewall.Enabled && cfg.Firewall.Mode == "guard" {
		firewallSudoPass := ""
		if cfg.Agent.SudoEnabled {
			firewallSudoPass, _ = vault.ReadSecret("sudo_password")
		}
		go tools.StartFirewallGuard(context.Background(), cfg, logger, firewallSudoPass, func(prompt string) {
			go func() {
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
	if cfg.CloudflareTunnel.Enabled && cfg.CloudflareTunnel.AutoStart {
		go func() {
			tunnelCfg := tools.CloudflareTunnelConfig{
				Enabled:        cfg.CloudflareTunnel.Enabled,
				ReadOnly:       cfg.CloudflareTunnel.ReadOnly,
				Mode:           cfg.CloudflareTunnel.Mode,
				AutoStart:      cfg.CloudflareTunnel.AutoStart,
				AuthMethod:     cfg.CloudflareTunnel.AuthMethod,
				TunnelName:     cfg.CloudflareTunnel.TunnelName,
				AccountID:      cfg.CloudflareTunnel.AccountID,
				TunnelID:       cfg.CloudflareTunnel.TunnelID,
				LoopbackPort:   cfg.CloudflareTunnel.LoopbackPort,
				ExposeWebUI:    cfg.CloudflareTunnel.ExposeWebUI,
				ExposeHomepage: cfg.CloudflareTunnel.ExposeHomepage,
				MetricsPort:    cfg.CloudflareTunnel.MetricsPort,
				LogLevel:       cfg.CloudflareTunnel.LogLevel,
				DockerHost:     cfg.Docker.Host,
				WebUIPort:      cfg.Server.Port,
				HomepagePort:   cfg.Homepage.WebServerPort,
				DataDir:        cfg.Directories.DataDir,
				HTTPSEnabled:   cfg.Server.HTTPS.Enabled,
				HTTPSPort:      cfg.Server.HTTPS.HTTPSPort,
			}
			for _, r := range cfg.CloudflareTunnel.CustomIngress {
				tunnelCfg.CustomIngress = append(tunnelCfg.CustomIngress, tools.CloudflareIngress{
					Hostname: r.Hostname,
					Service:  r.Service,
					Path:     r.Path,
				})
			}
			result := tools.CloudflareTunnelStart(tunnelCfg, vault, registry, logger)
			logger.Info("[CloudflareTunnel] Auto-start result", "result", result)
		}()
	}

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

	// Auto-start managed Ollama container if enabled
	if cfg.Ollama.ManagedInstance.Enabled {
		go tools.EnsureOllamaManagedRunning(cfg, logger)
	}

	// Start Fritz!Box telephony poller if enabled
	if cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled && cfg.FritzBox.Telephony.Polling.Enabled {
		fbPoller := fritzbox.NewPoller(*cfg, func(kind, summary string) {
			// Fire mission triggers for Fritz!Box events
			s.MissionManagerV2.NotifyFritzBoxEvent(kind, summary)
			go func() {
				url := InternalAPIURL(cfg) + "/v1/chat/completions"
				prompt := fmt.Sprintf("[FRITZ!BOX EVENT: %s] %s", kind, summary)
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
					logger.Error("[FritzBox Poller] Failed to create loopback request", "error", err)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Internal-FollowUp", "true")
				req.Header.Set("X-Internal-Token", s.internalToken)
				client := NewInternalHTTPClient(10 * time.Minute)
				if resp, err := client.Do(req); err != nil {
					logger.Error("[FritzBox Poller] Loopback request failed", "error", err)
				} else {
					_ = resp.Body.Close()
				}
			}()
		}, logger)
		fbPoller.Start()
		logger.Info("[FritzBox Poller] Telephony polling started")
		go func() {
			<-shutdownCh
			fbPoller.Stop()
		}()
	}

	// Initialize A2A Protocol support
	if cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled {
		serverCtx, serverCancel := context.WithCancel(context.Background())
		go func() {
			<-shutdownCh
			serverCancel()
		}()

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

	return s.run(shutdownCh)
}

func newServerFromOptions(opts StartOptions) *Server {
	cfg := opts.Cfg
	logger := opts.Logger

	return &Server{
		Cfg:                cfg,
		Logger:             logger,
		AccessLogger:       opts.AccessLogger,
		LLMClient:          opts.LLMClient,
		ShortTermMem:       opts.ShortTermMem,
		LongTermMem:        opts.LongTermMem,
		Vault:              opts.Vault,
		Registry:           opts.Registry,
		CronManager:        opts.CronManager,
		BackgroundTasks:    opts.BackgroundTasks,
		HistoryManager:     opts.HistoryManager,
		KG:                 opts.KG,
		InventoryDB:        opts.InventoryDB,
		InvasionDB:         opts.InvasionDB,
		CheatsheetDB:       opts.CheatsheetDB,
		ImageGalleryDB:     opts.ImageGalleryDB,
		MediaRegistryDB:    opts.MediaRegistryDB,
		HomepageRegistryDB: opts.HomepageRegistryDB,
		ContactsDB:         opts.ContactsDB,
		PlannerDB:          opts.PlannerDB,
		SQLConnectionsDB:   opts.SQLConnectionsDB,
		SQLConnectionPool:  opts.SQLConnectionPool,
		Guardian: security.NewGuardianWithOptions(logger, security.GuardianOptions{
			MaxScanBytes:  cfg.Guardian.MaxScanBytes,
			ScanEdgeBytes: cfg.Guardian.ScanEdgeBytes,
			Preset:        cfg.Guardian.PromptSec.Preset,
			Spotlight:     cfg.Guardian.PromptSec.Spotlight,
			Canary:        cfg.Guardian.PromptSec.Canary,
		}),
		LLMGuardian:      security.NewLLMGuardian(cfg, logger),
		CoAgentRegistry:  agent.NewCoAgentRegistry(cfg.CoAgents.MaxConcurrent, logger),
		BudgetTracker:    budget.NewTracker(cfg, logger, cfg.Directories.DataDir),
		IsFirstStart:     opts.IsFirstStart,
		StartedAt:        time.Now(),
		ShutdownCh:       opts.ShutdownCh,
		MissionManagerV2: tools.NewMissionManagerV2(cfg.Directories.DataDir, opts.CronManager),
		EggHub:           bridge.NewEggHub(logger),
		WarningsRegistry: opts.WarningsRegistry,
	}
}

// runHTTP starts the server in HTTP mode (for local/LAN use)
func (s *Server) runHTTP(mux *http.ServeMux, ttsServer *http.Server, shutdownCh chan struct{}) error {
	addr := fmt.Sprintf("%s:%d", s.Cfg.Server.Host, s.Cfg.Server.Port)
	s.Logger.Info("Starting HTTP server", "host", s.Cfg.Server.Host, "port", s.Cfg.Server.Port, "tls", false)

	// Apply security headers (relaxed for HTTP, but still present)
	handler := accessLogMiddleware(s.accessLogger(), securityHeadersMiddleware(authMiddleware(s, mux), false, s.Cfg.Server.HTTPS.BehindProxy), s.Cfg.Server.HTTPS.BehindProxy)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	return s.serveWithShutdown(server, nil, ttsServer, shutdownCh)
}

// runHTTPS starts the server with auto-TLS (Let's Encrypt)
func (s *Server) runHTTPS(mux *http.ServeMux, ttsServer *http.Server, tlsCfg *TLSConfig, shutdownCh chan struct{}) error {
	tlsCfg.HTTPSPort = s.Cfg.Server.HTTPS.HTTPSPort
	tlsCfg.HTTPPort = s.Cfg.Server.HTTPS.HTTPPort

	// Apply security headers (strict for HTTPS)
	handler := accessLogMiddleware(s.accessLogger(), securityHeadersMiddleware(authMiddleware(s, mux), true, s.Cfg.Server.HTTPS.BehindProxy), s.Cfg.Server.HTTPS.BehindProxy)

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
		<-shutdownCh
		s.Logger.Info("Initiating graceful server shutdown...")

		// Shut down tsnet node
		if s.TsNetManager != nil {
			s.TsNetManager.Stop()
		}

		// Shut down Heartbeat scheduler
		if s.HeartbeatScheduler != nil {
			s.HeartbeatScheduler.Stop()
		}
		if s.UptimeKumaPoller != nil {
			s.UptimeKumaPoller.Stop()
		}

		// Shut down MCP servers
		tools.ShutdownMCPManager()
		// Shut down Sandbox
		tools.ShutdownSandboxManager()
		// Shut down Discord bot
		discord.StopBot(s.Logger)
		// Shut down Cloudflare Tunnel (Docker containers won't be killed by KillAll)
		if tools.IsTunnelRunning() {
			tunnelCfg := tools.CloudflareTunnelConfig{DockerHost: s.Cfg.Docker.Host}
			tools.CloudflareTunnelStop(tunnelCfg, s.Registry, s.Logger)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if ttsServer != nil {
			ttsServer.Shutdown(ctx)
		}
		if redirectServer != nil {
			redirectServer.Shutdown(ctx)
		}
		if s.loopbackSrv != nil {
			s.loopbackSrv.Shutdown(ctx)
		}
		if err := server.Shutdown(ctx); err != nil {
			s.Logger.Error("Server shutdown error", "error", err)
		}
	}()

	// Start main server
	var err error
	ln, listenErr := net.Listen("tcp", server.Addr)
	if listenErr != nil {
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

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	s.Logger.Info("Server stopped gracefully")
	return nil
}

// securityHeadersMiddleware adds security headers based on TLS mode
func securityHeadersMiddleware(next http.Handler, tlsActive, behindProxy bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always set these headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy
		// NOTE: unsafe-inline is required for inline scripts in the SPA.
		// NOTE: unsafe-eval is required by Tailwind CSS JIT and CodeMirror 6.
		// Removing either would require a full frontend rewrite.
		// TODO: Replace unsafe-inline with nonce-based CSP to improve security.
		// TODO: Evaluate if unsafe-eval can be removed by migrating CodeMirror to a CSP-compliant build.
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com https://unpkg.com; " +
			"style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com https://fonts.googleapis.com; " +
			"img-src 'self' data: blob:; " +
			"font-src 'self' https://fonts.gstatic.com; " +
			"connect-src 'self' ws: wss:; " +
			"object-src 'none'; " +
			"form-action 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self';"
		w.Header().Set("Content-Security-Policy", csp)

		if tlsActive {
			// Strict Transport Security (only for HTTPS)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Cache control: static assets get public 1-hour cache; everything else no-store.
		path := r.URL.Path
		isStaticAsset := strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".svg") ||
			strings.HasSuffix(path, ".woff") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".ttf") ||
			strings.HasSuffix(path, ".map")
		if isStaticAsset {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		} else if !strings.HasPrefix(path, "/auth/") &&
			!strings.HasPrefix(path, "/api/auth/") &&
			!strings.HasPrefix(path, "/setup") &&
			!strings.HasPrefix(path, "/static/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
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
// security monitoring and incident response.  Static asset requests (JS, CSS,
// fonts, images) are silently skipped to keep the log concise.
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
			path == "/api/ready"
		if skip {
			next.ServeHTTP(w, r)
			return
		}

		// High-frequency dashboard polling paths — logged at Debug to avoid log spam.
		// These are read-only status checks that fire every few seconds from the UI.
		quietPoll := strings.HasPrefix(path, "/api/dashboard/") ||
			path == "/api/personality/state" ||
			path == "/api/tsnet/status" ||
			path == "/events"
		if quietPoll {
			next.ServeHTTP(w, r)
			return
		}

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
		if isError || isAuthWarn {
			logger.Warn("[Access]", args...)
		} else {
			logger.Info("[Access]", args...)
		}
	})
}
