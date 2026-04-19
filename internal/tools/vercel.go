package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VercelConfig holds the Vercel API connection parameters.
type VercelConfig struct {
	Token            string // Personal Access Token
	DefaultProjectID string // Default project ID or name for operations
	TeamID           string // Team identifier for scoped API calls
	TeamSlug         string // Team slug for scoped API calls
}

var vercelBaseURL = "https://api.vercel.com"

func vercelDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
	}
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil || len(addrs) == 0 {
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
	}
	var lastErr error
	for _, a := range addrs {
		conn, dialErr := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(a, port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}

var vercelHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		DialContext:         vercelDial,
		TLSHandshakeTimeout: 15 * time.Second,
		ForceAttemptHTTP2:   true,
	},
}

func vercelRequest(cfg VercelConfig, method, endpoint string, body interface{}) ([]byte, int, error) {
	fullURL, err := vercelURLWithScope(cfg, endpoint)
	if err != nil {
		return nil, 0, fmt.Errorf("build request url: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("User-Agent", "AuraGo-Agent/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := vercelHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func vercelURLWithScope(cfg VercelConfig, endpoint string) (string, error) {
	u, err := url.Parse(vercelBaseURL + endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if cfg.TeamID != "" {
		q.Set("teamId", cfg.TeamID)
	}
	if cfg.TeamSlug != "" {
		q.Set("slug", cfg.TeamSlug)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func vercelResolveProjectID(cfg VercelConfig, projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID != "" {
		return projectID
	}
	return strings.TrimSpace(cfg.DefaultProjectID)
}

func vercelFrameworkSlug(framework string) string {
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "next", "nextjs":
		return "nextjs"
	case "vite", "react", "react-vite":
		return "vite"
	case "astro":
		return "astro"
	case "nuxt", "nuxtjs":
		return "nuxtjs"
	case "vue":
		return "vue"
	case "html", "static":
		return "other"
	default:
		return strings.TrimSpace(framework)
	}
}

func vercelEnvTargets(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return []string{"production", "preview", "development"}
	}
	parts := strings.Split(target, ",")
	targets := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			targets = append(targets, part)
		}
	}
	if len(targets) == 0 {
		return []string{"production", "preview", "development"}
	}
	return targets
}

func vercelErrorResponse(code int, data []byte, fallback string) string {
	msg := strings.TrimSpace(string(data))
	if msg == "" {
		msg = fallback
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err == nil {
		if errObj, ok := parsed["error"].(map[string]interface{}); ok {
			if m, ok := errObj["message"].(string); ok && strings.TrimSpace(m) != "" {
				msg = m
			}
			if c, ok := errObj["code"].(string); ok && c != "" {
				msg = c + ": " + msg
			}
		}
		if m, ok := parsed["message"].(string); ok && strings.TrimSpace(m) != "" {
			msg = m
		}
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":    "error",
		"http_code": code,
		"message":   msg,
	})
	return string(out)
}

func compactVercelProject(project map[string]interface{}) map[string]interface{} {
	latestAliases := []string{}
	if latest := project["latestDeployments"]; latest != nil {
		if arr, ok := latest.([]interface{}); ok && len(arr) > 0 {
			if first, ok := arr[0].(map[string]interface{}); ok {
				if aliases, ok := first["alias"].([]interface{}); ok {
					for _, alias := range aliases {
						if s, ok := alias.(string); ok && s != "" {
							latestAliases = append(latestAliases, s)
						}
					}
				}
			}
		}
	}
	return map[string]interface{}{
		"id":              strVal(project, "id"),
		"name":            strVal(project, "name"),
		"framework":       strVal(project, "framework"),
		"root_directory":  strVal(project, "rootDirectory"),
		"output_directory": strVal(project, "outputDirectory"),
		"build_command":   strVal(project, "buildCommand"),
		"dev_command":     strVal(project, "devCommand"),
		"install_command": strVal(project, "installCommand"),
		"created_at":      project["createdAt"],
		"updated_at":      project["updatedAt"],
		"latest_aliases":  latestAliases,
	}
}

func compactVercelDeployment(dep map[string]interface{}) map[string]interface{} {
	aliases := []string{}
	if arr, ok := dep["alias"].([]interface{}); ok {
		for _, alias := range arr {
			if s, ok := alias.(string); ok && s != "" {
				aliases = append(aliases, s)
			}
		}
	}
	return map[string]interface{}{
		"id":          strVal(dep, "uid"),
		"name":        strVal(dep, "name"),
		"url":         strVal(dep, "url"),
		"ready_state": strVal(dep, "readyState"),
		"target":      strVal(dep, "target"),
		"project_id":  strVal(dep, "projectId"),
		"created_at":  dep["createdAt"],
		"aliases":     aliases,
		"error":       strVal(dep, "errorMessage"),
	}
}

// VercelCheckConnection checks whether the Vercel API is reachable and the token is valid.
func VercelCheckConnection(cfg VercelConfig) string {
	result := map[string]interface{}{"api_url": vercelBaseURL}

	addrs, dnsErr := net.LookupHost("api.vercel.com")
	if dnsErr != nil {
		result["dns_ok"] = false
		result["dns_error"] = dnsErr.Error()
		result["status"] = "error"
		result["message"] = "DNS resolution failed for api.vercel.com: " + dnsErr.Error()
		out, _ := json.Marshal(result)
		return string(out)
	}
	result["dns_ok"] = true
	result["resolved_ips"] = addrs

	conn, tcpErr := net.DialTimeout("tcp", "api.vercel.com:443", 10*time.Second)
	if tcpErr != nil {
		result["tcp_ok"] = false
		result["tcp_error"] = tcpErr.Error()
		result["status"] = "error"
		result["message"] = "TCP connection to api.vercel.com:443 failed: " + tcpErr.Error()
		out, _ := json.Marshal(result)
		return string(out)
	}
	conn.Close()
	result["tcp_ok"] = true

	data, code, apiErr := vercelRequest(cfg, http.MethodGet, "/v2/user", nil)
	if apiErr != nil {
		result["api_ok"] = false
		result["api_error"] = apiErr.Error()
		result["status"] = "error"
		result["message"] = "API request failed: " + apiErr.Error()
		out, _ := json.Marshal(result)
		return string(out)
	}
	if code == http.StatusUnauthorized {
		result["api_ok"] = false
		result["api_http_code"] = code
		result["status"] = "error"
		result["message"] = "Authentication failed (HTTP 401) — the vercel_token in the vault is invalid or expired."
		out, _ := json.Marshal(result)
		return string(out)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Vercel API returned an unexpected response")
	}

	var user map[string]interface{}
	_ = json.Unmarshal(data, &user)
	result["api_ok"] = true
	result["status"] = "ok"
	result["message"] = "Vercel API is reachable and token is valid"
	result["user_id"] = strVal(user, "user.id")
	result["username"] = strVal(user, "user.username")
	result["email"] = strVal(user, "user.email")
	result["name"] = strVal(user, "user.name")
	out, _ := json.Marshal(result)
	return string(out)
}

func VercelListProjects(cfg VercelConfig) string {
	data, code, err := vercelRequest(cfg, http.MethodGet, "/v9/projects?limit=100", nil)
	if err != nil {
		return errJSON("Failed to list Vercel projects: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to list Vercel projects")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel projects: %v", err)
	}
	var items []interface{}
	if raw, ok := resp["projects"].([]interface{}); ok {
		items = raw
	}

	projects := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if project, ok := item.(map[string]interface{}); ok {
			projects = append(projects, compactVercelProject(project))
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"count":    len(projects),
		"projects": projects,
	})
	return string(out)
}

func VercelGetProject(cfg VercelConfig, projectID string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	data, code, err := vercelRequest(cfg, http.MethodGet, "/v9/projects/"+url.PathEscape(projectID), nil)
	if err != nil {
		return errJSON("Failed to get Vercel project: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to get Vercel project")
	}

	var project map[string]interface{}
	if err := json.Unmarshal(data, &project); err != nil {
		return errJSON("Failed to parse Vercel project: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"project": compactVercelProject(project),
	})
	return string(out)
}

func VercelCreateProject(cfg VercelConfig, name, framework, rootDirectory, outputDirectory string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return errJSON("project_name is required")
	}
	body := map[string]interface{}{"name": name}
	if fw := vercelFrameworkSlug(framework); fw != "" && fw != "other" {
		body["framework"] = fw
	}
	if rootDirectory = strings.TrimSpace(rootDirectory); rootDirectory != "" {
		body["rootDirectory"] = rootDirectory
	}
	if outputDirectory = strings.TrimSpace(outputDirectory); outputDirectory != "" {
		body["outputDirectory"] = outputDirectory
	}

	data, code, err := vercelRequest(cfg, http.MethodPost, "/v11/projects", body)
	if err != nil {
		return errJSON("Failed to create Vercel project: %v", err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return vercelErrorResponse(code, data, "Failed to create Vercel project")
	}

	var project map[string]interface{}
	if err := json.Unmarshal(data, &project); err != nil {
		return errJSON("Failed to parse Vercel project: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel project created",
		"project": compactVercelProject(project),
	})
	return string(out)
}

func VercelUpdateProject(cfg VercelConfig, projectID, name, framework, rootDirectory, outputDirectory string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	body := map[string]interface{}{}
	if name = strings.TrimSpace(name); name != "" {
		body["name"] = name
	}
	if fw := vercelFrameworkSlug(framework); fw != "" {
		body["framework"] = fw
	}
	if rootDirectory = strings.TrimSpace(rootDirectory); rootDirectory != "" {
		body["rootDirectory"] = rootDirectory
	}
	if outputDirectory = strings.TrimSpace(outputDirectory); outputDirectory != "" {
		body["outputDirectory"] = outputDirectory
	}
	if len(body) == 0 {
		return errJSON("No project fields provided to update")
	}

	data, code, err := vercelRequest(cfg, http.MethodPatch, "/v9/projects/"+url.PathEscape(projectID), body)
	if err != nil {
		return errJSON("Failed to update Vercel project: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to update Vercel project")
	}

	var project map[string]interface{}
	if err := json.Unmarshal(data, &project); err != nil {
		return errJSON("Failed to parse updated Vercel project: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel project updated",
		"project": compactVercelProject(project),
	})
	return string(out)
}

func VercelListDeployments(cfg VercelConfig, projectID string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	endpoint := "/v6/deployments?limit=20"
	if projectID != "" {
		endpoint += "&projectId=" + url.QueryEscape(projectID)
	}
	data, code, err := vercelRequest(cfg, http.MethodGet, endpoint, nil)
	if err != nil {
		return errJSON("Failed to list Vercel deployments: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to list Vercel deployments")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel deployments: %v", err)
	}
	var items []interface{}
	if raw, ok := resp["deployments"].([]interface{}); ok {
		items = raw
	}

	deployments := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if dep, ok := item.(map[string]interface{}); ok {
			deployments = append(deployments, compactVercelDeployment(dep))
		}
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"count":       len(deployments),
		"deployments": deployments,
	})
	return string(out)
}

func VercelGetDeployment(cfg VercelConfig, deploymentID string) string {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return errJSON("deployment_id is required")
	}
	data, code, err := vercelRequest(cfg, http.MethodGet, "/v13/deployments/"+url.PathEscape(deploymentID), nil)
	if err != nil {
		return errJSON("Failed to get Vercel deployment: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to get Vercel deployment")
	}

	var deployment map[string]interface{}
	if err := json.Unmarshal(data, &deployment); err != nil {
		return errJSON("Failed to parse Vercel deployment: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"deployment": compactVercelDeployment(deployment),
	})
	return string(out)
}

func VercelListEnv(cfg VercelConfig, projectID string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	data, code, err := vercelRequest(cfg, http.MethodGet, "/v9/projects/"+url.PathEscape(projectID)+"/env", nil)
	if err != nil {
		return errJSON("Failed to list Vercel environment variables: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to list Vercel environment variables")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel environment variables: %v", err)
	}
	var items []interface{}
	switch raw := resp["envs"].(type) {
	case []interface{}:
		items = raw
	default:
		if rawArr, ok := resp["env"].([]interface{}); ok {
			items = rawArr
		}
	}

	envs := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if env, ok := item.(map[string]interface{}); ok {
			envs = append(envs, map[string]interface{}{
				"id":         strVal(env, "id"),
				"key":        strVal(env, "key"),
				"type":       strVal(env, "type"),
				"target":     env["target"],
				"git_branch": strVal(env, "gitBranch"),
				"created_at": env["createdAt"],
				"updated_at": env["updatedAt"],
			})
		}
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(envs),
		"envs":   envs,
	})
	return string(out)
}

func VercelSetEnv(cfg VercelConfig, projectID, key, value, target string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errJSON("env_key is required")
	}
	body := map[string]interface{}{
		"key":    key,
		"value":  value,
		"type":   "plain",
		"target": vercelEnvTargets(target),
	}
	data, code, err := vercelRequest(cfg, http.MethodPost, "/v10/projects/"+url.PathEscape(projectID)+"/env?upsert=true", body)
	if err != nil {
		return errJSON("Failed to set Vercel environment variable: %v", err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return vercelErrorResponse(code, data, "Failed to set Vercel environment variable")
	}

	var env map[string]interface{}
	if err := json.Unmarshal(data, &env); err != nil {
		return errJSON("Failed to parse Vercel environment variable response: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel environment variable saved",
		"env": map[string]interface{}{
			"id":         strVal(env, "id"),
			"key":        strVal(env, "key"),
			"type":       strVal(env, "type"),
			"target":     env["target"],
			"created_at": env["createdAt"],
			"updated_at": env["updatedAt"],
		},
	})
	return string(out)
}

func VercelDeleteEnv(cfg VercelConfig, projectID, key string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errJSON("env_key is required")
	}

	listRaw := VercelListEnv(cfg, projectID)
	var listResp map[string]interface{}
	if err := json.Unmarshal([]byte(listRaw), &listResp); err != nil || listResp["status"] != "ok" {
		return errJSON("Failed to resolve Vercel environment variable %q before deletion", key)
	}
	envs, _ := listResp["envs"].([]interface{})
	envID := ""
	for _, item := range envs {
		if env, ok := item.(map[string]interface{}); ok {
			if strVal(env, "key") == key || strVal(env, "id") == key {
				envID = strVal(env, "id")
				break
			}
		}
	}
	if envID == "" {
		return errJSON("Environment variable %q not found on project %q", key, projectID)
	}

	data, code, err := vercelRequest(cfg, http.MethodDelete, "/v9/projects/"+url.PathEscape(projectID)+"/env/"+url.PathEscape(envID), nil)
	if err != nil {
		return errJSON("Failed to delete Vercel environment variable: %v", err)
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return vercelErrorResponse(code, data, "Failed to delete Vercel environment variable")
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel environment variable deleted",
		"key":     key,
		"id":      envID,
	})
	return string(out)
}

func VercelListDomains(cfg VercelConfig, projectID string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	data, code, err := vercelRequest(cfg, http.MethodGet, "/v9/projects/"+url.PathEscape(projectID)+"/domains?limit=100", nil)
	if err != nil {
		return errJSON("Failed to list Vercel project domains: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to list Vercel project domains")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel project domains: %v", err)
	}
	var items []interface{}
	if raw, ok := resp["domains"].([]interface{}); ok {
		items = raw
	}
	domains := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if domain, ok := item.(map[string]interface{}); ok {
			domains = append(domains, map[string]interface{}{
				"name":         strVal(domain, "name"),
				"apex_name":    strVal(domain, "apexName"),
				"verified":     domain["verified"],
				"created_at":   domain["createdAt"],
				"updated_at":   domain["updatedAt"],
				"git_branch":   strVal(domain, "gitBranch"),
				"redirect":     strVal(domain, "redirect"),
				"verification": domain["verification"],
			})
		}
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(domains),
		"domains": domains,
	})
	return string(out)
}

func VercelAddDomain(cfg VercelConfig, projectID, domain string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errJSON("domain is required")
	}
	data, code, err := vercelRequest(cfg, http.MethodPost, "/v10/projects/"+url.PathEscape(projectID)+"/domains", map[string]interface{}{"name": domain})
	if err != nil {
		return errJSON("Failed to add Vercel project domain: %v", err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return vercelErrorResponse(code, data, "Failed to add Vercel project domain")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel domain response: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel project domain added",
		"domain": map[string]interface{}{
			"name":         strVal(resp, "name"),
			"apex_name":    strVal(resp, "apexName"),
			"verified":     resp["verified"],
			"verification": resp["verification"],
		},
	})
	return string(out)
}

func VercelVerifyDomain(cfg VercelConfig, projectID, domain string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errJSON("domain is required")
	}
	data, code, err := vercelRequest(cfg, http.MethodPost, "/v9/projects/"+url.PathEscape(projectID)+"/domains/"+url.PathEscape(domain)+"/verify", nil)
	if err != nil {
		return errJSON("Failed to verify Vercel project domain: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to verify Vercel project domain")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel domain verification response: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel project domain verification checked",
		"domain": map[string]interface{}{
			"name":         strVal(resp, "name"),
			"verified":     resp["verified"],
			"verification": resp["verification"],
		},
	})
	return string(out)
}

func VercelListAliases(cfg VercelConfig, projectID, deploymentID string) string {
	endpoint := "/v2/aliases?limit=100"
	if deploymentID = strings.TrimSpace(deploymentID); deploymentID != "" {
		endpoint = "/v2/deployments/" + url.PathEscape(deploymentID) + "/aliases"
	} else if projectID = vercelResolveProjectID(cfg, projectID); projectID != "" {
		endpoint += "&projectId=" + url.QueryEscape(projectID)
	}

	data, code, err := vercelRequest(cfg, http.MethodGet, endpoint, nil)
	if err != nil {
		return errJSON("Failed to list Vercel aliases: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to list Vercel aliases")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel aliases: %v", err)
	}
	var items []interface{}
	if raw, ok := resp["aliases"].([]interface{}); ok {
		items = raw
	}
	aliases := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if alias, ok := item.(map[string]interface{}); ok {
			aliases = append(aliases, map[string]interface{}{
				"uid":        strVal(alias, "uid"),
				"alias":      strVal(alias, "alias"),
				"created_at": alias["created"],
				"deployment": strVal(alias, "deploymentId"),
				"project_id": strVal(alias, "projectId"),
			})
		}
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(aliases),
		"aliases": aliases,
	})
	return string(out)
}

func VercelAssignAlias(cfg VercelConfig, deploymentID, alias string) string {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return errJSON("deployment_id is required")
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return errJSON("alias is required")
	}
	data, code, err := vercelRequest(cfg, http.MethodPost, "/v2/deployments/"+url.PathEscape(deploymentID)+"/aliases", map[string]interface{}{
		"alias":    alias,
		"redirect": nil,
	})
	if err != nil {
		return errJSON("Failed to assign Vercel alias: %v", err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return vercelErrorResponse(code, data, "Failed to assign Vercel alias")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel alias response: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel alias assigned",
		"alias": map[string]interface{}{
			"uid":        strVal(resp, "uid"),
			"alias":      strVal(resp, "alias"),
			"created_at": resp["created"],
		},
	})
	return string(out)
}

func VercelDeleteProject(cfg VercelConfig, projectID string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	data, code, err := vercelRequest(cfg, http.MethodDelete, "/v9/projects/"+url.PathEscape(projectID), nil)
	if err != nil {
		return errJSON("Failed to delete Vercel project: %v", err)
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return vercelErrorResponse(code, data, "Failed to delete Vercel project")
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "Vercel project deleted",
		"project_id": projectID,
	})
	return string(out)
}

func VercelRollback(cfg VercelConfig, projectID, deploymentID string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return errJSON("deployment_id is required for rollback")
	}
	data, code, err := vercelRequest(cfg, http.MethodPost, "/v9/projects/"+url.PathEscape(projectID)+"/rollback/"+url.PathEscape(deploymentID), map[string]interface{}{})
	if err != nil {
		return errJSON("Failed to rollback Vercel deployment: %v", err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return vercelErrorResponse(code, data, "Failed to rollback Vercel deployment")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel rollback response: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":       "ok",
		"message":      "Rolled back to deployment " + deploymentID,
		"deployment_id": deploymentID,
		"project_id":   projectID,
		"url":          strVal(resp, "url"),
	})
	return string(out)
}

func VercelCancelDeploy(cfg VercelConfig, deploymentID string) string {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return errJSON("deployment_id is required")
	}
	data, code, err := vercelRequest(cfg, http.MethodPatch, "/v12/deployments/"+url.PathEscape(deploymentID)+"/cancel", nil)
	if err != nil {
		return errJSON("Failed to cancel Vercel deployment: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to cancel Vercel deployment")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel cancel deploy response: %v", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":       "ok",
		"message":      "Deployment cancelled",
		"deployment_id": deploymentID,
		"state":        strVal(resp, "state"),
	})
	return string(out)
}

func VercelGetEnv(cfg VercelConfig, projectID, key string) string {
	projectID = vercelResolveProjectID(cfg, projectID)
	if projectID == "" {
		return errJSON("project_id is required (or set default_project_id in config)")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errJSON("env_key is required")
	}

	data, code, err := vercelRequest(cfg, http.MethodGet, "/v9/projects/"+url.PathEscape(projectID)+"/env/"+url.PathEscape(key), nil)
	if err != nil {
		return errJSON("Failed to get Vercel environment variable: %v", err)
	}
	if code != http.StatusOK {
		return vercelErrorResponse(code, data, "Failed to get Vercel environment variable")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errJSON("Failed to parse Vercel environment variable: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"env": map[string]interface{}{
			"id":         strVal(resp, "id"),
			"key":        strVal(resp, "key"),
			"type":       strVal(resp, "type"),
			"target":     resp["target"],
			"git_branch": strVal(resp, "gitBranch"),
			"created_at": resp["createdAt"],
			"updated_at": resp["updatedAt"],
		},
	})
	return string(out)
}
