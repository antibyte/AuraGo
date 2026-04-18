package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"aurago/internal/inventory"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
)

var _ = (*slog.Logger)(nil)

func dispatchPlatform(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	vault := dc.Vault
	registry := dc.Registry
	manifest := dc.Manifest
	longTermMem := dc.LongTermMem
	shortTermMem := dc.ShortTermMem
	kg := dc.KG
	inventoryDB := dc.InventoryDB
	invasionDB := dc.InvasionDB
	remoteHub := dc.RemoteHub
	coAgentRegistry := dc.CoAgentRegistry
	budgetTracker := dc.BudgetTracker
	handled := true

	result := func() string {
		switch tc.Action {
		case "co_agent", "co_agents":
			req := decodeCoAgentArgs(tc)
			if budgetTracker != nil && budgetTracker.IsBlocked("coagent") {
				return `Tool Output: {"status": "error", "message": "Co-Agent spawn blocked: daily budget exceeded. Try again tomorrow."}`
			}
			if budgetTracker != nil && budgetTracker.IsCategoryQuotaBlocked("coagent", cfg.CoAgents.BudgetQuotaPercent) {
				return `Tool Output: {"status": "error", "message": "Co-Agent quota reached for today. Reuse existing agents or continue without spawning more co-agents."}`
			}
			if !cfg.CoAgents.Enabled {
				return `Tool Output: {"status": "error", "message": "Co-Agent system is not enabled. Set co_agents.enabled=true in config.yaml."}`
			}
			if coAgentRegistry == nil {
				return `Tool Output: {"status": "error", "message": "Co-Agent registry not initialized."}`
			}
			switch req.Operation {
			case "spawn", "start", "create":
				task := req.Task
				if task == "" {
					return `Tool Output: {"status": "error", "message": "'task' is required to spawn a co-agent."}`
				}
				coReq := CoAgentRequest{
					Task:         task,
					ContextHints: req.ContextHints,
					Priority:     req.Priority,
				}
				id, state, err := SpawnCoAgent(cfg, ctx, logger, coAgentRegistry,
					shortTermMem, longTermMem, vault, registry, manifest, kg, inventoryDB, dc.CheatsheetDB, coReq, budgetTracker, nil)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				slots := coAgentRegistry.AvailableSlots()
				message := "Co-Agent started. Use operation 'list' to check status and 'get_result' when completed."
				if state == CoAgentQueued {
					message = "Co-Agent queued because all slots are busy. Use operation 'list' to monitor queue position."
				}

				return fmt.Sprintf(`Tool Output: {"status": "ok", "co_agent_id": "%s", "state": "%s", "available_slots": %d, "message": "%s"}`, id, state, slots, message)

			case "spawn_specialist":
				task := req.Task
				if task == "" {
					return `Tool Output: {"status": "error", "message": "'task' is required to spawn a specialist."}`
				}
				specialist := req.Specialist
				if specialist == "" {
					return `Tool Output: {"status": "error", "message": "'specialist' is required. Choose: researcher, coder, designer, security, writer."}`
				}
				coReq := CoAgentRequest{
					Task:         task,
					ContextHints: req.ContextHints,
					Specialist:   specialist,
					Priority:     req.Priority,
				}
				id, state, err := SpawnCoAgent(cfg, ctx, logger, coAgentRegistry,
					shortTermMem, longTermMem, vault, registry, manifest, kg, inventoryDB, dc.CheatsheetDB, coReq, budgetTracker, nil)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				slots := coAgentRegistry.AvailableSlots()
				message := fmt.Sprintf("Specialist '%s' started. Use 'list' to check status and 'get_result' when completed.", specialist)
				if state == CoAgentQueued {
					message = fmt.Sprintf("Specialist '%s' queued because all slots are busy. Use 'list' to monitor queue position.", specialist)
				}

				return fmt.Sprintf(`Tool Output: {"status": "ok", "co_agent_id": "%s", "specialist": "%s", "state": "%s", "available_slots": %d, "message": "%s"}`, id, specialist, state, slots, message)

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
				coID := req.CoAgentID
				if coID == "" {
					return `Tool Output: {"status": "error", "message": "'co_agent_id' is required."}`
				}
				status, err := coAgentRegistry.GetStatus(coID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				state, _ := status["state"].(string)
				status["status"] = "ok"
				switch state {
				case string(CoAgentQueued):
					status["message"] = "Co-Agent is still queued."
				case string(CoAgentRunning):
					status["message"] = "Co-Agent is still running."
				case string(CoAgentCompleted):
					status["message"] = "Co-Agent completed successfully."
				case string(CoAgentFailed):
					status["message"] = "Co-Agent failed."
				case string(CoAgentCancelled):
					status["message"] = "Co-Agent was cancelled."
				}
				out, _ := json.Marshal(status)

				return "Tool Output: " + string(out)

			case "stop", "cancel", "kill":
				coID := req.CoAgentID
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

		case "tts":
			if !cfg.Chromecast.Enabled && cfg.TTS.Provider == "" && !cfg.TTS.Piper.Enabled {
				return `Tool Output: {"status": "error", "message": "TTS is not configured. Set tts.provider in config.yaml."}`
			}
			req := decodeTTSArgs(tc)
			provider := cfg.TTS.Provider
			if provider == "" && cfg.TTS.Piper.Enabled {
				provider = "piper"
			}
			ttsCfg := tools.TTSConfig{
				Provider:            provider,
				Language:            req.Language,
				DataDir:             cfg.Directories.DataDir,
				CacheRetentionHours: cfg.TTS.CacheRetentionHours,
				CacheMaxFiles:       cfg.TTS.CacheMaxFiles,
			}
			if ttsCfg.Language == "" {
				ttsCfg.Language = cfg.TTS.Language
			}
			ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
			ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
			ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
			ttsCfg.MiniMax.APIKey = cfg.TTS.MiniMax.APIKey
			ttsCfg.MiniMax.VoiceID = cfg.TTS.MiniMax.VoiceID
			ttsCfg.MiniMax.ModelID = cfg.TTS.MiniMax.ModelID
			ttsCfg.MiniMax.Speed = cfg.TTS.MiniMax.Speed
			ttsCfg.Piper.Port = cfg.TTS.Piper.ContainerPort
			ttsCfg.Piper.Voice = cfg.TTS.Piper.Voice
			ttsCfg.Piper.SpeakerID = cfg.TTS.Piper.SpeakerID
			filename, err := tools.TTSSynthesize(ttsCfg, req.Text)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "TTS failed: %v"}`, err)
			}

			ttsPort := cfg.Server.Port // TTS is always served on the main server
			if cfg.Chromecast.Enabled && cfg.Chromecast.TTSPort > 0 {
				ttsPort = cfg.Chromecast.TTSPort // Chromecast has its own dedicated TTS server
			}
			audioURL := fmt.Sprintf("http://%s:%d/tts/%s", getLocalIP(cfg), ttsPort, filename)
			absLocalPath, _ := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "tts", filename))
			absLocalPath = filepath.ToSlash(absLocalPath)
			return fmt.Sprintf(`Tool Output: {"status": "success", "file": "%s", "url": "%s", "local_path": "%s"}`, filename, audioURL, absLocalPath)

		case "chromecast":
			if !cfg.Chromecast.Enabled {
				return `Tool Output: {"status": "error", "message": "Chromecast is disabled. Set chromecast.enabled=true in config.yaml."}`
			}
			// Resolve device_name → device_addr via inventory if device_addr is empty
			req := decodeChromecastArgs(tc)
			if req.DeviceAddr == "" && req.DeviceName != "" && inventoryDB != nil {
				devices, err := inventory.QueryDevices(inventoryDB, "", "chromecast", req.DeviceName)
				if err == nil && len(devices) > 0 {
					req.DeviceAddr = devices[0].IPAddress
					if req.DevicePort == 0 && devices[0].Port > 0 {
						req.DevicePort = devices[0].Port
					}
					logger.Info("Resolved chromecast device name", "name", req.DeviceName, "addr", req.DeviceAddr, "port", req.DevicePort)
				} else {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Could not find chromecast device named '%s' in the device registry."}`, req.DeviceName)
				}
			}
			switch req.Operation {
			case "discover":
				return "Tool Output: " + tools.ChromecastDiscover(logger)
			case "play":
				return "Tool Output: " + tools.ChromecastPlay(req.DeviceAddr, req.DevicePort, req.URL, req.ContentType, logger)
			case "speak":
				ttsCfg := tools.TTSConfig{
					Provider:            cfg.TTS.Provider,
					Language:            req.Language,
					DataDir:             cfg.Directories.DataDir,
					CacheRetentionHours: cfg.TTS.CacheRetentionHours,
					CacheMaxFiles:       cfg.TTS.CacheMaxFiles,
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
				return "Tool Output: " + tools.ChromecastSpeak(req.DeviceAddr, req.DevicePort, req.Text, ttsCfg, ccCfg, logger)
			case "stop":
				return "Tool Output: " + tools.ChromecastStop(req.DeviceAddr, req.DevicePort, logger)
			case "volume":
				return "Tool Output: " + tools.ChromecastVolume(req.DeviceAddr, req.DevicePort, req.Volume, logger)
			case "status":
				return "Tool Output: " + tools.ChromecastStatus(req.DeviceAddr, req.DevicePort, logger)
			default:
				return `Tool Output: {"status": "error", "message": "Unknown chromecast operation. Use: discover, play, speak, stop, volume, status"}`
			}

		case "manage_webhooks":
			if !cfg.Webhooks.Enabled {
				return `Tool Output: {"status":"error","message":"Webhooks are not enabled. Set webhooks.enabled: true in config."}`
			}
			req := decodeManageWebhooksArgs(tc)
			if cfg.Webhooks.ReadOnly {
				switch req.Operation {
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
			return handleWebhookToolCall(req, whMgr, logger)

		case "proxmox", "proxmox_ve":
			if !cfg.Proxmox.Enabled {
				return `Tool Output: {"status":"error","message":"Proxmox integration is not enabled. Set proxmox.enabled=true in config.yaml."}`
			}
			req := decodeProxmoxArgs(tc)
			if cfg.Proxmox.ReadOnly {
				switch req.Operation {
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
			node := req.node()
			vmid := req.vmid()
			vmType := req.VMType
			switch req.Operation {
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
				logger.Info("LLM requested Proxmox action", "action", req.Operation, "vmid", vmid)
				return "Tool Output: " + tools.ProxmoxVMAction(pxCfg, node, vmType, vmid, req.Operation)
			case "node_status":
				logger.Info("LLM requested Proxmox node_status", "node", node)
				return "Tool Output: " + tools.ProxmoxNodeStatus(pxCfg, node)
			case "cluster_resources", "resources":
				resType := req.ResourceType
				logger.Info("LLM requested Proxmox cluster_resources", "type", resType)
				return "Tool Output: " + tools.ProxmoxClusterResources(pxCfg, resType)
			case "storage":
				logger.Info("LLM requested Proxmox storage", "node", node)
				return "Tool Output: " + tools.ProxmoxGetStorage(pxCfg, node)
			case "create_snapshot", "snapshot":
				logger.Info("LLM requested Proxmox create_snapshot", "vmid", vmid, "name", req.Name)
				return "Tool Output: " + tools.ProxmoxCreateSnapshot(pxCfg, node, vmType, vmid, req.Name, req.Description)
			case "list_snapshots", "snapshots":
				logger.Info("LLM requested Proxmox list_snapshots", "vmid", vmid)
				return "Tool Output: " + tools.ProxmoxListSnapshots(pxCfg, node, vmType, vmid)
			case "task_log":
				upid := req.upid()
				logger.Info("LLM requested Proxmox task_log", "upid", upid)
				return "Tool Output: " + tools.ProxmoxGetTaskLog(pxCfg, node, upid)
			default:
				return `Tool Output: {"status":"error","message":"Unknown proxmox operation. Use: overview, list_nodes, list_vms, list_containers, status, start, stop, shutdown, reboot, node_status, cluster_resources, storage, create_snapshot, list_snapshots, task_log"}`
			}

		case "ollama", "ollama_management":
			if !cfg.Ollama.Enabled {
				return `Tool Output: {"status":"error","message":"Ollama integration is not enabled. Set ollama.enabled=true in config.yaml."}`
			}
			req := decodeOllamaArgs(tc)
			if cfg.Ollama.ReadOnly {
				switch req.Operation {
				case "pull", "download", "delete", "remove", "copy", "load", "unload":
					return `Tool Output: {"status":"error","message":"Ollama is in read-only mode. Disable ollama.read_only to allow changes."}`
				}
			}
			olCfg := tools.OllamaConfig{URL: cfg.Ollama.URL}
			modelName := req.modelName()
			switch req.Operation {
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
				src := req.Source
				dst := req.destinationName()
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

		case "ansible":
			if !cfg.Ansible.Enabled {
				return `Tool Output: {"status":"error","message":"Ansible integration is not enabled. Set ansible.enabled=true in config.yaml."}`
			}
			req := decodeAnsibleArgs(tc)
			if cfg.Ansible.ReadOnly {
				switch req.Operation {
				case "adhoc", "command", "run_module", "playbook", "run", "run_playbook":
					return `Tool Output: {"status":"error","message":"Ansible is in read-only mode. Disable ansible.read_only to allow changes."}`
				}
			}
			// Resolve host pattern (hosts for ad-hoc / limit for playbooks)
			hosts := req.hosts()
			inventoryPath := req.Inventory
			extraVars := req.extraVars()

			isLocal := cfg.Ansible.Mode == "local"

			if isLocal {
				// ── Local CLI mode ──────────────────────────────────────────────────────
				localCfg := tools.AnsibleLocalConfig{
					PlaybooksDir:     cfg.Ansible.PlaybooksDir,
					DefaultInventory: cfg.Ansible.DefaultInventory,
					Timeout:          cfg.Ansible.Timeout,
				}
				switch req.Operation {
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
					module := req.moduleName()
					moduleArgs := req.Command
					logger.Info("LLM requested Ansible adhoc (local)", "hosts", hosts, "module", module)
					return "Tool Output: " + tools.AnsibleLocalAdhoc(localCfg, hosts, module, moduleArgs, inventoryPath, extraVars)
				case "playbook", "run", "run_playbook":
					playbook := req.Name
					if playbook == "" {
						return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=playbook"}`
					}
					logger.Info("LLM requested Ansible playbook (local)", "playbook", playbook, "limit", req.HostLimit)
					return "Tool Output: " + tools.AnsibleLocalRunPlaybook(localCfg, playbook, inventoryPath, req.HostLimit, req.Tags, req.SkipTags, extraVars, req.Preview, false)
				case "check", "dry_run":
					playbook := req.Name
					if playbook == "" {
						return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=check"}`
					}
					logger.Info("LLM requested Ansible playbook dry-run (local)", "playbook", playbook)
					return "Tool Output: " + tools.AnsibleLocalRunPlaybook(localCfg, playbook, inventoryPath, req.HostLimit, req.Tags, req.SkipTags, extraVars, true, true)
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
			switch req.Operation {
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
				module := req.moduleName()
				moduleArgs := req.Command
				logger.Info("LLM requested Ansible adhoc", "hosts", hosts, "module", module)
				return "Tool Output: " + tools.AnsibleAdhoc(ansCfg, hosts, module, moduleArgs, inventoryPath, extraVars)
			case "playbook", "run", "run_playbook":
				playbook := req.Name
				if playbook == "" {
					return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=playbook"}`
				}
				logger.Info("LLM requested Ansible playbook", "playbook", playbook, "limit", req.HostLimit, "check", req.Preview)
				return "Tool Output: " + tools.AnsibleRunPlaybook(ansCfg, playbook, inventoryPath, req.HostLimit, req.Tags, req.SkipTags, extraVars, req.Preview, false)
			case "check", "dry_run":
				playbook := req.Name
				if playbook == "" {
					return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=check"}`
				}
				logger.Info("LLM requested Ansible playbook dry-run", "playbook", playbook)
				return "Tool Output: " + tools.AnsibleRunPlaybook(ansCfg, playbook, inventoryPath, req.HostLimit, req.Tags, req.SkipTags, extraVars, true, true)
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

		case "mcp_call":
			// Two-gate security: allow_mcp (Danger Zone) AND mcp.enabled
			if !cfg.Agent.AllowMCP {
				return `Tool Output: [PERMISSION DENIED] MCP is disabled in Danger Zone settings (agent.allow_mcp: false).`
			}
			if !cfg.MCP.Enabled {
				return `Tool Output: [PERMISSION DENIED] MCP is disabled (mcp.enabled: false).`
			}

			req := decodeMCPCallArgs(tc)
			op := strings.ToLower(strings.TrimSpace(req.Operation))
			switch op {
			case "list_servers":
				servers, err := tools.MCPListServers(logger)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP list servers failed: %v"}`, err)
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "success", "servers": servers})
				return "Tool Output: " + string(data)

			case "list_tools":
				mcpTools, err := tools.MCPListTools(req.Server, logger)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP list tools failed: %v"}`, err)
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "success", "tools": mcpTools})
				return "Tool Output: " + string(data)

			case "call_tool", "call":
				if req.Server == "" || req.ToolName == "" {
					return `Tool Output: {"status": "error", "message": "mcp_call with operation=call requires 'server' and 'tool_name'"}`
				}
				result, err := tools.MCPCallTool(req.Server, req.ToolName, req.Args, logger)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP call failed: %v"}`, err)
				}
				return "Tool Output: " + security.Scrub(result)

			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "unknown mcp_call operation '%s'. Use list_servers, list_tools, or call_tool."}`, op)
			}

		case "truenas",
			"truenas_health", "truenas_pool_list", "truenas_pool_scrub",
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
			req := decodeTrueNASArgs(tc)
			logger.Info("LLM requested TrueNAS operation", "action", req.Action)
			// Build a comprehensive params map from ToolCall fields.
			// The unified "truenas" schema maps: name→tc.Name, path→tc.Path, query→pool/dataset,
			// port→pool_id/share_id, limit→quota_gb/retention_days, content→compression,
			// recursive→tc.Recursive, force→tc.Force.
			return "Tool Output: " + tools.DispatchTrueNASTool(req.Action, req.params(), cfg, nil, logger)

		// ── Jellyfin Media Server ──
		case "jellyfin":
			if !cfg.Jellyfin.Enabled {
				return `Tool Output: {"status":"error","message":"Jellyfin integration is not enabled. Set jellyfin.enabled=true in config.yaml."}`
			}
			req := decodeJellyfinArgs(tc)
			// Read-only check for mutating operations
			if cfg.Jellyfin.ReadOnly {
				switch req.Operation {
				case "playback_control", "library_refresh":
					return `Tool Output: {"status":"error","message":"Jellyfin is in read-only mode. Disable jellyfin.read_only to allow changes."}`
				}
			}
			// Destructive operation check
			if !cfg.Jellyfin.AllowDestructive {
				switch req.Operation {
				case "delete_item":
					return `Tool Output: {"status":"error","message":"Destructive Jellyfin operations are disabled. Set jellyfin.allow_destructive=true in config.yaml."}`
				}
			}
			logger.Info("LLM requested Jellyfin operation", "operation", req.Operation)
			return "Tool Output: " + tools.DispatchJellyfinTool(req.Operation, req.params(), cfg, logger)

		// ── Obsidian Knowledge Management ──
		case "obsidian":
			if !cfg.Obsidian.Enabled {
				return `Tool Output: {"status":"error","message":"Obsidian integration is not enabled. Set obsidian.enabled=true in config.yaml."}`
			}
			req := decodeObsidianArgs(tc)
			// Read-only check for mutating operations
			if cfg.Obsidian.ReadOnly {
				switch req.Operation {
				case "create_note", "update_note", "patch_note", "execute_command", "open_in_obsidian":
					return `Tool Output: {"status":"error","message":"Obsidian is in read-only mode. Disable obsidian.readonly to allow changes."}`
				case "daily_note", "periodic_note":
					if req.Content != "" {
						return `Tool Output: {"status":"error","message":"Obsidian is in read-only mode. Disable obsidian.readonly to allow changes."}`
					}
				}
			}
			// Destructive operation check
			if !cfg.Obsidian.AllowDestructive {
				switch req.Operation {
				case "delete_note":
					return `Tool Output: {"status":"error","message":"Destructive Obsidian operations are disabled. Set obsidian.allow_destructive=true in config.yaml."}`
				}
			}
			logger.Info("LLM requested Obsidian operation", "operation", req.Operation)
			return "Tool Output: " + tools.DispatchObsidianTool(req.Operation, req.params(), cfg, vault, logger)

		default:
			handled = false
			return ""
		}
	}()
	return result, handled
}
