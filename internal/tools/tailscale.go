package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// TailscaleConfig holds the Tailscale API connection parameters.
type TailscaleConfig struct {
	APIKey   string // Tailscale API key (tskey-api-…)
	Tailnet  string // Tailnet name, e.g. "example.com" or "-" for default/implicit tailnet
	ReadOnly bool   // true = block route changes
}

var tailscaleHTTPClient = &http.Client{Timeout: 30 * time.Second}
var tailscaleLocalHTTPClient = &http.Client{Timeout: 5 * time.Second}

// tailscaleTailnet returns the tailnet identifier, defaulting to "-" (implicit tailnet).
func tailscaleTailnet(cfg TailscaleConfig) string {
	if cfg.Tailnet == "" {
		return "-"
	}
	return cfg.Tailnet
}

func tailscaleReadOnlyError(cfg TailscaleConfig) string {
	if !cfg.ReadOnly {
		return ""
	}
	return errJSON("Tailscale is in read-only mode. Disable tailscale.readonly to allow changes.")
}

// tailscaleRequest executes an authenticated HTTP request against the Tailscale API v2.
func tailscaleRequest(cfg TailscaleConfig, method, endpoint, body string) ([]byte, int, error) {
	url := "https://api.tailscale.com" + endpoint
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := tailscaleHTTPClient.Do(req)
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

// tailscaleFindDeviceID resolves a hostname, MagicDNS name, IP address, or node ID to a
// Tailscale device node ID. If the query is already a node ID it is returned unchanged.
func tailscaleFindDeviceID(cfg TailscaleConfig, query string) (string, error) {
	// Node IDs are long alphanumeric strings (no dots or spaces).
	if len(query) > 10 && !strings.ContainsAny(query, ". \t") {
		return query, nil
	}
	endpoint := fmt.Sprintf("/api/v2/tailnet/%s/devices", tailscaleTailnet(cfg))
	data, code, err := tailscaleRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("API returned HTTP %d", code)
	}
	var result struct {
		Devices []struct {
			ID        string   `json:"id"`
			Hostname  string   `json:"hostname"`
			Name      string   `json:"name"`
			Addresses []string `json:"addresses"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse devices: %w", err)
	}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, d := range result.Devices {
		// Match by short hostname or full MagicDNS name
		if strings.ToLower(d.Hostname) == q || strings.ToLower(d.Name) == q ||
			strings.HasPrefix(strings.ToLower(d.Name), q+".") {
			return d.ID, nil
		}
		// Match by Tailscale IP (strip CIDR suffix if present)
		for _, addr := range d.Addresses {
			bare := strings.SplitN(addr, "/", 2)[0]
			if bare == query {
				return d.ID, nil
			}
		}
	}
	return "", fmt.Errorf("device not found: %q", query)
}

// ─── Public API functions ────────────────────────────────────────────────────

// TailscaleListDevices lists all devices in the tailnet with their IPs and online status.
func TailscaleListDevices(cfg TailscaleConfig) string {
	endpoint := fmt.Sprintf("/api/v2/tailnet/%s/devices", tailscaleTailnet(cfg))
	data, code, err := tailscaleRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to list devices: %v", err)
	}
	if code != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code, "body": string(data)})
	}
	var result struct {
		Devices []struct {
			ID        string   `json:"id"`
			Hostname  string   `json:"hostname"`
			Name      string   `json:"name"`
			Addresses []string `json:"addresses"`
			OS        string   `json:"os"`
			LastSeen  string   `json:"lastSeen"`
			Online    *bool    `json:"online"`
			Tags      []string `json:"tags"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return marshalToolJSON(map[string]interface{}{"status": "ok", "raw": jsonRawOrString(data)})
	}
	type devSummary struct {
		ID        string   `json:"id"`
		Hostname  string   `json:"hostname"`
		Name      string   `json:"name"`
		Addresses []string `json:"addresses"`
		OS        string   `json:"os"`
		LastSeen  string   `json:"last_seen"`
		Online    *bool    `json:"online"`
		Tags      []string `json:"tags,omitempty"`
	}
	summaries := make([]devSummary, 0, len(result.Devices))
	for _, d := range result.Devices {
		summaries = append(summaries, devSummary{
			ID:        d.ID,
			Hostname:  d.Hostname,
			Name:      d.Name,
			Addresses: d.Addresses,
			OS:        d.OS,
			LastSeen:  d.LastSeen,
			Online:    d.Online,
			Tags:      d.Tags,
		})
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(summaries),
		"devices": summaries,
	})
	return string(out)
}

// TailscaleGetDevice returns full details for a device looked up by node ID, hostname, or IP.
func TailscaleGetDevice(cfg TailscaleConfig, query string) string {
	if query == "" {
		return errJSON("device query (hostname, IP, or node ID) is required")
	}
	deviceID, err := tailscaleFindDeviceID(cfg, query)
	if err != nil {
		return errJSON("Device lookup failed: %v", err)
	}
	data, code, rerr := tailscaleRequest(cfg, "GET", fmt.Sprintf("/api/v2/device/%s", deviceID), "")
	if rerr != nil {
		return errJSON("Failed to get device: %v", rerr)
	}
	if code != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code, "body": string(data)})
	}
	return marshalToolJSON(map[string]interface{}{"status": "ok", "device": jsonRawOrString(data)})
}

// TailscaleGetRoutes returns the subnet routes (advertised and enabled) for a device.
func TailscaleGetRoutes(cfg TailscaleConfig, query string) string {
	if query == "" {
		return errJSON("device query (hostname, IP, or node ID) is required")
	}
	deviceID, err := tailscaleFindDeviceID(cfg, query)
	if err != nil {
		return errJSON("Device lookup failed: %v", err)
	}
	data, code, rerr := tailscaleRequest(cfg, "GET", fmt.Sprintf("/api/v2/device/%s/routes", deviceID), "")
	if rerr != nil {
		return errJSON("Failed to get routes: %v", rerr)
	}
	if code != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code, "body": string(data)})
	}
	return marshalToolJSON(map[string]interface{}{"status": "ok", "device_id": deviceID, "routes": jsonRawOrString(data)})
}

// TailscaleSetRoutes enables or disables specific subnet routes for a device.
// routes is a slice of CIDR prefixes (e.g. "192.168.1.0/24").
// enable=true adds them to the approved set; enable=false removes them.
func TailscaleSetRoutes(cfg TailscaleConfig, query string, routes []string, enable bool) string {
	if msg := tailscaleReadOnlyError(cfg); msg != "" {
		return msg
	}
	if query == "" {
		return errJSON("device query is required")
	}
	if len(routes) == 0 {
		return errJSON("at least one route (CIDR) is required")
	}
	deviceID, err := tailscaleFindDeviceID(cfg, query)
	if err != nil {
		return errJSON("Device lookup failed: %v", err)
	}
	endpoint := fmt.Sprintf("/api/v2/device/%s/routes", deviceID)

	// Fetch current enabled routes so we can do a delta update.
	current, code, rerr := tailscaleRequest(cfg, "GET", endpoint, "")
	if rerr != nil {
		return errJSON("Failed to fetch current routes: %v", rerr)
	}
	if code != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code, "message": "Failed to fetch current routes"})
	}
	var routeData struct {
		EnabledRoutes []string `json:"enabledRoutes"`
	}
	_ = json.Unmarshal(current, &routeData)

	// Validate each provided CIDR before touching the API.
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, _, cidrErr := net.ParseCIDR(r); cidrErr != nil {
			return errJSON("Invalid CIDR %q: %v", r, cidrErr)
		}
	}

	// Build the new enabled set.
	set := make(map[string]bool)
	for _, r := range routeData.EnabledRoutes {
		set[r] = true
	}
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if enable {
			set[r] = true
		} else {
			delete(set, r)
		}
	}
	newRoutes := make([]string, 0, len(set))
	for r := range set {
		newRoutes = append(newRoutes, r)
	}
	body, _ := json.Marshal(map[string]interface{}{"routes": newRoutes})
	respData, code2, rerr2 := tailscaleRequest(cfg, "POST", endpoint, string(body))
	if rerr2 != nil {
		return errJSON("Failed to set routes: %v", rerr2)
	}
	if code2 != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code2, "body": string(respData)})
	}
	action := "enabled"
	if !enable {
		action = "disabled"
	}
	return marshalToolJSON(map[string]interface{}{"status": "ok", "message": fmt.Sprintf("Routes %s successfully", action), "device_id": deviceID, "result": jsonRawOrString(respData)})
}

// TailscaleGetDNS returns the DNS nameserver configuration for the tailnet.
func TailscaleGetDNS(cfg TailscaleConfig) string {
	endpoint := fmt.Sprintf("/api/v2/tailnet/%s/dns/nameservers", tailscaleTailnet(cfg))
	data, code, err := tailscaleRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to get DNS: %v", err)
	}
	if code != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code, "body": string(data)})
	}
	return marshalToolJSON(map[string]interface{}{"status": "ok", "dns": jsonRawOrString(data)})
}

// TailscaleGetACL returns the current ACL policy document for the tailnet.
func TailscaleGetACL(cfg TailscaleConfig) string {
	endpoint := fmt.Sprintf("/api/v2/tailnet/%s/acl", tailscaleTailnet(cfg))
	data, code, err := tailscaleRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to get ACL: %v", err)
	}
	if code != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": code, "body": string(data)})
	}
	return marshalToolJSON(map[string]interface{}{"status": "ok", "acl": jsonRawOrString(data)})
}

// TailscaleLocalStatus queries the Tailscale daemon running on the same host as AuraGo
// via the local API (http://127.0.0.1:41112). Only available if Tailscale is installed locally.
func TailscaleLocalStatus() string {
	req, err := http.NewRequest("GET", "http://127.0.0.1:41112/localapi/v0/status", nil)
	if err != nil {
		return errJSON("Failed to create request: %v", err)
	}
	// Tailscale local API requires a capability header.
	req.Header.Set("Tailscale-Cap", "72")
	resp, err := tailscaleLocalHTTPClient.Do(req)
	if err != nil {
		return errJSON("Local Tailscale daemon not reachable (is Tailscale installed on this host?): %v", err)
	}
	defer resp.Body.Close()
	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return errJSON("Failed to read local daemon response: %v", err)
	}
	if resp.StatusCode != 200 {
		return marshalToolJSON(map[string]interface{}{"status": "error", "http_code": resp.StatusCode, "message": "Local daemon query failed"})
	}
	// Parse and present a concise summary.
	var status struct {
		BackendState string `json:"BackendState"`
		Self         struct {
			ID           string   `json:"ID"`
			HostName     string   `json:"HostName"`
			DNSName      string   `json:"DNSName"`
			TailscaleIPs []string `json:"TailscaleIPs"`
			OS           string   `json:"OS"`
			Online       bool     `json:"Online"`
		} `json:"Self"`
		Peer map[string]struct {
			HostName     string   `json:"HostName"`
			DNSName      string   `json:"DNSName"`
			TailscaleIPs []string `json:"TailscaleIPs"`
			OS           string   `json:"OS"`
			Online       bool     `json:"Online"`
			Active       bool     `json:"Active"`
			LastSeen     string   `json:"LastSeen"`
		} `json:"Peer"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return marshalToolJSON(map[string]interface{}{"status": "ok", "raw": jsonRawOrString(data)})
	}
	type peerSummary struct {
		Hostname string   `json:"hostname"`
		DNSName  string   `json:"dns_name"`
		IPs      []string `json:"ips"`
		OS       string   `json:"os"`
		Online   bool     `json:"online"`
		Active   bool     `json:"active"`
		LastSeen string   `json:"last_seen"`
	}
	peers := make([]peerSummary, 0, len(status.Peer))
	for _, p := range status.Peer {
		peers = append(peers, peerSummary{
			Hostname: p.HostName,
			DNSName:  p.DNSName,
			IPs:      p.TailscaleIPs,
			OS:       p.OS,
			Online:   p.Online,
			Active:   p.Active,
			LastSeen: p.LastSeen,
		})
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"backend_state": status.BackendState,
		"self": map[string]interface{}{
			"id":       status.Self.ID,
			"hostname": status.Self.HostName,
			"dns_name": status.Self.DNSName,
			"ips":      status.Self.TailscaleIPs,
			"os":       status.Self.OS,
			"online":   status.Self.Online,
		},
		"peers":      peers,
		"peer_count": len(peers),
	})
	return string(out)
}
