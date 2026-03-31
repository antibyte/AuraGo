package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/security"
)

// NetlifyRollback restores a previous deploy for a site.
func NetlifyRollback(cfg NetlifyConfig, siteID, deployID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}
	if deployID == "" {
		return errJSON("deploy_id is required for rollback")
	}

	// Restore a previous deploy by publishing it
	data, code, err := netlifyRequest(cfg, "POST", fmt.Sprintf("/sites/%s/rollback/%s", siteID, deployID), nil)
	if err != nil {
		return errJSON("Failed to rollback: %v", err)
	}
	// Netlify returns a 204 for the restore endpoint — but the rollback endpoint
	// may vary; accept 200/201/204.
	if code != 200 && code != 201 && code != 204 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"message":   "Rolled back to deploy " + deployID,
		"deploy_id": deployID,
	})
	return string(out)
}

// NetlifyCancelDeploy cancels a pending or in-progress deploy.
func NetlifyCancelDeploy(cfg NetlifyConfig, deployID string) string {
	if deployID == "" {
		return errJSON("deploy_id is required")
	}

	data, code, err := netlifyRequest(cfg, "POST", "/deploys/"+deployID+"/cancel", nil)
	if err != nil {
		return errJSON("Failed to cancel deploy: %v", err)
	}
	if code != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Deploy cancelled", "deploy_id": deployID})
	return string(out)
}

// ── Environment Variables ───────────────────────────────────────────────────

// NetlifyListEnvVars returns all env vars for a site.
func NetlifyListEnvVars(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/accounts/%s/env?site_id=%s", cfg.TeamSlug, siteID), nil)
	if err != nil {
		return errJSON("Failed to list env vars: %v", err)
	}
	if code != 200 {
		// Fallback to site-level env vars API
		data, code, err = netlifyRequest(cfg, "GET", fmt.Sprintf("/sites/%s/env", siteID), nil)
		if err != nil {
			return errJSON("Failed to list env vars: %v", err)
		}
		if code != 200 {
			return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
		}
	}

	var envVars []map[string]interface{}
	if err := json.Unmarshal(data, &envVars); err != nil {
		return errJSON("Failed to parse env vars: %v", err)
	}

	type compactEnv struct {
		Key       string   `json:"key"`
		Scopes    []string `json:"scopes"`
		UpdatedAt string   `json:"updated_at"`
		Values    int      `json:"values_count"`
	}

	var result []compactEnv
	for _, ev := range envVars {
		key := strVal(ev, "key")
		scopes := toStringSlice(ev["scopes"])
		valuesRaw, _ := ev["values"].([]interface{})
		result = append(result, compactEnv{
			Key:       key,
			Scopes:    scopes,
			UpdatedAt: strVal(ev, "updated_at"),
			Values:    len(valuesRaw),
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "env_vars": result})
	return string(out)
}

// NetlifyGetEnvVar returns a specific env var's details.
func NetlifyGetEnvVar(cfg NetlifyConfig, siteID, key string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" || key == "" {
		return errJSON("site_id and key are required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/accounts/%s/env/%s?site_id=%s", cfg.TeamSlug, key, siteID), nil)
	if err != nil {
		return errJSON("Failed to get env var: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	// Return raw response (contains values with context info)
	var env map[string]interface{}
	if err := json.Unmarshal(data, &env); err != nil {
		return errJSON("Failed to parse env var: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "env_var": env})
	return string(out)
}

// NetlifySetEnvVar creates or updates an environment variable.
func NetlifySetEnvVar(cfg NetlifyConfig, siteID, key, value, envContext string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" || key == "" {
		return errJSON("site_id and key are required")
	}
	if envContext == "" {
		envContext = "all" // default context: all deploy contexts
	}

	// Use the account-level env API which supports context-scoped values
	body := []map[string]interface{}{
		{
			"key":    key,
			"scopes": []string{"builds", "functions", "runtime", "post_processing"},
			"values": []map[string]interface{}{
				{
					"value":   value,
					"context": envContext,
				},
			},
		},
	}

	data, code, err := netlifyRequest(cfg, "POST", fmt.Sprintf("/accounts/%s/env?site_id=%s", cfg.TeamSlug, siteID), body)
	if err != nil {
		return errJSON("Failed to set env var: %v", err)
	}
	// 200 for update, 201 for create
	if code != 200 && code != 201 {
		// Try PATCH for update if POST fails (already exists)
		patchBody := map[string]interface{}{
			"key":    key,
			"scopes": []string{"builds", "functions", "runtime", "post_processing"},
			"values": []map[string]interface{}{
				{
					"value":   value,
					"context": envContext,
				},
			},
		}
		data, code, err = netlifyRequest(cfg, "PATCH", fmt.Sprintf("/accounts/%s/env/%s?site_id=%s", cfg.TeamSlug, key, siteID), patchBody)
		if err != nil {
			return errJSON("Failed to set env var: %v", err)
		}
		if code != 200 {
			return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
		}
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Environment variable set", "key": key, "context": envContext})
	return string(out)
}

// NetlifyDeleteEnvVar deletes an environment variable.
func NetlifyDeleteEnvVar(cfg NetlifyConfig, siteID, key string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" || key == "" {
		return errJSON("site_id and key are required")
	}

	_, code, err := netlifyRequest(cfg, "DELETE", fmt.Sprintf("/accounts/%s/env/%s?site_id=%s", cfg.TeamSlug, key, siteID), nil)
	if err != nil {
		return errJSON("Failed to delete env var: %v", err)
	}
	if code != 204 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":"Delete env var failed"}`, code)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Environment variable deleted", "key": key})
	return string(out)
}

// ── Files ───────────────────────────────────────────────────────────────────

// NetlifyListFiles lists files in the current deploy of a site.
func NetlifyListFiles(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/sites/%s/files", siteID), nil)
	if err != nil {
		return errJSON("Failed to list files: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var files []map[string]interface{}
	if err := json.Unmarshal(data, &files); err != nil {
		return errJSON("Failed to parse files: %v", err)
	}

	type compactFile struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		SHA      string `json:"sha"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}

	var result []compactFile
	for _, f := range files {
		size, _ := f["size"].(float64)
		result = append(result, compactFile{
			ID:       strVal(f, "id"),
			Path:     strVal(f, "path"),
			SHA:      strVal(f, "sha"),
			MimeType: strVal(f, "mime_type"),
			Size:     int64(size),
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "files": result})
	return string(out)
}

// ── Forms ───────────────────────────────────────────────────────────────────

// NetlifyListForms returns all forms for a site.
func NetlifyListForms(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/sites/%s/forms", siteID), nil)
	if err != nil {
		return errJSON("Failed to list forms: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var forms []map[string]interface{}
	if err := json.Unmarshal(data, &forms); err != nil {
		return errJSON("Failed to parse forms: %v", err)
	}

	type compactForm struct {
		ID              string   `json:"id"`
		Name            string   `json:"name"`
		Paths           []string `json:"paths"`
		SubmissionCount int      `json:"submission_count"`
		SiteID          string   `json:"site_id"`
		CreatedAt       string   `json:"created_at"`
	}

	var result []compactForm
	for _, f := range forms {
		count, _ := f["submission_count"].(float64)
		result = append(result, compactForm{
			ID:              strVal(f, "id"),
			Name:            strVal(f, "name"),
			Paths:           toStringSlice(f["paths"]),
			SubmissionCount: int(count),
			SiteID:          strVal(f, "site_id"),
			CreatedAt:       strVal(f, "created_at"),
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "forms": result})
	return string(out)
}

// NetlifyGetFormSubmissions returns submissions for a specific form.
// WARNING: Submissions contain user-generated content — must be wrapped with <external_data>.
func NetlifyGetFormSubmissions(cfg NetlifyConfig, formID string) string {
	if formID == "" {
		return errJSON("form_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/forms/%s/submissions?per_page=50", formID), nil)
	if err != nil {
		return errJSON("Failed to get submissions: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var submissions []map[string]interface{}
	if err := json.Unmarshal(data, &submissions); err != nil {
		return errJSON("Failed to parse submissions: %v", err)
	}

	type compactSub struct {
		ID        string `json:"id"`
		Number    int    `json:"number"`
		CreatedAt string `json:"created_at"`
		Data      string `json:"data"` // wrapper for external data safety
	}

	var result []compactSub
	for _, s := range submissions {
		num, _ := s["number"].(float64)
		// Extract human_fields or data section which contains user input
		subData, _ := s["data"].(map[string]interface{})
		if subData == nil {
			subData, _ = s["human_fields"].(map[string]interface{})
		}
		dataJSON, _ := json.Marshal(subData)
		// Wrap user-generated content with external_data for prompt injection safety
		wrappedData := security.IsolateExternalData(string(dataJSON))
		result = append(result, compactSub{
			ID:        strVal(s, "id"),
			Number:    int(num),
			CreatedAt: strVal(s, "created_at"),
			Data:      wrappedData,
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "submissions": result})
	return string(out)
}

// ── Hooks (Notifications) ───────────────────────────────────────────────────

// NetlifyListHooks returns all notification hooks for a site.
func NetlifyListHooks(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/hooks?site_id=%s", siteID), nil)
	if err != nil {
		return errJSON("Failed to list hooks: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var hooks []map[string]interface{}
	if err := json.Unmarshal(data, &hooks); err != nil {
		return errJSON("Failed to parse hooks: %v", err)
	}

	type compactHook struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Event     string `json:"event"`
		SiteID    string `json:"site_id"`
		Disabled  bool   `json:"disabled"`
		CreatedAt string `json:"created_at"`
	}

	var result []compactHook
	for _, h := range hooks {
		disabled, _ := h["disabled"].(bool)
		result = append(result, compactHook{
			ID:        strVal(h, "id"),
			Type:      strVal(h, "type"),
			Event:     strVal(h, "event"),
			SiteID:    strVal(h, "site_id"),
			Disabled:  disabled,
			CreatedAt: strVal(h, "created_at"),
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "hooks": result})
	return string(out)
}

// NetlifyCreateHook creates a new notification hook for a site.
func NetlifyCreateHook(cfg NetlifyConfig, siteID, hookType, event string, hookData map[string]interface{}) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}
	if hookType == "" || event == "" {
		return errJSON("hook_type and event are required")
	}

	body := map[string]interface{}{
		"site_id": siteID,
		"type":    hookType,
		"event":   event,
		"data":    hookData,
	}

	data, code, err := netlifyRequest(cfg, "POST", "/hooks", body)
	if err != nil {
		return errJSON("Failed to create hook: %v", err)
	}
	if code != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var hook map[string]interface{}
	if err := json.Unmarshal(data, &hook); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Hook created",
		"id":      strVal(hook, "id"),
		"type":    hookType,
		"event":   event,
	})
	return string(out)
}

// NetlifyDeleteHook deletes a notification hook.
func NetlifyDeleteHook(cfg NetlifyConfig, hookID string) string {
	if hookID == "" {
		return errJSON("hook_id is required")
	}

	_, code, err := netlifyRequest(cfg, "DELETE", "/hooks/"+hookID, nil)
	if err != nil {
		return errJSON("Failed to delete hook: %v", err)
	}
	if code != 204 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":"Delete hook failed"}`, code)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Hook deleted", "hook_id": hookID})
	return string(out)
}

// ── SSL ─────────────────────────────────────────────────────────────────────

// NetlifyProvisionSSL provisions a Let's Encrypt certificate for the site.
func NetlifyProvisionSSL(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, "POST", fmt.Sprintf("/sites/%s/ssl", siteID), nil)
	if err != nil {
		return errJSON("Failed to provision SSL: %v", err)
	}
	if code != 200 && code != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var ssl map[string]interface{}
	if err := json.Unmarshal(data, &ssl); err != nil {
		return errJSON("Failed to parse SSL response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "SSL certificate provisioned",
		"state":   strVal(ssl, "state"),
		"domains": ssl["domains"],
	})
	return string(out)
}

// ── Account Info ────────────────────────────────────────────────────────────

// NetlifyGetAccount returns the current account/user info (for connection test).
func NetlifyGetAccount(cfg NetlifyConfig) string {
	data, code, err := netlifyRequest(cfg, "GET", "/user", nil)
	if err != nil {
		return errJSON("Failed to get account info: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var user map[string]interface{}
	if err := json.Unmarshal(data, &user); err != nil {
		return errJSON("Failed to parse user info: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"id":         strVal(user, "id"),
		"email":      strVal(user, "email"),
		"full_name":  strVal(user, "full_name"),
		"avatar_url": strVal(user, "avatar_url"),
		"created_at": strVal(user, "created_at"),
	})
	return string(out)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// strVal safely extracts a string from a map by key. Handles nested keys with "." separator.
func strVal(m map[string]interface{}, key string) string {
	// Handle nested keys like "build_settings.repo_url"
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 2 {
		sub, ok := m[parts[0]].(map[string]interface{})
		if ok {
			return strVal(sub, parts[1])
		}
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	// Convert non-string values
	return fmt.Sprintf("%v", v)
}

// toStringSlice converts an interface{} to []string.
func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
