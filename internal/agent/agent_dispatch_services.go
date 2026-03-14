package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/meshcentral"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// dispatchServices handles media, infrastructure management, and platform tool calls
// (vision, transcribe, meshcentral, docker, homepage, webdav, home_assistant).
func dispatchServices(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManager *tools.MissionManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
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
			result, err := mcClient.WakeOnLan([]string{tc.NodeID})
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send wake magic packet: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

		case "power_action":
			if tc.NodeID == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' is required for power_action"}`
			}
			if tc.PowerAction < 1 || tc.PowerAction > 4 {
				return `Tool Output: {"status": "error", "message": "Invalid power action. 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset"}`
			}
			result, err := mcClient.PowerAction([]string{tc.NodeID}, tc.PowerAction)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send power action: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

		case "run_command":
			if tc.NodeID == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for run_command"}`
			}
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
		case "init", "start", "stop", "rebuild", "destroy", "webserver_start", "webserver_stop":
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
			return "Tool Output: " + tools.HomepageExec(homepageCfg, tc.Command, logger)
		case "init_project":
			logger.Info("LLM requested homepage init_project", "framework", tc.Framework, "name", tc.Name)
			result := tools.HomepageInitProject(homepageCfg, tc.Framework, tc.Name, logger)
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
			logger.Info("LLM requested homepage build", "dir", tc.ProjectDir)
			result := tools.HomepageBuild(homepageCfg, tc.ProjectDir, logger)
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
		default:
			return `Tool Output: {"status":"error","message":"Unknown homepage operation. Use: init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, optimize_images, dev, deploy, deploy_netlify, test_connection, webserver_start, webserver_stop, webserver_status, publish_local"}`
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
			URL:      cfg.WebDAV.URL,
			Username: cfg.WebDAV.Username,
			Password: cfg.WebDAV.Password,
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
		return "Tool Output: " + tools.DispatchMediaRegistry(mediaRegistryDB, op, tc.Query, tc.MediaType, tc.Description, tags, tc.TagMode, itemID, tc.Limit, tc.Offset)

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

	default:
		return dispatchNotHandled
	}
}
