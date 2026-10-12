package tools

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/httporigin"
	"aurago/internal/security"
)

// ProxmoxConfig holds the Proxmox VE connection parameters.
type ProxmoxConfig struct {
	URL              string // e.g. "https://pve.example.com:8006"
	TokenID          string // e.g. "user@pam!tokenname"
	Secret           string // API token secret
	Node             string // default node name (e.g. "pve")
	Insecure         bool   // skip TLS verification
	ReadOnly         bool   // block mutating operations when true
	AllowDestructive bool   // allow stop/shutdown/reboot/suspend/reset actions
}

var (
	proxmoxClientCache sync.Map
	proxmoxNodePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
)

func getProxmoxClient(cfg ProxmoxConfig) *http.Client {
	cacheKey := cfg.URL + "|" + cfg.TokenID + "|" + strconv.FormatBool(cfg.Insecure)
	if cached, ok := proxmoxClientCache.Load(cacheKey); ok {
		return cached.(*http.Client)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.Insecure,
	}
	client := &http.Client{
		Timeout:       30 * time.Second,
		Transport:     transport,
		CheckRedirect: httporigin.SameOriginRedirect,
	}
	actual, _ := proxmoxClientCache.LoadOrStore(cacheKey, client)
	return actual.(*http.Client)
}

func proxmoxNodePath(node string) (string, error) {
	node = strings.TrimSpace(node)
	if node == "" {
		return "", errors.New("node is required")
	}
	if !proxmoxNodePattern.MatchString(node) {
		return "", fmt.Errorf("invalid node %q", node)
	}
	return url.PathEscape(node), nil
}

func proxmoxRequireNode(cfg ProxmoxConfig, node string) (string, string, error) {
	node = strings.TrimSpace(node)
	if node == "" {
		node = strings.TrimSpace(cfg.Node)
	}
	segment, err := proxmoxNodePath(node)
	return node, segment, err
}

func proxmoxStatusError(err error) string {
	return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
}

func proxmoxBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid proxmox url: only an https origin is supported")
	}
	prefix, err := proxmoxPathPrefix(parsed.EscapedPath())
	if err != nil {
		return "", err
	}
	return "https://" + parsed.Host + prefix, nil
}

func proxmoxPathPrefix(escaped string) (string, error) {
	if escaped == "" || escaped == "/" {
		return "", nil
	}
	decoded, err := url.PathUnescape(escaped)
	if err != nil || strings.Contains(decoded, "\\") || strings.Contains(decoded, "//") {
		return "", errors.New("invalid proxmox url: only an https origin is supported")
	}
	decoded = strings.TrimRight(decoded, "/")
	if !strings.HasPrefix(decoded, "/") {
		return "", errors.New("invalid proxmox url: only an https origin is supported")
	}
	for _, segment := range strings.Split(strings.Trim(decoded, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", errors.New("invalid proxmox url: only an https origin is supported")
		}
	}
	return decoded, nil
}

// proxmoxJSONStr produces a JSON-safe quoted string (unlike %q which uses Go escaping).
func proxmoxJSONStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func proxmoxReadOnlyMutationError(cfg ProxmoxConfig) string {
	if cfg.ReadOnly {
		return `{"status":"error","message":"Proxmox is in read-only mode. Disable proxmox.readonly to allow changes."}`
	}
	return ""
}

func proxmoxDestructiveActionError(cfg ProxmoxConfig, action string) string {
	switch action {
	case "stop", "shutdown", "reboot", "suspend", "reset":
		if !cfg.AllowDestructive {
			return `{"status":"error","message":"Destructive Proxmox operations are disabled. Set proxmox.allow_destructive=true in config.yaml."}`
		}
	}
	return ""
}

// proxmoxRequest performs a generic HTTP request against the Proxmox VE API.
func proxmoxRequest(ctx context.Context, cfg ProxmoxConfig, method, endpoint, body string) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Prevent Proxmox API credentials from leaking into logs or LLM output
	security.RegisterSensitive(cfg.Secret)
	security.RegisterSensitive(cfg.TokenID)
	baseURL, err := proxmoxBaseURL(cfg.URL)
	if err != nil {
		return nil, 0, err
	}
	cfg.URL = baseURL
	requestURL := baseURL + "/api2/json" + endpoint

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, requestURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "PVEAPIToken="+cfg.TokenID+"="+cfg.Secret)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := getProxmoxClient(cfg).Do(req)
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

// proxmoxExtractData extracts the "data" field from the standard Proxmox API response wrapper.
func proxmoxExtractData(raw []byte) json.RawMessage {
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &wrapper) == nil && wrapper.Data != nil {
		return wrapper.Data
	}
	return raw
}

func proxmoxGetClusterResourceList(ctx context.Context, cfg ProxmoxConfig, resType string) ([]map[string]interface{}, int, error) {
	endpoint := "/cluster/resources"
	if resType != "" {
		endpoint += "?type=" + url.QueryEscape(resType)
	}
	data, code, err := proxmoxRequest(ctx, cfg, "GET", endpoint, "")
	if err != nil {
		return nil, code, err
	}
	if code != 200 {
		return nil, code, fmt.Errorf("cluster resources returned HTTP %d: %s", code, string(data))
	}
	var resources []map[string]interface{}
	if err := json.Unmarshal(proxmoxExtractData(data), &resources); err != nil {
		return nil, code, fmt.Errorf("failed to parse cluster resources: %w", err)
	}
	return resources, code, nil
}

func proxmoxFilterClusterResources(resources []map[string]interface{}, resourceType, node, vmid string) []map[string]interface{} {
	filtered := make([]map[string]interface{}, 0, len(resources))
	wantVMID := strings.TrimSpace(vmid)
	wantNode := strings.TrimSpace(node)
	for _, item := range resources {
		itemType := strings.TrimSpace(fmt.Sprint(item["type"]))
		if resourceType != "" && itemType != resourceType {
			continue
		}
		if wantNode != "" && strings.TrimSpace(fmt.Sprint(item["node"])) != wantNode {
			continue
		}
		if wantVMID != "" {
			switch raw := item["vmid"].(type) {
			case float64:
				if strconv.FormatInt(int64(raw), 10) != wantVMID {
					continue
				}
			case string:
				if strings.TrimSpace(raw) != wantVMID {
					continue
				}
			default:
				if strings.TrimSpace(fmt.Sprint(raw)) != wantVMID {
					continue
				}
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func proxmoxNormalizeResourceList(items []map[string]interface{}, resourceType, defaultNode string) []map[string]interface{} {
	normalized := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		entryType := strings.TrimSpace(fmt.Sprint(item["type"]))
		if entryType == "" || entryType == "<nil>" {
			entryType = resourceType
		}
		if resourceType != "" && entryType != resourceType {
			continue
		}

		node := strings.TrimSpace(fmt.Sprint(item["node"]))
		if node == "" {
			node = defaultNode
		}

		status := strings.TrimSpace(fmt.Sprint(item["status"]))
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		vmid := strings.TrimSpace(fmt.Sprint(item["vmid"]))
		if vmid == "<nil>" {
			vmid = ""
		}

		normalized = append(normalized, map[string]interface{}{
			"type":    entryType,
			"node":    node,
			"vmid":    vmid,
			"name":    name,
			"status":  status,
			"running": status == "running",
			"cpu":     item["cpu"],
			"mem":     item["mem"],
			"maxmem":  item["maxmem"],
			"disk":    item["disk"],
			"maxdisk": item["maxdisk"],
			"uptime":  item["uptime"],
			"tags":    item["tags"],
		})
	}
	return normalized
}

func proxmoxBuildStateSummary(items []map[string]interface{}) map[string]interface{} {
	summary := map[string]interface{}{
		"total":   len(items),
		"running": 0,
		"stopped": 0,
		"paused":  0,
		"unknown": 0,
	}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(item["status"])))
		switch status {
		case "running":
			summary["running"] = summary["running"].(int) + 1
		case "stopped":
			summary["stopped"] = summary["stopped"].(int) + 1
		case "paused", "suspended":
			summary["paused"] = summary["paused"].(int) + 1
		default:
			summary["unknown"] = summary["unknown"].(int) + 1
		}
	}
	return summary
}

func proxmoxNormalizeNodeList(items []map[string]interface{}) []map[string]interface{} {
	normalized := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		node := strings.TrimSpace(fmt.Sprint(item["node"]))
		status := strings.TrimSpace(fmt.Sprint(item["status"]))
		normalized = append(normalized, map[string]interface{}{
			"type":    "node",
			"node":    node,
			"status":  status,
			"online":  status == "online",
			"level":   item["level"],
			"cpu":     item["cpu"],
			"mem":     item["mem"],
			"maxmem":  item["maxmem"],
			"disk":    item["disk"],
			"maxdisk": item["maxdisk"],
			"uptime":  item["uptime"],
			"maxcpu":  item["maxcpu"],
		})
	}
	return normalized
}

func proxmoxBuildNodeSummary(items []map[string]interface{}) map[string]interface{} {
	summary := map[string]interface{}{
		"total":   len(items),
		"online":  0,
		"offline": 0,
		"unknown": 0,
	}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(item["status"])))
		switch status {
		case "online":
			summary["online"] = summary["online"].(int) + 1
		case "offline":
			summary["offline"] = summary["offline"].(int) + 1
		default:
			summary["unknown"] = summary["unknown"].(int) + 1
		}
	}
	return summary
}

// ProxmoxListNodes returns all nodes in the cluster.
func ProxmoxListNodes(ctx context.Context, cfg ProxmoxConfig) string {
	data, code, err := proxmoxRequest(ctx, cfg, "GET", "/nodes", "")
	if err != nil {
		return errJSON("Failed to list nodes: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","nodes":%s}`, proxmoxExtractData(data))
}

// ProxmoxListVMs returns all QEMU VMs on a node.
func ProxmoxListVMs(ctx context.Context, cfg ProxmoxConfig, node string) string {
	node = strings.TrimSpace(node)
	if node == "" {
		node = strings.TrimSpace(cfg.Node)
	}
	if node != "" {
		segment, err := proxmoxNodePath(node)
		if err != nil {
			return proxmoxStatusError(err)
		}
		data, code, err := proxmoxRequest(ctx, cfg, "GET", "/nodes/"+segment+"/qemu", "")
		if err != nil {
			return errJSON("Failed to list VMs: %v", err)
		}
		if code == 200 {
			var raw []map[string]interface{}
			if err := json.Unmarshal(proxmoxExtractData(data), &raw); err != nil {
				return errJSON("Failed to parse VMs: %v", err)
			}
			if raw == nil {
				raw = []map[string]interface{}{}
			}
			normalized := proxmoxNormalizeResourceList(raw, "qemu", node)
			summary := proxmoxBuildStateSummary(normalized)
			rawOut, _ := json.Marshal(raw)
			normalizedOut, _ := json.Marshal(normalized)
			summaryOut, _ := json.Marshal(summary)
			return fmt.Sprintf(`{"status":"ok","scope":"node","node":%s,"vms":%s,"monitoring":%s,"summary":%s}`, proxmoxJSONStr(node), rawOut, normalizedOut, summaryOut)
		}
		if code != http.StatusForbidden {
			return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
		}
	}

	resources, _, err := proxmoxGetClusterResourceList(ctx, cfg, "qemu")
	if err != nil {
		if node == "" {
			return `{"status":"error","message":"No node specified. Set proxmox.node in config or provide node parameter."}`
		}
		return errJSON("Failed to list VMs: %v", err)
	}
	filtered := proxmoxFilterClusterResources(resources, "qemu", node, "")
	normalized := proxmoxNormalizeResourceList(filtered, "qemu", node)
	summary := proxmoxBuildStateSummary(normalized)
	rawOut, _ := json.Marshal(filtered)
	normalizedOut, _ := json.Marshal(normalized)
	summaryOut, _ := json.Marshal(summary)
	scope := "cluster"
	if node != "" {
		scope = "node"
	}
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","scope":%s,"node":%s,"vms":%s,"monitoring":%s,"summary":%s}`, proxmoxJSONStr(scope), proxmoxJSONStr(node), rawOut, normalizedOut, summaryOut)
}

// ProxmoxListContainers returns all LXC containers on a node.
func ProxmoxListContainers(ctx context.Context, cfg ProxmoxConfig, node string) string {
	node = strings.TrimSpace(node)
	if node == "" {
		node = strings.TrimSpace(cfg.Node)
	}
	if node != "" {
		segment, err := proxmoxNodePath(node)
		if err != nil {
			return proxmoxStatusError(err)
		}
		data, code, err := proxmoxRequest(ctx, cfg, "GET", "/nodes/"+segment+"/lxc", "")
		if err != nil {
			return errJSON("Failed to list containers: %v", err)
		}
		if code == 200 {
			var raw []map[string]interface{}
			if err := json.Unmarshal(proxmoxExtractData(data), &raw); err != nil {
				return errJSON("Failed to parse containers: %v", err)
			}
			if raw == nil {
				raw = []map[string]interface{}{}
			}
			normalized := proxmoxNormalizeResourceList(raw, "lxc", node)
			summary := proxmoxBuildStateSummary(normalized)
			rawOut, _ := json.Marshal(raw)
			normalizedOut, _ := json.Marshal(normalized)
			summaryOut, _ := json.Marshal(summary)
			return fmt.Sprintf(`{"status":"ok","scope":"node","node":%s,"containers":%s,"monitoring":%s,"summary":%s}`, proxmoxJSONStr(node), rawOut, normalizedOut, summaryOut)
		}
		if code != http.StatusForbidden {
			return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
		}
	}

	resources, _, err := proxmoxGetClusterResourceList(ctx, cfg, "lxc")
	if err != nil {
		if node == "" {
			return `{"status":"error","message":"No node specified."}`
		}
		return errJSON("Failed to list containers: %v", err)
	}
	filtered := proxmoxFilterClusterResources(resources, "lxc", node, "")
	normalized := proxmoxNormalizeResourceList(filtered, "lxc", node)
	summary := proxmoxBuildStateSummary(normalized)
	rawOut, _ := json.Marshal(filtered)
	normalizedOut, _ := json.Marshal(normalized)
	summaryOut, _ := json.Marshal(summary)
	scope := "cluster"
	if node != "" {
		scope = "node"
	}
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","scope":%s,"node":%s,"containers":%s,"monitoring":%s,"summary":%s}`, proxmoxJSONStr(scope), proxmoxJSONStr(node), rawOut, normalizedOut, summaryOut)
}

// proxmoxValidateVMType validates and defaults the VM type.
func proxmoxValidateVMType(vmType string) (string, error) {
	if vmType == "" {
		return "qemu", nil
	}
	if vmType == "qemu" || vmType == "lxc" {
		return vmType, nil
	}
	return "", fmt.Errorf("invalid vmType %q, must be qemu or lxc", vmType)
}

// proxmoxValidateVMID validates that vmid is numeric.
func proxmoxValidateVMID(vmid string) error {
	if _, err := strconv.Atoi(vmid); err != nil {
		return fmt.Errorf("vmid must be numeric, got %q", vmid)
	}
	return nil
}

// ProxmoxGetStatus returns the status of a VM or container.
func ProxmoxGetStatus(ctx context.Context, cfg ProxmoxConfig, node string, vmType string, vmid string) string {
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return proxmoxStatusError(err)
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return proxmoxStatusError(err)
	}
	node, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	endpoint := "/nodes/" + segment + "/" + url.PathEscape(vmType) + "/" + url.PathEscape(vmid) + "/status/current"
	data, code, err := proxmoxRequest(ctx, cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to get status: %v", err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","data":%s}`, proxmoxExtractData(data))
	}
	if code != http.StatusForbidden {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}

	resources, _, err := proxmoxGetClusterResourceList(ctx, cfg, vmType)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	filtered := proxmoxFilterClusterResources(resources, vmType, node, vmid)
	if len(filtered) == 0 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":"status endpoint forbidden and resource not found in cluster overview"}`, code)
	}
	out, _ := json.Marshal(filtered[0])
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","data":%s}`, out)
}

// ProxmoxVMAction performs start/stop/shutdown/reboot/suspend/resume on a VM or container.
func ProxmoxVMAction(ctx context.Context, cfg ProxmoxConfig, node string, vmType string, vmid string, action string) string {
	if denied := proxmoxReadOnlyMutationError(cfg); denied != "" {
		return denied
	}
	if denied := proxmoxDestructiveActionError(cfg, action); denied != "" {
		return denied
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return proxmoxStatusError(err)
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return proxmoxStatusError(err)
	}
	validActions := map[string]bool{
		"start": true, "stop": true, "shutdown": true,
		"reboot": true, "suspend": true, "resume": true, "reset": true,
	}
	if !validActions[action] {
		return errJSON("Invalid action '%s'. Use: start, stop, shutdown, reboot, suspend, resume, reset", action)
	}
	_, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	endpoint := "/nodes/" + segment + "/" + url.PathEscape(vmType) + "/" + url.PathEscape(vmid) + "/status/" + url.PathEscape(action)
	data, code, err := proxmoxRequest(ctx, cfg, "POST", endpoint, "")
	if err != nil {
		return errJSON("Action failed: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","action":"%s","vmid":"%s","upid":%s}`, action, vmid, proxmoxExtractData(data))
}

// ProxmoxNodeStatus returns resource usage for a node.
func ProxmoxNodeStatus(ctx context.Context, cfg ProxmoxConfig, node string) string {
	node, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	data, code, err := proxmoxRequest(ctx, cfg, "GET", "/nodes/"+segment+"/status", "")
	if err != nil {
		return errJSON("Failed to get node status: %v", err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","data":%s}`, proxmoxExtractData(data))
	}
	if code != http.StatusForbidden {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}

	resources, _, err := proxmoxGetClusterResourceList(ctx, cfg, "node")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	filtered := proxmoxFilterClusterResources(resources, "node", node, "")
	if len(filtered) == 0 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":"node status endpoint forbidden and node not found in cluster overview"}`, code)
	}
	out, _ := json.Marshal(filtered[0])
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","data":%s}`, out)
}

// ProxmoxOverview returns a monitoring-oriented overview of nodes, VMs, and containers.
// Uses a single unfiltered /cluster/resources call instead of two separate calls.
func ProxmoxOverview(ctx context.Context, cfg ProxmoxConfig, node string) string {
	allResources, _, err := proxmoxGetClusterResourceList(ctx, cfg, "")
	if err != nil {
		return errJSON("Failed to get cluster overview: %v", err)
	}

	filteredNodes := proxmoxFilterClusterResources(allResources, "node", node, "")
	normalizedNodes := proxmoxNormalizeNodeList(filteredNodes)
	nodeSummary := proxmoxBuildNodeSummary(normalizedNodes)

	filteredVMs := proxmoxFilterClusterResources(allResources, "qemu", node, "")
	filteredContainers := proxmoxFilterClusterResources(allResources, "lxc", node, "")
	normalizedVMs := proxmoxNormalizeResourceList(filteredVMs, "qemu", node)
	normalizedContainers := proxmoxNormalizeResourceList(filteredContainers, "lxc", node)
	vmSummary := proxmoxBuildStateSummary(normalizedVMs)
	containerSummary := proxmoxBuildStateSummary(normalizedContainers)

	scope := "cluster"
	if strings.TrimSpace(node) != "" {
		scope = "node"
	}

	nodesOut, _ := json.Marshal(normalizedNodes)
	vmsOut, _ := json.Marshal(normalizedVMs)
	containersOut, _ := json.Marshal(normalizedContainers)
	nodeSummaryOut, _ := json.Marshal(nodeSummary)
	vmSummaryOut, _ := json.Marshal(vmSummary)
	containerSummaryOut, _ := json.Marshal(containerSummary)

	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","scope":%s,"node":%s,"nodes":%s,"vms":%s,"containers":%s,"summary":{"nodes":%s,"vms":%s,"containers":%s}}`,
		proxmoxJSONStr(scope), proxmoxJSONStr(node), nodesOut, vmsOut, containersOut, nodeSummaryOut, vmSummaryOut, containerSummaryOut)
}

// ProxmoxClusterResources returns a unified list of all resources (VMs, CTs, storage, nodes).
func ProxmoxClusterResources(ctx context.Context, cfg ProxmoxConfig, resType string) string {
	endpoint := "/cluster/resources"
	if resType != "" {
		endpoint += "?type=" + url.QueryEscape(resType)
	}
	data, code, err := proxmoxRequest(ctx, cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to get resources: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","resources":%s}`, proxmoxExtractData(data))
}

// ProxmoxGetStorage returns storage info for a node.
func ProxmoxGetStorage(ctx context.Context, cfg ProxmoxConfig, node string) string {
	_, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	data, code, err := proxmoxRequest(ctx, cfg, "GET", "/nodes/"+segment+"/storage", "")
	if err != nil {
		return errJSON("Failed to get storage: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","storage":%s}`, proxmoxExtractData(data))
}

// ProxmoxGetTaskLog returns the log for a specific task (UPID).
func ProxmoxGetTaskLog(ctx context.Context, cfg ProxmoxConfig, node string, upid string) string {
	if strings.TrimSpace(upid) == "" {
		return `{"status":"error","message":"upid is required."}`
	}
	_, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	endpoint := "/nodes/" + segment + "/tasks/" + url.PathEscape(upid) + "/log"
	data, code, err := proxmoxRequest(ctx, cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to get task log: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","log":%s}`, proxmoxExtractData(data))
}

// ProxmoxCreateSnapshot creates a snapshot of a VM or container.
func ProxmoxCreateSnapshot(ctx context.Context, cfg ProxmoxConfig, node, vmType, vmid, snapName, description string) string {
	if denied := proxmoxReadOnlyMutationError(cfg); denied != "" {
		return denied
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return proxmoxStatusError(err)
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return proxmoxStatusError(err)
	}
	_, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	if snapName == "" {
		snapName = fmt.Sprintf("snap_%d", time.Now().Unix())
	}
	body := "snapname=" + url.QueryEscape(snapName)
	if description != "" {
		body += "&description=" + url.QueryEscape(description)
	}
	endpoint := "/nodes/" + segment + "/" + url.PathEscape(vmType) + "/" + url.PathEscape(vmid) + "/snapshot"
	data, code, err := proxmoxRequest(ctx, cfg, "POST", endpoint, body)
	if err != nil {
		return errJSON("Snapshot failed: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	out, err := json.Marshal(map[string]interface{}{"status": "ok", "snapshot": snapName, "data": json.RawMessage(proxmoxExtractData(data))})
	if err != nil {
		return fmt.Sprintf(`{"status":"ok","snapshot":%s,"data":%s}`, proxmoxJSONStr(snapName), proxmoxExtractData(data))
	}
	return string(out)
}

// ProxmoxListSnapshots lists all snapshots of a VM or container.
func ProxmoxListSnapshots(ctx context.Context, cfg ProxmoxConfig, node, vmType, vmid string) string {
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return proxmoxStatusError(err)
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return proxmoxStatusError(err)
	}
	_, segment, err := proxmoxRequireNode(cfg, node)
	if err != nil {
		return proxmoxStatusError(err)
	}
	endpoint := "/nodes/" + segment + "/" + url.PathEscape(vmType) + "/" + url.PathEscape(vmid) + "/snapshot"
	data, code, err := proxmoxRequest(ctx, cfg, "GET", endpoint, "")
	if err != nil {
		return errJSON("Failed to list snapshots: %v", err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","snapshots":%s}`, proxmoxExtractData(data))
}
