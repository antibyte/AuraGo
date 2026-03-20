package server

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/fritzbox"
	"aurago/internal/invasion/bridge"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/proxy"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"
	"aurago/internal/tsnetnode"
	"aurago/internal/webhooks"

	a2apkg "aurago/internal/a2a"
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
				if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &translations); err != nil {
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
	Cfg                *config.Config
	CfgMu              sync.RWMutex // protects Cfg during hot-reload
	Logger             *slog.Logger
	LLMClient          llm.ChatClient
	ShortTermMem       *memory.SQLiteMemory
	LongTermMem        memory.VectorDB
	Vault              *security.Vault
	Registry           *tools.ProcessRegistry
	CronManager        *tools.CronManager
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
	MissionManagerV2   *tools.MissionManagerV2
	EggHub             *bridge.EggHub
	RemoteHub          *remote.RemoteHub
	ProxyManager       *proxy.Manager
	TsNetManager       *tsnetnode.Manager
	FileIndexer        *services.FileIndexer
	CheatsheetDB       *sql.DB
	ImageGalleryDB     *sql.DB
	MediaRegistryDB    *sql.DB
	HomepageRegistryDB *sql.DB
	A2AServer          *a2apkg.Server        // A2A protocol server (nil if disabled)
	A2AClientMgr       *a2apkg.ClientManager // A2A client manager (nil if disabled)
	A2ABridge          *a2apkg.Bridge        // A2A co-agent bridge (nil if disabled)
	// IsFirstStart is true if core_memory.md was just freshly created (no prior data).
	IsFirstStart   bool
	StartedAt      time.Time     // server start time for uptime calculation
	ShutdownCh     chan struct{} // signal channel for graceful shutdown
	firstStartDone bool
	muFirstStart   sync.Mutex
}

func Start(cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, remoteControlDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, isFirstStart bool, shutdownCh chan struct{}) error {
	s := &Server{
		Cfg:                cfg,
		Logger:             logger,
		LLMClient:          llmClient,
		ShortTermMem:       shortTermMem,
		LongTermMem:        longTermMem,
		Vault:              vault,
		Registry:           registry,
		CronManager:        cronManager,
		HistoryManager:     historyManager,
		KG:                 kg,
		InventoryDB:        inventoryDB,
		InvasionDB:         invasionDB,
		CheatsheetDB:       cheatsheetDB,
		ImageGalleryDB:     imageGalleryDB,
		MediaRegistryDB:    mediaRegistryDB,
		HomepageRegistryDB: homepageRegistryDB,
		Guardian:           security.NewGuardian(logger),
		LLMGuardian:        security.NewLLMGuardian(cfg, logger),
		CoAgentRegistry:    agent.NewCoAgentRegistry(cfg.CoAgents.MaxConcurrent, logger),
		BudgetTracker:      budget.NewTracker(cfg, logger, cfg.Directories.DataDir),
		IsFirstStart:       isFirstStart,
		StartedAt:          time.Now(),
		ShutdownCh:         shutdownCh,
		MissionManagerV2:   tools.NewMissionManagerV2(cfg.Directories.DataDir, cronManager),
		EggHub:             bridge.NewEggHub(logger),
	}

	// Initialize Remote Control Hub
	remote.InsecureHostKey = cfg.RemoteControl.SSHInsecureHostKey
	if remoteControlDB != nil {
		s.RemoteHub = remote.NewRemoteHub(remoteControlDB, vault, logger)
		s.RemoteHub.DefaultReadOnly = cfg.RemoteControl.ReadOnly
		s.RemoteHub.AutoApprove = cfg.RemoteControl.AutoApprove
		s.RemoteHub.StartHeartbeatMonitor(30*time.Second, 90*time.Second)
		if err := remote.TrimAuditLog(remoteControlDB, 10000); err != nil {
			logger.Warn("Failed to trim remote audit log", "error", err)
		}
		logger.Info("Remote Control Hub initialized", "insecure_host_key", cfg.RemoteControl.SSHInsecureHostKey)
	}

	// Initialize Security Proxy Manager
	s.ProxyManager = proxy.NewManager(cfg, logger)
	if cfg.SecurityProxy.Enabled {
		logger.Info("Security proxy enabled in config — use API to start/stop the proxy container")
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
			s.WebhookHandler = webhooks.NewHandler(whMgr, tm, s.Guardian, s.LLMGuardian, cfg, logger, cfg.Server.Port, int64(cfg.Webhooks.MaxPayloadSize), cfg.Webhooks.RateLimit)
			logger.Info("Webhook system initialized", "max_webhooks", webhooks.MaxWebhooks)
		}
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
				// Extract the assistant's text from the OpenAI-format response
				output := extractAssistantContent(respBody)
				s.MissionManagerV2.SetResult(missionID, "success", output)
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

	// Set cheatsheet DB for mission prompt expansion
	if s.CheatsheetDB != nil {
		s.MissionManagerV2.SetCheatsheetDB(s.CheatsheetDB)
	}

	if err := s.MissionManagerV2.Start(); err != nil {
		logger.Warn("Failed to start MissionManagerV2", "error", err)
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
		s.FileIndexer.Start(context.Background())
		logger.Info("File indexer started", "directories", cfg.Indexing.Directories)
	}

	// Start Firewall Guard loop if enabled
	if cfg.Firewall.Enabled && cfg.Firewall.Mode == "guard" {
		go tools.StartFirewallGuard(context.Background(), cfg, logger, func(prompt string) {
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
					logger.Error("[FirewallGuard] Failed to create request", "error", err)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Internal-FollowUp", "true")

				client := &http.Client{Timeout: 10 * time.Minute}
				_, err = client.Do(req)
				if err != nil {
					logger.Error("[FirewallGuard] Execution failed", "error", err)
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
				ExposeWebUI:    cfg.CloudflareTunnel.ExposeWebUI,
				ExposeHomepage: cfg.CloudflareTunnel.ExposeHomepage,
				MetricsPort:    cfg.CloudflareTunnel.MetricsPort,
				LogLevel:       cfg.CloudflareTunnel.LogLevel,
				DockerHost:     cfg.Docker.Host,
				WebUIPort:      cfg.Server.Port,
				HomepagePort:   cfg.Homepage.WebServerPort,
				DataDir:        cfg.Directories.DataDir,
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

	// Start Fritz!Box telephony poller if enabled
	if cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled && cfg.FritzBox.Telephony.Polling.Enabled {
		fbPoller := fritzbox.NewPoller(*cfg, func(kind, summary string) {
			go func() {
				url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", cfg.Server.Port)
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
				client := &http.Client{Timeout: 10 * time.Minute}
				if _, err := client.Do(req); err != nil {
					logger.Error("[FritzBox Poller] Loopback request failed", "error", err)
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

// runHTTP starts the server in HTTP mode (for local/LAN use)
func (s *Server) runHTTP(mux *http.ServeMux, ttsServer *http.Server, shutdownCh chan struct{}) error {
	addr := fmt.Sprintf("%s:%d", s.Cfg.Server.Host, s.Cfg.Server.Port)
	s.Logger.Info("Starting HTTP server", "host", s.Cfg.Server.Host, "port", s.Cfg.Server.Port, "tls", false)

	// Apply security headers (relaxed for HTTP, but still present)
	handler := accessLogMiddleware(s.Logger, securityHeadersMiddleware(authMiddleware(s, mux), false, s.Cfg.Server.HTTPS.BehindProxy))

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
	handler := accessLogMiddleware(s.Logger, securityHeadersMiddleware(authMiddleware(s, mux), true, s.Cfg.Server.HTTPS.BehindProxy))

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
				s.Logger.Error("HTTP redirect server error", "error", err)
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

		// Shut down MCP servers
		tools.ShutdownMCPManager()
		// Shut down Sandbox
		tools.ShutdownSandboxManager()
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
		if err := server.Shutdown(ctx); err != nil {
			s.Logger.Error("Server shutdown error", "error", err)
		}
	}()

	// Start main server
	var err error
	if server.TLSConfig != nil {
		err = server.ListenAndServeTLS("", "")
	} else {
		err = server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return err
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

		// Content Security Policy - strict but allows necessary CDNs
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.tailwindcss.com https://cdn.jsdelivr.net https://cdnjs.cloudflare.com https://unpkg.com; " +
			"style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com https://fonts.googleapis.com; " +
			"img-src 'self' data: blob:; " +
			"font-src 'self' https://fonts.gstatic.com; " +
			"connect-src 'self' ws: wss:; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self';"
		w.Header().Set("Content-Security-Policy", csp)

		if tlsActive {
			// Strict Transport Security (only for HTTPS)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Add cache control for authenticated routes
		if !strings.HasPrefix(r.URL.Path, "/auth/") &&
			!strings.HasPrefix(r.URL.Path, "/api/auth/") &&
			!strings.HasPrefix(r.URL.Path, "/setup") &&
			!strings.HasPrefix(r.URL.Path, "/static/") {
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
func accessLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
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
			path == "/api/health"
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

		// Classify log level: 4xx/5xx responses and auth-related paths are
		// logged at Warn level so they can be filtered easily by monitoring tools.
		isError := rec.status >= 400
		isAuthPath := strings.HasPrefix(path, "/auth/") ||
			strings.HasPrefix(path, "/api/auth/")

		args := []any{
			"method", r.Method,
			"path", path,
			"status", rec.status,
			"duration_ms", elapsed,
			"ip", ClientIP(r),
			"user_agent", r.UserAgent(),
		}
		if isError || isAuthPath {
			logger.Warn("[Access]", args...)
		} else {
			logger.Info("[Access]", args...)
		}
	})
}
