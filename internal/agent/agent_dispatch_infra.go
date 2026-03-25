package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strings"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/fritzbox"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
)

// dispatchNotHandled is a sentinel value returned by sub-dispatchers when tc.Action is not recognized.
const dispatchNotHandled = "\x00__DISPATCH_NOT_HANDLED__"

// dispatchInfra handles network, cloud platform, and external service tool calls
// (co_agent, mdns, tts, chromecast, proxmox, ollama, tailscale, ansible, invasion, github, netlify, mqtt, mcp, adguard).
func dispatchInfra(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManagerV2 *tools.MissionManagerV2, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, contactsDB *sql.DB, sqlConnectionsDB *sql.DB, sqlConnectionPool *sqlconnections.ConnectionPool, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
	switch tc.Action {
	case "co_agent", "co_agents":
		if budgetTracker != nil && budgetTracker.IsBlocked("coagent") {
			return `Tool Output: {"status": "error", "message": "Co-Agent spawn blocked: daily budget exceeded. Try again tomorrow."}`
		}
		if !cfg.CoAgents.Enabled {
			return `Tool Output: {"status": "error", "message": "Co-Agent system is not enabled. Set co_agents.enabled=true in config.yaml."}`
		}
		if coAgentRegistry == nil {
			return `Tool Output: {"status": "error", "message": "Co-Agent registry not initialized."}`
		}
		switch tc.Operation {
		case "spawn", "start", "create":
			task := tc.Task
			if task == "" {
				task = tc.Content
			}
			if task == "" {
				return `Tool Output: {"status": "error", "message": "'task' is required to spawn a co-agent."}`
			}
			coReq := CoAgentRequest{
				Task:         task,
				ContextHints: tc.ContextHints,
			}
			id, err := SpawnCoAgent(cfg, ctx, logger, coAgentRegistry,
				shortTermMem, longTermMem, vault, registry, manifest, kg, inventoryDB, coReq, budgetTracker)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			slots := coAgentRegistry.AvailableSlots()
			return fmt.Sprintf(`Tool Output: {"status": "ok", "co_agent_id": "%s", "available_slots": %d, "message": "Co-Agent started. Use operation 'list' to check status and 'get_result' when completed."}`, id, slots)

		case "spawn_specialist":
			task := tc.Task
			if task == "" {
				task = tc.Content
			}
			if task == "" {
				return `Tool Output: {"status": "error", "message": "'task' is required to spawn a specialist."}`
			}
			specialist := tc.Specialist
			if specialist == "" {
				return `Tool Output: {"status": "error", "message": "'specialist' is required. Choose: researcher, coder, designer, security, writer."}`
			}
			coReq := CoAgentRequest{
				Task:         task,
				ContextHints: tc.ContextHints,
				Specialist:   specialist,
			}
			id, err := SpawnCoAgent(cfg, ctx, logger, coAgentRegistry,
				shortTermMem, longTermMem, vault, registry, manifest, kg, inventoryDB, coReq, budgetTracker)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			slots := coAgentRegistry.AvailableSlots()
			return fmt.Sprintf(`Tool Output: {"status": "ok", "co_agent_id": "%s", "specialist": "%s", "available_slots": %d, "message": "Specialist '%s' started. Use 'list' to check status and 'get_result' when completed."}`, id, specialist, slots, specialist)

		case "list", "status":
			list := coAgentRegistry.List()
			data, _ := json.Marshal(map[string]interface{}{
				"status":          "ok",
				"available_slots": coAgentRegistry.AvailableSlots(),
				"max_slots":       cfg.CoAgents.MaxConcurrent,
				"co_agents":       list,
			})
			return "Tool Output: " + string(data)

		case "get_result", "result":
			coID := tc.CoAgentID
			if coID == "" {
				coID = tc.ID
			}
			if coID == "" {
				return `Tool Output: {"status": "error", "message": "'co_agent_id' is required."}`
			}
			result, err := coAgentRegistry.GetResult(coID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			out, _ := json.Marshal(map[string]interface{}{
				"status":      "ok",
				"co_agent_id": coID,
				"result":      result,
			})
			return "Tool Output: " + string(out)

		case "stop", "cancel", "kill":
			coID := tc.CoAgentID
			if coID == "" {
				coID = tc.ID
			}
			if coID == "" {
				return `Tool Output: {"status": "error", "message": "'co_agent_id' is required."}`
			}
			if err := coAgentRegistry.Stop(coID); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "ok", "message": "Co-Agent '%s' stopped."}`, coID)

		case "stop_all", "cancel_all":
			n := coAgentRegistry.StopAll()
			return fmt.Sprintf(`Tool Output: {"status": "ok", "message": "Stopped %d co-agent(s)."}`, n)

		default:
			return `Tool Output: {"status": "error", "message": "Unknown co_agent operation. Use: spawn, spawn_specialist, list, get_result, stop, stop_all"}`
		}

	case "mdns_scan":
		if !cfg.Tools.NetworkScan.Enabled {
			return `Tool Output: {"status":"error","message":"mdns_scan is disabled. Enable tools.network_scan.enabled in config."}`
		}
		logger.Info("LLM requested mdns_scan", "service_type", tc.ServiceType, "timeout", tc.Timeout, "auto_register", tc.AutoRegister)
		scanResult := tools.MDNSScan(logger, tc.ServiceType, tc.Timeout)
		if tc.AutoRegister && inventoryDB != nil {
			scanResult = mdnsAutoRegister(scanResult, inventoryDB, tc.RegisterType, tc.RegisterTags, tc.OverwriteExisting, logger)
		}
		return "Tool Output: " + scanResult

	case "tts":
		if !cfg.Chromecast.Enabled && cfg.TTS.Provider == "" && !cfg.TTS.Piper.Enabled {
			return `Tool Output: {"status": "error", "message": "TTS is not configured. Set tts.provider in config.yaml."}`
		}
		text := tc.Text
		if text == "" {
			text = tc.Content
		}
		provider := cfg.TTS.Provider
		if provider == "" && cfg.TTS.Piper.Enabled {
			provider = "piper"
		}
		ttsCfg := tools.TTSConfig{
			Provider: provider,
			Language: tc.Language,
			DataDir:  cfg.Directories.DataDir,
		}
		if ttsCfg.Language == "" {
			ttsCfg.Language = cfg.TTS.Language
		}
		ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
		ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
		ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
		ttsCfg.Piper.Port = cfg.TTS.Piper.ContainerPort
		ttsCfg.Piper.Voice = cfg.TTS.Piper.Voice
		ttsCfg.Piper.SpeakerID = cfg.TTS.Piper.SpeakerID
		filename, err := tools.TTSSynthesize(ttsCfg, text)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "TTS failed: %v"}`, err)
		}

		// Auto-register in media registry
		if mediaRegistryDB != nil {
			format := "mp3"
			if strings.ToLower(provider) == "piper" {
				format = "wav"
			}
			tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
				MediaType:  "tts",
				SourceTool: "tts",
				Filename:   filename,
				FilePath:   filepath.Join(cfg.Directories.DataDir, "tts", filename),
				Format:     format,
				Provider:   provider,
				Prompt:     text,
				Language:   ttsCfg.Language,
				VoiceID:    ttsCfg.ElevenLabs.VoiceID,
				Tags:       []string{"auto-generated", "tts"},
			})
		}

		ttsPort := cfg.Server.Port // TTS is always served on the main server
		if cfg.Chromecast.Enabled && cfg.Chromecast.TTSPort > 0 {
			ttsPort = cfg.Chromecast.TTSPort // Chromecast has its own dedicated TTS server
		}
		audioURL := fmt.Sprintf("http://%s:%d/tts/%s", getLocalIP(cfg), ttsPort, filename)
		return fmt.Sprintf(`Tool Output: {"status": "success", "file": "%s", "url": "%s"}`, filename, audioURL)

	case "chromecast":
		if !cfg.Chromecast.Enabled {
			return `Tool Output: {"status": "error", "message": "Chromecast is disabled. Set chromecast.enabled=true in config.yaml."}`
		}
		// Resolve device_name → device_addr via inventory if device_addr is empty
		if tc.DeviceAddr == "" && tc.DeviceName != "" && inventoryDB != nil {
			devices, err := inventory.QueryDevices(inventoryDB, "", "chromecast", tc.DeviceName)
			if err == nil && len(devices) > 0 {
				tc.DeviceAddr = devices[0].IPAddress
				if tc.DevicePort == 0 && devices[0].Port > 0 {
					tc.DevicePort = devices[0].Port
				}
				logger.Info("Resolved chromecast device name", "name", tc.DeviceName, "addr", tc.DeviceAddr, "port", tc.DevicePort)
			} else {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Could not find chromecast device named '%s' in the device registry."}`, tc.DeviceName)
			}
		}
		op := tc.Operation
		switch op {
		case "discover":
			return "Tool Output: " + tools.ChromecastDiscover(logger)
		case "play":
			url := tc.URL
			ct := tc.ContentType
			return "Tool Output: " + tools.ChromecastPlay(tc.DeviceAddr, tc.DevicePort, url, ct, logger)
		case "speak":
			text := tc.Text
			if text == "" {
				text = tc.Content
			}
			ttsCfg := tools.TTSConfig{
				Provider: cfg.TTS.Provider,
				Language: tc.Language,
				DataDir:  cfg.Directories.DataDir,
			}
			if ttsCfg.Language == "" {
				ttsCfg.Language = cfg.TTS.Language
			}
			ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
			ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
			ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
			ccCfg := tools.ChromecastConfig{
				ServerHost: cfg.Server.Host,
				ServerPort: cfg.Chromecast.TTSPort,
			}
			return "Tool Output: " + tools.ChromecastSpeak(tc.DeviceAddr, tc.DevicePort, text, ttsCfg, ccCfg, logger)
		case "stop":
			return "Tool Output: " + tools.ChromecastStop(tc.DeviceAddr, tc.DevicePort, logger)
		case "volume":
			return "Tool Output: " + tools.ChromecastVolume(tc.DeviceAddr, tc.DevicePort, tc.Volume, logger)
		case "status":
			return "Tool Output: " + tools.ChromecastStatus(tc.DeviceAddr, tc.DevicePort, logger)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown chromecast operation. Use: discover, play, speak, stop, volume, status"}`
		}

	case "manage_webhooks":
		if !cfg.Webhooks.Enabled {
			return `Tool Output: {"status":"error","message":"Webhooks are not enabled. Set webhooks.enabled: true in config."}`
		}
		if cfg.Webhooks.ReadOnly {
			switch tc.Operation {
			case "create", "update", "delete":
				return `Tool Output: {"status":"error","message":"Webhooks are in read-only mode. Disable webhooks.read_only to allow changes."}`
			}
		}
		whFilePath := filepath.Join(cfg.Directories.DataDir, "webhooks.json")
		whLogPath := filepath.Join(cfg.Directories.DataDir, "webhook_log.json")
		whMgr, whErr := webhooks.NewManager(whFilePath, whLogPath)
		if whErr != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to load webhook manager: %s"}`, whErr)
		}
		return handleWebhookToolCall(tc, whMgr, logger)

	case "proxmox", "proxmox_ve":
		if !cfg.Proxmox.Enabled {
			return `Tool Output: {"status":"error","message":"Proxmox integration is not enabled. Set proxmox.enabled=true in config.yaml."}`
		}
		if cfg.Proxmox.ReadOnly {
			switch tc.Operation {
			case "start", "stop", "shutdown", "reboot", "suspend", "resume", "reset", "create_snapshot", "snapshot":
				return `Tool Output: {"status":"error","message":"Proxmox is in read-only mode. Disable proxmox.read_only to allow changes."}`
			}
		}
		pxCfg := tools.ProxmoxConfig{
			URL:      cfg.Proxmox.URL,
			TokenID:  cfg.Proxmox.TokenID,
			Secret:   cfg.Proxmox.Secret,
			Node:     cfg.Proxmox.Node,
			Insecure: cfg.Proxmox.Insecure,
		}
		node := tc.Hostname
		if node == "" {
			node = tc.Name
		}
		vmid := tc.VMID
		if vmid == "" {
			vmid = tc.ID
		}
		vmType := tc.VMType
		switch tc.Operation {
		case "overview":
			logger.Info("LLM requested Proxmox overview", "node", node)
			return "Tool Output: " + tools.ProxmoxOverview(pxCfg, node)
		case "list_nodes":
			logger.Info("LLM requested Proxmox list_nodes")
			return "Tool Output: " + tools.ProxmoxListNodes(pxCfg)
		case "list_vms", "vms":
			logger.Info("LLM requested Proxmox list_vms", "node", node)
			return "Tool Output: " + tools.ProxmoxListVMs(pxCfg, node)
		case "list_containers", "lxc":
			logger.Info("LLM requested Proxmox list_containers", "node", node)
			return "Tool Output: " + tools.ProxmoxListContainers(pxCfg, node)
		case "status":
			logger.Info("LLM requested Proxmox status", "vmid", vmid, "type", vmType)
			return "Tool Output: " + tools.ProxmoxGetStatus(pxCfg, node, vmType, vmid)
		case "start", "stop", "shutdown", "reboot", "suspend", "resume", "reset":
			logger.Info("LLM requested Proxmox action", "action", tc.Operation, "vmid", vmid)
			return "Tool Output: " + tools.ProxmoxVMAction(pxCfg, node, vmType, vmid, tc.Operation)
		case "node_status":
			logger.Info("LLM requested Proxmox node_status", "node", node)
			return "Tool Output: " + tools.ProxmoxNodeStatus(pxCfg, node)
		case "cluster_resources", "resources":
			resType := tc.ResourceType
			logger.Info("LLM requested Proxmox cluster_resources", "type", resType)
			return "Tool Output: " + tools.ProxmoxClusterResources(pxCfg, resType)
		case "storage":
			logger.Info("LLM requested Proxmox storage", "node", node)
			return "Tool Output: " + tools.ProxmoxGetStorage(pxCfg, node)
		case "create_snapshot", "snapshot":
			logger.Info("LLM requested Proxmox create_snapshot", "vmid", vmid, "name", tc.Name)
			return "Tool Output: " + tools.ProxmoxCreateSnapshot(pxCfg, node, vmType, vmid, tc.Name, tc.Description)
		case "list_snapshots", "snapshots":
			logger.Info("LLM requested Proxmox list_snapshots", "vmid", vmid)
			return "Tool Output: " + tools.ProxmoxListSnapshots(pxCfg, node, vmType, vmid)
		case "task_log":
			upid := tc.UPID
			if upid == "" {
				upid = tc.ID
			}
			logger.Info("LLM requested Proxmox task_log", "upid", upid)
			return "Tool Output: " + tools.ProxmoxGetTaskLog(pxCfg, node, upid)
		default:
			return `Tool Output: {"status":"error","message":"Unknown proxmox operation. Use: overview, list_nodes, list_vms, list_containers, status, start, stop, shutdown, reboot, node_status, cluster_resources, storage, create_snapshot, list_snapshots, task_log"}`
		}

	case "ollama", "ollama_management":
		if !cfg.Ollama.Enabled {
			return `Tool Output: {"status":"error","message":"Ollama integration is not enabled. Set ollama.enabled=true in config.yaml."}`
		}
		if cfg.Ollama.ReadOnly {
			switch tc.Operation {
			case "pull", "download", "delete", "remove", "copy", "load", "unload":
				return `Tool Output: {"status":"error","message":"Ollama is in read-only mode. Disable ollama.read_only to allow changes."}`
			}
		}
		olCfg := tools.OllamaConfig{URL: cfg.Ollama.URL}
		modelName := tc.Model
		if modelName == "" {
			modelName = tc.Name
		}
		switch tc.Operation {
		case "list", "list_models":
			logger.Info("LLM requested Ollama list models")
			return "Tool Output: " + tools.OllamaListModels(olCfg)
		case "running", "ps":
			logger.Info("LLM requested Ollama list running")
			return "Tool Output: " + tools.OllamaListRunning(olCfg)
		case "show", "info":
			logger.Info("LLM requested Ollama show model", "model", modelName)
			return "Tool Output: " + tools.OllamaShowModel(olCfg, modelName)
		case "pull", "download":
			logger.Info("LLM requested Ollama pull model", "model", modelName)
			return "Tool Output: " + tools.OllamaPullModel(olCfg, modelName)
		case "delete", "remove":
			logger.Info("LLM requested Ollama delete model", "model", modelName)
			return "Tool Output: " + tools.OllamaDeleteModel(olCfg, modelName)
		case "copy":
			src := tc.Source
			dst := tc.Destination
			if dst == "" {
				dst = tc.Dest
			}
			logger.Info("LLM requested Ollama copy model", "source", src, "destination", dst)
			return "Tool Output: " + tools.OllamaCopyModel(olCfg, src, dst)
		case "load":
			logger.Info("LLM requested Ollama load model", "model", modelName)
			return "Tool Output: " + tools.OllamaLoadModel(olCfg, modelName)
		case "unload":
			logger.Info("LLM requested Ollama unload model", "model", modelName)
			return "Tool Output: " + tools.OllamaUnloadModel(olCfg, modelName)
		case "container_status":
			logger.Info("LLM requested Ollama managed container status")
			return "Tool Output: " + tools.OllamaManagedContainerStatus(cfg.Docker.Host)
		case "container_start":
			logger.Info("LLM requested Ollama managed container start")
			return "Tool Output: " + tools.StartOllamaManagedContainer(cfg.Docker.Host)
		case "container_stop":
			logger.Info("LLM requested Ollama managed container stop")
			return "Tool Output: " + tools.StopOllamaManagedContainer(cfg.Docker.Host)
		case "container_restart":
			logger.Info("LLM requested Ollama managed container restart")
			return "Tool Output: " + tools.RestartOllamaManagedContainer(cfg.Docker.Host)
		case "container_logs":
			logger.Info("LLM requested Ollama managed container logs")
			return "Tool Output: " + tools.OllamaManagedContainerLogs(cfg.Docker.Host, 100)
		default:
			return `Tool Output: {"status":"error","message":"Unknown ollama operation. Use: list, running, show, pull, delete, copy, load, unload, container_status, container_start, container_stop, container_restart, container_logs"}`
		}

	case "tailscale":
		if !cfg.Tailscale.Enabled {
			return `Tool Output: {"status":"error","message":"Tailscale integration is not enabled. Set tailscale.enabled=true in config.yaml."}`
		}
		if cfg.Tailscale.ReadOnly {
			switch tc.Operation {
			case "enable_routes", "disable_routes":
				return `Tool Output: {"status":"error","message":"Tailscale is in read-only mode. Disable tailscale.read_only to allow changes."}`
			}
		}
		tsCfg := tools.TailscaleConfig{APIKey: cfg.Tailscale.APIKey, Tailnet: cfg.Tailscale.Tailnet}
		// query is hostname, IP, or node ID for device-specific operations
		query := tc.Query
		if query == "" {
			query = tc.Hostname
		}
		if query == "" {
			query = tc.ID
		}
		if query == "" {
			query = tc.Name
		}
		switch tc.Operation {
		case "devices", "list", "list_devices":
			logger.Info("LLM requested Tailscale list devices")
			return "Tool Output: " + tools.TailscaleListDevices(tsCfg)
		case "device", "get", "get_device":
			logger.Info("LLM requested Tailscale get device", "query", query)
			return "Tool Output: " + tools.TailscaleGetDevice(tsCfg, query)
		case "routes", "get_routes":
			logger.Info("LLM requested Tailscale get routes", "query", query)
			return "Tool Output: " + tools.TailscaleGetRoutes(tsCfg, query)
		case "enable_routes":
			routes := splitCSV(tc.Value)
			logger.Info("LLM requested Tailscale enable routes", "query", query, "routes", routes)
			return "Tool Output: " + tools.TailscaleSetRoutes(tsCfg, query, routes, true)
		case "disable_routes":
			routes := splitCSV(tc.Value)
			logger.Info("LLM requested Tailscale disable routes", "query", query, "routes", routes)
			return "Tool Output: " + tools.TailscaleSetRoutes(tsCfg, query, routes, false)
		case "dns", "get_dns":
			logger.Info("LLM requested Tailscale DNS config")
			return "Tool Output: " + tools.TailscaleGetDNS(tsCfg)
		case "acl", "get_acl":
			logger.Info("LLM requested Tailscale ACL policy")
			return "Tool Output: " + tools.TailscaleGetACL(tsCfg)
		case "local_status", "status":
			logger.Info("LLM requested Tailscale local status")
			return "Tool Output: " + tools.TailscaleLocalStatus()
		default:
			return `Tool Output: {"status":"error","message":"Unknown tailscale operation. Use: devices, device, routes, enable_routes, disable_routes, dns, acl, local_status"}`
		}

	case "cloudflare_tunnel":
		if !cfg.CloudflareTunnel.Enabled {
			return `Tool Output: {"status":"error","message":"Cloudflare Tunnel integration is not enabled. Set cloudflare_tunnel.enabled=true in config.yaml."}`
		}
		if cfg.CloudflareTunnel.ReadOnly {
			switch tc.Operation {
			case "start", "stop", "restart", "install":
				return `Tool Output: {"status":"error","message":"Cloudflare Tunnel is in read-only mode. Disable cloudflare_tunnel.readonly to allow changes."}`
			}
		}
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
		switch tc.Operation {
		case "start":
			logger.Info("LLM requested Cloudflare Tunnel start")
			return "Tool Output: " + tools.CloudflareTunnelStart(tunnelCfg, vault, registry, logger)
		case "stop":
			logger.Info("LLM requested Cloudflare Tunnel stop")
			return "Tool Output: " + tools.CloudflareTunnelStop(tunnelCfg, registry, logger)
		case "restart":
			logger.Info("LLM requested Cloudflare Tunnel restart")
			return "Tool Output: " + tools.CloudflareTunnelRestart(tunnelCfg, vault, registry, logger)
		case "status":
			logger.Info("LLM requested Cloudflare Tunnel status")
			return "Tool Output: " + tools.CloudflareTunnelStatus(tunnelCfg, registry, logger)
		case "quick_tunnel":
			port := tc.Port
			logger.Info("LLM requested Cloudflare quick tunnel", "port", port)
			return "Tool Output: " + tools.CloudflareTunnelQuickTunnel(tunnelCfg, registry, logger, port)
		case "logs":
			logger.Info("LLM requested Cloudflare Tunnel logs")
			return "Tool Output: " + tools.CloudflareTunnelLogs(registry, logger)
		case "list_routes":
			logger.Info("LLM requested Cloudflare Tunnel list routes")
			return "Tool Output: " + tools.CloudflareTunnelListRoutes(tunnelCfg, logger)
		case "install":
			logger.Info("LLM requested Cloudflare Tunnel install binary")
			return "Tool Output: " + tools.CloudflareTunnelInstall(tunnelCfg, logger)
		default:
			return `Tool Output: {"status":"error","message":"Unknown cloudflare_tunnel operation. Use: start, stop, restart, status, quick_tunnel, logs, list_routes, install"}`
		}

	case "ansible":
		if !cfg.Ansible.Enabled {
			return `Tool Output: {"status":"error","message":"Ansible integration is not enabled. Set ansible.enabled=true in config.yaml."}`
		}
		if cfg.Ansible.ReadOnly {
			switch tc.Operation {
			case "adhoc", "command", "run_module", "playbook", "run", "run_playbook":
				return `Tool Output: {"status":"error","message":"Ansible is in read-only mode. Disable ansible.read_only to allow changes."}`
			}
		}
		// Resolve host pattern (hosts for ad-hoc / limit for playbooks)
		hosts := tc.Hostname
		if hosts == "" {
			hosts = tc.HostLimit
		}
		if hosts == "" {
			hosts = tc.Query
		}
		inventoryPath := tc.Inventory
		// Parse extra_vars from tc.Body (JSON string → map)
		var extraVars map[string]interface{}
		if tc.Body != "" {
			_ = json.Unmarshal([]byte(tc.Body), &extraVars)
		}

		isLocal := cfg.Ansible.Mode == "local"

		if isLocal {
			// ── Local CLI mode ──────────────────────────────────────────────────────
			localCfg := tools.AnsibleLocalConfig{
				PlaybooksDir:     cfg.Ansible.PlaybooksDir,
				DefaultInventory: cfg.Ansible.DefaultInventory,
				Timeout:          cfg.Ansible.Timeout,
			}
			switch tc.Operation {
			case "status", "health":
				logger.Info("LLM requested Ansible status (local)")
				return "Tool Output: " + tools.AnsibleLocalStatus(localCfg)
			case "list_playbooks", "playbooks":
				logger.Info("LLM requested Ansible list playbooks (local)")
				return "Tool Output: " + tools.AnsibleLocalListPlaybooks(localCfg)
			case "inventory", "list_inventory":
				logger.Info("LLM requested Ansible inventory (local)", "path", inventoryPath)
				return "Tool Output: " + tools.AnsibleLocalListInventory(localCfg, inventoryPath)
			case "ping":
				logger.Info("LLM requested Ansible ping (local)", "hosts", hosts)
				return "Tool Output: " + tools.AnsibleLocalPing(localCfg, hosts, inventoryPath)
			case "adhoc", "command", "run_module":
				module := tc.Module
				if module == "" {
					module = tc.Package
				}
				moduleArgs := tc.Command
				logger.Info("LLM requested Ansible adhoc (local)", "hosts", hosts, "module", module)
				return "Tool Output: " + tools.AnsibleLocalAdhoc(localCfg, hosts, module, moduleArgs, inventoryPath, extraVars)
			case "playbook", "run", "run_playbook":
				playbook := tc.Name
				if playbook == "" {
					return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=playbook"}`
				}
				logger.Info("LLM requested Ansible playbook (local)", "playbook", playbook, "limit", tc.HostLimit)
				return "Tool Output: " + tools.AnsibleLocalRunPlaybook(localCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, tc.Preview, false)
			case "check", "dry_run":
				playbook := tc.Name
				if playbook == "" {
					return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=check"}`
				}
				logger.Info("LLM requested Ansible playbook dry-run (local)", "playbook", playbook)
				return "Tool Output: " + tools.AnsibleLocalRunPlaybook(localCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, true, true)
			case "facts", "gather_facts":
				logger.Info("LLM requested Ansible gather facts (local)", "hosts", hosts)
				return "Tool Output: " + tools.AnsibleLocalGatherFacts(localCfg, hosts, inventoryPath)
			default:
				return `Tool Output: {"status":"error","message":"Unknown ansible operation. Use: status, list_playbooks, inventory, ping, adhoc, playbook, check, facts"}`
			}
		}

		// ── Sidecar mode (default) ──────────────────────────────────────────────
		ansCfg := tools.AnsibleConfig{
			URL:     cfg.Ansible.URL,
			Token:   cfg.Ansible.Token,
			Timeout: cfg.Ansible.Timeout,
		}
		switch tc.Operation {
		case "status", "health":
			logger.Info("LLM requested Ansible status")
			return "Tool Output: " + tools.AnsibleStatus(ansCfg)
		case "list_playbooks", "playbooks":
			logger.Info("LLM requested Ansible list playbooks")
			return "Tool Output: " + tools.AnsibleListPlaybooks(ansCfg)
		case "inventory", "list_inventory":
			logger.Info("LLM requested Ansible inventory", "path", inventoryPath)
			return "Tool Output: " + tools.AnsibleListInventory(ansCfg, inventoryPath)
		case "ping":
			logger.Info("LLM requested Ansible ping", "hosts", hosts)
			return "Tool Output: " + tools.AnsiblePing(ansCfg, hosts, inventoryPath)
		case "adhoc", "command", "run_module":
			module := tc.Module
			if module == "" {
				module = tc.Package
			}
			moduleArgs := tc.Command
			logger.Info("LLM requested Ansible adhoc", "hosts", hosts, "module", module)
			return "Tool Output: " + tools.AnsibleAdhoc(ansCfg, hosts, module, moduleArgs, inventoryPath, extraVars)
		case "playbook", "run", "run_playbook":
			playbook := tc.Name
			if playbook == "" {
				return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=playbook"}`
			}
			logger.Info("LLM requested Ansible playbook", "playbook", playbook, "limit", tc.HostLimit, "check", tc.Preview)
			return "Tool Output: " + tools.AnsibleRunPlaybook(ansCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, tc.Preview, false)
		case "check", "dry_run":
			playbook := tc.Name
			if playbook == "" {
				return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=check"}`
			}
			logger.Info("LLM requested Ansible playbook dry-run", "playbook", playbook)
			return "Tool Output: " + tools.AnsibleRunPlaybook(ansCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, true, true)
		case "facts", "gather_facts":
			logger.Info("LLM requested Ansible gather facts", "hosts", hosts)
			return "Tool Output: " + tools.AnsibleGatherFacts(ansCfg, hosts, inventoryPath)
		default:
			return `Tool Output: {"status":"error","message":"Unknown ansible operation. Use: status, list_playbooks, inventory, ping, adhoc, playbook, check, facts"}`
		}

	case "invasion_control":
		return handleInvasionControl(tc, cfg, invasionDB, vault, logger)

	case "remote_control":
		return handleRemoteControl(tc, cfg, remoteHub, logger)

	case "github":
		if !cfg.GitHub.Enabled {
			return `Tool Output: {"status":"error","message":"GitHub integration is not enabled. Set github.enabled=true in config.yaml."}`
		}
		if cfg.GitHub.ReadOnly {
			switch tc.Operation {
			case "create_repo", "delete_repo", "create_issue", "close_issue", "create_or_update_file", "track_project", "untrack_project":
				return `Tool Output: {"status":"error","message":"GitHub is in read-only mode. Disable github.read_only to allow changes."}`
			}
		}
		token, err := vault.ReadSecret("github_token")
		if err != nil || token == "" {
			return `Tool Output: {"status":"error","message":"GitHub token not found in vault. Store it with key 'github_token' via the vault API."}`
		}

		// Allowed-repos enforcement: if a list is configured the agent may only access
		// repos that are explicitly allowed OR repos it created itself (tracked projects).
		if len(cfg.GitHub.AllowedRepos) > 0 {
			repoArg := tc.Name
			repoOpsNeedCheck := map[string]bool{
				"delete_repo": true, "get_repo": true, "list_issues": true,
				"create_issue": true, "close_issue": true, "list_pull_requests": true,
				"list_branches": true, "get_file": true, "create_or_update_file": true,
				"list_commits": true, "list_workflow_runs": true,
			}
			if repoArg != "" && repoOpsNeedCheck[tc.Operation] {
				allowedMap := map[string]bool{}
				for _, r := range cfg.GitHub.AllowedRepos {
					allowedMap[r] = true
				}
				// Agent-created repos (tracked in workspace) are always permitted
				isTracked := false
				trackedRaw := tools.GitHubListProjects(cfg.Directories.WorkspaceDir)
				var trackedResult map[string]interface{}
				if jsonErr := json.Unmarshal([]byte(trackedRaw), &trackedResult); jsonErr == nil {
					if projects, ok := trackedResult["projects"].([]interface{}); ok {
						for _, p := range projects {
							if pm, ok := p.(map[string]interface{}); ok {
								if name, _ := pm["name"].(string); name == repoArg {
									isTracked = true
									break
								}
							}
						}
					}
				}
				if !allowedMap[repoArg] && !isTracked {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Repo '%s' is not in the allowed repos list. Add it in Settings → GitHub to grant access."}`, repoArg)
				}
			}
		}

		ghCfg := tools.GitHubConfig{
			Token:          token,
			Owner:          cfg.GitHub.Owner,
			BaseURL:        cfg.GitHub.BaseURL,
			DefaultPrivate: cfg.GitHub.DefaultPrivate,
		}
		owner := tc.Owner
		if owner == "" {
			owner = cfg.GitHub.Owner
		}
		repo := tc.Name
		switch tc.Operation {
		case "list_repos":
			logger.Info("LLM requested GitHub list repos", "owner", owner)
			return "Tool Output: " + tools.GitHubListRepos(ghCfg, owner)
		case "create_repo":
			logger.Info("LLM requested GitHub create repo", "name", repo, "desc", tc.Description)
			return "Tool Output: " + tools.GitHubCreateRepo(ghCfg, repo, tc.Description, nil)
		case "delete_repo":
			logger.Info("LLM requested GitHub delete repo", "owner", owner, "repo", repo)
			return "Tool Output: " + tools.GitHubDeleteRepo(ghCfg, owner, repo)
		case "get_repo":
			logger.Info("LLM requested GitHub get repo", "owner", owner, "repo", repo)
			return "Tool Output: " + tools.GitHubGetRepo(ghCfg, owner, repo)
		case "list_issues":
			state := tc.Value
			if state == "" {
				state = "open"
			}
			logger.Info("LLM requested GitHub list issues", "repo", repo, "state", state)
			return "Tool Output: " + tools.GitHubListIssues(ghCfg, owner, repo, state)
		case "create_issue":
			var labels []string
			if tc.Label != "" {
				labels = splitCSV(tc.Label)
			}
			logger.Info("LLM requested GitHub create issue", "repo", repo, "title", tc.Title)
			return "Tool Output: " + tools.GitHubCreateIssue(ghCfg, owner, repo, tc.Title, tc.Body, labels)
		case "close_issue":
			issueNum := 0
			if tc.ID != "" {
				fmt.Sscanf(tc.ID, "%d", &issueNum)
			}
			logger.Info("LLM requested GitHub close issue", "repo", repo, "number", issueNum)
			return "Tool Output: " + tools.GitHubCloseIssue(ghCfg, owner, repo, issueNum)
		case "list_pull_requests":
			state := tc.Value
			if state == "" {
				state = "open"
			}
			logger.Info("LLM requested GitHub list PRs", "repo", repo, "state", state)
			return "Tool Output: " + tools.GitHubListPullRequests(ghCfg, owner, repo, state)
		case "list_branches":
			logger.Info("LLM requested GitHub list branches", "repo", repo)
			return "Tool Output: " + tools.GitHubListBranches(ghCfg, owner, repo)
		case "get_file":
			branch := tc.Query
			logger.Info("LLM requested GitHub get file", "repo", repo, "path", tc.Path, "branch", branch)
			return "Tool Output: " + tools.GitHubGetFileContent(ghCfg, owner, repo, tc.Path, branch)
		case "create_or_update_file":
			logger.Info("LLM requested GitHub create/update file", "repo", repo, "path", tc.Path)
			return "Tool Output: " + tools.GitHubCreateOrUpdateFile(ghCfg, owner, repo, tc.Path, tc.Content, tc.Body, tc.Value, tc.Query)
		case "list_commits":
			branch := tc.Query
			limit := tc.Limit
			if limit <= 0 {
				limit = 20
			}
			logger.Info("LLM requested GitHub list commits", "repo", repo, "branch", branch)
			return "Tool Output: " + tools.GitHubListCommits(ghCfg, owner, repo, branch, limit)
		case "list_workflow_runs":
			limit := tc.Limit
			if limit <= 0 {
				limit = 10
			}
			logger.Info("LLM requested GitHub list workflow runs", "repo", repo)
			return "Tool Output: " + tools.GitHubListWorkflowRuns(ghCfg, owner, repo, limit)
		case "search_repos":
			limit := tc.Limit
			if limit <= 0 {
				limit = 10
			}
			logger.Info("LLM requested GitHub search repos", "query", tc.Query)
			return "Tool Output: " + tools.GitHubSearchRepos(ghCfg, tc.Query, limit)
		case "list_projects":
			logger.Info("LLM requested GitHub list tracked projects")
			return "Tool Output: " + tools.GitHubListProjects(cfg.Directories.WorkspaceDir)
		case "track_project":
			purpose := tc.Content
			if purpose == "" {
				purpose = tc.Description
			}
			logger.Info("LLM requested GitHub track project", "name", repo, "purpose", purpose)
			return "Tool Output: " + tools.GitHubTrackProject(cfg.Directories.WorkspaceDir, repo, purpose, "", "", owner, cfg.GitHub.DefaultPrivate)
		case "untrack_project":
			logger.Info("LLM requested GitHub untrack project", "name", repo)
			return "Tool Output: " + tools.GitHubUntrackProject(cfg.Directories.WorkspaceDir, repo)
		default:
			return `Tool Output: {"status":"error","message":"Unknown github operation. Use: list_repos, create_repo, delete_repo, get_repo, list_issues, create_issue, close_issue, list_pull_requests, list_branches, get_file, create_or_update_file, list_commits, list_workflow_runs, search_repos, list_projects, track_project, untrack_project"}`
		}

	case "netlify":
		if !cfg.Netlify.Enabled {
			return `Tool Output: {"status":"error","message":"Netlify integration is not enabled. Set netlify.enabled=true in config.yaml."}`
		}
		token, tokenErr := vault.ReadSecret("netlify_token")
		if tokenErr != nil || token == "" {
			return `Tool Output: {"status":"error","message":"Netlify token not found in vault. Store it with key 'netlify_token' via the vault API."}`
		}
		nfCfg := tools.NetlifyConfig{
			Token:         token,
			DefaultSiteID: cfg.Netlify.DefaultSiteID,
			TeamSlug:      cfg.Netlify.TeamSlug,
		}
		// Read-only mode: block all mutating operations
		if cfg.Netlify.ReadOnly {
			switch tc.Operation {
			case "create_site", "update_site", "delete_site",
				"deploy_zip", "deploy_draft", "rollback", "cancel_deploy",
				"set_env", "delete_env",
				"create_hook", "delete_hook",
				"provision_ssl":
				return `Tool Output: {"status":"error","message":"Netlify is in read-only mode. Disable netlify.readonly to allow changes."}`
			}
		}
		// Granular permission checks
		if !cfg.Netlify.AllowDeploy {
			switch tc.Operation {
			case "deploy_zip", "deploy_draft", "rollback", "cancel_deploy":
				return `Tool Output: {"status":"error","message":"Netlify deploy is not allowed. Set netlify.allow_deploy=true in config.yaml."}`
			}
		}
		if !cfg.Netlify.AllowSiteManagement {
			switch tc.Operation {
			case "create_site", "update_site", "delete_site":
				return `Tool Output: {"status":"error","message":"Netlify site management is not allowed. Set netlify.allow_site_management=true in config.yaml."}`
			}
		}
		if !cfg.Netlify.AllowEnvManagement {
			switch tc.Operation {
			case "set_env", "delete_env":
				return `Tool Output: {"status":"error","message":"Netlify env var management is not allowed. Set netlify.allow_env_management=true in config.yaml."}`
			}
		}
		switch tc.Operation {
		// ── Sites ──
		case "list_sites":
			logger.Info("LLM requested Netlify list sites")
			return "Tool Output: " + tools.NetlifyListSites(nfCfg)
		case "get_site":
			logger.Info("LLM requested Netlify get site", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyGetSite(nfCfg, tc.SiteID)
		case "create_site":
			logger.Info("LLM requested Netlify create site", "name", tc.SiteName, "custom_domain", tc.CustomDomain)
			return "Tool Output: " + tools.NetlifyCreateSite(nfCfg, tc.SiteName, tc.CustomDomain)
		case "update_site":
			logger.Info("LLM requested Netlify update site", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyUpdateSite(nfCfg, tc.SiteID, tc.SiteName, tc.CustomDomain)
		case "delete_site":
			logger.Info("LLM requested Netlify delete site", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyDeleteSite(nfCfg, tc.SiteID)
		// ── Deploys ──
		case "list_deploys":
			logger.Info("LLM requested Netlify list deploys", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListDeploys(nfCfg, tc.SiteID)
		case "get_deploy":
			logger.Info("LLM requested Netlify get deploy", "deploy_id", tc.DeployID)
			return "Tool Output: " + tools.NetlifyGetDeploy(nfCfg, tc.DeployID)
		case "deploy_zip":
			logger.Info("LLM requested Netlify deploy ZIP", "site_id", tc.SiteID, "draft", tc.Draft)
			if tc.Content == "" {
				return `Tool Output: {"status":"error","message":"content (base64 ZIP) is required for deploy_zip."}`
			}
			zipData, decErr := decodeBase64(tc.Content)
			if decErr != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to decode base64 ZIP: %v"}`, decErr)
			}
			return "Tool Output: " + tools.NetlifyDeployZip(nfCfg, tc.SiteID, tc.Title, tc.Draft, zipData)
		case "deploy_draft":
			logger.Info("LLM requested Netlify draft deploy", "site_id", tc.SiteID)
			if tc.Content == "" {
				return `Tool Output: {"status":"error","message":"content (base64 ZIP) is required for deploy_draft."}`
			}
			zipData, decErr := decodeBase64(tc.Content)
			if decErr != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to decode base64 ZIP: %v"}`, decErr)
			}
			return "Tool Output: " + tools.NetlifyDeployZip(nfCfg, tc.SiteID, tc.Title, true, zipData)
		case "rollback":
			logger.Info("LLM requested Netlify rollback", "site_id", tc.SiteID, "deploy_id", tc.DeployID)
			return "Tool Output: " + tools.NetlifyRollback(nfCfg, tc.SiteID, tc.DeployID)
		case "cancel_deploy":
			logger.Info("LLM requested Netlify cancel deploy", "deploy_id", tc.DeployID)
			return "Tool Output: " + tools.NetlifyCancelDeploy(nfCfg, tc.DeployID)
		// ── Environment Variables ──
		case "list_env":
			logger.Info("LLM requested Netlify list env vars", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListEnvVars(nfCfg, tc.SiteID)
		case "get_env":
			logger.Info("LLM requested Netlify get env var", "site_id", tc.SiteID, "key", tc.EnvKey)
			return "Tool Output: " + tools.NetlifyGetEnvVar(nfCfg, tc.SiteID, tc.EnvKey)
		case "set_env":
			logger.Info("LLM requested Netlify set env var", "site_id", tc.SiteID, "key", tc.EnvKey)
			return "Tool Output: " + tools.NetlifySetEnvVar(nfCfg, tc.SiteID, tc.EnvKey, tc.EnvValue, tc.EnvContext)
		case "delete_env":
			logger.Info("LLM requested Netlify delete env var", "site_id", tc.SiteID, "key", tc.EnvKey)
			return "Tool Output: " + tools.NetlifyDeleteEnvVar(nfCfg, tc.SiteID, tc.EnvKey)
		// ── Files ──
		case "list_files":
			logger.Info("LLM requested Netlify list files", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListFiles(nfCfg, tc.SiteID)
		// ── Forms ──
		case "list_forms":
			logger.Info("LLM requested Netlify list forms", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListForms(nfCfg, tc.SiteID)
		case "get_submissions":
			logger.Info("LLM requested Netlify get form submissions", "form_id", tc.FormID)
			return "Tool Output: " + tools.NetlifyGetFormSubmissions(nfCfg, tc.FormID)
		// ── Hooks ──
		case "list_hooks":
			logger.Info("LLM requested Netlify list hooks", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListHooks(nfCfg, tc.SiteID)
		case "create_hook":
			logger.Info("LLM requested Netlify create hook", "site_id", tc.SiteID, "type", tc.HookType, "event", tc.HookEvent)
			hookData := map[string]interface{}{}
			if tc.URL != "" {
				hookData["url"] = tc.URL
			}
			if tc.Value != "" {
				hookData["email"] = tc.Value
			}
			return "Tool Output: " + tools.NetlifyCreateHook(nfCfg, tc.SiteID, tc.HookType, tc.HookEvent, hookData)
		case "delete_hook":
			logger.Info("LLM requested Netlify delete hook", "hook_id", tc.HookID)
			return "Tool Output: " + tools.NetlifyDeleteHook(nfCfg, tc.HookID)
		// ── SSL ──
		case "provision_ssl":
			logger.Info("LLM requested Netlify provision SSL", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyProvisionSSL(nfCfg, tc.SiteID)
		// ── Diagnostics ──
		case "check_connection":
			logger.Info("LLM requested Netlify connection check")
			return "Tool Output: " + tools.NetlifyTestConnection(nfCfg)
		default:
			return `Tool Output: {"status":"error","message":"Unknown netlify operation. Use: list_sites, get_site, create_site, update_site, delete_site, list_deploys, get_deploy, deploy_zip, deploy_draft, rollback, cancel_deploy, list_env, get_env, set_env, delete_env, list_files, list_forms, get_submissions, list_hooks, create_hook, delete_hook, provision_ssl, check_connection"}`
		}

	case "mqtt_publish":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		if cfg.MQTT.ReadOnly {
			return `Tool Output: {"status":"error","message":"MQTT is in read-only mode. Disable mqtt.read_only to allow changes."}`
		}
		topic := tc.Topic
		if topic == "" {
			return `Tool Output: {"status": "error", "message": "'topic' is required"}`
		}
		payload := tc.Payload
		if payload == "" {
			payload = tc.Message
		}
		if payload == "" {
			payload = tc.Content
		}
		qos := tc.QoS
		if qos < 0 || qos > 2 {
			qos = cfg.MQTT.QoS
		}
		logger.Info("LLM requested MQTT publish", "topic", topic, "retain", tc.Retain, "payload_len", len(payload))
		if err := tools.MQTTPublish(topic, payload, qos, tc.Retain, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT publish failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Published to topic '%s'"}`, topic)

	case "mqtt_subscribe":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		topic := tc.Topic
		if topic == "" {
			return `Tool Output: {"status": "error", "message": "'topic' is required"}`
		}
		qos := tc.QoS
		if qos < 0 || qos > 2 {
			qos = cfg.MQTT.QoS
		}
		logger.Info("LLM requested MQTT subscribe", "topic", topic, "qos", qos)
		if err := tools.MQTTSubscribe(topic, qos, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT subscribe failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Subscribed to topic '%s' with QoS %d"}`, topic, qos)

	case "mqtt_unsubscribe":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		topic := tc.Topic
		if topic == "" {
			return `Tool Output: {"status": "error", "message": "'topic' is required"}`
		}
		logger.Info("LLM requested MQTT unsubscribe", "topic", topic)
		if err := tools.MQTTUnsubscribe(topic, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT unsubscribe failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Unsubscribed from topic '%s'"}`, topic)

	case "mqtt_get_messages":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		topic := tc.Topic // empty = all topics
		limit := tc.Limit
		if limit <= 0 {
			limit = 20
		}
		logger.Info("LLM requested MQTT get messages", "topic", topic, "limit", limit)
		msgs, err := tools.MQTTGetMessages(topic, limit, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT get messages failed: %v"}`, err)
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(msgs),
			"data":   msgs,
		})
		return "Tool Output: " + string(data)

	case "mcp_call":
		// Two-gate security: allow_mcp (Danger Zone) AND mcp.enabled
		if !cfg.Agent.AllowMCP {
			return `Tool Output: [PERMISSION DENIED] MCP is disabled in Danger Zone settings (agent.allow_mcp: false).`
		}
		if !cfg.MCP.Enabled {
			return `Tool Output: [PERMISSION DENIED] MCP is disabled (mcp.enabled: false).`
		}

		op := strings.ToLower(strings.TrimSpace(tc.Operation))
		switch op {
		case "list_servers":
			servers, err := tools.MCPListServers(logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP list servers failed: %v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "success", "servers": servers})
			return "Tool Output: " + string(data)

		case "list_tools":
			mcpTools, err := tools.MCPListTools(tc.Server, logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP list tools failed: %v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "success", "tools": mcpTools})
			return "Tool Output: " + string(data)

		case "call_tool", "call":
			if tc.Server == "" || tc.ToolName == "" {
				return `Tool Output: {"status": "error", "message": "mcp_call with operation=call requires 'server' and 'tool_name'"}`
			}
			args := tc.MCPArgs
			if args == nil {
				args = map[string]interface{}{}
			}
			result, err := tools.MCPCallTool(tc.Server, tc.ToolName, args, logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP call failed: %v"}`, err)
			}
			return "Tool Output: " + result

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "unknown mcp_call operation '%s'. Use list_servers, list_tools, or call_tool."}`, op)
		}

	case "adguard", "adguard_home":
		if !cfg.AdGuard.Enabled {
			return `Tool Output: {"status":"error","message":"AdGuard Home is not enabled. Configure the adguard section in config.yaml."}`
		}
		adgCfg := tools.AdGuardConfig{
			URL:      cfg.AdGuard.URL,
			Username: cfg.AdGuard.Username,
			Password: cfg.AdGuard.Password,
		}
		op := strings.ToLower(strings.TrimSpace(tc.Operation))

		// Read-only operations
		switch op {
		case "status":
			logger.Info("LLM requested AdGuard status")
			return "Tool Output: " + tools.AdGuardStatus(adgCfg)
		case "stats":
			logger.Info("LLM requested AdGuard stats")
			return "Tool Output: " + tools.AdGuardStats(adgCfg)
		case "stats_top":
			logger.Info("LLM requested AdGuard top stats")
			return "Tool Output: " + tools.AdGuardStatsTop(adgCfg)
		case "query_log":
			logger.Info("LLM requested AdGuard query log", "search", tc.Query, "limit", tc.Limit)
			return "Tool Output: " + tools.AdGuardQueryLog(adgCfg, tc.Query, tc.Limit, tc.Offset)
		case "filtering_status":
			logger.Info("LLM requested AdGuard filtering status")
			return "Tool Output: " + tools.AdGuardFilteringStatus(adgCfg)
		case "rewrite_list":
			logger.Info("LLM requested AdGuard rewrite list")
			return "Tool Output: " + tools.AdGuardRewriteList(adgCfg)
		case "blocked_services_list":
			logger.Info("LLM requested AdGuard blocked services list")
			return "Tool Output: " + tools.AdGuardBlockedServicesList(adgCfg)
		case "safebrowsing_status":
			logger.Info("LLM requested AdGuard safe browsing status")
			return "Tool Output: " + tools.AdGuardSafeBrowsingStatus(adgCfg)
		case "parental_status":
			logger.Info("LLM requested AdGuard parental status")
			return "Tool Output: " + tools.AdGuardParentalStatus(adgCfg)
		case "dhcp_status":
			logger.Info("LLM requested AdGuard DHCP status")
			return "Tool Output: " + tools.AdGuardDHCPStatus(adgCfg)
		case "clients":
			logger.Info("LLM requested AdGuard clients")
			return "Tool Output: " + tools.AdGuardClients(adgCfg)
		case "dns_info":
			logger.Info("LLM requested AdGuard DNS info")
			return "Tool Output: " + tools.AdGuardDNSInfo(adgCfg)
		case "test_upstream":
			logger.Info("LLM requested AdGuard test upstream", "servers", tc.Services)
			return "Tool Output: " + tools.AdGuardTestUpstream(adgCfg, tc.Services)
		}

		// Write operations — check readonly
		if cfg.AdGuard.ReadOnly {
			return `Tool Output: {"status":"error","message":"AdGuard Home is in read-only mode. Disable adguard.readonly to allow changes."}`
		}
		switch op {
		case "query_log_clear":
			logger.Info("LLM requested AdGuard query log clear")
			return "Tool Output: " + tools.AdGuardQueryLogClear(adgCfg)
		case "filtering_toggle":
			logger.Info("LLM requested AdGuard filtering toggle", "enabled", tc.Enabled)
			return "Tool Output: " + tools.AdGuardFilteringToggle(adgCfg, tc.Enabled)
		case "filtering_add_url":
			logger.Info("LLM requested AdGuard add filter URL", "url", tc.URL)
			return "Tool Output: " + tools.AdGuardFilteringAddURL(adgCfg, tc.Name, tc.URL)
		case "filtering_remove_url":
			logger.Info("LLM requested AdGuard remove filter URL", "url", tc.URL)
			return "Tool Output: " + tools.AdGuardFilteringRemoveURL(adgCfg, tc.URL)
		case "filtering_refresh":
			logger.Info("LLM requested AdGuard filtering refresh")
			return "Tool Output: " + tools.AdGuardFilteringRefresh(adgCfg)
		case "filtering_set_rules":
			logger.Info("LLM requested AdGuard set filtering rules")
			return "Tool Output: " + tools.AdGuardFilteringSetRules(adgCfg, tc.Rules)
		case "rewrite_add":
			logger.Info("LLM requested AdGuard add rewrite", "domain", tc.Domain, "answer", tc.Answer)
			return "Tool Output: " + tools.AdGuardRewriteAdd(adgCfg, tc.Domain, tc.Answer)
		case "rewrite_delete":
			logger.Info("LLM requested AdGuard delete rewrite", "domain", tc.Domain, "answer", tc.Answer)
			return "Tool Output: " + tools.AdGuardRewriteDelete(adgCfg, tc.Domain, tc.Answer)
		case "blocked_services_set":
			logger.Info("LLM requested AdGuard set blocked services", "services", tc.Services)
			return "Tool Output: " + tools.AdGuardBlockedServicesSet(adgCfg, tc.Services)
		case "safebrowsing_toggle":
			logger.Info("LLM requested AdGuard safe browsing toggle", "enabled", tc.Enabled)
			return "Tool Output: " + tools.AdGuardSafeBrowsingToggle(adgCfg, tc.Enabled)
		case "parental_toggle":
			logger.Info("LLM requested AdGuard parental toggle", "enabled", tc.Enabled)
			return "Tool Output: " + tools.AdGuardParentalToggle(adgCfg, tc.Enabled)
		case "dhcp_set_config":
			logger.Info("LLM requested AdGuard DHCP set config")
			return "Tool Output: " + tools.AdGuardDHCPSetConfig(adgCfg, tc.Content)
		case "dhcp_add_lease":
			logger.Info("LLM requested AdGuard DHCP add lease", "mac", tc.MAC, "ip", tc.IP)
			return "Tool Output: " + tools.AdGuardDHCPAddLease(adgCfg, tc.MAC, tc.IP, tc.Hostname)
		case "dhcp_remove_lease":
			logger.Info("LLM requested AdGuard DHCP remove lease", "mac", tc.MAC, "ip", tc.IP)
			return "Tool Output: " + tools.AdGuardDHCPRemoveLease(adgCfg, tc.MAC, tc.IP, tc.Hostname)
		case "client_add":
			logger.Info("LLM requested AdGuard client add")
			return "Tool Output: " + tools.AdGuardClientAdd(adgCfg, tc.Content)
		case "client_update":
			logger.Info("LLM requested AdGuard client update")
			return "Tool Output: " + tools.AdGuardClientUpdate(adgCfg, tc.Content)
		case "client_delete":
			logger.Info("LLM requested AdGuard client delete", "name", tc.Name)
			return "Tool Output: " + tools.AdGuardClientDelete(adgCfg, tc.Name)
		case "dns_config":
			logger.Info("LLM requested AdGuard DNS config update")
			return "Tool Output: " + tools.AdGuardDNSConfig(adgCfg, tc.Content)
		default:
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown adguard operation '%s'. Use: status, stats, stats_top, query_log, query_log_clear, filtering_status, filtering_toggle, filtering_add_url, filtering_remove_url, filtering_refresh, filtering_set_rules, rewrite_list, rewrite_add, rewrite_delete, blocked_services_list, blocked_services_set, safebrowsing_status, safebrowsing_toggle, parental_status, parental_toggle, dhcp_status, dhcp_set_config, dhcp_add_lease, dhcp_remove_lease, clients, client_add, client_update, client_delete, dns_info, dns_config, test_upstream"}`, op)
		}

	case "fritzbox", "fritzbox_system", "fritzbox_network", "fritzbox_telephony", "fritzbox_smarthome", "fritzbox_storage", "fritzbox_tv":
		if !cfg.FritzBox.Enabled {
			return `Tool Output: {"status":"error","message":"Fritz!Box integration is not enabled. Set fritzbox.enabled=true in config.yaml."}`
		}
		fbClient, fbErr := fritzbox.NewClient(*cfg)
		if fbErr != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Fritz!Box client init failed: %s"}`, fbErr)
		}
		defer fbClient.Close()
		return handleFritzBoxToolCall(tc, fbClient, cfg, logger)

	// ── TrueNAS Storage Management ──
	case "truenas_health", "truenas_pool_list", "truenas_pool_scrub",
		"truenas_dataset_list", "truenas_dataset_create", "truenas_dataset_delete",
		"truenas_snapshot_list", "truenas_snapshot_create", "truenas_snapshot_delete", "truenas_snapshot_rollback",
		"truenas_smb_list", "truenas_smb_create", "truenas_smb_delete",
		"truenas_fs_space":
		if !cfg.TrueNAS.Enabled {
			return `Tool Output: {"status":"error","message":"TrueNAS integration is not enabled. Set truenas.enabled=true in config.yaml."}`
		}
		// Read-only check for mutating operations
		if cfg.TrueNAS.ReadOnly {
			switch tc.Action {
			case "truenas_pool_scrub",
				"truenas_dataset_create", "truenas_dataset_delete",
				"truenas_snapshot_create", "truenas_snapshot_delete", "truenas_snapshot_rollback",
				"truenas_smb_create", "truenas_smb_delete":
				return `Tool Output: {"status":"error","message":"TrueNAS is in read-only mode. Disable truenas.readonly to allow changes."}`
			}
		}
		logger.Info("LLM requested TrueNAS operation", "action", tc.Action)
		return "Tool Output: " + tools.DispatchTrueNASTool(tc.Action, toolCallParams(tc), cfg, nil, logger)

	default:
		return dispatchNotHandled
	}
}

// mdnsAutoRegister parses the JSON result from MDNSScan, bulk-inserts every discovered
// device into the inventory, and appends a registration summary to the result JSON.
func mdnsAutoRegister(scanJSON string, db *sql.DB, deviceType string, tags []string, overwrite bool, logger *slog.Logger) string {
	if deviceType == "" {
		deviceType = "mdns-device"
	}

	var result struct {
		Status  string              `json:"status"`
		Count   int                 `json:"count"`
		Devices []tools.MDNSService `json:"devices"`
		Message string              `json:"message,omitempty"`
	}
	if err := json.Unmarshal([]byte(scanJSON), &result); err != nil || result.Status != "success" || len(result.Devices) == 0 {
		return scanJSON // nothing to register
	}

	var created, updated, skipped int
	for _, dev := range result.Devices {
		name := mdnsCleanHostname(dev.Host)
		if name == "" {
			name = mdnsCleanName(dev.Name)
		}
		ip := ""
		if len(dev.IPs) > 0 {
			ip = dev.IPs[0]
			if net.ParseIP(ip) == nil {
				logger.Warn("mdns auto-register: skipping device with invalid IP", "host", name, "ip", ip)
				skipped++
				continue
			}
		}
		desc := dev.Info
		record := inventory.DeviceRecord{
			Name:        name,
			Type:        deviceType,
			IPAddress:   ip,
			Port:        dev.Port,
			Description: desc,
			Tags:        tags,
		}
		c, u, err := inventory.UpsertDeviceByName(db, record, overwrite)
		if err != nil {
			logger.Warn("mdns auto-register: failed to upsert device", "name", name, "error", err)
			skipped++
			continue
		}
		switch {
		case c:
			created++
		case u:
			updated++
		default:
			skipped++
		}
	}

	type regSummary struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Skipped int `json:"skipped"`
	}
	out := map[string]interface{}{
		"status":        result.Status,
		"count":         result.Count,
		"devices":       result.Devices,
		"auto_register": regSummary{Created: created, Updated: updated, Skipped: skipped},
	}
	if result.Message != "" {
		out["message"] = result.Message
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// mdnsCleanHostname strips the ".local." suffix from a mDNS hostname.
func mdnsCleanHostname(host string) string {
	h := strings.TrimSuffix(host, ".")
	h = strings.TrimSuffix(h, ".local")
	return h
}

// mdnsCleanName strips the service-type suffix from a full mDNS service name.
func mdnsCleanName(name string) string {
	if idx := strings.Index(name, "._"); idx > 0 {
		return name[:idx]
	}
	return strings.TrimSuffix(name, ".")
}
