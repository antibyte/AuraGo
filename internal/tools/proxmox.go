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

// ProxmoxListNodes returns all nodes in the cluster.
func ProxmoxListNodes(cfg ProxmoxConfig) string {
	data, code, err := proxmoxRequest(cfg, "GET", "/nodes", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to list nodes: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
			return fmt.Sprintf(`{"status":"ok","vms":%s}`, proxmoxExtractData(data))
		}
		if code != http.StatusForbidden {
			return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	out, _ := json.Marshal(filtered)
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","vms":%s}`, out)
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
			return fmt.Sprintf(`{"status":"ok","containers":%s}`, proxmoxExtractData(data))
		}
		if code != http.StatusForbidden {
			return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	out, _ := json.Marshal(filtered)
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","containers":%s}`, out)
}

// ProxmoxGetStatus returns the status of a VM or container.
func ProxmoxGetStatus(cfg ProxmoxConfig, node string, vmType string, vmid string) string {
	if node == "" {
		node = cfg.Node
	}
	if vmid == "" {
		return `{"status":"error","message":"vmid is required."}`
	}
	if vmType == "" {
		vmType = "qemu"
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/status/current", node, vmType, vmid)
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get status: %v"}`, err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","data":%s}`, proxmoxExtractData(data))
	}
	if code != http.StatusForbidden {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	resources, _, err := proxmoxGetClusterResourceList(cfg, "vm")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	if vmType == "" {
		vmType = "qemu"
	}
	validActions := map[string]bool{
		"start": true, "stop": true, "shutdown": true,
		"reboot": true, "suspend": true, "resume": true, "reset": true,
	}
	if !validActions[action] {
		return fmt.Sprintf(`{"status":"error","message":"Invalid action '%s'. Use: start, stop, shutdown, reboot, suspend, resume, reset"}`, action)
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/status/%s", node, vmType, vmid, action)
	data, code, err := proxmoxRequest(cfg, "POST", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Action failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	data, code, err := proxmoxRequest(cfg, "GET", fmt.Sprintf("/nodes/%s/status", node), "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get node status: %v"}`, err)
	}
	if code == 200 {
		return fmt.Sprintf(`{"status":"ok","data":%s}`, proxmoxExtractData(data))
	}
	if code != http.StatusForbidden {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	resources, _, err := proxmoxGetClusterResourceList(cfg, "node")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	filtered := proxmoxFilterClusterResources(resources, "node", node, "")
	if len(filtered) == 0 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":"node status endpoint forbidden and node not found in cluster overview"}`, code)
	}
	out, _ := json.Marshal(filtered[0])
	return fmt.Sprintf(`{"status":"ok","source":"cluster_resources","data":%s}`, out)
}

// ProxmoxClusterResources returns a unified list of all resources (VMs, CTs, storage, nodes).
func ProxmoxClusterResources(cfg ProxmoxConfig, resType string) string {
	endpoint := "/cluster/resources"
	if resType != "" {
		endpoint += "?type=" + resType
	}
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get resources: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	data, code, err := proxmoxRequest(cfg, "GET", fmt.Sprintf("/nodes/%s/storage", node), "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get storage: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	endpoint := fmt.Sprintf("/nodes/%s/tasks/%s/log", node, upid)
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to get task log: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	if vmType == "" {
		vmType = "qemu"
	}
	if snapName == "" {
		snapName = fmt.Sprintf("snap_%d", time.Now().Unix())
	}
	body := "snapname=" + snapName
	if description != "" {
		body += "&description=" + description
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", node, vmType, vmid)
	data, code, err := proxmoxRequest(cfg, "POST", endpoint, body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Snapshot failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
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
	if vmType == "" {
		vmType = "qemu"
	}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", node, vmType, vmid)
	data, code, err := proxmoxRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to list snapshots: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","snapshots":%s}`, proxmoxExtractData(data))
}
