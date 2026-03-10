package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NetlifyConfig holds the Netlify API connection parameters.
type NetlifyConfig struct {
	Token         string // Personal Access Token
	DefaultSiteID string // Default site ID for operations
	TeamSlug      string // Netlify team/account slug
}

const netlifyBaseURL = "https://api.netlify.com/api/v1"

var netlifyHTTPClient = &http.Client{Timeout: 60 * time.Second}

// ── Internal helpers ────────────────────────────────────────────────────────

// netlifyRequest executes an authenticated HTTP request against the Netlify API.
func netlifyRequest(cfg NetlifyConfig, method, endpoint string, body interface{}) ([]byte, int, error) {
	url := netlifyBaseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("User-Agent", "AuraGo-Agent/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := netlifyHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// netlifyRequestRaw executes an authenticated HTTP request with a raw byte body (for ZIP deploys).
func netlifyRequestRaw(cfg NetlifyConfig, method, endpoint, contentType string, rawBody io.Reader) ([]byte, int, error) {
	url := netlifyBaseURL + endpoint

	req, err := http.NewRequest(method, url, rawBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("User-Agent", "AuraGo-Agent/1.0")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := netlifyHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// netlifyResolveSiteID returns the given siteID if non-empty, else the default from config.
func netlifyResolveSiteID(cfg NetlifyConfig, siteID string) string {
	if siteID != "" {
		return siteID
	}
	return cfg.DefaultSiteID
}

// ── Sites ───────────────────────────────────────────────────────────────────

// NetlifyListSites returns all sites for the account/team.
func NetlifyListSites(cfg NetlifyConfig) string {
	endpoint := "/sites?per_page=100"
	if cfg.TeamSlug != "" {
		endpoint = fmt.Sprintf("/%s/sites?per_page=100", cfg.TeamSlug)
	}
	data, code, err := netlifyRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return errJSON("Failed to list sites: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	// Parse and return compact site list
	var sites []map[string]interface{}
	if err := json.Unmarshal(data, &sites); err != nil {
		return errJSON("Failed to parse sites: %v", err)
	}

	type compactSite struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		URL           string `json:"url"`
		SslURL        string `json:"ssl_url"`
		AdminURL      string `json:"admin_url"`
		State         string `json:"state"`
		CustomDomain  string `json:"custom_domain"`
		DefaultDomain string `json:"default_domain"`
		CreatedAt     string `json:"created_at"`
		UpdatedAt     string `json:"updated_at"`
	}

	var result []compactSite
	for _, s := range sites {
		result = append(result, compactSite{
			ID:            strVal(s, "id"),
			Name:          strVal(s, "name"),
			URL:           strVal(s, "url"),
			SslURL:        strVal(s, "ssl_url"),
			AdminURL:      strVal(s, "admin_url"),
			State:         strVal(s, "state"),
			CustomDomain:  strVal(s, "custom_domain"),
			DefaultDomain: strVal(s, "default_domain"),
			CreatedAt:     strVal(s, "created_at"),
			UpdatedAt:     strVal(s, "updated_at"),
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "sites": result})
	return string(out)
}

// NetlifyGetSite returns detailed info about a specific site.
func NetlifyGetSite(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required (or set default_site_id in config)")
	}
	data, code, err := netlifyRequest(cfg, "GET", "/sites/"+siteID, nil)
	if err != nil {
		return errJSON("Failed to get site: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var site map[string]interface{}
	if err := json.Unmarshal(data, &site); err != nil {
		return errJSON("Failed to parse site: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":         "ok",
		"id":             strVal(site, "id"),
		"name":           strVal(site, "name"),
		"url":            strVal(site, "url"),
		"ssl_url":        strVal(site, "ssl_url"),
		"admin_url":      strVal(site, "admin_url"),
		"state":          strVal(site, "state"),
		"custom_domain":  strVal(site, "custom_domain"),
		"default_domain": strVal(site, "default_domain"),
		"repo_url":       strVal(site, "build_settings.repo_url"),
		"created_at":     strVal(site, "created_at"),
		"updated_at":     strVal(site, "updated_at"),
		"deploy_url":     strVal(site, "deploy_url"),
		"screenshot_url": strVal(site, "screenshot_url"),
	})
	return string(out)
}

// NetlifyCreateSite creates a new Netlify site.
func NetlifyCreateSite(cfg NetlifyConfig, name, customDomain string) string {
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name // subdomain: name.netlify.app
	}
	if customDomain != "" {
		body["custom_domain"] = customDomain
	}

	endpoint := "/sites"
	if cfg.TeamSlug != "" {
		endpoint = fmt.Sprintf("/%s/sites", cfg.TeamSlug)
	}

	data, code, err := netlifyRequest(cfg, "POST", endpoint, body)
	if err != nil {
		return errJSON("Failed to create site: %v", err)
	}
	if code != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var site map[string]interface{}
	if err := json.Unmarshal(data, &site); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":         "ok",
		"message":        "Site created successfully",
		"id":             strVal(site, "id"),
		"name":           strVal(site, "name"),
		"url":            strVal(site, "url"),
		"ssl_url":        strVal(site, "ssl_url"),
		"admin_url":      strVal(site, "admin_url"),
		"default_domain": strVal(site, "default_domain"),
	})
	return string(out)
}

// NetlifyUpdateSite updates an existing site's configuration.
func NetlifyUpdateSite(cfg NetlifyConfig, siteID, name, customDomain string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if customDomain != "" {
		body["custom_domain"] = customDomain
	}

	data, code, err := netlifyRequest(cfg, "PATCH", "/sites/"+siteID, body)
	if err != nil {
		return errJSON("Failed to update site: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var site map[string]interface{}
	if err := json.Unmarshal(data, &site); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Site updated",
		"id":      strVal(site, "id"),
		"name":    strVal(site, "name"),
		"url":     strVal(site, "url"),
	})
	return string(out)
}

// NetlifyDeleteSite permanently deletes a site.
func NetlifyDeleteSite(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	_, code, err := netlifyRequest(cfg, "DELETE", "/sites/"+siteID, nil)
	if err != nil {
		return errJSON("Failed to delete site: %v", err)
	}
	if code != 204 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":"Delete failed"}`, code)
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": "Site deleted", "site_id": siteID})
	return string(out)
}

// ── Deploys ─────────────────────────────────────────────────────────────────

// NetlifyListDeploys returns recent deploys for a site.
func NetlifyListDeploys(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", fmt.Sprintf("/sites/%s/deploys?per_page=20", siteID), nil)
	if err != nil {
		return errJSON("Failed to list deploys: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var deploys []map[string]interface{}
	if err := json.Unmarshal(data, &deploys); err != nil {
		return errJSON("Failed to parse deploys: %v", err)
	}

	type compactDeploy struct {
		ID        string `json:"id"`
		State     string `json:"state"`
		Name      string `json:"name"`
		URL       string `json:"url"`
		DeployURL string `json:"deploy_url"`
		Branch    string `json:"branch"`
		Title     string `json:"title"`
		Context   string `json:"context"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		ErrorMsg  string `json:"error_message,omitempty"`
	}

	var result []compactDeploy
	for _, d := range deploys {
		result = append(result, compactDeploy{
			ID:        strVal(d, "id"),
			State:     strVal(d, "state"),
			Name:      strVal(d, "name"),
			URL:       strVal(d, "url"),
			DeployURL: strVal(d, "deploy_url"),
			Branch:    strVal(d, "branch"),
			Title:     strVal(d, "title"),
			Context:   strVal(d, "context"),
			CreatedAt: strVal(d, "created_at"),
			UpdatedAt: strVal(d, "updated_at"),
			ErrorMsg:  strVal(d, "error_message"),
		})
	}

	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(result), "deploys": result})
	return string(out)
}

// NetlifyGetDeploy returns details about a specific deploy.
func NetlifyGetDeploy(cfg NetlifyConfig, deployID string) string {
	if deployID == "" {
		return errJSON("deploy_id is required")
	}

	data, code, err := netlifyRequest(cfg, "GET", "/deploys/"+deployID, nil)
	if err != nil {
		return errJSON("Failed to get deploy: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var deploy map[string]interface{}
	if err := json.Unmarshal(data, &deploy); err != nil {
		return errJSON("Failed to parse deploy: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"id":            strVal(deploy, "id"),
		"state":         strVal(deploy, "state"),
		"url":           strVal(deploy, "url"),
		"deploy_url":    strVal(deploy, "deploy_url"),
		"branch":        strVal(deploy, "branch"),
		"title":         strVal(deploy, "title"),
		"context":       strVal(deploy, "context"),
		"created_at":    strVal(deploy, "created_at"),
		"error_message": strVal(deploy, "error_message"),
	})
	return string(out)
}

// NetlifyDeployZip creates a new deploy by uploading a ZIP archive.
// The zipData should be the raw ZIP file bytes.
func NetlifyDeployZip(cfg NetlifyConfig, siteID, title string, draft bool, zipData []byte) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}
	if len(zipData) == 0 {
		return errJSON("zip data is empty")
	}

	endpoint := fmt.Sprintf("/sites/%s/deploys", siteID)
	if draft {
		endpoint += "?draft=true"
	}
	if title != "" {
		sep := "?"
		if draft {
			sep = "&"
		}
		endpoint += sep + "title=" + title
	}

	data, code, err := netlifyRequestRaw(cfg, "POST", endpoint, "application/zip", bytes.NewReader(zipData))
	if err != nil {
		return errJSON("Failed to deploy: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var deploy map[string]interface{}
	if err := json.Unmarshal(data, &deploy); err != nil {
		return errJSON("Failed to parse deploy response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"message":    "Deploy initiated",
		"deploy_id":  strVal(deploy, "id"),
		"state":      strVal(deploy, "state"),
		"deploy_url": strVal(deploy, "deploy_url"),
		"url":        strVal(deploy, "url"),
	})
	return string(out)
}

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
		wrappedData := "<external_data>" + string(dataJSON) + "</external_data>"
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
