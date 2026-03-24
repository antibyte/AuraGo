package tools

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGetProxmoxClient_CacheIsolatedByInsecureFlag(t *testing.T) {
	proxmoxClientCache = sync.Map{}

	secureClient := getProxmoxClient(ProxmoxConfig{Insecure: false})
	insecureClient := getProxmoxClient(ProxmoxConfig{Insecure: true})

	if secureClient == insecureClient {
		t.Fatal("expected secure and insecure clients to be different instances")
	}
}

func TestProxmoxRequest_RejectsNonHTTPSURL(t *testing.T) {
	_, _, err := proxmoxRequest(ProxmoxConfig{
		URL:     "http://example.com:8006",
		TokenID: "user@pam!token",
		Secret:  "secret",
	}, "GET", "/nodes", "")
	if err == nil {
		t.Fatal("expected non-https URL to be rejected")
	}
}

func TestProxmoxRequest_RejectsHTTPSWithoutHost(t *testing.T) {
	_, _, err := proxmoxRequest(ProxmoxConfig{
		URL:     "https://",
		TokenID: "user@pam!token",
		Secret:  "secret",
	}, "GET", "/nodes", "")
	if err == nil {
		t.Fatal("expected https URL without host to be rejected")
	}
}

func TestProxmoxGetStatus_FallsBackToClusterResourcesOn403(t *testing.T) {
	proxmoxClientCache = sync.Map{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve/qemu/101/status/current":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":{"permission":"denied"}}`))
		case "/api2/json/cluster/resources":
			if got := r.URL.Query().Get("type"); got != "vm" {
				t.Fatalf("expected cluster resource type vm, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "qemu", "node": "pve", "vmid": 101, "status": "running"},
				},
			})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := server.Client()
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxmoxClientCache.Store(server.URL+"|insecure", client)

	got := ProxmoxGetStatus(ProxmoxConfig{
		URL:      server.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Node:     "pve",
		Insecure: true,
	}, "pve", "qemu", "101")

	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected ok response, got %s", got)
	}
	if !strings.Contains(got, `"source":"cluster_resources"`) {
		t.Fatalf("expected cluster_resources fallback, got %s", got)
	}
	if !strings.Contains(got, `"vmid":101`) || !strings.Contains(got, `"status":"running"`) {
		t.Fatalf("expected VM status data in fallback response, got %s", got)
	}
}

func TestProxmoxNodeStatus_FallsBackToClusterResourcesOn403(t *testing.T) {
	proxmoxClientCache = sync.Map{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve/status":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":{"permission":"denied"}}`))
		case "/api2/json/cluster/resources":
			if got := r.URL.Query().Get("type"); got != "node" {
				t.Fatalf("expected cluster resource type node, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "node", "node": "pve", "status": "online", "cpu": 0.42},
				},
			})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := server.Client()
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxmoxClientCache.Store(server.URL+"|insecure", client)

	got := ProxmoxNodeStatus(ProxmoxConfig{
		URL:      server.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Node:     "pve",
		Insecure: true,
	}, "pve")

	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected ok response, got %s", got)
	}
	if !strings.Contains(got, `"source":"cluster_resources"`) {
		t.Fatalf("expected cluster_resources fallback, got %s", got)
	}
	if !strings.Contains(got, `"node":"pve"`) || !strings.Contains(got, `"status":"online"`) {
		t.Fatalf("expected node status data in fallback response, got %s", got)
	}
}

func TestProxmoxListVMs_IncludesMonitoringSummary(t *testing.T) {
	proxmoxClientCache = sync.Map{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve/qemu":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"vmid": 100, "name": "web-01", "status": "running", "cpu": 0.12, "mem": 512, "maxmem": 1024, "disk": 100, "maxdisk": 200},
					{"vmid": 101, "name": "db-01", "status": "stopped", "cpu": 0.0, "mem": 0, "maxmem": 2048, "disk": 200, "maxdisk": 400},
				},
			})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := server.Client()
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxmoxClientCache.Store(server.URL+"|insecure", client)

	got := ProxmoxListVMs(ProxmoxConfig{
		URL:      server.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Node:     "pve",
		Insecure: true,
	}, "pve")

	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected ok response, got %s", got)
	}

	var payload struct {
		Status     string                   `json:"status"`
		Monitoring []map[string]interface{} `json:"monitoring"`
		Summary    map[string]interface{}   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nraw=%s", err, got)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %#v", payload)
	}
	if len(payload.Monitoring) != 2 {
		t.Fatalf("expected 2 monitoring entries, got %#v", payload.Monitoring)
	}
	if payload.Summary["total"] != float64(2) || payload.Summary["running"] != float64(1) || payload.Summary["stopped"] != float64(1) {
		t.Fatalf("unexpected summary: %#v", payload.Summary)
	}
	firstRunning, _ := payload.Monitoring[0]["running"].(bool)
	secondRunning, _ := payload.Monitoring[1]["running"].(bool)
	if !firstRunning || secondRunning {
		t.Fatalf("expected normalized running flags, got %#v", payload.Monitoring)
	}
}

func TestProxmoxOverview_ReturnsCombinedMonitoringData(t *testing.T) {
	proxmoxClientCache = sync.Map{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/resources" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("type") {
		case "node":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "node", "node": "pve", "status": "online", "cpu": 0.33, "mem": 4096, "maxmem": 8192},
				},
			})
		case "vm":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "qemu", "node": "pve", "vmid": 100, "name": "web-01", "status": "running"},
					{"type": "lxc", "node": "pve", "vmid": 200, "name": "cache-01", "status": "stopped"},
				},
			})
		default:
			t.Fatalf("unexpected cluster resource type: %q", r.URL.Query().Get("type"))
		}
	}))
	defer server.Close()

	got := ProxmoxOverview(ProxmoxConfig{
		URL:      server.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Node:     "pve",
		Insecure: true,
	}, "pve")

	var payload struct {
		Status     string                   `json:"status"`
		Source     string                   `json:"source"`
		Scope      string                   `json:"scope"`
		Node       string                   `json:"node"`
		Nodes      []map[string]interface{} `json:"nodes"`
		VMs        []map[string]interface{} `json:"vms"`
		Containers []map[string]interface{} `json:"containers"`
		Summary    map[string]interface{}   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nraw=%s", err, got)
	}
	if payload.Status != "ok" || payload.Source != "cluster_resources" || payload.Scope != "node" || payload.Node != "pve" {
		t.Fatalf("unexpected overview payload header: %#v", payload)
	}
	if len(payload.Nodes) != 1 || len(payload.VMs) != 1 || len(payload.Containers) != 1 {
		t.Fatalf("unexpected overview resources: %#v", payload)
	}
	nodesSummary, _ := payload.Summary["nodes"].(map[string]interface{})
	vmsSummary, _ := payload.Summary["vms"].(map[string]interface{})
	containersSummary, _ := payload.Summary["containers"].(map[string]interface{})
	if nodesSummary["online"] != float64(1) || vmsSummary["running"] != float64(1) || containersSummary["stopped"] != float64(1) {
		t.Fatalf("unexpected overview summaries: %#v", payload.Summary)
	}
}
