package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/inventory"
)

func quickConnectGreetingFixture(t *testing.T, protocol, banner string) (inventory.DeviceRecord, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, _ = io.WriteString(conn, banner)
		data, _ := io.ReadAll(conn)
		if len(data) != 0 {
			t.Error("eligibility probe sent data to the remote service")
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	address := listener.Addr().(*net.TCPAddr)
	return inventory.DeviceRecord{ID: "fixture", Name: "Fixture", Type: "server", Protocol: protocol, IPAddress: "127.0.0.1", Port: address.Port}, accepted
}

func TestQuickConnectDevicesVerifyProtocolGreetings(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, banner string
		want                   bool
	}{
		{"SSH on custom port", "ssh", "SSH-2.0-TestServer\r\n", true},
		{"SSH with preamble", "ssh", "Welcome\r\nSSH-2.0-TestServer\r\n", true},
		{"SSH compatibility version", "ssh", "SSH-1.99-TestServer\r\n", true},
		{"obsolete SSH", "ssh", "SSH-1.5-TestServer\r\n", false},
		{"VNC on custom port", "vnc", "RFB 003.008\n", true},
		{"HTTP marked as SSH", "ssh", "HTTP/1.1 200 OK\r\n\r\n", false},
		{"HTTP marked as VNC", "vnc", "HTTP/1.1 200 OK\r\n\r\n", false},
		{"SSH marked as VNC", "vnc", "SSH-2.0-TestServer\r\n", false},
		{"VNC malformed version", "vnc", "RFB abc.def\n", false},
		{"oversized greeting", "ssh", strings.Repeat("x", 4096) + "\nSSH-2.0-TestServer\r\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			device, _ := quickConnectGreetingFixture(t, tc.protocol, tc.banner)
			got := quickConnectDevices(context.Background(), []inventory.DeviceRecord{device})
			if len(got) != 1 || got[0].QuickConnect != tc.want {
				t.Fatalf("quick connect eligibility = %+v, want %v", got, tc.want)
			}
			if got[0].Protocol != device.Protocol || got[0].Port != device.Port {
				t.Fatal("eligibility check changed the stored endpoint")
			}
		})
	}
}

func TestQuickConnectDevicesPreserveConfiguredConnections(t *testing.T) {
	devices := []inventory.DeviceRecord{
		{ID: "ssh-credential", Protocol: "ssh", CredentialID: "fixture-credential"},
		{ID: "ssh-username", Protocol: "ssh", Username: "root"},
		{ID: "vnc-secret", Protocol: "vnc", VaultSecretID: "fixture-secret"},
		{ID: "inventory-only", Protocol: "none", CredentialID: "fixture-credential"},
		{ID: "unknown-protocol", Protocol: "http", Username: "root"},
		{ID: "unconfigured-ssh", Protocol: "ssh", Username: "  "},
		{ID: "empty-protocol", Type: "server"},
	}
	got := quickConnectDevices(context.Background(), devices)
	for index := range devices {
		if got[index].ID != devices[index].ID || got[index].QuickConnect != (index < 3) {
			t.Errorf("device %s: eligibility = %v", got[index].ID, got[index].QuickConnect)
		}
	}
}

func TestQuickConnectDevicesCancelStalledGreeting(t *testing.T) {
	device, accepted := quickConnectGreetingFixture(t, "ssh", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan []quickConnectDevice, 1)
	go func() { done <- quickConnectDevices(ctx, []inventory.DeviceRecord{device}) }()
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not connect to greeting fixture")
	}
	cancel()
	select {
	case got := <-done:
		if got[0].QuickConnect {
			t.Fatal("cancelled probe granted eligibility")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancellation did not close the stalled greeting read")
	}
}

func TestHandleListDevicesQuickConnectProjection(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	device, _ := quickConnectGreetingFixture(t, "ssh", "HTTP/1.1 200 OK\r\n\r\n")
	if err := inventory.AddDevice(db, device); err != nil {
		t.Fatal(err)
	}
	s := &Server{InventoryDB: db, Cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	handleListDevices(s)(rec, httptest.NewRequest(http.MethodGet, "/api/devices?quick_connect=1", nil))
	var projected []quickConnectDevice
	if err := json.Unmarshal(rec.Body.Bytes(), &projected); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || len(projected) != 1 || projected[0].QuickConnect {
		t.Fatalf("unexpected projection: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleListDevices(s)(rec, httptest.NewRequest(http.MethodGet, "/api/devices", nil))
	var ordinary []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &ordinary); err != nil {
		t.Fatal(err)
	}
	if len(ordinary) != 1 || ordinary[0]["protocol"] != "ssh" {
		t.Fatalf("ordinary inventory was filtered or changed: %s", rec.Body.String())
	}
	if _, exists := ordinary[0]["quick_connect"]; exists {
		t.Fatal("ordinary inventory unexpectedly includes probe results")
	}
}

func TestHandleListDevicesQuickConnectRequiresAdminBeforeProbing(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	device, accepted := quickConnectGreetingFixture(t, "ssh", "SSH-2.0-TestServer\r\n")
	if err := inventory.AddDevice(db, device); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	s := &Server{InventoryDB: db, Cfg: cfg}
	rec := httptest.NewRecorder()
	handleListDevices(s)(rec, httptest.NewRequest(http.MethodGet, "/api/devices?quick_connect=1", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	select {
	case <-accepted:
		t.Fatal("unauthorized request dialed a device")
	default:
	}
}
