package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/httporigin"
	"aurago/internal/testutil"
)

func TestProxmoxListVMsRejectsNodePathInjection(t *testing.T) {
	proxmoxClientCache = sync.Map{}
	var hits atomic.Int32
	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/api2/json/nodes/pve/qemu" && r.URL.Path != "/api2/json/nodes/pve/lxc" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	cfg := ProxmoxConfig{
		URL:      server.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Insecure: true,
	}
	for _, node := range []string{"pve/qemu/100/config#", "pve?x=1", "pve/qemu", "..", "pve node"} {
		t.Run(node, func(t *testing.T) {
			got := ProxmoxListVMs(context.Background(), cfg, node)
			if !strings.Contains(got, "invalid node") {
				t.Fatalf("response = %s, want invalid node", got)
			}
			got = ProxmoxListContainers(context.Background(), cfg, node)
			if !strings.Contains(got, "invalid node") {
				t.Fatalf("container response = %s, want invalid node", got)
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("invalid node names made %d HTTP requests", hits.Load())
	}

	got := ProxmoxListVMs(context.Background(), cfg, "pve")
	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("valid node response = %s", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("valid node hits = %d, want 1", hits.Load())
	}
}

func TestProxmoxListVMsEmptyNodeIsSuccess(t *testing.T) {
	proxmoxClientCache = sync.Map{}
	var clusterHits atomic.Int32
	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/cluster/resources" {
			clusterHits.Add(1)
			t.Errorf("empty 200 must not fall back to %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/api2/json/nodes/pve/qemu", "/api2/json/nodes/pve/lxc":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := ProxmoxConfig{
		URL:      server.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Insecure: true,
	}
	for _, list := range []struct {
		name string
		call func() string
		key  string
	}{
		{name: "vms", key: "vms", call: func() string { return ProxmoxListVMs(context.Background(), cfg, "pve") }},
		{name: "containers", key: "containers", call: func() string { return ProxmoxListContainers(context.Background(), cfg, "pve") }},
	} {
		t.Run(list.name, func(t *testing.T) {
			var payload struct {
				Status     string                   `json:"status"`
				VMs        []map[string]interface{} `json:"vms"`
				Containers []map[string]interface{} `json:"containers"`
				Summary    map[string]interface{}   `json:"summary"`
			}
			got := list.call()
			if err := json.Unmarshal([]byte(got), &payload); err != nil {
				t.Fatalf("parse: %v raw=%s", err, got)
			}
			if payload.Status != "ok" || payload.Summary["total"] != float64(0) {
				t.Fatalf("unexpected payload: %s", got)
			}
			guests := payload.VMs
			if list.key == "containers" {
				guests = payload.Containers
			}
			if len(guests) != 0 {
				t.Fatalf("expected empty %s, got %#v", list.key, guests)
			}
		})
	}
	if clusterHits.Load() != 0 {
		t.Fatalf("cluster fallback hits = %d", clusterHits.Load())
	}

	t.Run("forbidden falls back", func(t *testing.T) {
		proxmoxClientCache = sync.Map{}
		fallback := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api2/json/nodes/pve/qemu":
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"errors":{"permission":"denied"}}`))
			case "/api2/json/cluster/resources":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[{"type":"qemu","node":"pve","vmid":101,"status":"running"}]}`))
			default:
				t.Errorf("unexpected request path: %s", r.URL.Path)
			}
		}))
		defer fallback.Close()
		got := ProxmoxListVMs(context.Background(), ProxmoxConfig{
			URL:      fallback.URL,
			TokenID:  "user@pam!token",
			Secret:   "secret",
			Insecure: true,
		}, "pve")
		if !strings.Contains(got, `"source":"cluster_resources"`) || !strings.Contains(got, `"status":"ok"`) {
			t.Fatalf("403 response = %s, want cluster fallback", got)
		}
	})
}

func TestProxmoxRequestRejectsNonOriginURL(t *testing.T) {
	for _, raw := range []string{
		"https://user:pass@host:8006/extra?x=1#frag",
		"https://host:8006/pve/../api",
		"https://host:8006?x=1",
		"https://host:8006#frag",
	} {
		t.Run(raw, func(t *testing.T) {
			_, _, err := proxmoxRequest(context.Background(), ProxmoxConfig{
				URL:     raw,
				TokenID: "user@pam!token",
				Secret:  "secret",
			}, "GET", "/nodes", "")
			if err == nil {
				t.Fatal("expected origin rejection")
			}
			if strings.Contains(err.Error(), "pass") || strings.Contains(err.Error(), "user:") {
				t.Fatalf("error leaked credentials: %v", err)
			}
		})
	}
}

func TestProxmoxRequestKeepsPathPrefix(t *testing.T) {
	proxmoxClientCache = sync.Map{}
	var got string
	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	_, _, err := proxmoxRequest(context.Background(), ProxmoxConfig{
		URL:      server.URL + "/pve/",
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Insecure: true,
	}, "GET", "/nodes", "")
	if err != nil {
		t.Fatalf("proxmoxRequest: %v", err)
	}
	if got != "/pve/api2/json/nodes" {
		t.Fatalf("path = %q, want /pve/api2/json/nodes", got)
	}
}

func TestProxmoxRequestRejectsCrossOriginRedirect(t *testing.T) {
	proxmoxClientCache = sync.Map{}
	var secondHits atomic.Int32
	var secondAuth atomic.Value
	second := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		secondAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer second.Close()

	first := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL+"/api2/json/nodes", http.StatusFound)
	}))
	defer first.Close()

	_, _, err := proxmoxRequest(context.Background(), ProxmoxConfig{
		URL:      first.URL,
		TokenID:  "user@pam!token",
		Secret:   "secret",
		Insecure: true,
	}, "GET", "/nodes", "")
	if !errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		t.Fatalf("error = %v, want ErrCrossOriginRedirect", err)
	}
	auth, _ := secondAuth.Load().(string)
	if secondHits.Load() != 0 || auth != "" {
		t.Fatalf("second host hits=%d auth=%q", secondHits.Load(), auth)
	}
}

func TestProxmoxRequestHonorsCallerCancel(t *testing.T) {
	proxmoxClientCache = sync.Map{}
	started := make(chan struct{})
	server := testutil.NewHTTPSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		_, _, err := proxmoxRequest(ctx, ProxmoxConfig{
			URL:      server.URL,
			TokenID:  "user@pam!token",
			Secret:   "secret",
			Insecure: true,
		}, "GET", "/nodes", "")
		errCh <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach the handler")
	}
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("cancel took %s, want well under the 30s budget", time.Since(start))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled request did not return")
	}
}
