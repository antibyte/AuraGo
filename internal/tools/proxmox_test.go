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
