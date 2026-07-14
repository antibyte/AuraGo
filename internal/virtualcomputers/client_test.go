package virtualcomputers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientScreenshotReadsRawPNG(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines/vm-desktop/screenshot" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	shot, err := client.Screenshot(context.Background(), "vm-desktop")
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if shot.MimeType != "image/png" || !bytes.Equal(shot.Data, png) {
		t.Fatalf("screenshot = mime %q data %q", shot.MimeType, shot.Data)
	}
	if shot.Base64 == "" {
		t.Fatal("screenshot base64 is empty")
	}
}

func TestClientDecodesPersistentMachineWithEmptyExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"vm-persistent","status":"running","mode":"cold","boot_ms":42,"template":"desktop","display":true,"created_at":"2026-07-14T08:00:00Z","expires_at":"","persistent":true,"parent":"vm-parent"}`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	machine, err := client.GetMachine(context.Background(), "vm-persistent")
	if err != nil {
		t.Fatalf("GetMachine: %v", err)
	}
	if machine.ExpiresAt != nil {
		t.Fatalf("expires_at = %v, want nil", machine.ExpiresAt)
	}
	if !machine.Persistent || !machine.Display || machine.Mode != "cold" || machine.BootMS != 42 || machine.Parent != "vm-parent" {
		t.Fatalf("machine fields not normalized: %+v", machine)
	}
}

func TestClientUsesPinnedPublishForkExecAndVolumeContracts(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/machines/vm-1/publish":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "golden-template" {
				t.Fatalf("publish body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"name":"golden-template","published":true}`))
		case "/v1/machines/vm-1/branch":
			if r.URL.Query().Get("count") != "2" {
				t.Fatalf("fork query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"machines":[{"id":"vm-2"},{"id":"vm-3"}],"requested":2}`))
		case "/v1/machines/vm-1/exec":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, ok := body["args"]; ok {
				t.Fatalf("exec sent unsupported args: %#v", body)
			}
			_, _ = w.Write([]byte(`{"output":"ok","exit_code":null,"timed_out":true,"duration_ms":30001}`))
		case "/v1/volumes":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body) != 1 || body["ttl_seconds"] != float64(3600) {
				t.Fatalf("volume body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"id":"vol-1","created_at":"2026-07-14T08:00:00Z","expires_at":"2026-07-14T09:00:00Z","quota_mb":256}`))
		case "/v1/volumes/vol-1":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(`{"id":"vol-1","created_at":"2026-07-14T08:00:00Z","expires_at":"2026-07-14T09:00:00Z","quota_mb":256}`))
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			}
		case "/v1/machines/vm-1/save":
			if r.URL.Query().Get("volume") != "vol-1" {
				t.Fatalf("save query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"ok":true,"volume":"vol-1"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.Publish(context.Background(), "vm-1", "golden-template"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	forks, err := client.ForkMachines(context.Background(), "vm-1", 2)
	if err != nil || len(forks) != 2 {
		t.Fatalf("ForkMachines = %+v, %v", forks, err)
	}
	result, err := client.Exec(context.Background(), "vm-1", ExecRequest{Command: "sleep 31"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if result.ExitCode != nil || !result.TimedOut || result.DurationMS != 30001 || result.Output != "ok" {
		t.Fatalf("exec result = %+v", result)
	}
	volume, err := client.CreateVolume(context.Background(), 3600)
	if err != nil || volume.ID != "vol-1" || volume.QuotaMB != 256 {
		t.Fatalf("CreateVolume = %+v, %v", volume, err)
	}
	if _, err := client.GetVolume(context.Background(), "vol-1"); err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if err := client.DeleteVolume(context.Background(), "vol-1"); err != nil {
		t.Fatalf("DeleteVolume: %v", err)
	}
	if _, err := client.SaveMachine(context.Background(), "vm-1", "vol-1"); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}
	for _, request := range requests {
		if request == "GET /v1/volumes" {
			t.Fatalf("invalid global volume listing was requested: %v", requests)
		}
	}
}

func TestNewClientUsesDedicatedBoringdDefaultPort(t *testing.T) {
	client, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got := client.baseURL.String(); got != "http://127.0.0.1:18082" {
		t.Fatalf("base URL = %q", got)
	}
}

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
		VolumeID:      "vol-1",
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

func TestClassifyErrorMapsKnownBoringdFailures(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		code       string
		httpStatus int
	}{
		{name: "headless screenshot", err: RESTError{StatusCode: 502, Body: `{"error":"machine has no vsock device"}`}, code: "capability_unavailable", httpStatus: http.StatusConflict},
		{name: "headless screenshot with changed status", err: RESTError{StatusCode: 500, Body: `{"error":"machine has no vsock device"}`}, code: "capability_unavailable", httpStatus: http.StatusConflict},
		{name: "file transfer", err: RESTError{StatusCode: 409, Body: `{"error":"file transfer requires a connected computer"}`}, code: "machine_not_connected", httpStatus: http.StatusConflict},
		{name: "file transfer with changed status", err: RESTError{StatusCode: 500, Body: `{"error":"file transfer requires a connected computer"}`}, code: "machine_not_connected", httpStatus: http.StatusConflict},
		{name: "busy", err: RESTError{StatusCode: 409, Body: `{"error":"machine console is busy"}`}, code: "machine_busy", httpStatus: http.StatusConflict},
		{name: "missing", err: RESTError{StatusCode: 404, Body: `{"error":"not found"}`}, code: "not_found", httpStatus: http.StatusNotFound},
		{name: "volume routes disabled", err: RESTError{Path: "/v1/volumes/vol-1", StatusCode: 404, Body: "404 page not found"}, code: "storage_unavailable", httpStatus: http.StatusServiceUnavailable},
		{name: "rate limit", err: RESTError{StatusCode: 429, Body: `{"error":"too many requests"}`}, code: "rate_limited", httpStatus: http.StatusTooManyRequests},
		{name: "local validation", err: fmt.Errorf("upload filename is required"), code: "invalid_argument", httpStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classified := ClassifyError(tc.err)
			if classified.Code != tc.code || classified.HTTPStatus != tc.httpStatus {
				t.Fatalf("classification = %+v, want %s/%d", classified, tc.code, tc.httpStatus)
			}
		})
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
	if _, err := client.Upload(context.Background(), "vm-1", "..", []byte("blocked")); err == nil {
		t.Fatal("Upload accepted unsafe parent-directory filename")
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
