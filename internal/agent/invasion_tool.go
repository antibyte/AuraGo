package agent

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/invasion"
	"aurago/internal/security"
)

// hatchClient is used for loopback calls to the server's invasion API.
var hatchClient = &http.Client{Timeout: 30 * time.Second}

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
		case "assign_egg", "hatch_egg", "stop_egg", "send_task", "send_secret":
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
		return invasionEggDeployStatus(cfg, tc, logger)
	case "send_task":
		return invasionSendTask(cfg, db, tc, logger)
	case "send_secret":
		return invasionSendSecret(cfg, tc, logger)
	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown invasion_control operation '%s'. Use: list_nests, list_eggs, nest_status, assign_egg, hatch_egg, stop_egg, egg_status, send_task, send_secret"}`, tc.Operation)
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
	return fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Server.Port, path)
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

// invasionEggDeployStatus returns the deployment status of a nest.
func invasionEggDeployStatus(cfg *config.Config, tc ToolCall, logger *slog.Logger) string {
	if tc.NestID == "" {
		return `Tool Output: {"status":"error","message":"'nest_id' is required for egg_status."}`
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/status", tc.NestID))
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
	b, _ := json.Marshal(result)
	return "Tool Output: " + string(b)
}

// invasionSendTask sends a task to a running egg via the WebSocket bridge.
func invasionSendTask(cfg *config.Config, db *sql.DB, tc ToolCall, logger *slog.Logger) string {
	if tc.NestID == "" {
		return `Tool Output: {"status":"error","message":"'nest_id' is required for send_task."}`
	}
	taskDesc := tc.Task
	if taskDesc == "" {
		taskDesc = tc.Description
	}
	if taskDesc == "" {
		return `Tool Output: {"status":"error","message":"'task' (description) is required for send_task."}`
	}

	url := invasionLoopbackURL(cfg, fmt.Sprintf("/api/invasion/nests/%s/send-task", tc.NestID))
	body, _ := json.Marshal(map[string]interface{}{"description": taskDesc, "timeout": 0})
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
	logger.Info("[InvasionTool] Task sent to egg", "nest_id", tc.NestID, "task_id", taskID)
	bOut, _ := json.Marshal(map[string]string{"status": "success", "message": "Task sent to egg on nest " + tc.NestID, "task_id": taskID, "task": taskDesc})
	return "Tool Output: " + string(bOut)
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
