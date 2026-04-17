package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"aurago/internal/meshcentral"
	"aurago/internal/tools"
)

// dispatchServices handles media, infrastructure management, and platform tool calls
// (vision, transcribe, meshcentral, docker, homepage, webdav, home_assistant).
func dispatchServices(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	vault := dc.Vault
	mediaRegistryDB := dc.MediaRegistryDB
	homepageRegistryDB := dc.HomepageRegistryDB
	sessionID := dc.SessionID
	budgetTracker := dc.BudgetTracker
	handled := true

	result := func() string {
		switch tc.Action {
		case "analyze_image", "vision":
			if budgetTracker != nil && budgetTracker.IsBlocked("vision") {
				return `Tool Output: {"status": "error", "message": "Vision blocked: daily budget exceeded. Try again tomorrow."}`
			}
			req := decodeImageAnalysisArgs(tc)
			logger.Info("LLM requested image analysis", "file_path", req.FilePath)
			fpath := req.FilePath
			if fpath == "" {
				return `Tool Output: {"status": "error", "message": "'file_path' is required for analyze_image"}`
			}
			if strings.Contains(fpath, "..") {
				return `Tool Output: {"status": "error", "message": "path traversal sequences ('..') are not allowed"}`
			}
			prompt := req.Prompt
			if prompt == "" {
				prompt = "Describe this image in detail. What do you see? If there is text, transcribe it. If there are people, describe their actions."
			}
			result, pTokens, cTokens, err := tools.AnalyzeImageWithPrompt(fpath, prompt, cfg)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Vision analysis failed: %v"}`, err)
			}
			if budgetTracker != nil {
				vModel := cfg.Vision.Model
				if vModel == "" {
					vModel = "google/gemini-2.5-flash-lite-preview-09-2025"
				}
				budgetTracker.RecordForCategory("vision", vModel, pTokens, cTokens)
			}
			return fmt.Sprintf("Tool Output: %s", result)

		case "transcribe_audio", "speech_to_text":
			if budgetTracker != nil && budgetTracker.IsBlocked("stt") {
				return `Tool Output: {"status": "error", "message": "Speech-to-text blocked: daily budget exceeded. Try again tomorrow."}`
			}
			req := decodeImageAnalysisArgs(tc)
			logger.Info("LLM requested audio transcription", "file_path", req.FilePath)
			fpath := req.FilePath
			if fpath == "" {
				return `Tool Output: {"status": "error", "message": "'file_path' is required for transcribe_audio"}`
			}
			if strings.Contains(fpath, "..") {
				return `Tool Output: {"status": "error", "message": "path traversal sequences ('..') are not allowed"}`
			}
			result, sttCost, err := tools.TranscribeAudioFile(fpath, cfg)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Transcription failed: %v"}`, err)
			}
			if budgetTracker != nil {
				budgetTracker.RecordCostForCategory("stt", sttCost)
			}
			return fmt.Sprintf("Tool Output: %s", result)

		case "meshcentral":
			if !cfg.MeshCentral.Enabled {
				return `Tool Output: {"status": "error", "message": "MeshCentral integration is not enabled in config.yaml."}`
			}

			req := decodeMeshCentralArgs(tc)
			logger.Info("LLM requested MeshCentral operation", "op", req.Operation)

			op := req.Operation

			if cfg.MeshCentral.ReadOnly {
				switch op {
				case "list_groups", "list_devices":
					// allowed in read-only mode
				default:
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MeshCentral operation '%s' blocked: meshcentral.readonly is enabled."}`, req.Operation)
				}
			}

			for _, blocked := range cfg.MeshCentral.BlockedOperations {
				if normalizeMeshCentralOp(blocked) == op && op != "" {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MeshCentral operation '%s' blocked by policy (meshcentral.blocked_operations)."}`, req.Operation)
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

			mcClient, err := meshcentral.NewClient(cfg.MeshCentral.URL, cfg.MeshCentral.Username, pass, token, cfg.MeshCentral.Insecure)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Invalid MeshCentral configuration: %v"}`, err)
			}
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
				nodes, err := mcClient.ListDevices(req.MeshID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to list devices: %v"}`, err)
				}
				b, _ := json.Marshal(nodes)
				return fmt.Sprintf(`Tool Output: {"status": "success", "devices": %s}`, string(b))

			case "wake":
				if req.NodeID == "" {
					return `Tool Output: {"status": "error", "message": "'node_id' is required for wake"}`
				}
				if !strings.HasPrefix(req.NodeID, "node//") {
					return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
				}
				logger.Info("MeshCentral wake_on_lan", "node_id", req.NodeID, "session_id", sessionID)
				result, err := mcClient.WakeOnLan([]string{req.NodeID})
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send wake magic packet: %v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

			case "power_action":
				if req.NodeID == "" {
					return `Tool Output: {"status": "error", "message": "'node_id' is required for power_action"}`
				}
				if !strings.HasPrefix(req.NodeID, "node//") {
					return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
				}
				if req.PowerAction < 1 || req.PowerAction > 4 {
					return `Tool Output: {"status": "error", "message": "Invalid power action. 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset"}`
				}
				logger.Info("MeshCentral power_action", "node_id", req.NodeID, "power_action", req.PowerAction, "session_id", sessionID)
				result, err := mcClient.PowerAction([]string{req.NodeID}, req.PowerAction)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send power action: %v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

			case "run_command":
				if req.NodeID == "" || req.Command == "" {
					return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for run_command"}`
				}
				if !strings.HasPrefix(req.NodeID, "node//") {
					return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
				}
				logger.Info("MeshCentral run_command", "node_id", req.NodeID, "command", req.Command, "session_id", sessionID)
				result, err := mcClient.RunCommand(req.NodeID, req.Command)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to run command: %v"}`, err)
				}
				// Format result for display
				resultJSON, _ := json.Marshal(result)
				return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(resultJSON))

			case "shell":
				if req.NodeID == "" || req.Command == "" {
					return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for shell"}`
				}
				if !strings.HasPrefix(req.NodeID, "node//") {
					return `Tool Output: {"status": "error", "message": "invalid node_id: must start with 'node//'"}`
				}
				logger.Info("MeshCentral shell", "node_id", req.NodeID, "command", req.Command, "session_id", sessionID)
				result, err := mcClient.Shell(req.NodeID, req.Command)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to execute shell command: %v"}`, err)
				}
				// Format result for display
				resultJSON, _ := json.Marshal(result)
				return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(resultJSON))

			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, req.Operation)
			}

		case "docker", "docker_management":
			if !cfg.Docker.Enabled {
				return `Tool Output: {"status": "error", "message": "Docker integration is not enabled. Set docker.enabled=true in config.yaml."}`
			}
			req := decodeDockerArgs(tc)
			if cfg.Docker.ReadOnly {
				switch req.Operation {
				case "start", "stop", "restart", "pause", "unpause", "remove", "rm", "create", "create_container", "run", "pull_image", "pull", "remove_image", "rmi":
					return `Tool Output: {"status":"error","message":"Docker is in read-only mode. Disable docker.read_only to allow changes."}`
				}
			}
			dockerCfg := tools.DockerConfig{Host: cfg.Docker.Host, WorkspaceDir: cfg.Directories.WorkspaceDir}
			containerID := req.targetContainerID()
			switch req.Operation {
			case "list_containers", "ps":
				logger.Info("LLM requested Docker list_containers", "all", req.All)
				return "Tool Output: " + tools.DockerListContainers(dockerCfg, req.All)
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
				logger.Info("LLM requested Docker remove", "container_id", containerID, "force", req.Force)
				return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "remove", req.Force)
			case "logs":
				logger.Info("LLM requested Docker logs", "container_id", containerID, "tail", req.Tail)
				return "Tool Output: " + tools.DockerContainerLogs(dockerCfg, containerID, req.Tail)
			case "create", "create_container", "run":
				logger.Info("LLM requested Docker create", "image", req.Image, "name", req.Name)
				var cmd []string
				if req.Command != "" {
					cmd = strings.Fields(req.Command)
				}
				restart := req.Restart
				if restart == "" {
					restart = "no"
				}
				result := tools.DockerCreateContainer(dockerCfg, req.Name, req.Image, req.Env, req.Ports, req.Volumes, cmd, restart)
				// Auto-start if operation was "run"
				if req.Operation == "run" {
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
				logger.Info("LLM requested Docker pull", "image", req.Image)
				return "Tool Output: " + tools.DockerPullImage(dockerCfg, req.Image)
			case "remove_image", "rmi":
				logger.Info("LLM requested Docker remove_image", "image", req.Image, "force", req.Force)
				return "Tool Output: " + tools.DockerRemoveImage(dockerCfg, req.Image, req.Force)
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
				logger.Info("LLM requested Docker exec", "container_id", containerID, "cmd", Truncate(req.Command, 200))
				return "Tool Output: " + tools.DockerExec(dockerCfg, containerID, req.Command, req.User)
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
				logger.Info("LLM requested Docker cp", "container_id", containerID, "src", req.Source, "dest", req.Destination, "direction", req.Direction)
				return "Tool Output: " + tools.DockerCopy(dockerCfg, containerID, req.Source, req.Destination, req.Direction)
			case "create_network":
				logger.Info("LLM requested Docker create_network", "name", req.Name, "driver", req.Driver)
				return "Tool Output: " + tools.DockerCreateNetwork(dockerCfg, req.Name, req.Driver)
			case "remove_network":
				logger.Info("LLM requested Docker remove_network", "name", req.Name)
				return "Tool Output: " + tools.DockerRemoveNetwork(dockerCfg, req.Name)
			case "connect":
				logger.Info("LLM requested Docker connect", "container_id", containerID, "network", req.Network)
				return "Tool Output: " + tools.DockerConnectNetwork(dockerCfg, containerID, req.Network)
			case "disconnect":
				logger.Info("LLM requested Docker disconnect", "container_id", containerID, "network", req.Network)
				return "Tool Output: " + tools.DockerDisconnectNetwork(dockerCfg, containerID, req.Network)
			case "create_volume":
				logger.Info("LLM requested Docker create_volume", "name", req.Name, "driver", req.Driver)
				return "Tool Output: " + tools.DockerCreateVolume(dockerCfg, req.Name, req.Driver)
			case "remove_volume":
				logger.Info("LLM requested Docker remove_volume", "name", req.Name, "force", req.Force)
				return "Tool Output: " + tools.DockerRemoveVolume(dockerCfg, req.Name, req.Force)
			case "compose":
				logger.Info("LLM requested Docker compose", "file", req.File, "cmd", req.Command)
				return "Tool Output: " + tools.DockerCompose(dockerCfg, req.File, req.Command)
			default:
				return `Tool Output: {"status": "error", "message": "Unknown docker operation. Use: list_containers, inspect, start, stop, restart, pause, unpause, remove, logs, create, run, list_images, pull, remove_image, list_networks, create_network, remove_network, connect, disconnect, list_volumes, create_volume, remove_volume, exec, stats, top, port, cp, compose, info"}`
			}

		case "homepage", "homepage_tool":
			if !cfg.Homepage.Enabled {
				return `Tool Output: {"status": "error", "message": "Homepage tool is not enabled. Set homepage.enabled=true in config.yaml."}`
			}
			req := decodeHomepageArgs(tc)
			homepageCfg := tools.HomepageConfig{
				DockerHost:            cfg.Docker.Host,
				WorkspacePath:         cfg.Homepage.WorkspacePath,
				AgentWorkspaceDir:     cfg.Directories.WorkspaceDir,
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
			switch req.Operation {
			case "deploy", "test_connection":
				if !cfg.Homepage.AllowDeploy {
					return `Tool Output: {"status":"error","message":"Remote deployment is disabled by administrator. The homepage.allow_deploy setting is false in config.yaml. For local serving use operation 'publish_local' instead — it does not require allow_deploy. Do NOT retry this operation — use publish_local or inform the user that remote deployment must be enabled in the configuration."}`
				}
			case "init", "start", "stop", "rebuild", "destroy", "webserver_start", "webserver_stop", "exec":
				if !cfg.Homepage.AllowContainerManagement {
					return `Tool Output: {"status":"error","message":"Container management is disabled. Enable homepage.allow_container_management in config."}`
				}
			}

			switch req.Operation {
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
				logger.Info("LLM requested homepage exec", "cmd", req.Command)
				var execEnv []string
				// Auto-inject Netlify auth token when the command invokes the netlify CLI
				if vault != nil && strings.Contains(req.Command, "netlify ") {
					if nfTok, tokErr := vault.ReadSecret("netlify_token"); tokErr == nil && nfTok != "" {
						execEnv = []string{"NETLIFY_AUTH_TOKEN=" + nfTok}
					}
				}
				return "Tool Output: " + tools.HomepageExec(homepageCfg, req.Command, execEnv, logger)
			case "init_project":
				logger.Info("LLM requested homepage init_project", "framework", req.Framework, "name", req.Name, "template", req.Template)
				result := tools.HomepageInitProject(homepageCfg, req.Framework, req.Name, req.Template, logger)
				// Auto-register project in homepage registry
				if homepageRegistryDB != nil && req.Name != "" {
					tools.RegisterProject(homepageRegistryDB, tools.HomepageProject{
						Name:      req.Name,
						Framework: req.Framework,
						Status:    "active",
						Tags:      []string{"auto-registered"},
					})
				}
				return "Tool Output: " + result
			case "build":
				logger.Info("LLM requested homepage build", "dir", req.ProjectDir, "auto_fix", req.AutoFix)
				var result string
				if req.AutoFix {
					result = tools.HomepageBuildWithAutoFix(homepageCfg, req.ProjectDir, logger)
				} else {
					result = tools.HomepageBuild(homepageCfg, req.ProjectDir, logger)
				}
				// Auto-log edit in homepage registry
				if homepageRegistryDB != nil && req.ProjectDir != "" {
					if proj, err := tools.GetProjectByDir(homepageRegistryDB, req.ProjectDir); err == nil {
						tools.LogEdit(homepageRegistryDB, proj.ID, "build")
					}
				}
				return "Tool Output: " + result
			case "install_deps":
				logger.Info("LLM requested homepage install_deps", "packages", req.Packages)
				return "Tool Output: " + tools.HomepageInstallDeps(homepageCfg, req.ProjectDir, req.Packages, logger)
			case "lighthouse":
				logger.Info("LLM requested homepage lighthouse", "url", req.URL)
				result := tools.HomepageLighthouse(homepageCfg, req.URL, logger)
				// Auto-log lighthouse score in homepage registry
				if homepageRegistryDB != nil && req.URL != "" {
					projects, _, _ := tools.SearchProjects(homepageRegistryDB, req.URL, "", nil, 1, 0)
					if len(projects) > 0 {
						var scoreMap map[string]interface{}
						var perfScore float64
						if err := json.Unmarshal([]byte(result), &scoreMap); err == nil {
							if perf, ok := scoreMap["performance"].(float64); ok {
								perfScore = perf
							}
						}
						if perfScore > 0 {
							tools.UpdateProject(homepageRegistryDB, projects[0].ID, map[string]interface{}{
								"lighthouse_score": perfScore,
							})
						}
					}
				}
				return "Tool Output: " + result
			case "screenshot":
				logger.Info("LLM requested homepage screenshot", "url", req.URL, "viewport", req.Viewport)
				return "Tool Output: " + tools.HomepageScreenshot(ctx, homepageCfg, req.URL, req.Viewport, logger)
			case "check_js":
				logger.Info("LLM requested homepage check_js", "url", req.URL)
				return "Tool Output: " + tools.HomepageCheckJS(ctx, homepageCfg, req.URL, logger)
			case "lint":
				logger.Info("LLM requested homepage lint", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageLint(homepageCfg, req.ProjectDir, logger)
			case "list_files":
				logger.Info("LLM requested homepage list_files", "path", req.Path)
				return "Tool Output: " + tools.HomepageListFiles(homepageCfg, req.Path, logger)
			case "read_file":
				logger.Info("LLM requested homepage read_file", "path", req.Path)
				return "Tool Output: " + tools.HomepageReadFile(homepageCfg, req.Path, logger)
			case "write_file":
				logger.Info("LLM requested homepage write_file", "path", req.Path)
				return "Tool Output: " + tools.HomepageWriteFile(homepageCfg, req.Path, req.Content, logger)
			case "edit_file":
				editOp := req.SubOperation
				logger.Info("LLM requested homepage edit_file", "path", req.Path, "op", editOp)
				return "Tool Output: " + tools.HomepageEditFile(homepageCfg, req.Path, editOp, req.Old, req.New, req.Marker, req.Content, req.StartLine, req.EndLine, logger)
			case "json_edit":
				editOp := req.SubOperation
				logger.Info("LLM requested homepage json_edit", "path", req.Path, "op", editOp)
				return "Tool Output: " + tools.HomepageJsonEdit(homepageCfg, req.Path, editOp, req.JsonPath, req.SetValue, req.Content, logger)
			case "yaml_edit":
				editOp := req.SubOperation
				logger.Info("LLM requested homepage yaml_edit", "path", req.Path, "op", editOp)
				return "Tool Output: " + tools.HomepageYamlEdit(homepageCfg, req.Path, editOp, req.JsonPath, req.SetValue, logger)
			case "xml_edit":
				editOp := req.SubOperation
				logger.Info("LLM requested homepage xml_edit", "path", req.Path, "op", editOp)
				xpath := req.Xpath
				if xpath == "" {
					xpath = req.JsonPath
				}
				return "Tool Output: " + tools.HomepageXmlEdit(homepageCfg, req.Path, editOp, xpath, req.SetValue, logger)
			case "optimize_images":
				logger.Info("LLM requested homepage optimize_images", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageOptimizeImages(homepageCfg, req.ProjectDir, logger)
			case "dev":
				logger.Info("LLM requested homepage dev server", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageDev(homepageCfg, req.ProjectDir, 3000, logger)
			case "deploy":
				logger.Info("LLM requested homepage deploy", "host", deployCfg.Host)
				result := tools.HomepageDeploy(homepageCfg, deployCfg, req.ProjectDir, req.BuildDir, logger)
				// Auto-log deploy in homepage registry
				if homepageRegistryDB != nil && req.ProjectDir != "" {
					if proj, err := tools.GetProjectByDir(homepageRegistryDB, req.ProjectDir); err == nil {
						tools.LogDeploy(homepageRegistryDB, proj.ID, deployCfg.Host)
					}
				}
				return "Tool Output: " + result
			case "test_connection":
				logger.Info("LLM requested homepage test_connection")
				return "Tool Output: " + tools.HomepageTestConnection(deployCfg, logger)
			case "webserver_start":
				logger.Info("LLM requested homepage webserver_start")
				return "Tool Output: " + tools.HomepageWebServerStart(homepageCfg, req.ProjectDir, req.BuildDir, logger)
			case "webserver_stop":
				logger.Info("LLM requested homepage webserver_stop")
				return "Tool Output: " + tools.HomepageWebServerStop(homepageCfg, logger)
			case "webserver_status":
				logger.Info("LLM requested homepage webserver_status")
				return "Tool Output: " + tools.HomepageWebServerStatus(homepageCfg, logger)
			case "tunnel":
				logger.Info("LLM requested homepage tunnel", "port", req.Port)
				port := req.Port
				if port <= 0 {
					port = 3000
				}
				return "Tool Output: " + tools.HomepageTunnel(homepageCfg, port, logger)
			case "publish_local":
				logger.Info("LLM requested homepage publish_local")
				return "Tool Output: " + tools.HomepagePublishToLocal(homepageCfg, req.ProjectDir, logger)
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
				logger.Info("LLM requested homepage deploy_netlify", "project", req.ProjectDir, "build_dir", req.BuildDir, "site_id", req.SiteID, "draft", req.Draft)
				result := tools.HomepageDeployNetlify(homepageCfg, nfCfg, req.ProjectDir, req.BuildDir, req.SiteID, req.Title, req.Draft, logger)
				// Auto-log deploy in homepage registry
				if homepageRegistryDB != nil && req.ProjectDir != "" {
					deployURL := req.SiteID
					if deployURL == "" {
						deployURL = "netlify"
					}
					if proj, err := tools.GetProjectByDir(homepageRegistryDB, req.ProjectDir); err == nil {
						tools.LogDeploy(homepageRegistryDB, proj.ID, deployURL)
						if req.SiteID != "" {
							tools.UpdateProject(homepageRegistryDB, proj.ID, map[string]interface{}{
								"netlify_site_id": req.SiteID,
							})
						}
					}
				}
				return "Tool Output: " + result
			// ─── Git operations ─────────────────────────────────────
			case "git_init":
				logger.Info("LLM requested homepage git_init", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageGitInit(homepageCfg, req.ProjectDir, logger)
			case "git_commit":
				msg := req.GitMessage
				if msg == "" {
					msg = req.Message
				}
				logger.Info("LLM requested homepage git_commit", "dir", req.ProjectDir, "message", msg)
				return "Tool Output: " + tools.HomepageGitCommit(homepageCfg, req.ProjectDir, msg, logger)
			case "git_status":
				logger.Info("LLM requested homepage git_status", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageGitStatus(homepageCfg, req.ProjectDir, logger)
			case "git_diff":
				logger.Info("LLM requested homepage git_diff", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageGitDiff(homepageCfg, req.ProjectDir, logger)
			case "git_log":
				count := req.Count
				if count <= 0 {
					count = 10
				}
				logger.Info("LLM requested homepage git_log", "dir", req.ProjectDir, "count", count)
				return "Tool Output: " + tools.HomepageGitLog(homepageCfg, req.ProjectDir, count, logger)
			case "git_rollback":
				count := req.Count
				if count <= 0 {
					count = 1
				}
				logger.Info("LLM requested homepage git_rollback", "dir", req.ProjectDir, "steps", count)
				return "Tool Output: " + tools.HomepageGitRollback(homepageCfg, req.ProjectDir, count, logger)
			case "save_revision":
				logger.Info("LLM requested homepage save_revision", "dir", req.ProjectDir, "message", req.Message)
				return "Tool Output: " + tools.HomepageSaveRevision(homepageCfg, homepageRegistryDB, req.ProjectDir, req.Message, req.Reason, logger)
			case "list_revisions":
				logger.Info("LLM requested homepage list_revisions", "dir", req.ProjectDir, "count", req.Count)
				count := req.Count
				if count <= 0 {
					count = 20
				}
				return "Tool Output: " + tools.HomepageListRevisions(homepageRegistryDB, req.ProjectDir, count, 0, logger)
			case "get_revision":
				logger.Info("LLM requested homepage get_revision", "revision_id", req.RevisionID)
				return "Tool Output: " + tools.HomepageGetRevision(homepageRegistryDB, req.RevisionID, logger)
			case "diff_revision":
				logger.Info("LLM requested homepage diff_revision", "revision_id", req.RevisionID, "path", req.Path)
				return "Tool Output: " + tools.HomepageDiffRevision(homepageRegistryDB, req.RevisionID, req.Path, logger)
			case "restore_revision":
				logger.Info("LLM requested homepage restore_revision", "revision_id", req.RevisionID, "path", req.Path)
				return "Tool Output: " + tools.HomepageRestoreRevision(homepageCfg, homepageRegistryDB, req.RevisionID, req.Path, logger)
			case "revision_status":
				logger.Info("LLM requested homepage revision_status", "dir", req.ProjectDir)
				return "Tool Output: " + tools.HomepageRevisionStatus(homepageCfg, homepageRegistryDB, req.ProjectDir, logger)
			default:
				return `Tool Output: {"status":"error","message":"Unknown homepage operation. Use: init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, optimize_images, dev, deploy, deploy_netlify, test_connection, webserver_start, webserver_stop, webserver_status, publish_local, tunnel, git_init, git_commit, git_status, git_diff, git_log, git_rollback, save_revision, list_revisions, get_revision, diff_revision, restore_revision, revision_status"}`
			}

		case "webdav", "webdav_storage":
			if !cfg.WebDAV.Enabled {
				return `Tool Output: {"status": "error", "message": "WebDAV integration is not enabled. Set webdav.enabled=true in config.yaml."}`
			}
			req := decodeWebDAVArgs(tc)
			if cfg.WebDAV.ReadOnly {
				switch req.Operation {
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
			path := req.Path
			switch req.Operation {
			case "list", "ls":
				logger.Info("LLM requested WebDAV list", "path", path)
				return "Tool Output: " + tools.WebDAVList(davCfg, path)
			case "read", "get", "download":
				logger.Info("LLM requested WebDAV read", "path", path)
				return "Tool Output: " + tools.WebDAVRead(davCfg, path)
			case "write", "put", "upload":
				logger.Info("LLM requested WebDAV write", "path", path)
				return "Tool Output: " + tools.WebDAVWrite(davCfg, path, req.Content)
			case "mkdir", "create_dir":
				logger.Info("LLM requested WebDAV mkdir", "path", path)
				return "Tool Output: " + tools.WebDAVMkdir(davCfg, path)
			case "delete", "rm":
				logger.Info("LLM requested WebDAV delete", "path", path)
				return "Tool Output: " + tools.WebDAVDelete(davCfg, path)
			case "move", "rename", "mv":
				logger.Info("LLM requested WebDAV move", "path", path, "destination", req.Destination)
				return "Tool Output: " + tools.WebDAVMove(davCfg, path, req.Destination)
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
			req := decodeS3Args(tc)
			if cfg.S3.ReadOnly {
				switch req.Operation {
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
			logger.Info("LLM requested S3 operation", "op", req.Operation, "bucket", req.Bucket, "key", req.Key)
			return "Tool Output: " + tools.ExecuteS3(s3Cfg, req.Operation, req.Bucket, req.Key, req.LocalPath, req.Prefix, req.DestinationBucket, req.DestinationKey)

		case "paperless", "paperless_ngx":
			if result, ok := handleDirectBuiltinSkillAction(ctx, tc, dc); ok {
				return result
			}
			return unexpectedBuiltinActionError(tc.Action)

		case "home_assistant", "homeassistant", "ha":
			if !cfg.HomeAssistant.Enabled {
				return `Tool Output: {"status": "error", "message": "Home Assistant integration is not enabled. Set home_assistant.enabled=true in config.yaml."}`
			}
			req := decodeHomeAssistantArgs(tc)
			if cfg.HomeAssistant.ReadOnly {
				switch req.Operation {
				case "call_service", "service":
					return `Tool Output: {"status":"error","message":"Home Assistant is in read-only mode. Disable home_assistant.read_only to allow changes."}`
				}
			}
			haCfg := tools.HAConfig{
				URL:         cfg.HomeAssistant.URL,
				AccessToken: cfg.HomeAssistant.AccessToken,
			}
			switch req.Operation {
			case "get_states", "list_states", "states":
				logger.Info("LLM requested HA get_states", "domain", req.Domain)
				return "Tool Output: " + tools.HAGetStates(haCfg, req.Domain)
			case "get_state", "state":
				logger.Info("LLM requested HA get_state", "entity_id", req.EntityID)
				return "Tool Output: " + tools.HAGetState(haCfg, req.EntityID)
			case "call_service", "service":
				logger.Info("LLM requested HA call_service", "domain", req.Domain, "service", req.Service, "entity_id", req.EntityID)
				return "Tool Output: " + tools.HACallService(haCfg, req.Domain, req.Service, req.EntityID, req.ServiceData)
			case "list_services", "services":
				logger.Info("LLM requested HA list_services", "domain", req.Domain)
				return "Tool Output: " + tools.HAListServices(haCfg, req.Domain)
			default:
				return `Tool Output: {"status": "error", "message": "Unknown home_assistant operation. Use: get_states, get_state, call_service, list_services"}`
			}

		case "media_registry":
			if mediaRegistryDB == nil {
				return `Tool Output: {"status": "error", "message": "Media registry is not enabled or DB not initialized."}`
			}
			req := decodeMediaRegistryArgs(tc)
			op := req.Operation
			if op == "" {
				op = "list"
			}
			logger.Info("LLM requested media_registry", "operation", op, "media_type", req.MediaType)
			return "Tool Output: " + tools.DispatchMediaRegistry(mediaRegistryDB, op, req.Query, req.MediaType, req.Description, req.Tags, req.TagMode, req.ID, req.Limit, req.Offset, req.Filename, req.FilePath, req.WebPath)

		case "homepage_registry":
			if homepageRegistryDB == nil {
				return `Tool Output: {"status": "error", "message": "Homepage registry is not enabled or DB not initialized."}`
			}
			req := decodeHomepageRegistryArgs(tc)
			op := req.Operation
			if op == "" {
				op = "list"
			}
			logger.Info("LLM requested homepage_registry", "operation", op, "name", req.Name)
			return "Tool Output: " + tools.DispatchHomepageRegistry(homepageRegistryDB, op, req.Query, req.Name, req.Description, req.Framework, req.ProjectDir, req.URL, req.Status, req.Reason, req.Problem, req.Notes, req.Tags, req.ID, "", req.Limit, req.Offset)

		case "sql_query":
			return handleSQLQueryTool(ctx, tc, dc)

		case "manage_sql_connections":
			return handleManageSQLConnectionsTool(ctx, tc, dc)

		case "ldap":
			if !cfg.LDAP.Enabled {
				return `Tool Output: {"status": "error", "message": "LDAP integration is not enabled in config.yaml."}`
			}
			req := decodeLDAPArgs(tc)
			logger.Info("LLM requested LDAP operation", "op", req.Operation)
			if cfg.LDAP.ReadOnly {
				switch req.Operation {
				case "add_user", "update_user", "delete_user", "add_group", "update_group", "delete_group":
					return `Tool Output: {"status":"error","message":"LDAP is in read-only mode."}`
				}
			}
			args := make(map[string]interface{})
			if req.BaseDN != "" {
				args["base_dn"] = req.BaseDN
			}
			if req.Filter != "" {
				args["filter"] = req.Filter
			}
			if req.Username != "" {
				args["username"] = req.Username
			}
			if req.GroupName != "" {
				args["group_name"] = req.GroupName
			}
			if req.UserDN != "" {
				args["user_dn"] = req.UserDN
			}
			if req.DN != "" {
				args["dn"] = req.DN
			}
			if req.Password != "" {
				args["password"] = req.Password
			}
			if len(req.Attributes) > 0 {
				attrs := make([]interface{}, len(req.Attributes))
				for i, a := range req.Attributes {
					attrs[i] = a
				}
				args["attributes"] = attrs
			}
			if len(req.EntryAttributes) > 0 {
				args["entry_attributes"] = req.EntryAttributes
			}
			if len(req.Changes) > 0 {
				args["changes"] = req.Changes
			}
			return "Tool Output: " + tools.LDAP(cfg, vault, req.Operation, args, logger)

		default:
			handled = false
			return ""
		}
	}()
	return result, handled
}
