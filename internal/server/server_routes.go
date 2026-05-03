package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/discord"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/planner"
	"aurago/internal/rocketchat"
	"aurago/internal/telegram"
	"aurago/internal/telnyx"
	"aurago/internal/tools"
	"aurago/internal/warnings"
)

func (s *Server) run(shutdownCh chan struct{}) error {
	mux := http.NewServeMux()
	sse := NewSSEBroadcaster()
	s.SSE = sse // expose broadcaster for use by handlers and callbacks

	// Wire warnings registry to broadcast new warnings via SSE.
	if s.WarningsRegistry != nil {
		s.WarningsRegistry.OnNewWarning = func(w warnings.Warning) {
			total, unack := s.WarningsRegistry.Count()
			sse.BroadcastType(EventSystemWarning, map[string]interface{}{
				"warning":        w,
				"total":          total,
				"unacknowledged": unack,
			})
		}
	}

	// Create a context that cancels on shutdown
	serverCtx, serverCancel := context.WithCancel(context.Background())
	go func() {
		<-shutdownCh
		serverCancel()
	}()
	go func() {
		<-shutdownCh
		s.DesktopMu.Lock()
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
			s.DesktopService = nil
		}
		s.DesktopMu.Unlock()
	}()

	// Initialize Daemon Supervisor (long-running daemon skills)
	if s.Cfg.Tools.DaemonSkills.Enabled {
		dsCfg := tools.DaemonSupervisorConfig{
			Enabled:              true,
			MaxConcurrentDaemons: s.Cfg.Tools.DaemonSkills.MaxConcurrentDaemons,
			WakeUpGate: tools.WakeUpGateConfig{
				GlobalEnabled:       true,
				GlobalRateLimitSecs: s.Cfg.Tools.DaemonSkills.GlobalRateLimitSecs,
				MaxBudgetPerHourUSD: s.Cfg.Tools.DaemonSkills.MaxBudgetPerHourUSD,
				MaxWakeUpsPerHour:   s.Cfg.Tools.DaemonSkills.MaxWakeUpsPerHour,
			},
			WorkspaceDir:       s.Cfg.Directories.WorkspaceDir,
			SkillsDir:          s.Cfg.Directories.SkillsDir,
			BridgeEnabled:      s.Cfg.Tools.PythonToolBridge.Enabled,
			BridgeURL:          InternalAPIURL(s.Cfg) + "/api/internal/tool-bridge",
			BridgeToken:        s.internalToken,
			BridgeAllowedTools: s.Cfg.Tools.PythonToolBridge.AllowedTools,
		}
		s.DaemonSupervisor = tools.NewDaemonSupervisor(
			dsCfg,
			s.BudgetTracker,
			s.Registry,
			s.BackgroundTasks,
			newDaemonSSEAdapter(sse),
			s.Logger,
		)
		s.DaemonSupervisor.SetMissionManager(s.MissionManagerV2)
		s.DaemonSupervisor.SetCheatsheetDB(s.CheatsheetDB)
		if err := s.DaemonSupervisor.Start(); err != nil {
			s.Logger.Error("Failed to start daemon supervisor", "error", err)
		} else {
			s.Logger.Info("Daemon supervisor started",
				"max_concurrent", dsCfg.MaxConcurrentDaemons,
				"active", s.DaemonSupervisor.RunnerCount(),
			)
		}
		// Stop supervisor on shutdown
		go func() {
			<-shutdownCh
			s.DaemonSupervisor.Stop()
		}()
	}

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

	// Push container state changes via SSE so the containers page does not need
	// to poll /api/containers every 5 seconds. Only broadcasts on hash change.
	go func() {
		s.CfgMu.RLock()
		dockerEnabled := s.Cfg.Docker.Enabled
		s.CfgMu.RUnlock()
		if !dockerEnabled {
			return
		}
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var lastRaw string
		for {
			select {
			case <-ticker.C:
				s.CfgMu.RLock()
				dockerCfg := tools.DockerConfig{Host: s.Cfg.Docker.Host}
				s.CfgMu.RUnlock()
				raw := tools.DockerListContainers(dockerCfg, true)
				if raw == lastRaw {
					continue
				}
				lastRaw = raw
				var parsed struct {
					Status     string        `json:"status"`
					Containers []interface{} `json:"containers"`
				}
				if err := json.Unmarshal([]byte(raw), &parsed); err == nil && parsed.Status == "ok" {
					sse.BroadcastType(EventContainerUpdate, parsed.Containers)
				}
			case <-serverCtx.Done():
				return
			}
		}
	}()

	// Push personality state changes via SSE so the chat mood widget does not
	// need to poll /api/personality/state every 30 seconds.
	go func() {
		s.CfgMu.RLock()
		personalityEnabled := s.Cfg.Personality.Engine
		s.CfgMu.RUnlock()
		if !personalityEnabled {
			return
		}
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		var lastStateKey string
		for {
			select {
			case <-ticker.C:
				payload := s.buildPersonalityStatePayload()
				mood, _ := payload["mood"].(string)
				trigger, _ := payload["trigger"].(string)
				emotion, _ := payload["current_emotion"].(string)
				key := mood + "|" + trigger + "|" + emotion
				if key == lastStateKey {
					continue
				}
				lastStateKey = key
				sse.BroadcastType(EventPersonalityUpdate, payload)
			case <-serverCtx.Done():
				return
			}
		}
	}()

	// Send initial personality state immediately on startup so the mood widget
	// displays right away without waiting for the first 15s ticker tick.
	go func() {
		s.CfgMu.RLock()
		personalityEnabled := s.Cfg.Personality.Engine
		s.CfgMu.RUnlock()
		if !personalityEnabled {
			return
		}
		// Small delay to allow SSE clients to connect
		time.Sleep(2 * time.Second)
		sse.BroadcastType(EventPersonalityUpdate, s.buildPersonalityStatePayload())
	}()

	// Push tsnet status changes via SSE so shared.js does not need to poll
	// /api/tsnet/status. Only broadcasts when login_url / running state changes.
	go func() {
		if s.TsNetManager == nil {
			return
		}
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var lastState string
		for {
			select {
			case <-ticker.C:
				status := s.TsNetManager.GetStatus()
				state := fmt.Sprintf("%v|%v|%s|%s", status.Running, status.Starting, status.LoginURL, status.Error)
				if state == lastState {
					continue
				}
				lastState = state
				s.CfgMu.RLock()
				enabled := s.Cfg.Tailscale.TsNet.Enabled
				s.CfgMu.RUnlock()
				sse.BroadcastType(EventTsnetStatus, map[string]interface{}{
					"enabled":   enabled,
					"running":   status.Running,
					"starting":  status.Starting,
					"login_url": status.LoginURL,
					"hostname":  status.Hostname,
					"dns":       status.DNS,
					"ips":       status.IPs,
					"error":     status.Error,
				})
			case <-serverCtx.Done():
				return
			}
		}
	}()

	// Phase 34: Start the background daily reflection loop
	tools.StartDailyReflectionLoop(serverCtx, s.Cfg, s.Logger, s.LLMClient, s.HistoryManager, s.ShortTermMem)

	// Phase 68: Start the daily maintenance loop
	manifest := tools.NewManifest(s.Cfg.Directories.ToolsDir)
	agent.StartMaintenanceLoop(serverCtx, s.Cfg, s.Logger, s.LLMClient, s.Vault, s.Registry, manifest, s.CronManager, s.LongTermMem, s.ShortTermMem, s.HistoryManager, s.KG, s.InventoryDB, s.ContactsDB, s.PlannerDB, s.CheatsheetDB, s.MissionManagerV2)

	// Start Planner Notifier for appointment reminders with agent wake-up
	if s.PlannerDB != nil && s.Cfg.Tools.Planner.Enabled {
		plannerNotifier := planner.NewNotifier(s.PlannerDB, s.Logger)
		if s.MissionManagerV2 != nil {
			plannerNotifier.SetMissionTrigger(func(appointment planner.Appointment) {
				s.MissionManagerV2.NotifyPlannerAppointmentDue(appointment.ID, appointment.Title, appointment.DateTime)
			})
			plannerNotifier.SetTodoOverdueTrigger(func(todo planner.Todo) {
				s.MissionManagerV2.NotifyPlannerTodoOverdue(todo.ID, todo.Title, todo.DueDate)
			})
		}
		plannerNotifier.SetExecutor(func(prompt string) {
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
				PlannerDB:          s.PlannerDB,
				SQLConnectionsDB:   s.SQLConnectionsDB,
				SQLConnectionPool:  s.SQLConnectionPool,
				RemoteHub:          s.RemoteHub,
				Vault:              s.Vault,
				Registry:           s.Registry,
				CronManager:        s.CronManager,
				MissionManagerV2:   s.MissionManagerV2,
				CoAgentRegistry:    s.CoAgentRegistry,
				BudgetTracker:      s.BudgetTracker,
				DaemonSupervisor:   s.DaemonSupervisor,
				LLMGuardian:        s.LLMGuardian,
				PreparationService: s.PreparationService,
				SessionID:          "default",
				IsMaintenance:      false,
				MessageSource:      "planner_notification",
			}
			go agent.Loopback(runCfg, prompt, agent.NoopBroker{})
		})
		go plannerNotifier.Start(serverCtx)
		s.Logger.Info("Planner notifier started")
	}

	s.CoAgentRegistry.StartCleanupLoop()

	// Start OAuth2 token refresh loop (auto-refreshes before expiry)
	startOAuthRefreshLoop(s, serverCtx)

	// Health check — no auth required, used by Docker HEALTHCHECK and monitoring.
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Basic health check - just return ok for now (db checks done via separate endpoint if needed)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Readiness check — returns 503 until the server has finished initialization
	// and is actively accepting connections. Used by Docker HEALTHCHECK and load balancers.
	mux.HandleFunc("/api/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.ready.Load() {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "initializing"})
		}
	})

	// System warnings — returns runtime health warnings.
	mux.HandleFunc("/api/warnings", handleWarnings(s))
	mux.HandleFunc("/api/warnings/acknowledge", handleWarningsAcknowledge(s))

	mux.HandleFunc("/v1/chat/completions", handleChatCompletions(s, sse))
	mux.HandleFunc("/api/chat/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleListChatSessions(s)(w, r)
		case http.MethodPost:
			handleCreateChatSession(s)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/chat/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
		if path == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleGetChatSession(s)(w, r)
		case http.MethodDelete:
			handleDeleteChatSession(s)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
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
	mux.HandleFunc("/api/n8n/missions/", handleN8nMissionManage(s))
	mux.HandleFunc("/api/n8n/sessions", handleN8nSessions(s))
	mux.HandleFunc("/api/n8n/sessions/", handleN8nSessionManage(s))
	mux.HandleFunc("/api/n8n/webhooks/history", handleN8nWebhookHistory(s))

	// Internal Tool Bridge (loopback-only, for Python skills calling native tools)
	mux.HandleFunc("/api/internal/tool-bridge/", handleToolBridgeExecute(s))

	// Quick Setup wizard endpoints (always available — needed before config is complete)
	mux.HandleFunc("/api/setup/profiles", handleSetupProfiles(s))
	mux.HandleFunc("/api/setup/status", handleSetupStatus(s))
	mux.HandleFunc("/api/setup/test", handleSetupTestConnection(s))
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
	mux.HandleFunc("/api/auth/logout", handleAuthLogoutAPI(s))
	mux.HandleFunc("/api/security/status", handleSecurityStatus(s))
	mux.HandleFunc("/api/auth/password", handleAuthSetPassword(s))
	mux.HandleFunc("/api/auth/totp/setup", handleAuthTOTPSetup(s))
	mux.HandleFunc("/api/auth/totp/confirm", handleAuthTOTPConfirm(s))
	mux.HandleFunc("/api/auth/totp", handleAuthTOTPDelete(s))

	mux.HandleFunc("/api/personalities", handleListPersonalities(s))
	mux.HandleFunc("/api/personality", handleUpdatePersonality(s))
	mux.HandleFunc("/api/personality/state", handlePersonalityState(s))
	mux.HandleFunc("/api/personality/feedback", handlePersonalityFeedback(s))
	mux.HandleFunc("/api/agent/question-status", handleQuestionStatus(s))
	mux.HandleFunc("/api/agent/question-response", handleQuestionResponse(s))
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if enabled && !IsAuthenticated(r, secret) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
			return
		}
		sse.ServeHTTP(w, r)
	})

	mux.HandleFunc("/api/space-agent/status", handleSpaceAgentStatus(s))
	mux.HandleFunc("/api/space-agent/recreate", handleSpaceAgentRecreate(s))
	mux.HandleFunc("/api/space-agent/send", handleSpaceAgentSend(s))
	mux.HandleFunc("/api/space-agent/bridge/messages", handleSpaceAgentBridgeMessages(s))
	mux.HandleFunc("/api/integrations/webhosts", handleIntegrationWebhosts(s))
	mux.HandleFunc("/integrations/space-agent", handleSpaceAgentLegacyRedirect(s))
	mux.HandleFunc("/integrations/space-agent/", handleSpaceAgentLegacyRedirect(s))
	mux.HandleFunc("/api/desktop/bootstrap", handleDesktopBootstrap(s))
	mux.HandleFunc("/api/desktop/files", handleDesktopFiles(s))
	mux.HandleFunc("/api/desktop/file", handleDesktopFile(s))
	mux.HandleFunc("/api/desktop/directory", handleDesktopDirectory(s))
	mux.HandleFunc("/api/desktop/apps", handleDesktopApps(s))
	mux.HandleFunc("/api/desktop/widgets", handleDesktopWidgets(s))
	mux.HandleFunc("/api/desktop/embed-token", handleDesktopEmbedToken(s))
	mux.HandleFunc("/api/desktop/chat", handleDesktopChat(s))
	mux.HandleFunc("/api/desktop/ws", handleDesktopWS(s))

	s.registerConfigAPIRoutes(mux, sse)

	// ── Integration bots (disabled in egg mode — eggs are headless workers) ──
	if !s.Cfg.EggMode.Enabled {
		// Phase 35.2: Start the Telegram Long Polling loop
		telegram.StartLongPolling(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2, s.Guardian)

		// Discord Bot: listen for messages and relay to the agent
		discord.StartBot(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2, s.Guardian)

		// Email Watcher: poll IMAP for new messages and wake the agent
		if emailWatcher := tools.StartEmailWatcher(s.Cfg, s.Logger, s.Guardian, s.LLMGuardian); emailWatcher != nil {
			s.MissionManagerV2.SetEmailWatcher(emailWatcher)
		}

		// Rocket.Chat Bot: listen for messages and relay to the agent
		rocketchat.StartBot(s.Cfg, s.Logger, s.LLMClient, s.ShortTermMem, s.LongTermMem, s.Vault, s.Registry, s.CronManager, s.HistoryManager, s.KG, s.InventoryDB, s.MissionManagerV2, s.Guardian)

		// MQTT Client: connect to broker and register bridge
		s.configureMQTTRelay()
		mqtt.StartClient(s.Cfg, s.Logger)

		// Telnyx: register webhook endpoint for incoming SMS/calls
		if s.Cfg.Telnyx.Enabled {
			webhookPath := s.Cfg.Telnyx.WebhookPath
			if webhookPath == "" {
				webhookPath = "/api/telnyx/webhook"
			}
			telnyxHandler := telnyx.NewWebhookHandler(s.Cfg, s.Logger, func(from, text string, mediaURLs []string) {
				if tools.HasPendingQuestion("default") {
					if response, ok := tools.ResolveQuestionReply("default", text); ok {
						tools.CompleteQuestion("default", response)
						return
					}
					telnyx.NewSMSBroker(s.Cfg, from, s.Logger).Send("question_user", "Please reply with one of the listed numbers.")
					return
				}
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
					PlannerDB:          s.PlannerDB,
					SQLConnectionsDB:   s.SQLConnectionsDB,
					SQLConnectionPool:  s.SQLConnectionPool,
					RemoteHub:          s.RemoteHub,
					Vault:              s.Vault,
					Registry:           s.Registry,
					CronManager:        s.CronManager,
					MissionManagerV2:   s.MissionManagerV2,
					CoAgentRegistry:    s.CoAgentRegistry,
					BudgetTracker:      s.BudgetTracker,
					DaemonSupervisor:   s.DaemonSupervisor,
					LLMGuardian:        s.LLMGuardian,
					PreparationService: s.PreparationService,
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
			mcpConfigs := buildRuntimeMCPConfigs(s.Cfg, s.Vault, s.Logger)
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
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		sessionSecret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if authEnabled && !IsAuthenticated(r, sessionSecret) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
			return
		}

		// Support session-specific history via query parameter
		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID != "" && sessionID != "default" {
			messages, err := s.ShortTermMem.GetSessionMessages(sessionID)
			if err != nil {
				s.Logger.Error("Failed to get session messages", "session_id", sessionID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			var filtered []memory.HistoryMessage
			for _, m := range messages {
				if !m.IsInternal {
					filtered = append(filtered, m)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(filtered)
			return
		}

		all := s.HistoryManager.GetAll()
		var filtered []memory.HistoryMessage
		for _, m := range all {
			if memory.ShouldHideAutonomousMessage("default", m.Role, m.Content) {
				continue
			}
			if !m.IsInternal {
				filtered = append(filtered, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(filtered)
	})

	mux.HandleFunc("/api/plans/active", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID == "" {
			sessionID = "default"
		}
		var plan any
		if s.ShortTermMem != nil {
			activePlan, err := s.ShortTermMem.GetSessionPlan(sessionID)
			if err != nil {
				s.Logger.Error("Failed to fetch active session plan", "session_id", sessionID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			plan = activePlan
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"plan":   plan,
		})
	})
	mux.HandleFunc("/api/plans", handlePlansList(s))
	mux.HandleFunc("/api/plans/", handlePlanByID(s))

	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		sessionSecret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if authEnabled && !IsAuthenticated(r, sessionSecret) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
			return
		}
		// Support session-specific clear via query parameter
		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID != "" && sessionID != "default" {
			if err := s.ShortTermMem.ClearSession(sessionID); err != nil {
				s.Logger.Error("Failed to clear session", "session_id", sessionID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := s.HistoryManager.Clear(); err != nil {
			s.Logger.Error("Failed to clear chat history", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		agent.ResetInnerVoiceState()
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("/api/admin/stop", requireAdmin(s, handleInterrupt(s)))

	ttsServer, err := s.registerUIRoutes(mux, shutdownCh)
	if err != nil {
		return err
	}
	s.registerToolAPIRoutes(mux)
	s.registerInfrastructureRoutes(mux, shutdownCh)

	// Start Phase 1 TCP Bridge (Lifeboat IPC — lifeboat dials this on port 8089)
	// This port is intentionally separate from maintenance.lifeboat_port (8091),
	// which is the port lifeboat itself listens on.
	go s.StartTCPBridge("localhost:8089")

	// Dedicated loopback HTTP port:
	// When HTTPS is active, open a plain HTTP listener on 127.0.0.1 only for
	// internal self-calls. If cloudflare_tunnel.loopback_port is set, cloudflared
	// can use the same listener to avoid local TLS verification. The loopback
	// interface ensures this port is never reachable from the network.
	s.CfgMu.RLock()
	loopbackPort := DedicatedInternalLoopbackPort(s.Cfg)
	s.CfgMu.RUnlock()
	// Build a dynamic loopback handler: routes requests to either the Homepage caddy
	// server or the Web UI depending on the current expose-target config, without
	// requiring a cloudflared restart or port change.
	webUILoopbackHandler := accessLogMiddleware(s.accessLogger(), securityHeadersMiddleware(authMiddleware(s, mux), false, false), false)
	homepageProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			s.CfgMu.RLock()
			port := s.Cfg.Homepage.WebServerPort
			if port <= 0 {
				port = 8080
			}
			s.CfgMu.RUnlock()
			targetURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Header.Set("X-Forwarded-Host", req.Host)
		},
	}
	s.loopbackHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		exposeHomepage := s.Cfg.CloudflareTunnel.ExposeHomepage
		exposeWebUI := s.Cfg.CloudflareTunnel.ExposeWebUI
		s.CfgMu.RUnlock()
		if r.Header.Get("X-Internal-Token") != "" || r.Header.Get("X-Internal-FollowUp") != "" {
			webUILoopbackHandler.ServeHTTP(w, r)
			return
		}
		if exposeHomepage && !exposeWebUI {
			homepageProxy.ServeHTTP(w, r)
		} else {
			webUILoopbackHandler.ServeHTTP(w, r)
		}
	})
	if loopbackPort > 0 {
		bindAddr := fmt.Sprintf("127.0.0.1:%d", loopbackPort)
		ln, err := net.Listen("tcp4", bindAddr)
		if err != nil {
			s.Logger.Warn("[Loopback] Could not bind internal HTTP listener", "addr", bindAddr, "error", err)
		} else {
			s.Logger.Info("[Loopback] Starting internal HTTP listener", "port", loopbackPort)
			s.loopbackSrv = &http.Server{
				Handler:      s.loopbackHandler,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 5 * time.Minute,
				IdleTimeout:  2 * time.Minute,
			}
			go func() {
				if err := s.loopbackSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
					s.Logger.Warn("[Loopback] Internal HTTP listener stopped", "error", err)
				}
			}()
		}
	}

	s.reconcileSpaceAgentHTTPSProxy()

	// Always build and store the tsnet handler so it is available even when tsnet
	// is enabled later via the config UI without a restart.
	if s.TsNetManager != nil {
		tsHandler := accessLogMiddleware(s.accessLogger(), securityHeadersMiddleware(authMiddleware(s, mux), true, false), false)
		s.tsNetHandler = tsHandler // stored for /api/tsnet/start (runtime start after hot-reload)
		if s.Cfg.Tailscale.TsNet.Enabled {
			go func() {
				if err := s.TsNetManager.Start(tsHandler); err != nil {
					s.Logger.Error("Failed to start tsnet node", "error", err)
				}
			}()
		}
	}

	// Determine server mode: HTTPS auto, HTTPS custom, HTTPS self-signed, or HTTP
	tlsCfg := NewTLSConfigFromConfig(s.Cfg, s.Cfg.Directories.DataDir)
	tlsCfg.BehindProxy = s.Cfg.Server.HTTPS.BehindProxy

	if tlsCfg.IsTLSActive() {
		// Security Proxy (Caddy/Docker) and built-in HTTPS are mutually exclusive:
		// both want port 443. If the Security Proxy is running, AuraGo is already
		// behind a TLS-terminating reverse proxy — use plain HTTP.
		if s.Cfg.SecurityProxy.Enabled {
			s.Logger.Error("Built-in HTTPS and Security Proxy are both enabled — they compete for port 443. " +
				"Disabling built-in HTTPS and starting in HTTP mode (Security Proxy handles TLS). " +
				"Fix: disable server.https or disable security_proxy in config.yaml.")
			return s.runHTTP(mux, ttsServer, shutdownCh)
		}

		// Pre-check that the HTTPS port is actually bindable.
		// Ports < 1024 require root or CAP_NET_BIND_SERVICE on Linux.
		// Fail immediately with a clear, actionable error — never silently degrade to HTTP.
		if ln, err := net.Listen("tcp", fmt.Sprintf(":%d", tlsCfg.HTTPSPort)); err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "access is denied") {
				return fmt.Errorf(
					"cannot bind HTTPS port %d: permission denied\n\n"+
						"Fix options:\n"+
						"  1. Grant the binary the required network capability (recommended):\n"+
						"       sudo setcap cap_net_bind_service=+ep %s\n"+
						"  2. Use unprivileged ports (≥1024) in config.yaml:\n"+
						"       server.https.https_port: 8443\n"+
						"       server.https.http_port:  8080\n"+
						"  3. Run as root (not recommended)\n\n"+
						"The server did NOT fall back to HTTP. HTTPS configuration is active and must work.",
					tlsCfg.HTTPSPort, os.Args[0])
			}
			// Port in use or other error — let runHTTPS surface it with full details
		} else {
			ln.Close()
		}
		return s.runHTTPS(mux, ttsServer, tlsCfg, shutdownCh)
	}

	return s.runHTTP(mux, ttsServer, shutdownCh)
}
