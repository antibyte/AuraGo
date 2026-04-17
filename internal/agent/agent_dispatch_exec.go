package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func stringValueFromMap(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// resolveFilePath returns tc.FilePath if non-empty, falling back to tc.Path.
func resolveFilePath(tc ToolCall) string {
	if tc.FilePath != "" {
		return tc.FilePath
	}
	return tc.Path
}

func buildMemoryReflectionOutput(result interface{}) (string, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal reflection result: %w", err)
	}
	return fmt.Sprintf(`Tool Output: {"status":"success","reflection":%s}`, string(resultJSON)), nil
}

// resolveVaultKeys resolves vault secret keys for Python secret injection.
// Returns the resolved secrets map (may be empty) and an info message for rejected keys.
// If the feature is disabled or no keys requested, returns nil/empty.
func resolveVaultKeys(cfg *config.Config, vault *security.Vault, keys []string, logger *slog.Logger) (map[string]string, string) {
	if !cfg.Tools.PythonSecretInjection.Enabled || len(keys) == 0 || vault == nil {
		return nil, ""
	}
	resolved, rejected, err := tools.ResolveVaultSecrets(vault, keys)
	if err != nil {
		logger.Error("Failed to resolve vault secrets for Python", "error", err)
		return nil, ""
	}
	var info string
	if len(rejected) > 0 {
		info = fmt.Sprintf("[NOTE] The following vault keys were rejected (system/integration secrets cannot be accessed by Python tools): %s", strings.Join(rejected, ", "))
		logger.Warn("Vault keys rejected for Python injection", "rejected", rejected)
	}
	if len(resolved) > 0 {
		keyNames := make([]string, 0, len(resolved))
		for k := range resolved {
			keyNames = append(keyNames, k)
		}
		logger.Info("Vault secrets injected for Python execution", "keys", keyNames)
	}
	return resolved, info
}

// resolveCredentials resolves credential IDs for Python injection.
// Returns the resolved credential fields (may be empty) and an info message for rejected IDs.
// If python secret injection is disabled or no IDs requested, returns nil/empty.
func resolveCredentials(cfg *config.Config, vault *security.Vault, inventoryDB *sql.DB, ids []string, logger *slog.Logger) ([]tools.CredentialFields, string) {
	if !cfg.Tools.PythonSecretInjection.Enabled || len(ids) == 0 || vault == nil || inventoryDB == nil {
		return nil, ""
	}
	resolved, rejected, err := tools.ResolveCredentialSecrets(inventoryDB, vault, ids)
	if err != nil {
		logger.Error("Failed to resolve credentials for Python", "error", err)
		return nil, ""
	}
	var info string
	if len(rejected) > 0 {
		info = fmt.Sprintf("[NOTE] The following credential IDs were rejected (not found or python access not allowed): %s", strings.Join(rejected, ", "))
		logger.Warn("Credentials rejected for Python injection", "rejected", rejected)
	}
	if len(resolved) > 0 {
		names := make([]string, 0, len(resolved))
		for _, cf := range resolved {
			names = append(names, cf.Name)
		}
		logger.Info("Credentials injected for Python execution", "names", names)
	}
	return resolved, info
}

// dispatchExec handles execution, memory, security, filesystem, API, remote, and scheduling tool calls.
func dispatchExec(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	llmClient := dc.LLMClient
	vault := dc.Vault
	registry := dc.Registry
	cronManager := dc.CronManager
	longTermMem := dc.LongTermMem
	shortTermMem := dc.ShortTermMem
	kg := dc.KG
	plannerDB := dc.PlannerDB
	inventoryDB := dc.InventoryDB
	cheatsheetDB := dc.CheatsheetDB
	imageGalleryDB := dc.ImageGalleryDB
	mediaRegistryDB := dc.MediaRegistryDB
	budgetTracker := dc.BudgetTracker
	handled := true

	result := func() string {
		switch tc.Action {
		case "code_analysis":
			analyzer := tools.NewCodeAnalyzer()
			op := stringValueFromMap(tc.Params, "operation")
			target := stringValueFromMap(tc.Params, "target")
			symbol := stringValueFromMap(tc.Params, "symbol")

			if op == "" || target == "" {
				return "Tool Output: [ERROR] 'operation' and 'target' are required."
			}

			if op == "structure" {
				items, err := analyzer.ExtractStructure(target)
				if err != nil {
					return "Tool Output: [ERROR] ExtractStructure failed: " + err.Error()
				}
				if len(items) == 0 {
					return "Tool Output: No structural items identified in file."
				}
				var rs []string
				for _, it := range items {
					rs = append(rs, fmt.Sprintf("Line %d - %s: %s", it.Line, it.Type, it.Name))
				}
				return "Tool Output:\n" + strings.Join(rs, "\n")
			} else if op == "symbol_search" {
				if symbol == "" {
					return "Tool Output: [ERROR] 'symbol' is required for symbol_search operation."
				}
				locations, err := analyzer.SymbolSearch(target, symbol)
				if err != nil {
					return "Tool Output: [ERROR] SymbolSearch failed: " + err.Error()
				}
				if len(locations) == 0 {
					return "Tool Output: Symbol '" + symbol + "' not found."
				}
				return "Tool Output:\n" + strings.Join(locations, "\n")
			}
			return "Tool Output: [ERROR] Unknown operation: " + op

		case "execute_sandbox":
			return dispatchShell(tc, dc)

		case "execute_python":
			return dispatchPython(tc, dc)

		case "execute_shell":
			return dispatchShell(tc, dc)

		case "service_manager":
			return dispatchShell(tc, dc)

		case "execute_sudo":
			return dispatchShell(tc, dc)

		case "install_package":
			return dispatchShell(tc, dc)

		case "save_tool":
			return dispatchPython(tc, dc)

		case "list_tools":
			return dispatchPython(tc, dc)

		case "discover_tools":
			return dispatchPython(tc, dc)

		case "run_tool":
			return dispatchPython(tc, dc)

		case "list_processes":
			logger.Info("LLM requested process list")
			list := registry.List()
			if len(list) == 0 {
				return "Tool Output: No active background processes."
			}
			var sb strings.Builder
			sb.WriteString("Tool Output: Active processes:\n")
			for _, p := range list {
				pid, _ := p["pid"].(int)
				started, _ := p["started"].(string)
				sb.WriteString(fmt.Sprintf("- PID: %d, Started: %s\n", pid, started))
			}
			return sb.String()

		case "stop_process":
			if !cfg.Tools.StopProcess.Enabled {
				return `Tool Output: {"status":"error","message":"stop_process is disabled. Set tools.stop_process.enabled=true in config.yaml."}`
			}
			req := decodeProcessControlArgs(tc)
			logger.Info("LLM requested process stop", "pid", req.PID)
			if err := registry.Terminate(req.PID); err != nil {
				return fmt.Sprintf("Tool Output: ERROR stopping process %d: %v", req.PID, err)
			}
			return fmt.Sprintf("Tool Output: Process %d stopped.", req.PID)

		case "read_process_logs":
			req := decodeProcessControlArgs(tc)
			logger.Info("LLM requested process logs", "pid", req.PID)
			proc, ok := registry.Get(req.PID)
			if !ok {
				return fmt.Sprintf("Tool Output: ERROR process %d not found", req.PID)
			}
			return fmt.Sprintf("Tool Output: [LOGS for PID %d]\n%s", req.PID, security.Scrub(proc.ReadOutput()))

		case "query_memory":
			if !cfg.Tools.Memory.Enabled {
				return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
			}
			result, err := executeQueryMemory(tc, shortTermMem, longTermMem, kg, plannerDB)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"query_memory failed: %v"}`, err)
			}
			return result

		case "context_memory":
			if !cfg.Tools.Memory.Enabled {
				return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
			}
			result, err := executeContextMemoryQuery(tc, shortTermMem, longTermMem, kg, plannerDB)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"context_memory failed: %v"}`, err)
			}
			return result

		case "memory_reflect":
			if !resolveMemoryAnalysisSettings(cfg, shortTermMem).Enabled {
				return `Tool Output: {"status":"error","message":"Memory analysis is disabled. Enable memory_analysis.enabled in config."}`
			}
			req := decodeMemoryReflectArgs(tc)
			scope := req.Scope
			if scope == "" {
				scope = "recent"
			}
			logger.Info("LLM requested memory reflection", "scope", scope)
			result, err := generateMemoryReflection(ctx, cfg, logger, shortTermMem, kg, longTermMem, llmClient, scope)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Reflection failed: %v"}`, err)
			}
			resultOutput, err := buildMemoryReflectionOutput(result)
			if err != nil {
				logger.Warn("Failed to serialize memory reflection result", "error", err)
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Reflection serialization failed: %v"}`, err)
			}
			return resultOutput

		case "manage_updates":
			if !cfg.Agent.AllowSelfUpdate {
				return "Tool Output: [PERMISSION DENIED] manage_updates is disabled in Danger Zone settings (agent.allow_self_update: false)."
			}
			req := decodeUpdateManagementArgs(tc)
			logger.Info("LLM requested update management", "operation", req.Operation)
			switch req.Operation {
			case "check":
				installDir := filepath.Dir(cfg.ConfigPath)

				// Binary-only install: no .git directory → use GitHub Releases API
				if _, gitErr := os.Stat(filepath.Join(installDir, ".git")); os.IsNotExist(gitErr) {
					// Read installed version from .version file
					currentVer := "unknown"
					if vb, err := os.ReadFile(filepath.Join(installDir, ".version")); err == nil {
						currentVer = strings.TrimSpace(string(vb))
					}
					// Fetch latest release from GitHub
					type ghRelease struct {
						TagName string `json:"tag_name"`
					}
					httpClient := &http.Client{Timeout: 10 * time.Second}
					req, reqErr := http.NewRequest("GET", "https://api.github.com/repos/antibyte/AuraGo/releases/latest", nil)
					if reqErr != nil {
						return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to build request: %v"}`, reqErr)
					}
					req.Header.Set("User-Agent", "AuraGo-Agent/1.0")
					resp, fetchErr := httpClient.Do(req)
					if fetchErr != nil {
						return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to reach GitHub: %v"}`, fetchErr)
					}
					defer resp.Body.Close()
					var rel ghRelease
					if decErr := json.NewDecoder(resp.Body).Decode(&rel); decErr != nil {
						return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to parse GitHub response: %v"}`, decErr)
					}
					if currentVer != "unknown" && currentVer == rel.TagName {
						return fmt.Sprintf(`Tool Output: {"status":"success","update_available":false,"current_version":%q,"latest_version":%q,"message":"AuraGo is up to date."}`, currentVer, rel.TagName)
					}
					return fmt.Sprintf(`Tool Output: {"status":"success","update_available":true,"current_version":%q,"latest_version":%q,"message":"Update available."}`, currentVer, rel.TagName)
				}

				// Git-based install
				_, err := runGitCommand(filepath.Dir(cfg.ConfigPath), "fetch", "origin", "main", "--quiet")
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch updates: %v"}`, err)
				}

				countOut, err := runGitCommand(filepath.Dir(cfg.ConfigPath), "rev-list", "HEAD..origin/main", "--count")
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to check update count: %v"}`, err)
				}
				countStr := strings.TrimSpace(string(countOut))
				count, _ := strconv.Atoi(countStr)

				if count == 0 {
					return `Tool Output: {"status": "success", "update_available": false, "message": "AuraGo is up to date."}`
				}

				logOut, _ := runGitCommand(filepath.Dir(cfg.ConfigPath), "log", "HEAD..origin/main", "--oneline", "-n", "10")

				return fmt.Sprintf(`Tool Output: {"status": "success", "update_available": true, "count": %d, "changelog": %q}`, count, string(logOut))

			case "install":
				logger.Warn("LLM requested update installation")
				updateScript := filepath.Join(filepath.Dir(cfg.ConfigPath), "update.sh")
				if _, err := os.Stat(updateScript); err != nil {
					return `Tool Output: {"status": "error", "message": "update.sh not found in application directory"}`
				}

				// Run ./update.sh --yes
				updateCmd := exec.Command("/bin/bash", "./update.sh", "--yes")
				updateCmd.Dir = filepath.Dir(cfg.ConfigPath)
				// Ensure environment is passed for update script too
				home, _ := os.UserHomeDir()
				if home != "" {
					updateCmd.Env = append(os.Environ(), "HOME="+home)
				}
				// Start update script. It will handle the rest, potentially killing this process.
				if err := updateCmd.Start(); err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to start update script: %v"}`, err)
				}
				// Reap the child to avoid a zombie process. The update script may
				// kill this process, so we do not block — Wait runs in a goroutine.
				go func() { _ = updateCmd.Wait() }()
				return `Tool Output: {"status": "success", "message": "Update initiated. The system will restart and apply changes shortly."}`

			default:
				return `Tool Output: {"status": "error", "message": "Invalid operation. Use 'check' or 'install'."}`
			}

		case "archive_memory":
			if !cfg.Tools.MemoryMaintenance.Enabled {
				return `Tool Output: {"status":"error","message":"Memory maintenance is disabled. Set tools.memory_maintenance.enabled=true in config.yaml."}`
			}
			logger.Info("LLM requested memory archival", "id", tc.ID)
			return "Tool Output: " + runMemoryOrchestrator(decodeMemoryOrchestratorArgs(tc), cfg, logger, llmClient, longTermMem, shortTermMem, kg)

		case "optimize_memory":
			if !cfg.Tools.MemoryMaintenance.Enabled {
				return `Tool Output: {"status":"error","message":"Memory maintenance is disabled. Set tools.memory_maintenance.enabled=true in config.yaml."}`
			}
			logger.Info("LLM requested memory optimization")
			return "Tool Output: " + runMemoryOrchestrator(decodeMemoryOrchestratorArgs(tc), cfg, logger, llmClient, longTermMem, shortTermMem, kg)

		case "manage_knowledge", "knowledge_graph":
			req := decodeKnowledgeGraphArgs(tc)
			if !cfg.Tools.KnowledgeGraph.Enabled {
				return `Tool Output: {"status":"error","message":"Knowledge graph is disabled. Set tools.knowledge_graph.enabled=true in config.yaml."}`
			}
			if cfg.Tools.KnowledgeGraph.ReadOnly {
				switch req.Operation {
				case "add_node", "add_edge", "delete_node", "delete_edge", "update_node", "update_edge", "optimize":
					return `Tool Output: {"status":"error","message":"Knowledge graph is in read-only mode. Disable tools.knowledge_graph.read_only to allow changes."}`
				}
			}
			logger.Info("LLM requested knowledge graph operation", "op", req.Operation)
			switch req.Operation {
			case "add_node":
				err := kg.AddNode(req.ID, req.Label, req.Properties)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Node added to graph"}`

			case "add_edge":
				err := kg.AddEdge(req.Source, req.Target, req.Relation, req.Properties)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Edge added to graph"}`

			case "delete_node":
				err := kg.DeleteNode(req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Node deleted"}`

			case "delete_edge":
				err := kg.DeleteEdge(req.Source, req.Target, req.Relation)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "Edge deleted"}`

			case "update_node":
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "Node 'id' is required for update_node"}`
				}
				node, err := kg.UpdateNode(req.ID, req.Label, req.Properties)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				if node == nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Node not found: %s"}`, req.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{
					"status":     "success",
					"message":    "Node updated",
					"id":         node.ID,
					"label":      node.Label,
					"properties": node.Properties,
					"protected":  node.Protected,
				})
				return "Tool Output: " + string(data)

			case "update_edge":
				if req.Source == "" || req.Target == "" || req.Relation == "" {
					return `Tool Output: {"status": "error", "message": "source, target, and relation are required for update_edge"}`
				}
				newRel := req.NewRelation
				if newRel == "" {
					newRel = req.Relation
				}
				edge, err := kg.UpdateEdge(req.Source, req.Target, req.Relation, newRel, req.Properties)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				if edge == nil {
					return `Tool Output: {"status": "error", "message": "Edge not found"}`
				}
				data, _ := json.Marshal(map[string]interface{}{
					"status":     "success",
					"message":    "Edge updated",
					"source":     edge.Source,
					"target":     edge.Target,
					"relation":   edge.Relation,
					"properties": edge.Properties,
				})
				return "Tool Output: " + string(data)

			case "get_node":
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "Node 'id' is required for get_node"}`
				}
				node, err := kg.GetNode(req.ID)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				if node == nil {
					return fmt.Sprintf(`Tool Output: {"status": "not_found", "message": "Node not found: %s"}`, req.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{
					"id":         node.ID,
					"label":      node.Label,
					"properties": node.Properties,
					"protected":  node.Protected,
				})
				return "Tool Output: " + string(data)

			case "get_neighbors":
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "Node 'id' is required for get_neighbors"}`
				}
				limit := req.Limit
				if limit <= 0 {
					limit = 20
				}
				nodes, edges := kg.GetNeighbors(req.ID, limit)
				if len(nodes) == 0 && len(edges) == 0 {
					return fmt.Sprintf(`Tool Output: {"status": "not_found", "message": "No neighbors found for node: %s"}`, req.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{
					"center_id": req.ID,
					"nodes":     nodes,
					"edges":     edges,
				})
				return "Tool Output: " + string(data)

			case "subgraph":
				if req.ID == "" {
					return `Tool Output: {"status": "error", "message": "Node 'id' is required for subgraph"}`
				}
				depth := req.Depth
				if depth <= 0 {
					depth = 2
				}
				nodes, edges := kg.GetSubgraph(req.ID, depth)
				if len(nodes) == 0 && len(edges) == 0 {
					return fmt.Sprintf(`Tool Output: {"status": "not_found", "message": "No subgraph found around node: %s"}`, req.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{
					"center_id": req.ID,
					"depth":     depth,
					"nodes":     nodes,
					"edges":     edges,
				})
				return "Tool Output: " + string(data)

			case "search":
				res := kg.Search(req.Content)
				return fmt.Sprintf("Tool Output: %s", res)

			case "explore":
				if req.Content == "" {
					return `Tool Output: {"status": "error", "message": "Search 'content' is required for explore"}`
				}
				return fmt.Sprintf("Tool Output: %s", kg.Explore(req.Content))

			case "suggest_relations":
				return fmt.Sprintf("Tool Output: %s", kg.SuggestRelations(req.Limit))

			case "optimize":
				res := runMemoryOrchestrator(decodeMemoryOrchestratorArgs(tc), cfg, logger, llmClient, longTermMem, shortTermMem, kg)
				return fmt.Sprintf("Tool Output: %s", res)

			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown graph operation: %s"}`, req.Operation)
			}

		case "context_manager":
			if dc.HistoryMgr == nil || dc.BudgetTracker == nil {
				return `Tool Output: {"status": "error", "message": "Context manager dependencies unavailable."}`
			}
			var req struct {
				Operation string `json:"operation"`
				Index     int    `json:"index"`
			}
			if err := json.Unmarshal([]byte(tc.NativeArgsRaw), &req); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to parse arguments: %v"}`, err)
			}

			switch req.Operation {
			case "status":
				msgs := dc.HistoryMgr.GetAll()
				bStat := dc.BudgetTracker.GetStatusJSON()
				return fmt.Sprintf(`Tool Output: {"status": "success", "messages_count": %d, "total_chars": %d, "budget": %s}`, len(msgs), dc.HistoryMgr.TotalChars(), bStat)
			case "compact":
				msgs := dc.HistoryMgr.GetAll()
				if len(msgs) < 4 {
					return `Tool Output: {"status": "error", "message": "Not enough messages to compact."}`
				}

				toCompact := len(msgs) / 2
				var transcript strings.Builder
				var idsToDrop []int64
				for i := 0; i < toCompact; i++ {
					m := msgs[i]
					if m.Pinned {
						continue
					}
					transcript.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
					idsToDrop = append(idsToDrop, m.ID)
				}

				if len(idsToDrop) == 0 {
					return `Tool Output: {"status": "error", "message": "No unpinned messages to compact."}`
				}

				prompt := "Compress this conversation into a concise factual summary. Preserve key facts, tool results, decisions. Output ONLY the summary.\n\n" + transcript.String()
				llmReq := openai.ChatCompletionRequest{
					Model: dc.Cfg.LLM.Model,
					Messages: []openai.ChatCompletionMessage{
						{Role: openai.ChatMessageRoleUser, Content: prompt},
					},
					MaxTokens:   500,
					Temperature: 0.2,
				}

				resp, err := dc.LLMClient.CreateChatCompletion(ctx, llmReq)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Summarization failed: %v"}`, err)
				}

				summary := resp.Choices[0].Message.Content
				oldSummary := dc.HistoryMgr.GetSummary()
				if oldSummary != "" {
					summary = oldSummary + "\n" + summary
				}
				dc.HistoryMgr.SetSummary(summary)
				dc.HistoryMgr.DropMessages(idsToDrop)

				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Compacted %d messages into summary."}`, len(idsToDrop))
			case "drop":
				msgs := dc.HistoryMgr.GetAll()
				if req.Index < 0 || req.Index >= len(msgs) {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Index %d out of bounds (0-%d)."}`, req.Index, len(msgs)-1)
				}

				idToDrop := msgs[req.Index].ID
				dc.HistoryMgr.DropMessages([]int64{idToDrop})
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Dropped message at index %d (ID %d)."}`, req.Index, idToDrop)

			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown context operation: %s"}`, req.Operation)
			}

		case "manage_memory", "core_memory":
			req := decodeCoreMemoryArgs(tc)
			if !cfg.Tools.Memory.Enabled {
				return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
			}
			if cfg.Tools.Memory.ReadOnly {
				switch req.Operation {
				case "add", "store", "save", "set", "update", "delete", "remove", "reset_profile", "delete_profile_entry":
					return `Tool Output: {"status":"error","message":"Memory is in read-only mode. Disable tools.memory.read_only to allow changes."}`
				}
			}
			// Handle synonyms for 'fact'
			fact := req.Fact
			if fact == "" {
				if req.MemoryValue != "" {
					fact = req.MemoryValue
				} else if req.MemoryKey != "" {
					fact = req.MemoryKey
				} else if req.Value != "" {
					fact = req.Value
				} else if req.Content != "" {
					fact = req.Content
				}
			}
			// When LLM uses separate key+value fields, combine into a meaningful fact (e.g. "agent_name: Nova")
			// Only for add/update, and only when key is a descriptive word (not a numeric ID)
			{
				op := strings.ToLower(req.Operation)
				keyField := req.Key
				if keyField == "" {
					keyField = req.MemoryKey
				}
				if (op == "add" || op == "update") && keyField != "" && fact != "" && fact != keyField {
					if _, parseErr := strconv.ParseInt(keyField, 10, 64); parseErr != nil {
						// Key is not a numeric ID — prefix fact with key for context
						if !strings.HasPrefix(strings.ToLower(fact), strings.ToLower(keyField)+":") &&
							!strings.HasPrefix(strings.ToLower(fact), strings.ToLower(keyField)+" ") {
							fact = keyField + ": " + fact
						}
					}
				}
			}

			logger.Info("LLM requested core memory management", "op", req.Operation, "fact", fact)
			if req.Operation == "" {
				return `Tool Output: {"status": "error", "message": "'operation' is required for manage_memory"}`
			}

			// User Profile operations (sub-ops of manage_memory)
			switch req.Operation {
			case "view_profile":
				entries, err := shortTermMem.GetProfileEntries("")
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				if len(entries) == 0 {
					return `Tool Output: {"status": "success", "message": "No user profile data collected yet.", "entries": []}`
				}
				var sb strings.Builder
				sb.WriteString(`{"status":"success","entries":[`)
				for i, e := range entries {
					if i > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf(`{"category":%q,"key":%q,"value":%q,"confidence":%d}`, e.Category, e.Key, e.Value, e.Confidence))
				}
				sb.WriteString(`]}`)
				return fmt.Sprintf("Tool Output: %s", sb.String())
			case "reset_profile":
				if err := shortTermMem.ResetUserProfile(); err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return `Tool Output: {"status": "success", "message": "User profile has been completely reset."}`
			case "delete_profile_entry":
				cat := req.Key
				key := req.Value
				if cat == "" || key == "" {
					return `Tool Output: {"status": "error", "message": "'key' (category) and 'value' (key name) are required for delete_profile_entry"}`
				}
				if err := shortTermMem.DeleteProfileEntry(cat, key); err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Profile entry %s/%s deleted."}`, cat, key)
			}

			var memID int64
			fmt.Sscanf(req.ID, "%d", &memID)
			result, err := tools.ManageCoreMemory(req.Operation, fact, memID, shortTermMem, cfg.Agent.CoreMemoryMaxEntries, cfg.Agent.CoreMemoryCapMode, cfg.Server.UILanguage)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf("Tool Output: %s", result)

		case "cheatsheet":
			req := decodeCheatsheetArgs(tc)
			if cheatsheetDB == nil {
				return `Tool Output: {"status":"error","message":"Cheat sheet database is not available."}`
			}
			op := strings.ToLower(strings.TrimSpace(req.Operation))
			if op == "" {
				return `Tool Output: {"status":"error","message":"'operation' is required (list, get, create, update, delete)."}`
			}
			switch op {
			case "list":
				sheets, err := tools.CheatsheetList(cheatsheetDB, true)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				type entry struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				list := make([]entry, len(sheets))
				for i, s := range sheets {
					list[i] = entry{ID: s.ID, Name: s.Name}
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(list), "cheatsheets": list})
				return fmt.Sprintf("Tool Output: %s", string(data))
			case "get":
				var sheet *tools.CheatSheet
				var err error
				if req.ID != "" {
					sheet, err = tools.CheatsheetGet(cheatsheetDB, req.ID)
				} else if req.Name != "" {
					sheet, err = tools.CheatsheetGetByName(cheatsheetDB, req.Name)
				} else {
					return `Tool Output: {"status":"error","message":"'id' or 'name' is required for get."}`
				}
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"cheat sheet not found: %v"}`, err)
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "ok", "cheatsheet": sheet})
				return fmt.Sprintf("Tool Output: %s", string(data))
			case "create":
				if req.Name == "" {
					return `Tool Output: {"status":"error","message":"'name' is required for create."}`
				}
				sheet, err := tools.CheatsheetCreate(cheatsheetDB, req.Name, req.Content, "agent")
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				// Index cheatsheet in vector DB for semantic search (best-effort)
				if storeErr := tools.ReindexCheatsheetInVectorDB(cheatsheetDB, dc.LongTermMem, sheet.ID); storeErr != nil {
					dc.Logger.Warn("Failed to index cheatsheet in vector DB", "cs_id", sheet.ID, "error", storeErr)
				}
				if dc.PreparationService != nil {
					dc.PreparationService.InvalidateByCheatsheet(sheet.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Cheat sheet created.", "cheatsheet": sheet})
				return fmt.Sprintf("Tool Output: %s", string(data))
			case "update":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for update."}`
				}
				var namePtr, contentPtr *string
				var activePtr *bool
				if req.Name != "" {
					namePtr = &req.Name
				}
				if req.Content != "" {
					contentPtr = &req.Content
				}
				if req.Active != nil {
					activePtr = req.Active
				}
				sheet, err := tools.CheatsheetUpdate(cheatsheetDB, req.ID, namePtr, contentPtr, activePtr)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				// Update cheatsheet in vector DB (best-effort)
				if storeErr := tools.ReindexCheatsheetInVectorDB(cheatsheetDB, dc.LongTermMem, sheet.ID); storeErr != nil {
					dc.Logger.Warn("Failed to update cheatsheet in vector DB", "cs_id", sheet.ID, "error", storeErr)
				}
				if dc.PreparationService != nil {
					dc.PreparationService.InvalidateByCheatsheet(sheet.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Cheat sheet updated.", "cheatsheet": sheet})
				return fmt.Sprintf("Tool Output: %s", string(data))
			case "delete":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' is required for delete."}`
				}
				if err := tools.CheatsheetDelete(cheatsheetDB, req.ID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				// Remove cheatsheet from vector DB (best-effort)
				if dc.LongTermMem != nil {
					if delErr := dc.LongTermMem.DeleteCheatsheet(req.ID); delErr != nil {
						dc.Logger.Warn("Failed to delete cheatsheet from vector DB", "cs_id", req.ID, "error", delErr)
					}
				}
				if dc.PreparationService != nil {
					dc.PreparationService.InvalidateByCheatsheet(req.ID)
				}
				return `Tool Output: {"status":"ok","message":"Cheat sheet deleted."}`
			case "attach":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' (cheat sheet ID) is required for attach."}`
				}
				if req.Filename == "" {
					return `Tool Output: {"status":"error","message":"'filename' is required for attach."}`
				}
				source := "upload"
				if req.Source != "" {
					source = req.Source
				}
				attachment, err := tools.CheatsheetAttachmentAdd(cheatsheetDB, req.ID, req.Filename, source, req.Content)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				if storeErr := tools.ReindexCheatsheetInVectorDB(cheatsheetDB, dc.LongTermMem, req.ID); storeErr != nil {
					dc.Logger.Warn("Failed to re-index cheatsheet after attach", "cs_id", req.ID, "error", storeErr)
				}
				if dc.PreparationService != nil {
					dc.PreparationService.InvalidateByCheatsheet(req.ID)
				}
				data, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Attachment added.", "attachment": attachment})
				return fmt.Sprintf("Tool Output: %s", string(data))
			case "detach":
				if req.ID == "" {
					return `Tool Output: {"status":"error","message":"'id' (cheat sheet ID) is required for detach."}`
				}
				if req.AttachmentID == "" {
					return `Tool Output: {"status":"error","message":"'attachment_id' is required for detach."}`
				}
				if err := tools.CheatsheetAttachmentRemove(cheatsheetDB, req.ID, req.AttachmentID); err != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"%v"}`, err)
				}
				if storeErr := tools.ReindexCheatsheetInVectorDB(cheatsheetDB, dc.LongTermMem, req.ID); storeErr != nil {
					dc.Logger.Warn("Failed to re-index cheatsheet after detach", "cs_id", req.ID, "error", storeErr)
				}
				if dc.PreparationService != nil {
					dc.PreparationService.InvalidateByCheatsheet(req.ID)
				}
				return `Tool Output: {"status":"ok","message":"Attachment removed."}`
			default:
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown cheatsheet operation: %s. Use list, get, create, update, delete, attach, or detach."}`, op)
			}

		case "get_secret", "secrets_vault":
			req := decodeSecretVaultArgs(tc)
			if !cfg.Tools.SecretsVault.Enabled {
				return `Tool Output: {"status":"error","message":"Secrets vault is disabled. Set tools.secrets_vault.enabled=true in config.yaml."}`
			}
			op := strings.TrimSpace(strings.ToLower(req.Operation))
			if cfg.Tools.SecretsVault.ReadOnly && (op == "store" || op == "set" || req.Action == "set_secret") {
				return `Tool Output: {"status":"error","message":"Secrets vault is in read-only mode. Disable tools.secrets_vault.read_only to allow changes."}`
			}
			if op == "store" || op == "set" || (req.Action == "set_secret") {
				logger.Info("LLM requested secret storage", "key", req.Key)
				if req.Key == "" || req.Value == "" {
					return `Tool Output: {"status": "error", "message": "'key' and 'value' are required for set_secret/store"}`
				}
				if isSystemSecret(req.Key) {
					logger.Warn("LLM attempted to overwrite system-managed secret — access denied", "key", tc.Key)
					return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be overwritten via secrets_vault."}`
				}
				err := vault.WriteSecret(req.Key, req.Value)
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Secret '%s' stored safely."}`, req.Key)
			}

			// Default: read/list
			logger.Info("LLM requested secret retrieval", "key", req.Key)
			if req.Key == "" {
				// List available secret keys when no key is specified
				// Filter out system-managed keys — the agent must not know they exist
				keys, err := vault.ListKeys()
				if err != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
				}
				visibleKeys := make([]string, 0, len(keys))
				for _, k := range keys {
					if !isSystemSecret(k) {
						visibleKeys = append(visibleKeys, k)
					}
				}
				b, mErr := json.Marshal(visibleKeys)
				if mErr != nil {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize keys: %v"}`, mErr)
				}
				return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Stored secret keys (use get_secret with 'key' to retrieve a value)", "keys": %s}`, string(b))
			}
			// Block access to system-managed secrets
			if isSystemSecret(req.Key) {
				logger.Warn("LLM attempted to read system-managed secret — access denied", "key", tc.Key)
				return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be retrieved via secrets_vault."}`
			}
			secret, err := vault.ReadSecret(req.Key)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			// JSON-encode the secret value to prevent injection from special characters
			safeVal, _ := json.Marshal(secret)
			return fmt.Sprintf(`Tool Output: {"status": "success", "key": "%s", "value": %s}`, req.Key, string(safeVal))

		case "set_secret":
			req := decodeSecretVaultArgs(tc)
			if !cfg.Tools.SecretsVault.Enabled {
				return `Tool Output: {"status":"error","message":"Secrets vault is disabled. Set tools.secrets_vault.enabled=true in config.yaml."}`
			}
			if cfg.Tools.SecretsVault.ReadOnly {
				return `Tool Output: {"status":"error","message":"Secrets vault is in read-only mode. Disable tools.secrets_vault.read_only to allow changes."}`
			}
			logger.Info("LLM requested secret storage", "key", tc.Key)
			if req.Key == "" || req.Value == "" {
				return `Tool Output: {"status": "error", "message": "'key' and 'value' are required for set_secret"}`
			}
			if isSystemSecret(req.Key) {
				logger.Warn("LLM attempted to overwrite system-managed secret — access denied", "key", tc.Key)
				return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be overwritten via secrets_vault."}`
			}
			err := vault.WriteSecret(req.Key, req.Value)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Secret '%s' stored safely."}`, req.Key)

		case "archive":
			return dispatchFilesystem(ctx, tc, dc)

		case "pdf_operations":
			return dispatchFilesystem(ctx, tc, dc)

		case "image_processing":
			return dispatchFilesystem(ctx, tc, dc)

		case "filesystem", "filesystem_op":
			return dispatchFilesystem(ctx, tc, dc)

		case "file_editor":
			return dispatchFilesystem(ctx, tc, dc)

		case "json_editor":
			return dispatchFilesystem(ctx, tc, dc)

		case "yaml_editor":
			return dispatchFilesystem(ctx, tc, dc)

		case "xml_editor":
			return dispatchFilesystem(ctx, tc, dc)

		case "text_diff":
			return dispatchFilesystem(ctx, tc, dc)

		case "file_search":
			return dispatchFilesystem(ctx, tc, dc)

		case "file_reader_advanced":
			return dispatchFilesystem(ctx, tc, dc)

		case "smart_file_read":
			return dispatchFilesystem(ctx, tc, dc)

		case "api_request":
			if !cfg.Agent.AllowNetworkRequests {
				return "Tool Output: [PERMISSION DENIED] api_request is disabled in Danger Zone settings (agent.allow_network_requests: false)."
			}
			req := decodeAPIRequestArgs(tc)
			logger.Info("LLM requested generic API request", "url", req.URL)
			return tools.ExecuteAPIRequest(req.Method, req.URL, req.Body, req.Headers)

		case "koofr", "koofr_api", "koofr_op":
			if !cfg.Koofr.Enabled {
				return `Tool Output: {"status": "error", "message": "Koofr integration is not enabled. Set koofr.enabled=true in config.yaml."}`
			}
			req := decodeCloudStorageArgs(tc)
			if cfg.Koofr.ReadOnly {
				switch req.Operation {
				case "write", "put", "upload", "mkdir", "delete", "rm", "move", "rename", "mv":
					return `Tool Output: {"status":"error","message":"Koofr is in read-only mode. Disable koofr.read_only to allow changes."}`
				}
			}
			logger.Info("LLM requested koofr operation", "op", req.Operation, "path", req.FilePath, "dest", req.Destination)
			koofrCfg := tools.KoofrConfig{
				BaseURL:     cfg.Koofr.BaseURL,
				Username:    cfg.Koofr.Username,
				AppPassword: cfg.Koofr.AppPassword,
			}
			return tools.ExecuteKoofr(koofrCfg, req.Operation, req.FilePath, req.Destination, req.Content)

		case "google_workspace", "gworkspace":
			if !cfg.GoogleWorkspace.Enabled {
				return `Tool Output: {"status": "error", "message": "Google Workspace is not enabled. Enable it in Settings > Google Workspace."}`
			}
			req := decodeGoogleWorkspaceArgs(tc)
			op := req.Operation
			if op == "" {
				op = tc.Action
			}
			logger.Info("LLM requested google_workspace operation", "op", op)
			return "Tool Output: " + tools.ExecuteGoogleWorkspace(*cfg, vault, op, req.params())

		case "onedrive", "onedrive_op":
			if !cfg.OneDrive.Enabled {
				return `Tool Output: {"status": "error", "message": "OneDrive integration is not enabled. Set onedrive.enabled=true in config.yaml."}`
			}
			req := decodeCloudStorageArgs(tc)
			op := req.Operation
			if cfg.OneDrive.ReadOnly {
				switch op {
				case "upload", "write", "mkdir", "delete", "move", "copy", "share":
					return `Tool Output: {"status":"error","message":"OneDrive is in read-only mode. Disable onedrive.readonly to allow changes."}`
				}
			}
			logger.Info("LLM requested onedrive operation", "op", op, "path", req.FilePath, "dest", req.Destination)
			client, err := tools.NewOneDriveClient(*cfg, vault)
			if err != nil {
				return "Tool Output: " + tools.ODErrJSON("OneDrive client error: %v", err)
			}
			return "Tool Output: " + client.ExecuteOneDrive(op, req.FilePath, req.Destination, req.Content, req.MaxResults)

		case "generate_music":
			if !cfg.MusicGeneration.Enabled {
				return `Tool Output: {"status": "error", "message": "Music generation is not enabled. Enable it in Settings > Music Generation."}`
			}
			prompt := stringValueFromMap(tc.Params, "prompt")
			if prompt == "" {
				return `Tool Output: {"status": "error", "message": "'prompt' is required for music generation."}`
			}
			lyrics := stringValueFromMap(tc.Params, "lyrics")
			instrumental := false
			if v, ok := tc.Params["instrumental"]; ok {
				if b, ok := v.(bool); ok {
					instrumental = b
				}
			}
			title := stringValueFromMap(tc.Params, "title")
			logger.Info("LLM requested music generation", "prompt_len", len(prompt), "provider", cfg.MusicGeneration.Provider)

			if budgetTracker != nil && budgetTracker.IsBlocked("music_generation") {
				return `Tool Output: {"status": "error", "message": "Music generation blocked: daily budget exceeded."}`
			}

			musicResult := tools.GenerateMusicResult(ctx, cfg, mediaRegistryDB, logger, tools.MusicGenParams{
				Prompt:       prompt,
				Lyrics:       lyrics,
				Instrumental: instrumental,
				Title:        title,
			})

			// Record cost in budget tracker
			if budgetTracker != nil && musicResult.CostEstimate > 0 {
				budgetTracker.RecordCostForCategory("music_generation", musicResult.CostEstimate)
			}

			return "Tool Output: " + tools.MusicResultToJSON(musicResult)

		case "generate_image":
			if !cfg.ImageGeneration.Enabled {
				return `Tool Output: {"status": "error", "message": "Image generation is not enabled. Enable it in Settings > Image Generation."}`
			}
			if cfg.ImageGeneration.APIKey == "" {
				return `Tool Output: {"status": "error", "message": "Image generation provider not configured. Set a provider in Settings > Image Generation."}`
			}
			req := decodeImageGenerationArgs(tc)
			prompt := req.Prompt
			if prompt == "" {
				return `Tool Output: {"status": "error", "message": "'prompt' is required for image generation."}`
			}
			logger.Info("LLM requested image generation", "prompt_len", len(prompt), "provider", cfg.ImageGeneration.ProviderType)

			// Check budget
			if budgetTracker != nil && budgetTracker.IsBlocked("image_generation") {
				return `Tool Output: {"status": "error", "message": "Image generation blocked: daily budget exceeded."}`
			}

			// Check monthly limit
			if cfg.ImageGeneration.MaxMonthly > 0 {
				count, err := tools.ImageGalleryMonthlyCount(imageGalleryDB)
				if err == nil && count >= cfg.ImageGeneration.MaxMonthly {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Monthly image generation limit reached (%d/%d). Try again next month or increase the limit in settings."}`, count, cfg.ImageGeneration.MaxMonthly)
				}
			}

			// Check daily limit
			if cfg.ImageGeneration.MaxDaily > 0 {
				count, allowed := tools.ImageGenCounterIncrement(cfg.ImageGeneration.MaxDaily)
				if !allowed {
					return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Daily image generation limit reached (%d/%d). Try again tomorrow or increase the limit in settings."}`, count, cfg.ImageGeneration.MaxDaily)
				}
			}

			// Prompt enhancement
			enhancedPrompt := ""
			doEnhance := cfg.ImageGeneration.PromptEnhancement
			if req.EnhancePrompt != nil {
				doEnhance = *req.EnhancePrompt
			}
			effectivePrompt := prompt
			if doEnhance {
				enhanced, err := tools.EnhanceImagePrompt(llmClient, cfg.LLM.Model, prompt)
				if err != nil {
					logger.Warn("Image prompt enhancement failed, using original", "error", err)
				} else {
					enhancedPrompt = enhanced
					effectivePrompt = enhanced
				}
			}

			// Build config
			genCfg := tools.ImageGenConfig{
				ProviderType: cfg.ImageGeneration.ProviderType,
				BaseURL:      cfg.ImageGeneration.BaseURL,
				APIKey:       cfg.ImageGeneration.APIKey,
				Model:        cfg.ImageGeneration.ResolvedModel,
				DataDir:      cfg.Directories.DataDir,
			}
			if req.Model != "" {
				genCfg.Model = req.Model
			}

			opts := tools.ImageGenOptions{
				Size:    req.Size,
				Quality: req.Quality,
				Style:   req.Style,
			}
			if opts.Size == "" {
				opts.Size = cfg.ImageGeneration.DefaultSize
			}
			if opts.Quality == "" {
				opts.Quality = cfg.ImageGeneration.DefaultQuality
			}
			if opts.Style == "" {
				opts.Style = cfg.ImageGeneration.DefaultStyle
			}
			if req.SourceImage != "" {
				opts.SourceImage = tools.ResolveSourceImagePath(req.SourceImage, cfg.Directories.WorkspaceDir, cfg.Directories.DataDir)
			}

			result, err := tools.GenerateImage(genCfg, effectivePrompt, opts)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Image generation failed: %s"}`, err.Error())
			}

			result.Prompt = prompt
			result.EnhancedPrompt = enhancedPrompt

			// Save to gallery DB
			tools.SaveGeneratedImage(imageGalleryDB, result)

			// Auto-register in media registry
			if mediaRegistryDB != nil {
				imgPath := filepath.Join(cfg.Directories.DataDir, "generated_images", result.Filename)
				imgHash, _ := tools.ComputeMediaFileHash(imgPath)
				if regID, dup, regErr := tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
					MediaType:        "image",
					SourceTool:       "generate_image",
					Filename:         result.Filename,
					FilePath:         imgPath,
					WebPath:          result.WebPath,
					Format:           "png",
					Provider:         result.Provider,
					Model:            result.Model,
					Prompt:           result.Prompt,
					Quality:          result.Quality,
					Style:            result.Style,
					Size:             result.Size,
					SourceImage:      result.SourceImage,
					GenerationTimeMs: int64(result.DurationMs),
					CostEstimate:     result.CostEstimate,
					Tags:             []string{"auto-generated"},
					Hash:             imgHash,
				}); regErr != nil {
					logger.Warn("Auto-register image in media registry failed", "filename", result.Filename, "error", regErr)
				} else if !dup {
					logger.Debug("Auto-registered image in media registry", "id", regID, "filename", result.Filename)
				}
			}

			// Record cost in budget tracker under "image_generation" category
			if budgetTracker != nil && result.CostEstimate > 0 {
				budgetTracker.RecordCostForCategory("image_generation", result.CostEstimate)
			}

			resultJSON, _ := json.Marshal(map[string]interface{}{
				"status":          "success",
				"web_path":        result.WebPath,
				"markdown":        result.Markdown,
				"prompt":          result.Prompt,
				"enhanced_prompt": result.EnhancedPrompt,
				"model":           result.Model,
				"provider":        result.Provider,
				"size":            result.Size,
				"duration_ms":     result.DurationMs,
			})
			return "Tool Output: " + string(resultJSON)

		case "query_inventory":
			req := decodeInventoryQueryArgs(tc)
			logger.Info("LLM requested inventory query", "tag", req.Tag, "name", req.Hostname)
			devices, err := inventory.QueryDevices(inventoryDB, req.Tag, req.DeviceType, req.Hostname)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to query inventory: %v"}`, err)
			}
			b, mErr := json.Marshal(devices)
			if mErr != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize devices: %v"}`, mErr)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "tag": "%s", "device_type": "%s", "name_match": "%s", "devices": %s}`, req.Tag, req.DeviceType, req.Hostname, string(b))

		case "execute_remote_shell", "remote_execution":
			if !cfg.Agent.AllowRemoteShell {
				return "Tool Output: [PERMISSION DENIED] execute_remote_shell is disabled in Danger Zone settings (agent.allow_remote_shell: false)."
			}
			req := decodeRemoteShellArgs(tc)
			logger.Info("LLM requested remote shell execution", "server_id", req.ServerID, "command", Truncate(req.Command, 200))
			if req.ServerID == "" || req.Command == "" {
				return `Tool Output: {"status": "error", "message": "'server_id' and 'command' are required"}`
			}
			device, err := inventory.GetDeviceByIDOrName(inventoryDB, req.ServerID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
			}
			access, err := resolveDeviceSSHAccess(device, inventoryDB, vault)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to resolve SSH access: %v"}`, err)
			}
			rCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			output, err := remote.ExecuteRemoteCommand(rCtx, access.Host, access.Port, access.Username, access.Secret, req.Command)
			if err != nil {
				safeOutput, mErr := json.Marshal(output)
				if mErr != nil {
					safeOutput = []byte(`""`)
				}
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Remote execution failed", "output": %s, "error": "%v"}`, string(safeOutput), err)
			}
			safeOutput, mErr := json.Marshal(output)
			if mErr != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize output: %v"}`, mErr)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "output": %s}`, string(safeOutput))

		case "transfer_remote_file":
			if !cfg.Agent.AllowRemoteShell {
				return "Tool Output: [PERMISSION DENIED] transfer_remote_file is disabled in Danger Zone settings (agent.allow_remote_shell: false)."
			}
			req := decodeRemoteFileTransferArgs(tc)
			logger.Info("LLM requested remote file transfer", "server_id", req.ServerID, "direction", req.Direction)
			if req.ServerID == "" || req.Direction == "" || req.LocalPath == "" || req.RemotePath == "" {
				return `Tool Output: {"status": "error", "message": "'server_id', 'direction', 'local_path', and 'remote_path' are required"}`
			}
			// Sanitize and restrict local path
			absLocal, err := filepath.Abs(req.LocalPath)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Invalid local path: %v"}`, err)
			}
			workspaceWorkdir := filepath.Join(cfg.Directories.WorkspaceDir, "workdir")
			if !strings.HasPrefix(strings.ToLower(absLocal), strings.ToLower(workspaceWorkdir)) {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Permission denied: local_path must be within %s"}`, workspaceWorkdir)
			}

			device, err := inventory.GetDeviceByIDOrName(inventoryDB, req.ServerID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
			}
			access, err := resolveDeviceSSHAccess(device, inventoryDB, vault)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to resolve SSH access: %v"}`, err)
			}
			rCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			err = remote.TransferFile(rCtx, access.Host, access.Port, access.Username, access.Secret, absLocal, req.RemotePath, req.Direction)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "File transfer failed: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "File %s successfully"}`, req.Direction)

		case "manage_schedule", "cron_scheduler":
			req := decodeCronScheduleArgs(tc)
			if !cfg.Tools.Scheduler.Enabled {
				return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
			}
			if cfg.Tools.Scheduler.ReadOnly {
				switch req.Operation {
				case "add", "remove", "enable", "disable":
					return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
				}
			}
			logger.Info("LLM requested cron management", "operation", req.Operation)
			result, err := cronManager.ManageSchedule(req.Operation, req.ID, req.CronExpr, req.TaskPrompt, cfg.Server.UILanguage)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR in manage_schedule: %v", err)
			}
			return result

		case "schedule_cron":
			req := decodeCronScheduleArgs(tc)
			if !cfg.Tools.Scheduler.Enabled {
				return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
			}
			if cfg.Tools.Scheduler.ReadOnly {
				return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
			}
			logger.Info("LLM requested cron scheduling", "expr", req.CronExpr)
			result, err := cronManager.ManageSchedule("add", "", req.CronExpr, req.TaskPrompt, cfg.Server.UILanguage)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR scheduling cron: %v", err)
			}
			return result

		case "list_cron_jobs":
			if !cfg.Tools.Scheduler.Enabled {
				return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
			}
			logger.Info("LLM requested cron job list")
			result, _ := cronManager.ManageSchedule("list", "", "", "", cfg.Server.UILanguage)
			return result

		case "remove_cron_job":
			req := decodeCronScheduleArgs(tc)
			if !cfg.Tools.Scheduler.Enabled {
				return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
			}
			if cfg.Tools.Scheduler.ReadOnly {
				return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
			}
			logger.Info("LLM requested cron job removal", "id", req.ID)
			result, _ := cronManager.ManageSchedule("remove", req.ID, "", "", cfg.Server.UILanguage)
			return result

		case "document_creator":
			req := decodeDocumentCreatorArgs(tc)
			if !cfg.Tools.DocumentCreator.Enabled {
				return `Tool Output: {"status":"error","message":"Document Creator is disabled. Set tools.document_creator.enabled=true in config.yaml."}`
			}
			logger.Info("LLM requested document creation", "operation", req.Operation, "backend", cfg.Tools.DocumentCreator.Backend)
			docResult := tools.ExecuteDocumentCreator(ctx, &cfg.Tools.DocumentCreator, req.Operation, req.Title, req.Content, req.URL, req.Filename, req.PaperSize, req.Landscape, req.Sections, req.SourceFiles)
			// Auto-register every successfully created document in the media registry
			if mediaRegistryDB != nil {
				var parsed struct {
					Status   string `json:"status"`
					FilePath string `json:"file_path"`
					WebPath  string `json:"web_path"`
					Filename string `json:"filename"`
				}
				if jsonErr := json.Unmarshal([]byte(docResult), &parsed); jsonErr == nil && parsed.Status == "success" {
					mediaType := "document"
					if req.Operation == "screenshot_url" || req.Operation == "screenshot_html" {
						mediaType = "image"
					}
					var fileSize int64
					if fi, fiErr := os.Stat(parsed.FilePath); fiErr == nil {
						fileSize = fi.Size()
					}
					docHash, _ := tools.ComputeMediaFileHash(parsed.FilePath)
					if regID, dup, regErr := tools.RegisterMedia(mediaRegistryDB, tools.MediaItem{
						MediaType:   mediaType,
						SourceTool:  "document_creator",
						Filename:    parsed.Filename,
						FilePath:    parsed.FilePath,
						WebPath:     parsed.WebPath,
						FileSize:    fileSize,
						Format:      strings.TrimPrefix(filepath.Ext(parsed.Filename), "."),
						Description: req.Title,
						Tags:        []string{"auto-generated"},
						Hash:        docHash,
					}); regErr != nil {
						logger.Warn("Auto-register document in media registry failed", "filename", parsed.Filename, "error", regErr)
					} else if !dup {
						logger.Debug("Auto-registered document in media registry", "id", regID, "filename", parsed.Filename)
					}
				}
			}
			return docResult

		default:
			handled = false
			return ""
		}
	}()
	return result, handled
}

type resolvedDeviceSSHAccess struct {
	Host     string
	Port     int
	Username string
	Secret   []byte
}

func resolveDeviceSSHAccess(device inventory.DeviceRecord, inventoryDB *sql.DB, vault *security.Vault) (resolvedDeviceSSHAccess, error) {
	host := strings.TrimSpace(device.IPAddress)
	if host == "" {
		host = strings.TrimSpace(device.Name)
	}
	username := strings.TrimSpace(device.Username)
	port := device.Port
	if port <= 0 {
		port = 22
	}
	secretID := strings.TrimSpace(device.VaultSecretID)

	if strings.TrimSpace(device.CredentialID) != "" {
		cred, err := credentials.GetByID(inventoryDB, device.CredentialID)
		if err != nil {
			return resolvedDeviceSSHAccess{}, fmt.Errorf("linked credential %q could not be loaded: %w", device.CredentialID, err)
		}
		if strings.TrimSpace(cred.Host) != "" {
			host = strings.TrimSpace(cred.Host)
		}
		if strings.TrimSpace(cred.Username) != "" {
			username = strings.TrimSpace(cred.Username)
		}
		switch {
		case strings.TrimSpace(cred.CertificateVaultID) != "":
			secretID = strings.TrimSpace(cred.CertificateVaultID)
		case strings.TrimSpace(cred.PasswordVaultID) != "":
			secretID = strings.TrimSpace(cred.PasswordVaultID)
		default:
			return resolvedDeviceSSHAccess{}, fmt.Errorf("linked credential %q has neither password nor certificate stored in the vault", cred.Name)
		}
	}

	if host == "" {
		return resolvedDeviceSSHAccess{}, fmt.Errorf("device host is missing")
	}
	if username == "" {
		return resolvedDeviceSSHAccess{}, fmt.Errorf("SSH username is missing")
	}
	if secretID == "" {
		return resolvedDeviceSSHAccess{}, fmt.Errorf("SSH secret is missing")
	}
	if vault == nil {
		return resolvedDeviceSSHAccess{}, fmt.Errorf("vault is not available")
	}

	resolvedVia := "legacy_vault_secret"
	if strings.TrimSpace(device.CredentialID) != "" {
		resolvedVia = "linked_credential"
	}
	slog.Info("SSH access resolved", "device", device.Name, "host", host, "user", username, "port", port, "resolved_via", resolvedVia)

	secret, err := vault.ReadSecret(secretID)
	if err != nil {
		return resolvedDeviceSSHAccess{}, fmt.Errorf("read vault secret %q: %w", secretID, err)
	}

	return resolvedDeviceSSHAccess{
		Host:     host,
		Port:     port,
		Username: username,
		Secret:   []byte(secret),
	}, nil
}

// isBlockedEnvRead returns true if the shell command appears to read an AURAGO_*
// environment variable. These variables include the master vault key and must never
// be accessible through the shell tool.
func isBlockedEnvRead(command string) bool {
	upper := strings.ToUpper(command)
	if !strings.Contains(upper, "AURAGO_") {
		return false
	}
	lower := strings.ToLower(command)
	// Match common env-reading patterns across sh/bash/zsh/PowerShell/scripting languages
	patterns := []string{
		"printenv", "echo", "$env:", "get-item", "get-childitem",
		"getenvironmentvariable", "[system.environment]", "environ", "export",
		"env ", " env", "/usr/bin/env",
		"set ", "awk", "python", "ruby", "perl", "node ",
		"cat /proc", "strings /proc", "/proc/self", "/proc/1/",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
