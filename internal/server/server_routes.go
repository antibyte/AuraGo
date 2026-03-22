package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/discord"
	"aurago/internal/invasion"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/rocketchat"
	"aurago/internal/telegram"
	"aurago/internal/telnyx"
	"aurago/internal/tools"
	"aurago/ui"
)

func (s *Server) run(shutdownCh chan struct{}) error {
	mux := http.NewServeMux()
	sse := NewSSEBroadcaster()

	// Create a context that cancels on shutdown
	serverCtx, serverCancel := context.WithCancel(context.Background())
	go func() {
		<-shutdownCh
		serverCancel()
	}()

	// Push system metrics to all SSE clients every 10 seconds so the dashboard
	// does not need to poll /api/dashboard/system independently.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				raw := tools.GetSystemMetrics("")
				var metricsResult struct {
					Status string              `json:"status"`
					Data   tools.SystemMetrics `json:"data"`
				}
				if err := json.Unmarshal([]byte(raw), &metricsResult); err == nil {
					payload := map[string]interface{}{
						"cpu":            metricsResult.Data.CPU,
						"memory":         metricsResult.Data.Memory,
						"disk":           metricsResult.Data.Disk,
						"network":        metricsResult.Data.Network,
						"sse_clients":    sse.ClientCount(),
						"uptime_seconds": int(time.Since(s.StartedAt).Seconds()),
					}
					sse.BroadcastType(EventSystemMetrics, payload)
				}
			case <-serverCtx.Done():
				return
			}
		}
	}()

	// Phase 34: Start the background daily reflection loop
	tools.StartDailyReflectionLoop(serverCtx, s.Cfg, s.Logger, s.LLMClient, s.HistoryManager, s.ShortTermMem)

	// Phase 68: Start the daily maintenance loop
	manifest := tools.NewManifest(s.Cfg.Directories.ToolsDir)
	agent.StartMaintenanceLoop(serverCtx, s.Cfg, s.Logger, s.LLMClient, s.Vault, s.Registry, manifest, s.CronManager, s.LongTermMem, s.ShortTermMem, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2)

	s.CoAgentRegistry.StartCleanupLoop()

	// Start OAuth2 token refresh loop (auto-refreshes before expiry)
	startOAuthRefreshLoop(s, serverCtx)

	// Health check — no auth required, used by Docker HEALTHCHECK and monitoring.
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/v1/chat/completions", handleChatCompletions(s, sse))
	mux.HandleFunc("/api/memory/archive", handleArchiveMemory(s))
	mux.HandleFunc("/api/upload", handleUpload(s))
	mux.HandleFunc("/api/upload-voice", handleVoiceUpload(s))
	mux.HandleFunc("/api/budget", handleBudgetStatus(s))
	mux.HandleFunc("/api/credits", handleOpenRouterCredits(s))

	// MCP Server endpoint (handles its own auth via Bearer token / session)
	mux.HandleFunc("/mcp", handleMCPEndpoint(s))

	// n8n Integration endpoints
	mux.HandleFunc("/api/n8n/status", handleN8nStatus(s))
	mux.HandleFunc("/api/n8n/chat", handleN8nChat(s))
	mux.HandleFunc("/api/n8n/tools", handleN8nToolsList(s))
	mux.HandleFunc("/api/n8n/tools/", handleN8nToolExecute(s))
	mux.HandleFunc("/api/n8n/memory/search", handleN8nMemorySearch(s))
	mux.HandleFunc("/api/n8n/memory/store", handleN8nMemoryStore(s))
	mux.HandleFunc("/api/n8n/missions", handleN8nMissionCreate(s))

	// Quick Setup wizard endpoints (always available — needed before config is complete)
	mux.HandleFunc("/api/setup/status", handleSetupStatus(s))
	mux.HandleFunc("/api/setup", handleSetupSave(s))

	// i18n translations endpoint (always available — used by setup wizard pre-auth)
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		lang := normalizeLang(r.URL.Query().Get("lang"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, string(getI18NJSON(lang)))
	})

	// OpenRouter model browser (always available — needed in both setup wizard and config UI)
	mux.HandleFunc("/api/openrouter/models", handleOpenRouterModels(s))

	// Auth endpoints — always reachable (whitelisted in authMiddleware)
	mux.HandleFunc("/api/auth/status", handleAuthStatus(s))
	mux.HandleFunc("/api/security/status", handleSecurityStatus(s))
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
		// Security audit — list hints and apply auto-fixable hardening measures
		mux.HandleFunc("/api/security/hints", handleSecurityHints(s))
		mux.HandleFunc("/api/security/harden", handleSecurityHarden(s))

		mux.HandleFunc("/api/providers", handleProviders(s))
		mux.HandleFunc("/api/providers/pricing", handleProviderPricing(s))
		mux.HandleFunc("/api/email-accounts", handleEmailAccounts(s))
		mux.HandleFunc("/api/mcp-servers", handleMCPServers(s))
		mux.HandleFunc("/api/outgoing-webhooks", handleOutgoingWebhooks(s))
		mux.HandleFunc("/api/sandbox/status", handleSandboxStatus(s))
		mux.HandleFunc("/api/sandbox/shell-status", handleShellSandboxStatus(s))

		// MCP Server config API
		mux.HandleFunc("/api/mcp-server/tools", handleMCPServerTools(s))
		mux.HandleFunc("/api/mcp-server/token", handleMCPServerToken(s))

		// n8n Integration config API
		mux.HandleFunc("/api/n8n/token", handleN8nToken(s))

		// OAuth2 Authorization Code flow endpoints
		mux.HandleFunc("/api/oauth/start", handleOAuthStart(s))
		mux.HandleFunc("/api/oauth/callback", handleOAuthCallback(s))
		mux.HandleFunc("/api/oauth/manual", handleOAuthManual(s))
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

		// Chromecast mDNS discovery
		mux.HandleFunc("/api/chromecast/discover", handleChromecastDiscover(s))

		// Runtime environment detection (Docker, socket, broadcast, firewall)
		mux.HandleFunc("/api/runtime", handleRuntime(s))

		// Homepage tool endpoints
		mux.HandleFunc("/api/homepage/status", handleHomepageStatus(s))
		mux.HandleFunc("/api/homepage/test-connection", handleHomepageTestConnection(s))

		// Cloudflare Tunnel endpoints
		mux.HandleFunc("/api/tunnel/status", handleTunnelStatus(s))
		mux.HandleFunc("/api/tunnel/quick", handleTunnelQuick(s))

		// Netlify integration endpoints
		mux.HandleFunc("/api/netlify/status", handleNetlifyStatus(s))
		mux.HandleFunc("/api/netlify/test-connection", handleNetlifyTestConnection(s))

		// Google Workspace integration endpoints
		mux.HandleFunc("/api/google-workspace/test", handleGoogleWorkspaceTest(s))

		// OneDrive integration endpoints (Device Code Flow)
		mux.HandleFunc("/api/onedrive/auth/start", handleOneDriveAuthStart(s))
		mux.HandleFunc("/api/onedrive/auth/poll", handleOneDriveAuthPoll(s))
		mux.HandleFunc("/api/onedrive/auth/status", handleOneDriveAuthStatus(s))
		mux.HandleFunc("/api/onedrive/auth/revoke", handleOneDriveAuthRevoke(s))
		mux.HandleFunc("/api/onedrive/test", handleOneDriveTest(s))

		// Image Generation endpoints
		mux.HandleFunc("/api/image-generation/test", handleImageGenerationTest(s))
		mux.HandleFunc("/api/image-gallery", handleImageGalleryList(s))
		mux.HandleFunc("/api/image-gallery/", handleImageGalleryByID(s))

		// Media Registry endpoints (audio, documents, and agent-sent media)
		mux.HandleFunc("/api/media", handleMediaList(s))
		mux.HandleFunc("/api/media/", handleMediaByID(s))

		// AdGuard Home integration endpoints
		mux.HandleFunc("/api/adguard/status", handleAdGuardStatus(s))
		mux.HandleFunc("/api/adguard/test", handleAdGuardTest(s))

		// Fritz!Box integration endpoints
		mux.HandleFunc("/api/fritzbox/status", handleFritzBoxStatus(s))
		mux.HandleFunc("/api/fritzbox/test", handleFritzBoxTest(s))

		// Document Creator endpoints
		mux.HandleFunc("/api/document-creator/test", handleGotenbergTest(s))

		// A2A Protocol endpoints (config UI management)
		mux.HandleFunc("/api/a2a/status", handleA2AStatus(s))
		mux.HandleFunc("/api/a2a/remote-agents", handleA2ARemoteAgents(s))
		mux.HandleFunc("/api/a2a/card", handleA2ACard(s))
		mux.HandleFunc("/api/a2a/test", handleA2ATest(s))
		mux.HandleFunc("/api/a2a/remote-agents/test", handleA2ARemoteAgentTest(s))

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
		mux.HandleFunc("/api/dashboard/emotion-history", handleDashboardEmotionHistory(s))
		mux.HandleFunc("/api/dashboard/memory", handleDashboardMemory(s))
		mux.HandleFunc("/api/dashboard/core-memory", handleDashboardCoreMemory(s))
		mux.HandleFunc("/api/dashboard/core-memory/mutate", handleDashboardCoreMemoryMutate(s, sse))
		mux.HandleFunc("/api/dashboard/profile", handleDashboardProfile(s))
		mux.HandleFunc("/api/dashboard/profile/entry", handleDashboardProfileEntry(s))
		mux.HandleFunc("/api/dashboard/activity", handleDashboardActivity(s))
		mux.HandleFunc("/api/cron", handleCronAPI(s))
		mux.HandleFunc("/api/dashboard/prompt-stats", handleDashboardPromptStats())
		mux.HandleFunc("/api/dashboard/tool-stats", handleDashboardToolStats(s.Cfg))
		mux.HandleFunc("/api/dashboard/github-repos", handleDashboardGitHubRepos(s))
		mux.HandleFunc("/api/github/repos", handleGitHubReposForUI(s))
		mux.HandleFunc("/api/dashboard/logs", handleDashboardLogs(s))
		mux.HandleFunc("/api/dashboard/overview", handleDashboardOverview(s))
		mux.HandleFunc("/api/dashboard/notes", handleDashboardNotes(s))
		mux.HandleFunc("/api/dashboard/journal", handleDashboardJournal(s))
		mux.HandleFunc("/api/dashboard/journal/summaries", handleDashboardJournalSummary(s))
		mux.HandleFunc("/api/dashboard/journal/stats", handleDashboardJournalStats(s))
		mux.HandleFunc("/api/dashboard/guardian", handleDashboardGuardian(s))
		mux.HandleFunc("/api/dashboard/errors", handleDashboardErrors(s))

		// System endpoints
		mux.HandleFunc("/api/system/os", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"os": runtime.GOOS})
		})

		// File Indexing API endpoints
		mux.HandleFunc("/api/indexing/status", handleIndexingStatus(s))
		mux.HandleFunc("/api/indexing/rescan", handleIndexingRescan(s))
		mux.HandleFunc("/api/indexing/directories", handleIndexingDirectories(s))

		// Mission Control API endpoints (enhanced with triggers and queue)
		mux.HandleFunc("/api/missions", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleListMissionsV2(s)(w, r)
			case http.MethodPost:
				handleCreateMissionV2(s)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})

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
		telegram.StartLongPolling(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2)

		// Discord Bot: listen for messages and relay to the agent
		discord.StartBot(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2)

		// Email Watcher: poll IMAP for new messages and wake the agent
		tools.StartEmailWatcher(s.Cfg, s.Logger, s.Guardian, s.LLMGuardian)

		// Rocket.Chat Bot: listen for messages and relay to the agent
		rocketchat.StartBot(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB)

		// MQTT Client: connect to broker and register bridge
		mqtt.StartClient(s.Cfg, s.Logger)

		// Telnyx: register webhook endpoint for incoming SMS/calls
		if s.Cfg.Telnyx.Enabled {
			webhookPath := s.Cfg.Telnyx.WebhookPath
			if webhookPath == "" {
				webhookPath = "/api/telnyx/webhook"
			}
			telnyxHandler := telnyx.NewWebhookHandler(s.Cfg, s.Logger, func(from, text string, mediaURLs []string) {
				// Relay incoming SMS to agent via loopback
				msg := telnyx.FormatSMSForAgent(from, text, mediaURLs)
				s.Logger.Info("Telnyx SMS relayed to agent", "from", from)
				runCfg := agent.RunConfig{
					Config:             s.Cfg,
					Logger:             s.Logger,
					LLMClient:          s.LLMClient,
					ShortTermMem:       s.ShortTermMem,
					HistoryManager:     s.HistoryManager,
					LongTermMem:        s.LongTermMem,
					KG:                 s.KG,
					InventoryDB:        s.InventoryDB,
					InvasionDB:         s.InvasionDB,
					CheatsheetDB:       s.CheatsheetDB,
					ImageGalleryDB:     s.ImageGalleryDB,
					MediaRegistryDB:    s.MediaRegistryDB,
					HomepageRegistryDB: s.HomepageRegistryDB,
					ContactsDB:         s.ContactsDB,
					RemoteHub:          s.RemoteHub,
					Vault:              s.Vault,
					Registry:           s.Registry,
					CronManager:        s.CronManager,
					MissionManagerV2:   s.MissionManagerV2,
					CoAgentRegistry:    s.CoAgentRegistry,
					BudgetTracker:      s.BudgetTracker,
					LLMGuardian:        s.LLMGuardian,
					SessionID:          "default",
					IsMaintenance:      tools.IsBusy(),
					MessageSource:      "sms",
				}
				go agent.Loopback(runCfg, msg, telnyx.NewSMSBroker(s.Cfg, from, s.Logger))
			}, nil)
			mux.HandleFunc(webhookPath, telnyxHandler.HandleWebhook)
			s.Logger.Info("Telnyx webhook registered", "path", webhookPath)
		}

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

	// A2A Protocol routes (shared port mode — protocol endpoints on main mux)
	if s.A2AServer != nil && s.Cfg.A2A.Server.Port == 0 {
		s.A2AServer.RegisterRoutes(mux)
		s.Logger.Info("A2A protocol routes registered on main server (shared port)")
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

	// Web Push (PWA) API
	mux.HandleFunc("/api/push/vapid-pubkey", handlePushVAPIDPublicKey(s))
	mux.HandleFunc("/api/push/subscribe", handlePushSubscribe(s))
	mux.HandleFunc("/api/push/unsubscribe", handlePushUnsubscribe(s))
	mux.HandleFunc("/api/push/status", handlePushStatus(s))

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

		// Cheat Sheet Editor page
		cheatsheetTmpl, cheatsheetErr := template.ParseFS(uiFS, "cheatsheets.html")
		if cheatsheetErr != nil {
			s.Logger.Error("Failed to parse cheatsheet UI template", "error", cheatsheetErr)
		}
		mux.HandleFunc("/cheatsheets", func(w http.ResponseWriter, r *http.Request) {
			if cheatsheetTmpl == nil {
				http.Error(w, "Cheatsheet template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := cheatsheetTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute cheatsheet template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Cheat Sheet Editor UI enabled at /cheatsheets")

		// ── Media View Page (replaces Gallery) ──
		mediaTmpl, mediaTmplErr := template.ParseFS(uiFS, "media.html")
		if mediaTmplErr != nil {
			s.Logger.Error("Failed to parse media UI template", "error", mediaTmplErr)
		}
		serveMediaPage := func(w http.ResponseWriter, r *http.Request) {
			if mediaTmpl == nil {
				http.Error(w, "Media template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := mediaTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute media template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		}
		mux.HandleFunc("/media", serveMediaPage)
		// Legacy /gallery redirect for backward compatibility
		mux.HandleFunc("/gallery", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/media", http.StatusMovedPermanently)
		})
		s.Logger.Info("Media View UI enabled at /media (/gallery redirects here)")

		// ── Knowledge Center Page ──
		knowledgeTmpl, knowledgeTmplErr := template.ParseFS(uiFS, "knowledge.html")
		if knowledgeTmplErr != nil {
			s.Logger.Error("Failed to parse knowledge UI template", "error", knowledgeTmplErr)
		}
		mux.HandleFunc("/knowledge", func(w http.ResponseWriter, r *http.Request) {
			if knowledgeTmpl == nil {
				http.Error(w, "Knowledge template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := knowledgeTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute knowledge template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Knowledge Center UI enabled at /knowledge")

		// ── Containers Page ──
		containersTmpl, containersTmplErr := template.ParseFS(uiFS, "containers.html")
		if containersTmplErr != nil {
			s.Logger.Error("Failed to parse containers UI template", "error", containersTmplErr)
		}
		mux.HandleFunc("/containers", func(w http.ResponseWriter, r *http.Request) {
			if containersTmpl == nil {
				http.Error(w, "Containers template error", http.StatusInternalServerError)
				return
			}
			lang := normalizeLang(s.Cfg.Server.UILanguage)
			data := map[string]interface{}{
				"Lang": lang,
				"I18N": getI18NJSON(lang),
			}
			if err := containersTmpl.Execute(w, data); err != nil {
				s.Logger.Error("Failed to execute containers template", "error", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
			}
		})
		s.Logger.Info("Containers UI enabled at /containers")

		// ── Containers API ──
		mux.HandleFunc("/api/containers", handleContainersList(s))
		mux.HandleFunc("/api/containers/", handleContainerAction(s))

		// ── Cheat Sheets API ──
		mux.HandleFunc("/api/cheatsheets", handleCheatSheets(s))
		mux.HandleFunc("/api/cheatsheets/", handleCheatSheetByID(s))

		// ── Contacts (Address Book) API ──
		mux.HandleFunc("/api/contacts", handleContacts(s))
		mux.HandleFunc("/api/contacts/", handleContactByID(s))

		// ── Knowledge Files API ──
		mux.HandleFunc("/api/knowledge", handleKnowledgeFiles(s))
		mux.HandleFunc("/api/knowledge/upload", handleKnowledgeUpload(s))
		mux.HandleFunc("/api/knowledge/", handleKnowledgeFile(s))

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

	// ── Remote Control API (handlers guard themselves with s.RemoteHub == nil check) ──
	if s.RemoteHub != nil {
		mux.HandleFunc("/api/remote/devices", handleRemoteDevices(s))
		mux.HandleFunc("/api/remote/devices/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/remote/devices/")
			if strings.HasSuffix(path, "/approve") {
				handleRemoteDeviceApprove(s)(w, r)
			} else if strings.HasSuffix(path, "/reject") {
				handleRemoteDeviceReject(s)(w, r)
			} else if strings.HasSuffix(path, "/revoke") {
				handleRemoteDeviceRevoke(s)(w, r)
			} else {
				handleRemoteDevice(s)(w, r)
			}
		})
		mux.HandleFunc("/api/remote/enroll", handleRemoteEnrollmentCreate(s))
		mux.HandleFunc("/api/remote/audit", handleRemoteAuditLog(s))
		mux.HandleFunc("/api/remote/platforms", handleRemotePlatforms(s))
		mux.HandleFunc("/api/remote/download/", handleRemoteDownload(s))
		mux.HandleFunc("/api/remote/ws", handleRemoteWebSocket(s))
		s.Logger.Info("Remote Control API registered at /api/remote/...")
	}

	// ── Security Proxy API ──
	mux.HandleFunc("/api/proxy/status", handleProxyStatus(s))
	mux.HandleFunc("/api/proxy/start", handleProxyStart(s))
	mux.HandleFunc("/api/proxy/stop", handleProxyStop(s))
	mux.HandleFunc("/api/proxy/destroy", handleProxyDestroy(s))
	mux.HandleFunc("/api/proxy/reload", handleProxyReload(s))
	mux.HandleFunc("/api/proxy/logs", handleProxyLogs(s))
	s.Logger.Info("Security Proxy API registered at /api/proxy/...")

	// ── tsnet API (Tailscale embedded node) ──
	mux.HandleFunc("/api/tsnet/status", handleTsNetStatus(s))
	mux.HandleFunc("/api/tsnet/start", handleTsNetStart(s))
	mux.HandleFunc("/api/tsnet/stop", handleTsNetStop(s))
	s.Logger.Info("tsnet API registered at /api/tsnet/...")

	// ── Certificate Management API ──
	mux.HandleFunc("/api/cert/status", handleCertStatus(s))
	mux.HandleFunc("/api/cert/regenerate", handleCertRegenerate(s))
	mux.HandleFunc("/api/cert/upload", handleCertUpload(s))
	s.Logger.Info("Certificate API registered at /api/cert/...")

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

	// Serve generated documents from the document_creator output directory
	docDir := s.Cfg.Tools.DocumentCreator.OutputDir
	if docDir == "" {
		docDir = filepath.Join(s.Cfg.Directories.DataDir, "documents")
	}
	os.MkdirAll(docDir, 0755) // ensure directory exists
	docHandler := http.StripPrefix("/files/documents/", http.FileServer(neuteredFileSystem{http.Dir(docDir)}))
	mux.HandleFunc("/files/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		filename := filepath.Base(r.URL.Path)
		// Allow inline display when ?inline=1 is set (e.g. PDF preview)
		if r.URL.Query().Get("inline") == "1" {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
		} else {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		}
		docHandler.ServeHTTP(w, r)
	})

	// Serve agent audio files from data/audio directory
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	os.MkdirAll(audioDir, 0755) // ensure directory exists
	audioHandler := http.StripPrefix("/files/audio/", http.FileServer(neuteredFileSystem{http.Dir(audioDir)}))
	mux.HandleFunc("/files/audio/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		audioHandler.ServeHTTP(w, r)
	})

	// Serve generated images from data directory
	genImgDir := filepath.Join(s.Cfg.Directories.DataDir, "generated_images")
	genImgHandler := http.StripPrefix("/files/generated_images/", http.FileServer(neuteredFileSystem{http.Dir(genImgDir)}))
	mux.HandleFunc("/files/generated_images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		genImgHandler.ServeHTTP(w, r)
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

		// Bind TTS to the configured server host so it doesn’t accidentally
		// listen on all interfaces when the server is internet-facing.
		// Chromecasts reach it on the LAN IP the operator put in server.host.
		ttsHost := s.Cfg.Server.Host
		if ttsHost == "" {
			ttsHost = "0.0.0.0"
		}
		ttsServer = &http.Server{
			Addr:    fmt.Sprintf("%s:%d", ttsHost, s.Cfg.Chromecast.TTSPort),
			Handler: ttsMux,
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.Logger.Error("[TTS Server] Goroutine panic recovered", "error", r)
				}
			}()
			s.Logger.Info("Starting Dedicated TTS Server", "host", ttsHost, "port", s.Cfg.Chromecast.TTSPort)
			if err := ttsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.Logger.Warn("Dedicated TTS Server failed (Chromecast audio will not be available)", "error", err)
			}
		}()
	}

	// Start Phase 1 TCP Bridge (Lifeboat IPC — lifeboat dials this on port 8089)
	// This port is intentionally separate from maintenance.lifeboat_port (8091),
	// which is the port lifeboat itself listens on.
	go s.StartTCPBridge("localhost:8089")

	// Start tsnet embedded Tailscale node (serves same handler over Tailscale network)
	if s.Cfg.Tailscale.TsNet.Enabled && s.TsNetManager != nil {
		tsHandler := accessLogMiddleware(s.Logger, securityHeadersMiddleware(authMiddleware(s, mux), true, false))
		s.tsNetHandler = tsHandler // store for runtime restart via /api/tsnet/start
		go func() {
			if err := s.TsNetManager.Start(tsHandler); err != nil {
				s.Logger.Error("Failed to start tsnet node", "error", err)
			}
		}()
	}

	// Determine server mode: HTTPS auto, HTTPS custom, HTTPS self-signed, or HTTP
	tlsCfg := NewTLSConfigFromConfig(s.Cfg, s.Cfg.Directories.DataDir)
	tlsCfg.BehindProxy = s.Cfg.Server.HTTPS.BehindProxy

	if tlsCfg.IsTLSActive() {
		return s.runHTTPS(mux, ttsServer, tlsCfg, shutdownCh)
	}

	return s.runHTTP(mux, ttsServer, shutdownCh)
}
