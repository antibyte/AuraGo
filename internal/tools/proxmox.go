package tools

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProxmoxConfig holds the Proxmox VE connection parameters.
type ProxmoxConfig struct {
	URL      string // e.g. "https://pve.example.com:8006"
	TokenID  string // e.g. "user@pam!tokenname"
	Secret   string // API token secret
	Node     string // default node name (e.g. "pve")
	Insecure bool   // skip TLS verification
}

var proxmoxClientCache sync.Map

func getProxmoxClient(cfg ProxmoxConfig) *http.Client {
	if cached, ok := proxmoxClientCache.Load(cfg.Insecure); ok {
		return cached.(*http.Client)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.Insecure,
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
	actual, _ := proxmoxClientCache.LoadOrStore(cfg.Insecure, client)
	return actual.(*http.Client)
}

// proxmoxJSONStr produces a JSON-safe quoted string (unlike %q which uses Go escaping).
func proxmoxJSONStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// proxmoxRequest performs a generic HTTP request against the Proxmox VE API.
func proxmoxRequest(cfg ProxmoxConfig, method, endpoint string, body string) ([]byte, int, error) {
	baseURL := strings.TrimRight(cfg.URL, "/")
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid proxmox url %q: %w", cfg.URL, err)
	}
	if parsedURL.Scheme != "https" {
		return nil, 0, fmt.Errorf("invalid proxmox url scheme %q: only https is supported", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return nil, 0, fmt.Errorf("invalid proxmox url %q: host is required", cfg.URL)
	}
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

	resp, err := getProxmoxClient(cfg).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
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

func proxmoxGetClusterResourceList(cfg ProxmoxConfig, resType string) ([]map[string]interface{}, int, error) {
	endpoint := "/cluster/resources"
	if resType != "" {
		endpoint += "?type=" + url.QueryEscape(resType)
	}
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
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
func ProxmoxListNodes(cfg ProxmoxConfig) string {
	data, code, err := proxmoxRequest(cfg, "GET", "/nodes", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to list nodes: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","nodes":%s}`, proxmoxExtractData(data))
}

// ProxmoxListVMs returns all QEMU VMs on a node.
func ProxmoxListVMs(cfg ProxmoxConfig, node string) string {
	if node == "" {
		node = cfg.Node
	}
	if node != "" {
		data, code, err := proxmoxRequest(cfg, "GET", fmt.Sprintf("/nodes/%s/qemu", node), "")
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"Failed to list VMs: %v"}`, err)
		}
		if code == 200 {
			var raw []map[string]interface{}
			if err := json.Unmarshal(proxmoxExtractData(data), &raw); err != nil {
				return fmt.Sprintf(`{"status":"error","message":"Failed to parse VMs: %v"}`, err)
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

	resources, _, err := proxmoxGetClusterResourceList(cfg, "vm")
	if err != nil {
		if node == "" {
			return `{"status":"error","message":"No node specified. Set proxmox.node in config or provide node parameter."}`
		}
		return fmt.Sprintf(`{"status":"error","message":"Failed to list VMs: %v"}`, err)
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
func ProxmoxListContainers(cfg ProxmoxConfig, node string) string {
	if node == "" {
		node = cfg.Node
	}
	if node != "" {
		data, code, err := proxmoxRequest(cfg, "GET", fmt.Sprintf("/nodes/%s/lxc", node), "")
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"Failed to list containers: %v"}`, err)
		}
		if code == 200 {
			var raw []map[string]interface{}
			if err := json.Unmarshal(proxmoxExtractData(data), &raw); err != nil {
				return fmt.Sprintf(`{"status":"error","message":"Failed to parse containers: %v"}`, err)
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

	resources, _, err := proxmoxGetClusterResourceList(cfg, "vm")
	if err != nil {
		if node == "" {
			return `{"status":"error","message":"No node specified."}`
		}
		return fmt.Sprintf(`{"status":"error","message":"Failed to list containers: %v"}`, err)
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
func ProxmoxGetStatus(cfg ProxmoxConfig, node string, vmType string, vmid string) string {
	if node == "" {
		node = cfg.Node
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/status/current", url.PathEscape(node), url.PathEscape(vmType), url.PathEscape(vmid))
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get status: %v"}`, err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","data":%s}`, proxmoxExtractData(data))
	}
	if code != http.StatusForbidden {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}

	resources, _, err := proxmoxGetClusterResourceList(cfg, "vm")
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
func ProxmoxVMAction(cfg ProxmoxConfig, node string, vmType string, vmid string, action string) string {
	if node == "" {
		node = cfg.Node
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	validActions := map[string]bool{
		"start": true, "stop": true, "shutdown": true,
		"reboot": true, "suspend": true, "resume": true, "reset": true,
	}
	if !validActions[action] {
		return fmt.Sprintf(`{"status":"error","message":"Invalid action '%s'. Use: start, stop, shutdown, reboot, suspend, resume, reset"}`, action)
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/status/%s", url.PathEscape(node), url.PathEscape(vmType), url.PathEscape(vmid), url.PathEscape(action))
	data, code, err := proxmoxRequest(cfg, "POST", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Action failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","action":"%s","vmid":"%s","upid":%s}`, action, vmid, proxmoxExtractData(data))
}

// ProxmoxNodeStatus returns resource usage for a node.
func ProxmoxNodeStatus(cfg ProxmoxConfig, node string) string {
	if node == "" {
		node = cfg.Node
	}
	if node == "" {
		return `{"status":"error","message":"No node specified."}`
	}
	data, code, err := proxmoxRequest(cfg, "GET", fmt.Sprintf("/nodes/%s/status", url.PathEscape(node)), "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get node status: %v"}`, err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","data":%s}`, proxmoxExtractData(data))
	}
	if code != http.StatusForbidden {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}

	resources, _, err := proxmoxGetClusterResourceList(cfg, "node")
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
func ProxmoxOverview(cfg ProxmoxConfig, node string) string {
	allResources, _, err := proxmoxGetClusterResourceList(cfg, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get cluster overview: %v"}`, err)
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
func ProxmoxClusterResources(cfg ProxmoxConfig, resType string) string {
	endpoint := "/cluster/resources"
	if resType != "" {
		endpoint += "?type=" + url.QueryEscape(resType)
	}
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get resources: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","resources":%s}`, proxmoxExtractData(data))
}

// ProxmoxGetStorage returns storage info for a node.
func ProxmoxGetStorage(cfg ProxmoxConfig, node string) string {
	if node == "" {
		node = cfg.Node
	}
	if node == "" {
		return `{"status":"error","message":"No node specified."}`
	}
	data, code, err := proxmoxRequest(cfg, "GET", fmt.Sprintf("/nodes/%s/storage", url.PathEscape(node)), "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get storage: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","storage":%s}`, proxmoxExtractData(data))
}

// ProxmoxGetTaskLog returns the log for a specific task (UPID).
func ProxmoxGetTaskLog(cfg ProxmoxConfig, node string, upid string) string {
	if node == "" {
		node = cfg.Node
	}
	if upid == "" {
		return `{"status":"error","message":"upid is required."}`
	}
	endpoint := fmt.Sprintf("/nodes/%s/tasks/%s/log", url.PathEscape(node), url.PathEscape(upid))
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get task log: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","log":%s}`, proxmoxExtractData(data))
}

// ProxmoxCreateSnapshot creates a snapshot of a VM or container.
func ProxmoxCreateSnapshot(cfg ProxmoxConfig, node, vmType, vmid, snapName, description string) string {
	if node == "" {
		node = cfg.Node
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	if snapName == "" {
		snapName = fmt.Sprintf("snap_%d", time.Now().Unix())
	}
	body := "snapname=" + url.QueryEscape(snapName)
	if description != "" {
		body += "&description=" + url.QueryEscape(description)
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", url.PathEscape(node), url.PathEscape(vmType), url.PathEscape(vmid))
	data, code, err := proxmoxRequest(cfg, "POST", endpoint, body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Snapshot failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","snapshot":"%s","data":%s}`, snapName, proxmoxExtractData(data))
}

// ProxmoxListSnapshots lists all snapshots of a VM or container.
func ProxmoxListSnapshots(cfg ProxmoxConfig, node, vmType, vmid string) string {
	if node == "" {
		node = cfg.Node
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	var err error
	vmType, err = proxmoxValidateVMType(vmType)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	if err = proxmoxValidateVMID(vmid); err != nil {
		return fmt.Sprintf(`{"status":"error","message":%s}`, proxmoxJSONStr(err.Error()))
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", url.PathEscape(node), url.PathEscape(vmType), url.PathEscape(vmid))
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to list snapshots: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%s}`, code, proxmoxJSONStr(string(data)))
	}
	return fmt.Sprintf(`{"status":"ok","snapshots":%s}`, proxmoxExtractData(data))
}
