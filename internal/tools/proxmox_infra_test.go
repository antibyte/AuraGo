package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProxmoxInfraResourcesReturnsOnlyTypedProjection(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/resources" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"data":[{"type":"qemu","vmid":101,"node":"pve-a","name":"web","status":"running","cpu":0.25,"maxcpu":4,"mem":1024,"maxmem":4096,"disk":20,"maxdisk":80,"tags":"prod"},{"type":"node","node":"pve-a","name":"pve-a","status":"online","maxcpu":16,"maxmem":68719476736},{"type":"storage","vmid":9,"node":"pve-a","secret":"do-not-project"},{"type":"lxc","vmid":102,"node":"pve-b","name":"db","status":"stopped","tags":"db"}]}`)
	}))
	defer server.Close()

	resources, err := ProxmoxInfraResources(context.Background(), ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 3 || resources[0].Type != "qemu" || resources[0].VMID != "101" || resources[0].Node != "pve-a" || resources[1].Type != "node" || resources[1].VMID != "" || resources[2].Type != "lxc" {
		t.Fatalf("unexpected projection: %+v", resources)
	}
	encoded, _ := json.Marshal(resources)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "storage") {
		t.Fatalf("unallowlisted provider data escaped: %s", encoded)
	}
}

func TestProxmoxInfraResourcesRejectsIncompleteEnumeration(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"type":"qemu","vmid":101,"name":"missing-node"}]}`)
	}))
	defer server.Close()
	if _, err := ProxmoxInfraResources(context.Background(), ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}); err == nil {
		t.Fatal("incomplete VM enumeration was accepted and could mark a valid source removed")
	}
}

func TestProxmoxInfraActionReturnsUPIDAndHonorsGates(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/api2/json/nodes/pve-a/qemu/101/status/start" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"data":"UPID:pve-a:00000001:start"}`)
	}))
	defer server.Close()
	cfg := ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}
	status, upid, err := ProxmoxInfraAction(context.Background(), cfg, "pve-a", "qemu", "101", "start")
	if err != nil || status != "accepted" || upid != "UPID:pve-a:00000001:start" {
		t.Fatalf("action = %q, %q, %v", status, upid, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("action requests = %d, want one", calls.Load())
	}
	status, _, _ = ProxmoxInfraAction(context.Background(), ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true, ReadOnly: true}, "pve-a", "qemu", "101", "start")
	if status != "failed" || calls.Load() != 1 {
		t.Fatalf("readonly action = %q, requests = %d", status, calls.Load())
	}
	status, _, _ = ProxmoxInfraAction(context.Background(), cfg, "pve-a", "qemu", "101", "shutdown")
	if status != "failed" || calls.Load() != 1 {
		t.Fatalf("ungated shutdown = %q, requests = %d", status, calls.Load())
	}
}

func TestProxmoxInfraActionDoesNotRetryUncertainWrite(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer server.Close()
	status, _, err := ProxmoxInfraAction(context.Background(), ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}, "pve-a", "qemu", "101", "start")
	if status != "unknown" || err == nil || calls.Load() != 1 {
		t.Fatalf("uncertain action = %q, err=%v, requests=%d; want one non-retried unknown", status, err, calls.Load())
	}
}

func TestProxmoxInfraActionRejectsUnsafeUPID(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"data":"UPID:pve-a:00000001:start\\nsecret"}`)
	}))
	defer server.Close()
	status, upid, err := ProxmoxInfraAction(context.Background(), ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}, "pve-a", "qemu", "101", "start")
	if status != "unknown" || upid != "" || err == nil || calls.Load() != 1 {
		t.Fatalf("unsafe task id = status %q, upid %q, err %v, requests %d", status, upid, err, calls.Load())
	}
}

func TestProxmoxInfraReadConfigUsesPendingAndCurrentQEMUViews(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api2/json/nodes/pve-a/qemu/101/config" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("current") == "1" {
			fmt.Fprint(w, `{"data":{"name":"old-name","description":"old description","tags":"old","digest":"digest-1"}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"name":"new-name","description":"new description","tags":"new","digest":"digest-1"}}`)
	}))
	defer server.Close()

	values, err := ProxmoxInfraReadConfig(context.Background(), ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}, "pve-a", "qemu", "101")
	if err != nil {
		t.Fatal(err)
	}
	if values.Name != "new-name" || values.Description != "new description" || values.Tags != "new" || values.Digest != "digest-1" || !values.Pending {
		t.Fatalf("effective QEMU config = %#v, want pending effective values", values)
	}
}

func TestProxmoxInfraReadAndEditLXCUsesHostnameField(t *testing.T) {
	var wrote atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve-a/lxc/102/config" {
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
		switch r.Method {
		case http.MethodGet:
			if r.URL.RawQuery != "" {
				t.Fatalf("LXC config should not request QEMU current view: %s", r.URL.RawQuery)
			}
			hostname := "old-host"
			if wrote.Load() {
				hostname = "new-host"
			}
			fmt.Fprintf(w, `{"data":{"hostname":%q,"description":"container","tags":"lab","digest":"digest-2"}}`, hostname)
		case http.MethodPut:
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("hostname") != "new-host" || r.Form.Get("name") != "" || r.Form.Get("digest") != "digest-2" {
				t.Fatalf("LXC config form = %#v", r.Form)
			}
			wrote.Store(true)
			fmt.Fprint(w, `{"data":null}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()
	cfg := ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}
	before, err := ProxmoxInfraReadConfig(context.Background(), cfg, "pve-a", "lxc", "102")
	if err != nil || before.Name != "old-host" || before.Pending {
		t.Fatalf("initial LXC config = %#v, err=%v", before, err)
	}
	status, err := ProxmoxInfraEditConfig(context.Background(), cfg, "pve-a", "lxc", "102", before, "new-host", "container", "lab")
	if err != nil || status != "completed" {
		t.Fatalf("LXC edit status=%q err=%v", status, err)
	}
}

func TestProxmoxInfraEditConfigReportsPendingQEMUMetadata(t *testing.T) {
	var wrote atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve-a/qemu/101/config" {
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("current") == "1" {
				fmt.Fprint(w, `{"data":{"name":"old-name","description":"old description","tags":"old","digest":"digest-3"}}`)
				return
			}
			if wrote.Load() {
				fmt.Fprint(w, `{"data":{"name":"new-name","description":"new description","tags":"new","digest":"digest-4"}}`)
				return
			}
			fmt.Fprint(w, `{"data":{"name":"old-name","description":"old description","tags":"old","digest":"digest-3"}}`)
		case http.MethodPut:
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("name") != "new-name" || r.Form.Get("hostname") != "" || r.Form.Get("description") != "new description" || r.Form.Get("tags") != "new" || r.Form.Get("digest") != "digest-3" {
				t.Fatalf("QEMU config form = %#v", r.Form)
			}
			wrote.Store(true)
			fmt.Fprint(w, `{"data":null}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()
	cfg := ProxmoxConfig{URL: server.URL, TokenID: "operator!infra", Secret: "secret", Insecure: true}
	before, err := ProxmoxInfraReadConfig(context.Background(), cfg, "pve-a", "qemu", "101")
	if err != nil || before.Pending {
		t.Fatalf("initial QEMU config = %#v, err=%v", before, err)
	}
	status, err := ProxmoxInfraEditConfig(context.Background(), cfg, "pve-a", "qemu", "101", before, "new-name", "new description", "new")
	if err != nil || status != "pending" {
		t.Fatalf("QEMU edit status=%q err=%v, want pending", status, err)
	}
}
