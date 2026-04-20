package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AdGuardConfig holds the connection parameters for an AdGuard Home instance.
type AdGuardConfig struct {
	URL      string
	Username string
	Password string
}

// adguardClient is a lazily-initialized shared HTTP client.
// AdGuard Home runs on the local network (private IPs), so SSRF protection
// is not applicable — the URL is admin-configured, not user-supplied.
var adguardClient *http.Client
var adguardClientOnce sync.Once
var adguardClientFactory = func() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func getAdGuardClient() *http.Client {
	adguardClientOnce.Do(func() {
		adguardClient = adguardClientFactory()
	})
	return adguardClient
}

// adguardRequest performs an HTTP request against the AdGuard Home API.
func adguardRequest(cfg AdGuardConfig, method, endpoint string, body string) ([]byte, int, error) {
	if cfg.URL == "" {
		return nil, 0, fmt.Errorf("AdGuard Home URL is not configured")
	}

	baseURL := strings.TrimRight(cfg.URL, "/")
	reqURL := baseURL + endpoint

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Username != "" || cfg.Password != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	resp, err := getAdGuardClient().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func adgOK(payload interface{}) string {
	result := map[string]interface{}{"status": "ok"}
	if payload != nil {
		result["data"] = payload
	}
	out, _ := json.Marshal(result)
	return string(out)
}

func adgError(msg string, args ...interface{}) string {
	text := fmt.Sprintf(msg, args...)
	out, _ := json.Marshal(map[string]interface{}{"status": "error", "message": text})
	return string(out)
}

func adgHTTPError(code int, data []byte) string {
	out, _ := json.Marshal(map[string]interface{}{"status": "error", "http_code": code, "message": string(data)})
	return string(out)
}

// adgGet performs a GET request and returns the raw response as a wrapped JSON result.
func adgGet(cfg AdGuardConfig, endpoint string) string {
	data, code, err := adguardRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return adgError("%v", err)
	}
	if code != 200 {
		return adgHTTPError(code, data)
	}
	var parsed interface{}
	if json.Unmarshal(data, &parsed) == nil {
		return adgOK(parsed)
	}
	// Plain text response (e.g. some toggle endpoints)
	return adgOK(strings.TrimSpace(string(data)))
}

// adgPost performs a POST request with a JSON body and returns a wrapped result.
func adgPost(cfg AdGuardConfig, endpoint string, body string) string {
	data, code, err := adguardRequest(cfg, "POST", endpoint, body)
	if err != nil {
		return adgError("%v", err)
	}
	if code < 200 || code >= 300 {
		return adgHTTPError(code, data)
	}
	if len(data) == 0 {
		return adgOK(nil)
	}
	var parsed interface{}
	if json.Unmarshal(data, &parsed) == nil {
		return adgOK(parsed)
	}
	return adgOK(strings.TrimSpace(string(data)))
}

// ─── Status & Statistics ──────────────────────────────────────────────────

// AdGuardStatus returns the AdGuard Home server status (version, running state, DNS addresses, etc.).
func AdGuardStatus(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/status")
}

// AdGuardStats returns general DNS statistics (queries, blocked, avg processing time, etc.).
func AdGuardStats(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/stats")
}

// AdGuardStatsTop returns top clients, top queried domains, and top blocked domains.
func AdGuardStatsTop(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/stats/top")
}

// ─── Query Log ────────────────────────────────────────────────────────────

// AdGuardQueryLog retrieves recent DNS query log entries.
func AdGuardQueryLog(cfg AdGuardConfig, search string, limit int, offset int) string {
	params := url.Values{}
	if search != "" {
		params.Set("search", search)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	} else {
		params.Set("limit", "25")
	}
	if offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}
	endpoint := "/control/querylog?" + params.Encode()
	return adgGet(cfg, endpoint)
}

// AdGuardQueryLogClear clears the entire query log.
func AdGuardQueryLogClear(cfg AdGuardConfig) string {
	return adgPost(cfg, "/control/querylog_clear", "")
}

// ─── Filtering ────────────────────────────────────────────────────────────

// AdGuardFilteringStatus returns the current filtering configuration and enabled filter lists.
func AdGuardFilteringStatus(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/filtering/status")
}

// AdGuardFilteringToggle enables or disables DNS filtering globally.
func AdGuardFilteringToggle(cfg AdGuardConfig, enabled bool) string {
	body := fmt.Sprintf(`{"enabled":%t,"interval":0}`, enabled)
	return adgPost(cfg, "/control/filtering/config", body)
}

// AdGuardFilteringAddURL adds a new filter list by URL.
func AdGuardFilteringAddURL(cfg AdGuardConfig, name, filterURL string) string {
	if filterURL == "" {
		return adgError("filter URL is required")
	}
	if name == "" {
		name = filterURL
	}
	body, _ := json.Marshal(map[string]interface{}{
		"name":    name,
		"url":     filterURL,
		"enabled": true,
	})
	return adgPost(cfg, "/control/filtering/add_url", string(body))
}

// AdGuardFilteringRemoveURL removes a filter list by its URL.
func AdGuardFilteringRemoveURL(cfg AdGuardConfig, filterURL string) string {
	if filterURL == "" {
		return adgError("filter URL is required")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"url": filterURL,
	})
	return adgPost(cfg, "/control/filtering/remove_url", string(body))
}

// AdGuardFilteringRefresh forces a refresh of all filter lists.
func AdGuardFilteringRefresh(cfg AdGuardConfig) string {
	return adgPost(cfg, "/control/filtering/refresh", `{"whitelist":false}`)
}

// AdGuardFilteringSetRules sets the custom filtering rules (one rule per line).
func AdGuardFilteringSetRules(cfg AdGuardConfig, rules string) string {
	body, _ := json.Marshal(map[string]interface{}{
		"rules": strings.Split(rules, "\n"),
	})
	return adgPost(cfg, "/control/filtering/set_rules", string(body))
}

// ─── DNS Rewrites ─────────────────────────────────────────────────────────

// AdGuardRewriteList returns all configured DNS rewrite rules.
func AdGuardRewriteList(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/rewrite/list")
}

// AdGuardRewriteAdd adds a new DNS rewrite rule.
func AdGuardRewriteAdd(cfg AdGuardConfig, domain, answer string) string {
	if domain == "" || answer == "" {
		return adgError("both domain and answer are required")
	}
	body, _ := json.Marshal(map[string]string{"domain": domain, "answer": answer})
	return adgPost(cfg, "/control/rewrite/add", string(body))
}

// AdGuardRewriteDelete removes a DNS rewrite rule.
func AdGuardRewriteDelete(cfg AdGuardConfig, domain, answer string) string {
	if domain == "" || answer == "" {
		return adgError("both domain and answer are required")
	}
	body, _ := json.Marshal(map[string]string{"domain": domain, "answer": answer})
	return adgPost(cfg, "/control/rewrite/delete", string(body))
}

// ─── Blocked Services ─────────────────────────────────────────────────────

// AdGuardBlockedServicesList returns the list of available services and which are currently blocked.
func AdGuardBlockedServicesList(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/blocked_services/list")
}

// AdGuardBlockedServicesSet sets which services are blocked (e.g. ["tiktok", "facebook"]).
func AdGuardBlockedServicesSet(cfg AdGuardConfig, services []string) string {
	if services == nil {
		services = []string{}
	}
	body, _ := json.Marshal(services)
	return adgPost(cfg, "/control/blocked_services/set", string(body))
}

// ─── Safe Browsing & Parental ─────────────────────────────────────────────

// AdGuardSafeBrowsingStatus returns whether safe browsing protection is enabled.
func AdGuardSafeBrowsingStatus(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/safebrowsing/status")
}

// AdGuardSafeBrowsingToggle enables or disables safe browsing protection.
func AdGuardSafeBrowsingToggle(cfg AdGuardConfig, enabled bool) string {
	if enabled {
		return adgPost(cfg, "/control/safebrowsing/enable", "")
	}
	return adgPost(cfg, "/control/safebrowsing/disable", "")
}

// AdGuardParentalStatus returns whether parental control is enabled.
func AdGuardParentalStatus(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/parental/status")
}

// AdGuardParentalToggle enables or disables parental control.
func AdGuardParentalToggle(cfg AdGuardConfig, enabled bool) string {
	if enabled {
		return adgPost(cfg, "/control/parental/enable", "")
	}
	return adgPost(cfg, "/control/parental/disable", "")
}

// ─── DHCP ─────────────────────────────────────────────────────────────────

// AdGuardDHCPStatus returns the DHCP server configuration and active leases.
func AdGuardDHCPStatus(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/dhcp/status")
}

// AdGuardDHCPSetConfig updates the DHCP server configuration.
// configJSON is a raw JSON string matching the AdGuard Home DHCP config schema.
func AdGuardDHCPSetConfig(cfg AdGuardConfig, configJSON string) string {
	if configJSON == "" {
		return adgError("DHCP config JSON is required")
	}
	return adgPost(cfg, "/control/dhcp/set_config", configJSON)
}

// AdGuardDHCPAddLease adds a static DHCP lease.
func AdGuardDHCPAddLease(cfg AdGuardConfig, mac, ip, hostname string) string {
	if mac == "" || ip == "" {
		return adgError("mac and ip are required for a static lease")
	}
	body, _ := json.Marshal(map[string]string{
		"mac":      mac,
		"ip":       ip,
		"hostname": hostname,
	})
	return adgPost(cfg, "/control/dhcp/add_static_lease", string(body))
}

// AdGuardDHCPRemoveLease removes a static DHCP lease.
func AdGuardDHCPRemoveLease(cfg AdGuardConfig, mac, ip, hostname string) string {
	if mac == "" || ip == "" {
		return adgError("mac and ip are required")
	}
	body, _ := json.Marshal(map[string]string{
		"mac":      mac,
		"ip":       ip,
		"hostname": hostname,
	})
	return adgPost(cfg, "/control/dhcp/remove_static_lease", string(body))
}

// ─── Clients ──────────────────────────────────────────────────────────────

// AdGuardClients returns the list of known (persistent) clients and their settings.
func AdGuardClients(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/clients")
}

// AdGuardClientAdd adds a new known client.
// clientJSON is a raw JSON string matching the AdGuard Home client schema.
func AdGuardClientAdd(cfg AdGuardConfig, clientJSON string) string {
	if clientJSON == "" {
		return adgError("client JSON is required")
	}
	return adgPost(cfg, "/control/clients/add", clientJSON)
}

// AdGuardClientUpdate updates an existing known client.
// clientJSON should contain both "name" (lookup key) and the updated "data" object.
func AdGuardClientUpdate(cfg AdGuardConfig, clientJSON string) string {
	if clientJSON == "" {
		return adgError("client JSON is required")
	}
	return adgPost(cfg, "/control/clients/update", clientJSON)
}

// AdGuardClientDelete removes a known client by name.
func AdGuardClientDelete(cfg AdGuardConfig, name string) string {
	if name == "" {
		return adgError("client name is required")
	}
	body, _ := json.Marshal(map[string]string{"name": name})
	return adgPost(cfg, "/control/clients/delete", string(body))
}

// ─── DNS Settings ─────────────────────────────────────────────────────────

// AdGuardDNSInfo returns the current DNS configuration.
func AdGuardDNSInfo(cfg AdGuardConfig) string {
	return adgGet(cfg, "/control/dns_info")
}

// AdGuardDNSConfig updates DNS server settings.
// settingsJSON is a raw JSON string matching the AdGuard Home DNS config schema.
func AdGuardDNSConfig(cfg AdGuardConfig, settingsJSON string) string {
	if settingsJSON == "" {
		return adgError("DNS settings JSON is required")
	}
	return adgPost(cfg, "/control/dns_config", settingsJSON)
}

// AdGuardTestUpstream tests the provided upstream DNS servers for reachability.
func AdGuardTestUpstream(cfg AdGuardConfig, upstreams []string) string {
	if len(upstreams) == 0 {
		return adgError("at least one upstream DNS server is required")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"upstream_dns": upstreams,
	})
	return adgPost(cfg, "/control/test_upstream_dns", string(body))
}
