package virtualcomputers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientLaunchMachineAddsAuthAndClampsTTL(t *testing.T) {
	var authHeader string
	var ttl int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		var req launchMachineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		ttl = req.TTLSeconds
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"vm-1","status":"running"}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "super-secret", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	machine, err := client.LaunchMachine(context.Background(), LaunchMachineRequest{
		Template:   "python",
		TTLSeconds: 5000,
	})
	if err != nil {
		t.Fatalf("LaunchMachine: %v", err)
	}
	if machine.ID != "vm-1" {
		t.Fatalf("machine id = %q", machine.ID)
	}
	if authHeader != "Bearer super-secret" {
		t.Fatalf("Authorization = %q", authHeader)
	}
	if ttl != MaxTTLSeconds {
		t.Fatalf("ttl = %d, want %d", ttl, MaxTTLSeconds)
	}
}

func TestClientRESTErrorRedactsToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "token super-secret rejected", http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "super-secret", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.ListMachines(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error leaked token: %v", err)
	}
	if !strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("error did not mark redaction: %v", err)
	}
}

func TestPreviewProxyPath(t *testing.T) {
	got := PreviewProxyPath("vm/../../one", 8080, "/deep/path")
	want := "/api/virtual-computers/machines/vm..one/web/8080/deep/path"
	if got != want {
		t.Fatalf("PreviewProxyPath = %q, want %q", got, want)
	}
}
