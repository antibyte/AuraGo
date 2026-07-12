package virtualcomputers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientStatusUsesHealthz(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"machines":2,"kvm":true}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "super-secret", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if gotPath != "/healthz" {
		t.Fatalf("path = %q, want /healthz", gotPath)
	}
	if status["ok"] != true {
		t.Fatalf("status = %v", status)
	}
}

func TestClientLaunchMachineAddsAuthAndClampsTTL(t *testing.T) {
	var authHeader string
	var ttl int
	var netEnabled bool
	var volume string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		ttl = int(req["ttl_seconds"].(float64))
		netEnabled = req["net"] == true
		volume, _ = req["volume"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"vm-1","status":"running"}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "super-secret", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	machine, err := client.LaunchMachine(context.Background(), LaunchMachineRequest{
		Template:      "python",
		TTLSeconds:    5000,
		AllowInternet: true,
		Volumes:       []string{"vol-1", "vol-ignored"},
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
	if !netEnabled {
		t.Fatalf("net flag was not forwarded in boringd payload")
	}
	if volume != "vol-1" {
		t.Fatalf("volume = %q, want first configured volume", volume)
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

func TestClientReportsNonJSONResponseWithURLHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>AuraGo</title>token super-secret"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "super-secret", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.ListTemplates(context.Background())
	if err == nil {
		t.Fatal("expected non-JSON error")
	}
	msg := err.Error()
	for _, want := range []string{"non-JSON", "boringd_url", "not AuraGo UI"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
	if strings.Contains(msg, "super-secret") {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestClientUsesBoringdContractPathsForBranchAndFiles(t *testing.T) {
	var paths []string
	var uploadFilename string
	var uploadBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/machines/vm-1/branch":
			_, _ = w.Write([]byte(`{"id":"vm-2","status":"running"}`))
		case "/v1/machines/vm-1/upload":
			uploadFilename = r.Header.Get("X-Filename")
			data, _ := io.ReadAll(r.Body)
			uploadBody = string(data)
			_, _ = w.Write([]byte(`{"ok":true,"path":"/root/file.txt","bytes":5}`))
		case "/v1/machines/vm-1/download":
			if r.URL.Query().Get("path") != "/root/file.txt" {
				t.Fatalf("download path query = %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("hello"))
		default:
			t.Fatalf("unexpected path %s", r.URL.RequestURI())
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "super-secret", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if machine, err := client.ForkMachine(context.Background(), "vm-1", 300); err != nil || machine.ID != "vm-2" {
		t.Fatalf("ForkMachine = %+v, %v", machine, err)
	}
	if _, err := client.Upload(context.Background(), "vm-1", "/root/file.txt", []byte("hello")); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	data, contentType, err := client.Download(context.Background(), "vm-1", "/root/file.txt")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(data) != "hello" || contentType != "application/octet-stream" {
		t.Fatalf("download = %q %q", string(data), contentType)
	}
	if uploadFilename != "file.txt" {
		t.Fatalf("X-Filename = %q, want file.txt", uploadFilename)
	}
	if uploadBody != "hello" {
		t.Fatalf("upload body = %q", uploadBody)
	}
	wantPaths := []string{
		"POST /v1/machines/vm-1/branch",
		"POST /v1/machines/vm-1/upload",
		"GET /v1/machines/vm-1/download?path=%2Froot%2Ffile.txt",
	}
	if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("paths = %v, want %v", paths, wantPaths)
	}
}

func TestPreviewProxyPath(t *testing.T) {
	got := PreviewProxyPath("vm/../../one", 8080, "/deep/path")
	want := "/api/virtual-computers/machines/vm..one/web/8080/deep/path"
	if got != want {
		t.Fatalf("PreviewProxyPath = %q, want %q", got, want)
	}
}
