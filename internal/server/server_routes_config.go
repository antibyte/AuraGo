package server

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"

	"aurago/internal/services/optimizer"
)

func (s *Server) registerConfigAPIRoutes(mux *http.ServeMux, sse *SSEBroadcaster) {
	if !s.Cfg.WebConfig.Enabled {
		return
	}

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
	mux.HandleFunc("/api/mcp-server/vscode-bridge", handleMCPServerVSCodeBridgeInfo(s))

	// Python Tool Bridge config API (Skill Manager UI helper)
	mux.HandleFunc("/api/python-tool-bridge/tools", handlePythonToolBridgeTools(s))

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
	// Returns status of the managed Ollama container.
	mux.HandleFunc("/api/ollama/managed/status", handleOllamaManagedStatus(s))
	// Creates/starts the managed Ollama container (recovery after manual deletion).
	mux.HandleFunc("/api/ollama/managed/recreate", handleOllamaManagedRecreate(s))
	// Tests MeshCentral connectivity using saved or provided credentials.
	mux.HandleFunc("/api/meshcentral/test", handleMeshCentralTest(s))
	mux.HandleFunc("/api/restart", handleRestart(s))
	mux.HandleFunc("/api/embeddings/reset", handleEmbeddingsReset(s))
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
	mux.HandleFunc("/api/debug/helper-llm/stats", handleHelperLLMStats(s))

	// Homepage tool endpoints
	mux.HandleFunc("/api/homepage/status", handleHomepageStatus(s))
	mux.HandleFunc("/api/homepage/detect-workspace", handleHomepageDetectWorkspace(s))
	mux.HandleFunc("/api/homepage/test-connection", handleHomepageTestConnection(s))

	// Cloudflare Tunnel endpoints
	mux.HandleFunc("/api/tunnel/status", handleTunnelStatus(s))
	mux.HandleFunc("/api/tunnel/quick", handleTunnelQuick(s))

	// Netlify integration endpoints
	mux.HandleFunc("/api/netlify/status", handleNetlifyStatus(s))
	mux.HandleFunc("/api/netlify/test-connection", handleNetlifyTestConnection(s))
	mux.HandleFunc("/api/vercel/status", handleVercelStatus(s))
	mux.HandleFunc("/api/vercel/test-connection", handleVercelTestConnection(s))

	// Google Workspace integration endpoints
	mux.HandleFunc("/api/google-workspace/test", handleGoogleWorkspaceTest(s))

	// OneDrive integration endpoints (Device Code Flow)
	mux.HandleFunc("/api/onedrive/auth/start", handleOneDriveAuthStart(s))
	mux.HandleFunc("/api/onedrive/auth/poll", handleOneDriveAuthPoll(s))
	mux.HandleFunc("/api/onedrive/auth/status", handleOneDriveAuthStatus(s))
	mux.HandleFunc("/api/onedrive/auth/revoke", handleOneDriveAuthRevoke(s))
	mux.HandleFunc("/api/onedrive/test", handleOneDriveTest(s))

	// Music Generation endpoints
	mux.HandleFunc("/api/music-generation/test", handleMusicGenerationTest(s))

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
	mux.HandleFunc("/api/uptime-kuma/status", handleUptimeKumaStatus(s))
	mux.HandleFunc("/api/uptime-kuma/test", handleUptimeKumaTest(s))

	// MQTT integration endpoints
	mux.HandleFunc("/api/mqtt/status", handleMQTTStatus(s))
	mux.HandleFunc("/api/mqtt/test", handleMQTTTest(s))
	mux.HandleFunc("/api/mqtt/messages", handleMQTTMessages(s))

	// Fritz!Box integration endpoints
	mux.HandleFunc("/api/fritzbox/status", handleFritzBoxStatus(s))
	mux.HandleFunc("/api/fritzbox/test", handleFritzBoxTest(s))

	// Paperless-ngx integration endpoints
	mux.HandleFunc("/api/paperless/test", handlePaperlessTest(s))

	// LDAP integration endpoints
	mux.HandleFunc("/api/ldap/test", handleLDAPTest(s))

	// Document Creator endpoints
	mux.HandleFunc("/api/document-creator/test", handleGotenbergTest(s))
	mux.HandleFunc("/api/browser-automation/status", handleBrowserAutomationStatus(s))
	mux.HandleFunc("/api/browser-automation/test", handleBrowserAutomationTest(s))

	// Ansible endpoints
	mux.HandleFunc("/api/ansible/generate-token", handleAnsibleGenerateToken(s))

	// Piper TTS endpoints
	mux.HandleFunc("/api/piper/status", handlePiperStatus(s))
	mux.HandleFunc("/api/piper/voices", handlePiperVoices(s))

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
	mux.HandleFunc("/api/tools/mac_lookup", handleMACLookup(s))

	// Credentials Registry (vault-backed access data)
	mux.HandleFunc("/api/credentials", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleListCredentials(s)(w, r)
		case http.MethodPost:
			handleCreateCredential(s)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/credentials/python-accessible", func(w http.ResponseWriter, r *http.Request) {
		handleListPythonAccessibleCredentials(s)(w, r)
	})
	mux.HandleFunc("/api/credentials/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetCredential(s)(w, r)
		case http.MethodPut:
			handleUpdateCredential(s)(w, r)
		case http.MethodDelete:
			handleDeleteCredential(s)(w, r)
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
				handleListWebhooks(s, s.WebhookManager)(w, r)
			case http.MethodPost:
				handleCreateWebhook(s, s.WebhookManager)(w, r)
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
				handleUpdateWebhook(s, s.WebhookManager)(w, r)
			case http.MethodDelete:
				handleDeleteWebhook(s, s.WebhookManager)(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
	} else {
		// Return empty list when webhooks are disabled so the UI doesn't get a 404.
		mux.HandleFunc("/api/webhooks", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
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
	mux.HandleFunc("/api/background-tasks", handleBackgroundTasks(s))
	mux.HandleFunc("/api/background-tasks/", handleBackgroundTaskByID(s))
	mux.HandleFunc("/api/dashboard/prompt-stats", handleDashboardPromptStats())
	mux.HandleFunc("/api/dashboard/tool-stats", handleDashboardToolStats(s.Cfg))
	mux.HandleFunc("/api/dashboard/github-repos", handleDashboardGitHubRepos(s))
	mux.HandleFunc("/api/github/repos", handleGitHubReposForUI(s))
	mux.HandleFunc("/api/dashboard/logs", handleDashboardLogs(s))
	mux.HandleFunc("/api/dashboard/overview", handleDashboardOverview(s))
	mux.HandleFunc("/api/dashboard/notes", handleDashboardNotes(s))
	mux.HandleFunc("/api/dashboard/optimization", optimizer.OptimizationDashboardHandler)
	mux.HandleFunc("/api/memory/activity-overview", handleActivityOverview(s))
	mux.HandleFunc("/api/dashboard/journal", handleDashboardJournal(s))
	mux.HandleFunc("/api/dashboard/journal/summaries", handleDashboardJournalSummary(s))
	mux.HandleFunc("/api/dashboard/journal/stats", handleDashboardJournalStats(s))
	mux.HandleFunc("/api/dashboard/helper-llm", handleDashboardHelperLLM(s))
	mux.HandleFunc("/api/dashboard/guardian", handleDashboardGuardian(s))
	mux.HandleFunc("/api/dashboard/errors", handleDashboardErrors(s))
	mux.HandleFunc("/api/dashboard/compression", handleDashboardCompression(s))
	mux.HandleFunc("/api/dashboard/mission-history", handleDashboardMissionHistory(s))
	mux.HandleFunc("/api/knowledge-graph/node", handleKnowledgeGraphNodeDetail(s))
	mux.HandleFunc("/api/knowledge-graph/node/protect", handleKnowledgeGraphNodeProtect(s))
	mux.HandleFunc("/api/knowledge-graph/edge", handleKnowledgeGraphEdgeMutate(s))
	mux.HandleFunc("/api/knowledge-graph/nodes", handleKnowledgeGraphNodes(s))
	mux.HandleFunc("/api/knowledge-graph/edges", handleKnowledgeGraphEdges(s))
	mux.HandleFunc("/api/knowledge-graph/important", handleKnowledgeGraphImportant(s))
	mux.HandleFunc("/api/knowledge-graph/stats", handleKnowledgeGraphStats(s))
	mux.HandleFunc("/api/knowledge-graph/search", handleKnowledgeGraphSearch(s))
	mux.HandleFunc("/api/knowledge-graph/quality", handleKnowledgeGraphQuality(s))

	// System endpoints
	mux.HandleFunc("/api/system/os", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"os": runtime.GOOS})
	})

	// File Indexing API endpoints
	mux.HandleFunc("/api/indexing/status", handleIndexingStatus(s))
	mux.HandleFunc("/api/indexing/rescan", handleIndexingRescan(s))
	mux.HandleFunc("/api/indexing/directories", handleIndexingDirectories(s))

	// FileIndexer→KG Debug/Inspection endpoints
	mux.HandleFunc("/api/debug/kg-file-sync-stats", handleDebugKGFileSyncStats(s))
	mux.HandleFunc("/api/debug/kg-orphans", handleDebugKGOrphans(s))
	mux.HandleFunc("/api/debug/file-sync-status", handleDebugFileSyncStatus(s))
	mux.HandleFunc("/api/debug/file-sync-last-run", handleDebugFileSyncLastRun(s))
	mux.HandleFunc("/api/debug/kg-file-entities", handleDebugKGFileEntities(s))
	mux.HandleFunc("/api/debug/kg-node-sources", handleDebugKGNodeSources(s))
	mux.HandleFunc("/api/debug/kg-file-sync-cleanup", handleDebugKGFileSyncCleanup(s))

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
	mux.HandleFunc("/api/missions/v2/history", handleMissionV2History(s))
	mux.HandleFunc("/api/missions/v2/", handleMissionV2ByID(s))
}
