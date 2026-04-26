package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/security"
)

// hatchClient is used for loopback calls to the server's invasion API.
var hatchClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // SECURE: loopback-only internal API calls may use self-signed TLS
		},
		ForceAttemptHTTP2: false,
		DisableKeepAlives: true,
	},
}

const (
	invasionTaskResultDefaultWait = 25 * time.Second
	invasionTaskResultMaxWait     = 60 * time.Second
	invasionTaskResultPollEvery   = 250 * time.Millisecond
)

// agentInternalToken holds the per-process crypto token for loopback auth.
// Set by SetAgentInternalToken during startup before any loopback call is made.
// Accessed from multiple goroutines — stored as atomic.Value for race-free reads.
var agentInternalToken atomic.Value

// SetAgentInternalToken stores the loopback crypto token so all agent loopback
// HTTP calls can present it alongside X-Internal-FollowUp.
func SetAgentInternalToken(token string) {
	agentInternalToken.Store(token)
}

// handleInvasionControl dispatches invasion_control tool operations.
// It exposes read-only listing, status lookup, egg assignment, and deployment actions.
// Secrets are never included in responses.
func handleInvasionControl(tc ToolCall, cfg *config.Config, db *sql.DB, vault *security.Vault, logger *slog.Logger) string {
	if !cfg.InvasionControl.Enabled || db == nil {
		return `Tool Output: {"status":"error","message":"Invasion Control is disabled. Set invasion_control.enabled=true in config.yaml."}`
	}
	if cfg.InvasionControl.ReadOnly {
		switch tc.Operation {
		case "assign_egg", "hatch_egg", "stop_egg", "send_task", "send_secret", "ack_egg_message", "upload_artifact", "send_host_message":
			return `Tool Output: {"status":"error","message":"Invasion Control is in read-only mode. Disable invasion_control.read_only to allow changes."}`
		}
	}

	switch tc.Operation {
	case "list_nests":
		return invasionListNests(db, logger)
	case "list_eggs":
		return invasionListEggs(db, logger)
	case "nest_status":
		return invasionNestStatus(db, tc, logger)
	case "assign_egg":
		return invasionAssignEgg(db, tc, logger)
	case "hatch_egg":
		return invasionHatchEgg(cfg, tc, logger)
	case "stop_egg":
		return invasionStopEgg(cfg, tc, logger)
	case "egg_status":
		return invasionEggDeployStatus(cfg, db, tc, logger)
	case "send_task":
		return invasionSendTask(cfg, db, tc, logger)
	case "task_status", "get_result":
		return invasionTaskStatus(db, tc, logger)
	case "list_artifacts":
		return invasionListArtifacts(db, tc, logger)
	case "get_artifact":
		return invasionGetArtifact(db, tc, logger)
	case "read_artifact":
		return invasionReadArtifact(db, tc, logger)
	case "list_egg_messages":
		return invasionListEggMessages(db, tc, logger)
	case "ack_egg_message":
		return invasionAckEggMessage(db, tc, logger)
	case "upload_artifact":
		return invasionUploadArtifact(cfg, tc, logger)
	case "send_host_message":
		return invasionSendHostMessage(cfg, tc, logger)
	case "send_secret":
		return invasionSendSecret(cfg, tc, logger)
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown invasion_control operation '%s'. Use: list_nests, list_eggs, nest_status, assign_egg, hatch_egg, stop_egg, egg_status, send_task, task_status, get_result, list_artifacts, get_artifact, read_artifact, list_egg_messages, ack_egg_message, upload_artifact, send_host_message, send_secret"}`, tc.Operation)
	}
}

// invasionListNests returns all nests with safe metadata (no secrets).
func invasionListNests(db *sql.DB, logger *slog.Logger) string {
	nests, err := invasion.ListNests(db)
	if err != nil {
		logger.Error("[InvasionTool] Failed to list nests", "error", err)
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to list nests: %v"}`, err)
	}

	type safeNest struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Notes        string `json:"notes,omitempty"`
		AccessType   string `json:"access_type"`
		Host         string `json:"host"`
		Port         int    `json:"port"`
		Username     string `json:"username,omitempty"`
		Active       bool   `json:"active"`
		EggID        string `json:"egg_id,omitempty"`
		HatchStatus  string `json:"hatch_status"`
		DeployMethod string `json:"deploy_method"`
		TargetArch   string `json:"target_arch"`
		Route        string `json:"route"`
	}

	safe := make([]safeNest, len(nests))
	for i, n := range nests {
		safe[i] = safeNest{
			ID:           n.ID,
			Name:         n.Name,
			Notes:        n.Notes,
			AccessType:   n.AccessType,
			Host:         n.Host,
			Port:         n.Port,
			Username:     n.Username,
			Active:       n.Active,
			EggID:        n.EggID,
			HatchStatus:  n.HatchStatus,
			DeployMethod: n.DeployMethod,
			TargetArch:   n.TargetArch,
			Route:        n.Route,
		}
	}

	b, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"count":  len(safe),
		"nests":  safe,
	})
	return "Tool Output: " + string(b)
}

// invasionListEggs returns all eggs with safe metadata (no API keys).
func invasionListEggs(db *sql.DB, logger *slog.Logger) string {
	eggs, err := invasion.ListEggs(db)
	if err != nil {
		logger.Error("[InvasionTool] Failed to list eggs", "error", err)
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to list eggs: %v"}`, err)
	}

	type safeEgg struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Description  string `json:"description,omitempty"`
		Model        string `json:"model,omitempty"`
		Provider     string `json:"provider,omitempty"`
		Active       bool   `json:"active"`
		HasAPIKey    bool   `json:"has_api_key"`
		Permanent    bool   `json:"permanent"`
		IncludeVault bool   `json:"include_vault"`
		InheritLLM   bool   `json:"inherit_llm"`
		EggPort      int    `json:"egg_port"`
	}

	safe := make([]safeEgg, len(eggs))
	for i, e := range eggs {
		safe[i] = safeEgg{
			ID:           e.ID,
			Name:         e.Name,
			Description:  e.Description,
			Model:        e.Model,
			Provider:     e.Provider,
			Active:       e.Active,
			HasAPIKey:    e.APIKeyRef != "",
			Permanent:    e.Permanent,
			IncludeVault: e.IncludeVault,
			InheritLLM:   e.InheritLLM,
			EggPort:      e.EggPort,
		}
	}

	b, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"count":  len(safe),
		"eggs":   safe,
	})
	return "Tool Output: " + string(b)
}

// invasionNestStatus returns detailed info about a specific nest by ID or name.
func invasionNestStatus(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	var nest invasion.NestRecord
	var err error

	if tc.NestID != "" {
		nest, err = invasion.GetNest(db, tc.NestID)
	} else if tc.NestName != "" {
		nest, err = invasion.GetNestByName(db, tc.NestName)
	} else {
		return `Tool Output: {"status":"error","message":"Provide 'nest_id' or 'nest_name' for nest_status operation."}`
	}

	if err != nil {
		logger.Warn("[InvasionTool] Nest lookup failed", "nest_id", tc.NestID, "nest_name", tc.NestName, "error", err)
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Nest not found: %v"}`, err)
	}

	// Optionally resolve egg name if assigned
	eggName := ""
	if nest.EggID != "" {
		if egg, eErr := invasion.GetEgg(db, nest.EggID); eErr == nil {
			eggName = egg.Name
		}
	}

	result := map[string]interface{}{
		"status":        "success",
		"id":            nest.ID,
		"name":          nest.Name,
		"notes":         nest.Notes,
		"access_type":   nest.AccessType,
		"host":          nest.Host,
		"port":          nest.Port,
		"username":      nest.Username,
		"active":        nest.Active,
		"has_secret":    nest.VaultSecretID != "",
		"egg_id":        nest.EggID,
		"egg_name":      eggName,
		"hatch_status":  nest.HatchStatus,
		"hatch_error":   nest.HatchError,
		"last_hatch_at": nest.LastHatchAt,
		"deploy_method": nest.DeployMethod,
		"target_arch":   nest.TargetArch,
		"route":         nest.Route,
		"created_at":    nest.CreatedAt,
		"updated_at":    nest.UpdatedAt,
	}

	b, _ := json.Marshal(result)
	return "Tool Output: " + string(b)
}

// invasionAssignEgg assigns an egg to a nest.
func invasionAssignEgg(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	if tc.NestID == "" {
		return `Tool Output: {"status":"error","message":"'nest_id' is required for assign_egg operation."}`
	}

	// Verify nest exists
	nest, err := invasion.GetNest(db, tc.NestID)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Nest not found: %v"}`, err)
	}

	// If egg_id is empty, unassign the egg
	if tc.EggID == "" {
		nest.EggID = ""
		if uErr := invasion.UpdateNest(db, nest); uErr != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to unassign egg: %v"}`, uErr)
		}
		logger.Info("[InvasionTool] Egg unassigned from nest", "nest_id", nest.ID, "nest_name", nest.Name)
		b, _ := json.Marshal(map[string]string{"status": "success", "message": "Egg unassigned from nest '" + nest.Name + "'."})
		return "Tool Output: " + string(b)
	}

	// Verify egg exists and is active
	egg, err := invasion.GetEgg(db, tc.EggID)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Egg not found: %v"}`, err)
	}
	if !egg.Active {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": "Egg '" + egg.Name + "' is inactive. Activate it first before assigning."})
		return "Tool Output: " + string(b)
	}

	nest.EggID = tc.EggID
	if uErr := invasion.UpdateNest(db, nest); uErr != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to assign egg: %v"}`, uErr)
	}

	logger.Info("[InvasionTool] Egg assigned to nest", "nest_id", nest.ID, "nest_name", nest.Name, "egg_id", egg.ID, "egg_name", egg.Name)
	b, _ := json.Marshal(map[string]string{"status": "success", "message": "Egg '" + egg.Name + "' assigned to nest '" + nest.Name + "'.", "nest_id": nest.ID, "egg_id": egg.ID})
	return "Tool Output: " + string(b)
}

// ── Deployment operations (use loopback HTTP to server endpoints) ───────────

// invasionLoopbackURL builds a URL for the local server API.
func invasionLoopbackURL(cfg *config.Config, path string) string {
	return internalAPIBaseURL(cfg) + path
}

// invasionPost performs an authenticated loopback POST to the server's invasion API.
// The X-Internal-FollowUp header bypasses session auth for loopback callers.
func invasionPost(url, contentType string, body *bytes.Reader) (*http.Response, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(http.MethodPost, url, body)
	} else {
		req, err = http.NewRequest(http.MethodPost, url, nil)
	}
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Internal-FollowUp", "true")
	if tok, _ := agentInternalToken.Load().(string); tok != "" {
		req.Header.Set("X-Internal-Token", tok)
	}
	return hatchClient.Do(req)
}

// invasionGet performs an authenticated loopback GET to the server's invasion API.
func invasionGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-FollowUp", "true")
	if tok, _ := agentInternalToken.Load().(string); tok != "" {
		req.Header.Set("X-Internal-Token", tok)
	}
	return hatchClient.Do(req)
}

// invasionHatchEgg triggers deployment of an egg to a nest.
func invasionHatchEgg(cfg *config.Config, tc ToolCall, logger *slog.Logger) string {
	if tc.NestID == "" {
		return `Tool Output: {"status":"error","message":"'nest_id' is required for hatch_egg."}`
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/hatch", tc.NestID))
	resp, err := invasionPost(url, "application/json", nil)
	if err != nil {
		logger.Error("[InvasionTool] hatch_egg loopback failed", "error", err)
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Hatch request failed: %v"}`, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return `Tool Output: {"status":"error","message":"Failed to decode hatch response"}`
	}

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := result["error"].(string)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": errMsg})
		return "Tool Output: " + string(b)
	}

	b, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Hatch initiated for nest %s", tc.NestID),
		"detail":  result,
	})
	logger.Info("[InvasionTool] Hatch initiated", "nest_id", tc.NestID)
	return "Tool Output: " + string(b)
}

// invasionStopEgg stops a running egg on a nest.
func invasionStopEgg(cfg *config.Config, tc ToolCall, logger *slog.Logger) string {
	if tc.NestID == "" {
		return `Tool Output: {"status":"error","message":"'nest_id' is required for stop_egg."}`
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/stop", tc.NestID))
	resp, err := invasionPost(url, "application/json", nil)
	if err != nil {
		logger.Error("[InvasionTool] stop_egg loopback failed", "error", err)
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Stop request failed: %v"}`, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := result["error"].(string)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": errMsg})
		return "Tool Output: " + string(b)
	}

	logger.Info("[InvasionTool] Egg stopped", "nest_id", tc.NestID)
	return fmt.Sprintf(`Tool Output: {"status":"success","message":"Egg stopped on nest %s"}`, tc.NestID)
}

// invasionEggDeployStatus returns the deployment status of a nest or named egg.
func invasionEggDeployStatus(cfg *config.Config, db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	nest, egg, err := resolveInvasionEggStatusTarget(db, tc, logger)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
		return "Tool Output: " + string(b)
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/status", nest.ID))
	resp, err := invasionGet(url)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Status request failed: %v"}`, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := result["error"].(string)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": errMsg})
		return "Tool Output: " + string(b)
	}

	result["status"] = "success"
	result["nest_id"] = nest.ID
	result["nest_name"] = nest.Name
	result["egg_id"] = egg.ID
	result["egg_name"] = egg.Name
	b, _ := json.Marshal(result)
	return "Tool Output: " + string(b)
}

// invasionSendTask sends a task to a running egg via the WebSocket bridge.
func invasionSendTask(cfg *config.Config, db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	taskDesc := tc.Task
	if taskDesc == "" {
		taskDesc = tc.Description
	}
	if taskDesc == "" {
		return `Tool Output: {"status":"error","message":"'task' (description) is required for send_task."}`
	}

	nest, egg, err := resolveInvasionTaskNest(db, tc, logger)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
		return "Tool Output: " + string(b)
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/send-task", nest.ID))
	taskTimeout := tc.Timeout
	if taskTimeout < 0 {
		taskTimeout = 0
	}
	body, _ := json.Marshal(map[string]interface{}{"description": taskDesc, "timeout": taskTimeout})
	resp, err := invasionPost(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Error("[InvasionTool] send_task loopback failed", "error", err)
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Send-task request failed: %v"}`, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := result["error"].(string)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": errMsg})
		return "Tool Output: " + string(b)
	}

	taskID, _ := result["task_id"].(string)
	logger.Info("[InvasionTool] Task sent to egg", "nest_id", nest.ID, "nest_name", nest.Name, "egg_id", egg.ID, "egg_name", egg.Name, "task_id", taskID)
	out := map[string]interface{}{
		"status":    "success",
		"message":   "Task sent to egg '" + egg.Name + "' on nest '" + nest.Name + "'.",
		"task_id":   taskID,
		"task":      taskDesc,
		"nest_id":   nest.ID,
		"nest_name": nest.Name,
		"egg_id":    egg.ID,
		"egg_name":  egg.Name,
	}
	if taskID != "" {
		task, completed, waitErr := waitForInvasionTaskResult(db, taskID, invasionTaskWaitDuration(tc), invasionTaskResultPollEvery)
		if waitErr != nil {
			logger.Warn("[InvasionTool] Failed to wait for egg task result", "task_id", taskID, "error", waitErr)
			out["result_available"] = false
			out["message"] = "Task sent, but checking the result failed: " + waitErr.Error()
		} else if task != nil {
			mergeInvasionTaskResult(out, task)
			if completed {
				out["result_available"] = true
				out["message"] = "Task completed by egg '" + egg.Name + "' on nest '" + nest.Name + "'."
				if isFailedInvasionTaskStatus(task.Status) {
					out["status"] = "error"
					out["message"] = "Task failed on egg '" + egg.Name + "' on nest '" + nest.Name + "'."
				}
			} else {
				out["result_available"] = false
				out["message"] = "Task sent to egg '" + egg.Name + "' on nest '" + nest.Name + "', but the result is still pending. Call invasion_control with operation 'task_status' and this task_id to check again."
			}
		}
	}
	bOut, _ := json.Marshal(out)
	return "Tool Output: " + string(bOut)
}

func invasionTaskStatus(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	taskID := strings.TrimSpace(tc.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(tc.ID)
	}
	if taskID == "" {
		return `Tool Output: {"status":"error","message":"'task_id' is required for task_status/get_result."}`
	}

	task, err := invasion.GetTaskByID(db, taskID)
	if err != nil {
		logger.Warn("[InvasionTool] Failed to get task status", "task_id", taskID, "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "task_id": taskID})
		return "Tool Output: " + string(b)
	}

	out := map[string]interface{}{
		"status":           "success",
		"task_id":          task.ID,
		"task_status":      task.Status,
		"result_available": isTerminalInvasionTaskStatus(task.Status),
	}
	mergeInvasionTaskResult(out, task)
	if nest, err := invasion.GetNest(db, task.NestID); err == nil {
		out["nest_name"] = nest.Name
	}
	if egg, err := invasion.GetEgg(db, task.EggID); err == nil {
		out["egg_name"] = egg.Name
	}
	if isFailedInvasionTaskStatus(task.Status) {
		out["status"] = "error"
	}
	b, _ := json.Marshal(out)
	return "Tool Output: " + string(b)
}

func invasionListArtifacts(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	artifacts, err := invasion.ListArtifacts(db, invasion.ArtifactFilter{
		NestID:    strings.TrimSpace(tc.NestID),
		EggID:     strings.TrimSpace(tc.EggID),
		MissionID: strings.TrimSpace(tc.MissionID),
		TaskID:    strings.TrimSpace(tc.TaskID),
		Status:    strings.TrimSpace(tc.Status),
		Limit:     tc.Limit,
	})
	if err != nil {
		logger.Warn("[InvasionTool] Failed to list artifacts", "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
		return "Tool Output: " + string(b)
	}
	out := map[string]interface{}{"status": "success", "count": len(artifacts), "artifacts": artifacts}
	b, _ := json.Marshal(out)
	return "Tool Output: " + string(b)
}

func invasionGetArtifact(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	artifactID := invasionArtifactID(tc)
	if artifactID == "" {
		return `Tool Output: {"status":"error","message":"'artifact_id' is required"}`
	}
	artifact, err := invasion.GetArtifact(db, artifactID)
	if err != nil {
		logger.Warn("[InvasionTool] Failed to get artifact", "artifact_id", artifactID, "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "artifact_id": artifactID})
		return "Tool Output: " + string(b)
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "success", "artifact": artifact})
	return "Tool Output: " + string(b)
}

func invasionReadArtifact(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	artifactID := invasionArtifactID(tc)
	if artifactID == "" {
		return `Tool Output: {"status":"error","message":"'artifact_id' is required"}`
	}
	artifact, err := invasion.GetArtifact(db, artifactID)
	if err != nil {
		logger.Warn("[InvasionTool] Failed to read artifact metadata", "artifact_id", artifactID, "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "artifact_id": artifactID})
		return "Tool Output: " + string(b)
	}
	if artifact.Status != invasion.ArtifactStatusCompleted || artifact.StoragePath == "" {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": "artifact is not available", "artifact_id": artifactID})
		return "Tool Output: " + string(b)
	}
	if !isTextReadableArtifact(artifact) {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": "artifact is not text-readable; use get_artifact for path and metadata", "artifact_id": artifactID})
		return "Tool Output: " + string(b)
	}
	info, err := os.Stat(artifact.StoragePath)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "artifact_id": artifactID})
		return "Tool Output: " + string(b)
	}
	const maxRead = 1 << 20
	if info.Size() > maxRead {
		b, _ := json.Marshal(map[string]interface{}{"status": "error", "message": "artifact is too large to read directly", "artifact_id": artifactID, "size_bytes": info.Size(), "max_bytes": maxRead})
		return "Tool Output: " + string(b)
	}
	data, err := os.ReadFile(artifact.StoragePath)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "artifact_id": artifactID})
		return "Tool Output: " + string(b)
	}
	out := map[string]interface{}{"status": "success", "artifact": artifact, "content": string(data)}
	b, _ := json.Marshal(out)
	return "Tool Output: " + string(b)
}

func invasionListEggMessages(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	messages, err := invasion.ListEggMessages(db, invasion.EggMessageFilter{
		NestID: strings.TrimSpace(tc.NestID),
		EggID:  strings.TrimSpace(tc.EggID),
		Limit:  tc.Limit,
	})
	if err != nil {
		logger.Warn("[InvasionTool] Failed to list egg messages", "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
		return "Tool Output: " + string(b)
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "success", "count": len(messages), "messages": messages})
	return "Tool Output: " + string(b)
}

func invasionAckEggMessage(db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	id := strings.TrimSpace(firstNonEmpty(tc.ID, tc.MessageID))
	if id == "" {
		return `Tool Output: {"status":"error","message":"'id' is required for ack_egg_message"}`
	}
	if err := invasion.AcknowledgeEggMessage(db, id, time.Now()); err != nil {
		logger.Warn("[InvasionTool] Failed to acknowledge egg message", "id", id, "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "id": id})
		return "Tool Output: " + string(b)
	}
	b, _ := json.Marshal(map[string]string{"status": "success", "id": id})
	return "Tool Output: " + string(b)
}

func invasionUploadArtifact(cfg *config.Config, tc ToolCall, logger *slog.Logger) string {
	if cfg == nil || !cfg.EggMode.Enabled {
		return `Tool Output: {"status":"error","message":"upload_artifact is only available when this AuraGo instance runs in egg mode."}`
	}
	localPath := strings.TrimSpace(firstNonEmpty(tc.FilePath, tc.Path, tc.LocalPath))
	if localPath == "" {
		return `Tool Output: {"status":"error","message":"'file_path' is required for upload_artifact"}`
	}
	resolvedPath, err := resolveEggArtifactPath(cfg, localPath)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "file_path": localPath})
		return "Tool Output: " + string(b)
	}
	f, err := os.Open(resolvedPath)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "file_path": localPath})
		return "Tool Output: " + string(b)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "file_path": localPath})
		return "Tool Output: " + string(b)
	}
	if info.IsDir() {
		return `Tool Output: {"status":"error","message":"upload_artifact requires a file, not a directory"}`
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "file_path": localPath})
		return "Tool Output: " + string(b)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "file_path": localPath})
		return "Tool Output: " + string(b)
	}
	filename := strings.TrimSpace(tc.Filename)
	if filename == "" {
		filename = filepath.Base(resolvedPath)
	}
	mimeType := strings.TrimSpace(firstNonEmpty(tc.MIMEType, tc.ContentType))
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(filename))
	}
	client := newEggHTTPClient(cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := client.UploadArtifact(ctx, bridge.EggArtifactUpload{
		MissionID:      strings.TrimSpace(tc.MissionID),
		TaskID:         strings.TrimSpace(tc.TaskID),
		Filename:       filename,
		MIMEType:       mimeType,
		ExpectedSize:   info.Size(),
		ExpectedSHA256: hex.EncodeToString(hasher.Sum(nil)),
		Metadata:       tc.Metadata,
		Reader:         f,
	})
	if err != nil {
		logger.Warn("[InvasionTool] Egg artifact upload failed", "file_path", localPath, "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error(), "file_path": localPath})
		return "Tool Output: " + string(b)
	}
	out := map[string]interface{}{
		"status":      "success",
		"artifact_id": result.ArtifactID,
		"web_path":    result.WebPath,
		"sha256":      result.SHA256,
		"size_bytes":  result.SizeBytes,
		"file_path":   resolvedPath,
	}
	b, _ := json.Marshal(out)
	return "Tool Output: " + string(b)
}

func resolveEggArtifactPath(cfg *config.Config, requestedPath string) (string, error) {
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" {
		return "", fmt.Errorf("file_path is required")
	}
	workspace := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	candidate := requestedPath
	if workspace != "" && !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspace, candidate)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	if evaluated, err := filepath.EvalSymlinks(absCandidate); err == nil {
		absCandidate = evaluated
	}
	if workspace == "" {
		return absCandidate, nil
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	if evaluated, err := filepath.EvalSymlinks(absWorkspace); err == nil {
		absWorkspace = evaluated
	}
	rel, err := filepath.Rel(absWorkspace, absCandidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("artifact path must stay inside the Egg workspace")
	}
	return absCandidate, nil
}

func invasionSendHostMessage(cfg *config.Config, tc ToolCall, logger *slog.Logger) string {
	if cfg == nil || !cfg.EggMode.Enabled {
		return `Tool Output: {"status":"error","message":"send_host_message is only available when this AuraGo instance runs in egg mode."}`
	}
	body := strings.TrimSpace(firstNonEmpty(tc.Body, tc.Message, tc.Content, tc.Text, tc.Description))
	if body == "" {
		return `Tool Output: {"status":"error","message":"'body' or 'message' is required for send_host_message"}`
	}
	artifactIDs := append([]string{}, tc.ArtifactIDs...)
	if len(artifactIDs) == 0 && strings.TrimSpace(tc.ArtifactID) != "" {
		artifactIDs = []string{strings.TrimSpace(tc.ArtifactID)}
	}
	client := newEggHTTPClient(cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := client.SendHostMessage(ctx, bridge.EggHostMessage{
		MissionID:       strings.TrimSpace(tc.MissionID),
		TaskID:          strings.TrimSpace(tc.TaskID),
		Severity:        strings.TrimSpace(tc.Severity),
		Title:           strings.TrimSpace(firstNonEmpty(tc.Title, tc.Subject, "Egg message")),
		Body:            body,
		ArtifactIDs:     artifactIDs,
		DedupKey:        strings.TrimSpace(tc.DedupKey),
		WakeupRequested: tc.WakeupRequested,
	})
	if err != nil {
		logger.Warn("[InvasionTool] Egg host message failed", "error", err)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
		return "Tool Output: " + string(b)
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "success", "message_id": result.MessageID, "wakeup_allowed": result.WakeupAllowed})
	return "Tool Output: " + string(b)
}

func newEggHTTPClient(cfg *config.Config, logger *slog.Logger) *bridge.EggClient {
	client := bridge.NewEggClient(
		cfg.EggMode.MasterURL,
		cfg.EggMode.EggID,
		cfg.EggMode.NestID,
		cfg.EggMode.SharedKey,
		"1.0.0",
		logger,
	)
	client.TLSSkipVerify = cfg.EggMode.TLSSkipVerify
	return client
}

func invasionArtifactID(tc ToolCall) string {
	return strings.TrimSpace(firstNonEmpty(tc.ArtifactID, tc.ID, tc.FileID))
}

func isTextReadableArtifact(artifact invasion.ArtifactRecord) bool {
	mime := strings.ToLower(strings.TrimSpace(artifact.MIMEType))
	name := strings.ToLower(strings.TrimSpace(artifact.Filename))
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	switch mime {
	case "application/json", "application/xml", "application/yaml", "application/x-yaml", "application/toml", "application/csv":
		return true
	}
	for _, suffix := range []string{".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml", ".toml", ".xml"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func waitForInvasionTaskResult(db *sql.DB, taskID string, waitFor, pollEvery time.Duration) (*invasion.TaskRecord, bool, error) {
	if db == nil || strings.TrimSpace(taskID) == "" {
		return nil, false, nil
	}
	if waitFor <= 0 {
		waitFor = invasionTaskResultDefaultWait
	}
	if pollEvery <= 0 {
		pollEvery = invasionTaskResultPollEvery
	}

	deadline := time.Now().Add(waitFor)
	for {
		task, err := invasion.GetTaskByID(db, taskID)
		if err != nil {
			return nil, false, err
		}
		if isTerminalInvasionTaskStatus(task.Status) {
			return task, true, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return task, false, nil
		}
		if remaining < pollEvery {
			time.Sleep(remaining)
		} else {
			time.Sleep(pollEvery)
		}
	}
}

func invasionTaskWaitDuration(tc ToolCall) time.Duration {
	if tc.Timeout <= 0 {
		return invasionTaskResultDefaultWait
	}
	waitFor := time.Duration(tc.Timeout) * time.Second
	if waitFor > invasionTaskResultMaxWait {
		return invasionTaskResultMaxWait
	}
	return waitFor
}

func mergeInvasionTaskResult(out map[string]interface{}, task *invasion.TaskRecord) {
	out["task_id"] = task.ID
	out["task_status"] = task.Status
	out["description"] = task.Description
	out["timeout"] = task.Timeout
	out["nest_id"] = task.NestID
	out["egg_id"] = task.EggID
	out["result_output"] = task.ResultOutput
	out["result_error"] = task.ResultError
	out["artifact_ids"] = task.ArtifactIDs
	out["created_at"] = task.CreatedAt
	out["sent_at"] = task.SentAt
	out["completed_at"] = task.CompletedAt
}

func isTerminalInvasionTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "timeout":
		return true
	default:
		return false
	}
}

func isFailedInvasionTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "timeout":
		return true
	default:
		return false
	}
}

func resolveInvasionTaskNest(db *sql.DB, tc ToolCall, logger *slog.Logger) (invasion.NestRecord, invasion.EggRecord, error) {
	if db == nil {
		return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("invasion database is unavailable")
	}

	var nest invasion.NestRecord
	var err error
	switch {
	case strings.TrimSpace(tc.NestID) != "":
		nest, err = resolveInvasionNestRef(db, strings.TrimSpace(tc.NestID))
	case strings.TrimSpace(tc.NestName) != "":
		nest, err = invasion.GetNestByName(db, strings.TrimSpace(tc.NestName))
	}
	if err != nil {
		return invasion.NestRecord{}, invasion.EggRecord{}, err
	}
	if nest.ID != "" {
		if nest.EggID == "" {
			return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("nest %q has no assigned egg", nest.Name)
		}
		egg, eggErr := invasion.GetEgg(db, nest.EggID)
		if eggErr != nil {
			return invasion.NestRecord{}, invasion.EggRecord{}, eggErr
		}
		return nest, egg, nil
	}

	egg, err := resolveInvasionEggForTask(db, tc)
	if err != nil {
		return invasion.NestRecord{}, invasion.EggRecord{}, err
	}

	nests, err := invasion.ListNests(db)
	if err != nil {
		return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("failed to list nests for egg lookup: %w", err)
	}

	var running []invasion.NestRecord
	var active []invasion.NestRecord
	for _, candidate := range nests {
		if candidate.EggID != egg.ID {
			continue
		}
		if !candidate.Active {
			continue
		}
		active = append(active, candidate)
		if strings.EqualFold(candidate.HatchStatus, "running") {
			running = append(running, candidate)
		}
	}

	switch {
	case len(running) == 1:
		return running[0], egg, nil
	case len(running) > 1:
		return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("egg %q is running on multiple nests (%s); provide nest_id or nest_name", egg.Name, joinNestNames(running))
	case len(active) == 1:
		return active[0], egg, nil
	case len(active) > 1:
		return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("egg %q is assigned to multiple active nests (%s); provide nest_id or nest_name", egg.Name, joinNestNames(active))
	default:
		return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("egg %q is not assigned to an active nest", egg.Name)
	}
}

func resolveInvasionEggStatusTarget(db *sql.DB, tc ToolCall, logger *slog.Logger) (invasion.NestRecord, invasion.EggRecord, error) {
	if strings.TrimSpace(tc.NestID) == "" &&
		strings.TrimSpace(tc.NestName) == "" &&
		strings.TrimSpace(tc.EggID) == "" &&
		strings.TrimSpace(tc.EggName) == "" {
		return invasion.NestRecord{}, invasion.EggRecord{}, fmt.Errorf("provide nest_id, nest_name, egg_id, or egg_name for egg_status")
	}
	return resolveInvasionTaskNest(db, tc, logger)
}

func resolveInvasionNestRef(db *sql.DB, ref string) (invasion.NestRecord, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return invasion.NestRecord{}, fmt.Errorf("nest reference is empty")
	}

	nest, directErr := invasion.GetNest(db, trimmed)
	if directErr == nil {
		return nest, nil
	}

	shortRef := strings.TrimPrefix(trimmed, "aurago-egg-")
	shortRef = strings.TrimPrefix(shortRef, "egg-")
	if shortRef == trimmed || len(shortRef) < 6 {
		return invasion.NestRecord{}, directErr
	}

	nests, err := invasion.ListNests(db)
	if err != nil {
		return invasion.NestRecord{}, fmt.Errorf("failed to list nests for %q lookup: %w", trimmed, err)
	}
	var matches []invasion.NestRecord
	for _, candidate := range nests {
		if strings.HasPrefix(strings.ToLower(candidate.ID), strings.ToLower(shortRef)) {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return invasion.NestRecord{}, directErr
	case 1:
		return matches[0], nil
	default:
		return invasion.NestRecord{}, fmt.Errorf("nest reference %q is ambiguous; provide the full nest_id", trimmed)
	}
}

func resolveInvasionEggForTask(db *sql.DB, tc ToolCall) (invasion.EggRecord, error) {
	if strings.TrimSpace(tc.EggID) != "" {
		return invasion.GetEgg(db, strings.TrimSpace(tc.EggID))
	}
	eggName := strings.TrimSpace(tc.EggName)
	if eggName == "" {
		return invasion.EggRecord{}, fmt.Errorf("provide nest_id, nest_name, egg_id, or egg_name for send_task")
	}

	eggs, err := invasion.ListEggs(db)
	if err != nil {
		return invasion.EggRecord{}, fmt.Errorf("failed to list eggs for name lookup: %w", err)
	}
	var matches []invasion.EggRecord
	for _, egg := range eggs {
		if strings.EqualFold(strings.TrimSpace(egg.Name), eggName) {
			matches = append(matches, egg)
		}
	}
	switch len(matches) {
	case 0:
		return invasion.EggRecord{}, fmt.Errorf("egg not found by name: %s", eggName)
	case 1:
		return matches[0], nil
	default:
		return invasion.EggRecord{}, fmt.Errorf("multiple eggs are named %q; provide egg_id", eggName)
	}
}

func joinNestNames(nests []invasion.NestRecord) string {
	names := make([]string, 0, len(nests))
	for _, nest := range nests {
		if nest.Name != "" {
			names = append(names, nest.Name)
		} else {
			names = append(names, nest.ID)
		}
	}
	return strings.Join(names, ", ")
}

// invasionSendSecret sends a secret to a running egg via the encrypted channel.
func invasionSendSecret(cfg *config.Config, tc ToolCall, logger *slog.Logger) string {
	if tc.NestID == "" {
		return `Tool Output: {"status":"error","message":"'nest_id' is required for send_secret."}`
	}
	if tc.Key == "" {
		return `Tool Output: {"status":"error","message":"'key' is required for send_secret."}`
	}
	if tc.Value == "" {
		return `Tool Output: {"status":"error","message":"'value' is required for send_secret."}`
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/send-secret", tc.NestID))
	body, _ := json.Marshal(map[string]string{"key": tc.Key, "value": tc.Value})
	resp, err := invasionPost(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Send-secret request failed: %v"}`, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := result["error"].(string)
		b, _ := json.Marshal(map[string]string{"status": "error", "message": errMsg})
		return "Tool Output: " + string(b)
	}

	logger.Info("[InvasionTool] Secret sent to egg", "nest_id", tc.NestID, "key", tc.Key)
	bOut, _ := json.Marshal(map[string]string{"status": "success", "message": "Secret '" + tc.Key + "' sent to egg on nest " + tc.NestID})
	return "Tool Output: " + string(bOut)
}
