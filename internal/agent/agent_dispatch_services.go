package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/meshcentral"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
)

// dispatchServices handles media, infrastructure management, and platform tool calls
// (vision, transcribe, meshcentral, docker, homepage, webdav, home_assistant).
func dispatchServices(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManagerV2 *tools.MissionManagerV2, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, contactsDB *sql.DB, sqlConnectionsDB *sql.DB, sqlConnectionPool *sqlconnections.ConnectionPool, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
	switch tc.Action {
	case "analyze_image", "vision":
		if budgetTracker != nil && budgetTracker.IsBlocked("vision") {
			return `Tool Output: {"status": "error", "message": "Vision blocked: daily budget exceeded. Try again tomorrow."}`
		}
		logger.Info("LLM requested image analysis", "file_path", tc.FilePath)
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		if fpath == "" {
			return `Tool Output: {"status": "error", "message": "'file_path' is required for analyze_image"}`
		}
		if strings.Contains(fpath, "..") {
			return `Tool Output: {"status": "error", "message": "path traversal sequences ('..') are not allowed"}`
		}
		prompt := tc.Prompt
		if prompt == "" {
			prompt = "Describe this image in detail. What do you see? If there is text, transcribe it. If there are people, describe their actions."
		}
		result, err := tools.AnalyzeImageWithPrompt(fpath, prompt, cfg)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Vision analysis failed: %v"}`, err)
		}
		return fmt.Sprintf("Tool Output: %s", result)

	case "transcribe_audio", "speech_to_text":
		if budgetTracker != nil && budgetTracker.IsBlocked("stt") {
			return `Tool Output: {"status": "error", "message": "Speech-to-text blocked: daily budget exceeded. Try again tomorrow."}`
		}
		logger.Info("LLM requested audio transcription", "file_path", tc.FilePath)
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		if fpath == "" {
			return `Tool Output: {"status": "error", "message": "'file_path' is required for transcribe_audio"}`
		}
		if strings.Contains(fpath, "..") {
			return `Tool Output: {"status": "error", "message": "path traversal sequences ('..') are not allowed"}`
		}
		result, err := tools.TranscribeAudioFile(fpath, cfg)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Transcription failed: %v"}`, err)
		}
		return fmt.Sprintf("Tool Output: %s", result)

	case "meshcentral":
		if !cfg.MeshCentral.Enabled {
			return `Tool Output: {"status": "error", "message": "MeshCentral integration is not enabled in config.yaml."}`
		}

		logger.Info("LLM requested MeshCentral operation", "op", tc.Operation)

		normalizeMeshCentralOp := func(op string) string {
			switch strings.ToLower(strings.TrimSpace(op)) {
			case "meshes":
				return "list_groups"
			case "nodes":
				return "list_devices"
			case "wakeonlan":
				return "wake"
			default:
				return strings.ToLower(strings.TrimSpace(op))
			}
		}

		op := normalizeMeshCentralOp(tc.Operation)

		if cfg.MeshCentral.ReadOnly {
			switch op {
			case "list_groups", "list_devices":
				// allowed in read-only mode
			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MeshCentral operation '%s' blocked: meshcentral.readonly is enabled."}`, tc.Operation)
			}
		}

		for _, blocked := range cfg.MeshCentral.BlockedOperations {
			if normalizeMeshCentralOp(blocked) == op && op != "" {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MeshCentral operation '%s' blocked by policy (meshcentral.blocked_operations)."}`, tc.Operation)
			}
		}

		// Attempt to resolve password/token from vault if missing
		token := cfg.MeshCentral.LoginToken
		pass := cfg.MeshCentral.Password
		if token == "" {
			vToken, _ := vault.ReadSecret("meshcentral_token")
			if vToken != "" {
				token = vToken
			}
		}
		if pass == "" {
			vPass, _ := vault.ReadSecret("meshcentral_password")
			if vPass != "" {
				pass = vPass
			}
		}
		if pass == "" && token == "" && cfg.MeshCentral.Username != "" {
			return `Tool Output: {"status": "error", "message": "No password or token found. Please set 'meshcentral_password' or 'meshcentral_token' in the vault."}`
		}

		mcClient := meshcentral.NewClient(cfg.MeshCentral.URL, cfg.MeshCentral.Username, pass, token, cfg.MeshCentral.Insecure)
		if err := mcClient.Connect(); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to connect to MeshCentral: %v"}`, err)
		}
		defer mcClient.Close()

		switch op {
		case "list_groups":
			meshes, err := mcClient.ListDeviceGroups()
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to list device groups: %v"}`, err)
			}
			b, _ := json.Marshal(meshes)
			return fmt.Sprintf(`Tool Output: {"status": "success", "groups": %s}`, string(b))

		case "list_devices":
			nodes, err := mcClient.ListDevices(tc.MeshID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to list devices: %v"}`, err)
			}
			b, _ := json.Marshal(nodes)
			return fmt.Sprintf(`Tool Output: {"status": "success", "devices": %s}`, string(b))

		case "wake":
			if tc.NodeID == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' is required for wake"}`
			}
			if !strings.HasPrefix(tc.NodeID, "node//") {
				return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
			}
			logger.Info("MeshCentral wake_on_lan", "node_id", tc.NodeID, "session_id", sessionID)
			result, err := mcClient.WakeOnLan([]string{tc.NodeID})
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send wake magic packet: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

		case "power_action":
			if tc.NodeID == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' is required for power_action"}`
			}
			if !strings.HasPrefix(tc.NodeID, "node//") {
				return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
			}
			if tc.PowerAction < 1 || tc.PowerAction > 4 {
				return `Tool Output: {"status": "error", "message": "Invalid power action. 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset"}`
			}
			logger.Info("MeshCentral power_action", "node_id", tc.NodeID, "power_action", tc.PowerAction, "session_id", sessionID)
			result, err := mcClient.PowerAction([]string{tc.NodeID}, tc.PowerAction)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send power action: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

		case "run_command":
			if tc.NodeID == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for run_command"}`
			}
			if !strings.HasPrefix(tc.NodeID, "node//") {
				return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
			}
			logger.Info("MeshCentral run_command", "node_id", tc.NodeID, "command", tc.Command, "session_id", sessionID)
			result, err := mcClient.RunCommand(tc.NodeID, tc.Command)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to run command: %v"}`, err)
			}
			// Format result for display
			resultJSON, _ := json.Marshal(result)
			return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(resultJSON))

		case "shell":
			if tc.NodeID == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for shell"}`
			}
			if !strings.HasPrefix(tc.NodeID, "node//") {
				return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
			}
			logger.Info("MeshCentral shell", "node_id", tc.NodeID, "command", tc.Command, "session_id", sessionID)
			result, err := mcClient.Shell(tc.NodeID, tc.Command)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to execute shell command: %v"}`, err)
			}
			// Format result for display
			resultJSON, _ := json.Marshal(result)
			return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(resultJSON))

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, tc.Operation)
		}

	case "docker", "docker_management":
		if !cfg.Docker.Enabled {
			return `Tool Output: {"status": "error", "message": "Docker integration is not enabled. Set docker.enabled=true in config.yaml."}`
		}
		if cfg.Docker.ReadOnly {
			switch tc.Operation {
			case "start", "stop", "restart", "pause", "unpause", "remove", "rm", "create", "create_container", "run", "pull_image", "pull", "remove_image", "rmi":
				return `Tool Output: {"status":"error","message":"Docker is in read-only mode. Disable docker.read_only to allow changes."}`
			}
		}
		dockerCfg := tools.DockerConfig{Host: cfg.Docker.Host}
		containerID := tc.ContainerID
		if containerID == "" {
			containerID = tc.Name
		}
		switch tc.Operation {
		case "list_containers", "ps":
			logger.Info("LLM requested Docker list_containers", "all", tc.All)
			return "Tool Output: " + tools.DockerListContainers(dockerCfg, tc.All)
		case "inspect", "inspect_container":
			logger.Info("LLM requested Docker inspect", "container_id", containerID)
			return "Tool Output: " + tools.DockerInspectContainer(dockerCfg, containerID)
		case "start":
			logger.Info("LLM requested Docker start", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "start", false)
		case "stop":
			logger.Info("LLM requested Docker stop", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "stop", false)
		case "restart":
			logger.Info("LLM requested Docker restart", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "restart", false)
		case "pause":
			logger.Info("LLM requested Docker pause", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "pause", false)
		case "unpause":
			logger.Info("LLM requested Docker unpause", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "unpause", false)
		case "remove", "rm":
			logger.Info("LLM requested Docker remove", "container_id", containerID, "force", tc.Force)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "remove", tc.Force)
		case "logs":
			logger.Info("LLM requested Docker logs", "container_id", containerID, "tail", tc.Tail)
			return "Tool Output: " + tools.DockerContainerLogs(dockerCfg, containerID, tc.Tail)
		case "create", "create_container", "run":
			logger.Info("LLM requested Docker create", "image", tc.Image, "name", tc.Name)
			var cmd []string
			if tc.Command != "" {
				cmd = strings.Fields(tc.Command)
			}
			restart := tc.Restart
			if restart == "" {
				restart = "no"
			}
			result := tools.DockerCreateContainer(dockerCfg, tc.Name, tc.Image, tc.Env, tc.Ports, tc.Volumes, cmd, restart)
			// Auto-start if operation was "run"
			if tc.Operation == "run" {
				var created map[string]interface{}
				if json.Unmarshal([]byte(result), &created) == nil {
					if id, ok := created["id"].(string); ok && id != "" {
						tools.DockerContainerAction(dockerCfg, id, "start", false)
						created["message"] = "Container created and started"
						updated, _ := json.Marshal(created)
						result = string(updated)
					}
				}
			}
			return "Tool Output: " + result
		case "list_images", "images":
			logger.Info("LLM requested Docker list_images")
			return "Tool Output: " + tools.DockerListImages(dockerCfg)
		case "pull_image", "pull":
			logger.Info("LLM requested Docker pull", "image", tc.Image)
			return "Tool Output: " + tools.DockerPullImage(dockerCfg, tc.Image)
		case "remove_image", "rmi":
			logger.Info("LLM requested Docker remove_image", "image", tc.Image, "force", tc.Force)
			return "Tool Output: " + tools.DockerRemoveImage(dockerCfg, tc.Image, tc.Force)
		case "list_networks", "networks":
			logger.Info("LLM requested Docker list_networks")
			return "Tool Output: " + tools.DockerListNetworks(dockerCfg)
		case "list_volumes", "volumes":
			logger.Info("LLM requested Docker list_volumes")
			return "Tool Output: " + tools.DockerListVolumes(dockerCfg)
		case "info", "system_info":
			logger.Info("LLM requested Docker system_info")
			return "Tool Output: " + tools.DockerSystemInfo(dockerCfg)
		case "exec":
			logger.Info("LLM requested Docker exec", "container_id", containerID, "cmd", tc.Command)
			return "Tool Output: " + tools.DockerExec(dockerCfg, containerID, tc.Command, tc.User)
		case "stats":
			logger.Info("LLM requested Docker stats", "container_id", containerID)
			return "Tool Output: " + tools.DockerStats(dockerCfg, containerID)
		case "top":
			logger.Info("LLM requested Docker top", "container_id", containerID)
			return "Tool Output: " + tools.DockerTop(dockerCfg, containerID)
		case "port":
			logger.Info("LLM requested Docker port", "container_id", containerID)
			return "Tool Output: " + tools.DockerPort(dockerCfg, containerID)
		case "cp", "copy":
			logger.Info("LLM requested Docker cp", "container_id", containerID, "src", tc.Source, "dest", tc.Destination, "direction", tc.Direction)
			return "Tool Output: " + tools.DockerCopy(dockerCfg, containerID, tc.Source, tc.Destination, tc.Direction)
		case "create_network":
			logger.Info("LLM requested Docker create_network", "name", tc.Name, "driver", tc.Driver)
			return "Tool Output: " + tools.DockerCreateNetwork(dockerCfg, tc.Name, tc.Driver)
		case "remove_network":
			logger.Info("LLM requested Docker remove_network", "name", tc.Name)
			return "Tool Output: " + tools.DockerRemoveNetwork(dockerCfg, tc.Name)
		case "connect":
			logger.Info("LLM requested Docker connect", "container_id", containerID, "network", tc.Network)
			return "Tool Output: " + tools.DockerConnectNetwork(dockerCfg, containerID, tc.Network)
		case "disconnect":
			logger.Info("LLM requested Docker disconnect", "container_id", containerID, "network", tc.Network)
			return "Tool Output: " + tools.DockerDisconnectNetwork(dockerCfg, containerID, tc.Network)
		case "create_volume":
			logger.Info("LLM requested Docker create_volume", "name", tc.Name, "driver", tc.Driver)
			return "Tool Output: " + tools.DockerCreateVolume(dockerCfg, tc.Name, tc.Driver)
		case "remove_volume":
			logger.Info("LLM requested Docker remove_volume", "name", tc.Name, "force", tc.Force)
			return "Tool Output: " + tools.DockerRemoveVolume(dockerCfg, tc.Name, tc.Force)
		case "compose":
			logger.Info("LLM requested Docker compose", "file", tc.File, "cmd", tc.Command)
			return "Tool Output: " + tools.DockerCompose(dockerCfg, tc.File, tc.Command)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown docker operation. Use: list_containers, inspect, start, stop, restart, pause, unpause, remove, logs, create, run, list_images, pull, remove_image, list_networks, create_network, remove_network, connect, disconnect, list_volumes, create_volume, remove_volume, exec, stats, top, port, cp, compose, info"}`
		}

	case "homepage", "homepage_tool":
		if !cfg.Docker.Enabled {
			return `Tool Output: {"status": "error", "message": "Homepage tool requires Docker. Set docker.enabled=true in config.yaml."}`
		}
		if !cfg.Homepage.Enabled {
			return `Tool Output: {"status": "error", "message": "Homepage tool is not enabled. Set homepage.enabled=true in config.yaml."}`
		}
		homepageCfg := tools.HomepageConfig{
			DockerHost:            cfg.Docker.Host,
			WorkspacePath:         cfg.Homepage.WorkspacePath,
			DataDir:               cfg.Directories.DataDir,
			WebServerPort:         cfg.Homepage.WebServerPort,
			WebServerDomain:       cfg.Homepage.WebServerDomain,
			WebServerInternalOnly: cfg.Homepage.WebServerInternalOnly,
			AllowLocalServer:      cfg.Homepage.AllowLocalServer,
		}
		if homepageCfg.WorkspacePath == "" {
			homepageCfg.WorkspacePath = filepath.Join(cfg.Directories.DataDir, "homepage")
		}
		deployCfg := tools.HomepageDeployConfig{
			Host:     cfg.Homepage.DeployHost,
			Port:     cfg.Homepage.DeployPort,
			User:     cfg.Homepage.DeployUser,
			Password: cfg.Homepage.DeployPassword,
			Key:      cfg.Homepage.DeployKey,
			Path:     cfg.Homepage.DeployPath,
			Method:   cfg.Homepage.DeployMethod,
		}

		// Permission checks for restricted operations
		switch tc.Operation {
		case "deploy", "test_connection":
			if !cfg.Homepage.AllowDeploy {
				return `Tool Output: {"status":"error","message":"Deployment is disabled. Enable homepage.allow_deploy in config."}`
			}
		case "init", "start", "stop", "rebuild", "destroy", "webserver_start", "webserver_stop", "exec":
			if !cfg.Homepage.AllowContainerManagement {
				return `Tool Output: {"status":"error","message":"Container management is disabled. Enable homepage.allow_container_management in config."}`
			}
		}

		switch tc.Operation {
		case "init":
			logger.Info("LLM requested homepage init")
			return "Tool Output: " + tools.HomepageInit(homepageCfg, logger)
		case "start":
			logger.Info("LLM requested homepage start")
			return "Tool Output: " + tools.HomepageStart(homepageCfg, logger)
		case "stop":
			logger.Info("LLM requested homepage stop")
			return "Tool Output: " + tools.HomepageStop(homepageCfg, logger)
		case "status":
			logger.Info("LLM requested homepage status")
			return "Tool Output: " + tools.HomepageStatus(homepageCfg, logger)
		case "rebuild":
			logger.Info("LLM requested homepage rebuild")
			return "Tool Output: " + tools.HomepageRebuild(homepageCfg, logger)
		case "destroy":
			logger.Info("LLM requested homepage destroy")
			return "Tool Output: " + tools.HomepageDestroy(homepageCfg, logger)
		case "exec":
			logger.Info("LLM requested homepage exec", "cmd", tc.Command)
			execCmd := tc.Command
			// Auto-inject Netlify auth token when the command invokes the netlify CLI
			if vault != nil && strings.Contains(tc.Command, "netlify ") {
				if nfTok, tokErr := vault.ReadSecret("netlify_token"); tokErr == nil && nfTok != "" {
					execCmd = "NETLIFY_AUTH_TOKEN=" + nfTok + " " + tc.Command
				}
			}
			return "Tool Output: " + tools.HomepageExec(homepageCfg, execCmd, logger)
		case "init_project":
			logger.Info("LLM requested homepage init_project", "framework", tc.Framework, "name", tc.Name, "template", tc.Template)
			result := tools.HomepageInitProject(homepageCfg, tc.Framework, tc.Name, tc.Template, logger)
			// Auto-register project in homepage registry
			if homepageRegistryDB != nil && tc.Name != "" {
				tools.RegisterProject(homepageRegistryDB, tools.HomepageProject{
					Name:      tc.Name,
					Framework: tc.Framework,
					Status:    "active",
					Tags:      []string{"auto-registered"},
				})
			}
			return "Tool Output: " + result
		case "build":
			logger.Info("LLM requested homepage build", "dir", tc.ProjectDir, "auto_fix", tc.AutoFix)
			var result string
			if tc.AutoFix {
				result = tools.HomepageBuildWithAutoFix(homepageCfg, tc.ProjectDir, logger)
			} else {
				result = tools.HomepageBuild(homepageCfg, tc.ProjectDir, logger)
			}
			// Auto-log edit in homepage registry
			if homepageRegistryDB != nil && tc.ProjectDir != "" {
				if proj, err := tools.GetProjectByDir(homepageRegistryDB, tc.ProjectDir); err == nil {
					tools.LogEdit(homepageRegistryDB, proj.ID, "build")
				}
			}
			return "Tool Output: " + result
		case "install_deps":
			logger.Info("LLM requested homepage install_deps", "packages", tc.Packages)
			return "Tool Output: " + tools.HomepageInstallDeps(homepageCfg, tc.ProjectDir, tc.Packages, logger)
		case "lighthouse":
			logger.Info("LLM requested homepage lighthouse", "url", tc.URL)
			result := tools.HomepageLighthouse(homepageCfg, tc.URL, logger)
			// Auto-log lighthouse score in homepage registry
			if homepageRegistryDB != nil && tc.URL != "" {
				projects, _, _ := tools.SearchProjects(homepageRegistryDB, tc.URL, "", nil, 1, 0)
				if len(projects) > 0 {
					tools.UpdateProject(homepageRegistryDB, projects[0].ID, map[string]interface{}{
						"lighthouse_score": result,
					})
				}
			}
			return "Tool Output: " + result
		case "screenshot":
			logger.Info("LLM requested homepage screenshot", "url", tc.URL, "viewport", tc.Viewport)
			return "Tool Output: " + tools.HomepageScreenshot(homepageCfg, tc.URL, tc.Viewport, logger)
		case "lint":
			logger.Info("LLM requested homepage lint", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageLint(homepageCfg, tc.ProjectDir, logger)
		case "list_files":
			logger.Info("LLM requested homepage list_files", "path", tc.Path)
			return "Tool Output: " + tools.HomepageListFiles(homepageCfg, tc.Path, logger)
		case "read_file":
			logger.Info("LLM requested homepage read_file", "path", tc.Path)
			return "Tool Output: " + tools.HomepageReadFile(homepageCfg, tc.Path, logger)
		case "write_file":
			logger.Info("LLM requested homepage write_file", "path", tc.Path)
			return "Tool Output: " + tools.HomepageWriteFile(homepageCfg, tc.Path, tc.Content, logger)
		case "edit_file":
			logger.Info("LLM requested homepage edit_file", "path", tc.Path, "op", tc.Action)
			return "Tool Output: " + tools.HomepageEditFile(homepageCfg, tc.Path, tc.Action, tc.Old, tc.New, tc.Marker, tc.Content, tc.StartLine, tc.EndLine, logger)
		case "json_edit":
			logger.Info("LLM requested homepage json_edit", "path", tc.Path, "op", tc.Action)
			return "Tool Output: " + tools.HomepageJsonEdit(homepageCfg, tc.Path, tc.Action, tc.JsonPath, tc.SetValue, tc.Content, logger)
		case "yaml_edit":
			logger.Info("LLM requested homepage yaml_edit", "path", tc.Path, "op", tc.Action)
			return "Tool Output: " + tools.HomepageYamlEdit(homepageCfg, tc.Path, tc.Action, tc.JsonPath, tc.SetValue, logger)
		case "xml_edit":
			logger.Info("LLM requested homepage xml_edit", "path", tc.Path, "op", tc.Action)
			xpath := tc.Xpath
			if xpath == "" {
				xpath = tc.JsonPath
			}
			return "Tool Output: " + tools.HomepageXmlEdit(homepageCfg, tc.Path, tc.Action, xpath, tc.SetValue, logger)
		case "optimize_images":
			logger.Info("LLM requested homepage optimize_images", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageOptimizeImages(homepageCfg, tc.ProjectDir, logger)
		case "dev":
			logger.Info("LLM requested homepage dev server", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageDev(homepageCfg, tc.ProjectDir, 3000, logger)
		case "deploy":
			logger.Info("LLM requested homepage deploy", "host", deployCfg.Host)
			result := tools.HomepageDeploy(homepageCfg, deployCfg, tc.ProjectDir, tc.BuildDir, logger)
			// Auto-log deploy in homepage registry
			if homepageRegistryDB != nil && tc.ProjectDir != "" {
				if proj, err := tools.GetProjectByDir(homepageRegistryDB, tc.ProjectDir); err == nil {
					tools.LogDeploy(homepageRegistryDB, proj.ID, deployCfg.Host)
				}
			}
			return "Tool Output: " + result
		case "test_connection":
			logger.Info("LLM requested homepage test_connection")
			return "Tool Output: " + tools.HomepageTestConnection(deployCfg, logger)
		case "webserver_start":
			logger.Info("LLM requested homepage webserver_start")
			return "Tool Output: " + tools.HomepageWebServerStart(homepageCfg, tc.ProjectDir, tc.BuildDir, logger)
		case "webserver_stop":
			logger.Info("LLM requested homepage webserver_stop")
			return "Tool Output: " + tools.HomepageWebServerStop(homepageCfg, logger)
		case "webserver_status":
			logger.Info("LLM requested homepage webserver_status")
			return "Tool Output: " + tools.HomepageWebServerStatus(homepageCfg, logger)
		case "tunnel":
			logger.Info("LLM requested homepage tunnel", "port", tc.Port)
			port := tc.Port
			if port <= 0 {
				port = 3000
			}
			return "Tool Output: " + tools.HomepageTunnel(homepageCfg, port, logger)
		case "publish_local":
			logger.Info("LLM requested homepage publish_local")
			return "Tool Output: " + tools.HomepagePublishToLocal(homepageCfg, tc.ProjectDir, logger)
		case "deploy_netlify":
			if !cfg.Netlify.AllowDeploy {
				return `Tool Output: {"status":"error","message":"Deployment is disabled. Enable netlify.allow_deploy in config."}`
			}
			if !cfg.Netlify.Enabled {
				return `Tool Output: {"status":"error","message":"Netlify integration is not enabled. Set netlify.enabled=true in config.yaml."}`
			}
			nfToken, nfErr := vault.ReadSecret("netlify_token")
			if nfErr != nil || nfToken == "" {
				return `Tool Output: {"status":"error","message":"Netlify token not found in vault. Store it with key 'netlify_token' via the Config UI."}`
			}
			nfCfg := tools.NetlifyConfig{
				Token:         nfToken,
				DefaultSiteID: cfg.Netlify.DefaultSiteID,
				TeamSlug:      cfg.Netlify.TeamSlug,
			}
			logger.Info("LLM requested homepage deploy_netlify", "project", tc.ProjectDir, "build_dir", tc.BuildDir, "site_id", tc.SiteID, "draft", tc.Draft)
			result := tools.HomepageDeployNetlify(homepageCfg, nfCfg, tc.ProjectDir, tc.BuildDir, tc.SiteID, tc.Title, tc.Draft, logger)
			// Auto-log deploy in homepage registry
			if homepageRegistryDB != nil && tc.ProjectDir != "" {
				deployURL := tc.SiteID
				if deployURL == "" {
					deployURL = "netlify"
				}
				if proj, err := tools.GetProjectByDir(homepageRegistryDB, tc.ProjectDir); err == nil {
					tools.LogDeploy(homepageRegistryDB, proj.ID, deployURL)
					if tc.SiteID != "" {
						tools.UpdateProject(homepageRegistryDB, proj.ID, map[string]interface{}{
							"netlify_site_id": tc.SiteID,
						})
					}
				}
			}
			return "Tool Output: " + result
		// ─── Git operations ─────────────────────────────────────
		case "git_init":
			logger.Info("LLM requested homepage git_init", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageGitInit(homepageCfg, tc.ProjectDir, logger)
		case "git_commit":
			msg := tc.GitMessage
			if msg == "" {
				msg = tc.Message
			}
			logger.Info("LLM requested homepage git_commit", "dir", tc.ProjectDir, "message", msg)
			return "Tool Output: " + tools.HomepageGitCommit(homepageCfg, tc.ProjectDir, msg, logger)
		case "git_status":
			logger.Info("LLM requested homepage git_status", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageGitStatus(homepageCfg, tc.ProjectDir, logger)
		case "git_diff":
			logger.Info("LLM requested homepage git_diff", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageGitDiff(homepageCfg, tc.ProjectDir, logger)
		case "git_log":
			count := tc.Count
			if count <= 0 {
				count = 10
			}
			logger.Info("LLM requested homepage git_log", "dir", tc.ProjectDir, "count", count)
			return "Tool Output: " + tools.HomepageGitLog(homepageCfg, tc.ProjectDir, count, logger)
		case "git_rollback":
			count := tc.Count
			if count <= 0 {
				count = 1
			}
			logger.Info("LLM requested homepage git_rollback", "dir", tc.ProjectDir, "steps", count)
			return "Tool Output: " + tools.HomepageGitRollback(homepageCfg, tc.ProjectDir, count, logger)
		default:
			return `Tool Output: {"status":"error","message":"Unknown homepage operation. Use: init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, optimize_images, dev, deploy, deploy_netlify, test_connection, webserver_start, webserver_stop, webserver_status, publish_local, tunnel, git_init, git_commit, git_status, git_diff, git_log, git_rollback"}`
		}

	case "webdav", "webdav_storage":
		if !cfg.WebDAV.Enabled {
			return `Tool Output: {"status": "error", "message": "WebDAV integration is not enabled. Set webdav.enabled=true in config.yaml."}`
		}
		if cfg.WebDAV.ReadOnly {
			switch tc.Operation {
			case "write", "put", "upload", "mkdir", "create_dir", "delete", "rm", "move", "rename", "mv":
				return `Tool Output: {"status":"error","message":"WebDAV is in read-only mode. Disable webdav.read_only to allow changes."}`
			}
		}
		davCfg := tools.WebDAVConfig{
			AuthType: cfg.WebDAV.AuthType,
			URL:      cfg.WebDAV.URL,
			Username: cfg.WebDAV.Username,
			Password: cfg.WebDAV.Password,
			Token:    cfg.WebDAV.Token,
		}
		path := tc.Path
		if path == "" {
			path = tc.RemotePath
		}
		if path == "" {
			path = tc.FilePath
		}
		switch tc.Operation {
		case "list", "ls":
			logger.Info("LLM requested WebDAV list", "path", path)
			return "Tool Output: " + tools.WebDAVList(davCfg, path)
		case "read", "get", "download":
			logger.Info("LLM requested WebDAV read", "path", path)
			return "Tool Output: " + tools.WebDAVRead(davCfg, path)
		case "write", "put", "upload":
			logger.Info("LLM requested WebDAV write", "path", path)
			content := tc.Content
			if content == "" {
				content = tc.Body
			}
			return "Tool Output: " + tools.WebDAVWrite(davCfg, path, content)
		case "mkdir", "create_dir":
			logger.Info("LLM requested WebDAV mkdir", "path", path)
			return "Tool Output: " + tools.WebDAVMkdir(davCfg, path)
		case "delete", "rm":
			logger.Info("LLM requested WebDAV delete", "path", path)
			return "Tool Output: " + tools.WebDAVDelete(davCfg, path)
		case "move", "rename", "mv":
			logger.Info("LLM requested WebDAV move", "path", path, "destination", tc.Destination)
			dst := tc.Destination
			if dst == "" {
				dst = tc.Dest
			}
			return "Tool Output: " + tools.WebDAVMove(davCfg, path, dst)
		case "info", "stat":
			logger.Info("LLM requested WebDAV info", "path", path)
			return "Tool Output: " + tools.WebDAVInfo(davCfg, path)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown webdav operation. Use: list, read, write, mkdir, delete, move, info"}`
		}

	case "s3_storage", "s3":
		if !cfg.S3.Enabled {
			return `Tool Output: {"status": "error", "message": "S3 integration is not enabled. Set s3.enabled=true in config.yaml."}`
		}
		if cfg.S3.ReadOnly {
			switch tc.Operation {
			case "upload", "delete", "copy", "move":
				return `Tool Output: {"status":"error","message":"S3 is in read-only mode. Disable s3.readonly to allow changes."}`
			}
		}
		s3Cfg := tools.S3Config{
			Endpoint:     cfg.S3.Endpoint,
			Region:       cfg.S3.Region,
			Bucket:       cfg.S3.Bucket,
			AccessKey:    cfg.S3.AccessKey,
			SecretKey:    cfg.S3.SecretKey,
			UsePathStyle: cfg.S3.UsePathStyle,
			Insecure:     cfg.S3.Insecure,
		}
		logger.Info("LLM requested S3 operation", "op", tc.Operation, "bucket", tc.Bucket, "key", tc.Key)
		return "Tool Output: " + tools.ExecuteS3(s3Cfg, tc.Operation, tc.Bucket, tc.Key, tc.LocalPath, tc.Prefix, tc.DestinationBucket, tc.DestinationKey)

	case "paperless", "paperless_ngx":
		if !cfg.PaperlessNGX.Enabled {
			return `Tool Output: {"status": "error", "message": "Paperless-ngx integration is not enabled. Set paperless_ngx.enabled=true in config.yaml."}`
		}
		if cfg.PaperlessNGX.ReadOnly {
			switch tc.Operation {
			case "upload", "post", "update", "patch", "delete", "rm":
				return `Tool Output: {"status":"error","message":"Paperless-ngx is in read-only mode. Disable paperless_ngx.readonly to allow changes."}`
			}
		}
		plCfg := tools.PaperlessConfig{
			URL:      cfg.PaperlessNGX.URL,
			APIToken: cfg.PaperlessNGX.APIToken,
		}
		docID := tc.DocumentID
		if docID == "" {
			docID = tc.ID
		}
		query := tc.Query
		if query == "" {
			query = tc.Content
		}
		switch tc.Operation {
		case "search", "find", "query":
			logger.Info("LLM requested Paperless search", "query", query)
			return "Tool Output: " + tools.PaperlessSearch(plCfg, query, tc.Tags, tc.Name, tc.Category, tc.Limit)
		case "get", "info":
			logger.Info("LLM requested Paperless get", "document_id", docID)
			return "Tool Output: " + tools.PaperlessGet(plCfg, docID)
		case "download", "read", "content":
			logger.Info("LLM requested Paperless download", "document_id", docID)
			return "Tool Output: " + tools.PaperlessDownload(plCfg, docID)
		case "upload", "post":
			logger.Info("LLM requested Paperless upload", "title", tc.Title)
			return "Tool Output: " + tools.PaperlessUpload(plCfg, tc.Title, tc.Content, tc.Tags, tc.Name, tc.Category)
		case "update", "patch":
			logger.Info("LLM requested Paperless update", "document_id", docID)
			return "Tool Output: " + tools.PaperlessUpdate(plCfg, docID, tc.Title, tc.Tags, tc.Name, tc.Category)
		case "delete", "rm":
			logger.Info("LLM requested Paperless delete", "document_id", docID)
			return "Tool Output: " + tools.PaperlessDelete(plCfg, docID)
		case "list_tags", "tags":
			logger.Info("LLM requested Paperless list tags")
			return "Tool Output: " + tools.PaperlessListTags(plCfg)
		case "list_correspondents", "correspondents":
			logger.Info("LLM requested Paperless list correspondents")
			return "Tool Output: " + tools.PaperlessListCorrespondents(plCfg)
		case "list_document_types", "document_types":
			logger.Info("LLM requested Paperless list document types")
			return "Tool Output: " + tools.PaperlessListDocumentTypes(plCfg)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown paperless operation. Use: search, get, download, upload, update, delete, list_tags, list_correspondents, list_document_types"}`
		}

	case "home_assistant", "homeassistant", "ha":
		if !cfg.HomeAssistant.Enabled {
			return `Tool Output: {"status": "error", "message": "Home Assistant integration is not enabled. Set home_assistant.enabled=true in config.yaml."}`
		}
		if cfg.HomeAssistant.ReadOnly {
			switch tc.Operation {
			case "call_service", "service":
				return `Tool Output: {"status":"error","message":"Home Assistant is in read-only mode. Disable home_assistant.read_only to allow changes."}`
			}
		}
		haCfg := tools.HAConfig{
			URL:         cfg.HomeAssistant.URL,
			AccessToken: cfg.HomeAssistant.AccessToken,
		}
		// Merge service_data from Params if ServiceData is nil
		serviceData := tc.ServiceData
		if serviceData == nil && tc.Params != nil {
			if sd, ok := tc.Params["service_data"].(map[string]interface{}); ok {
				serviceData = sd
			}
		}
		switch tc.Operation {
		case "get_states", "list_states", "states":
			logger.Info("LLM requested HA get_states", "domain", tc.Domain)
			return "Tool Output: " + tools.HAGetStates(haCfg, tc.Domain)
		case "get_state", "state":
			logger.Info("LLM requested HA get_state", "entity_id", tc.EntityID)
			return "Tool Output: " + tools.HAGetState(haCfg, tc.EntityID)
		case "call_service", "service":
			logger.Info("LLM requested HA call_service", "domain", tc.Domain, "service", tc.Service, "entity_id", tc.EntityID)
			return "Tool Output: " + tools.HACallService(haCfg, tc.Domain, tc.Service, tc.EntityID, serviceData)
		case "list_services", "services":
			logger.Info("LLM requested HA list_services", "domain", tc.Domain)
			return "Tool Output: " + tools.HAListServices(haCfg, tc.Domain)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown home_assistant operation. Use: get_states, get_state, call_service, list_services"}`
		}

	case "media_registry":
		if mediaRegistryDB == nil {
			return `Tool Output: {"status": "error", "message": "Media registry is not enabled or DB not initialized."}`
		}
		op := tc.Operation
		if op == "" {
			op = "list"
		}
		// Parse tags from array or comma-separated string
		var tags []string
		if arr, ok := tc.Params["tags"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					tags = append(tags, s)
				}
			}
		} else if tc.Tags != "" {
			for _, t := range strings.Split(tc.Tags, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
		var itemID int64
		if v, ok := tc.Params["id"].(float64); ok {
			itemID = int64(v)
		}
		logger.Info("LLM requested media_registry", "operation", op, "media_type", tc.MediaType)
		return "Tool Output: " + tools.DispatchMediaRegistry(mediaRegistryDB, op, tc.Query, tc.MediaType, tc.Description, tags, tc.TagMode, itemID, tc.Limit, tc.Offset, tc.Filename, tc.FilePath)

	case "homepage_registry":
		if homepageRegistryDB == nil {
			return `Tool Output: {"status": "error", "message": "Homepage registry is not enabled or DB not initialized."}`
		}
		op := tc.Operation
		if op == "" {
			op = "list"
		}
		var tags []string
		if arr, ok := tc.Params["tags"].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					tags = append(tags, s)
				}
			}
		} else if tc.Tags != "" {
			for _, t := range strings.Split(tc.Tags, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
		var projectID int64
		if v, ok := tc.Params["id"].(float64); ok {
			projectID = int64(v)
		}
		logger.Info("LLM requested homepage_registry", "operation", op, "name", tc.Name)
		return "Tool Output: " + tools.DispatchHomepageRegistry(homepageRegistryDB, op, tc.Query, tc.Name, tc.Description, tc.Framework, tc.ProjectDir, tc.URL, tc.Status, tc.Reason, tc.Problem, tc.Notes, tags, projectID, "", tc.Limit, tc.Offset)

	case "sql_query":
		if !cfg.SQLConnections.Enabled {
			return `Tool Output: {"status":"error","message":"SQL Connections feature is disabled. Enable sql_connections.enabled in config."}`
		}
		if sqlConnectionsDB == nil || sqlConnectionPool == nil {
			return `Tool Output: {"status":"error","message":"SQL Connections database not available."}`
		}
		if tc.ConnectionName == "" {
			return `Tool Output: {"status":"error","message":"'connection_name' is required"}`
		}
		logger.Info("LLM requested sql_query", "op", tc.Operation, "connection", tc.ConnectionName)
		queryTimeout := time.Duration(cfg.SQLConnections.QueryTimeoutSec) * time.Second
		maxRows := cfg.SQLConnections.MaxResultRows

		switch tc.Operation {
		case "query":
			if tc.SQLQuery == "" {
				return `Tool Output: {"status":"error","message":"'sql_query' is required for query operation"}`
			}
			result, err := sqlconnections.ExecuteQuery(ctx, sqlConnectionPool, sqlConnectionsDB, tc.ConnectionName, tc.SQLQuery, maxRows, queryTimeout)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "result": result})
			return "Tool Output: " + string(b)
		case "describe":
			if tc.TableName == "" {
				return `Tool Output: {"status":"error","message":"'table_name' is required for describe operation"}`
			}
			cols, err := sqlconnections.DescribeTable(ctx, sqlConnectionPool, sqlConnectionsDB, tc.ConnectionName, tc.TableName, queryTimeout)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "table": tc.TableName, "columns": cols})
			return "Tool Output: " + string(b)
		case "list_tables":
			tables, err := sqlconnections.ListTables(ctx, sqlConnectionPool, sqlConnectionsDB, tc.ConnectionName, queryTimeout)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "tables": tables, "count": len(tables)})
			return "Tool Output: " + string(b)
		default:
			return `Tool Output: {"status":"error","message":"Unknown operation. Use: query, describe, list_tables"}`
		}

	case "manage_sql_connections":
		if !cfg.SQLConnections.Enabled {
			return `Tool Output: {"status":"error","message":"SQL Connections feature is disabled. Enable sql_connections.enabled in config."}`
		}
		if sqlConnectionsDB == nil || sqlConnectionPool == nil {
			return `Tool Output: {"status":"error","message":"SQL Connections database not available."}`
		}
		logger.Info("LLM requested manage_sql_connections", "op", tc.Operation)

		switch tc.Operation {
		case "list":
			list, err := sqlconnections.List(sqlConnectionsDB)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			// Redact sensitive fields before returning
			type safeConn struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Driver      string `json:"driver"`
				Host        string `json:"host"`
				Port        int    `json:"port"`
				Database    string `json:"database_name"`
				Description string `json:"description"`
				AllowRead   bool   `json:"allow_read"`
				AllowWrite  bool   `json:"allow_write"`
				AllowChange bool   `json:"allow_change"`
				AllowDelete bool   `json:"allow_delete"`
				SSLMode     string `json:"ssl_mode"`
			}
			var safe []safeConn
			for _, c := range list {
				safe = append(safe, safeConn{
					ID: c.ID, Name: c.Name, Driver: c.Driver,
					Host: c.Host, Port: c.Port, Database: c.DatabaseName,
					Description: c.Description, AllowRead: c.AllowRead,
					AllowWrite: c.AllowWrite, AllowChange: c.AllowChange,
					AllowDelete: c.AllowDelete, SSLMode: c.SSLMode,
				})
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "connections": safe, "count": len(safe)})
			return "Tool Output: " + string(b)

		case "get":
			if tc.ConnectionName == "" {
				return `Tool Output: {"status":"error","message":"'connection_name' is required"}`
			}
			c, err := sqlconnections.GetByName(sqlConnectionsDB, tc.ConnectionName)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(map[string]interface{}{
				"status": "success", "id": c.ID, "name": c.Name, "driver": c.Driver,
				"host": c.Host, "port": c.Port, "database_name": c.DatabaseName,
				"description": c.Description, "allow_read": c.AllowRead,
				"allow_write": c.AllowWrite, "allow_change": c.AllowChange,
				"allow_delete": c.AllowDelete, "ssl_mode": c.SSLMode,
			})
			return "Tool Output: " + string(b)

		case "create":
			if tc.ConnectionName == "" || tc.Driver == "" {
				return `Tool Output: {"status":"error","message":"'connection_name' and 'driver' are required for create"}`
			}
			allowRead := true
			if tc.AllowRead != nil {
				allowRead = *tc.AllowRead
			}
			allowWrite := false
			if tc.AllowWrite != nil {
				allowWrite = *tc.AllowWrite
			}
			allowChange := false
			if tc.AllowChange != nil {
				allowChange = *tc.AllowChange
			}
			allowDelete := false
			if tc.AllowDelete != nil {
				allowDelete = *tc.AllowDelete
			}

			sslMode := tc.SSLMode
			if sslMode == "" {
				sslMode = "disable"
			}

			// Store credentials in vault
			vaultKey := ""
			username := ""
			password := ""
			if tc.Params != nil {
				if u, ok := tc.Params["username"].(string); ok {
					username = u
				}
				if p, ok := tc.Params["password"].(string); ok {
					password = p
				}
			}
			if username != "" || password != "" {
				credJSON, marshalErr := sqlconnections.MarshalCredentials(username, password)
				if marshalErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to marshal credentials: %s"}`, marshalErr.Error())
				}
				vaultKey = "sqlconn_" + tc.ConnectionName
				if err := vault.WriteSecret(vaultKey, credJSON); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to store credentials: %s"}`, err.Error())
				}
			}

			id, err := sqlconnections.Create(sqlConnectionsDB,
				tc.ConnectionName, tc.Driver, tc.Host, tc.Port, tc.DatabaseName, tc.Description,
				allowRead, allowWrite, allowChange, allowDelete, vaultKey, sslMode)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Connection created","id":"%s","name":"%s"}`, id, tc.ConnectionName)

		case "update":
			if tc.ConnectionName == "" {
				return `Tool Output: {"status":"error","message":"'connection_name' is required for update"}`
			}
			existing, err := sqlconnections.GetByName(sqlConnectionsDB, tc.ConnectionName)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			if tc.Description != "" {
				existing.Description = tc.Description
			}
			if tc.Host != "" {
				existing.Host = tc.Host
			}
			if tc.Port > 0 {
				existing.Port = tc.Port
			}
			if tc.DatabaseName != "" {
				existing.DatabaseName = tc.DatabaseName
			}
			if tc.SSLMode != "" {
				existing.SSLMode = tc.SSLMode
			}
			if tc.AllowRead != nil {
				existing.AllowRead = *tc.AllowRead
			}
			if tc.AllowWrite != nil {
				existing.AllowWrite = *tc.AllowWrite
			}
			if tc.AllowChange != nil {
				existing.AllowChange = *tc.AllowChange
			}
			if tc.AllowDelete != nil {
				existing.AllowDelete = *tc.AllowDelete
			}

			// Update credentials if provided
			username := ""
			password := ""
			if tc.Params != nil {
				if u, ok := tc.Params["username"].(string); ok {
					username = u
				}
				if p, ok := tc.Params["password"].(string); ok {
					password = p
				}
			}
			if username != "" || password != "" {
				credJSON, marshalErr := sqlconnections.MarshalCredentials(username, password)
				if marshalErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to marshal credentials: %s"}`, marshalErr.Error())
				}
				vaultKey := existing.VaultSecretID
				if vaultKey == "" {
					vaultKey = "sqlconn_" + tc.ConnectionName
				}
				if err := vault.WriteSecret(vaultKey, credJSON); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to update credentials: %s"}`, err.Error())
				}
				existing.VaultSecretID = vaultKey
			}

			// Close existing pool connection to force reconnect with new settings
			sqlConnectionPool.CloseConnection(existing.ID)

			if err := sqlconnections.Update(sqlConnectionsDB,
				existing.ID, existing.Name, existing.Driver, existing.Host, existing.Port,
				existing.DatabaseName, existing.Description,
				existing.AllowRead, existing.AllowWrite, existing.AllowChange, existing.AllowDelete,
				existing.VaultSecretID, existing.SSLMode); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Connection updated","name":"%s"}`, tc.ConnectionName)

		case "delete":
			if tc.ConnectionName == "" {
				return `Tool Output: {"status":"error","message":"'connection_name' is required for delete"}`
			}
			existing, err := sqlconnections.GetByName(sqlConnectionsDB, tc.ConnectionName)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			sqlConnectionPool.CloseConnection(existing.ID)
			if err := sqlconnections.Delete(sqlConnectionsDB, existing.ID); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			// Clean up vault secret
			if existing.VaultSecretID != "" {
				_ = vault.DeleteSecret(existing.VaultSecretID)
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Connection deleted","name":"%s"}`, tc.ConnectionName)

		case "test":
			if tc.ConnectionName == "" {
				return `Tool Output: {"status":"error","message":"'connection_name' is required for test"}`
			}
			rec, err := sqlconnections.GetByName(sqlConnectionsDB, tc.ConnectionName)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			if err := sqlConnectionPool.TestConnection(rec); err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Connection test failed: %s"}`, err.Error())
			}
			return fmt.Sprintf(`Tool Output: {"status":"success","message":"Connection test successful","name":"%s","driver":"%s"}`, tc.ConnectionName, rec.Driver)

		case "docker_create":
			if tc.ConnectionName == "" {
				return `Tool Output: {"status":"error","message":"'connection_name' is required for docker_create"}`
			}
			templateName := tc.DockerTemplate
			if templateName == "" {
				if tc.Params != nil {
					if t, ok := tc.Params["docker_template"].(string); ok {
						templateName = t
					}
				}
			}
			if templateName == "" {
				return `Tool Output: {"status":"error","message":"'docker_template' is required (postgres, mysql, mariadb)"}`
			}
			dbName := tc.DatabaseName
			if dbName == "" {
				dbName = tc.ConnectionName
			}
			dockerReq, err := sqlconnections.PrepareDockerDB(templateName, tc.ConnectionName, dbName)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err.Error())
			}
			b, _ := json.Marshal(map[string]interface{}{
				"status":  "success",
				"message": "Docker database prepared. Use the 'docker' tool with operation 'run' to start the container, then create the connection with 'manage_sql_connections' create.",
				"docker":  dockerReq,
			})
			return "Tool Output: " + string(b)

		default:
			return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, get, create, update, delete, test, docker_create"}`
		}

	default:
		return dispatchNotHandled
	}
}
