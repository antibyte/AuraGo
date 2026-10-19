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
	"strconv"
	"strings"
	"time"

	"aurago/internal/httporigin"
)

// NetlifyConfig holds the Netlify API connection parameters.
type NetlifyConfig struct {
	Context             context.Context `json:"-"` // Request owner; nil retains the standalone API lifetime.
	Token               string          // Personal Access Token
	DefaultSiteID       string          // Default site ID for operations
	TeamSlug            string          // Netlify team/account slug
	ReadOnly            bool
	AllowDeploy         bool
	AllowSiteManagement bool
	AllowEnvManagement  bool
}

var netlifyBaseURL = "https://api.netlify.com/api/v1"

var (
	netlifyDeployPollAttempts = 30
	netlifyDeployPollInterval = 2 * time.Second
)

const (
	// netlifyRequestTimeout bounds one API call including the body read.
	netlifyRequestTimeout = 60 * time.Second
	// netlifyUploadTimeout bounds a ZIP deploy upload. Netlify terminates API
	// requests after 30 s, so a longer budget only protects slow uplinks before
	// the request body has been accepted.
	netlifyUploadTimeout = 10 * time.Minute
	// netlifyListMaxPages caps Link-header pagination for collections.
	netlifyListMaxPages = 10
	// netlifyRetryMaxAttempts applies to idempotent requests only.
	netlifyRetryMaxAttempts = 3
	netlifyRetryBackoff     = 500 * time.Millisecond
	netlifyRetryAfterMax    = 30 * time.Second
	// netlifyErrorTextMaxRunes bounds provider text inside an error envelope.
	netlifyErrorTextMaxRunes = 300
	netlifyUserAgent         = "AuraGo-Agent/1.0"
)

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

// netlifyTransport is shared by the API and upload clients so every call keeps
// the multi-address dial behaviour.
var netlifyTransport = &http.Transport{
	DialContext:         netlifyDial,
	TLSHandshakeTimeout: 15 * time.Second,
	ForceAttemptHTTP2:   true,
}

// netlifyHTTPClient sends only Netlify API traffic. httporigin binds every
// redirect to the API origin before the bearer token can reach another host;
// the per-call deadline below replaces a client-wide timeout so an upload is
// not cut off by the API budget.
var netlifyHTTPClient = newNetlifyHTTPClient()

func newNetlifyHTTPClient() *http.Client {
	client := httporigin.NewClient(0)
	client.Transport = netlifyTransport
	return client
}

// netlifyURL builds an absolute request URL. Callers pass endpoint paths; the
// pagination helper passes the provider's absolute Link target.
func netlifyURL(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return netlifyBaseURL + endpoint
}

// netlifyResponse is one Netlify answer.
type netlifyResponse struct {
	data    []byte
	code    int
	header  http.Header
	bodyErr error
}

// netlifyDo executes one request under its own deadline.
func netlifyDo(cfg NetlifyConfig, method, endpoint, contentType string, body io.Reader, timeout time.Duration) netlifyResponse {
	ctx, cancel := context.WithTimeout(nativeHTTPContext(cfg.Context), timeout)
	defer cancel()

	req, err := newToolHTTPRequest(ctx, method, netlifyURL(endpoint), body)
	if err != nil {
		return netlifyResponse{bodyErr: fmt.Errorf("create request: %w", err)}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("User-Agent", netlifyUserAgent)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := netlifyHTTPClient.Do(req)
	if err != nil {
		return netlifyResponse{bodyErr: fmt.Errorf("execute request: %w", err)}
	}
	defer resp.Body.Close()

	data, readErr := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if readErr != nil {
		return netlifyResponse{code: resp.StatusCode, header: resp.Header, bodyErr: fmt.Errorf("read response: %w", readErr)}
	}
	return netlifyResponse{data: data, code: resp.StatusCode, header: resp.Header}
}

// netlifyRequestFull executes one API call. Idempotent requests retry a bounded
// number of times on a rate limit or a transient gateway error; mutations are
// sent exactly once.
func netlifyRequestFull(cfg NetlifyConfig, method, endpoint string, body interface{}) netlifyResponse {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return netlifyResponse{bodyErr: fmt.Errorf("marshal body: %w", err)}
		}
		payload = encoded
	}
	idempotent := method == http.MethodGet || method == http.MethodHead
	var last netlifyResponse
	for attempt := 1; ; attempt++ {
		var reader io.Reader
		contentType := ""
		if payload != nil {
			reader = bytes.NewReader(payload)
			contentType = "application/json"
		}
		last = netlifyDo(cfg, method, endpoint, contentType, reader, netlifyRequestTimeout)
		if last.bodyErr != nil {
			return last
		}
		if !idempotent || attempt >= netlifyRetryMaxAttempts || !netlifyRetryableStatus(last.code) {
			return last
		}
		if err := waitToolHTTP(cfg.Context, netlifyRetryDelay(last.header, attempt)); err != nil {
			last.bodyErr = err
			return last
		}
	}
}

// netlifyRequest executes an authenticated HTTP request against the Netlify API.
func netlifyRequest(cfg NetlifyConfig, method, endpoint string, body interface{}) ([]byte, int, error) {
	res := netlifyRequestFull(cfg, method, endpoint, body)
	return res.data, res.code, res.bodyErr
}

// netlifyRequestRaw executes an authenticated HTTP request with a raw byte body (for ZIP deploys).
func netlifyRequestRaw(cfg NetlifyConfig, method, endpoint, contentType string, rawBody io.Reader) ([]byte, int, error) {
	res := netlifyDo(cfg, method, endpoint, contentType, rawBody, netlifyUploadTimeout)
	return res.data, res.code, res.bodyErr
}

// netlifyRetryableStatus reports the answers worth repeating for an idempotent call.
func netlifyRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// netlifyRetryDelay honours Retry-After and falls back to a bounded backoff.
func netlifyRetryDelay(header http.Header, attempt int) time.Duration {
	if header != nil {
		if value := strings.TrimSpace(header.Get("Retry-After")); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
				delay := time.Duration(seconds) * time.Second
				if delay > netlifyRetryAfterMax {
					delay = netlifyRetryAfterMax
				}
				return delay
			}
			if when, err := http.ParseTime(value); err == nil {
				if delay := time.Until(when); delay > 0 {
					if delay > netlifyRetryAfterMax {
						delay = netlifyRetryAfterMax
					}
					return delay
				}
				return 0
			}
		}
	}
	return time.Duration(attempt) * netlifyRetryBackoff
}

// netlifyProviderErrorText returns a bounded, valid-UTF-8 provider message.
// Raw bodies never reach the agent: they can carry HTML error pages of any size.
func netlifyProviderErrorText(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text != "" {
		var payload map[string]interface{}
		if json.Unmarshal(body, &payload) == nil {
			if parsed := firstNonEmptyString(strVal(payload, "message"), strVal(payload, "error")); parsed != "" {
				text = parsed
			} else if entries, ok := payload["errors"].([]interface{}); ok && len(entries) > 0 {
				switch first := entries[0].(type) {
				case map[string]interface{}:
					text = firstNonEmptyString(strVal(first, "message"), strVal(first, "title"))
				case string:
					text = first
				}
			} else {
				text = ""
			}
		}
	}
	text = strings.ToValidUTF8(text, "?")
	text = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > netlifyErrorTextMaxRunes {
		text = string(runes[:netlifyErrorTextMaxRunes]) + "…"
	}
	return text
}

// netlifyCodedError builds the one error envelope for a Netlify answer.
func netlifyCodedError(code int, format string, args ...interface{}) string {
	out, _ := json.Marshal(map[string]interface{}{
		"status":    "error",
		"http_code": code,
		"message":   fmt.Sprintf(format, args...),
	})
	return string(out)
}

// netlifyHTTPError converts a non-2xx answer into a bounded JSON error.
func netlifyHTTPError(code int, body []byte) string {
	if msg := netlifyProviderErrorText(body); msg != "" {
		return netlifyCodedError(code, "%s", msg)
	}
	return netlifyCodedError(code, "Netlify API returned HTTP %d", code)
}

// netlifyNextPageLink returns the rel="next" target from a Link header.
func netlifyNextPageLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		target := strings.TrimSpace(segments[0])
		if !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
			continue
		}
		for _, attr := range segments[1:] {
			attr = strings.TrimSpace(attr)
			if attr == `rel="next"` || attr == "rel=next" {
				return strings.TrimSuffix(strings.TrimPrefix(target, "<"), ">")
			}
		}
	}
	return ""
}

// netlifySameAPIOrigin reports whether a provider-supplied link stays on the
// configured API origin, so the bearer token is never replayed elsewhere.
func netlifySameAPIOrigin(candidate string) bool {
	base, err := url.Parse(netlifyBaseURL)
	if err != nil {
		return false
	}
	next, err := url.Parse(candidate)
	if err != nil || !next.IsAbs() {
		return false
	}
	return httporigin.SameOrigin(base, next)
}

// netlifyPage is the collected result of a paginated collection request.
type netlifyPage struct {
	items     []json.RawMessage
	code      int
	body      []byte
	truncated bool
	err       error
}

// netlifyGetPages follows Link: rel="next" up to maxPages. A link that leaves
// the API origin ends pagination and marks the result truncated.
func netlifyGetPages(cfg NetlifyConfig, endpoint string, maxPages int) netlifyPage {
	if maxPages <= 0 {
		maxPages = netlifyListMaxPages
	}
	out := netlifyPage{code: http.StatusOK}
	next := endpoint
	for page := 1; ; page++ {
		res := netlifyRequestFull(cfg, http.MethodGet, next, nil)
		out.code, out.body = res.code, res.data
		if res.bodyErr != nil {
			out.err = res.bodyErr
			return out
		}
		if res.code != http.StatusOK {
			return out
		}
		var items []json.RawMessage
		if err := json.Unmarshal(res.data, &items); err != nil {
			out.err = fmt.Errorf("parse page %d: %w", page, err)
			return out
		}
		out.items = append(out.items, items...)

		link := netlifyNextPageLink(res.header.Get("Link"))
		if link == "" {
			return out
		}
		if page >= maxPages || !netlifySameAPIOrigin(link) {
			out.truncated = true
			return out
		}
		next = link
	}
}

// netlifyResolveSiteID returns the given siteID if non-empty, else the default from config.
func netlifyResolveSiteID(cfg NetlifyConfig, siteID string) string {
	if siteID != "" {
		return siteID
	}
	return cfg.DefaultSiteID
}

func netlifyPathSegment(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}

func netlifySitesEndpoint(teamSlug string) string {
	if strings.TrimSpace(teamSlug) == "" {
		return "/sites?per_page=100"
	}
	return fmt.Sprintf("/%s/sites?per_page=100", netlifyPathSegment(teamSlug))
}

func netlifyReadOnlyError(cfg NetlifyConfig) string {
	if !cfg.ReadOnly {
		return ""
	}
	return errJSON("Netlify is in read-only mode. Disable netlify.readonly to allow changes.")
}

func netlifySiteManagementError(cfg NetlifyConfig) string {
	if msg := netlifyReadOnlyError(cfg); msg != "" {
		return msg
	}
	if cfg.AllowSiteManagement {
		return ""
	}
	return errJSON("Netlify site management is not allowed. Set netlify.allow_site_management=true in config.yaml.")
}

func netlifyDeployError(cfg NetlifyConfig) string {
	if msg := netlifyReadOnlyError(cfg); msg != "" {
		return msg
	}
	if cfg.AllowDeploy {
		return ""
	}
	return errJSON("Netlify deploy is not allowed. Set netlify.allow_deploy=true in config.yaml.")
}

func netlifyEnvManagementError(cfg NetlifyConfig) string {
	if msg := netlifyReadOnlyError(cfg); msg != "" {
		return msg
	}
	if cfg.AllowEnvManagement {
		return ""
	}
	return errJSON("Netlify env var management is not allowed. Set netlify.allow_env_management=true in config.yaml.")
}

// netlifyResolveNameToID looks up a site by name, custom domain or default
// domain and returns its UUID. If the name is already a UUID or the lookup
// fails, it returns "".
func netlifyResolveNameToID(cfg NetlifyConfig, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if looksLikeUUID(name) {
		// Already a UUID — no lookup needed.
		return name
	}
	page := netlifyGetPages(cfg, netlifySitesEndpoint(cfg.TeamSlug), netlifyListMaxPages)
	if page.err != nil || page.code != http.StatusOK {
		return ""
	}
	for _, raw := range page.items {
		var site map[string]interface{}
		if json.Unmarshal(raw, &site) != nil {
			continue
		}
		for _, key := range []string{"name", "custom_domain", "default_domain"} {
			if strVal(site, key) == name {
				return strVal(site, "id")
			}
		}
	}
	return ""
}

// ── Sites ───────────────────────────────────────────────────────────────────

// NetlifyListSites returns all sites for the account/team.
func NetlifyListSites(cfg NetlifyConfig) string {
	page := netlifyGetPages(cfg, netlifySitesEndpoint(cfg.TeamSlug), netlifyListMaxPages)
	if page.err != nil {
		return errJSON("Failed to list sites: %v", page.err)
	}
	if page.code != http.StatusOK {
		return netlifyHTTPError(page.code, page.body)
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
	for _, raw := range page.items {
		var s map[string]interface{}
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
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

	payload := map[string]interface{}{"status": "ok", "count": len(result), "sites": result}
	if page.truncated {
		payload["truncated"] = true
	}
	out, _ := json.Marshal(payload)
	return string(out)
}

// NetlifyGetSite returns detailed info about a specific site.
func NetlifyGetSite(cfg NetlifyConfig, siteID string) string {
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required (or set default_site_id in config)")
	}
	data, code, err := netlifyRequest(cfg, http.MethodGet, "/sites/"+netlifyPathSegment(siteID), nil)
	if err != nil {
		return errJSON("Failed to get site: %v", err)
	}
	if code != http.StatusOK {
		return netlifyHTTPError(code, data)
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
	if msg := netlifySiteManagementError(cfg); msg != "" {
		return msg
	}
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name // subdomain: name.netlify.app
	}
	if customDomain != "" {
		body["custom_domain"] = customDomain
	}

	endpoint := "/sites"
	if cfg.TeamSlug != "" {
		endpoint = fmt.Sprintf("/%s/sites", netlifyPathSegment(cfg.TeamSlug))
	}

	data, code, err := netlifyRequest(cfg, http.MethodPost, endpoint, body)
	if err != nil {
		return errJSON("Failed to create site: %v", err)
	}
	if code != http.StatusCreated {
		return netlifyHTTPError(code, data)
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
	if msg := netlifySiteManagementError(cfg); msg != "" {
		return msg
	}
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

	data, code, err := netlifyRequest(cfg, http.MethodPatch, "/sites/"+netlifyPathSegment(siteID), body)
	if err != nil {
		return errJSON("Failed to update site: %v", err)
	}
	if code != http.StatusOK {
		return netlifyHTTPError(code, data)
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
	if msg := netlifySiteManagementError(cfg); msg != "" {
		return msg
	}
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required")
	}

	data, code, err := netlifyRequest(cfg, http.MethodDelete, "/sites/"+netlifyPathSegment(siteID), nil)
	if err != nil {
		return errJSON("Failed to delete site: %v", err)
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return netlifyHTTPError(code, data)
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

	page := netlifyGetPages(cfg, fmt.Sprintf("/sites/%s/deploys?per_page=20", netlifyPathSegment(siteID)), 1)
	if page.err != nil {
		return errJSON("Failed to list deploys: %v", page.err)
	}
	if page.code != http.StatusOK {
		return netlifyHTTPError(page.code, page.body)
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
	for _, raw := range page.items {
		var d map[string]interface{}
		if json.Unmarshal(raw, &d) != nil {
			continue
		}
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

	payload := map[string]interface{}{"status": "ok", "count": len(result), "deploys": result}
	if page.truncated {
		payload["truncated"] = true
	}
	out, _ := json.Marshal(payload)
	return string(out)
}

// NetlifyGetDeploy returns details about a specific deploy.
func NetlifyGetDeploy(cfg NetlifyConfig, deployID string) string {
	if deployID == "" {
		return errJSON("deploy_id is required")
	}

	data, code, err := netlifyRequest(cfg, http.MethodGet, "/deploys/"+netlifyPathSegment(deployID), nil)
	if err != nil {
		return errJSON("Failed to get deploy: %v", err)
	}
	if code != http.StatusOK {
		return netlifyHTTPError(code, data)
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

// NetlifyWaitForDeploy polls a deploy until Netlify reports a terminal state.
// The returned object keeps the provider's own url (site URL) and deploy_url
// (immutable deploy permalink); callers must not use one as the other.
func NetlifyWaitForDeploy(cfg NetlifyConfig, deployID string, maxAttempts int, delay time.Duration) string {
	if deployID == "" {
		return errJSON("deploy_id is required")
	}
	if maxAttempts <= 0 {
		maxAttempts = netlifyDeployPollAttempts
	}
	if delay < 0 {
		delay = 0
	}
	var last map[string]interface{}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		raw := NetlifyGetDeploy(cfg, deployID)
		var current map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &current); err != nil {
			return errJSON("Failed to parse deploy status: %v", err)
		}
		last = current
		if current["status"] != "ok" {
			return raw
		}
		state := strings.ToLower(strVal(current, "state"))
		switch state {
		case "ready":
			current["attempts"] = attempt
			out, _ := json.Marshal(current)
			return string(out)
		case "error", "failed", "rejected":
			msg := strVal(current, "error_message")
			if msg == "" {
				msg = "Netlify deploy ended in state " + state
			}
			return errJSON("%s", msg)
		}
		if attempt < maxAttempts && delay > 0 {
			if err := waitToolHTTP(cfg.Context, delay); err != nil {
				return errJSON("Deploy status canceled: %v", err)
			}
		}
	}
	if last != nil {
		last["status"] = "error"
		last["message"] = "Timed out waiting for Netlify deploy to become ready"
		out, _ := json.Marshal(last)
		return string(out)
	}
	return errJSON("Timed out waiting for Netlify deploy to become ready")
}

// NetlifyTestConnection checks whether the Netlify API is reachable and the token is valid.
// Returns a diagnostic object with dns_ok, tcp_ok, api_ok, resolved_ips, and account info.
// The probes use the same transport as the real requests so a diagnosis never
// contradicts the caller's actual network path.
func NetlifyTestConnection(cfg NetlifyConfig) string {
	result := map[string]interface{}{"api_url": netlifyBaseURL}

	base, parseErr := url.Parse(netlifyBaseURL)
	if parseErr != nil || base.Hostname() == "" {
		result["status"] = "error"
		result["message"] = "Netlify API base URL is not a valid absolute URL"
		out, _ := json.Marshal(result)
		return string(out)
	}
	host := base.Hostname()
	port := base.Port()
	if port == "" {
		if base.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}

	// 1. DNS resolution
	ctx := nativeHTTPContext(cfg.Context)
	dnsCtx, cancelDNS := context.WithTimeout(ctx, 10*time.Second)
	addrs, dnsErr := net.DefaultResolver.LookupHost(dnsCtx, host)
	cancelDNS()
	if dnsErr != nil || len(addrs) == 0 {
		result["dns_ok"] = false
		message := "DNS resolution failed for " + host
		if dnsErr != nil {
			result["dns_error"] = dnsErr.Error()
			message += ": " + dnsErr.Error()
		}
		result["status"] = "error"
		result["message"] = message + " — Check DNS settings or whether the hostname is blocked."
		out, _ := json.Marshal(result)
		return string(out)
	}
	result["dns_ok"] = true
	result["resolved_ips"] = addrs

	// 2. TCP connection through the same multi-address dialer the API uses.
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	conn, tcpErr := netlifyDial(dialCtx, "tcp", net.JoinHostPort(host, port))
	cancel()
	if tcpErr != nil {
		result["tcp_ok"] = false
		result["tcp_error"] = tcpErr.Error()
		result["status"] = "error"
		result["message"] = fmt.Sprintf(
			"TCP connection to %s:%s failed: %v — "+
				"The Netlify API is blocked by a firewall or network policy on this host. "+
				"Check iptables/nftables rules, or ask your provider to allow outbound HTTPS "+
				"to %v.",
			host, port, tcpErr, addrs)
		out, _ := json.Marshal(result)
		return string(out)
	}
	conn.Close()
	result["tcp_ok"] = true

	// 3. Authenticated API call
	data, code, apiErr := netlifyRequest(cfg, http.MethodGet, "/user", nil)
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
		result["message"] = "Authentication failed (HTTP 401) — the netlify_token in the vault is invalid or expired."
		out, _ := json.Marshal(result)
		return string(out)
	}
	if code != http.StatusOK {
		result["api_ok"] = false
		result["api_http_code"] = code
		result["status"] = "error"
		result["message"] = netlifyProviderErrorText(data)
		if result["message"] == "" {
			result["message"] = fmt.Sprintf("API returned unexpected HTTP %d", code)
		}
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

// NetlifyDeployZip creates a new deploy by uploading a ZIP archive.
// The zipData must be the raw ZIP file bytes.
func NetlifyDeployZip(cfg NetlifyConfig, siteID, title string, draft bool, zipData []byte) string {
	if msg := netlifyDeployError(cfg); msg != "" {
		return msg
	}
	siteID = netlifyResolveSiteID(cfg, siteID)
	if siteID == "" {
		return errJSON("site_id is required. Use 'netlify check_connection' to verify API access, then 'netlify list_sites' to find your site ID, or set default_site_id in the Netlify config.")
	}
	if len(zipData) == 0 {
		return errJSON("zip data is empty")
	}

	endpoint := fmt.Sprintf("/sites/%s/deploys", netlifyPathSegment(siteID))
	query := url.Values{}
	if draft {
		query.Set("draft", "true")
	}
	if title != "" {
		query.Set("title", title)
	}
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	data, code, err := netlifyRequestRaw(cfg, http.MethodPost, endpoint, "application/zip", bytes.NewReader(zipData))
	if err != nil {
		return errJSON("Failed to deploy: %v", err)
	}
	// Netlify returns 200 for re-deploys and 201 Created for new deploys.
	if code != http.StatusOK && code != http.StatusCreated {
		if code == http.StatusNotFound {
			return netlifyCodedError(code, "Site %q not found on Netlify. Either the site_id is wrong or the site was deleted. Use 'netlify list_sites' to see existing sites, or use 'homepage deploy_netlify' which auto-creates the site if it does not exist.", siteID)
		}
		return netlifyHTTPError(code, data)
	}

	var deploy map[string]interface{}
	if err := json.Unmarshal(data, &deploy); err != nil {
		return errJSON("Failed to parse deploy response: %v", err)
	}

	deployID := firstNonEmptyString(strVal(deploy, "id"), strVal(deploy, "deploy_id"))
	if deployID == "" {
		return errJSON("Netlify deploy response missing deploy_id")
	}
	deploySiteID := firstNonEmptyString(strVal(deploy, "site_id"), siteID)
	out, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"message":   "Deploy initiated",
		"id":        deployID,
		"deploy_id": deployID,
		"site_id":   deploySiteID,
		"state":     strVal(deploy, "state"),
		// url is the site's published URL; deploy_url is this deploy's immutable
		// permalink. Keep both distinct.
		"deploy_url": strVal(deploy, "deploy_url"),
		"url":        strVal(deploy, "url"),
		"draft":      draft,
	})
	return string(out)
}
