package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/discord"
	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/rocketchat"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/telegram"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
	"aurago/ui"
)

// normalizeLang converts the config language string to an ISO code for the frontend
func normalizeLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.Contains(l, "german") || strings.Contains(l, "deutsch") || l == "de":
		return "de"
	case strings.Contains(l, "english") || l == "en":
		return "en"
	case strings.Contains(l, "spanish") || strings.Contains(l, "español") || l == "es":
		return "es"
	case strings.Contains(l, "french") || strings.Contains(l, "français") || l == "fr":
		return "fr"
	case strings.Contains(l, "polish") || strings.Contains(l, "polski") || l == "pl":
		return "pl"
	case strings.Contains(l, "chinese") || strings.Contains(l, "mandarin") || l == "zh":
		return "zh"
	case strings.Contains(l, "hindi") || l == "hi":
		return "hi"
	case strings.Contains(l, "dutch") || strings.Contains(l, "nederlands") || l == "nl":
		return "nl"
	case strings.Contains(l, "italian") || strings.Contains(l, "italiano") || l == "it":
		return "it"
	case strings.Contains(l, "portuguese") || strings.Contains(l, "português") || l == "pt":
		return "pt"
	case strings.Contains(l, "danish") || strings.Contains(l, "dansk") || l == "da":
		return "da"
	case strings.Contains(l, "japanese") || strings.Contains(l, "日本語") || l == "ja":
		return "ja"
	case strings.Contains(l, "swedish") || strings.Contains(l, "svenska") || l == "sv":
		return "sv"
	case strings.Contains(l, "norwegian") || strings.Contains(l, "norsk") || l == "no":
		return "no"
	case strings.Contains(l, "czech") || strings.Contains(l, "čeština") || l == "cs":
		return "cs"
	default:
		return "en" // Fallback
	}
}

// i18nStore holds the parsed i18n.json keyed by language code.
// Each value is the raw JSON string for that language, ready for template injection.
var (
	i18nLangJSON map[string]string // lang -> JSON string of {key: translation, ...}
	i18nMetaJSON string            // JSON string of {key: {options:[...], provider_ref:bool}, ...}
)

// loadI18N reads ui/lang/*/<lang>.json from the embedded FS and prepares per-language JSON blobs.
// Files are organized in category subdirectories (chat/, config/, help/, etc.)
// Config files have an additional level: config/*/<lang>.json
// Each <lang>.json contains a flat {key: translation} map.
// The special file meta.json in the root holds field-option metadata.
func loadI18N(uiFS fs.FS, logger *slog.Logger) {
	i18nLangJSON = make(map[string]string)
	langData := make(map[string]map[string]string) // lang -> key -> translation

	// Read root lang directory
	entries, err := fs.ReadDir(uiFS, "lang")
	if err != nil {
		logger.Error("Failed to read lang/ directory", "error", err)
		i18nLangJSON = map[string]string{"en": "{}"}
		i18nMetaJSON = "{}"
		return
	}

	// Process meta.json from root
	metaData, err := fs.ReadFile(uiFS, "lang/meta.json")
	if err == nil {
		i18nMetaJSON = string(metaData)
	} else {
		i18nMetaJSON = "{}"
	}

	// Process subdirectories recursively
	var processDir func(path string, logger *slog.Logger)
	processDir = func(dirPath string, logger *slog.Logger) {
		entries, err := fs.ReadDir(uiFS, dirPath)
		if err != nil {
			logger.Warn("Failed to read directory", "path", dirPath, "error", err)
			return
		}

		for _, e := range entries {
			itemPath := dirPath + "/" + e.Name()

			if e.IsDir() {
				// Recurse into subdirectory
				processDir(itemPath, logger)
			} else if strings.HasSuffix(e.Name(), ".json") {
				// Process JSON file
				lang := strings.TrimSuffix(e.Name(), ".json")
				data, err := fs.ReadFile(uiFS, itemPath)
				if err != nil {
					logger.Warn("Failed to read lang file", "file", itemPath, "error", err)
					continue
				}

				// Parse JSON and merge into langData
				var translations map[string]string
				if err := json.Unmarshal(data, &translations); err != nil {
					logger.Warn("Failed to parse lang file", "file", itemPath, "error", err)
					continue
				}

				if langData[lang] == nil {
					langData[lang] = make(map[string]string)
				}
				for key, value := range translations {
					langData[lang][key] = value
				}
			}
		}
	}

	// Process all subdirectories in lang/
	for _, e := range entries {
		if e.IsDir() {
			processDir("lang/"+e.Name(), logger)
		}
	}

	// Convert merged data to JSON strings
	for lang, translations := range langData {
		jsonBytes, err := json.Marshal(translations)
		if err != nil {
			logger.Warn("Failed to marshal translations", "lang", lang, "error", err)
			continue
		}
		i18nLangJSON[lang] = string(jsonBytes)
	}

	if len(i18nLangJSON) == 0 {
		i18nLangJSON = map[string]string{"en": "{}"}
	}

	logger.Info("i18n loaded", "languages", len(i18nLangJSON))
}

// getI18NJSON returns the JSON string for the given language, falling back to "en".
func getI18NJSON(lang string) template.JS {
	if j, ok := i18nLangJSON[lang]; ok {
		return template.JS(j)
	}
	if j, ok := i18nLangJSON["en"]; ok {
		return template.JS(j)
	}
	return template.JS("{}")
}

// getI18NMetaJSON returns the _meta section JSON for config_help metadata.
func getI18NMetaJSON() template.JS {
	return template.JS(i18nMetaJSON)
}

// Server holds the state and dependencies for the web server and socket bridge.
type Server struct {
	Cfg              *config.Config
	CfgMu            sync.RWMutex // protects Cfg during hot-reload
	Logger           *slog.Logger
	LLMClient        llm.ChatClient
	ShortTermMem     *memory.SQLiteMemory
	LongTermMem      memory.VectorDB
	Vault            *security.Vault
	Registry         *tools.ProcessRegistry
	CronManager      *tools.CronManager
	HistoryManager   *memory.HistoryManager
	KG               *memory.KnowledgeGraph
	InventoryDB      *sql.DB
	InvasionDB       *sql.DB
	Guardian         *security.Guardian
	CoAgentRegistry  *agent.CoAgentRegistry
	BudgetTracker    *budget.Tracker
	TokenManager     *security.TokenManager
	WebhookManager   *webhooks.Manager
	WebhookHandler   *webhooks.Handler
	MissionManager   *tools.MissionManager
	MissionManagerV2 *tools.MissionManagerV2
	EggHub           *bridge.EggHub
	FileIndexer      *services.FileIndexer
	// IsFirstStart is true if core_memory.md was just freshly created (no prior data).
	IsFirstStart   bool
	StartedAt      time.Time     // server start time for uptime calculation
	ShutdownCh     chan struct{} // signal channel for graceful shutdown
	firstStartDone bool
	muFirstStart   sync.Mutex
}

func Start(cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, isFirstStart bool, shutdownCh chan struct{}) error {
	s := &Server{
		Cfg:              cfg,
		Logger:           logger,
		LLMClient:        llmClient,
		ShortTermMem:     shortTermMem,
		LongTermMem:      longTermMem,
		Vault:            vault,
		Registry:         registry,
		CronManager:      cronManager,
		HistoryManager:   historyManager,
		KG:               kg,
		InventoryDB:      inventoryDB,
		InvasionDB:       invasionDB,
		Guardian:         security.NewGuardian(logger),
		CoAgentRegistry:  agent.NewCoAgentRegistry(cfg.CoAgents.MaxConcurrent, logger),
		BudgetTracker:    budget.NewTracker(cfg, logger, cfg.Directories.DataDir),
		IsFirstStart:     isFirstStart,
		StartedAt:        time.Now(),
		ShutdownCh:       shutdownCh,
		MissionManager:   tools.NewMissionManager(cfg.Directories.DataDir, cronManager),
		MissionManagerV2: tools.NewMissionManagerV2(cfg.Directories.DataDir, cronManager),
		EggHub:           bridge.NewEggHub(logger),
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
			s.WebhookHandler = webhooks.NewHandler(whMgr, tm, logger, cfg.Server.Port, int64(cfg.Webhooks.MaxPayloadSize), cfg.Webhooks.RateLimit)
			logger.Info("Webhook system initialized", "max_webhooks", webhooks.MaxWebhooks)
		}
	}

	// Start MissionManager: loads missions from disk and registers cron jobs for scheduled ones.
	// Wire the callback so RunNow() and cron-triggered missions actually execute the prompt.
	missionCallback := func(prompt string) {
		go func() {
			url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", cfg.Server.Port)
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
				logger.Error("[MissionRun] Failed to create request", "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-FollowUp", "true")

			client := &http.Client{Timeout: 10 * time.Minute}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("[MissionRun] Execution failed", "error", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Info("[MissionRun] Mission executed", "status", resp.Status)
			} else {
				bodyBytes, _ := io.ReadAll(resp.Body)
				logger.Error("[MissionRun] Mission returned non-OK status", "status", resp.Status, "body", string(bodyBytes))
			}
		}()
	}
	if err := s.MissionManager.Start(missionCallback); err != nil {
		logger.Warn("Failed to start MissionManager", "error", err)
	}

	// Start MissionManagerV2 with enhanced callback that reports completion
	missionCallbackV2 := func(prompt string, missionID string) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("[MissionV2] Recovered from panic", "mission_id", missionID, "panic", r)
					s.MissionManagerV2.SetResult(missionID, "error", fmt.Sprintf("panic: %v", r))
				}
			}()

			url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", cfg.Server.Port)
			payload := map[string]interface{}{
				"model":  "aurago",
				"stream": false,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
			}
			// Add mission ID header for tracking
			body, _ := json.Marshal(payload)
			req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
			if err != nil {
				logger.Error("[MissionV2] Failed to create request", "error", err, "mission_id", missionID)
				s.MissionManagerV2.SetResult(missionID, "error", err.Error())
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-FollowUp", "true")
			req.Header.Set("X-Mission-ID", missionID)

			client := &http.Client{Timeout: 10 * time.Minute}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("[MissionV2] Execution failed", "error", err, "mission_id", missionID)
				s.MissionManagerV2.SetResult(missionID, "error", err.Error())
				return
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Info("[MissionV2] Mission executed successfully", "mission_id", missionID)
				s.MissionManagerV2.SetResult(missionID, "success", string(respBody))
			} else {
				logger.Error("[MissionV2] Mission returned non-OK status", "status", resp.Status, "mission_id", missionID)
				s.MissionManagerV2.SetResult(missionID, "error", string(respBody))
			}
		}()
	}
	s.MissionManagerV2.SetCallback(missionCallbackV2)

	// Set webhook manager for webhook triggers
	if s.WebhookManager != nil {
		s.MissionManagerV2.SetWebhookManager(&missionWebhookAdapter{mgr: s.WebhookManager, logger: logger})
	}

	// Set MQTT manager for MQTT message triggers
	if cfg.MQTT.Enabled {
		s.MissionManagerV2.SetMQTTManager(&missionMQTTAdapter{logger: logger})
	}

	if err := s.MissionManagerV2.Start(); err != nil {
		logger.Warn("Failed to start MissionManagerV2", "error", err)
	}

	// Initialize Notes schema in SQLite (idempotent: CREATE TABLE IF NOT EXISTS)
	if err := shortTermMem.InitNotesTables(); err != nil {
		logger.Warn("Failed to initialize notes schema (notes tool may not work)", "error", err)
	}

	// Start File Indexer if enabled
	if cfg.Indexing.Enabled {
		s.FileIndexer = services.NewFileIndexer(cfg, &s.CfgMu, longTermMem, shortTermMem, logger)
		s.FileIndexer.Start(context.Background())
		logger.Info("File indexer started", "directories", cfg.Indexing.Directories)
	}

	return s.run(shutdownCh)
}

func (s *Server) run(shutdownCh chan struct{}) error {
	mux := http.NewServeMux()
	sse := NewSSEBroadcaster()

	// Create a context that cancels on shutdown
	serverCtx, serverCancel := context.WithCancel(context.Background())
	go func() {
		<-shutdownCh
		serverCancel()
	}()

	// Phase 34: Start the background daily reflection loop
	tools.StartDailyReflectionLoop(serverCtx, s.Cfg, s.Logger, s.LLMClient, s.HistoryManager, s.ShortTermMem)

	// Phase 68: Start the daily maintenance loop
	manifest := tools.NewManifest(s.Cfg.Directories.ToolsDir)
	agent.StartMaintenanceLoop(serverCtx, s.Cfg, s.Logger, s.LLMClient, s.Vault, s.Registry, manifest, s.CronManager, s.LongTermMem, s.ShortTermMem, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManager)

	s.CoAgentRegistry.StartCleanupLoop()

	// Start OAuth2 token refresh loop (auto-refreshes before expiry)
	startOAuthRefreshLoop(s, serverCtx)

	mux.HandleFunc("/v1/chat/completions", handleChatCompletions(s, sse))
	mux.HandleFunc("/api/memory/archive", handleArchiveMemory(s))
	mux.HandleFunc("/api/upload", handleUpload(s))
	mux.HandleFunc("/api/budget", handleBudgetStatus(s))
	mux.HandleFunc("/api/credits", handleOpenRouterCredits(s))

	// Quick Setup wizard endpoints (always available — needed before config is complete)
	mux.HandleFunc("/api/setup/status", handleSetupStatus(s))
	mux.HandleFunc("/api/setup", handleSetupSave(s))

	// OpenRouter model browser (always available — needed in both setup wizard and config UI)
	mux.HandleFunc("/api/openrouter/models", handleOpenRouterModels(s))

	// Auth endpoints — always reachable (whitelisted in authMiddleware)
	mux.HandleFunc("/api/auth/status", handleAuthStatus(s))
	mux.HandleFunc("/api/auth/password", handleAuthSetPassword(s))
	mux.HandleFunc("/api/auth/totp/setup", handleAuthTOTPSetup(s))
	mux.HandleFunc("/api/auth/totp/confirm", handleAuthTOTPConfirm(s))
	mux.HandleFunc("/api/auth/totp", handleAuthTOTPDelete(s))

	mux.HandleFunc("/api/personalities", handleListPersonalities(s))
	mux.HandleFunc("/api/personality", handleUpdatePersonality(s))
	mux.HandleFunc("/api/personality/state", handlePersonalityState(s))
	mux.HandleFunc("/api/personality/feedback", handlePersonalityFeedback(s))
	mux.HandleFunc("/events", sse.ServeHTTP) // SSE usually authenticates via cookie/query; keeping open for now unless explicitly needed

	// Config UI endpoints (only when explicitly enabled for security)
	if s.Cfg.WebConfig.Enabled {
		mux.HandleFunc("/api/providers", handleProviders(s))
		mux.HandleFunc("/api/email-accounts", handleEmailAccounts(s))
		mux.HandleFunc("/api/mcp-servers", handleMCPServers(s))
		mux.HandleFunc("/api/sandbox/status", handleSandboxStatus(s))

		// OAuth2 Authorization Code flow endpoints
		mux.HandleFunc("/api/oauth/start", handleOAuthStart(s))
		mux.HandleFunc("/api/oauth/callback", handleOAuthCallback(s))
		mux.HandleFunc("/api/oauth/status", handleOAuthStatus(s))
		mux.HandleFunc("/api/oauth/revoke", handleOAuthRevoke(s))

		mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleGetConfig(s)(w, r)
			case http.MethodPut:
				handleUpdateConfig(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/config/schema", handleGetConfigSchema(s))
		mux.HandleFunc("/api/ui-language", handleUILanguage(s))
		// Lists models available on the configured Ollama instance.
		// Returns the model names as JSON so the UI can offer a model picker.
		mux.HandleFunc("/api/ollama/models", handleOllamaModels(s))
		// Tests MeshCentral connectivity using saved or provided credentials.
		mux.HandleFunc("/api/meshcentral/test", handleMeshCentralTest(s))
		mux.HandleFunc("/api/restart", handleRestart(s))
		mux.HandleFunc("/api/updates/check", handleUpdateCheck(s))
		mux.HandleFunc("/api/updates/install", handleUpdateInstall(s))
		mux.HandleFunc("/api/vault/status", handleVaultStatus(s))
		mux.HandleFunc("/api/vault/secrets", handleVaultSecrets(s))
		mux.HandleFunc("/api/vault", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				handleVaultDelete(s)(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

		// Backup & Restore (.ago archives)
		mux.HandleFunc("/api/backup/create", handleBackupCreate(s))
		mux.HandleFunc("/api/backup/import", handleBackupImport(s))

		// Device Registry (inventory CRUD)
		mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleListDevices(s)(w, r)
			case http.MethodPost:
				handleCreateDevice(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/devices/", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleGetDevice(s)(w, r)
			case http.MethodPut:
				handleUpdateDevice(s)(w, r)
			case http.MethodDelete:
				handleDeleteDevice(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

		// Personality file management (create / read / delete .md files)
		mux.HandleFunc("/api/config/personality-files", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleGetPersonalityContent(s)(w, r)
			case http.MethodPost:
				handleSavePersonalityFile(s)(w, r)
			case http.MethodDelete:
				handleDeletePersonalityFile(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

		// Token admin API (requires web config enabled)
		if s.TokenManager != nil {
			mux.HandleFunc("/api/tokens", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					handleListTokens(s.TokenManager)(w, r)
				case http.MethodPost:
					handleCreateToken(s.TokenManager)(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			})
			mux.HandleFunc("/api/tokens/", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPut:
					handleUpdateToken(s.TokenManager)(w, r)
				case http.MethodDelete:
					handleDeleteToken(s.TokenManager)(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			})
		}

		// Webhook admin API (requires web config enabled)
		if s.WebhookManager != nil {
			mux.HandleFunc("/api/webhooks", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					handleListWebhooks(s.WebhookManager)(w, r)
				case http.MethodPost:
					handleCreateWebhook(s.WebhookManager)(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			})
			mux.HandleFunc("/api/webhooks/presets", handleWebhookPresets)
			mux.HandleFunc("/api/webhooks/log", handleWebhookLogGlobal(s.WebhookManager))
			mux.HandleFunc("/api/webhooks/", func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/api/webhooks/")
				parts := strings.Split(path, "/")
				if len(parts) >= 2 && parts[1] == "log" {
					handleWebhookLog(s.WebhookManager)(w, r)
					return
				}
				if len(parts) >= 2 && parts[1] == "test" {
					handleTestWebhook(s.WebhookManager, s.WebhookHandler)(w, r)
					return
				}
				switch r.Method {
				case http.MethodPut:
					handleUpdateWebhook(s.WebhookManager)(w, r)
				case http.MethodDelete:
					handleDeleteWebhook(s.WebhookManager)(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			})
		}

		// Dashboard API endpoints
		mux.HandleFunc("/api/dashboard/system", handleDashboardSystem(s, sse, s.StartedAt))
		mux.HandleFunc("/api/dashboard/mood-history", handleDashboardMoodHistory(s))
		mux.HandleFunc("/api/dashboard/memory", handleDashboardMemory(s))
		mux.HandleFunc("/api/dashboard/core-memory", handleDashboardCoreMemory(s))
		mux.HandleFunc("/api/dashboard/core-memory/mutate", handleDashboardCoreMemoryMutate(s))
		mux.HandleFunc("/api/dashboard/profile", handleDashboardProfile(s))
		mux.HandleFunc("/api/dashboard/activity", handleDashboardActivity(s))
		mux.HandleFunc("/api/dashboard/prompt-stats", handleDashboardPromptStats())
		mux.HandleFunc("/api/dashboard/github-repos", handleDashboardGitHubRepos(s))
		mux.HandleFunc("/api/dashboard/logs", handleDashboardLogs(s))
		mux.HandleFunc("/api/dashboard/overview", handleDashboardOverview(s))
		mux.HandleFunc("/api/dashboard/notes", handleDashboardNotes(s))

		// File Indexing API endpoints
		mux.HandleFunc("/api/indexing/status", handleIndexingStatus(s))
		mux.HandleFunc("/api/indexing/rescan", handleIndexingRescan(s))
		mux.HandleFunc("/api/indexing/directories", handleIndexingDirectories(s))

		// Mission Control API endpoints (legacy v1)
		mux.HandleFunc("/api/missions", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleListMissions(s)(w, r)
			case http.MethodPost:
				handleCreateMission(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/missions/", handleMissionByID(s))

		// Mission Control API endpoints v2 (enhanced with triggers and queue)
		mux.HandleFunc("/api/missions/v2", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleListMissionsV2(s)(w, r)
			case http.MethodPost:
				handleCreateMissionV2(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc("/api/missions/v2/queue", handleMissionQueue(s))
		mux.HandleFunc("/api/missions/v2/execution", handleMissionsV2ByExecution(s))
		mux.HandleFunc("/api/missions/v2/dependencies", handleMissionDependencies(s))
		mux.HandleFunc("/api/missions/v2/", handleMissionV2ByID(s))
	}

	// ── Integration bots (disabled in egg mode — eggs are headless workers) ──
	if !s.Cfg.EggMode.Enabled {
		// Phase 35.2: Start the Telegram Long Polling loop
		telegram.StartLongPolling(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManager)

		// Discord Bot: listen for messages and relay to the agent
		discord.StartBot(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManager)

		// Email Watcher: poll IMAP for new messages and wake the agent
		tools.StartEmailWatcher(s.Cfg, s.Logger, s.Guardian)

		// Rocket.Chat Bot: listen for messages and relay to the agent
		rocketchat.StartBot(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB)

		// MQTT Client: connect to broker and register bridge
		mqtt.StartClient(s.Cfg, s.Logger)

		// MCP: start external MCP server connections (only when both gates are open)
		if s.Cfg.Agent.AllowMCP && s.Cfg.MCP.Enabled && len(s.Cfg.MCP.Servers) > 0 {
			mcpConfigs := make([]tools.MCPServerConfig, len(s.Cfg.MCP.Servers))
			for i, srv := range s.Cfg.MCP.Servers {
				mcpConfigs[i] = tools.MCPServerConfig{
					Name:    srv.Name,
					Command: srv.Command,
					Args:    srv.Args,
					Env:     srv.Env,
					Enabled: srv.Enabled,
				}
			}
			tools.InitMCPManager(mcpConfigs, s.Logger)
		}

		// Sandbox: start the llm-sandbox MCP server (separate from user MCP servers)
		if s.Cfg.Sandbox.Enabled {
			sandboxCfg := tools.SandboxConfig{
				Enabled:        s.Cfg.Sandbox.Enabled,
				Backend:        s.Cfg.Sandbox.Backend,
				DockerHost:     s.Cfg.Sandbox.DockerHost,
				Image:          s.Cfg.Sandbox.Image,
				AutoInstall:    s.Cfg.Sandbox.AutoInstall,
				PoolSize:       s.Cfg.Sandbox.PoolSize,
				TimeoutSeconds: s.Cfg.Sandbox.TimeoutSeconds,
				NetworkEnabled: s.Cfg.Sandbox.NetworkEnabled,
				KeepAlive:      s.Cfg.Sandbox.KeepAlive,
			}
			tools.InitSandboxManager(sandboxCfg, s.Cfg.Directories.WorkspaceDir, s.Logger)
		}
	} else {
		s.Logger.Info("Egg mode active — integration bots disabled")
	}

	// Webhook receiver endpoint (public — token-authenticated)
	if s.WebhookHandler != nil {
		s.WebhookHandler.SetSSE(sse)
		mux.Handle("/webhook/", s.WebhookHandler)
		s.Logger.Info("Webhook receiver registered at /webhook/{slug}")
	}

	// Phase 34: Notifications endpoints
	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		notes, err := s.ShortTermMem.GetUnreadNotifications()
		if err != nil {
			s.Logger.Error("Failed to fetch unread notifications", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(notes)
	})

	mux.HandleFunc("/notifications/read", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.ShortTermMem.MarkNotificationsRead(); err != nil {
			s.Logger.Error("Failed to mark notifications read", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		all := s.HistoryManager.GetAll()
		var filtered []memory.HistoryMessage
		for _, m := range all {
			if !m.IsInternal {
				filtered = append(filtered, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(filtered)
	})

	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.HistoryManager.Clear(); err != nil {
			s.Logger.Error("Failed to clear chat history", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/admin/stop", handleInterrupt(s))

	// Serve the embedded Web UI at root via html/template for i18n injection
	uiFS, err := fs.Sub(ui.Content, ".")
	if err != nil {
		return fmt.Errorf("failed to create UI filesystem: %w", err)
	}

	// Load i18n translations from embedded i18n.json
	loadI18N(uiFS, s.Logger)

	tmpl, err := template.ParseFS(uiFS, "index.html")
	if err != nil {
		s.Logger.Error("Failed to parse UI template", "error", err)
	}

	// Config page (separate template, guarded by WebConfig.Enabled)
	if s.Cfg.WebConfig.Enabled {
		cfgTmpl, cfgErr := template.ParseFS(uiFS, "config.html")
		if cfgErr != nil {
			s.Logger.Error("Failed to parse config UI template", "error", cfgErr)
		}
		mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
			if cfgTmpl == nil {
				http.Error(w, "Config template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang":     lang,
				"I18N":     getI18NJSON(lang),
				"I18NMeta": getI18NMetaJSON(),
			}
			if err := cfgTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute config template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		// Serve the help texts JSON
		mux.HandleFunc("/config_help.json", func(w http.ResponseWriter, r *http.Request) {
			helpData, err := fs.ReadFile(uiFS, "config_help.json")
			if err != nil {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(helpData)
		})
		s.Logger.Info("Config UI enabled at /config")

		// Dashboard page (separate template, guarded by WebConfig.Enabled)
		dashTmpl, dashErr := template.ParseFS(uiFS, "dashboard.html")
		if dashErr != nil {
			s.Logger.Error("Failed to parse dashboard UI template", "error", dashErr)
		}
		mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			if dashTmpl == nil {
				http.Error(w, "Dashboard template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := dashTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute dashboard template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Dashboard UI enabled at /dashboard")

		// Mission Control page (legacy v1)
		mux.HandleFunc("/missions", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/missions/v2", http.StatusMovedPermanently)
		})
		s.Logger.Info("Mission Control UI /missions redirects to /missions/v2")

		// Mission Control V2 page (enhanced with triggers)
		missionV2Tmpl, missionV2Err := template.ParseFS(uiFS, "missions_v2.html")
		if missionV2Err != nil {
			s.Logger.Error("Failed to parse mission V2 UI template", "error", missionV2Err)
		}
		mux.HandleFunc("/missions/v2", func(w http.ResponseWriter, r *http.Request) {
			if missionV2Tmpl == nil {
				http.Error(w, "Mission V2 template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := missionV2Tmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute mission V2 template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Mission Control V2 UI enabled at /missions/v2")

		// ── Invasion Control API (handlers guard themselves with s.InvasionDB == nil check) ──
		mux.HandleFunc("/api/invasion/nests", handleInvasionNests(s))
		mux.HandleFunc("/api/invasion/eggs", handleInvasionEggs(s))
		mux.HandleFunc("/api/invasion/eggs/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/invasion/eggs/")
			if strings.HasSuffix(path, "/toggle") {
				handleInvasionEggToggle(s)(w, r)
			} else {
				handleInvasionEgg(s)(w, r)
			}
		})

		// ── Egg deployment lifecycle routes ──
		mux.HandleFunc("/api/invasion/nests/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/invasion/nests/")
			if strings.HasSuffix(path, "/toggle") {
				handleInvasionNestToggle(s)(w, r)
			} else if strings.HasSuffix(path, "/validate") {
				handleInvasionNestValidate(s)(w, r)
			} else if strings.HasSuffix(path, "/hatch") {
				handleInvasionNestHatch(s)(w, r)
			} else if strings.HasSuffix(path, "/stop") {
				handleInvasionNestStop(s)(w, r)
			} else if strings.HasSuffix(path, "/status") {
				handleInvasionNestHatchStatus(s)(w, r)
			} else if strings.HasSuffix(path, "/send-secret") {
				handleInvasionNestSendSecret(s)(w, r)
			} else if strings.HasSuffix(path, "/send-task") {
				handleInvasionNestSendTask(s)(w, r)
			} else {
				handleInvasionNest(s)(w, r)
			}
		})

		// ── WebSocket endpoint for egg connections ──
		mux.HandleFunc("/api/invasion/ws", handleInvasionWebSocket(s))

		// Start heartbeat monitor
		s.EggHub.StartHeartbeatMonitor(30*time.Second, 90*time.Second, func(nestID, eggID string) {
			s.Logger.Warn("Egg heartbeat stale, marking as failed", "nest_id", nestID, "egg_id", eggID)
			_ = invasion.UpdateNestHatchStatus(s.InvasionDB, nestID, "failed", "heartbeat timeout")
		})

		s.Logger.Info("Invasion Control API registered at /api/invasion/...")
	}

	// Invasion Control UI page (always registered — same pattern as /setup)
	invasionTmpl, invasionErr := template.ParseFS(uiFS, "invasion_control.html")
	if invasionErr != nil {
		s.Logger.Error("Failed to parse invasion control UI template", "error", invasionErr)
	}
	mux.HandleFunc("/invasion", func(w http.ResponseWriter, r *http.Request) {
		if invasionTmpl == nil {
			http.Error(w, "Invasion Control template error", http.StatusInternalServerError)
			return
		}
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		data := map[string]interface{}{
			"Lang": lang,
			"I18N": getI18NJSON(lang),
		}
		if err := invasionTmpl.Execute(w, data); err != nil {
			s.Logger.Error("Failed to execute invasion control template", "error", err)
			http.Error(w, "Template render error", http.StatusInternalServerError)
		}
	})
	s.Logger.Info("Invasion Control UI registered at /invasion")

	// Quick Setup wizard page (always available — parsed outside WebConfig guard)
	setupTmpl, setupErr := template.ParseFS(uiFS, "setup.html")
	if setupErr != nil {
		s.Logger.Error("Failed to parse setup UI template", "error", setupErr)
	}
	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		if setupTmpl == nil {
			http.Error(w, "Setup template error", http.StatusInternalServerError)
			return
		}
		lang := normalizeLang(s.Cfg.Server.UILanguage)
		data := map[string]interface{}{
			"Lang": lang,
			"I18N": getI18NJSON(lang),
		}
		if err := setupTmpl.Execute(w, data); err != nil {
			s.Logger.Error("Failed to execute setup template", "error", err)
			http.Error(w, "Template render error", http.StatusInternalServerError)
		}
	})

	// Auth login / logout pages (registered here so they can use uiFS)
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleAuthLoginPage(s, uiFS)(w, r)
		case http.MethodPost:
			handleAuthLogin(s)(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/auth/logout", handleAuthLogout(s))

	staticHandler := http.FileServer(http.FS(uiFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Redirect to setup wizard if LLM is not configured (first start)
			s.CfgMu.RLock()
			showSetup := needsSetup(s.Cfg)
			s.CfgMu.RUnlock()

			if showSetup && r.URL.Query().Get("skip_setup") != "1" {
				http.Redirect(w, r, "/setup", http.StatusTemporaryRedirect)
				return
			}

			if tmpl != nil {
				lang := normalizeLang(s.Cfg.Server.UILanguage)
				data := map[string]interface{}{
					"Lang":               lang,
					"I18N":               getI18NJSON(lang),
					"ShowToolResults":    s.Cfg.Agent.ShowToolResults,
					"DebugMode":          agent.GetDebugMode(),
					"PersonalityEnabled": s.Cfg.Agent.PersonalityEngine,
				}
				if err := tmpl.Execute(w, data); err != nil {
					s.Logger.Error("Failed to execute UI template", "error", err)
					http.Error(w, "Template render error", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Template error", http.StatusInternalServerError)
			}
			return
		}
		// Serve static assets from embedded UI FS (logos, etc.)
		staticHandler.ServeHTTP(w, r)
	})

	// Serve static files securely from the workspace directory
	fsHandler := http.StripPrefix("/files/", http.FileServer(neuteredFileSystem{http.Dir(s.Cfg.Directories.WorkspaceDir)}))
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fsHandler.ServeHTTP(w, r)
	})

	// Phase X: Dedicated TTS Server for Chromecast
	// Declared outside the if-block so the graceful shutdown goroutine can close it.
	var ttsServer *http.Server
	if s.Cfg.Chromecast.Enabled && s.Cfg.Chromecast.TTSPort > 0 {
		ttsDir := tools.TTSAudioDir(s.Cfg.Directories.DataDir)
		ttsMux := http.NewServeMux()
		ttsFsHandler := http.StripPrefix("/tts/", http.FileServer(http.Dir(ttsDir)))
		ttsMux.HandleFunc("/tts/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			ttsFsHandler.ServeHTTP(w, r)
		})

		ttsServer = &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", s.Cfg.Chromecast.TTSPort),
			Handler: ttsMux,
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.Logger.Error("[TTS Server] Goroutine panic recovered", "error", r)
				}
			}()
			s.Logger.Info("Starting Dedicated TTS Server", "host", "0.0.0.0", "port", s.Cfg.Chromecast.TTSPort)
			if err := ttsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.Logger.Warn("Dedicated TTS Server failed (Chromecast audio will not be available)", "error", err)
			}
		}()
	}

	addr := fmt.Sprintf("%s:%d", s.Cfg.Server.Host, s.Cfg.Server.Port)
	s.Logger.Info("Starting server", "host", s.Cfg.Server.Host, "port", s.Cfg.Server.Port)

	// Start Phase 1 TCP Bridge
	bridgeAddr := s.Cfg.Server.BridgeAddress
	if bridgeAddr == "" {
		bridgeAddr = "localhost:8089"
	}
	go s.StartTCPBridge(bridgeAddr)

	server := &http.Server{
		Addr:         addr,
		Handler:      authMiddleware(s, mux), // auth check; respects s.Cfg.Auth.Enabled dynamically
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // generous for streaming responses
		IdleTimeout:  2 * time.Minute,
	}

	// Graceful shutdown goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.Logger.Error("[Shutdown] Goroutine panic recovered", "error", r)
			}
		}()
		<-shutdownCh
		s.Logger.Info("Initiating graceful HTTP server shutdown...")
		// Shut down MCP servers
		tools.ShutdownMCPManager()
		// Shut down Sandbox
		tools.ShutdownSandboxManager()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Shut down TTS server (releases port so next startup doesn't get EADDRINUSE)
		if ttsServer != nil {
			if err := ttsServer.Shutdown(ctx); err != nil {
				s.Logger.Warn("TTS Server shutdown error", "error", err)
			}
		}
		if err := server.Shutdown(ctx); err != nil {
			s.Logger.Error("HTTP server shutdown error", "error", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	s.Logger.Info("Server stopped gracefully")
	return nil
}
