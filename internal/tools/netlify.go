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
	"time"
)

// NetlifyConfig holds the Netlify API connection parameters.
type NetlifyConfig struct {
	Token         string // Personal Access Token
	DefaultSiteID string // Default site ID for operations
	TeamSlug      string // Netlify team/account slug
}

var netlifyBaseURL = "https://api.netlify.com/api/v1"

// netlifyDial tries all resolved IP addresses for the host in order.
// Go's default dialer stops at the first failure; this mirrors curl's behavior
// where one IPv4 address may be unreachable but another (or an IPv6 address) works.
func netlifyDial(ctx context.Context, network, addr string) (net.Conn, error) {
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

var netlifyHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		DialContext:         netlifyDial,
		TLSHandshakeTimeout: 15 * time.Second,
		ForceAttemptHTTP2:   true,
	},
}

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

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
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

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
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

// netlifyResolveNameToID looks up a site by name and returns its UUID.
// If the name is already a UUID or the lookup fails, it returns "".
func netlifyResolveNameToID(cfg NetlifyConfig, name string) string {
	if looksLikeUUID(name) {
		// Already a UUID — no lookup needed.
		return name
	}
	endpoint := "/sites?per_page=100"
	if cfg.TeamSlug != "" {
		endpoint = fmt.Sprintf("/%s/sites?per_page=100", cfg.TeamSlug)
	}
	data, code, err := netlifyRequest(cfg, "GET", endpoint, nil)
	if err != nil || code != 200 {
		return ""
	}
	var sites []map[string]interface{}
	if err := json.Unmarshal(data, &sites); err != nil {
		return ""
	}
	for _, s := range sites {
		if strVal(s, "name") == name {
			return strVal(s, "id")
		}
	}
	return ""
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
	if code != 200 && code != 204 {
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
// NetlifyTestConnection checks whether the Netlify API is reachable and the token is valid.
// Returns a diagnostic object with dns_ok, tcp_ok, api_ok, resolved_ips, and account info.
func NetlifyTestConnection(cfg NetlifyConfig) string {
	result := map[string]interface{}{"api_url": netlifyBaseURL}

	// 1. DNS resolution
	addrs, dnsErr := net.LookupHost("api.netlify.com")
	if dnsErr != nil {
		result["dns_ok"] = false
		result["dns_error"] = dnsErr.Error()
		result["status"] = "error"
		result["message"] = "DNS resolution failed for api.netlify.com: " + dnsErr.Error() +
			" — Check DNS settings or whether the hostname is blocked."
		out, _ := json.Marshal(result)
		return string(out)
	}
	result["dns_ok"] = true
	result["resolved_ips"] = addrs

	// 2. TCP connection
	conn, tcpErr := net.DialTimeout("tcp", "api.netlify.com:443", 10*time.Second)
	if tcpErr != nil {
		result["tcp_ok"] = false
		result["tcp_error"] = tcpErr.Error()
		result["status"] = "error"
		result["message"] = fmt.Sprintf(
			"TCP connection to api.netlify.com:443 failed: %v — "+
				"The Netlify API is blocked by a firewall or network policy on this host. "+
				"Check iptables/nftables rules, or ask your provider to allow outbound HTTPS "+
				"to %v.",
			tcpErr, addrs)
		out, _ := json.Marshal(result)
		return string(out)
	}
	conn.Close()
	result["tcp_ok"] = true

	// 3. Authenticated API call
	data, code, apiErr := netlifyRequest(cfg, "GET", "/user", nil)
	if apiErr != nil {
		result["api_ok"] = false
		result["api_error"] = apiErr.Error()
		result["status"] = "error"
		result["message"] = "API request failed: " + apiErr.Error()
		out, _ := json.Marshal(result)
		return string(out)
	}
	if code == 401 {
		result["api_ok"] = false
		result["api_http_code"] = code
		result["status"] = "error"
		result["message"] = "Authentication failed (HTTP 401) — the netlify_token in the vault is invalid or expired."
		out, _ := json.Marshal(result)
		return string(out)
	}
	if code != 200 {
		result["api_ok"] = false
		result["api_http_code"] = code
		result["status"] = "error"
		result["message"] = fmt.Sprintf("API returned unexpected HTTP %d", code)
		out, _ := json.Marshal(result)
		return string(out)
	}

	var user map[string]interface{}
	_ = json.Unmarshal(data, &user)
	result["api_ok"] = true
	result["status"] = "ok"
	result["message"] = "Netlify API is reachable and token is valid"
	if email, ok := user["email"].(string); ok {
		result["account_email"] = email
	}
	if slug, ok := user["slug"].(string); ok {
		result["account_slug"] = slug
	}
	out, _ := json.Marshal(result)
	return string(out)
}

func NetlifyDeployZip(cfg NetlifyConfig, siteID, title string, draft bool, zipData []byte) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required. Use 'netlify check_connection' to verify API access, then 'netlify list_sites' to find your site ID, or set default_site_id in the Netlify config.")
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
		endpoint += sep + "title=" + url.QueryEscape(title)
	}

	data, code, err := netlifyRequestRaw(cfg, "POST", endpoint, "application/zip", bytes.NewReader(zipData))
	if err != nil {
		return errJSON("Failed to deploy: %v", err)
	}
	// Netlify returns 200 for re-deploys and 201 Created for new deploys.
	if code != 200 && code != 201 {
		if code == 404 {
			return fmt.Sprintf(`{"status":"error","http_code":404,"message":"Site %q not found on Netlify. Either the site_id is wrong or the site was deleted. Use 'netlify list_sites' to see existing sites, or use 'homepage deploy_netlify' which auto-creates the site if it does not exist."}`, siteID)
		}
		// Try to extract a helpful message from the Netlify error body
		errMsg := string(data)
		var errBody map[string]interface{}
		if jsonErr := json.Unmarshal(data, &errBody); jsonErr == nil {
			if msg, ok := errBody["message"]; ok {
				errMsg = fmt.Sprintf("%v", msg)
			} else if msg, ok := errBody["error"]; ok {
				errMsg = fmt.Sprintf("%v", msg)
			}
		}
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, errMsg)
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
